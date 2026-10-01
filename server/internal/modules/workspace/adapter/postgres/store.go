// Package postgresadapter is the workspace module's repository adapter:
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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Store implements the app's repository ports, and the access module's
// fact of the workspace level (RoleOf).
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

// uniqueViolation reports whether err broke the unique constraint or index
// name.
func uniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

// CreateWorkspace implements app.WorkspaceCreator.
func (s *Store) CreateWorkspace(ctx context.Context, w domain.Workspace, by uuid.UUID) error {
	err := s.queries(ctx).CreateWorkspace(ctx, gen.CreateWorkspaceParams{ID: w.ID, Slug: w.Slug, Name: w.Name, By: by, Now: w.CreatedAt})
	switch {
	case uniqueViolation(err, "workspaces_slug_key"):
		return domain.ErrSlugTaken
	case err != nil:
		return fmt.Errorf("create workspace: %w", err)
	}
	return nil
}

// AddMember implements app.WorkspaceCreator.
func (s *Store) AddMember(ctx context.Context, m domain.Member, by uuid.UUID, now time.Time) error {
	err := s.queries(ctx).AddMember(ctx, gen.AddMemberParams{
		ID: m.ID, WorkspaceID: m.WorkspaceID, UserID: m.UserID, Role: string(m.Role), By: by, Now: now,
	})
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

// FindWorkspaceBySlug implements app.WorkspaceFinder.
func (s *Store) FindWorkspaceBySlug(ctx context.Context, slug string) (domain.Workspace, error) {
	row, err := s.queries(ctx).FindWorkspaceBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Workspace{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("find workspace: %w", err)
	}
	return domain.Workspace{ID: row.ID, Slug: row.Slug, Name: row.Name, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

// ListWorkspacesOf implements app.MembershipLister.
func (s *Store) ListWorkspacesOf(ctx context.Context, userID uuid.UUID) ([]app.Membership, error) {
	rows, err := s.queries(ctx).ListWorkspacesOf(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	list := make([]app.Membership, len(rows))
	for i, r := range rows {
		list[i] = app.Membership{
			Workspace: domain.Workspace{ID: r.ID, Slug: r.Slug, Name: r.Name, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt},
			Role:      shared.WorkspaceRole(r.Role),
		}
	}
	return list, nil
}

// SlugTaken implements app.SlugChecker.
func (s *Store) SlugTaken(ctx context.Context, slug string) (bool, error) {
	taken, err := s.queries(ctx).SlugTaken(ctx, slug)
	if err != nil {
		return false, fmt.Errorf("slug taken: %w", err)
	}
	return taken, nil
}

// RoleOf returns userID's role in workspaceID, and whether userID is an
// active member of it, in the transaction ctx carries: the access module's
// fact of the workspace level.
func (s *Store) RoleOf(ctx context.Context, workspaceID, userID uuid.UUID) (shared.WorkspaceRole, bool, error) {
	role, err := s.queries(ctx).RoleOf(ctx, gen.RoleOfParams{WorkspaceID: workspaceID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("role of: %w", err)
	}
	return shared.WorkspaceRole(role), true, nil
}
