package app

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Restoring a trash item in a unit (M8/P2 design 3.5).

// TrashRestore is what a restore read of its trash item before its unit,
// unlocked: the item's root and its group, the group's pages' facts, and
// the purge's retention.
type TrashRestore struct {
	Root      domain.TrashRoot
	Group     []domain.TrashMember
	Facts     map[uuid.UUID]Facts
	Retention time.Duration
}

// RestoredItem is a trash item back: its root as the restore left it,
// whether the restore renamed it, where it put it and why, and how many
// of the rest of its group it renamed.
type RestoredItem struct {
	Node      domain.Node
	Renamed   bool
	Placement domain.Placement
	Others    int
}

// Restore brings the trash item r read back, under the notebook's lock.
// The item is read again: its root, still deleted at its time and a root,
// within the time it can be restored in at the unit's time
// (page.restore_expired), and its group, each node of the same kind and
// each page at the revision r read, its content there (else
// page.trash_changed: the item was restored meanwhile, or purged in part).
// It goes back where PlacementOf puts it on its line as it is now, under
// its parent at its order, or last at the notebook's root; its root
// numbered as an import numbers a name a sibling holds, and so is a node
// of the group whose name's key, recomputed, another before it under the
// same parent holds: refused, the item could never be restored. Each node
// is a change with no before, restored, a page with its revision and
// facts; then the rows deleted at the item's time come back, counted:
// every node, each page's content and its versions in one chain from its
// creation, else page.trash_changed.
func (u *Unit) Restore(ctx context.Context, r TrashRestore) (RestoredItem, error) {
	id, at := r.Root.Node.ID, r.Root.DeletedAt
	n, err := u.w.d.Nodes.FindAnyNode(ctx, id)
	switch {
	case errors.Is(err, ErrNotFound):
		return RestoredItem{}, domain.ErrTrashChanged
	case err != nil:
		return RestoredItem{}, err
	case n.NotebookID != u.write.NotebookID || n.DeletedAt == nil || !n.DeletedAt.Equal(at) || !n.TrashRoot:
		return RestoredItem{}, domain.ErrTrashChanged
	case !domain.Restorable(at, u.write.At, r.Retention):
		return RestoredItem{}, domain.ErrRestoreExpired
	}
	group, err := u.w.d.Trash.TrashGroup(ctx, u.write.NotebookID, id, at)
	if err != nil {
		return RestoredItem{}, found(err, domain.ErrTrashChanged)
	}
	if !sameGroup(group, r.Group) {
		return RestoredItem{}, domain.ErrTrashChanged
	}
	lines, err := u.w.d.Trash.TrashLines(ctx, []uuid.UUID{id})
	if err != nil {
		return RestoredItem{}, err
	}
	place := domain.PlacementOf(lines[id], groupHeight(group))
	siblings, err := u.w.d.Nodes.Children(ctx, u.write.NotebookID, place.ParentID)
	if err != nil {
		return RestoredItem{}, err
	}
	root := n.Node
	order, renumber := root.SortOrder, func(context.Context) error { return nil }
	if place.ParentID == nil && root.ParentID != nil {
		order, _, renumber = u.placeAmong(siblings, len(siblings)-1)
	}
	names, renamed, err := restoredNames(group, siblings)
	if err != nil {
		return RestoredItem{}, err
	}
	states, keys := make([]domain.TreeState, len(group)), make([]string, len(group))
	for i, m := range group {
		states[i], keys[i] = domain.TreeState{ParentID: m.Node.ParentID, Name: names[i].Name, SortOrder: m.Node.SortOrder}, names[i].Key
	}
	states[0].ParentID, states[0].SortOrder = place.ParentID, order
	if err := u.bringBack(ctx, group, at, states, keys, r.Facts, renumber); err != nil {
		return RestoredItem{}, err
	}
	root.ParentID, root.SortOrder, root.Name, root.NameKey = place.ParentID, order, names[0].Name, names[0].Key
	root.UpdatedBy, root.UpdatedAt = u.write.By, u.write.At
	others := renamed
	if names[0].Name != n.Name {
		others--
	}
	return RestoredItem{Node: root, Renamed: names[0].Name != n.Name, Placement: place, Others: others}, nil
}

// bringBack brings back the group of a trash item deleted at at, under
// the notebook's lock, its checks passed: each member to its state,
// keyed by its key, a change with no before, restored, a page with its
// revision and facts; before runs first in the step's write. Then the
// rows deleted at at come back, counted: every node, each page's content
// and its versions in one chain from its creation, else
// page.trash_changed. A restore and a revert share it (M8/P3 design 3.6).
func (u *Unit) bringBack(ctx context.Context, group []domain.TrashMember, at time.Time, states []domain.TreeState, keys []string,
	facts map[uuid.UUID]Facts, before func(context.Context) error,
) error {
	back := Restoration{Root: group[0].Node.ID, ParentID: states[0].ParentID, SortOrder: states[0].SortOrder, DeletedAt: at,
		By: u.write.By, At: u.write.At}
	changes := make([]domain.Change, len(group))
	pages := 0
	for i, m := range group {
		changes[i] = domain.Change{NodeID: m.Node.ID, After: &states[i],
			Restored: &domain.Restored{DeletedAt: at, Asset: m.Node.Kind == domain.KindAsset}}
		if m.Node.Kind == domain.KindPage {
			changes[i].Revision, changes[i].Facts = m.Revision, facts[m.Node.ID]
			pages++
		}
		back.IDs, back.Names, back.NameKeys = append(back.IDs, m.Node.ID), append(back.Names, states[i].Name), append(back.NameKeys, keys[i])
	}
	return u.apply(ctx, u.step(domain.OpRestore, changes...), true, func(ctx context.Context) error {
		if err := before(ctx); err != nil {
			return err
		}
		rows, err := u.w.d.NodeWriter.RestoreNodes(ctx, back)
		switch {
		case err != nil:
			return err
		case rows != (RestoredRows{Nodes: len(group), Contents: pages, CurrentVersions: pages, Creations: pages}):
			return domain.ErrTrashChanged
		}
		return nil
	})
}

// sameGroup reports whether a trash item's group read under the lock is
// the one read before it: the same nodes, of the same kinds, the pages at
// the same revisions, their contents there. The read before it refused a
// page whose content the purge took, revision 0: one taken since differs.
func sameGroup(now, before []domain.TrashMember) bool {
	if len(now) != len(before) {
		return false
	}
	for i, m := range now {
		b := before[i]
		if m.Node.ID != b.Node.ID || m.Node.Kind != b.Node.Kind || m.Revision != b.Revision {
			return false
		}
	}
	return true
}

// groupHeight is how many levels of pages a trash item's group spans, as
// Subtree.Height counts a subtree's.
func groupHeight(group []domain.TrashMember) int {
	h := 0
	for _, m := range group {
		if m.Node.Kind == domain.KindPage {
			h = max(h, m.Level)
		}
	}
	return h
}

// restoredNames are the names a trash item's group goes back with, its
// root's first, and how many differ from the names they had: each checked
// and keyed by today's rules; the root's numbered when one of siblings,
// alive where it goes, holds its key; another's when a node of the group
// before it under its parent, by order and id, does: a Unicode update's
// reindex may have keyed two names alike (M8/P2 design 3.5).
func restoredNames(group []domain.TrashMember, siblings []domain.Node) ([]domain.Title, int, error) {
	under := map[uuid.UUID]*children{}
	kidsOf := func(i int) *children {
		if i == 0 {
			kids := &children{keys: make(map[string]bool, len(siblings)), next: map[string]int{}}
			for _, s := range siblings {
				kids.keys[s.NameKey] = true
			}
			return kids
		}
		parent := *group[i].Node.ParentID
		if kids, ok := under[parent]; ok {
			return kids
		}
		kids := &children{keys: map[string]bool{}, next: map[string]int{}}
		under[parent] = kids
		return kids
	}
	out := make([]domain.Title, len(group))
	renamed := 0
	for i, m := range group {
		kids := kidsOf(i)
		title, err := kids.free(m.Node.Name, m.Node.Kind == domain.KindAsset, nil)
		if err != nil {
			return nil, 0, err
		}
		kids.keys[title.Key] = true
		out[i] = title
		if title.Name != m.Node.Name {
			renamed++
		}
	}
	return out, renamed, nil
}
