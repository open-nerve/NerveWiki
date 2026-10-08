package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// The attachments' parts (M7/P2 design 3.3, 3.8): the asset module creates
// the nodes through the page module's TreeWrites and reads them through
// its AssetNodes; it follows the page module's units and the notebook
// module's deletions, and tells the notebooks' activity. The modules do
// not import each other, so their values meet here.

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

// assetObserver is the attachments' observer of the page module's units:
// the nodes a unit deleted, of whatever kind, a subtree's each node.
type assetObserver struct {
	asset asset.PageObserver
}

func (o assetObserver) PagesChanged(ctx context.Context, e page.Event) error {
	var deleted []uuid.UUID
	for _, c := range e.Changes {
		if c.After == nil {
			deleted = append(deleted, c.NodeID)
		}
	}
	return o.asset.NodesDeleted(ctx, deleted, e.At)
}

// assetNotebookDeletion is the attachments' part in a notebook's deletion
// as the notebook module calls it.
type assetNotebookDeletion struct {
	asset asset.NotebookDeletion
}

func (d assetNotebookDeletion) NotebookDeleted(ctx context.Context, x notebook.NotebookDeletion) error {
	return d.asset.NotebooksDeleted(ctx, x.NotebookIDs, x.At)
}

// assetActivity is the attachments' part in notebooks' activity as the
// notebook module reads it: their bytes; their uploads are writes of the
// tree, which pageActivity tells.
type assetActivity struct {
	asset asset.Activities
}

func (a assetActivity) NotebookActivities(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]notebook.NotebookActivity, error) {
	got, err := a.asset.NotebookActivities(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]notebook.NotebookActivity, len(got))
	for id, x := range got {
		out[id] = notebook.NotebookActivity{Bytes: x.Bytes}
	}
	return out, nil
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
