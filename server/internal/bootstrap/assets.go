package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// The attachments' parts (M7/P2 design 3.3): the asset module creates the
// nodes through the page module's TreeWrites and reads them through its
// AssetNodes, and the two do not import each other, so their values meet
// here.

// assetTree is the page module's writes of attachments' nodes, as the
// asset module calls them.
type assetTree struct {
	page page.TreeWrites
}

func (t assetTree) Check(ctx context.Context, n asset.NewNode) error {
	return t.page.Check(ctx, pageAsset(n))
}

func (t assetTree) CreateAsset(ctx context.Context, n asset.NewNode, after func(ctx context.Context, n asset.Node) error) (asset.Node, error) {
	created, err := t.page.CreateAsset(ctx, pageAsset(n), func(ctx context.Context, p page.NodeInfo) error {
		return after(ctx, assetNode(p))
	})
	return assetNode(created), err
}

// assetNodes is the page module's reads of the trees, as the asset module
// calls them.
type assetNodes struct {
	page page.AssetNodes
}

func (a assetNodes) Node(ctx context.Context, id uuid.UUID) (asset.Node, bool, error) {
	n, ok, err := a.page.Node(ctx, id)
	return assetNode(n), ok, err
}

func (a assetNodes) Parent(ctx context.Context, notebookID, parentID uuid.UUID) (bool, error) {
	return a.page.Parent(ctx, notebookID, parentID)
}

func (a assetNodes) Assets(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID, after *asset.Cursor, limit int) (
	[]asset.Node, error,
) {
	var cursor *page.AssetCursor
	if after != nil {
		cursor = &page.AssetCursor{NameKey: after.NameKey, ID: after.ID}
	}
	nodes, err := a.page.Assets(ctx, notebookID, parentID, cursor, limit)
	out := make([]asset.Node, len(nodes))
	for i, n := range nodes {
		out[i] = assetNode(n)
	}
	return out, err
}

// pageAsset is the asset module's new node as the page module takes it.
func pageAsset(n asset.NewNode) page.NewAsset {
	return page.NewAsset{
		NotebookID: n.NotebookID, ParentID: n.ParentID, Name: n.Name, Action: n.Action, Client: page.Client(n.Client),
		Meta: page.AssetMeta{MIME: n.Meta.MIME, Bytes: n.Meta.Bytes, SHA256: n.Meta.SHA256},
	}
}

// assetNode is the page module's node as the asset module reads it.
func assetNode(n page.NodeInfo) asset.Node {
	return asset.Node{
		ID: n.ID, NotebookID: n.NotebookID, ParentID: n.ParentID, Asset: n.Asset, Name: n.Name, NameKey: n.NameKey,
		CreatedBy: n.CreatedBy, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt,
	}
}
