package page

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// MaxDepth is how deep pages nest, a root at depth 1: a link's landing is
// no deeper (M6/P6 design 2).
const MaxDepth = domain.MaxDepth

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
	// All is the pages not deleted of notebookID, by id, each with its path
	// from the root (M6/P5 design 7: the link targets).
	All(ctx context.Context, notebookID uuid.UUID) ([]LinkNode, error)
	// NotebookOf is the notebook of the page not deleted id; false for no
	// such page (M6/P5 design 7: the reads by page).
	NotebookOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error)
	// Content is the content of the page not deleted id and its revision;
	// false for no such page.
	Content(ctx context.Context, id uuid.UUID) (string, int, bool, error)
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

// MaxContentBytes is the most bytes a page's content holds: M6's rewrite
// of links leaves a page it would make larger (M6/P4 review r2-2).
const MaxContentBytes = domain.MaxContentBytes

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

func (l linkTargets) All(ctx context.Context, notebookID uuid.UUID) ([]LinkNode, error) {
	nodes, err := l.store.ListNodes(ctx, notebookID)
	if err != nil {
		return nil, fmt.Errorf("page: the pages of %s: %w", notebookID, err)
	}
	byID := make(map[uuid.UUID]domain.Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	var out []LinkNode
	for _, n := range nodes {
		if n.Kind != domain.KindPage {
			continue
		}
		path := []LinkStep{{ID: n.ID, Key: n.NameKey, Name: n.Name}}
		for at := n; at.ParentID != nil; {
			parent, ok := byID[*at.ParentID]
			if !ok || len(path) == domain.MaxDepth {
				return nil, fmt.Errorf("page: the path of %s in %s has no root within a page's depth", n.ID, notebookID)
			}
			path = append(path, LinkStep{ID: parent.ID, Key: parent.NameKey, Name: parent.Name})
			at = parent
		}
		slices.Reverse(path)
		out = append(out, LinkNode{ID: n.ID, Path: path})
	}
	slices.SortFunc(out, func(a, b LinkNode) int { return a.ID.Compare(b.ID) })
	return out, nil
}

func (l linkTargets) NotebookOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	n, err := l.store.FindNode(ctx, id)
	switch {
	case errors.Is(err, app.ErrNotFound):
		return uuid.UUID{}, false, nil
	case err != nil:
		return uuid.UUID{}, false, fmt.Errorf("page: the notebook of %s: %w", id, err)
	}
	if n.Kind != domain.KindPage {
		return uuid.UUID{}, false, nil
	}
	return n.NotebookID, true, nil
}

func (l linkTargets) Content(ctx context.Context, id uuid.UUID) (string, int, bool, error) {
	c, err := l.store.PageContent(ctx, id)
	switch {
	case errors.Is(err, app.ErrNotFound):
		return "", 0, false, nil
	case err != nil:
		return "", 0, false, fmt.Errorf("page: the content of %s: %w", id, err)
	}
	return c.Content, c.Revision, true, nil
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
