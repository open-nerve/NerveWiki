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
// subtree, a title a new sibling holds, or a subtree that would reach
// deeper than domain.MaxDepth. A move to where the node is writes nothing.
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
	sub, err := u.w.d.Nodes.Subtree(ctx, u.write.NotebookID, n.ID)
	if err != nil {
		return domain.Node{}, err
	}
	if parentID != nil && sub.Holds(*parentID) {
		return domain.Node{}, domain.ErrCycle
	}
	if !sameParent {
		if err := titleFree(others, domain.Title{Name: n.Name, Key: n.NameKey}, n.ID); err != nil {
			return domain.Node{}, err
		}
	}
	// The subtree's deepest node lands Height - 1 levels below the node.
	if domain.Depth(line)+sub.Height()-1 > domain.MaxDepth {
		return domain.Node{}, domain.ErrTooDeep
	}
	orders := make([]float64, len(others))
	for i, s := range others {
		orders[i] = s.SortOrder
	}
	order, renumbered := domain.Place(orders, after)
	moved := n
	moved.ParentID, moved.SortOrder, moved.UpdatedBy, moved.UpdatedAt = parentID, order, u.write.By, u.write.At
	before, now := n.State(), moved.State()
	changes := []domain.Change{{NodeID: n.ID, Before: &before, After: &now}}
	for _, d := range sub[1:] {
		state := d.Node.State()
		changes = append(changes, domain.Change{NodeID: d.Node.ID, Before: &state, After: &state})
	}
	err = u.apply(ctx, u.step(domain.OpMove, changes...), true, func(ctx context.Context) error {
		// Renumbering keeps the siblings' order: it moves no one.
		for i, o := range renumbered {
			if err := u.w.d.NodeWriter.SetSortOrder(ctx, others[i].ID, o); err != nil {
				return err
			}
		}
		return u.w.d.NodeWriter.MoveNode(ctx, moved)
	})
	return moved, err
}
