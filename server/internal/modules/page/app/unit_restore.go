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
	var held []string
	if place.ParentID == nil && u.write.Notebook.Mode == domain.ModeLLMWiki {
		held = domain.ReservedKeys()
	}
	names, renamed, err := restoredNames(group, siblings, held)
	if err != nil {
		return RestoredItem{}, err
	}
	states, keys := make([]domain.TreeState, len(group)), make([]string, len(group))
	for i, m := range group {
		states[i], keys[i] = domain.TreeState{ParentID: m.Node.ParentID, Name: names[i].Name, SortOrder: m.Node.SortOrder}, names[i].Key
	}
	states[0].ParentID, states[0].SortOrder = place.ParentID, order
	if err := u.bringBack(ctx, group, at, states, keys, r.Facts, renumber, restoredLines(place.ParentID, lines[id])); err != nil {
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
	facts map[uuid.UUID]Facts, before func(context.Context) error, lines *StepLines,
) error {
	back := Restoration{Root: group[0].Node.ID, ParentID: states[0].ParentID, SortOrder: states[0].SortOrder, DeletedAt: at,
		By: u.write.By, At: u.write.At}
	changes := make([]domain.Change, len(group))
	kinds := make(map[uuid.UUID]domain.Kind, len(group))
	pages := 0
	for i, m := range group {
		kinds[m.Node.ID] = m.Node.Kind
		changes[i] = domain.Change{NodeID: m.Node.ID, After: &states[i],
			Restored: &domain.Restored{DeletedAt: at, Asset: m.Node.Kind == domain.KindAsset}}
		if m.Node.Kind == domain.KindPage {
			changes[i].Revision, changes[i].Facts = m.Revision, facts[m.Node.ID]
			pages++
		}
		back.IDs, back.Names, back.NameKeys = append(back.IDs, m.Node.ID), append(back.Names, states[i].Name), append(back.NameKeys, keys[i])
	}
	step := u.step(domain.OpRestore, changes...)
	step.Kinds, step.Lines = kinds, lines
	return u.applyStep(ctx, step, true, func(ctx context.Context, prepared Step) error {
		if err := before(ctx); err != nil {
			return err
		}
		if len(prepared.AreaHashes) > 0 {
			back.SourceHashes = make(map[uuid.UUID][]byte, len(prepared.AreaHashes))
			for id, value := range prepared.AreaHashes {
				back.SourceHashes[id] = value.Hash
			}
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
// and keyed by today's rules; the root's numbered when a live sibling
// where it goes holds its key, or the key is among held; another's when
// a node of the group before it under its parent, by order and id, does:
// a Unicode update's
// reindex may have keyed two names alike (M8/P2 design 3.5).
func restoredNames(group []domain.TrashMember, siblings []domain.Node, held []string) ([]domain.Title, int, error) {
	under := map[uuid.UUID]*children{}
	kidsOf := func(i int) *children {
		if i == 0 {
			kids := &children{keys: make(map[string]bool, len(siblings)), next: map[string]int{}}
			for _, s := range siblings {
				kids.keys[s.NameKey] = true
			}
			for _, key := range held {
				kids.keys[key] = true
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

// TrashPart is a background restore's batch (M8/P4 design 3.4): of the
// trash item deleted at At whose group is Nodes, in its pre-order with
// their counts, the nodes From to To (not included); the root's counts as
// the job's plan read them; the facts of the batch's pages parsed before
// the unit; the purge's retention.
type TrashPart struct {
	At                    time.Time
	Nodes                 []domain.GroupNode
	From, To              int
	Pages, Assets, Height int
	Facts                 map[uuid.UUID]Facts
	Retention             time.Duration
}

// PartRestored is what a batch of a background restore did: the pages and
// attachments it restored, those it renamed, those someone else restored
// before it, which it skipped, and where its first batch put the item's
// root.
type PartRestored struct {
	Pages, Assets, Renamed, Skipped int
	Placement                       *domain.Placement
}

// RestorePart restores a batch of a trash item's group under the
// notebook's lock (M8/P4 design 3.4). Within the time the item can be
// restored in at the unit's time (page.restore_expired), its members are
// read at the item's time: the first batch's root must be the item's
// root still, of its counts as planned (page.trash_changed); a member no
// longer deleted then someone restored, a trash item of its own the batch
// before it left, and is skipped; one still deleted then whose parent the
// batch does not restore must be such an item, of its counts in the
// group (page.trash_changed). The parent of each must be alive
// (ErrNoParent) and its pages no deeper than domain.MaxDepth
// (domain.ErrTooDeep): someone may have deleted or moved what the batches
// before it restored. The root goes where PlacementOf puts it, as Restore
// has it; every other member under its parent, at its order, its name
// numbered when a sibling alive or restored before it under the same
// parent holds its key. Then the nodes of the group under those it
// restored that later batches restore become trash items of their own,
// with their counts (domain.Frontier).
func (u *Unit) RestorePart(ctx context.Context, p TrashPart) (PartRestored, error) {
	if !domain.Restorable(p.At, u.write.At, p.Retention) {
		return PartRestored{}, domain.ErrRestoreExpired
	}
	at := make(map[uuid.UUID]time.Time, p.To-p.From)
	for _, n := range p.Nodes[p.From:p.To] {
		at[n.ID] = p.At
	}
	read, err := u.w.d.Trash.TrashNodesAt(ctx, u.write.NotebookID, at)
	if err != nil {
		return PartRestored{}, err
	}
	if p.From == 0 {
		if r, ok := read[p.Nodes[0].ID]; !ok || !r.Root || r.Pages != p.Pages || r.Assets != p.Assets || r.Height != p.Height {
			return PartRestored{}, domain.ErrTrashChanged
		}
	}
	var out PartRestored
	restored := make([]bool, p.To-p.From)
	inBatch := func(i int) bool { return i >= p.From && i < p.To && restored[i-p.From] }
	var members []domain.TrashMember
	var positions []int
	for i := p.From; i < p.To; i++ {
		n := p.Nodes[i]
		m, ok := read[n.ID]
		switch {
		case !ok:
			out.Skipped++
			continue
		case i > 0 && !inBatch(int(n.Parent)) &&
			(!m.Root || m.Pages != int(n.Pages) || m.Assets != int(n.Assets) || m.Height != int(n.Height)):
			return PartRestored{}, domain.ErrTrashChanged
		}
		restored[i-p.From] = true
		members, positions = append(members, m.TrashMember), append(positions, i)
	}
	if len(members) == 0 {
		return out, nil
	}
	places := partPlaces{u: u, p: p, kids: map[int]*children{}, depths: map[int]int{}, lines: map[int][]domain.Ancestor{}}
	var sourceLines *StepLines
	states, keys := make([]domain.TreeState, len(members)), make([]string, len(members))
	renumber := func(context.Context) error { return nil }
	for k, i := range positions {
		m := members[k]
		state := m.Node.State()
		var kids *children
		if i == 0 {
			place, order, siblings, before, line, err := u.placeRoot(ctx, m.Node, p.Height)
			if err != nil {
				return PartRestored{}, err
			}
			out.Placement, renumber, sourceLines = &place, before, line
			state.ParentID, state.SortOrder = place.ParentID, order
			kids = places.under(-1, siblings)
			if place.ParentID == nil && u.write.Notebook.Mode == domain.ModeLLMWiki {
				for _, key := range domain.ReservedKeys() {
					kids.keys[key] = true
				}
			}
		} else {
			if kids, err = places.of(ctx, int(p.Nodes[i].Parent), inBatch); err != nil {
				return PartRestored{}, err
			}
			if k == 0 {
				sourceLines = &StepLines{After: places.lines[int(p.Nodes[i].Parent)]}
			}
			if err := places.deep(i, m.Node.Kind); err != nil {
				return PartRestored{}, err
			}
		}
		title, err := kids.free(m.Node.Name, m.Node.Kind == domain.KindAsset, nil)
		if err != nil {
			return PartRestored{}, err
		}
		kids.keys[title.Key] = true
		state.Name = title.Name
		states[k], keys[k] = state, title.Key
		if title.Name != m.Node.Name {
			out.Renamed++
		}
		if m.Node.Kind == domain.KindAsset {
			out.Assets++
		} else {
			out.Pages++
		}
	}
	facts := make(map[uuid.UUID]Facts, out.Pages)
	for _, m := range members {
		if m.Node.Kind == domain.KindPage {
			f, ok := p.Facts[m.Node.ID]
			if !ok {
				// Deleted at the time, unread before the unit: the purge took
				// its content.
				return PartRestored{}, domain.ErrTrashChanged
			}
			facts[m.Node.ID] = f
		}
	}
	if u.write.Notebook.Mode == domain.ModeLLMWiki {
		if sourceLines == nil {
			sourceLines = &StepLines{}
		}
		sourceLines.AfterParents = make(map[uuid.UUID][]domain.Ancestor, len(places.lines))
		for at, line := range places.lines {
			sourceLines.AfterParents[p.Nodes[at].ID] = line
		}
	}
	if err := u.bringBack(ctx, members, p.At, states, keys, facts, renumber, sourceLines); err != nil {
		return PartRestored{}, err
	}
	return out, u.markFrontier(ctx, p.Nodes, domain.Frontier(p.Nodes, p.From, p.To, func(i int) bool { return restored[i-p.From] }), p.At)
}

// placeRoot is where a trash item's root, n, goes back, its group spanning
// height levels of pages, as Restore puts it: PlacementOf on its line as it
// is now; its order there, its own under its parent, last at the
// notebook's root, with the renumbering of the siblings that takes; and
// the siblings alive where it goes. PlacementOf keeps its group within
// domain.MaxDepth.
func (u *Unit) placeRoot(ctx context.Context, n domain.Node, height int) (domain.Placement, float64, []domain.Node,
	func(context.Context) error, *StepLines, error,
) {
	lines, err := u.w.d.Trash.TrashLines(ctx, []uuid.UUID{n.ID})
	if err != nil {
		return domain.Placement{}, 0, nil, nil, nil, err
	}
	place := domain.PlacementOf(lines[n.ID], height)
	siblings, err := u.w.d.Nodes.Children(ctx, u.write.NotebookID, place.ParentID)
	if err != nil {
		return domain.Placement{}, 0, nil, nil, nil, err
	}
	order, renumber := n.SortOrder, func(context.Context) error { return nil }
	if place.ParentID == nil && n.ParentID != nil {
		order, _, renumber = u.placeAmong(siblings, len(siblings)-1)
	}
	return place, order, siblings, renumber, restoredLines(place.ParentID, lines[n.ID]), nil
}

// partPlaces are where a batch of a background restore puts its members
// but the item's root: the names taken under each parent, by its position
// in the group, and the depths of the parents the batch did not restore,
// read once each, and of the members under them. The members under the
// root its first batch restores have none: PlacementOf kept the group
// within domain.MaxDepth.
type partPlaces struct {
	u      *Unit
	p      TrashPart
	kids   map[int]*children
	depths map[int]int
	lines  map[int][]domain.Ancestor
}

// under are the names taken where siblings are alive, under the parent at
// position at (-1: where the root goes).
func (s partPlaces) under(at int, siblings []domain.Node) *children {
	kids := &children{nodes: siblings, keys: make(map[string]bool, len(siblings)), next: map[string]int{}}
	for _, n := range siblings {
		kids.keys[n.NameKey] = true
	}
	s.kids[at] = kids
	return kids
}

// of are the names taken under the parent at position at: the batch's
// members restored before under it, when the batch restored it; else its
// children alive, those of the batches before it among them, read once,
// the parent alive (ErrNoParent) and its depth read with them.
func (s partPlaces) of(ctx context.Context, at int, inBatch func(int) bool) (*children, error) {
	if kids, ok := s.kids[at]; ok {
		return kids, nil
	}
	if inBatch(at) {
		return s.under(at, nil), nil
	}
	id := s.p.Nodes[at].ID
	parent, err := s.u.w.d.Nodes.FindNodeIn(ctx, s.u.write.NotebookID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNoParent
	} else if err != nil {
		return nil, err
	}
	ancestors, err := s.u.w.d.Nodes.Ancestors(ctx, id)
	if err != nil {
		return nil, err
	}
	siblings, err := s.u.w.d.Nodes.Children(ctx, s.u.write.NotebookID, &id)
	if err != nil {
		return nil, err
	}
	s.depths[at] = domain.Depth(ancestors)
	s.lines[at] = append(ancestors, domain.Ancestor{ID: parent.ID, Name: parent.Name})
	return s.under(at, siblings), nil
}

// deep keeps the depth of the member at position i, of kind, one below
// its parent's when that is known, and refuses a page deeper than
// domain.MaxDepth: someone moved what the batches before it restored
// deeper.
func (s partPlaces) deep(i int, kind domain.Kind) error {
	d, ok := s.depths[int(s.p.Nodes[i].Parent)]
	if !ok {
		return nil
	}
	s.depths[i] = d + 1
	if kind == domain.KindPage && d+1 > domain.MaxDepth {
		return domain.ErrTooDeep
	}
	return nil
}

// markFrontier makes the nodes at positions frontier of a group in
// pre-order, deleted at at, trash items of their own with their counts in
// the group: each must be deleted at at still, and no root, else
// page.trash_changed. No change of the tree: they stay in the trash.
func (u *Unit) markFrontier(ctx context.Context, nodes []domain.GroupNode, frontier []int, at time.Time) error {
	if len(frontier) == 0 {
		return nil
	}
	marks := make([]TrashMark, len(frontier))
	for i, f := range frontier {
		n := nodes[f]
		marks[i] = TrashMark{ID: n.ID, Pages: int(n.Pages), Assets: int(n.Assets), Height: int(n.Height)}
	}
	n, err := u.w.d.NodeWriter.MarkTrashRoots(ctx, marks, at)
	switch {
	case err != nil:
		return err
	case n != len(marks):
		return domain.ErrTrashChanged
	}
	return nil
}

// restoredLines uses the line already read to place a restore. Falling back
// to the notebook root has a known empty After, regardless of its old line.
func restoredLines(parent *uuid.UUID, line []domain.TrashAncestor) *StepLines {
	out := &StepLines{}
	if parent != nil {
		for _, a := range line {
			out.After = append(out.After, domain.Ancestor{ID: a.ID, Name: a.Name})
		}
	}
	return out
}
