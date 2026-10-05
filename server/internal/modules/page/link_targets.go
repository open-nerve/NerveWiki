package page

import (
	"context"
	"fmt"
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
	// it, each its id and title key.
	Subtree(ctx context.Context, notebookID, id uuid.UUID) ([]LinkStep, error)
}

// LinkNode is a page with its path from the root, itself last.
type LinkNode struct {
	ID   uuid.UUID
	Path []LinkStep
}

// LinkStep is a page on a path: its id and title key.
type LinkStep struct {
	ID  uuid.UUID
	Key string
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
			out = append(out, LinkStep{ID: n.Node.ID, Key: n.Node.NameKey})
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
