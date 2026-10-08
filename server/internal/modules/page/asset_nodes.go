package page

import (
	"context"
	"errors"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// AssetNodes is what the asset module reads of a notebook's tree (M7/P2
// design 3.3), in the caller's transaction when it has one.
type AssetNodes interface {
	// Node is the node id not deleted, of whatever kind; false for none.
	Node(ctx context.Context, id uuid.UUID) (NodeInfo, bool, error)
	// Parent reports whether parentID is a page not deleted of notebookID.
	Parent(ctx context.Context, notebookID, parentID uuid.UUID) (bool, error)
	// Assets is the attachments not deleted under parentID (nil: the root)
	// of notebookID, by title key and id, after the cursor's when it is
	// set, at most limit.
	Assets(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID, after *AssetCursor, limit int) ([]NodeInfo, error)
}

// AssetCursor is where a list of attachments goes on: after the node of
// this title key and id.
type AssetCursor struct {
	NameKey string
	ID      uuid.UUID
}

// NewAssetNodes returns AssetNodes over pool alone.
func NewAssetNodes(pool *pgxpool.Pool) AssetNodes {
	return assetNodes{store: postgresadapter.New(pool)}
}

type assetNodes struct {
	store *postgresadapter.Store
}

func (a assetNodes) Node(ctx context.Context, id uuid.UUID) (NodeInfo, bool, error) {
	n, err := a.store.FindNode(ctx, id)
	switch {
	case errors.Is(err, app.ErrNotFound):
		return NodeInfo{}, false, nil
	case err != nil:
		return NodeInfo{}, false, err
	}
	return nodeInfo(n), true, nil
}

func (a assetNodes) Parent(ctx context.Context, notebookID, parentID uuid.UUID) (bool, error) {
	n, err := a.store.FindNodeIn(ctx, notebookID, parentID)
	switch {
	case errors.Is(err, app.ErrNotFound):
		return false, nil
	case err != nil:
		return false, err
	}
	return n.Kind == domain.KindPage, nil
}

func (a assetNodes) Assets(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID, after *AssetCursor, limit int) (
	[]NodeInfo, error,
) {
	var cursor *postgresadapter.NameCursor
	if after != nil {
		cursor = &postgresadapter.NameCursor{Key: after.NameKey, ID: after.ID}
	}
	nodes, err := a.store.AssetsUnder(ctx, notebookID, parentID, cursor, limit)
	if err != nil {
		return nil, err
	}
	out := make([]NodeInfo, len(nodes))
	for i, n := range nodes {
		out[i] = nodeInfo(n)
	}
	return out, nil
}
