package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Creating an attachment in a unit (M7/P2 design 3.3).

// AssetDraft is a new attachment: under ParentID (nil: at the notebook's
// root), named Name, its file Meta.
type AssetDraft struct {
	ParentID *uuid.UUID
	Name     string
	Meta     AssetMeta
}

// CreateAsset creates the attachment d, last among its siblings: 422 for a
// name that breaks an attachment's rules or a parent that is no page of
// the notebook, then 409 for a name a sibling holds. An attachment is no
// level of the tree: a page as deep as pages go holds them. The node has
// no content: its change has no revision, and the guards see its file in
// the step.
func (u *Unit) CreateAsset(ctx context.Context, d AssetDraft) (domain.Node, error) {
	title, siblings, err := assetPlace(ctx, u.w.d.Nodes, u.write.NotebookID, d.ParentID, d.Name)
	if err != nil {
		return domain.Node{}, err
	}
	order, renumber := u.placeAmong(siblings, len(siblings)-1)
	n := domain.Node{
		ID: uuid.NewV7(), NotebookID: u.write.NotebookID, ParentID: d.ParentID, Kind: domain.KindAsset, Name: title.Name,
		NameKey: title.Key, SortOrder: order, CreatedBy: u.write.By, UpdatedBy: u.write.By, CreatedAt: u.write.At, UpdatedAt: u.write.At,
	}
	state := n.State()
	step := u.step(domain.OpCreate, domain.Change{NodeID: n.ID, After: &state})
	meta := d.Meta
	step.Asset = &meta
	err = u.apply(ctx, step, true, func(ctx context.Context) error {
		if err := renumber(ctx); err != nil {
			return err
		}
		return u.w.d.NodeWriter.CreateNode(ctx, n)
	})
	return n, err
}

// assetPlace checks an attachment named name under parentID of the
// notebook, as CreateAsset does, in a unit or unlocked: its name, its
// parent, a sibling holding the name. It answers the name checked and the
// siblings, in order.
func assetPlace(ctx context.Context, nodes Nodes, notebookID uuid.UUID, parentID *uuid.UUID, name string) (
	domain.Title, []domain.Node, error,
) {
	title, err := domain.CheckAssetName("name", name)
	if err != nil {
		return domain.Title{}, nil, err
	}
	if parentID != nil {
		if _, err := parentPage(ctx, nodes, notebookID, *parentID); err != nil {
			return domain.Title{}, nil, err
		}
	}
	siblings, err := nodes.Children(ctx, notebookID, parentID)
	if err != nil {
		return domain.Title{}, nil, err
	}
	if err := titleFree(siblings, title, uuid.UUID{}); err != nil {
		return domain.Title{}, nil, err
	}
	return title, siblings, nil
}
