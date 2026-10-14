package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Reverting a changeset in a unit (M8/P3 design 3.6).

// ParsedContent is a content a revert writes back or restores, parsed
// before its unit: its hash, its text and its facts.
type ParsedContent struct {
	Hash    []byte
	Content string
	Facts   Facts
}

// RevertRead is what a revert read before its unit, unlocked: the
// changeset it reverts, the contents it writes back or restores parsed,
// by page, the purge's retention, and the bounds of the changeset's items
// and of its round's steps (domain.RevertPlan.Steps).
type RevertRead struct {
	Changeset uuid.UUID
	Parsed    map[uuid.UUID]ParsedContent
	Retention time.Duration
	MaxItems  int
	MaxSteps  int
}

// RevertDone is what a revert's unit did: how many nodes it put back,
// subtrees it deleted, trash items it restored and pages it wrote back.
type RevertDone struct {
	Placed, Deleted, Restored, Written int
}

// Revert reverts the changeset r names, under the notebook's lock: it is
// read again, its items and versions, then its row, page.changeset_trimmed
// for one the purge marked or took meanwhile (the lock keeps its notebook), page.revert_too_large past
// the bound of its items, merged into meanwhile; then the state, planned
// at the unit's time, the plan this one, which is authoritative:
// page.revert_conflict, page.changeset_trimmed for a version the purge
// took, page.revert_too_large past the bound of its round's steps. Each content it writes back or restores must have been parsed at
// the hash it writes now, else its page is changed. Then its round's
// steps, each a unit's operation through the guards (page.locked for a
// page under another's lock, or the caller's elsewhere), its write, the
// changeset and the participants: a place moves the node, its subtree
// with it, then renames it; a deletion deletes a subtree; a restore
// brings a trash item back where the changeset deleted it, at its time;
// a content writes the version back on the revision the unit read. Their
// checks are the plan's: a name taken or a revision moved in its write is
// a defect, unclassified, as the purge's take of what a restore brings
// back is page.trash_changed, which the use case answers.
func (u *Unit) Revert(ctx context.Context, r RevertRead) (RevertDone, error) {
	reads := revertReads{history: u.w.d.History, trash: u.w.d.Trash, nodes: u.w.d.Nodes}
	items, versions, err := reads.changeset(ctx, r.Changeset)
	switch {
	case errors.Is(err, ErrNotFound):
		// Under the notebook's lock, its deletion waits: only the purge,
		// which took what the changeset held, deletes the changeset.
		return RevertDone{}, domain.ErrChangesetTrimmed
	case err != nil:
		return RevertDone{}, err
	case len(items) > r.MaxItems:
		return RevertDone{}, domain.ErrRevertTooLarge
	}
	state, err := reads.state(ctx, u.write.NotebookID, r.Changeset, items, versions, u.write.At, r.Retention)
	if err != nil {
		return RevertDone{}, err
	}
	plan, err := planned(state)
	switch {
	case err != nil:
		return RevertDone{}, err
	case plan.Steps() > r.MaxSteps:
		return RevertDone{}, domain.ErrRevertTooLarge
	}
	if err := parsedAsPlanned(plan, state, r.Parsed); err != nil {
		return RevertDone{}, err
	}
	var done RevertDone
	for _, op := range plan.Ops {
		switch op.Kind {
		case domain.RevertPlace:
			err = u.place(ctx, op)
			done.Placed++
		case domain.RevertDelete:
			_, err = u.Delete(ctx, op.NodeID)
			done.Deleted++
		case domain.RevertRestore:
			keys, facts := make([]string, len(op.Group)), map[uuid.UUID]Facts{}
			for i, m := range op.Group {
				keys[i], facts[m.Node.ID] = shared.TitleKey(op.States[i].Name), r.Parsed[m.Node.ID].Facts
			}
			err = u.bringBack(ctx, op.Group, op.DeletedAt, op.States, keys, facts, func(context.Context) error { return nil })
			done.Restored++
		case domain.RevertContent:
			parsed := r.Parsed[op.NodeID]
			_, err = u.WriteContent(ctx, ContentWrite{NodeID: op.NodeID, Content: parsed.Content, Facts: parsed.Facts,
				Base: state.Contents[op.NodeID].Revision})
			if errors.Is(err, domain.ErrRevisionMismatch) {
				err = unclassified(fmt.Sprintf("a revert's write back of page %s", op.NodeID), err)
			}
			done.Written++
		}
		if err != nil {
			return RevertDone{}, err
		}
	}
	return done, nil
}

// place puts a node back where a revert's plan says, its checks the
// plan's: a move with the name it has, its subtree with it, at the order
// it had, then a rename, keyed by today's rules. A title taken in either
// write is the plan's defect.
func (u *Unit) place(ctx context.Context, op domain.RevertOp) error {
	n, err := u.w.d.Nodes.FindNodeIn(ctx, u.write.NotebookID, op.NodeID)
	if err != nil {
		return err
	}
	if op.Move {
		sub, err := u.w.d.Nodes.Subtree(ctx, u.write.NotebookID, n.ID)
		if err != nil {
			return err
		}
		n.ParentID, n.SortOrder, n.UpdatedBy, n.UpdatedAt = op.To.ParentID, op.To.SortOrder, u.write.By, u.write.At
		if err := u.moveSubtree(ctx, sub, n, func(context.Context) error { return nil }); err != nil {
			return taken(op.NodeID, err)
		}
	}
	if op.Rename {
		renamed := n
		renamed.Name, renamed.NameKey, renamed.UpdatedBy, renamed.UpdatedAt = op.To.Name, shared.TitleKey(op.To.Name), u.write.By, u.write.At
		before, after := n.State(), renamed.State()
		err := u.apply(ctx, u.step(domain.OpRename, domain.Change{NodeID: n.ID, Before: &before, After: &after}), true,
			func(ctx context.Context) error { return u.w.d.NodeWriter.RenameNode(ctx, renamed) })
		if err != nil {
			return taken(op.NodeID, err)
		}
	}
	return nil
}

// taken is err of a revert's place of the node id, unclassified when it
// is a title taken: the plan found the title free, so its write meeting
// a sibling's is a defect, no conflict of the revert's contract (M8/P3
// design 3.6, as RestoreNodes' at M8/P2A review B-4).
func taken(id uuid.UUID, err error) error {
	if errors.Is(err, domain.ErrTitleTaken) {
		return unclassified(fmt.Sprintf("a revert's place of node %s", id), err)
	}
	return err
}

// unclassified is err as a defect of what: its text, not the error, so
// that no problem of another operation's contract leaks through it.
func unclassified(what string, err error) error {
	return fmt.Errorf("%s: %s", what, err.Error())
}

// planned is the plan of state, which must have no conflict and no
// version the purge took: page.revert_conflict, page.changeset_trimmed.
func planned(state domain.RevertState) (domain.RevertPlan, error) {
	plan, err := domain.PlanRevert(state)
	switch {
	case err != nil:
		return domain.RevertPlan{}, err
	case plan.Count > 0:
		return domain.RevertPlan{}, domain.RevertConflicts(plan.Conflicts, plan.Count)
	case len(plan.Missing) > 0:
		return domain.RevertPlan{}, domain.ErrChangesetTrimmed
	}
	return plan, nil
}

// parsedAsPlanned checks each content the plan writes back or restores
// was parsed before the unit at the hash it writes now: a version
// written back, a page's content in the trash. One that was not, or
// differs, is a page changed between the two reads, a conflict, listed
// by node id; one of the same bytes at another revision is none, its
// revision the unit's.
func parsedAsPlanned(plan domain.RevertPlan, state domain.RevertState, parsed map[uuid.UUID]ParsedContent) error {
	return parsedAs(plannedContents(plan, state, nil), parsed)
}

// writtenContent is a content a revert writes: its page, the page's name
// and the hash it writes.
type writtenContent struct {
	id   uuid.UUID
	name string
	hash []byte
}

// plannedContents are the contents the plan writes back or restores, of
// the pages within keeps, all of them when it is nil.
func plannedContents(plan domain.RevertPlan, state domain.RevertState, within func(uuid.UUID) bool) []writtenContent {
	var out []writtenContent
	for _, op := range plan.Ops {
		switch op.Kind {
		case domain.RevertRestore:
			for i, m := range op.Group {
				if m.Node.Kind == domain.KindPage && (within == nil || within(m.Node.ID)) {
					out = append(out, writtenContent{id: m.Node.ID, name: op.States[i].Name,
						hash: state.Hashes[domain.VersionRef{NodeID: m.Node.ID, Revision: m.Revision}]})
				}
			}
		case domain.RevertContent:
			out = append(out, writtenContent{id: op.NodeID, name: state.Nodes[op.NodeID].Name, hash: op.Hash})
		}
	}
	return out
}

// parsedAs checks each content was parsed at the hash it writes: one that
// was not, or differs, is changed, all of them listed in their order, by
// node id, as the plan's.
func parsedAs(contents []writtenContent, parsed map[uuid.UUID]ParsedContent) error {
	var changed []domain.RevertConflict
	for _, c := range contents {
		if p, ok := parsed[c.id]; !ok || !bytes.Equal(p.Hash, c.hash) {
			changed = append(changed, domain.RevertConflict{NodeID: c.id, Name: c.name, Reason: domain.ConflictChanged})
		}
	}
	if len(changed) > 0 {
		slices.SortFunc(changed, func(a, b domain.RevertConflict) int { return a.NodeID.Compare(b.NodeID) })
		return domain.RevertConflicts(changed[:min(len(changed), domain.MaxConflicts)], len(changed))
	}
	return nil
}

// revertReads are what a revert reads of its changeset and its notebook,
// unlocked before its unit and again in it (M8/P3 design 3.6).
type revertReads struct {
	history History
	trash   Trash
	nodes   Nodes
}

// changeset reads the changeset id's items and versions, then its row:
// page.changeset_trimmed when the purge marked it meanwhile, ErrNotFound
// when the row is gone, which its caller tells (the purge took it, or its
// notebook was deleted). The purge marks a changeset as it takes its
// rows: read after them, an unmarked one had none taken before (M8/P3
// design 3.3).
func (r revertReads) changeset(ctx context.Context, id uuid.UUID) ([]domain.ChangesetItem, []domain.ChangesetVersion, error) {
	items, err := r.history.ChangesetItems(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	versions, err := r.history.ChangesetVersions(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	c, err := r.history.FindChangeset(ctx, id)
	switch {
	case err != nil:
		return nil, nil, err
	case c.Trimmed:
		return nil, nil, domain.ErrChangesetTrimmed
	}
	return items, versions, nil
}

// groups reads the nodes the changeset reverted changed as they are now,
// each root's deletion against that changeset (History.NodesNow), and the
// groups of its deletions' trash items whose roots pass the check
// (domain.RevertGroups), each at its time, all a level at a time (M8/P4
// design 3.3): one gone since is left out.
func (r revertReads) groups(ctx context.Context, notebookID, reverted uuid.UUID, items []domain.ChangesetItem,
	versions []domain.ChangesetVersion,
) (map[uuid.UUID]domain.NodeNow, map[uuid.UUID][]domain.TrashMember, error) {
	nodes, err := r.nodesNow(ctx, reverted, items, versions)
	if err != nil {
		return nil, nil, err
	}
	groups, err := r.trashGroups(ctx, notebookID, domain.RevertGroups(items, nodes))
	if err != nil {
		return nil, nil, err
	}
	return nodes, groups, nil
}

// nodesNow reads the nodes of the items and versions of the changeset
// reverted as they are now.
func (r revertReads) nodesNow(ctx context.Context, reverted uuid.UUID, items []domain.ChangesetItem, versions []domain.ChangesetVersion) (
	map[uuid.UUID]domain.NodeNow, error,
) {
	ids := make([]uuid.UUID, 0, len(items)+len(versions))
	for _, it := range items {
		ids = append(ids, it.NodeID)
	}
	for _, v := range versions {
		ids = append(ids, v.NodeID)
	}
	return r.history.NodesNow(ctx, ids, reverted)
}

// trashGroups reads the groups of the roots, each at its time: none when
// there is no root.
func (r revertReads) trashGroups(ctx context.Context, notebookID uuid.UUID, roots map[uuid.UUID]time.Time) (
	map[uuid.UUID][]domain.TrashMember, error,
) {
	if len(roots) == 0 {
		return map[uuid.UUID][]domain.TrashMember{}, nil
	}
	return r.trash.TrashGroups(ctx, notebookID, roots)
}

// rest reads the rest of the state: the contents of the changeset's pages
// alive, the hashes of the versions the plan compares, and, when the
// changeset has items, the notebook's tree.
func (r revertReads) rest(ctx context.Context, notebookID uuid.UUID, s domain.RevertState) (domain.RevertState, error) {
	ids := make([]uuid.UUID, 0, len(s.Nodes))
	for id, n := range s.Nodes {
		if n.Kind == domain.KindPage && n.DeletedAt == nil {
			ids = append(ids, id)
		}
	}
	var err error
	if s.Contents, err = r.history.ContentsNow(ctx, ids); err != nil {
		return domain.RevertState{}, err
	}
	if s.Hashes, err = r.history.VersionHashes(ctx, domain.RevertRefs(s.Items, s.Versions, s.Groups)); err != nil {
		return domain.RevertState{}, err
	}
	if len(s.Items) > 0 {
		if s.Tree, err = r.nodes.ListNodes(ctx, notebookID); err != nil {
			return domain.RevertState{}, err
		}
	}
	return s, nil
}

// state reads the whole state of the revert of the changeset reverted in
// the notebook, to be judged at at under retention.
func (r revertReads) state(ctx context.Context, notebookID, reverted uuid.UUID, items []domain.ChangesetItem,
	versions []domain.ChangesetVersion, at time.Time, retention time.Duration,
) (domain.RevertState, error) {
	nodes, groups, err := r.groups(ctx, notebookID, reverted, items, versions)
	if err != nil {
		return domain.RevertState{}, err
	}
	return r.rest(ctx, notebookID, domain.RevertState{Items: items, Versions: versions, Nodes: nodes, Groups: groups, At: at, Retention: retention})
}

// RevertPart is a background revert's batch (M8/P4 design 3.5): the steps
// from to to (not included) of the job's plan; the contents they restore
// or write back, parsed before the unit, by page; the purge's retention.
type RevertPart struct {
	plan      *revertPlan
	from, to  int
	parsed    map[uuid.UUID]ParsedContent
	retention time.Duration
}

// stepKey is a step of a revert's round by its kind and node: a node may
// be put back and written back.
type stepKey struct {
	kind domain.RevertOpKind
	node uuid.UUID
}

// RevertBatch reverts a batch of the changeset the plan reverts, under the
// notebook's lock (M8/P4 design 3.5). The changeset's items and versions
// of the batch's nodes are read, then its row: page.changeset_trimmed for
// one the purge marked or took. The state of those alone is planned again
// at the unit's time, S* being what the batches before it left with this
// batch's steps applied: page.revert_conflict, page.changeset_trimmed for
// a version the purge took; a node another changeset put under a top it
// deletes is children_added, its item none of the batch's. The plan's
// steps must be the batch's. A group whose first step an earlier batch
// took is restored a member at a time (its continuation), each checked
// alone, kept out of the plan: one no longer deleted at the group's time
// that is alive where the changeset deleted it from, someone's restore,
// is skipped; else changed, as is one still deleted whose parent the
// batch does not restore but no trash item of its counts; then expired,
// parent_gone, title_taken with the node holding its name (no number:
// it goes back where it was or not at all), too_deep. Each content it
// writes must have been parsed at the hash it writes now. Then its steps,
// in their order, as Revert has them; a group's part of the batch is
// restored as it was deleted, the nodes under it later batches restore
// made trash items of their own. A classified error but the guards'
// page.locked and the restore's page.trash_changed is a defect.
func (u *Unit) RevertBatch(ctx context.Context, r RevertPart) (BatchDone, error) {
	steps := r.plan.steps[r.from:r.to]
	var items, versions, conts []uuid.UUID
	var contSteps []revertStep
	opened := map[int]bool{}
	for _, s := range steps {
		switch {
		case s.kind == domain.RevertPlace:
			items = append(items, s.node)
		case s.kind == domain.RevertContent:
			versions = append(versions, s.node)
		case s.kind == domain.RevertRestore && s.member > 0:
			if !opened[s.group] {
				conts, contSteps = append(conts, s.node), append(contSteps, s)
			}
		default:
			items, versions = append(items, s.nodes...), append(versions, s.nodes...)
			if s.kind == domain.RevertRestore {
				opened[s.group] = true
			}
		}
	}
	planned, befores, plans, err := u.batchChangeset(ctx, r.plan.changeset, items, versions, conts)
	if err != nil {
		return BatchDone{}, err
	}
	plan, state, err := u.replan(ctx, r, planned, plans)
	if err != nil {
		return BatchDone{}, err
	}
	ops, err := batchOps(steps, plan)
	if err != nil {
		return BatchDone{}, err
	}
	restoring, err := u.continuation(ctx, r, contSteps, befores)
	if err != nil {
		return BatchDone{}, err
	}
	// Of a group the batch begins, its members in the batch alone.
	restored := map[uuid.UUID]bool{}
	for _, s := range steps {
		if s.kind == domain.RevertRestore {
			restored[s.node] = true
		}
	}
	written := plannedContents(plan, state, func(id uuid.UUID) bool { return restored[id] })
	if err := parsedAs(append(written, restoring.contents...), r.parsed); err != nil {
		return BatchDone{}, err
	}
	var done BatchDone
	for at := 0; at < len(steps); {
		s := steps[at]
		var err error
		switch s.kind {
		case domain.RevertPlace:
			err = u.place(ctx, ops[stepKey{s.kind, s.node}])
			done.PutBack++
		case domain.RevertDelete:
			var sub domain.Subtree
			sub, err = u.Delete(ctx, s.node)
			done.Deleted += len(sub)
		case domain.RevertRestore:
			end := at + 1
			for end < len(steps) && steps[end].kind == domain.RevertRestore && steps[end].group == s.group {
				end++
			}
			var n, skipped int
			n, skipped, err = u.restoreSegment(ctx, r, steps[at:end], ops, restoring)
			done.Restored, done.Skipped = done.Restored+n, done.Skipped+skipped
			at = end - 1
		case domain.RevertContent:
			op, parsed := ops[stepKey{s.kind, s.node}], r.parsed[s.node]
			_, err = u.WriteContent(ctx, ContentWrite{NodeID: s.node, Content: parsed.Content, Facts: parsed.Facts,
				Base: state.Contents[op.NodeID].Revision})
			done.Written++
		}
		if err != nil {
			return BatchDone{}, revertDefect(s, err)
		}
		at++
	}
	done.Nodes = done.PutBack + done.Deleted + done.Restored + done.Written
	return done, nil
}

// batchChangeset reads the changeset's items of the nodes items and of
// the continuation's conts, and its versions of the pages versions, then
// its row: page.changeset_trimmed for one the purge marked or took (the
// lock keeps its notebook). It answers the items and versions planned
// again, the continuation's items by node, and whether the batch plans
// at all.
func (u *Unit) batchChangeset(ctx context.Context, id uuid.UUID, items, versions, conts []uuid.UUID) (
	domain.RevertState, map[uuid.UUID]domain.ChangesetItem, bool, error,
) {
	var state domain.RevertState
	befores := map[uuid.UUID]domain.ChangesetItem{}
	if all := append(slices.Clone(items), conts...); len(all) > 0 {
		read, err := u.w.d.History.ChangesetItemsOf(ctx, id, all)
		if err != nil {
			return domain.RevertState{}, nil, false, err
		}
		cont := make(map[uuid.UUID]bool, len(conts))
		for _, c := range conts {
			cont[c] = true
		}
		for _, it := range read {
			if cont[it.NodeID] {
				befores[it.NodeID] = it
			} else {
				state.Items = append(state.Items, it)
			}
		}
	}
	if len(versions) > 0 {
		var err error
		if state.Versions, err = u.w.d.History.ChangesetVersionsOf(ctx, id, versions); err != nil {
			return domain.RevertState{}, nil, false, err
		}
	}
	c, err := u.w.d.History.FindChangeset(ctx, id)
	switch {
	case errors.Is(err, ErrNotFound):
		return domain.RevertState{}, nil, false, domain.ErrChangesetTrimmed
	case err != nil:
		return domain.RevertState{}, nil, false, err
	case c.Trimmed:
		return domain.RevertState{}, nil, false, domain.ErrChangesetTrimmed
	}
	return state, befores, len(items)+len(versions) > 0, nil
}

// replan plans the batch's items and versions again at the unit's time,
// none when it has neither: a batch of a continuation alone.
func (u *Unit) replan(ctx context.Context, r RevertPart, s domain.RevertState, plans bool) (domain.RevertPlan, domain.RevertState,
	error,
) {
	if !plans {
		return domain.RevertPlan{}, domain.RevertState{}, nil
	}
	reads := revertReads{history: u.w.d.History, trash: u.w.d.Trash, nodes: u.w.d.Nodes}
	state, err := reads.state(ctx, u.write.NotebookID, r.plan.changeset, s.Items, s.Versions, u.write.At, r.retention)
	if err != nil {
		return domain.RevertPlan{}, domain.RevertState{}, err
	}
	plan, err := planned(state)
	if err != nil {
		return domain.RevertPlan{}, domain.RevertState{}, err
	}
	return plan, state, nil
}

// batchOps are the plan's steps by kind and node, which must be the
// batch's: but for a group's later members, one a step each.
func batchOps(steps []revertStep, plan domain.RevertPlan) (map[stepKey]domain.RevertOp, error) {
	ops := make(map[stepKey]domain.RevertOp, len(plan.Ops))
	for _, op := range plan.Ops {
		ops[stepKey{op.Kind, op.NodeID}] = op
	}
	n := 0
	for _, s := range steps {
		if s.kind == domain.RevertRestore && s.member > 0 {
			continue
		}
		if _, ok := ops[stepKey{s.kind, s.node}]; !ok {
			return nil, fmt.Errorf("a background revert's batch plans no %s of node %s, its step", s.kind, s.node)
		}
		n++
	}
	if n != len(ops) {
		return nil, fmt.Errorf("a background revert's batch of %d steps plans %d", n, len(ops))
	}
	return ops, nil
}

// restoring is what a batch's continuation restores: its members, by
// node, each as it is in the trash and the state it goes back to; those
// it skips; and the contents of the pages it restores, as they hash in
// the trash.
type restoring struct {
	members  map[uuid.UUID]domain.TrashMember
	states   map[uuid.UUID]domain.TreeState
	skipped  map[uuid.UUID]bool
	contents []writtenContent
}

// continuation checks the batch's members of the groups whose first steps
// earlier batches took, conts, one by one, as RevertBatch has it, by the
// changeset's items of them, befores, and answers what it restores; every
// conflict it finds, in their order.
func (u *Unit) continuation(ctx context.Context, r RevertPart, conts []revertStep, befores map[uuid.UUID]domain.ChangesetItem) (
	restoring, error,
) {
	out := restoring{members: map[uuid.UUID]domain.TrashMember{}, states: map[uuid.UUID]domain.TreeState{}, skipped: map[uuid.UUID]bool{}}
	if len(conts) == 0 {
		return out, nil
	}
	at := make(map[uuid.UUID]time.Time, len(conts))
	for _, s := range conts {
		at[s.node] = r.plan.groups[s.group].at
	}
	read, err := u.w.d.Trash.TrashNodesAt(ctx, u.write.NotebookID, at)
	if err != nil {
		return restoring{}, err
	}
	var conflicts []domain.RevertConflict
	conflict := func(id uuid.UUID, name string, reason domain.ConflictReason, holder *uuid.UUID) {
		conflicts = append(conflicts, domain.RevertConflict{NodeID: id, Name: name, Reason: reason, HolderID: holder})
	}
	places := contPlaces{u: u, kids: map[uuid.UUID]map[string]uuid.UUID{}, depths: map[uuid.UUID]int{}}
	var refs []domain.VersionRef
	for _, s := range conts {
		g := r.plan.groups[s.group]
		n := g.nodes[s.member]
		it, ok := befores[s.node]
		if !ok || it.Before == nil || it.After != nil {
			// The changeset's item of it is gone with it: the purge took it.
			conflict(s.node, "", domain.ConflictChanged, nil)
			continue
		}
		before := *it.Before
		parent := g.nodes[n.Parent].ID
		m, deleted := read[s.node]
		_, parentBack := out.members[parent]
		switch {
		case !deleted:
			if now, err := u.w.d.Nodes.FindNodeIn(ctx, u.write.NotebookID, s.node); err == nil && now.State().Same(before) {
				out.skipped[s.node] = true
			} else if err != nil && !errors.Is(err, ErrNotFound) {
				return restoring{}, err
			} else {
				conflict(s.node, before.Name, domain.ConflictChanged, nil)
			}
			continue
		case !parentBack && (!m.Root || m.Pages != int(n.Pages) || m.Assets != int(n.Assets) || m.Height != int(n.Height)):
			conflict(s.node, before.Name, domain.ConflictChanged, nil)
			continue
		case !domain.Restorable(g.at, u.write.At, r.retention):
			conflict(s.node, before.Name, domain.ConflictExpired, nil)
			continue
		}
		reason, holder, err := places.check(ctx, s.node, m.Node.Kind, before, parentBack)
		if err != nil {
			return restoring{}, err
		}
		if reason != "" {
			conflict(s.node, before.Name, reason, holder)
			continue
		}
		out.members[s.node], out.states[s.node] = m.TrashMember, before
		if m.Node.Kind == domain.KindPage {
			refs = append(refs, domain.VersionRef{NodeID: s.node, Revision: m.Revision})
		}
	}
	if len(conflicts) > 0 {
		slices.SortFunc(conflicts, domain.CompareConflicts)
		return restoring{}, domain.RevertConflicts(conflicts[:min(len(conflicts), domain.MaxConflicts)], len(conflicts))
	}
	if len(refs) > 0 {
		hashes, err := u.w.d.History.VersionHashes(ctx, refs)
		if err != nil {
			return restoring{}, err
		}
		for _, ref := range refs {
			hash, ok := hashes[ref]
			if !ok {
				// The purge took the version it restores, as planned's.
				return restoring{}, domain.ErrChangesetTrimmed
			}
			out.contents = append(out.contents, writtenContent{id: ref.NodeID, name: out.states[ref.NodeID].Name, hash: hash})
		}
	}
	return out, nil
}

// contPlaces are where a batch's continuation puts its members back:
// the keys of the names taken under each parent, by the node holding
// each, and each parent's depth, read once each.
type contPlaces struct {
	u      *Unit
	kids   map[uuid.UUID]map[string]uuid.UUID
	depths map[uuid.UUID]int
}

// check is why the node id, of kind, cannot go back to before: its parent
// not alive, nor restored by the batch (parentBack), parent_gone; its name
// held there, title_taken by the node holding it; its page deeper than
// domain.MaxDepth, too_deep. None: it takes its name and depth there.
func (c contPlaces) check(ctx context.Context, id uuid.UUID, kind domain.Kind, before domain.TreeState, parentBack bool) (
	domain.ConflictReason, *uuid.UUID, error,
) {
	parent := *before.ParentID
	kids, ok := c.kids[parent]
	if !ok {
		kids = map[string]uuid.UUID{}
		if !parentBack {
			if _, err := c.u.w.d.Nodes.FindNodeIn(ctx, c.u.write.NotebookID, parent); errors.Is(err, ErrNotFound) {
				return domain.ConflictParentGone, nil, nil
			} else if err != nil {
				return "", nil, err
			}
			ancestors, err := c.u.w.d.Nodes.Ancestors(ctx, parent)
			if err != nil {
				return "", nil, err
			}
			siblings, err := c.u.w.d.Nodes.Children(ctx, c.u.write.NotebookID, &parent)
			if err != nil {
				return "", nil, err
			}
			for _, s := range siblings {
				kids[s.NameKey] = s.ID
			}
			c.depths[parent] = domain.Depth(ancestors)
		}
		c.kids[parent] = kids
	}
	key := shared.TitleKey(before.Name)
	if holder, held := kids[key]; held {
		return domain.ConflictTitleTaken, &holder, nil
	}
	depth := c.depths[parent] + 1
	if kind == domain.KindPage && depth > domain.MaxDepth {
		return domain.ConflictTooDeep, nil, nil
	}
	kids[key], c.depths[id] = id, depth
	return "", nil, nil
}

// restoreSegment restores the batch's part of a group, its steps in the
// group's pre-order: from its first member the whole group's restore as
// planned again, each member where the changeset deleted it, the group's
// time the plan's from then on; else the continuation's members it
// restores, as they are. Then the nodes under them later batches restore
// become trash items of their own. It answers how many it restored and
// skipped.
func (u *Unit) restoreSegment(ctx context.Context, r RevertPart, steps []revertStep, ops map[stepKey]domain.RevertOp,
	cont restoring,
) (int, int, error) {
	g := &r.plan.groups[steps[0].group]
	from, to := steps[0].member, steps[len(steps)-1].member+1
	var members []domain.TrashMember
	var states []domain.TreeState
	back := make([]bool, to-from)
	skipped := 0
	if from == 0 {
		op := ops[stepKey{domain.RevertRestore, steps[0].node}]
		if len(op.Group) != len(g.nodes) {
			return 0, 0, fmt.Errorf("a background revert's group of %s holds %d nodes, its plan's %d", steps[0].node, len(op.Group), len(g.nodes))
		}
		at := make(map[uuid.UUID]int, len(op.Group))
		for i, m := range op.Group {
			at[m.Node.ID] = i
		}
		for j := from; j < to; j++ {
			i, ok := at[g.nodes[j].ID]
			if !ok {
				return 0, 0, fmt.Errorf("a background revert's group of %s lacks its plan's node %s", steps[0].node, g.nodes[j].ID)
			}
			members, states, back[j-from] = append(members, op.Group[i]), append(states, op.States[i]), true
		}
		// The group deleted again since the plan, as the revert of its
		// restore did: the time the later batches read it at.
		g.at = op.DeletedAt
	} else {
		for j := from; j < to; j++ {
			id := g.nodes[j].ID
			if m, ok := cont.members[id]; ok {
				members, states, back[j-from] = append(members, m), append(states, cont.states[id]), true
			} else if cont.skipped[id] {
				skipped++
			}
		}
	}
	if len(members) == 0 {
		return 0, skipped, nil
	}
	keys, facts := make([]string, len(members)), map[uuid.UUID]Facts{}
	for i, m := range members {
		keys[i], facts[m.Node.ID] = shared.TitleKey(states[i].Name), r.parsed[m.Node.ID].Facts
	}
	if err := u.bringBack(ctx, members, g.at, states, keys, facts, func(context.Context) error { return nil }); err != nil {
		return 0, 0, err
	}
	return len(members), skipped, u.markFrontier(ctx, g.nodes, domain.Frontier(g.nodes, from, to, func(j int) bool { return back[j-from] }), g.at)
}

// revertDefect is err of a batch's step s: a classified error but the
// guards' page.locked, the restore's page.trash_changed and the server's
// 503 is a defect, as Revert has it: the plan checked the step.
func revertDefect(s revertStep, err error) error {
	var e *shared.Error
	if !errors.As(err, &e) || e.Kind == shared.KindUnavailable || errors.Is(err, domain.ErrLocked) || errors.Is(err, domain.ErrTrashChanged) {
		return err
	}
	return unclassified(fmt.Sprintf("a background revert's %s of node %s", s.kind, s.node), err)
}
