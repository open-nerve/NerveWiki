package app

import (
	"context"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Moving a node in a unit (M4/P2 design 3.4).

// Move puts the node id under parentID (nil: the notebook's root) at p
// among its new siblings. 404 for a node that is not in the notebook; 422
// for a parent that is no page of the notebook, or a sibling to follow
// that is no other child of it; then 409 for a parent in the node's own
// subtree or a title a new sibling holds. A source-entry subtree past 10,000
// nodes answers 422 before unread descendants' depth; otherwise depth past
// domain.MaxDepth is 409. A move to where the node is writes nothing.
// The node's descendants go with it: the step tells each, in a place of
// the same parent, name and order, and only the node's change is an item.
func (u *Unit) Move(ctx context.Context, id uuid.UUID, parentID *uuid.UUID, p Position) (domain.Node, error) {
	n, err := u.w.d.Nodes.FindNodeIn(ctx, u.write.NotebookID, id)
	if err != nil {
		return domain.Node{}, found(err, domain.ErrNotFound)
	}
	line, err := u.lineOf(ctx, parentID)
	if err != nil {
		return domain.Node{}, err
	}
	siblings, err := u.w.d.Nodes.Children(ctx, u.write.NotebookID, parentID)
	if err != nil {
		return domain.Node{}, err
	}
	self := func(s domain.Node) bool { return s.ID == n.ID }
	others := slices.DeleteFunc(slices.Clone(siblings), self)
	after, err := slotOf(others, p)
	if err != nil {
		return domain.Node{}, err
	}
	sameParent := domain.SameParent(n.ParentID, parentID)
	// Under the same parent, the node followed the sibling before it.
	if sameParent && after == slices.IndexFunc(siblings, self)-1 {
		return n, nil
	}
	if slices.ContainsFunc(line, func(a domain.Ancestor) bool { return a.ID == n.ID }) {
		return domain.Node{}, domain.ErrCycle
	}
	if !sameParent {
		if err := titleFree(others, domain.Title{Name: n.Name, Key: n.NameKey}, n.ID); err != nil {
			return domain.Node{}, err
		}
	}
	moved := n
	moved.ParentID = parentID
	sub, lines, err := u.subtreeForMove(ctx, n, moved, line)
	if err != nil {
		return domain.Node{}, err
	}
	// The subtree's deepest node lands Height - 1 levels below the node.
	if domain.Depth(line)+sub.Height()-1 > domain.MaxDepth {
		return domain.Node{}, domain.ErrTooDeep
	}
	order, _, renumber := u.placeAmong(others, after)
	moved.ParentID, moved.SortOrder, moved.UpdatedBy, moved.UpdatedAt = parentID, order, u.write.By, u.write.At
	return moved, u.moveSubtree(ctx, sub, moved, renumber, u.w.d.NodeWriter.MoveNode, lines)
}

// moveSubtree moves sub's node to where moved is, its checks passed,
// before running first in the step's write and write writing moved: the
// step tells each of its descendants, in a place of the same parent, name
// and order, and only the node's change is an item. A move, a relocation
// and a revert's place share it: a move writes MoveNode, the two others
// RelocateNode when moved has a new name too (M9/P2 design 3.5.1, 3.5.4).
func (u *Unit) moveSubtree(ctx context.Context, sub domain.Subtree, moved domain.Node, before func(context.Context) error,
	write func(context.Context, domain.Node) error, lines *StepLines,
) error {
	from, now := sub[0].Node.State(), moved.State()
	changes := []domain.Change{{NodeID: moved.ID, Before: &from, After: &now}}
	for _, d := range sub[1:] {
		state := d.Node.State()
		changes = append(changes, domain.Change{NodeID: d.Node.ID, Before: &state, After: &state})
	}
	step := u.step(domain.OpMove, changes...)
	step.Kinds = make(map[uuid.UUID]domain.Kind, len(sub))
	for _, n := range sub {
		step.Kinds[n.Node.ID] = n.Node.Kind
	}
	if lines != nil && u.write.Notebook.Mode == domain.ModeLLMWiki {
		if domain.SameParent(from.ParentID, now.ParentID) {
			lines.Before = lines.After
		} else if from.ParentID != nil && lines.Before == nil {
			var err error
			lines.Before, err = u.w.d.Nodes.Ancestors(ctx, moved.ID)
			if err != nil {
				return err
			}
		}
		step.Lines = lines
	}
	return u.apply(ctx, step, true, func(ctx context.Context) error {
		if err := before(ctx); err != nil {
			return err
		}
		return write(ctx, moved)
	})
}
