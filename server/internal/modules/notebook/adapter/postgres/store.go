// Package postgresadapter is the notebook module's repository adapter:
// sqlc queries (queries/, generated into gen/) over the transaction that
// the context carries, or the pool.
package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Store implements the app's repository ports, and the access module's
// facts of the notebook level (NotebookFacts).
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

// CreateNotebook implements app.NotebookCreator.
func (s *Store) CreateNotebook(ctx context.Context, n domain.Notebook, by uuid.UUID) error {
	err := s.queries(ctx).CreateNotebook(ctx, gen.CreateNotebookParams{
		ID: n.ID, WorkspaceID: n.WorkspaceID, Name: n.Name, WorkspaceAccess: string(n.Access), By: by, Now: n.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("create notebook: %w", err)
	}
	return nil
}

// AddMember implements app.NotebookCreator.
func (s *Store) AddMember(ctx context.Context, m domain.Member, by uuid.UUID) error {
	err := s.queries(ctx).AddMember(ctx, gen.AddMemberParams{
		ID: m.ID, NotebookID: m.NotebookID, UserID: m.UserID, Role: string(m.Role), By: by, Now: m.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("add notebook member: %w", err)
	}
	return nil
}

// FindNotebook implements app.NotebookFinder.
func (s *Store) FindNotebook(ctx context.Context, id uuid.UUID) (domain.Notebook, error) {
	row, err := s.queries(ctx).FindNotebook(ctx, id)
	if err != nil {
		return domain.Notebook{}, notFound("find notebook", err)
	}
	return notebookOf(row), nil
}

// LockNotebook implements app.NotebookWriter.
func (s *Store) LockNotebook(ctx context.Context, id uuid.UUID) (domain.Notebook, error) {
	row, err := s.queries(ctx).LockNotebook(ctx, id)
	if err != nil {
		return domain.Notebook{}, notFound("lock notebook", err)
	}
	return notebookOf(gen.FindNotebookRow(row)), nil
}

// notebookOf is a row of the queries that read a notebook's columns: they
// read the same, so their rows convert to this one.
func notebookOf(r gen.FindNotebookRow) domain.Notebook {
	return domain.Notebook{
		ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, Access: shared.WorkspaceAccess(r.WorkspaceAccess),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// notFound is err of what, app.ErrNotFound for no row.
func notFound(what string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	return fmt.Errorf("%s: %w", what, err)
}

// CountMembers implements app.NotebookFinder.
func (s *Store) CountMembers(ctx context.Context, id uuid.UUID) (int, error) {
	n, err := s.queries(ctx).CountMembers(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("count notebook members: %w", err)
	}
	return int(n), nil
}

// ListNotebooks implements app.NotebookLister.
func (s *Store) ListNotebooks(ctx context.Context, workspaceID, userID uuid.UUID, reached bool) ([]app.Listed, error) {
	rows, err := s.queries(ctx).ListNotebooks(ctx, gen.ListNotebooksParams{WorkspaceID: workspaceID, UserID: userID, Reached: reached})
	if err != nil {
		return nil, fmt.Errorf("list notebooks: %w", err)
	}
	list := make([]app.Listed, len(rows))
	for i, r := range rows {
		list[i] = app.Listed{
			Notebook: domain.Notebook{
				ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, Access: shared.WorkspaceAccess(r.WorkspaceAccess),
				CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			},
			Explicit:    role(r.Role),
			MemberCount: int(r.MemberCount),
		}
	}
	return list, nil
}

// role is a nullable role column: "" for NULL.
func role(r *string) shared.NotebookRole {
	if r == nil {
		return ""
	}
	return shared.NotebookRole(*r)
}

// UpdateNotebook implements app.NotebookWriter.
func (s *Store) UpdateNotebook(ctx context.Context, n domain.Notebook, by uuid.UUID) error {
	err := s.queries(ctx).UpdateNotebook(ctx, gen.UpdateNotebookParams{
		ID: n.ID, Name: n.Name, WorkspaceAccess: string(n.Access), By: by, Now: n.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("update notebook: %w", err)
	}
	return nil
}

// DeleteNotebook implements app.NotebookWriter: the notebook, then its
// member rows, at one time. The caller's transaction makes the two one.
func (s *Store) DeleteNotebook(ctx context.Context, id, by uuid.UUID, now time.Time) error {
	q := s.queries(ctx)
	if err := q.DeleteNotebook(ctx, gen.DeleteNotebookParams{ID: id, By: by, Now: now}); err != nil {
		return fmt.Errorf("delete notebook: %w", err)
	}
	if err := q.DeleteMembersOf(ctx, gen.DeleteMembersOfParams{NotebookID: id, By: by, Now: now}); err != nil {
		return fmt.Errorf("delete notebook members: %w", err)
	}
	return nil
}

// NotebookFacts is what userID has of notebookID: the access module's
// facts of the notebook level, in the transaction ctx carries.
func (s *Store) NotebookFacts(ctx context.Context, notebookID, userID uuid.UUID) (app.Fact, error) {
	row, err := s.queries(ctx).NotebookFacts(ctx, gen.NotebookFactsParams{NotebookID: notebookID, UserID: userID})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return app.Fact{}, nil
	case err != nil:
		return app.Fact{}, fmt.Errorf("notebook facts: %w", err)
	}
	return app.Fact{Found: true, WorkspaceID: row.WorkspaceID, Access: shared.WorkspaceAccess(row.WorkspaceAccess), Role: role(row.Role)}, nil
}

// DeleteNotebooksOf implements app.NotebooksDeleter: the notebooks, then
// their member rows, at one time. The caller's transaction makes the two
// one.
func (s *Store) DeleteNotebooksOf(ctx context.Context, workspaceID, by uuid.UUID, at time.Time) ([]uuid.UUID, error) {
	q := s.queries(ctx)
	ids, err := q.DeleteNotebooksOf(ctx, gen.DeleteNotebooksOfParams{WorkspaceID: workspaceID, By: by, Now: at})
	if err != nil {
		return nil, fmt.Errorf("delete notebooks of workspace: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if err := q.DeleteMembersOfNotebooks(ctx, gen.DeleteMembersOfNotebooksParams{NotebookIds: ids, By: by, Now: at}); err != nil {
		return nil, fmt.Errorf("delete members of notebooks: %w", err)
	}
	return ids, nil
}
