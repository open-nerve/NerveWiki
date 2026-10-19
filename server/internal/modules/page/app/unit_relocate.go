package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Relocating a node in a unit (M9/P2 design 3.5.1): MCP's move, a new
// parent and a new name in one step.

// Placement is where Relocate puts a node: under ParentID (nil: the
// notebook's root), named Name; Field is the field a 422 on the name
// names (move's "to"; P3's ops[i].to).
type Placement struct {
	ParentID *uuid.UUID
	Name     string
	Field    string
}

// Relocate moves the node id under p's parent and names it p's name, in
// one step and one statement (M9 design 4.10). Under another parent it is
// an OpMove: the node goes last among its new siblings, its descendants
// with it, and only its change is an item. Under its parent it is an
// OpRename and keeps its place. 404 for a node not in the notebook; 422
// for a name against its kind's rules (p.Field) or a parent that is no
// page of the notebook (parent_id); then 409 for a parent in the node's
// own subtree, a name a sibling holds there (*domain.PathTaken's holder
// named from the siblings it read), or a subtree deeper than
// domain.MaxDepth there. A node there and so named already writes
// nothing; a name that differs in case alone is written, as a rename's
// is.
func (u *Unit) Relocate(ctx context.Context, id uuid.UUID, p Placement) (domain.Node, error) {
	n, err := u.w.d.Nodes.FindNodeIn(ctx, u.write.NotebookID, id)
	if err != nil {
		return domain.Node{}, found(err, domain.ErrNotFound)
	}
	title, err := nameOf(n, p.Name, p.Field)
	if err != nil {
		return domain.Node{}, err
	}
	if domain.SameParent(n.ParentID, p.ParentID) {
		return u.renameTo(ctx, n, title, true, pathFree, u.w.d.NodeWriter.RelocateNode)
	}
	line, err := u.lineOf(ctx, p.ParentID)
	if err != nil {
		return domain.Node{}, err
	}
	siblings, err := u.w.d.Nodes.Children(ctx, u.write.NotebookID, p.ParentID)
	if err != nil {
		return domain.Node{}, err
	}
	sub, err := u.w.d.Nodes.Subtree(ctx, u.write.NotebookID, n.ID)
	if err != nil {
		return domain.Node{}, err
	}
	// The 409s in Move's order: the cycle, the name, the depth.
	if p.ParentID != nil && sub.Holds(*p.ParentID) {
		return domain.Node{}, domain.ErrCycle
	}
	if err := pathFree(siblings, title, n.ID); err != nil {
		return domain.Node{}, err
	}
	// The subtree's deepest node lands Height - 1 levels below the node.
	if domain.Depth(line)+sub.Height()-1 > domain.MaxDepth {
		return domain.Node{}, domain.ErrTooDeep
	}
	order, _, renumber := u.placeAmong(siblings, len(siblings)-1)
	moved := n
	moved.ParentID, moved.SortOrder, moved.Name, moved.NameKey = p.ParentID, order, title.Name, title.Key
	moved.UpdatedBy, moved.UpdatedAt = u.write.By, u.write.At
	return moved, u.moveSubtree(ctx, sub, moved, renumber, u.w.d.NodeWriter.RelocateNode)
}
