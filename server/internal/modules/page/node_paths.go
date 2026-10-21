package page

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	mcpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/mcp"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// NodePaths is what another module reads of a notebook's paths (M9 design
// 4.1, 4.8): linking's backlinks and index, on the pool alone. It decides
// nothing: its callers decided the notebook.
type NodePaths interface {
	// Check is the path's 422 at field, before any read, as page's tools
	// check theirs (domain.ParsePath: the segments' check, M9 design 4.8).
	Check(field, path string) error
	// FindPage is the id of the page at path in notebookID, a point lookup
	// a segment (app.NodePaths.Find): page.not_found in page's tools' words,
	// its hint while the answer holds it, for none, and for an attachment's
	// path, which names no page.
	FindPage(ctx context.Context, notebookID uuid.UUID, field, path string) (uuid.UUID, error)
	// Paths is the paths of ids' nodes alive in notebookID, by id
	// (app.NodePaths.Paths: one statement, PathStore.PathsOf).
	Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error)
	// Pages is Paths of ids' pages, each with when its content last
	// changed (PageStates): two statements.
	Pages(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]PagePath, error)
	// ChildPages is the pages alive of notebookID whose parent is parent,
	// the root's for uuid.UUID{}, in no order.
	ChildPages(ctx context.Context, notebookID, parent uuid.UUID) ([]uuid.UUID, error)
}

// PagePath is a page's path and its content's last change.
type PagePath struct {
	Path      string
	ChangedAt time.Time
}

// NewNodePaths returns NodePaths over pool alone (M9/P4 design 3.9.1).
func NewNodePaths(pool *pgxpool.Pool) NodePaths {
	store := postgresadapter.New(pool)
	return nodePaths{paths: app.NewNodePaths(store), store: store}
}

type nodePaths struct {
	paths app.NodePaths
	store *postgresadapter.Store
}

func (n nodePaths) Check(field, path string) error {
	_, err := domain.ParsePath(field, path, false)
	return err
}

func (n nodePaths) FindPage(ctx context.Context, notebookID uuid.UUID, field, path string) (uuid.UUID, error) {
	p, err := domain.ParsePath(field, path, false)
	if err != nil {
		return uuid.UUID{}, err
	}
	found, err := n.paths.Find(ctx, notebookID, p)
	switch {
	case err != nil:
		return uuid.UUID{}, mcpadapter.PathErr(err)
	case found.Node.Kind != domain.KindPage:
		return uuid.UUID{}, mcpadapter.NotAPage(path)
	}
	return found.Node.ID, nil
}

func (n nodePaths) Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	paths, err := n.paths.Paths(ctx, notebookID, ids)
	if err != nil {
		return nil, fmt.Errorf("page: the paths in %s: %w", notebookID, err)
	}
	return paths, nil
}

func (n nodePaths) Pages(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]PagePath, error) {
	paths, err := n.Paths(ctx, notebookID, ids)
	if err != nil {
		return nil, err
	}
	alive := make([]uuid.UUID, 0, len(paths))
	for _, id := range ids {
		if _, ok := paths[id]; ok {
			alive = append(alive, id)
		}
	}
	out := make(map[uuid.UUID]PagePath, len(alive))
	if len(alive) == 0 {
		return out, nil
	}
	// An attachment has no content: its path has no state.
	states, err := n.store.PageStates(ctx, alive)
	if err != nil {
		return nil, fmt.Errorf("page: the pages' states in %s: %w", notebookID, err)
	}
	for id, s := range states {
		if path, ok := paths[id]; ok {
			out[id] = PagePath{Path: path, ChangedAt: s.UpdatedAt}
		}
	}
	return out, nil
}

func (n nodePaths) ChildPages(ctx context.Context, notebookID, parent uuid.UUID) ([]uuid.UUID, error) {
	return n.store.ChildPageIDs(ctx, notebookID, parent)
}
