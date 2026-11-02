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
	// Pages reads live same-notebook pages with their stored title and real
	// parent from path steps, then their content change (PageStates): two statements.
	Pages(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]PagePath, error)
	// ChildPages is the pages alive of notebookID whose parent is parent,
	// the root's for uuid.UUID{}, in no order.
	ChildPages(ctx context.Context, notebookID, parent uuid.UUID) ([]uuid.UUID, error)
	// ChildPagesAt takes nil for the root; a zero UUID still names a parent.
	ChildPagesAt(ctx context.Context, notebookID uuid.UUID, parent *uuid.UUID) ([]uuid.UUID, error)
}

// PagePath is a live page's path, stored title, real parent and content change.
type PagePath struct {
	ParentID  *uuid.UUID
	Title     string
	Path      string
	ChangedAt time.Time
}

// NewNodePaths returns NodePaths over pool with the notebook state read (M9/P4 design 3.9.1).
func NewNodePaths(pool *pgxpool.Pool, notebooks NotebookStates) NodePaths {
	store := postgresadapter.New(pool)
	return nodePaths{paths: app.NewNodePaths(store, notebooks), store: store}
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
	paths, err := n.store.LinkTargetsByIDs(ctx, notebookID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]PagePath, len(paths))
	alive := make([]uuid.UUID, 0, len(paths))
	for _, p := range paths {
		if p.Asset {
			continue
		}
		names := make([]string, len(p.Steps))
		for i, step := range p.Steps {
			names[i] = step.Name
		}
		last := len(p.Steps) - 1
		meta := PagePath{Path: domain.PathOfNames(names, domain.KindPage), Title: p.Steps[last].Name}
		if last > 0 {
			parent := p.Steps[last-1].ID
			meta.ParentID = &parent
		}
		out[p.ID] = meta
		alive = append(alive, p.ID)
	}
	if len(alive) == 0 {
		return out, nil
	}
	states, err := n.store.PageStates(ctx, alive)
	if err != nil {
		return nil, fmt.Errorf("page: the pages' states in %s: %w", notebookID, err)
	}
	for id, meta := range out {
		state, ok := states[id]
		if !ok {
			delete(out, id)
			continue
		}
		meta.ChangedAt = state.UpdatedAt
		out[id] = meta
	}
	return out, nil
}

func (n nodePaths) ChildPages(ctx context.Context, notebookID, parent uuid.UUID) ([]uuid.UUID, error) {
	return n.store.ChildPageIDs(ctx, notebookID, parent)
}

func (n nodePaths) ChildPagesAt(ctx context.Context, notebookID uuid.UUID, parent *uuid.UUID) ([]uuid.UUID, error) {
	return n.store.IndexChildPagesAt(ctx, notebookID, parent)
}
