package app

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
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
	// node restores: MaxUnitHistory.
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
// changeset, made by its first batch that writes; a background job's, its
// nodes queued for the search in bulk.
func restoreUnit(spec JobSpec) UnitSpec {
	return UnitSpec{NotebookID: spec.NotebookID, Action: domain.ActionRestore, Tree: true, Client: spec.Client,
		NotFound: domain.ErrNotebookNotFound, Kind: domain.ChangesetTrashRestore, Changeset: spec.Changeset, Create: true, Background: true}
}

// revertUnit is the unit of a revert's batch of spec, of the changeset
// reverted: the job's changeset, a revert of it with spec's message; a
// background job's, its nodes queued for the search in bulk.
func revertUnit(spec JobSpec, reverted uuid.UUID) UnitSpec {
	return UnitSpec{NotebookID: spec.NotebookID, Action: domain.ActionRevert, Tree: true, Client: spec.Client,
		NotFound: domain.ErrNotebookNotFound, Kind: domain.ChangesetRevert, RevertsID: reverted, Changeset: spec.Changeset, Create: true,
		Message: spec.Message, Background: true}
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
// it reverts, its schedule's steps as the job keeps them, and the groups
// its restores bring back.
type revertPlan struct {
	changeset uuid.UUID
	steps     []revertStep
	groups    []revertGroup
}

// revertStep is a step of a background revert as its job keeps it from
// the plan at its start (M9/P3 design 3.22.1): its kind and place in the
// schedule (seq); a place's or a step aside's node, and its parent (the
// zero id for the root), name and order there; a write back's page, base
// and hash, and its name in S* when the changeset's item of it has one
// (a batch reads its version, not its item: review B's M-1); a
// deletion's top and its subtree at the deletion's moment; a
// restore's member, its node, and its group, whose members
// revertPlan.groups keeps (-1 for none). A group's members share their
// restore's seq. A restore's time and states, and every item and version,
// are the batch's reads.
type revertStep struct {
	kind    domain.RevertOpKind
	seq     int32
	node    uuid.UUID
	parent  uuid.UUID
	name    string
	order   float64
	base    int32
	rewrite bool
	hash    []byte
	bytes   int
	nodes   []uuid.UUID
	group   int32
	member  int32
}

// groupIDs are the ids of the members of the plan's group g, in its
// pre-order.
func (p *revertPlan) groupIDs(g int32) []uuid.UUID {
	nodes := p.groups[g].nodes
	ids := make([]uuid.UUID, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}

// revertGroup is a trash item a background revert restores: its root,
// deleted at at as the plan read it, and its group in pre-order with its
// counts.
type revertGroup struct {
	at    time.Time
	nodes []domain.GroupNode
}

// PlanRevert plans the background revert of the changeset changesetID in
// spec's notebook, unlocked, as its job starts: the decision first, then
// the changeset, page.changeset_trimmed for one gone since, then the
// revert's check as CheckRevert has it (page.revert_conflict,
// page.changeset_trimmed, page.locked), its schedule's steps as the job
// keeps them, in segments no batch splits (revertSteps).
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
// changeset reverted, in the schedule's order (M9/P3 design 3.22): a place
// or a step aside a node each; a deletion its subtree at its moment; a
// restore its group in pre-order, a member a step, each page's write back
// right after its member's restore; a write back a page, right before the
// deletion that takes it if one does (review A-M1: a page a revert
// deletes is alive from the start to its deletion, and only its deletion
// waits for it). The steps no batch splits are joined (segments).
func revertSteps(reverted uuid.UUID, checked revertChecked) JobPlan {
	ops := checked.plan.Ops
	restored := map[uuid.UUID]bool{}
	taken := map[uuid.UUID]int{} // a page a deletion takes, by the deletion's place among the ops
	for i, op := range ops {
		switch op.Kind {
		case domain.RevertRestore:
			for _, m := range op.Group {
				restored[m.Node.ID] = true
			}
		case domain.RevertDelete:
			for _, id := range op.Subtree {
				taken[id] = i
			}
		}
	}
	after := map[uuid.UUID]domain.RevertOp{}
	before := map[int][]domain.RevertOp{} // the write backs right before a deletion, by its place
	moved := map[uuid.UUID]bool{}
	for i, op := range ops {
		if op.Kind != domain.RevertContent {
			continue
		}
		if restored[op.NodeID] {
			after[op.NodeID] = op
		} else if d, ok := taken[op.NodeID]; ok && d > i {
			before[d], moved[op.NodeID] = append(before[d], op), true
		}
	}
	names := map[uuid.UUID]string{} // a node's name in S*, its item's Before, as before() has it
	for _, it := range checked.state.Items {
		if it.Before != nil {
			names[it.NodeID] = it.Before.Name
		}
	}
	p := &revertPlan{changeset: reverted}
	var steps []domain.JobStep
	add := func(s revertStep, step domain.JobStep) {
		p.steps, steps = append(p.steps, s), append(steps, step)
	}
	write := func(op domain.RevertOp) {
		add(revertStep{kind: op.Kind, seq: int32(op.Seq), node: op.NodeID, name: names[op.NodeID], base: int32(op.Base), hash: op.Hash,
			bytes: op.Bytes, rewrite: op.Rewrite, group: -1}, domain.JobStep{Nodes: 1, Bytes: int64(op.Bytes)})
	}
	members := func(g, from, to int, seq int32) {
		for i := from; i < to; i++ {
			n := p.groups[g].nodes[i]
			add(revertStep{kind: domain.RevertRestore, seq: seq, node: n.ID, group: int32(g), member: int32(i)},
				domain.JobStep{Nodes: 1, Bytes: int64(n.Bytes)})
			if op, ok := after[n.ID]; ok {
				write(op)
			}
		}
	}
	// A restore within a segment of steps aside brings back the members
	// later places of the segment need; the rest come right after it.
	type rest struct {
		group, from int
		seq         int32
	}
	spans := asideSpans(ops)
	ends := map[int]int{}
	for _, sp := range spans {
		for i := sp[0]; i < sp[1]; i++ {
			ends[i] = sp[1]
		}
	}
	later := map[int][]rest{}
	starts := make(map[int]int, len(spans))
	for _, sp := range spans {
		starts[sp[1]] = sp[0]
	}
	first := make([]int, len(ops))
	var joins [][2]int
	for i, op := range ops {
		first[i] = len(p.steps)
		switch op.Kind {
		case domain.RevertPlace, domain.RevertAside:
			to := op.To
			add(revertStep{kind: op.Kind, seq: int32(op.Seq), node: op.NodeID, parent: parentOf(to.ParentID), name: to.Name,
				order: to.SortOrder, group: -1}, domain.JobStep{Nodes: 1})
		case domain.RevertDelete:
			for _, w := range before[i] {
				write(w)
			}
			add(revertStep{kind: op.Kind, seq: int32(op.Seq), node: op.NodeID, nodes: op.Subtree, group: -1},
				domain.JobStep{Nodes: len(op.Subtree)})
		case domain.RevertRestore:
			g := len(p.groups)
			p.groups = append(p.groups, revertGroup{at: op.DeletedAt, nodes: groupNodes(op.Group)})
			n := len(p.groups[g].nodes)
			if end, ok := ends[i]; ok {
				if n = needed(ops[i+1:end+1], p.groups[g].nodes); n < len(p.groups[g].nodes) {
					later[end] = append(later[end], rest{group: g, from: n, seq: int32(op.Seq)})
				}
			}
			members(g, 0, n, int32(op.Seq))
		case domain.RevertContent:
			if _, ok := after[op.NodeID]; !ok && !moved[op.NodeID] {
				write(op)
			}
		}
		if start, ok := starts[i]; ok {
			joins = append(joins, [2]int{first[start], len(p.steps) - 1})
		}
		for _, r := range later[i] {
			members(r.group, r.from, len(p.groups[r.group].nodes), r.seq)
		}
	}
	for _, j := range append(joins, pageJoins(p.steps)...) {
		for k := j[0]; k < j[1]; k++ {
			steps[k].Joined = true
		}
	}
	return JobPlan{steps: steps, revert: p}
}

// parentOf is the id of parent, the zero id for the root.
func parentOf(parent *uuid.UUID) uuid.UUID {
	if parent == nil {
		return uuid.UUID{}
	}
	return *parent
}

// groupNodes are the members of a group, as the plan read them, in its
// pre-order with their counts.
func groupNodes(group []domain.TrashMember) []domain.GroupNode {
	below := make([]domain.ShapeNode, 0, len(group)-1)
	for _, m := range group[1:] {
		below = append(below, domain.ShapeNode{ID: m.Node.ID, ParentID: *m.Node.ParentID, Asset: m.Node.Kind == domain.KindAsset,
			Bytes: m.Bytes})
	}
	top := group[0]
	nodes := domain.GroupPreOrder(domain.ShapeNode{ID: top.Node.ID, Asset: top.Node.Kind == domain.KindAsset, Bytes: top.Bytes}, below)
	domain.GroupCounts(nodes)
	return nodes
}

// asideSpans are the segments of a schedule's steps aside (M9/P3 design
// 3.22.2), by the ops' places in it: from each step aside to its node's
// last place, which follows it (T1), the steps between included, those
// that overlap merged.
func asideSpans(ops []domain.RevertOp) [][2]int {
	last := map[uuid.UUID]int{}
	for i, op := range ops {
		if op.Kind == domain.RevertPlace {
			last[op.NodeID] = i
		}
	}
	var out [][2]int
	for i, op := range ops {
		if op.Kind != domain.RevertAside {
			continue
		}
		end := max(i, last[op.NodeID])
		if n := len(out); n > 0 && i <= out[n-1][1] {
			out[n-1][1] = max(out[n-1][1], end)
		} else {
			out = append(out, [2]int{i, end})
		}
	}
	return out
}

// needed is how many of a group's members, in its pre-order, its restore
// within a segment of steps aside brings back there (M9/P3 design
// 3.22.2): up to the last one a later place of the segment puts a node
// under; the group's root at least. No other step needs one there: a
// member's write back follows its own restore, and only a deletion waits
// for a write back (review A-I1); a step aside's parent is alive, and a
// restore under a member of another group is a nested trash item, which
// L refuses (review A-N5, C-N1).
func needed(later []domain.RevertOp, nodes []domain.GroupNode) int {
	at := make(map[uuid.UUID]int, len(nodes))
	for i, n := range nodes {
		at[n.ID] = i
	}
	n := 1
	for _, op := range later {
		if op.Kind != domain.RevertPlace || op.To.ParentID == nil {
			continue
		}
		if i, ok := at[*op.To.ParentID]; ok {
			n = max(n, i+1)
		}
	}
	return n
}

// pageJoins are the segments of a page's steps (M9/P3 design 3.22.2), by
// their places among the steps: its restore and its write back right
// after; its write back and the deletion that takes it, the write backs
// right before that deletion between them.
func pageJoins(steps []revertStep) [][2]int {
	var out [][2]int
	written := map[uuid.UUID]int{}
	for k, s := range steps {
		switch s.kind {
		case domain.RevertContent:
			if k > 0 && steps[k-1].kind == domain.RevertRestore && steps[k-1].node == s.node {
				out = append(out, [2]int{k - 1, k})
			}
			written[s.node] = k
		case domain.RevertDelete:
			for _, id := range s.nodes {
				if j, ok := written[id]; ok {
					out = append(out, [2]int{j, k})
				}
			}
		}
	}
	return out
}

// RunBatch writes the plan's steps from to to (not included) in a unit
// of spec, merged into the job's changeset (M8/P4 design 3.3): before it,
// a revert's nodes it restores are counted, past MaxUnitNodes for steps
// of more than one node ErrJobBatchTooLarge, and the pages it restores and
// writes back by the plan's sizes, past the unit's pages, copies or bytes
// for more than one page; the rows of history they restore, past
// MaxUnitHistory; the contents they write read, past the unit's pages,
// copies or bytes as they are, then parsed,
// largest first, server_busy when the waits run out, past the unit's
// links; then the unit, which checks each step again under the lock
// (Unit.RestorePart, Unit.RevertBatch). A node's steps are never past the
// bounds (M9/P3 design 3.23): its restore's history is its page's, its
// write back's links its content's, as M8's lone step's were. It answers
// those sentinels, each saying which bound, and the unit's errors: a
// revert's trash rows the purge took, page.trash_changed in the unit, is
// page.changeset_trimmed.
func (w *JobWrites) RunBatch(ctx context.Context, spec JobSpec, plan JobPlan, from, to int) (BatchDone, error) {
	if from < 0 || from >= to || to > len(plan.steps) {
		return BatchDone{}, fmt.Errorf("a background job's batch of steps %d to %d of %d", from, to, len(plan.steps))
	}
	if plan.restore != nil {
		return w.restoreBatch(ctx, spec, plan.restore, from, to)
	}
	done, err := w.revertBatch(ctx, spec, plan.revert, plan.steps[from:to], from, to)
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
	if err := w.withinHistory(ctx, len(ids), map[time.Time][]uuid.UUID{p.at: ids}); err != nil {
		return BatchDone{}, err
	}
	// A page someone restored meanwhile is not among them: the unit skips
	// it.
	contents, err := w.reads.Trash.TrashContents(ctx, pages, p.at)
	if err != nil {
		return BatchDone{}, err
	}
	facts, release, err := w.parser.DecidedAll(ctx, contents, errPastLinks)
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

// The bounds a background job's batch is past, ErrJobBatchTooLarge, each
// saying which (M9/P3 design 3.23): the nodes a revert's batch restores,
// MaxUnitNodes (review B-S1, C-S2: a cut restore may bring back thousands
// of attachments); the rows of history it restores; the pages of the
// contents it reads, their copies or their bytes (UnitTooLarge; review
// A-M2); the links of the contents it parses.
var (
	errPastNodes   = fmt.Errorf("%w: past the nodes a unit restores", domain.ErrJobBatchTooLarge)
	errPastHistory = fmt.Errorf("%w: past a unit's rows of history", domain.ErrJobBatchTooLarge)
	errPastPages   = fmt.Errorf("%w: past a unit's pages or their bytes", domain.ErrJobBatchTooLarge)
	errPastLinks   = fmt.Errorf("%w: past a unit's links", domain.ErrJobBatchTooLarge)
)

// withinHistory checks a batch of steps that write nodes nodes restores at
// most maxHistory rows of history, the nodes deleted at each time's: of
// more than one node, past it is errPastHistory, before anything is
// parsed. A lone node is never past it: its page's deletion was one unit,
// and no batch is smaller.
func (w *JobWrites) withinHistory(ctx context.Context, nodes int, deleted map[time.Time][]uuid.UUID) error {
	if nodes == 1 {
		return nil
	}
	rows := 0
	for at, ids := range deleted {
		n, err := w.reads.Trash.TrashHistory(ctx, ids, at, w.maxHistory+1)
		if err != nil {
			return err
		}
		if rows += n; rows > w.maxHistory {
			return errPastHistory
		}
	}
	return nil
}

// revertBatch writes the plan's steps from to to, sizes their sizes. A
// group the batch begins is read at the time its root is deleted at now,
// which its unit plans it at: deleted again since the plan, its pages'
// contents and history are there (M8/P4A review A-M1); a group earlier
// batches began, at the time they restored it at.
func (w *JobWrites) revertBatch(ctx context.Context, spec JobSpec, p *revertPlan, sizes []domain.JobStep, from, to int) (BatchDone, error) {
	steps := p.steps[from:to]
	// The bounds the plan's steps tell, before anything is read (review
	// B's N-1, C1-N2): the nodes it restores, a restore step each, and its
	// pages, copies and bytes.
	nodes, restoring := map[uuid.UUID]bool{}, 0
	for _, s := range steps {
		nodes[s.node] = true
		for _, id := range s.nodes {
			nodes[id] = true
		}
		if s.kind == domain.RevertRestore {
			restoring++
		}
	}
	if len(nodes) > 1 && restoring > domain.MaxUnitNodes {
		return BatchDone{}, errPastNodes
	}
	if err := withinSizes(p, steps, sizes); err != nil {
		return BatchDone{}, err
	}
	begun, err := w.begun(ctx, p, steps)
	if err != nil {
		return BatchDone{}, err
	}
	deleted, trashed := map[time.Time][]uuid.UUID{}, map[time.Time][]uuid.UUID{}
	var written []uuid.UUID
	for _, s := range steps {
		switch s.kind {
		case domain.RevertRestore:
			g := p.groups[s.group]
			at, ok := begun[s.group]
			if !ok {
				at = g.at
			}
			n := g.nodes[s.member]
			deleted[at] = append(deleted[at], n.ID)
			if !n.Asset {
				trashed[at] = append(trashed[at], n.ID)
			}
		case domain.RevertContent:
			written = append(written, s.node)
		}
	}
	if err := w.withinHistory(ctx, len(nodes), deleted); err != nil {
		return BatchDone{}, err
	}
	contents, err := w.revertContents(ctx, p.changeset, trashed, written, trashBound(p, steps, sizes))
	if err != nil {
		return BatchDone{}, err
	}
	if err := withinPages(contents); err != nil {
		return BatchDone{}, err
	}
	facts, release, err := w.parser.DecidedContents(ctx, contents, errPastLinks)
	if err != nil {
		return BatchDone{}, err
	}
	defer release()
	parsed := parsedContents(contents, facts)
	var done BatchDone
	var retimed map[int32]time.Time
	if _, err := w.writer.Run(ctx, revertUnit(spec, p.changeset), func(ctx context.Context, u *Unit) error {
		var err error
		retimed = map[int32]time.Time{}
		done, err = u.RevertBatch(ctx, RevertPart{plan: p, from: from, to: to, parsed: parsed, retention: w.reads.Retention,
			retimed: retimed})
		return err
	}); err != nil {
		return BatchDone{}, err
	}
	for i, at := range retimed {
		p.groups[i].at = at
	}
	return done, nil
}

// begun are the times the roots of the groups the steps begin are
// deleted at now, by the group's index, read at once: the plan's for a
// root not deleted, or not there, which the batch's unit tells.
func (w *JobWrites) begun(ctx context.Context, p *revertPlan, steps []revertStep) (map[int32]time.Time, error) {
	var roots []uuid.UUID
	for _, s := range steps {
		if s.kind == domain.RevertRestore && s.member == 0 {
			roots = append(roots, s.node)
		}
	}
	out := make(map[int32]time.Time, len(roots))
	if len(roots) == 0 {
		return out, nil
	}
	now, err := w.history.NodesNow(ctx, roots, uuid.UUID{})
	if err != nil {
		return nil, err
	}
	for _, s := range steps {
		if s.kind == domain.RevertRestore && s.member == 0 {
			out[s.group] = p.groups[s.group].at
			if n, ok := now[s.node]; ok && n.DeletedAt != nil {
				out[s.group] = *n.DeletedAt
			}
		}
	}
	return out, nil
}

// withinSizes checks a revert's batch reads at most a unit's pages, copies
// and bytes by the sizes its plan measured, before it reads a content
// (review A1-I1, B's I-1), as a request's revert does (withinBounds): the
// pages it restores and writes back, a content each, a page restored and
// written back two, each of the size its step has (JobStep.Bytes: a
// member's in the trash, a write back's base); a lone page's never past
// them. Else errPastPages. A first segment is in its batch whole, past
// Batch.Bytes, and a cut restore may hold thousands of pages: read, they
// would all be in memory to be refused. withinPages checks the contents
// once read, as they are then.
func withinSizes(p *revertPlan, steps []revertStep, sizes []domain.JobStep) error {
	pages, copies, bytes := map[uuid.UUID]bool{}, 0, int64(0)
	for i, s := range steps {
		if s.kind == domain.RevertContent || s.kind == domain.RevertRestore && !p.groups[s.group].nodes[s.member].Asset {
			pages[s.node], copies, bytes = true, copies+1, bytes+sizes[i].Bytes
		}
	}
	if domain.UnitTooLarge(len(pages), copies, int(bytes), 0) {
		return errPastPages
	}
	return nil
}

// trashBound is the most bytes a revert's batch may read of the trash's
// contents, in their statements (M9 Codex review D2): a unit's, less the
// bytes of the bases it writes back, whose versions never change, for the
// contents of more than one page; any for a lone page's. A group a batch
// begins is read at the time its root is deleted at now: deleted again
// since the plan, its contents may be larger than the plan sized them, and
// the statement reads none past the bound.
func trashBound(p *revertPlan, steps []revertStep, sizes []domain.JobStep) int64 {
	pages, bases := map[uuid.UUID]bool{}, int64(0)
	for i, s := range steps {
		switch {
		case s.kind == domain.RevertContent:
			pages[s.node], bases = true, bases+sizes[i].Bytes
		case s.kind == domain.RevertRestore && !p.groups[s.group].nodes[s.member].Asset:
			pages[s.node] = true
		}
	}
	if len(pages) <= 1 {
		return math.MaxInt64
	}
	return domain.MaxUnitBytes - bases
}

// withinPages checks the contents a revert's batch reads, by page and
// hash, are of at most a unit's pages, copies and bytes, unless they are a
// lone page's: else errPastPages, before they are parsed.
func withinPages(contents map[domain.ContentRef]string) error {
	pages, bytes := map[uuid.UUID]bool{}, 0
	for ref, content := range contents {
		pages[ref.NodeID], bytes = true, bytes+len(content)
	}
	if domain.UnitTooLarge(len(pages), len(contents), bytes, 0) {
		return errPastPages
	}
	return nil
}

// revertContents are the contents a revert's batch restores or writes
// back, by page and hash: those of the pages in the trash at each time
// (one someone restored meanwhile is not among them, which the unit
// tells), bound bytes at most in all, else errPastPages, the content past
// it not read (trashBound); and the bases of the changeset's versions of
// the pages written, read by their changeset's versions; one the purge
// took is page.changeset_trimmed.
func (w *JobWrites) revertContents(ctx context.Context, reverted uuid.UUID, trashed map[time.Time][]uuid.UUID, written []uuid.UUID,
	bound int64,
) (map[domain.ContentRef]string, error) {
	out := map[domain.ContentRef]string{}
	for at, pages := range trashed {
		got, err := w.reads.Trash.TrashContentsWithin(ctx, pages, at, bound)
		switch {
		case err != nil:
			return nil, err
		case got.Past != uuid.UUID{}:
			return nil, errPastPages
		}
		for page, content := range got.Contents {
			out[contentOf(page, content)] = content
			bound -= int64(len(content))
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
		out[contentOf(ref.NodeID, v.Content)] = v.Content
	}
	return out, nil
}
