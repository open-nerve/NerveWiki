package page

import (
	"context"
	"fmt"
	"slices"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// LinkTargets is what the link index reads of a notebook's pages (M6/P3
// design 3.3), in the caller's transaction: bootstrap wires it to the
// linking module.
type LinkTargets interface {
	// ByKeys is the pages not deleted of notebookID whose title key is one
	// of keys, each with its path from the root.
	ByKeys(ctx context.Context, notebookID uuid.UUID, keys []string) ([]LinkNode, error)
	// Paths is the pages not deleted of notebookID among ids, each with its
	// path from the root.
	Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]LinkNode, error)
	// Subtree is the page id of notebookID and the pages not deleted under
	// it, each its id, title key and name.
	Subtree(ctx context.Context, notebookID, id uuid.UUID) ([]LinkStep, error)
	// PageIDs is the pages not deleted of notebookID, by id (nervewiki
	// reindex).
	PageIDs(ctx context.Context, notebookID uuid.UUID) ([]uuid.UUID, error)
	// Content is the content of the page not deleted id and its revision.
	Content(ctx context.Context, id uuid.UUID) (string, int, error)
	// Rekey takes the title keys of notebookID's nodes not deleted anew
	// from their names by the current Unicode data. When siblings would
	// share a key it changes none and returns them, a group a key, each by
	// id.
	Rekey(ctx context.Context, notebookID uuid.UUID) ([][]NamedNode, error)
}

// NamedNode is a node by its id and name.
type NamedNode struct {
	ID   uuid.UUID
	Name string
}

// LinkNode is a page with its path from the root, itself last.
type LinkNode struct {
	ID   uuid.UUID
	Path []LinkStep
}

// LinkStep is a page on a path: its id, title key and name.
type LinkStep struct {
	ID   uuid.UUID
	Key  string
	Name string
}

// NewLinkTargets returns LinkTargets over pool alone.
func NewLinkTargets(pool *pgxpool.Pool) LinkTargets {
	return linkTargets{store: postgresadapter.New(pool)}
}

type linkTargets struct {
	store *postgresadapter.Store
}

func (l linkTargets) ByKeys(ctx context.Context, notebookID uuid.UUID, keys []string) ([]LinkNode, error) {
	paths, err := l.store.LinkTargetsByKeys(ctx, notebookID, keys)
	if err != nil {
		return nil, fmt.Errorf("page: link targets of %s: %w", notebookID, err)
	}
	return linkNodes(paths), nil
}

func (l linkTargets) Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]LinkNode, error) {
	paths, err := l.store.LinkTargetsByIDs(ctx, notebookID, ids)
	if err != nil {
		return nil, fmt.Errorf("page: link targets of %s: %w", notebookID, err)
	}
	return linkNodes(paths), nil
}

func (l linkTargets) Subtree(ctx context.Context, notebookID, id uuid.UUID) ([]LinkStep, error) {
	sub, err := l.store.Subtree(ctx, notebookID, id)
	if err != nil {
		return nil, fmt.Errorf("page: the subtree of %s: %w", id, err)
	}
	var out []LinkStep
	for _, n := range sub {
		if n.Node.Kind == domain.KindPage {
			out = append(out, LinkStep{ID: n.Node.ID, Key: n.Node.NameKey, Name: n.Node.Name})
		}
	}
	return out, nil
}

func linkNodes(paths []postgresadapter.LinkPath) []LinkNode {
	out := make([]LinkNode, len(paths))
	for i, p := range paths {
		steps := make([]LinkStep, len(p.Steps))
		for j, s := range p.Steps {
			steps[j] = LinkStep(s)
		}
		out[i] = LinkNode{ID: p.ID, Path: steps}
	}
	return out
}

func (l linkTargets) PageIDs(ctx context.Context, notebookID uuid.UUID) ([]uuid.UUID, error) {
	nodes, err := l.store.ListNodes(ctx, notebookID)
	if err != nil {
		return nil, fmt.Errorf("page: the pages of %s: %w", notebookID, err)
	}
	var out []uuid.UUID
	for _, n := range nodes {
		if n.Kind == domain.KindPage {
			out = append(out, n.ID)
		}
	}
	slices.SortFunc(out, uuid.UUID.Compare)
	return out, nil
}

func (l linkTargets) Content(ctx context.Context, id uuid.UUID) (string, int, error) {
	c, err := l.store.PageContent(ctx, id)
	if err != nil {
		return "", 0, fmt.Errorf("page: the content of %s: %w", id, err)
	}
	return c.Content, c.Revision, nil
}

func (l linkTargets) Rekey(ctx context.Context, notebookID uuid.UUID) ([][]NamedNode, error) {
	nodes, err := l.store.ListNodes(ctx, notebookID)
	if err != nil {
		return nil, fmt.Errorf("page: the nodes of %s: %w", notebookID, err)
	}
	slices.SortFunc(nodes, func(a, b domain.Node) int { return a.ID.Compare(b.ID) })
	changed, clashes := domain.Rekey(nodes)
	if len(clashes) > 0 {
		out := make([][]NamedNode, len(clashes))
		for i, c := range clashes {
			for _, n := range c {
				out[i] = append(out[i], NamedNode{ID: n.ID, Name: n.Name})
			}
		}
		return out, nil
	}
	if len(changed) == 0 {
		return nil, nil
	}
	if err := l.store.SetNameKeys(ctx, changed); err != nil {
		return nil, fmt.Errorf("page: rekey %s: %w", notebookID, err)
	}
	return nil, nil
}
