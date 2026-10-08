// Package postgresadapter is the page module's repository adapter: sqlc
// queries (queries/, generated into gen/) over the transaction that the
// context carries, or the pool.
package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// siblingNameKey is the unique index on siblings' name keys.
const siblingNameKey = "nodes_notebook_id_parent_id_name_key_idx"

// Store implements the app's repository ports.
type Store struct {
	pool *pgxpool.Pool
}

// New returns the store over pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// queries runs in the context's transaction when there is one.
func (s *Store) queries(ctx context.Context) *gen.Queries {
	return gen.New(postgres.DB(ctx, s.pool))
}

// planned is queries with each statement planned with its arguments
// (postgres.Planned): for those a plan for any arguments makes much slower.
func (s *Store) planned(ctx context.Context) *gen.Queries {
	return gen.New(postgres.Planned(postgres.DB(ctx, s.pool)))
}

// uniqueViolation reports whether err broke the unique constraint or index
// name.
func uniqueViolation(err error, name string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == name
}

// notFound turns no row into app.ErrNotFound.
func notFound(what string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	return fmt.Errorf("%s: %w", what, err)
}

// nodeOf is a row of the queries that read a node's columns: they read the
// same, so their rows convert to this one.
func nodeOf(r gen.FindNodeRow) domain.Node {
	return domain.Node{
		ID: r.ID, NotebookID: r.NotebookID, ParentID: r.ParentID, Kind: domain.Kind(r.Kind), Name: r.Name,
		NameKey: r.NameKey, SortOrder: r.SortOrder, CreatedBy: r.CreatedByID, UpdatedBy: r.UpdatedByID,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// FindNode implements app.Nodes.
func (s *Store) FindNode(ctx context.Context, id uuid.UUID) (domain.Node, error) {
	row, err := s.queries(ctx).FindNode(ctx, id)
	if err != nil {
		return domain.Node{}, notFound("find node", err)
	}
	return nodeOf(row), nil
}

// FindNodeIn implements app.Nodes.
func (s *Store) FindNodeIn(ctx context.Context, notebookID, id uuid.UUID) (domain.Node, error) {
	row, err := s.queries(ctx).FindNodeIn(ctx, gen.FindNodeInParams{ID: id, NotebookID: notebookID})
	if err != nil {
		return domain.Node{}, notFound("find node in notebook", err)
	}
	return nodeOf(gen.FindNodeRow(row)), nil
}

// ListNodes implements app.Nodes.
func (s *Store) ListNodes(ctx context.Context, notebookID uuid.UUID) ([]domain.Node, error) {
	rows, err := s.queries(ctx).ListNodes(ctx, notebookID)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	out := make([]domain.Node, len(rows))
	for i, r := range rows {
		out[i] = nodeOf(gen.FindNodeRow(r))
	}
	return out, nil
}

// Children implements app.Nodes.
func (s *Store) Children(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID) ([]domain.Node, error) {
	rows, err := s.queries(ctx).Children(ctx, gen.ChildrenParams{NotebookID: notebookID, ParentID: parentID})
	if err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}
	out := make([]domain.Node, len(rows))
	for i, r := range rows {
		out[i] = nodeOf(gen.FindNodeRow(r))
	}
	return out, nil
}

// NameCursor is where a list by title key and id goes on: after the node
// of this key and id.
type NameCursor struct {
	Key string
	ID  uuid.UUID
}

// AssetsUnder is the attachments not deleted under parentID (nil: the
// root) of notebookID, by title key and id, after after when it is set, at
// most limit.
func (s *Store) AssetsUnder(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID, after *NameCursor, limit int) (
	[]domain.Node, error,
) {
	params := gen.AssetsUnderParams{NotebookID: notebookID, ParentID: parentID, MaxRows: int32(limit)}
	if after != nil {
		params.AfterKey, params.AfterID = &after.Key, &after.ID
	}
	rows, err := s.queries(ctx).AssetsUnder(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	out := make([]domain.Node, len(rows))
	for i, r := range rows {
		out[i] = nodeOf(gen.FindNodeRow(r))
	}
	return out, nil
}

// Ancestors implements app.Nodes. A chain that does not reach a root is an
// error: only a defect makes one.
func (s *Store) Ancestors(ctx context.Context, id uuid.UUID) ([]domain.Ancestor, error) {
	rows, err := s.queries(ctx).Ancestors(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("ancestors: %w", err)
	}
	if len(rows) > 0 && rows[len(rows)-1].ParentID != nil {
		return nil, fmt.Errorf("ancestors of node %s: the chain does not reach a root", id)
	}
	out := make([]domain.Ancestor, len(rows))
	for i, r := range rows {
		out[i] = domain.Ancestor{ID: r.ID, Name: r.Name}
	}
	slices.Reverse(out)
	return out, nil
}

// subtreeLevels bounds Subtree's levels: only a defect could make a chain
// that loops, and the tree is far shallower (domain.MaxDepth).
const subtreeLevels = 64

// Subtree implements app.Nodes: the node, then its descendants level by
// level, each level in order and read in one statement by its parents,
// planned with them.
func (s *Store) Subtree(ctx context.Context, notebookID, id uuid.UUID) (domain.Subtree, error) {
	q, levels := s.queries(ctx), s.planned(ctx)
	root, err := q.FindNodeIn(ctx, gen.FindNodeInParams{ID: id, NotebookID: notebookID})
	if err != nil {
		return nil, notFound("subtree", err)
	}
	out := domain.Subtree{{Level: 1, Node: nodeOf(gen.FindNodeRow(root))}}
	parents := []uuid.UUID{root.ID}
	for level := 2; level <= subtreeLevels && len(parents) > 0; level++ {
		rows, err := levels.ChildrenOfAll(ctx, gen.ChildrenOfAllParams{NotebookID: notebookID, Parents: parents})
		if err != nil {
			return nil, fmt.Errorf("subtree: %w", err)
		}
		parents = make([]uuid.UUID, len(rows))
		for i, r := range rows {
			out = append(out, domain.SubtreeNode{Level: level, Node: nodeOf(gen.FindNodeRow(r))})
			parents[i] = r.ID
		}
	}
	return out, nil
}

// ContentMeta implements app.Nodes.
func (s *Store) ContentMeta(ctx context.Context, id uuid.UUID) (app.ContentMeta, error) {
	row, err := s.queries(ctx).ContentMeta(ctx, id)
	if err != nil {
		return app.ContentMeta{}, notFound("content meta", err)
	}
	return app.ContentMeta{Revision: int(row.Revision), ByteSize: int(row.ByteSize), UpdatedBy: row.UpdatedByID,
		UpdatedAt: row.UpdatedAt}, nil
}

// PageContent implements app.Nodes.
func (s *Store) PageContent(ctx context.Context, id uuid.UUID) (app.PageContent, error) {
	row, err := s.queries(ctx).PageContent(ctx, id)
	if err != nil {
		return app.PageContent{}, notFound("page content", err)
	}
	return app.PageContent{Content: row.Content, Revision: int(row.Revision), Hash: row.ContentHash}, nil
}

// LockContent implements app.Nodes.
func (s *Store) LockContent(ctx context.Context, notebookID, id uuid.UUID) (app.ContentLock, error) {
	row, err := s.queries(ctx).LockContent(ctx, gen.LockContentParams{NodeID: id, NotebookID: notebookID})
	if err != nil {
		return app.ContentLock{}, notFound("lock content", err)
	}
	return app.ContentLock{Revision: int(row.Revision), Hash: row.ContentHash, ByteSize: int(row.ByteSize)}, nil
}

// CreateNode implements app.NodeWriter.
func (s *Store) CreateNode(ctx context.Context, n domain.Node) error {
	err := s.queries(ctx).CreateNode(ctx, gen.CreateNodeParams{
		ID: n.ID, NotebookID: n.NotebookID, ParentID: n.ParentID, Kind: string(n.Kind), Name: n.Name, NameKey: n.NameKey,
		SortOrder: n.SortOrder, By: n.CreatedBy, Now: n.CreatedAt,
	})
	switch {
	case uniqueViolation(err, siblingNameKey):
		return domain.ErrTitleTaken
	case err != nil:
		return fmt.Errorf("create node: %w", err)
	}
	return nil
}

// CreateContent implements app.NodeWriter.
func (s *Store) CreateContent(ctx context.Context, c app.Content) error {
	if err := s.queries(ctx).CreateContent(ctx, gen.CreateContentParams{
		NodeID: c.NodeID, Content: c.Content, Revision: int32(c.Revision), ContentHash: c.Hash,
		ByteSize: int32(c.ByteSize), By: c.By, Now: c.At,
	}); err != nil {
		return fmt.Errorf("create content: %w", err)
	}
	return nil
}

// WriteContent implements app.NodeWriter.
func (s *Store) WriteContent(ctx context.Context, c app.Content) error {
	if err := s.queries(ctx).WriteContent(ctx, gen.WriteContentParams{
		NodeID: c.NodeID, Content: c.Content, Revision: int32(c.Revision), ContentHash: c.Hash,
		ByteSize: int32(c.ByteSize), By: c.By, Now: c.At,
	}); err != nil {
		return fmt.Errorf("write content: %w", err)
	}
	return nil
}

// RenameNode implements app.NodeWriter.
func (s *Store) RenameNode(ctx context.Context, n domain.Node) error {
	err := s.queries(ctx).RenameNode(ctx, gen.RenameNodeParams{
		ID: n.ID, Name: n.Name, NameKey: n.NameKey, By: n.UpdatedBy, Now: n.UpdatedAt,
	})
	switch {
	case uniqueViolation(err, siblingNameKey):
		return domain.ErrTitleTaken
	case err != nil:
		return fmt.Errorf("rename node: %w", err)
	}
	return nil
}

// MoveNode implements app.NodeWriter.
func (s *Store) MoveNode(ctx context.Context, n domain.Node) error {
	err := s.queries(ctx).MoveNode(ctx, gen.MoveNodeParams{
		ID: n.ID, ParentID: n.ParentID, SortOrder: n.SortOrder, By: n.UpdatedBy, Now: n.UpdatedAt,
	})
	switch {
	case uniqueViolation(err, siblingNameKey):
		return domain.ErrTitleTaken
	case err != nil:
		return fmt.Errorf("move node: %w", err)
	}
	return nil
}

// DeleteNodes implements app.NodeWriter.
func (s *Store) DeleteNodes(ctx context.Context, ids []uuid.UUID, by uuid.UUID, at time.Time) error {
	if err := s.queries(ctx).DeleteNodes(ctx, gen.DeleteNodesParams{Ids: ids, By: by, Now: at}); err != nil {
		return fmt.Errorf("delete nodes: %w", err)
	}
	return nil
}

// SetSortOrder implements app.NodeWriter.
func (s *Store) SetSortOrder(ctx context.Context, id uuid.UUID, order float64) error {
	if err := s.queries(ctx).SetSortOrder(ctx, gen.SetSortOrderParams{ID: id, SortOrder: order}); err != nil {
		return fmt.Errorf("set sort order: %w", err)
	}
	return nil
}

// DeleteNotebooksPages implements app.NotebookPages.
func (s *Store) DeleteNotebooksPages(ctx context.Context, ids []uuid.UUID, by uuid.UUID, at time.Time) error {
	if err := s.queries(ctx).DeleteNotebooksPages(ctx, gen.DeleteNotebooksPagesParams{NotebookIds: ids, By: by, Now: at}); err != nil {
		return fmt.Errorf("delete the notebooks' pages: %w", err)
	}
	return nil
}
