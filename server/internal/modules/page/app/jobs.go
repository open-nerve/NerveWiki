package app

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The background restores and reverts (M8 design 4.10; M8/P4 design 3.3
// to 3.5): a request's check, unlocked; the job's plan, read unlocked as
// it starts; and its batches, each a unit merged into the job's changeset,
// which the first that writes makes with the id the job's row names.

// JobSpec is a background job's units': its notebook and client; the
// changeset its row names, which its first unit that writes makes and its
// later units merge into; a revert's message, written as it is made.
type JobSpec struct {
	NotebookID uuid.UUID
	Client     domain.Client
	Changeset  uuid.UUID
	Message    *string
}

// JobPlan is a background job's plan: its steps, each a write no batch
// splits, in their order; its caller cuts them into batches and hands
// each back to RunBatch with the plan.
type JobPlan struct {
	steps   []domain.JobStep
	restore *restorePlan
	revert  *revertPlan
}

// Steps are the plan's steps in their order.
func (p JobPlan) Steps() []domain.JobStep {
	return p.steps
}

// Total is how many nodes the plan's steps write: a job's progress's
// total.
func (p JobPlan) Total() int {
	n := 0
	for _, s := range p.steps {
		n += s.Nodes
	}
	return n
}

// BatchDone is what a batch did: the nodes it wrote, for the statistics'
// pace; a restore's pages and attachments it brought back, those it
// renamed, those someone restored before it, skipped, and where it put the
// item's root, its first batch's; a revert's nodes it restored, deleted
// and put back, and pages it wrote back, and the members of a group
// someone restored where it would, skipped.
type BatchDone struct {
	Nodes, Pages, Assets, Renamed, Skipped int
	Restored, Deleted, PutBack, Written    int
	Placed                                 *domain.Placement
}

// JobWrites are the background restores' and reverts' writes.
type JobWrites struct {
	reads    TrashReads
	history  History
	writer   *Writer
	parser   *ContentParser
	sessions AliveSessions
	maxItems int
	// maxHistory bounds the rows of history a batch of more than one
	// step restores: MaxUnitHistory.
	maxHistory int
}

// NewJobWrites returns them, the domain's bounds where maxItems and
// maxHistory have none (0).
func NewJobWrites(reads TrashReads, history History, writer *Writer, parser *ContentParser, sessions AliveSessions, maxItems,
	maxHistory int,
) *JobWrites {
	return &JobWrites{reads: reads, history: history, writer: writer, parser: parser, sessions: sessions,
		maxItems: cmp.Or(maxItems, domain.MaxJobItems), maxHistory: cmp.Or(maxHistory, domain.MaxUnitHistory)}
}

// restoreUnit is the unit of a restore's batch of spec: the job's
// changeset, made by its first batch that writes.
func restoreUnit(spec JobSpec) UnitSpec {
	return UnitSpec{NotebookID: spec.NotebookID, Action: domain.ActionRestore, Tree: true, Client: spec.Client,
		NotFound: domain.ErrNotebookNotFound, Kind: domain.ChangesetTrashRestore, Changeset: spec.Changeset, Create: true}
}

// revertUnit is the unit of a revert's batch of spec, of the changeset
// reverted: the job's changeset, a revert of it with spec's message.
func revertUnit(spec JobSpec, reverted uuid.UUID) UnitSpec {
	return UnitSpec{NotebookID: spec.NotebookID, Action: domain.ActionRevert, Tree: true, Client: spec.Client,
		NotFound: domain.ErrNotebookNotFound, Kind: domain.ChangesetRevert, RevertsID: reverted, Changeset: spec.Changeset, Create: true,
		Message: spec.Message}
}

// checks are the reverts' unlocked checks over the job's reads.
func (w *JobWrites) checks() revertChecks {
	return revertChecks{reads: w.reads, history: w.history, writer: w.writer, sessions: w.sessions}
}

// CheckRestore checks a background restore of the trash item of the root
// nodeID in the notebook, unlocked, and answers its name (M8 design 4.14,
// startTrashRestore's order): notebook.not_found for a notebook that is
// not there or the caller cannot see, forbidden for a reader, page.not_found
// for a node not of the notebook, or purged; then page.not_in_trash,
// page.not_trash_root and page.restore_expired. Its size is no bound: any
// item restores in the background.
func (w *JobWrites) CheckRestore(ctx context.Context, notebookID, nodeID uuid.UUID) (string, error) {
	if err := w.writer.Allowed(ctx, restoreUnit(JobSpec{NotebookID: notebookID})); err != nil {
		return "", err
	}
	n, err := w.reads.Nodes.FindAnyNode(ctx, nodeID)
	switch {
	case errors.Is(err, ErrNotFound):
		return "", domain.ErrNotFound
	case err != nil:
		return "", err
	case n.NotebookID != notebookID:
		return "", domain.ErrNotFound
	}
	root, err := w.reads.trashRoot(ctx, n, w.reads.Clock.Now())
	if err != nil {
		return "", err
	}
	return root.Node.Name, nil
}

// CheckRevert checks a background revert of the changeset changesetID in
// the notebook with message (nil: none), unlocked, and answers the
// changeset's kind (M8 design 4.14, startRevert's order): a message that
// breaks its rules is 422 validation_failed; notebook.not_found and
// forbidden as CheckRestore; page.changeset_not_found for a changeset not
// of the notebook; then the revert's check (revertChecks.check), its
// items bounded by MaxJobItems, admit called right after them, before
// anything else is read, its error answered as it is; and page.locked
// after the plan's conflicts, for a page it deletes or writes back that
// an edit session holds.
func (w *JobWrites) CheckRevert(ctx context.Context, notebookID, changesetID uuid.UUID, message *string,
	admit func(context.Context) error,
) (domain.ChangesetKind, error) {
	if message != nil {
		if err := domain.CheckMessage(*message); err != nil {
			return "", err
		}
	}
	spec := revertUnit(JobSpec{NotebookID: notebookID, Client: domain.ClientWeb, Message: message}, changesetID)
	if err := w.writer.Allowed(ctx, spec); err != nil {
		return "", err
	}
	c, err := w.history.FindChangeset(ctx, changesetID)
	switch {
	case errors.Is(err, ErrNotFound):
		return "", domain.ErrChangesetNotFound
	case err != nil:
		return "", err
	case c.NotebookID != notebookID:
		return "", domain.ErrChangesetNotFound
	}
	if _, err := w.checks().check(ctx, c, spec, revertBounds{Items: w.maxItems, Admit: admit, Locks: true}); err != nil {
		return "", err
	}
	return c.Kind, nil
}

// restorePlan is a background restore's (M8/P4 design 3.4): the trash
// item of the root deleted at at, its counts as read, and its group in
// pre-order, a step each node.
type restorePlan struct {
	at                    time.Time
	pages, assets, height int
	nodes                 []domain.GroupNode
}

// PlanRestore plans the background restore of the trash item of the root
// nodeID in spec's notebook, unlocked, as its job starts: the decision
// first (a notebook gone or the job's account's role is notebook.not_found
// or forbidden); then the item, page.trash_changed for a node no longer an
// item's root, of the notebook, in the trash, page.restore_expired past
// its time; then its group's shape, a level at a time, put in pre-order
// and counted, page.trash_changed when it is not the group the root
// counts.
func (w *JobWrites) PlanRestore(ctx context.Context, spec JobSpec, nodeID uuid.UUID) (JobPlan, error) {
	if err := w.writer.Allowed(ctx, restoreUnit(spec)); err != nil {
		return JobPlan{}, err
	}
	n, err := w.reads.Nodes.FindAnyNode(ctx, nodeID)
	switch {
	case errors.Is(err, ErrNotFound):
		return JobPlan{}, domain.ErrTrashChanged
	case err != nil:
		return JobPlan{}, err
	case n.NotebookID != spec.NotebookID:
		return JobPlan{}, domain.ErrTrashChanged
	}
	root, err := w.reads.trashRoot(ctx, n, w.reads.Clock.Now())
	var refused *shared.Error
	switch {
	case errors.Is(err, domain.ErrRestoreExpired):
		return JobPlan{}, err
	case errors.As(err, &refused):
		// Restored since the job was started, or within another's item.
		return JobPlan{}, domain.ErrTrashChanged
	case err != nil:
		return JobPlan{}, err
	}
	read, err := w.reads.Trash.TrashNodesAt(ctx, spec.NotebookID, map[uuid.UUID]time.Time{nodeID: root.DeletedAt})
	if err != nil {
		return JobPlan{}, err
	}
	r, ok := read[nodeID]
	if !ok || !r.Root {
		return JobPlan{}, domain.ErrTrashChanged
	}
	below, err := w.reads.Trash.TrashShape(ctx, spec.NotebookID, nodeID, root.DeletedAt)
	if err != nil {
		return JobPlan{}, err
	}
	nodes := domain.GroupPreOrder(domain.ShapeNode{ID: nodeID, Asset: r.Node.Kind == domain.KindAsset, Bytes: r.Bytes}, below)
	domain.GroupCounts(nodes)
	if len(nodes) != len(below)+1 || int(nodes[0].Pages) != r.Pages || int(nodes[0].Assets) != r.Assets || int(nodes[0].Height) != r.Height {
		return JobPlan{}, domain.ErrTrashChanged
	}
	steps := make([]domain.JobStep, len(nodes))
	for i, m := range nodes {
		steps[i] = domain.JobStep{Nodes: 1, Bytes: int64(m.Bytes)}
	}
	return JobPlan{steps: steps, restore: &restorePlan{at: root.DeletedAt, pages: r.Pages, assets: r.Assets, height: r.Height,
		nodes: nodes}}, nil
}

// revertPlan is a background revert's (M8/P4 design 3.5): the changeset
// it reverts, its round's steps, and the groups its restores bring back.
type revertPlan struct {
	changeset uuid.UUID
	steps     []revertStep
	groups    []revertGroup
}

// revertStep is a step of a background revert: a place of node, a
// deletion of the top node's subtree, a restore of the member of a group,
// or a write back of the page node. A batch plans it again with the
// changeset's items and versions of nodes: a deletion's top and the
// changeset's nodes in its subtree; a group's, its first member's step;
// none for a later member of the group (a continuation's, checked alone).
// A place plans again with its node's item, a write back with its page's
// version.
type revertStep struct {
	kind   domain.RevertOpKind
	node   uuid.UUID
	nodes  []uuid.UUID
	group  int
	member int
}

// revertGroup is a trash item a background revert restores: its root,
// deleted at at, and its group in pre-order with its counts, whose member
// first is the restore's first step.
type revertGroup struct {
	at    time.Time
	nodes []domain.GroupNode
	first int
}

// PlanRevert plans the background revert of the changeset changesetID in
// spec's notebook, unlocked, as its job starts: the decision first, then
// the changeset, page.changeset_trimmed for one gone since, then the
// revert's check as CheckRevert has it (page.revert_conflict,
// page.changeset_trimmed, page.locked), its round's steps in their order:
// each place one node; each deletion its top's subtree alive now; each
// restore its group in pre-order, a node a step; each write back a page.
func (w *JobWrites) PlanRevert(ctx context.Context, spec JobSpec, changesetID uuid.UUID) (JobPlan, error) {
	unit := revertUnit(spec, changesetID)
	if err := w.writer.Allowed(ctx, unit); err != nil {
		return JobPlan{}, err
	}
	c, err := w.history.FindChangeset(ctx, changesetID)
	if errors.Is(err, ErrNotFound) {
		if err := w.writer.Allowed(ctx, unit); err != nil {
			return JobPlan{}, err
		}
		return JobPlan{}, domain.ErrChangesetTrimmed
	}
	if err != nil {
		return JobPlan{}, err
	}
	checked, err := w.checks().check(ctx, c, unit, revertBounds{Items: w.maxItems, Locks: true})
	if err != nil {
		return JobPlan{}, err
	}
	return revertSteps(c.ID, checked), nil
}

// revertSteps are the steps of the plan checked of the revert of the
// changeset reverted.
func revertSteps(reverted uuid.UUID, checked revertChecked) JobPlan {
	children := map[uuid.UUID][]uuid.UUID{}
	for _, n := range checked.state.Tree {
		if n.ParentID != nil {
			children[*n.ParentID] = append(children[*n.ParentID], n.ID)
		}
	}
	bases := map[uuid.UUID]int{}
	for _, v := range checked.state.Versions {
		bases[v.NodeID] = v.BaseBytes
	}
	p := &revertPlan{changeset: reverted}
	var steps []domain.JobStep
	add := func(s revertStep, step domain.JobStep) {
		p.steps, steps = append(p.steps, s), append(steps, step)
	}
	for _, op := range checked.plan.Ops {
		switch op.Kind {
		case domain.RevertPlace:
			add(revertStep{kind: op.Kind, node: op.NodeID, group: -1}, domain.JobStep{Nodes: 1})
		case domain.RevertDelete:
			sub := []uuid.UUID{op.NodeID}
			for at := 0; at < len(sub); at++ {
				sub = append(sub, children[sub[at]]...)
			}
			add(revertStep{kind: op.Kind, node: op.NodeID, nodes: sub, group: -1}, domain.JobStep{Nodes: len(sub)})
		case domain.RevertRestore:
			below := make([]domain.ShapeNode, 0, len(op.Group)-1)
			for _, m := range op.Group[1:] {
				below = append(below, domain.ShapeNode{ID: m.Node.ID, ParentID: *m.Node.ParentID, Asset: m.Node.Kind == domain.KindAsset,
					Bytes: m.Bytes})
			}
			top := op.Group[0]
			nodes := domain.GroupPreOrder(domain.ShapeNode{ID: top.Node.ID, Asset: top.Node.Kind == domain.KindAsset, Bytes: top.Bytes}, below)
			domain.GroupCounts(nodes)
			ids := make([]uuid.UUID, len(nodes))
			for i, n := range nodes {
				ids[i] = n.ID
			}
			g := len(p.groups)
			p.groups = append(p.groups, revertGroup{at: op.DeletedAt, nodes: nodes, first: len(p.steps)})
			for i, n := range nodes {
				s := revertStep{kind: op.Kind, node: n.ID, group: g, member: i}
				if i == 0 {
					s.nodes = ids
				}
				add(s, domain.JobStep{Nodes: 1, Bytes: int64(n.Bytes)})
			}
		case domain.RevertContent:
			add(revertStep{kind: op.Kind, node: op.NodeID, group: -1}, domain.JobStep{Nodes: 1, Bytes: int64(bases[op.NodeID])})
		}
	}
	return JobPlan{steps: steps, revert: p}
}

// RunBatch writes the plan's steps from to to (not included) in a unit
// of spec, merged into the job's changeset (M8/P4 design 3.3): before it,
// the rows of history they restore are counted, and past MaxUnitHistory
// for more than one step are ErrBatchTooLarge; the contents they write
// read and parsed, largest first, server_busy when the waits run out,
// ErrBatchTooLarge past the unit's links for more than one page; then the
// unit, which checks each step again under the lock (Unit.RestorePart,
// Unit.RevertBatch). It answers those sentinels and the unit's errors: a
// revert's trash rows the purge took, page.trash_changed in the unit, is
// page.changeset_trimmed.
func (w *JobWrites) RunBatch(ctx context.Context, spec JobSpec, plan JobPlan, from, to int) (BatchDone, error) {
	if from < 0 || from >= to || to > len(plan.steps) {
		return BatchDone{}, fmt.Errorf("a background job's batch of steps %d to %d of %d", from, to, len(plan.steps))
	}
	if plan.restore != nil {
		return w.restoreBatch(ctx, spec, plan.restore, from, to)
	}
	done, err := w.revertBatch(ctx, spec, plan.revert, from, to)
	if errors.Is(err, domain.ErrTrashChanged) {
		return BatchDone{}, domain.ErrChangesetTrimmed
	}
	return done, err
}

// restoreBatch restores the members of the plan's group from to to.
func (w *JobWrites) restoreBatch(ctx context.Context, spec JobSpec, p *restorePlan, from, to int) (BatchDone, error) {
	ids, pages := make([]uuid.UUID, 0, to-from), []uuid.UUID(nil)
	for _, n := range p.nodes[from:to] {
		ids = append(ids, n.ID)
		if !n.Asset {
			pages = append(pages, n.ID)
		}
	}
	if err := w.withinHistory(ctx, to-from, map[time.Time][]uuid.UUID{p.at: ids}); err != nil {
		return BatchDone{}, err
	}
	// A page someone restored meanwhile is not among them: the unit skips
	// it.
	contents, err := w.reads.Trash.TrashContents(ctx, pages, p.at)
	if err != nil {
		return BatchDone{}, err
	}
	facts, release, err := w.parser.DecidedAll(ctx, contents, domain.ErrBatchTooLarge)
	if err != nil {
		return BatchDone{}, err
	}
	defer release()
	var part PartRestored
	if _, err := w.writer.Run(ctx, restoreUnit(spec), func(ctx context.Context, u *Unit) error {
		var err error
		part, err = u.RestorePart(ctx, TrashPart{At: p.at, Nodes: p.nodes, From: from, To: to, Pages: p.pages, Assets: p.assets,
			Height: p.height, Facts: facts, Retention: w.reads.Retention})
		return err
	}); err != nil {
		return BatchDone{}, err
	}
	return BatchDone{Nodes: part.Pages + part.Assets, Pages: part.Pages, Assets: part.Assets, Renamed: part.Renamed,
		Skipped: part.Skipped, Placed: part.Placement}, nil
}

// withinHistory checks a batch of steps steps restores at most
// maxHistory rows of history, the nodes deleted at each time's: more than
// one step past it is ErrBatchTooLarge, before anything is parsed. A lone
// step is never past it: its page's deletion was one unit.
func (w *JobWrites) withinHistory(ctx context.Context, steps int, deleted map[time.Time][]uuid.UUID) error {
	if steps == 1 {
		return nil
	}
	rows := 0
	for at, ids := range deleted {
		n, err := w.reads.Trash.TrashHistory(ctx, ids, at, w.maxHistory+1)
		if err != nil {
			return err
		}
		if rows += n; rows > w.maxHistory {
			return domain.ErrBatchTooLarge
		}
	}
	return nil
}

// revertBatch writes the plan's steps from to to.
func (w *JobWrites) revertBatch(ctx context.Context, spec JobSpec, p *revertPlan, from, to int) (BatchDone, error) {
	steps := p.steps[from:to]
	deleted, trashed := map[time.Time][]uuid.UUID{}, map[time.Time][]uuid.UUID{}
	var written []uuid.UUID
	for _, s := range steps {
		switch s.kind {
		case domain.RevertRestore:
			g := p.groups[s.group]
			n := g.nodes[s.member]
			deleted[g.at] = append(deleted[g.at], n.ID)
			if !n.Asset {
				trashed[g.at] = append(trashed[g.at], n.ID)
			}
		case domain.RevertContent:
			written = append(written, s.node)
		}
	}
	if err := w.withinHistory(ctx, len(steps), deleted); err != nil {
		return BatchDone{}, err
	}
	contents, err := w.revertContents(ctx, p.changeset, trashed, written)
	if err != nil {
		return BatchDone{}, err
	}
	facts, release, err := w.parser.DecidedAll(ctx, contents, domain.ErrBatchTooLarge)
	if err != nil {
		return BatchDone{}, err
	}
	defer release()
	parsed := make(map[uuid.UUID]ParsedContent, len(contents))
	for page, content := range contents {
		sum := sha256.Sum256([]byte(content))
		parsed[page] = ParsedContent{Hash: sum[:], Content: content, Facts: facts[page]}
	}
	var done BatchDone
	if _, err := w.writer.Run(ctx, revertUnit(spec, p.changeset), func(ctx context.Context, u *Unit) error {
		var err error
		done, err = u.RevertBatch(ctx, RevertPart{plan: p, from: from, to: to, parsed: parsed, retention: w.reads.Retention})
		return err
	}); err != nil {
		return BatchDone{}, err
	}
	return done, nil
}

// revertContents are the contents a revert's batch restores or writes
// back, by page: those of the pages in the trash at each time (one
// someone restored meanwhile is not among them, which the unit tells),
// and the bases of the changeset's versions of the pages written, read by
// their changeset's versions; one the purge took is
// page.changeset_trimmed.
func (w *JobWrites) revertContents(ctx context.Context, reverted uuid.UUID, trashed map[time.Time][]uuid.UUID, written []uuid.UUID) (
	map[uuid.UUID]string, error,
) {
	out := map[uuid.UUID]string{}
	for at, pages := range trashed {
		contents, err := w.reads.Trash.TrashContents(ctx, pages, at)
		if err != nil {
			return nil, err
		}
		for page, content := range contents {
			out[page] = content
		}
	}
	if len(written) == 0 {
		return out, nil
	}
	versions, err := w.history.ChangesetVersionsOf(ctx, reverted, written)
	if err != nil {
		return nil, err
	}
	refs := make([]domain.VersionRef, 0, len(versions))
	for _, v := range versions {
		if v.Base != nil {
			refs = append(refs, domain.VersionRef{NodeID: v.NodeID, Revision: *v.Base})
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
		out[ref.NodeID] = v.Content
	}
	return out, nil
}
