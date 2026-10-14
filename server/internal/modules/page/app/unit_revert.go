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
	var changed []domain.RevertConflict
	check := func(id uuid.UUID, name string, hash []byte) {
		if p, ok := parsed[id]; !ok || !bytes.Equal(p.Hash, hash) {
			changed = append(changed, domain.RevertConflict{NodeID: id, Name: name, Reason: domain.ConflictChanged})
		}
	}
	for _, op := range plan.Ops {
		switch op.Kind {
		case domain.RevertRestore:
			for i, m := range op.Group {
				if m.Node.Kind == domain.KindPage {
					check(m.Node.ID, op.States[i].Name, state.Hashes[domain.VersionRef{NodeID: m.Node.ID, Revision: m.Revision}])
				}
			}
		case domain.RevertContent:
			check(op.NodeID, state.Nodes[op.NodeID].Name, op.Hash)
		}
	}
	if len(changed) > 0 {
		// All changed: in their order, by node id, as the plan's.
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
