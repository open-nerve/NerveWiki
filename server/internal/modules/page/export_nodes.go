package page

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// ExportNodes is what the transfer module reads of a notebook's tree for an
// export (M7/P5 design 3.8), in the caller's transaction: the export's
// snapshot.
type ExportNodes interface {
	// Page is the name of the page not deleted id of notebookID; false for
	// none, or for an attachment.
	Page(ctx context.Context, notebookID, id uuid.UUID) (string, bool, error)
	// Scope is the nodes not deleted of notebookID, or of the page root and
	// its subtree when root is set: none when root is no page not deleted
	// of the notebook.
	Scope(ctx context.Context, notebookID uuid.UUID, root *uuid.UUID) ([]ExportNode, error)
	// Contents is the content of each page of ids not deleted, by id.
	Contents(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// ExportNode is a node of an export's scope. Empty tells a page without
// content; Modified is when the node was last written: its name, its
// place, or a page's content.
type ExportNode struct {
	ID        uuid.UUID
	ParentID  *uuid.UUID
	Asset     bool
	Name      string
	SortOrder float64
	Empty     bool
	Modified  time.Time
}

// NewExportNodes returns ExportNodes over pool alone.
func NewExportNodes(pool *pgxpool.Pool) ExportNodes {
	return exportNodes{store: postgresadapter.New(pool)}
}

type exportNodes struct {
	store *postgresadapter.Store
}

// sizesBatch is how many pages one read of the contents' sizes asks
// about.
const sizesBatch = 10000

func (e exportNodes) Page(ctx context.Context, notebookID, id uuid.UUID) (string, bool, error) {
	n, err := e.store.FindNodeIn(ctx, notebookID, id)
	switch {
	case errors.Is(err, app.ErrNotFound):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	return n.Name, n.Kind == domain.KindPage, nil
}

func (e exportNodes) Scope(ctx context.Context, notebookID uuid.UUID, root *uuid.UUID) ([]ExportNode, error) {
	var nodes []domain.Node
	if root == nil {
		all, err := e.store.ListNodes(ctx, notebookID)
		if err != nil {
			return nil, err
		}
		nodes = all
	} else {
		sub, err := e.store.Subtree(ctx, notebookID, *root)
		switch {
		case errors.Is(err, app.ErrNotFound):
			return nil, nil
		case err != nil:
			return nil, err
		}
		if sub[0].Node.Kind != domain.KindPage {
			return nil, nil
		}
		for _, n := range sub {
			nodes = append(nodes, n.Node)
		}
	}
	var pages []uuid.UUID
	for _, n := range nodes {
		if n.Kind == domain.KindPage {
			pages = append(pages, n.ID)
		}
	}
	sizes := make(map[uuid.UUID]postgresadapter.ContentSize, len(pages))
	for start := 0; start < len(pages); start += sizesBatch {
		got, err := e.store.ContentSizes(ctx, pages[start:min(start+sizesBatch, len(pages))])
		if err != nil {
			return nil, fmt.Errorf("page: the export's scope: %w", err)
		}
		for id, s := range got {
			sizes[id] = s
		}
	}
	out := make([]ExportNode, len(nodes))
	for i, n := range nodes {
		x := ExportNode{ID: n.ID, ParentID: n.ParentID, Asset: n.Kind == domain.KindAsset, Name: n.Name, SortOrder: n.SortOrder,
			Modified: n.UpdatedAt}
		if s, ok := sizes[n.ID]; ok && !x.Asset {
			x.Empty = s.Bytes == 0
			x.Modified = later(n.UpdatedAt, s.UpdatedAt)
		} else if !x.Asset {
			x.Empty = true
		}
		out[i] = x
	}
	return out, nil
}

func (e exportNodes) Contents(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	return e.store.Contents(ctx, ids)
}

// later is the later of a and b.
func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
