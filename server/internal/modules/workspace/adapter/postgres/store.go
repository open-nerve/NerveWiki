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
func (s *Store) AddMember(ctx context.Context, m domain.Member, by uuid.UUID) error {
	err := s.queries(ctx).AddMember(ctx, gen.AddMemberParams{
		ID: m.ID, WorkspaceID: m.WorkspaceID, UserID: m.UserID, Role: string(m.Role), By: by, CreatedAt: m.CreatedAt,
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
	return workspaceOf(row), nil
}

// workspaceOf is a row of the queries that read a workspace's columns: they
// read the same, so their rows convert to this one.
func workspaceOf(r gen.FindWorkspaceBySlugRow) domain.Workspace {
	return domain.Workspace{ID: r.ID, Slug: r.Slug, Name: r.Name, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

// memberOf is a row of the queries that read a member's columns.
func memberOf(r gen.FindActiveMemberRow) domain.Member {
	return domain.Member{ID: r.ID, WorkspaceID: r.WorkspaceID, UserID: r.UserID, Role: shared.WorkspaceRole(r.Role), CreatedAt: r.CreatedAt}
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

// LockWorkspaceBySlug implements app.WorkspaceLocker.
func (s *Store) LockWorkspaceBySlug(ctx context.Context, slug string) (domain.Workspace, error) {
	row, err := s.queries(ctx).LockWorkspaceBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Workspace{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("lock workspace: %w", err)
	}
	return workspaceOf(gen.FindWorkspaceBySlugRow(row)), nil
}

// LockWorkspaceByID implements app.WorkspaceLocker.
func (s *Store) LockWorkspaceByID(ctx context.Context, id uuid.UUID) (domain.Workspace, error) {
	row, err := s.queries(ctx).LockWorkspaceByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Workspace{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("lock workspace: %w", err)
	}
	return workspaceOf(gen.FindWorkspaceBySlugRow(row)), nil
}

// ShareWorkspaceBySlug returns the workspace not deleted with slug, locked
// FOR SHARE until the transaction ends; app.ErrNotFound when there is none,
// a deletion committed while it waited too.
func (s *Store) ShareWorkspaceBySlug(ctx context.Context, slug string) (domain.Workspace, error) {
	row, err := s.queries(ctx).ShareWorkspaceBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Workspace{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("share workspace: %w", err)
	}
	return workspaceOf(gen.FindWorkspaceBySlugRow(row)), nil
}

// ShareWorkspaceByID is ShareWorkspaceBySlug by the workspace's id.
func (s *Store) ShareWorkspaceByID(ctx context.Context, id uuid.UUID) (domain.Workspace, error) {
	row, err := s.queries(ctx).ShareWorkspaceByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Workspace{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("share workspace: %w", err)
	}
	return workspaceOf(gen.FindWorkspaceBySlugRow(row)), nil
}

// FindWorkspaceByID is FindWorkspaceBySlug by the workspace's id.
func (s *Store) FindWorkspaceByID(ctx context.Context, id uuid.UUID) (domain.Workspace, error) {
	row, err := s.queries(ctx).FindWorkspaceByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Workspace{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("find workspace: %w", err)
	}
	return workspaceOf(gen.FindWorkspaceBySlugRow(row)), nil
}

// RenameWorkspace implements app.WorkspaceUpdater.
func (s *Store) RenameWorkspace(ctx context.Context, id uuid.UUID, name string, by uuid.UUID, now time.Time) error {
	if err := s.queries(ctx).RenameWorkspace(ctx, gen.RenameWorkspaceParams{ID: id, Name: name, By: by, Now: now}); err != nil {
		return fmt.Errorf("rename workspace: %w", err)
	}
	return nil
}

// DeleteWorkspace implements app.WorkspaceUpdater.
func (s *Store) DeleteWorkspace(ctx context.Context, id, by uuid.UUID, now time.Time) error {
	if err := s.queries(ctx).DeleteWorkspace(ctx, gen.DeleteWorkspaceParams{ID: id, By: by, Now: now}); err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	return nil
}

// FindActiveMember implements app.MemberFinder.
func (s *Store) FindActiveMember(ctx context.Context, id uuid.UUID) (domain.Member, error) {
	row, err := s.queries(ctx).FindActiveMember(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Member{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Member{}, fmt.Errorf("find member: %w", err)
	}
	return memberOf(row), nil
}

// FindMembership returns userID's membership of workspaceID, and whether
// it is active: ended ones too, deleted ones not; app.ErrNotFound when there
// is none.
func (s *Store) FindMembership(ctx context.Context, workspaceID, userID uuid.UUID) (domain.Member, bool, error) {
	row, err := s.queries(ctx).FindMembership(ctx, gen.FindMembershipParams{WorkspaceID: workspaceID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Member{}, false, app.ErrNotFound
	}
	if err != nil {
		return domain.Member{}, false, fmt.Errorf("find membership: %w", err)
	}
	return domain.Member{ID: row.ID, WorkspaceID: row.WorkspaceID, UserID: row.UserID, Role: shared.WorkspaceRole(row.Role),
		CreatedAt: row.CreatedAt}, row.Active, nil
}

// ListActiveMembers implements app.MemberFinder.
func (s *Store) ListActiveMembers(ctx context.Context, workspaceID uuid.UUID) ([]domain.Member, error) {
	rows, err := s.queries(ctx).ListActiveMembers(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	list := make([]domain.Member, len(rows))
	for i, r := range rows {
		list[i] = memberOf(gen.FindActiveMemberRow(r))
	}
	return list, nil
}

// CountActiveAdmins implements app.MemberFinder.
func (s *Store) CountActiveAdmins(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	n, err := s.queries(ctx).CountActiveAdmins(ctx, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return int(n), nil
}

// UpdateMemberRole implements app.MemberUpdater.
func (s *Store) UpdateMemberRole(ctx context.Context, id uuid.UUID, role shared.WorkspaceRole, by uuid.UUID, now time.Time) error {
	err := s.queries(ctx).UpdateMemberRole(ctx, gen.UpdateMemberRoleParams{ID: id, Role: string(role), By: by, Now: now})
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	return nil
}

// RestoreMember makes the ended membership id active again with role,
// updated by by at now; when it was first created stays.
func (s *Store) RestoreMember(ctx context.Context, id uuid.UUID, role shared.WorkspaceRole, by uuid.UUID, now time.Time) error {
	err := s.queries(ctx).RestoreMember(ctx, gen.RestoreMemberParams{ID: id, Role: string(role), By: by, Now: now})
	if err != nil {
		return fmt.Errorf("restore member: %w", err)
	}
	return nil
}

// EndMemberships implements app.MemberUpdater.
func (s *Store) EndMemberships(ctx context.Context, userID uuid.UUID, workspaceIDs []uuid.UUID, by uuid.UUID, now time.Time) error {
	err := s.queries(ctx).EndMemberships(ctx, gen.EndMembershipsParams{UserID: userID, WorkspaceIds: workspaceIDs, By: by, Now: now})
	if err != nil {
		return fmt.Errorf("end memberships: %w", err)
	}
	return nil
}

// DeleteMembersOf implements app.MemberUpdater.
func (s *Store) DeleteMembersOf(ctx context.Context, workspaceID, by uuid.UUID, now time.Time) error {
	err := s.queries(ctx).DeleteMembersOf(ctx, gen.DeleteMembersOfParams{WorkspaceID: workspaceID, By: by, Now: now})
	if err != nil {
		return fmt.Errorf("delete members: %w", err)
	}
	return nil
}
