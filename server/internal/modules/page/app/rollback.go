package app

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// An upgrade's rollback (M11 design 4.10; M11/P1 design 3.18), a
// background job of the transfer module, pass by pass: each pass deletes
// the new subtrees that hold no baseline node, or, with none left, puts
// the rest back by a revert's plan made from the baseline; the next pass
// reads what is left anew.

// RollbackReads are what a rollback reads of its upgrade's baseline and
// of the notebook, unlocked (M11/P1 design 3.18): what it has left to do,
// the free nodes, the baseline rows, the new nodes holding baseline nodes
// as they are now, and the baseline's nodes alive and pages' contents as
// they are now. Each reads the notebook's rows by their keys, a row at a
// time, its cost the notebook's and not the instance's (decision 240):
// nodes and contents of a revert's NodesNow and ContentsNow, by arrays of
// their ids, read the tables whole at this scale without statistics.
type RollbackReads interface {
	RollbackLeft(ctx context.Context, upgradeID, notebookID uuid.UUID) (domain.RollbackLeft, error)
	RollbackFreeForest(ctx context.Context, upgradeID, notebookID uuid.UUID) ([]domain.FreeNode, error)
	RollbackBaseline(ctx context.Context, upgradeID uuid.UUID) ([]domain.BaselineNode, error)
	RollbackHolders(ctx context.Context, upgradeID uuid.UUID) ([]domain.Node, error)
	RollbackNow(ctx context.Context, upgradeID uuid.UUID) (map[uuid.UUID]domain.NodeNow, map[uuid.UUID]domain.ContentNow, error)
}

// ErrRollbackGone is a rollback's job whose upgrade is no longer the
// notebook's, rolling back (M11 decision 233): it was ended meanwhile,
// and the job stops writing nothing; the composition root answers the
// transfer module's ErrJobGone.
var ErrRollbackGone = errors.New("page: the rollback's upgrade is no longer the notebook's, rolling back")

// RollbackPass is what one pass of an upgrade's rollback does (M11 design
// 4.10): delete the new subtrees that hold no baseline node, or, with
// none left, put the rest back by a plan made from the baseline; or
// nothing, the notebook being as the upgrade started. Left is what was
// left before it; Items its plan's items and versions, none for a pass
// that deletes.
type RollbackPass struct {
	Deletes bool
	Done    bool
	Left    domain.RollbackLeft
	Items   int
}

// rollbackPlan is a pass of a rollback (M11/P1 design 3.18): the roots of
// each step that deletes first; or, for a pass put back by a plan, the
// items and versions of the state it planned by, by node, which its
// batches check under the lock as a revert's check its changeset's.
type rollbackPlan struct {
	deletes  [][]uuid.UUID
	items    map[uuid.UUID]domain.ChangesetItem
	versions map[uuid.UUID]domain.ChangesetVersion
}

// rollbackMessage is the message of a rollback's changeset, fixed.
func rollbackMessage() *string {
	m := "Upgrade rollback"
	return &m
}

// rollbackUnit is the unit of a rollback's batch of spec (M11/P1 design
// 3.18): its changeset, made by its first batch that writes, the kind
// rollback with its message; the upgrade's own write, which the freeze
// lets through (UpgradeOwn); the notebook module's notebook.change_mode
// decided for the job's starter; no participant rewrites links, the
// pages' contents written back putting their links back (M11 design
// 4.10).
func rollbackUnit(spec JobSpec, action shared.Action) UnitSpec {
	return UnitSpec{NotebookID: spec.NotebookID, Action: action, Tree: true, Client: spec.Client,
		NotFound: domain.ErrNotebookNotFound, Kind: domain.ChangesetRollback, Changeset: spec.Changeset, Create: true,
		Message: rollbackMessage(), Background: true, UpgradeOwn: spec.UpgradeID, Options: Options{UpdateLinks: false}}
}

// PlanRollback plans one pass of the rollback of spec's upgrade, unlocked
// (the notebook is frozen, the rollback its one writer), in M11/P1 design
// 3.18's order: the decision of the job's starter first, as each batch's
// (notebook.not_found, forbidden); the notebook's upgrade must still be
// spec's, rolling back, else ErrRollbackGone; nothing left is Done; the
// free subtrees, when there are any, packed into steps of MaxUnitNodes at
// most; else the plan of the state of the baseline (domain.RollbackState),
// page.revert_too_large past MaxRollbackItems, its steps as a revert's
// (revertSteps). A plan with conflicts answers them, a defect: the
// baseline was valid.
func (w *JobWrites) PlanRollback(ctx context.Context, spec JobSpec) (JobPlan, RollbackPass, error) {
	if err := w.writer.Allowed(ctx, rollbackUnit(spec, w.upgradeAction)); err != nil {
		return JobPlan{}, RollbackPass{}, err
	}
	notebook, ok, err := w.writer.d.Notebooks.StateOf(ctx, spec.NotebookID)
	switch {
	case err != nil:
		return JobPlan{}, RollbackPass{}, err
	case !ok:
		return JobPlan{}, RollbackPass{}, domain.ErrNotebookNotFound
	case spec.UpgradeID == (uuid.UUID{}) || notebook.Upgrade.ID != spec.UpgradeID ||
		notebook.Upgrade.State != domain.UpgradeRollingBack:
		return JobPlan{}, RollbackPass{}, ErrRollbackGone
	}
	left, err := w.rollbacks.RollbackLeft(ctx, spec.UpgradeID, spec.NotebookID)
	if err != nil {
		return JobPlan{}, RollbackPass{}, err
	}
	pass := RollbackPass{Left: left}
	if left.Zero() {
		pass.Done = true
		return JobPlan{}, pass, nil
	}
	forest, err := w.rollbacks.RollbackFreeForest(ctx, spec.UpgradeID, spec.NotebookID)
	if err != nil {
		return JobPlan{}, RollbackPass{}, err
	}
	if len(forest) > 0 {
		pass.Deletes = true
		return deleteFirst(domain.PackFreeSubtrees(forest, w.maxUnitNodes), forest), pass, nil
	}
	state, err := w.rollbackState(ctx, spec)
	if err != nil {
		return JobPlan{}, RollbackPass{}, err
	}
	pass.Items = len(state.Items) + len(state.Versions)
	plan, err := planned(state)
	if err != nil {
		return JobPlan{}, RollbackPass{}, err
	}
	job := revertSteps(uuid.UUID{}, revertChecked{state: state, plan: plan})
	r := &rollbackPlan{items: make(map[uuid.UUID]domain.ChangesetItem, len(state.Items)),
		versions: make(map[uuid.UUID]domain.ChangesetVersion, len(state.Versions))}
	for _, it := range state.Items {
		r.items[it.NodeID] = it
	}
	for _, v := range state.Versions {
		r.versions[v.NodeID] = v
	}
	if len(job.revert.groups) > 0 {
		return JobPlan{}, RollbackPass{}, errors.New("page: a rollback's plan restores from the trash, a defect: it puts back nodes alive")
	}
	job.revert.rollback, job.rollback = r, r
	return job, pass, nil
}

// deleteFirst is a pass that deletes the free subtrees, a step for each
// pack of their roots, its nodes the subtrees' (sized from the forest).
func deleteFirst(packs [][]uuid.UUID, forest []domain.FreeNode) JobPlan {
	children := make(map[uuid.UUID][]uuid.UUID, len(forest))
	for _, n := range forest {
		if n.ParentID != nil {
			children[*n.ParentID] = append(children[*n.ParentID], n.ID)
		}
	}
	size := func(root uuid.UUID) int {
		n, stack := 0, []uuid.UUID{root}
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = append(stack[:len(stack)-1], children[id]...)
			n++
		}
		return n
	}
	steps := make([]domain.JobStep, len(packs))
	for i, roots := range packs {
		for _, root := range roots {
			steps[i].Nodes += size(root)
		}
	}
	return JobPlan{steps: steps, rollback: &rollbackPlan{deletes: packs}}
}

// rollbackState reads the state of the baseline of spec's upgrade, as a
// revert's check reads its changeset's (revertReads): the baseline rows,
// the new nodes holding baseline nodes, the baseline's nodes and contents
// now, and the notebook's tree when there is an item.
func (w *JobWrites) rollbackState(ctx context.Context, spec JobSpec) (domain.RevertState, error) {
	baseline, err := w.rollbacks.RollbackBaseline(ctx, spec.UpgradeID)
	if err != nil {
		return domain.RevertState{}, err
	}
	held, err := w.rollbacks.RollbackHolders(ctx, spec.UpgradeID)
	if err != nil {
		return domain.RevertState{}, err
	}
	now, contents, err := w.rollbacks.RollbackNow(ctx, spec.UpgradeID)
	if err != nil {
		return domain.RevertState{}, err
	}
	if now == nil {
		now = map[uuid.UUID]domain.NodeNow{}
	}
	holders := make([]uuid.UUID, len(held))
	for i, n := range held {
		holders[i], now[n.ID] = n.ID, domain.NodeNow{Node: n}
	}
	state, err := domain.RollbackState(baseline, holders, now, contents, w.maxRollbackItems)
	if err != nil {
		return domain.RevertState{}, err
	}
	if len(state.Items) > 0 {
		if state.Tree, err = w.reads.RevertTrees.ListRevertTree(ctx, spec.NotebookID); err != nil {
			return domain.RevertState{}, err
		}
	}
	state.At, state.Retention = w.reads.Clock.Now(), w.reads.Retention
	return state, nil
}

// deleteBatch deletes the free subtrees of the plan's steps from to to in
// a unit of spec: past MaxUnitNodes for steps of more than one pack,
// errPastNodes, as a revert's batch past its nodes.
func (w *JobWrites) deleteBatch(ctx context.Context, spec JobSpec, plan JobPlan, from, to int) (BatchDone, error) {
	var roots []uuid.UUID
	nodes := 0
	for i := from; i < to; i++ {
		roots, nodes = append(roots, plan.rollback.deletes[i]...), nodes+plan.steps[i].Nodes
	}
	if to-from > 1 && nodes > w.maxUnitNodes {
		return BatchDone{}, errPastNodes
	}
	var deleted int
	if _, err := w.writer.Run(ctx, rollbackUnit(spec, w.upgradeAction), func(ctx context.Context, u *Unit) error {
		var err error
		deleted, err = u.DeleteFreeSubtrees(ctx, roots)
		return err
	}); err != nil {
		return BatchDone{}, err
	}
	return BatchDone{Nodes: deleted, Deleted: deleted}, nil
}

// DeleteFreeSubtrees deletes the subtrees of roots into the trash, each
// a trash item of its own, as Delete does (M11/P1 design 3.18), and
// answers how many nodes they held: a root not in the notebook is a
// defect, the rollback being the frozen notebook's one writer.
func (u *Unit) DeleteFreeSubtrees(ctx context.Context, roots []uuid.UUID) (int, error) {
	n := 0
	for _, root := range roots {
		sub, err := u.Delete(ctx, root)
		if err != nil {
			return 0, fmt.Errorf("delete the free subtree of %s: %w", root, err)
		}
		n += len(sub)
	}
	return n, nil
}

// rollbackContents are the contents a rollback's batch writes back, by
// page and hash: the baseline's versions of the pages written, by their
// pages and revisions; one the purge took is page.changeset_trimmed.
func (w *JobWrites) rollbackContents(ctx context.Context, r *rollbackPlan, written []uuid.UUID) (map[domain.ContentRef]string, error) {
	out := map[domain.ContentRef]string{}
	if len(written) == 0 {
		return out, nil
	}
	refs := make([]domain.VersionRef, 0, len(written))
	for _, id := range written {
		if v, ok := r.versions[id]; ok && v.Base != nil {
			refs = append(refs, domain.VersionRef{NodeID: id, Revision: *v.Base})
		}
	}
	bases, err := w.history.VersionContents(ctx, refs)
	switch {
	case err != nil:
		return nil, err
	case len(refs) != len(written) || len(bases) != len(refs):
		return nil, domain.ErrChangesetTrimmed
	}
	for ref, v := range bases {
		out[contentOf(ref.NodeID, v.Content)] = v.Content
	}
	return out, nil
}

// read is the batch's items of the nodes items and versions of the pages
// versions of a rollback's plan, as batchChangeset reads a changeset's:
// those the plan has.
func (r *rollbackPlan) read(items, versions []uuid.UUID) domain.RevertState {
	var s domain.RevertState
	seen := map[uuid.UUID]bool{}
	for _, id := range items {
		if it, ok := r.items[id]; ok && !seen[id] {
			s.Items, seen[id] = append(s.Items, it), true
		}
	}
	seen = map[uuid.UUID]bool{}
	for _, id := range versions {
		if v, ok := r.versions[id]; ok && !seen[id] {
			s.Versions, seen[id] = append(s.Versions, v), true
		}
	}
	return s
}

// RollbackBatch puts back a batch of a rollback's pass planned from the
// baseline, under the notebook's lock (M11/P1 design 3.18), as
// RevertBatch reverts a changeset's: its items and versions the plan's,
// not a changeset's, the state of their nodes read at the unit's time
// and checked again, then its steps.
func (u *Unit) RollbackBatch(ctx context.Context, r RevertPart) (BatchDone, error) {
	if r.plan.rollback == nil {
		return BatchDone{}, errors.New("page: a rollback's batch of a revert's plan")
	}
	return u.revertPart(ctx, r)
}
