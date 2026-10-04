package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Creating a page in a unit (M4/P1 design 3.6).

// PageDraft is a new page: under ParentID (nil: at the notebook's root),
// with Title, at Position among its siblings, holding Content. A unit
// takes Content checked, and Facts its facts, both before its
// transaction.
type PageDraft struct {
	ParentID *uuid.UUID
	Title    string
	Position Position
	Content  string
	Facts    Facts
}

// CreatePage creates the page d at revision 1 of its content. 422 for a title
// that breaks the rules, a parent that is no page of the notebook, or a
// sibling to follow that is no child of the parent; then 409 for a title a
// sibling holds, or a page deeper than domain.MaxDepth.
func (u *Unit) CreatePage(ctx context.Context, d PageDraft) (domain.Node, error) {
	title, err := domain.CheckTitle("title", d.Title)
	if err != nil {
		return domain.Node{}, err
	}
	ancestors, err := u.lineOf(ctx, d.ParentID)
	if err != nil {
		return domain.Node{}, err
	}
	siblings, err := u.w.d.Nodes.Children(ctx, u.write.NotebookID, d.ParentID)
	if err != nil {
		return domain.Node{}, err
	}
	after, err := slotOf(siblings, d.Position)
	if err != nil {
		return domain.Node{}, err
	}
	if err := titleFree(siblings, title, uuid.UUID{}); err != nil {
		return domain.Node{}, err
	}
	if domain.Depth(ancestors) > domain.MaxDepth {
		return domain.Node{}, domain.ErrTooDeep
	}
	order, renumber := u.placeAmong(siblings, after)
	n := domain.Node{
		ID: uuid.NewV7(), NotebookID: u.write.NotebookID, ParentID: d.ParentID, Kind: domain.KindPage, Name: title.Name,
		NameKey: title.Key, SortOrder: order, CreatedBy: u.write.By, UpdatedBy: u.write.By, CreatedAt: u.write.At, UpdatedAt: u.write.At,
	}
	state := n.State()
	content := u.content(n.ID, d.Content, 1)
	step := u.step(domain.OpCreate, domain.Change{NodeID: n.ID, After: &state, Revision: content.Revision, Facts: d.Facts})
	err = u.apply(ctx, step, true, func(ctx context.Context) error {
		if err := renumber(ctx); err != nil {
			return err
		}
		if err := u.w.d.NodeWriter.CreateNode(ctx, n); err != nil {
			return err
		}
		if err := u.w.d.NodeWriter.CreateContent(ctx, content); err != nil {
			return err
		}
		return u.recordRevision(ctx, content, nil)
	})
	return n, err
}
