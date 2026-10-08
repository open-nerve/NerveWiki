package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// NewAsset is an attachment to create (M7/P2 design 3.3): under ParentID
// (nil: the notebook's root), named Name, its file Meta, decided on Action
// from Client.
type NewAsset struct {
	NotebookID uuid.UUID
	ParentID   *uuid.UUID
	Name       string
	Meta       AssetMeta
	Action     shared.Action
	Client     domain.Client
}

// AssetWrites creates the attachments' nodes for the module that keeps
// their files: each in a unit that changes the tree, which the creation
// of the file's row joins.
type AssetWrites struct {
	writer *Writer
	nodes  Nodes
}

// NewAssetWrites returns them, writing through writer.
func NewAssetWrites(writer *Writer, nodes Nodes) *AssetWrites {
	return &AssetWrites{writer: writer, nodes: nodes}
}

// spec is a's unit: one that changes the tree, deciding on a's action, in
// which a notebook the caller cannot see is notebook.not_found.
func (w *AssetWrites) spec(a NewAsset) UnitSpec {
	return UnitSpec{NotebookID: a.NotebookID, Action: a.Action, Tree: true, Client: a.Client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotebookNotFound}
}

// Check decides as CreateAsset would, unlocked, for an upload to answer
// before it reads the file: notebook.not_found, forbidden, then the name's
// rules and a parent that is no page of the notebook (validation_failed),
// and page.title_taken for a name a sibling holds now. CreateAsset decides
// again under its locks.
func (w *AssetWrites) Check(ctx context.Context, a NewAsset) error {
	if err := w.writer.Allowed(ctx, w.spec(a)); err != nil {
		return err
	}
	_, _, err := assetPlace(ctx, w.nodes, a.NotebookID, a.ParentID, a.Name)
	return err
}

// CreateAsset creates a's node in a unit (Unit.CreateAsset); after runs
// in the unit's transaction once the node is written, before the
// observers, and its error rolls the whole unit back and is returned.
func (w *AssetWrites) CreateAsset(ctx context.Context, a NewAsset, after func(ctx context.Context, n domain.Node) error) (
	domain.Node, error,
) {
	var created domain.Node
	_, err := w.writer.Run(ctx, w.spec(a), func(ctx context.Context, u *Unit) error {
		n, err := u.CreateAsset(ctx, AssetDraft{ParentID: a.ParentID, Name: a.Name, Meta: a.Meta})
		if err != nil {
			return err
		}
		if err := after(ctx, n); err != nil {
			return err
		}
		created = n
		return nil
	})
	if err != nil {
		return domain.Node{}, err
	}
	return created, nil
}
