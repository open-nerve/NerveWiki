package postgresadapter_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// now is in whole microseconds, as timestamptz stores them.
func now() time.Time { return time.Date(2026, 10, 1, 10, 0, 0, 123456000, time.UTC) }

func newStore(t *testing.T) (*postgresadapter.Store, *pgxpool.Pool) {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return postgresadapter.New(pool), pool
}

// exec runs statements of the test's own: identity's accounts, and the
// writes no use case makes yet.
func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// newAccount inserts an account into identity's table and returns its id.
func newAccount(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	exec(t, pool, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, $2, 'x', 'x', $3, $3)",
		id, email, now())
	return id
}

// newWorkspace creates a workspace with by as its admin.
func newWorkspace(t *testing.T, s *postgresadapter.Store, slug, name string, by uuid.UUID) domain.Workspace {
	t.Helper()
	w := domain.Workspace{ID: uuid.NewV7(), Slug: slug, Name: name, CreatedAt: now(), UpdatedAt: now()}
	if err := s.CreateWorkspace(context.Background(), w, by); err != nil {
		t.Fatal(err)
	}
	addMember(t, s, w.ID, by, shared.WorkspaceAdmin, by)
	return w
}

func addMember(t *testing.T, s *postgresadapter.Store, workspaceID, userID uuid.UUID, role shared.WorkspaceRole, by uuid.UUID) uuid.UUID {
	t.Helper()
	m := domain.Member{ID: uuid.NewV7(), WorkspaceID: workspaceID, UserID: userID, Role: role}
	if err := s.AddMember(context.Background(), m, by, now()); err != nil {
		t.Fatal(err)
	}
	return m.ID
}

func TestCreateWorkspaceWritesTheAuditColumns(t *testing.T) {
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	w := newWorkspace(t, s, "acme", "Acme", alice)

	got, err := s.FindWorkspaceBySlug(context.Background(), "acme")
	if err != nil || got != w {
		t.Errorf("FindWorkspaceBySlug() = %+v, %v; want %+v", got, err, w)
	}
	var createdBy, updatedBy uuid.UUID
	var deleted *time.Time
	if err := pool.QueryRow(context.Background(), "SELECT created_by_id, updated_by_id, deleted_at FROM workspaces WHERE id = $1", w.ID).
		Scan(&createdBy, &updatedBy, &deleted); err != nil {
		t.Fatal(err)
	}
	if createdBy != alice || updatedBy != alice || deleted != nil {
		t.Errorf("created by %s, updated by %s, deleted %v; want alice, alice, not deleted", createdBy, updatedBy, deleted)
	}
}

func TestATakenSlugAndItsRelease(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	w := newWorkspace(t, s, "acme", "Acme", alice)

	again := domain.Workspace{ID: uuid.NewV7(), Slug: "acme", Name: "Other", CreatedAt: now(), UpdatedAt: now()}
	if err := s.CreateWorkspace(ctx, again, alice); !errors.Is(err, domain.ErrSlugTaken) {
		t.Errorf("CreateWorkspace(a taken slug) = %v, want ErrSlugTaken", err)
	}
	if taken, err := s.SlugTaken(ctx, "acme"); err != nil || !taken {
		t.Errorf("SlugTaken(acme) = %v, %v; want true", taken, err)
	}
	exec(t, pool, "UPDATE workspaces SET deleted_at = $2 WHERE id = $1", w.ID, now())

	if taken, err := s.SlugTaken(ctx, "acme"); err != nil || taken {
		t.Errorf("SlugTaken(acme) after its deletion = %v, %v; want false", taken, err)
	}
	if _, err := s.FindWorkspaceBySlug(ctx, "acme"); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("FindWorkspaceBySlug(deleted) = %v, want ErrNotFound", err)
	}
	if err := s.CreateWorkspace(ctx, again, alice); err != nil {
		t.Errorf("CreateWorkspace(a deleted workspace's slug) = %v", err)
	}
}

func TestListWorkspacesOfIsTheActiveMemberships(t *testing.T) {
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	// By name, then by id: two named alike keep their order of creation.
	zeta := newWorkspace(t, s, "zeta", "Zeta", bob)
	beta1 := newWorkspace(t, s, "beta-1", "Beta", bob)
	beta2 := newWorkspace(t, s, "beta-2", "Beta", bob)
	alpha := newWorkspace(t, s, "alpha", "Alpha", bob)
	ended := newWorkspace(t, s, "ended", "Ended", bob)
	gone := newWorkspace(t, s, "gone", "Gone", bob)
	_ = newWorkspace(t, s, "others", "Others", bob)
	addMember(t, s, zeta.ID, alice, shared.WorkspaceGuest, bob)
	addMember(t, s, beta2.ID, alice, shared.WorkspaceMember, bob)
	addMember(t, s, beta1.ID, alice, shared.WorkspaceAdmin, bob)
	addMember(t, s, alpha.ID, alice, shared.WorkspaceMember, bob)
	endedID := addMember(t, s, ended.ID, alice, shared.WorkspaceMember, bob)
	addMember(t, s, gone.ID, alice, shared.WorkspaceMember, bob)
	exec(t, pool, "UPDATE workspace_members SET ended_at = $2 WHERE id = $1", endedID, now())
	exec(t, pool, "UPDATE workspaces SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	exec(t, pool, "UPDATE workspace_members SET deleted_at = $2 WHERE workspace_id = $1", gone.ID, now())

	got, err := s.ListWorkspacesOf(context.Background(), alice)

	want := []app.Membership{
		{Workspace: alpha, Role: shared.WorkspaceMember},
		{Workspace: beta1, Role: shared.WorkspaceAdmin},
		{Workspace: beta2, Role: shared.WorkspaceMember},
		{Workspace: zeta, Role: shared.WorkspaceGuest},
	}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("ListWorkspacesOf() = %+v, %v; want %+v", got, err, want)
	}
}

func TestRoleOfIsTheActiveMembershipInTheCallersTransaction(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	other := newWorkspace(t, s, "other", "Other", bob)

	tests := []struct {
		name      string
		workspace uuid.UUID
		user      uuid.UUID
		role      shared.WorkspaceRole
		ok        bool
	}{
		{"the admin", acme.ID, alice, shared.WorkspaceAdmin, true},
		{"not a member", acme.ID, bob, "", false},
		{"a member of another workspace", other.ID, alice, "", false},
		{"no such workspace", uuid.NewV7(), alice, "", false},
	}
	for _, tt := range tests {
		role, ok, err := s.RoleOf(ctx, tt.workspace, tt.user)
		if err != nil || role != tt.role || ok != tt.ok {
			t.Errorf("%s: RoleOf() = %q, %v, %v; want %q, %v", tt.name, role, ok, err, tt.role, tt.ok)
		}
	}

	// Within a transaction it reads what the transaction wrote.
	err := postgres.NewTxManager(pool, time.Second).WithinTx(ctx, func(ctx context.Context) error {
		if _, err := postgres.DB(ctx, pool).Exec(ctx, "UPDATE workspace_members SET ended_at = $2 WHERE workspace_id = $1", acme.ID, now()); err != nil {
			return err
		}
		if _, ok, err := s.RoleOf(ctx, acme.ID, alice); err != nil || ok {
			return errors.New("RoleOf() in the transaction that ended the membership still sees it")
		}
		return errors.New("roll back")
	})
	if err == nil || err.Error() != "roll back" {
		t.Error(err)
	}
	if _, ok, err := s.RoleOf(ctx, acme.ID, alice); err != nil || !ok {
		t.Errorf("RoleOf() after the rollback = %v, %v; want the membership", ok, err)
	}
	exec(t, pool, "UPDATE workspaces SET deleted_at = $2 WHERE id = $1", acme.ID, now())
	exec(t, pool, "UPDATE workspace_members SET deleted_at = $2 WHERE workspace_id = $1", acme.ID, now())
	if _, ok, err := s.RoleOf(ctx, acme.ID, alice); err != nil || ok {
		t.Errorf("RoleOf() in a deleted workspace = %v, %v; want no membership", ok, err)
	}
}
