package app

import (
	"context"
	"errors"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Where an operation puts a node (M4/P1 design 3.6): its place among the
// siblings, and the checks of a parent and a title there, which creating,
// renaming and moving a node share.

// Position is where a new node goes among its siblings: after the last
// (the zero value), first, or after one of them.
type Position struct {
	place   placement
	sibling uuid.UUID
}

type placement int

const (
	last placement = iota
	first
	afterSibling
)

// First is the position before every sibling.
func First() Position { return Position{place: first} }

// After is the position right after the sibling id.
func After(id uuid.UUID) Position { return Position{place: afterSibling, sibling: id} }

// lineOf is the ancestors a child of parentID has, from the root down to
// the parent itself: none at the root. A parent that is no page of the
// notebook is 422 on parent_id.
func (u *Unit) lineOf(ctx context.Context, parentID *uuid.UUID) ([]domain.Ancestor, error) {
	if parentID == nil {
		return nil, nil
	}
	parent, err := u.w.d.Nodes.FindNodeIn(ctx, u.write.NotebookID, *parentID)
	switch {
	case errors.Is(err, ErrNotFound) || err == nil && parent.Kind != domain.KindPage:
		return nil, domain.NotAllowed("parent_id", "The parent is no page of this notebook.")
	case err != nil:
		return nil, err
	}
	ancestors, err := u.w.d.Nodes.Ancestors(ctx, parent.ID)
	if err != nil {
		return nil, err
	}
	return append(ancestors, domain.Ancestor{ID: parent.ID, Name: parent.Name}), nil
}

// slotOf is the index among siblings of the one p puts a node after: -1
// for the first place. A sibling to follow that is not among them is 422
// on after_id.
func slotOf(siblings []domain.Node, p Position) (int, error) {
	switch p.place {
	case first:
		return -1, nil
	case last:
		return len(siblings) - 1, nil
	}
	i := slices.IndexFunc(siblings, func(s domain.Node) bool { return s.ID == p.sibling })
	if i < 0 {
		return 0, domain.NotAllowed("after_id", "The page to follow is no child of the parent.")
	}
	return i, nil
}

// titleFree is 409 when a sibling other than self holds title's key. The
// unique index would refuse it too, failing the transaction: a unit of
// several operations could not go on.
func titleFree(siblings []domain.Node, title domain.Title, self uuid.UUID) error {
	for _, s := range siblings {
		if s.ID != self && s.NameKey == title.Key {
			return domain.ErrTitleTaken
		}
	}
	return nil
}
