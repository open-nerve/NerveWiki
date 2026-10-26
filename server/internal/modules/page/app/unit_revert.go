package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
// by page and hash (a page may come back at one and be written back with
// another, M9/P3 design 3.18.3), the purge's retention, and the bounds of
// the changeset's items and of its schedule's steps
// (domain.RevertPlan.Steps).
type RevertRead struct {
	Changeset uuid.UUID
	Parsed    map[domain.ContentRef]ParsedContent
	Retention time.Duration
	MaxItems  int
	MaxSteps  int
}

// RevertDone is what a revert's unit did: how many nodes it put back,
// subtrees it deleted, trash items it restored and pages it wrote back,
// and the steps aside it took on the way (M9/P3 design 3.21.3).
type RevertDone struct {
	Placed, Deleted, Restored, Written, Aside int
}

// Revert reverts the changeset r names, under the notebook's lock: it is
// read again, its items and versions, then its row, page.changeset_trimmed
// for one the purge marked or took meanwhile (the lock keeps its notebook), page.revert_too_large past
// the bound of its items, merged into meanwhile; then the state, planned
// at the unit's time, the plan this one, which is authoritative:
// page.revert_conflict, page.changeset_trimmed for a version the purge
// took, page.revert_too_large past the bound of its schedule's steps;
// then the notebook contract on every scheduled step, using its Grants.
// Each content it writes back or restores must have been parsed at the
// hash it writes now, else its page is changed. Then its schedule's
// steps, each a unit's operation through the guards (page.locked for a
// page under another's lock, or the caller's elsewhere), its write, the
// changeset and the participants: a place moves the node, its subtree
// with it, then renames it, and a step aside so too, to its name for a
// while; a deletion deletes a subtree; a restore brings a trash item back
// where the changeset deleted it, at its time; a content writes the
// version back on the revision the plan has it at, the unit's read of an
// alive page's, a restored one's in the trash. Their checks are the
// plan's: a name taken or a revision moved in its write is a defect,
// unclassified, as the purge's take of what a restore brings back is
// page.trash_changed, which the use case answers.
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
	guard := NewModeGuard(u.w.d.Auth)
	step := Step{Write: u.write}
	ask := func(action shared.Action) error { return u.write.Grants.Ask(ctx, action, guard.decide(step, action)) }
	if err := checkRevertContract(state, plan, u.write.Notebook, ask); err != nil {
		return RevertDone{}, err
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
		case domain.RevertAside:
			err = u.place(ctx, op)
			done.Aside++
		case domain.RevertDelete:
			_, err = u.Delete(ctx, op.NodeID)
			done.Deleted++
		case domain.RevertRestore:
			keys, facts := make([]string, len(op.Group)), map[uuid.UUID]Facts{}
			for i, m := range op.Group {
				keys[i] = shared.TitleKey(op.States[i].Name)
				facts[m.Node.ID] = r.Parsed[refOf(m.Node.ID, state.Hashes[domain.VersionRef{NodeID: m.Node.ID, Revision: m.Revision}])].Facts
			}
			err = u.bringBack(ctx, op.Group, op.DeletedAt, op.States, keys, facts, func(context.Context) error { return nil })
			done.Restored++
		case domain.RevertContent:
			parsed := r.Parsed[refOf(op.NodeID, op.Hash)]
			_, err = u.WriteContent(ctx, ContentWrite{NodeID: op.NodeID, Content: parsed.Content, Facts: parsed.Facts, Base: op.Current,
				Rewrite: op.Rewrite})
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

// place puts a node back where a revert's plan says, or aside for a while
// (M9/P3 design 3.16.3), its checks the plan's: a move with the name it
// has, its subtree with it, at the order it had, then a rename, keyed by
// today's rules; a move and a rename at once, as one MCP move writes it
// and every step aside to the root, in one step of OpMove, one statement
// (M9/P2 design 3.5.4). A title taken in any write is the plan's defect.
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
		write := u.w.d.NodeWriter.MoveNode
		if op.Rename {
			// The step renames too: no rename is left after it.
			n.Name, n.NameKey, write = op.To.Name, shared.TitleKey(op.To.Name), u.w.d.NodeWriter.RelocateNode
			op.Rename = false
		}
		if err := u.moveSubtree(ctx, sub, n, func(context.Context) error { return nil }, write); err != nil {
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
// written back, a page's content in the trash, each by page and hash. One
// that was not is a page changed between the two reads, a conflict,
// listed by node id; one of the same bytes at another revision is none,
// its revision the unit's.
func parsedAsPlanned(plan domain.RevertPlan, state domain.RevertState, parsed map[domain.ContentRef]ParsedContent) error {
	return parsedAs(plannedContents(plan, state, nil), parsed)
}

// refOf is the content of the page id that hashes to hash.
func refOf(id uuid.UUID, hash []byte) domain.ContentRef {
	return domain.ContentRef{NodeID: id, Hash: hex.EncodeToString(hash)}
}

// parsedContents are contents parsed, by page and hash, with their facts.
func parsedContents(contents map[domain.ContentRef]string, facts map[domain.ContentRef]Facts) map[domain.ContentRef]ParsedContent {
	out := make(map[domain.ContentRef]ParsedContent, len(contents))
	for ref, content := range contents {
		sum := sha256.Sum256([]byte(content))
		out[ref] = ParsedContent{Hash: sum[:], Content: content, Facts: facts[ref]}
	}
	return out
}

// contentOf is the page id's content text, by its hash.
func contentOf(id uuid.UUID, text string) domain.ContentRef {
	sum := sha256.Sum256([]byte(text))
	return refOf(id, sum[:])
}

// writtenContent is a content a revert writes: its page, the page's name
// and the hash it writes, and whether it restores it, else writes it
// back.
type writtenContent struct {
	id       uuid.UUID
	name     string
	hash     []byte
	restores bool
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
						hash: state.Hashes[domain.VersionRef{NodeID: m.Node.ID, Revision: m.Revision}], restores: true})
				}
			}
		case domain.RevertContent:
			out = append(out, writtenContent{id: op.NodeID, name: state.Nodes[op.NodeID].Name, hash: op.Hash})
		}
	}
	return out
}

// parsedAs checks each content was parsed at the hash it writes: one that
// was not is changed, all of them listed in their order, by node id, as
// the plan's.
func parsedAs(contents []writtenContent, parsed map[domain.ContentRef]ParsedContent) error {
	var changed []domain.RevertConflict
	for _, c := range contents {
		if _, ok := parsed[refOf(c.id, c.hash)]; !ok {
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
// or write back, parsed before the unit, by page and hash; the purge's
// retention;
// and retimed, where the unit tells the time it restored each group it
// begins at, by its index, which the plan takes once the unit commits.
type RevertPart struct {
	plan      *revertPlan
	from, to  int
	parsed    map[domain.ContentRef]ParsedContent
	retention time.Duration
	retimed   map[int32]time.Time
}

// RevertBatch reverts a batch of the changeset the plan reverts, under the
// notebook's lock (M8/P4 design 3.5; M9/P3 design 3.24), its steps the
// plan's, as the job planned them at its start. It reads the changeset's
// items and versions of the batch's nodes, then its row:
// page.changeset_trimmed for one the purge marked or took. Its nodes are
// a place's and a step aside's node, a deletion's subtree at its moment, a
// write back's page, and a group's members, all of them, when the batch
// holds its first; a group's later member whose group an earlier batch
// began is read alone (its continuation). Then the state of those, at the
// unit's time, and L (domain.CheckRevertPart): page.revert_conflict, a
// page it writes back named as in S* (starNamed), then
// page.changeset_trimmed for a version the purge took. A continuation's
// members are checked alone, as they are in the trash: one no longer
// deleted at the group's time that is alive where the changeset deleted
// it from, someone's restore, is skipped; else changed, as is one still
// deleted whose parent the batch does not restore but no trash item of
// its counts, or one whose content in the trash is not the changeset's
// version the batch writes back; then expired, parent_gone, title_taken
// with the node holding its name (no number: it goes back where it was
// or not at all), too_deep. Then each step's premise in its order
// (domain.CheckRevertSteps): the first that fails is page.revert_conflict.
// Each content it writes must have been parsed at the hash it writes now.
// Then its steps, in their order, as Revert has them: a group's members in
// the batch restored at once, at its first step in it, each where the
// changeset deleted it, at the time the batch read; the nodes under them
// later batches restore made trash items of their own; a write back on the
// revision just checked. A classified error but the guards' page.locked
// and the restore's page.trash_changed is a defect.
func (u *Unit) RevertBatch(ctx context.Context, r RevertPart) (BatchDone, error) {
	steps := r.plan.steps[r.from:r.to]
	var items, versions, conts []uuid.UUID
	var contSteps []revertStep
	opened, cont := map[int32]bool{}, map[uuid.UUID]bool{}
	for _, s := range steps {
		switch {
		case s.kind == domain.RevertPlace || s.kind == domain.RevertAside:
			items = append(items, s.node)
		case s.kind == domain.RevertContent:
			versions = append(versions, s.node)
		case s.kind == domain.RevertDelete:
			items, versions = append(items, s.nodes...), append(versions, s.nodes...)
		case s.member == 0:
			ids := r.plan.groupIDs(s.group)
			items, versions, opened[s.group] = append(items, ids...), append(versions, ids...), true
		case !opened[s.group]:
			conts, contSteps, cont[s.node] = append(conts, s.node), append(contSteps, s), true
		}
	}
	read, befores, err := u.batchChangeset(ctx, r.plan.changeset, items, versions, conts)
	if err != nil {
		return BatchDone{}, err
	}
	state, err := u.batchState(ctx, r, read)
	if err != nil {
		return BatchDone{}, err
	}
	// A page the batch's continuation restores is in the trash, written
	// back after: its version is checked as the trash holds it, not as a
	// page alive's.
	trashedVersions := map[uuid.UUID]domain.ChangesetVersion{}
	state.Versions = slices.DeleteFunc(state.Versions, func(v domain.ChangesetVersion) bool {
		n, ok := state.Nodes[v.NodeID]
		if cont[v.NodeID] && ok && n.DeletedAt != nil {
			trashedVersions[v.NodeID] = v
			return true
		}
		return false
	})
	switch plan := domain.CheckRevertPart(state); {
	case plan.Count > 0:
		return BatchDone{}, domain.RevertConflicts(starNamed(plan.Conflicts, steps), plan.Count)
	case len(plan.Missing) > 0:
		return BatchDone{}, domain.ErrChangesetTrimmed
	}
	restoring, err := u.continuation(ctx, r, contSteps, befores, trashedVersions)
	if err != nil {
		return BatchDone{}, err
	}
	ops, conflicts := domain.CheckRevertSteps(state, r.plan.ops(steps, restoring))
	if len(conflicts) > 0 {
		return BatchDone{}, domain.RevertConflicts(starNamed(conflicts[:min(len(conflicts), domain.MaxConflicts)], steps), len(conflicts))
	}
	// Of a group the batch begins, its members in the batch alone.
	restored := map[uuid.UUID]bool{}
	for _, s := range steps {
		if s.kind == domain.RevertRestore && opened[s.group] {
			restored[s.node] = true
		}
	}
	written := append(plannedContents(domain.RevertPlan{Ops: ops}, state, func(id uuid.UUID) bool { return restored[id] }),
		restoring.contents...)
	if err := parsedAs(written, r.parsed); err != nil {
		return BatchDone{}, err
	}
	// The pages the batch restores, each at its content in the trash.
	trashed := map[uuid.UUID][]byte{}
	for _, c := range written {
		if c.restores {
			trashed[c.id] = c.hash
		}
	}
	var done BatchDone
	aside := 0
	begun := map[int32]bool{}
	for i, s := range steps {
		op := ops[i]
		var err error
		switch s.kind {
		case domain.RevertPlace:
			err = u.place(ctx, op)
			done.PutBack++
		case domain.RevertAside:
			err = u.place(ctx, op)
			aside++
		case domain.RevertDelete:
			var sub domain.Subtree
			sub, err = u.Delete(ctx, s.node)
			done.Deleted += len(sub)
		case domain.RevertRestore:
			if begun[s.group] {
				continue
			}
			begun[s.group] = true
			var n, skipped int
			n, skipped, err = u.restoreGroup(ctx, r, groupSteps(steps[i:], s.group), op, restoring, trashed)
			done.Restored, done.Skipped = done.Restored+n, done.Skipped+skipped
		case domain.RevertContent:
			parsed := r.parsed[refOf(s.node, op.Hash)]
			_, err = u.WriteContent(ctx, ContentWrite{NodeID: s.node, Content: parsed.Content, Facts: parsed.Facts, Base: op.Current,
				Rewrite: op.Rewrite})
			done.Written++
		}
		if err != nil {
			return BatchDone{}, revertDefect(s, err)
		}
	}
	done.Nodes = done.PutBack + aside + done.Deleted + done.Restored + done.Written
	return done, nil
}

// starNamed are conflicts, a page the steps write back named as in S*,
// its step's name when the changeset's item of it has one: the batch
// reads the page's version, not its item, so L names it as it is now,
// where the job's start and a start planned again name it as in S*
// (review B's M-1). Only a conflict that names the page by its own name:
// a step aside's title_taken names the aside name its holder holds, and
// keeps it (review B-N1, B2-N-3). Its order is by reason and node, so it
// holds.
func starNamed(conflicts []domain.RevertConflict, steps []revertStep) []domain.RevertConflict {
	names, aside := map[uuid.UUID]string{}, map[uuid.UUID]string{}
	for _, s := range steps {
		switch {
		case s.kind == domain.RevertContent && s.name != "":
			names[s.node] = s.name
		case s.kind == domain.RevertAside:
			aside[s.node] = s.name
		}
	}
	for i, c := range conflicts {
		if name, ok := names[c.NodeID]; ok && (c.Reason != domain.ConflictTitleTaken || aside[c.NodeID] != c.Name) {
			conflicts[i].Name = name
		}
	}
	return conflicts
}

// groupSteps are the steps of the group g among steps, from its first:
// its members in the batch, in its pre-order, whatever steps come between.
func groupSteps(steps []revertStep, g int32) []revertStep {
	var out []revertStep
	for _, s := range steps {
		if s.kind == domain.RevertRestore && s.group == g {
			out = append(out, s)
		}
	}
	return out
}

// ops are the plan's steps as the domain checks them in their order
// (domain.CheckRevertSteps): a place and a step aside to their parent,
// name and order; a deletion with its subtree at its moment; a write back
// of its base and hash; a restore of its member, a continuation's with its
// member as the trash holds it and the state it goes back to, none for one
// it skips.
func (p *revertPlan) ops(steps []revertStep, cont restoring) []domain.RevertOp {
	out := make([]domain.RevertOp, len(steps))
	for i, s := range steps {
		op := domain.RevertOp{Kind: s.kind, Seq: int(s.seq), NodeID: s.node}
		switch s.kind {
		case domain.RevertPlace, domain.RevertAside:
			op.To = domain.TreeState{Name: s.name, SortOrder: s.order}
			if s.parent != (uuid.UUID{}) {
				op.To.ParentID = &s.parent
			}
		case domain.RevertDelete:
			op.Subtree = s.nodes
		case domain.RevertContent:
			op.Base, op.Hash = int(s.base), s.hash
			op.Bytes, op.Rewrite = s.bytes, s.rewrite
		case domain.RevertRestore:
			if m, ok := cont.members[s.node]; ok {
				op.Group, op.States = []domain.TrashMember{m}, []domain.TreeState{cont.states[s.node]}
			}
		}
		out[i] = op
	}
	return out
}

// batchChangeset reads the changeset's items of the nodes items and of
// the continuation's conts, and its versions of the pages versions, then
// its row: page.changeset_trimmed for one the purge marked or took (the
// lock keeps its notebook). It answers the items and versions the batch
// checks, and the continuation's items by node.
func (u *Unit) batchChangeset(ctx context.Context, id uuid.UUID, items, versions, conts []uuid.UUID) (
	domain.RevertState, map[uuid.UUID]domain.ChangesetItem, error,
) {
	var state domain.RevertState
	befores := map[uuid.UUID]domain.ChangesetItem{}
	if all := append(slices.Clone(items), conts...); len(all) > 0 {
		read, err := u.w.d.History.ChangesetItemsOf(ctx, id, all)
		if err != nil {
			return domain.RevertState{}, nil, err
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
			return domain.RevertState{}, nil, err
		}
	}
	c, err := u.w.d.History.FindChangeset(ctx, id)
	switch {
	case errors.Is(err, ErrNotFound):
		return domain.RevertState{}, nil, domain.ErrChangesetTrimmed
	case err != nil:
		return domain.RevertState{}, nil, err
	case c.Trimmed:
		return domain.RevertState{}, nil, domain.ErrChangesetTrimmed
	}
	return state, befores, nil
}

// batchState reads the state of the batch's items and versions s at the
// unit's time, as a revert's plan reads its changeset's (revertReads), the
// tree whole when it has items; none when it has neither: a batch of a
// continuation alone.
func (u *Unit) batchState(ctx context.Context, r RevertPart, s domain.RevertState) (domain.RevertState, error) {
	if len(s.Items) == 0 && len(s.Versions) == 0 {
		return domain.RevertState{At: u.write.At, Retention: r.retention}, nil
	}
	reads := revertReads{history: u.w.d.History, trash: u.w.d.Trash, nodes: u.w.d.Nodes}
	return reads.state(ctx, u.write.NotebookID, r.plan.changeset, s.Items, s.Versions, u.write.At, r.retention)
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
// changeset's items of them, befores, and its versions of those the batch
// writes back after, versions, and answers what it restores. Its
// conflicts, in their order: first every one it finds of the members, but
// for those under one that conflicts, which are none of their own; when
// there are none, each page it restores and writes back after whose
// content in the trash is not the changeset's version, changed (M9/P3
// design 3.24, step 2: L does not see it; review B-N2).
func (u *Unit) continuation(ctx context.Context, r RevertPart, conts []revertStep, befores map[uuid.UUID]domain.ChangesetItem,
	versions map[uuid.UUID]domain.ChangesetVersion,
) (restoring, error) {
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
	conflicted := map[uuid.UUID]bool{}
	conflict := func(id uuid.UUID, name string, reason domain.ConflictReason, holder *uuid.UUID) {
		conflicts, conflicted[id] = append(conflicts, domain.RevertConflict{NodeID: id, Name: name, Reason: reason, HolderID: holder}), true
	}
	places := contPlaces{u: u, kids: map[uuid.UUID]map[string]uuid.UUID{}, depths: map[uuid.UUID]int{}}
	var refs []domain.VersionRef
	for _, s := range conts {
		g := r.plan.groups[s.group]
		n := g.nodes[s.member]
		parent := g.nodes[n.Parent].ID
		if conflicted[parent] {
			// Under a member that cannot go back: no conflict of its own,
			// as a request's revert has a group's (M8/P4A review A-M3).
			conflicted[s.node] = true
			continue
		}
		it, ok := befores[s.node]
		if !ok || it.Before == nil || it.After != nil {
			// The changeset's item of it is gone with it: the purge took it.
			conflict(s.node, "", domain.ConflictChanged, nil)
			continue
		}
		before := *it.Before
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
			if v, ok := versions[ref.NodeID]; ok && !bytes.Equal(v.Hash, hash) {
				// Written back after: in the trash at another content than
				// the changeset left it at (M9/P3 design 3.24, step 2).
				conflict(ref.NodeID, out.states[ref.NodeID].Name, domain.ConflictChanged, nil)
			}
			out.contents = append(out.contents, writtenContent{id: ref.NodeID, name: out.states[ref.NodeID].Name, hash: hash, restores: true})
		}
	}
	if len(conflicts) > 0 {
		slices.SortFunc(conflicts, domain.CompareConflicts)
		return restoring{}, domain.RevertConflicts(conflicts[:min(len(conflicts), domain.MaxConflicts)], len(conflicts))
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

// restoreGroup restores the batch's members of a group at once, steps
// their steps in the group's pre-order (M9/P3 design 3.24, step 6): from
// its first member, op, the group's restore as the batch read and checked
// it, each member where the changeset deleted it, at the time the group
// is deleted at now, which r.retimed tells; else the continuation's
// members it restores, as they are; each page with the facts of its
// content in the trash, which trashed hashes. Then the nodes under them
// later batches restore become trash items of their own, once. It answers
// how many it restored and skipped.
func (u *Unit) restoreGroup(ctx context.Context, r RevertPart, steps []revertStep, op domain.RevertOp, cont restoring,
	trashed map[uuid.UUID][]byte,
) (int, int, error) {
	g := r.plan.groups[steps[0].group]
	from, to := int(steps[0].member), int(steps[len(steps)-1].member)+1
	deletedAt := g.at
	var members []domain.TrashMember
	var states []domain.TreeState
	back := make([]bool, to-from)
	skipped := 0
	if from == 0 {
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
		// The group deleted again since the plan (restored and deleted, or
		// the revert of its restore reverted): its unit plans it at that
		// time, the later batches read it there once this one commits.
		deletedAt = op.DeletedAt
		r.retimed[steps[0].group] = deletedAt
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
		keys[i], facts[m.Node.ID] = shared.TitleKey(states[i].Name), r.parsed[refOf(m.Node.ID, trashed[m.Node.ID])].Facts
	}
	if err := u.bringBack(ctx, members, deletedAt, states, keys, facts, func(context.Context) error { return nil }); err != nil {
		return 0, 0, err
	}
	return len(members), skipped, u.markFrontier(ctx, g.nodes, domain.Frontier(g.nodes, from, to, func(j int) bool { return back[j-from] }), deletedAt)
}

// revertDefect is err of a batch's step s: a classified error but the
// guards' marked refusals or page.locked, the restore's page.trash_changed
// and the server's 503 is a defect, as Revert has it: the plan checked the step.
func revertDefect(s revertStep, err error) error {
	var refused *domain.Refused
	if errors.As(err, &refused) {
		return err
	}
	var e *shared.Error
	if !errors.As(err, &e) || e.Kind == shared.KindUnavailable || errors.Is(err, domain.ErrLocked) || errors.Is(err, domain.ErrTrashChanged) {
		return err
	}
	return unclassified(fmt.Sprintf("a background revert's %s of node %s", s.kind, s.node), err)
}
