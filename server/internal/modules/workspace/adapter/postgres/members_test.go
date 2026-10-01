package postgresadapter_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// later is a time after now, in whole microseconds.
func later() time.Time { return now().Add(time.Hour) }

// inTx runs fn in a transaction of the store's pool.
func inTx(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context) error) error {
	t.Helper()
	return postgres.NewTxManager(pool, 10*time.Second).WithinTx(context.Background(), fn)
}

// memberRow is what a membership's row holds beside its key.
type memberRow struct {
	Role      string
	UpdatedBy uuid.UUID
	UpdatedAt time.Time
	EndedAt   *time.Time
	DeletedAt *time.Time
}

func readMember(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) memberRow {
	t.Helper()
	var r memberRow
	err := pool.QueryRow(context.Background(),
		"SELECT role, updated_by_id, updated_at, ended_at, deleted_at FROM workspace_members WHERE id = $1", id).
		Scan(&r.Role, &r.UpdatedBy, &r.UpdatedAt, &r.EndedAt, &r.DeletedAt)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The two locks wait for a transaction that holds the workspace's row, and
// then see what it committed: a deletion leaves no row.
func TestLockWorkspaceWaitsForTheRowAndSeesADeletion(t *testing.T) {
	locks := map[string]func(s *postgresadapter.Store, ctx context.Context, w domain.Workspace) (domain.Workspace, error){
		"by slug": func(s *postgresadapter.Store, ctx context.Context, w domain.Workspace) (domain.Workspace, error) {
			return s.LockWorkspaceBySlug(ctx, w.Slug)
		},
		"by id": func(s *postgresadapter.Store, ctx context.Context, w domain.Workspace) (domain.Workspace, error) {
			return s.LockWorkspaceByID(ctx, w.ID)
		},
	}
	ends := []struct {
		name   string
		commit bool
	}{{"deletion committed", true}, {"deletion rolled back", false}}
	for name, lock := range locks {
		for _, end := range ends {
			commit := end.commit
			t.Run(name+", "+end.name, func(t *testing.T) {
				ctx := context.Background()
				s, pool := newStore(t)
				alice := newAccount(t, pool, "alice@corp.com")
				w := newWorkspace(t, s, "acme", "Acme", alice)
				holder, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = holder.Rollback(ctx) }()
				if _, err := holder.Exec(ctx, "UPDATE workspaces SET deleted_at = $2 WHERE id = $1", w.ID, now()); err != nil {
					t.Fatal(err)
				}

				type locked struct {
					w   domain.Workspace
					err error
				}
				done := make(chan locked, 1)
				go func() {
					var got locked
					if err := inTx(t, pool, func(ctx context.Context) error {
						got.w, got.err = lock(s, ctx, w)
						return nil
					}); err != nil {
						got.err = err
					}
					done <- got
				}()
				pgtest.WaitForLockWaitsOn(t, pool, "workspaces", 1, 10*time.Second)
				if commit {
					err = holder.Commit(ctx)
				} else {
					err = holder.Rollback(ctx)
				}
				if err != nil {
					t.Fatal(err)
				}

				got := <-done
				switch {
				case commit && !errors.Is(got.err, app.ErrNotFound):
					t.Errorf("lock after the deletion = %+v, %v; want ErrNotFound", got.w, got.err)
				case !commit && (got.err != nil || got.w != w):
					t.Errorf("lock after the rollback = %+v, %v; want %+v", got.w, got.err, w)
				}
			})
		}
	}
}

func TestLockWorkspaceOfNoWorkspace(t *testing.T) {
	s, pool := newStore(t)
	err := inTx(t, pool, func(ctx context.Context) error {
		if _, err := s.LockWorkspaceBySlug(ctx, "nowhere"); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("LockWorkspaceBySlug(nowhere) = %v, want ErrNotFound", err)
		}
		if _, err := s.LockWorkspaceByID(ctx, uuid.NewV7()); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("LockWorkspaceByID(unknown) = %v, want ErrNotFound", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenameWorkspaceWritesTheAuditColumns(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	w := newWorkspace(t, s, "acme", "Acme", alice)

	if err := s.RenameWorkspace(ctx, w.ID, "Acme Labs", bob, later()); err != nil {
		t.Fatal(err)
	}

	got, err := s.FindWorkspaceBySlug(ctx, "acme")
	want := domain.Workspace{ID: w.ID, Slug: "acme", Name: "Acme Labs", CreatedAt: now(), UpdatedAt: later()}
	if err != nil || got != want {
		t.Errorf("after the rename = %+v, %v; want %+v", got, err, want)
	}
	var updatedBy uuid.UUID
	if err := pool.QueryRow(ctx, "SELECT updated_by_id FROM workspaces WHERE id = $1", w.ID).Scan(&updatedBy); err != nil || updatedBy != bob {
		t.Errorf("updated by %s, %v; want bob", updatedBy, err)
	}
}

// A deletion deletes every row of the workspace, ended ones too, at the
// workspace's time, and no other workspace's.
func TestDeleteWorkspaceAndItsMembers(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	other := newWorkspace(t, s, "other", "Other", bob)
	ended := addMember(t, s, acme.ID, bob, shared.WorkspaceMember, alice)
	exec(t, pool, "UPDATE workspace_members SET ended_at = $2 WHERE id = $1", ended, now())
	elsewhere := addMember(t, s, other.ID, alice, shared.WorkspaceMember, bob)

	err := inTx(t, pool, func(ctx context.Context) error {
		if err := s.DeleteMembersOf(ctx, acme.ID, alice, later()); err != nil {
			return err
		}
		return s.DeleteWorkspace(ctx, acme.ID, alice, later())
	})
	if err != nil {
		t.Fatal(err)
	}

	var deletedAt *time.Time
	var updatedBy uuid.UUID
	if err := pool.QueryRow(ctx, "SELECT deleted_at, updated_by_id FROM workspaces WHERE id = $1", acme.ID).Scan(&deletedAt, &updatedBy); err != nil {
		t.Fatal(err)
	}
	if deletedAt == nil || !deletedAt.Equal(later()) || updatedBy != alice {
		t.Errorf("the workspace: deleted at %v by %s; want %v by alice", deletedAt, updatedBy, later())
	}
	rows, err := pool.Query(ctx, "SELECT id FROM workspace_members WHERE workspace_id = $1", acme.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil || len(ids) != 2 {
		t.Fatalf("acme's members = %v, %v; want the admin and the ended member", ids, err)
	}
	for _, id := range ids {
		r := readMember(t, pool, id)
		if r.DeletedAt == nil || !r.DeletedAt.Equal(later()) || r.UpdatedBy != alice || !r.UpdatedAt.Equal(later()) {
			t.Errorf("member %s: %+v; want deleted at %v by alice", id, r, later())
		}
	}
	if r := readMember(t, pool, elsewhere); r.DeletedAt != nil {
		t.Errorf("another workspace's member was deleted: %+v", r)
	}
	if r := readMember(t, pool, ended); r.EndedAt == nil || !r.EndedAt.Equal(now()) {
		t.Errorf("the ended member's end changed: %+v", r)
	}
}

func TestFindActiveMember(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	gone := newWorkspace(t, s, "gone", "Gone", alice)
	active := addMember(t, s, acme.ID, bob, shared.WorkspaceGuest, alice)
	deleted := addMember(t, s, gone.ID, bob, shared.WorkspaceMember, alice)
	exec(t, pool, "UPDATE workspace_members SET deleted_at = $2 WHERE id = $1", deleted, now())

	got, err := s.FindActiveMember(ctx, active)
	want := domain.Member{ID: active, WorkspaceID: acme.ID, UserID: bob, Role: shared.WorkspaceGuest, CreatedAt: now()}
	if err != nil || got != want {
		t.Errorf("FindActiveMember(active) = %+v, %v; want %+v", got, err, want)
	}
	exec(t, pool, "UPDATE workspace_members SET ended_at = $2 WHERE id = $1", active, now())
	for name, id := range map[string]uuid.UUID{"ended": active, "deleted": deleted, "unknown": uuid.NewV7()} {
		if got, err := s.FindActiveMember(ctx, id); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("FindActiveMember(%s) = %+v, %v; want ErrNotFound", name, got, err)
		}
	}
}

// The active members of the workspace, by when they joined, then by id.
func TestListActiveMembers(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	carol := newAccount(t, pool, "carol@corp.com")
	dave := newAccount(t, pool, "dave@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	other := newWorkspace(t, s, "other", "Other", alice)
	add := func(user uuid.UUID, at time.Time) domain.Member {
		m := domain.Member{ID: uuid.NewV7(), WorkspaceID: acme.ID, UserID: user, Role: shared.WorkspaceMember, CreatedAt: at}
		if err := s.AddMember(ctx, m, alice); err != nil {
			t.Fatal(err)
		}
		return m
	}
	// Bob joined before carol, though his row was written after hers.
	carolM := add(carol, later())
	bobM := add(bob, now().Add(time.Minute))
	endedM := add(dave, now().Add(time.Minute))
	exec(t, pool, "UPDATE workspace_members SET ended_at = $2 WHERE id = $1", endedM.ID, now())
	erin := newAccount(t, pool, "erin@corp.com")
	deletedM := add(erin, now().Add(time.Minute))
	exec(t, pool, "UPDATE workspace_members SET deleted_at = $2 WHERE id = $1", deletedM.ID, now())
	_ = addMember(t, s, other.ID, dave, shared.WorkspaceMember, alice)

	got, err := s.ListActiveMembers(ctx, acme.ID)

	admin := domain.Member{WorkspaceID: acme.ID, UserID: alice, Role: shared.WorkspaceAdmin, CreatedAt: now()}
	if err != nil || len(got) != 3 {
		t.Fatalf("ListActiveMembers() = %+v, %v; want 3", got, err)
	}
	admin.ID = got[0].ID
	if want := []domain.Member{admin, bobM, carolM}; !slices.Equal(got, want) {
		t.Errorf("ListActiveMembers() = %+v, want %+v", got, want)
	}
}

func TestCountActiveAdmins(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	carol := newAccount(t, pool, "carol@corp.com")
	dave := newAccount(t, pool, "dave@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	other := newWorkspace(t, s, "other", "Other", bob)
	addMember(t, s, acme.ID, bob, shared.WorkspaceAdmin, alice)
	ended := addMember(t, s, acme.ID, carol, shared.WorkspaceAdmin, alice)
	exec(t, pool, "UPDATE workspace_members SET ended_at = $2 WHERE id = $1", ended, now())
	addMember(t, s, acme.ID, dave, shared.WorkspaceMember, alice)
	addMember(t, s, other.ID, carol, shared.WorkspaceAdmin, bob)

	if n, err := s.CountActiveAdmins(ctx, acme.ID); err != nil || n != 2 {
		t.Errorf("CountActiveAdmins() = %d, %v; want alice and bob", n, err)
	}
	exec(t, pool, "UPDATE workspace_members SET deleted_at = $2 WHERE workspace_id = $1", acme.ID, now())
	if n, err := s.CountActiveAdmins(ctx, acme.ID); err != nil || n != 0 {
		t.Errorf("CountActiveAdmins() after the deletion = %d, %v; want 0", n, err)
	}
}

func TestUpdateMemberRoleWritesTheAuditColumns(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	id := addMember(t, s, acme.ID, bob, shared.WorkspaceMember, alice)

	if err := s.UpdateMemberRole(ctx, id, shared.WorkspaceGuest, alice, later()); err != nil {
		t.Fatal(err)
	}

	if r := readMember(t, pool, id); r.Role != "guest" || r.UpdatedBy != alice || !r.UpdatedAt.Equal(later()) {
		t.Errorf("after the update: %+v; want guest, by alice at %v", r, later())
	}
}

// EndMemberships ends the account's active memberships of the workspaces
// named, and no other row.
func TestEndMemberships(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	beta := newWorkspace(t, s, "beta", "Beta", alice)
	other := newWorkspace(t, s, "other", "Other", alice)
	inAcme := addMember(t, s, acme.ID, bob, shared.WorkspaceMember, alice)
	inBeta := addMember(t, s, beta.ID, bob, shared.WorkspaceAdmin, alice)
	inOther := addMember(t, s, other.ID, bob, shared.WorkspaceMember, alice)
	acmeAdmin, err := s.ListActiveMembers(ctx, acme.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.EndMemberships(ctx, bob, []uuid.UUID{acme.ID, beta.ID}, bob, later()); err != nil {
		t.Fatal(err)
	}

	for _, id := range []uuid.UUID{inAcme, inBeta} {
		if r := readMember(t, pool, id); r.EndedAt == nil || !r.EndedAt.Equal(later()) || r.UpdatedBy != bob || !r.UpdatedAt.Equal(later()) {
			t.Errorf("member %s: %+v; want ended at %v by bob", id, r, later())
		}
	}
	if r := readMember(t, pool, inOther); r.EndedAt != nil {
		t.Errorf("the membership of a workspace not named ended: %+v", r)
	}
	if r := readMember(t, pool, acmeAdmin[0].ID); r.EndedAt != nil {
		t.Errorf("another account's membership ended: %+v", r)
	}
	// An ended membership keeps its end.
	if err := s.EndMemberships(ctx, bob, []uuid.UUID{acme.ID}, alice, later().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if r := readMember(t, pool, inAcme); !r.EndedAt.Equal(later()) || r.UpdatedBy != bob {
		t.Errorf("ending an ended membership changed it: %+v", r)
	}
}

// The account's membership of the workspace, ended or not; a deleted one is
// none.
func TestFindMembership(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	carol := newAccount(t, pool, "carol@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	gone := newWorkspace(t, s, "gone", "Gone", alice)
	active := addMember(t, s, acme.ID, bob, shared.WorkspaceGuest, alice)
	ended := addMember(t, s, acme.ID, carol, shared.WorkspaceMember, alice)
	exec(t, pool, "UPDATE workspace_members SET ended_at = $2 WHERE id = $1", ended, now())
	deleted := addMember(t, s, gone.ID, bob, shared.WorkspaceMember, alice)
	exec(t, pool, "UPDATE workspace_members SET deleted_at = $2 WHERE id = $1", deleted, now())

	for _, tt := range []struct {
		name   string
		user   uuid.UUID
		want   domain.Member
		active bool
	}{
		{"active", bob, domain.Member{ID: active, WorkspaceID: acme.ID, UserID: bob, Role: shared.WorkspaceGuest, CreatedAt: now()}, true},
		{"ended", carol, domain.Member{ID: ended, WorkspaceID: acme.ID, UserID: carol, Role: shared.WorkspaceMember, CreatedAt: now()}, false},
	} {
		if got, ok, err := s.FindMembership(ctx, acme.ID, tt.user); err != nil || got != tt.want || ok != tt.active {
			t.Errorf("FindMembership(%s) = %+v, %v, %v; want %+v, %v", tt.name, got, ok, err, tt.want, tt.active)
		}
	}
	for name, ids := range map[string][2]uuid.UUID{
		"deleted": {gone.ID, bob}, "of another workspace": {gone.ID, carol}, "none": {acme.ID, uuid.NewV7()},
	} {
		if got, _, err := s.FindMembership(ctx, ids[0], ids[1]); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("FindMembership(%s) = %+v, %v; want ErrNotFound", name, got, err)
		}
	}
}

// A restored membership is the same row, active again with the new role;
// it keeps when the account first joined.
func TestRestoreMember(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	id := addMember(t, s, acme.ID, bob, shared.WorkspaceMember, alice)
	exec(t, pool, "UPDATE workspace_members SET ended_at = $2 WHERE id = $1", id, now())

	if err := s.RestoreMember(ctx, id, shared.WorkspaceGuest, bob, later()); err != nil {
		t.Fatal(err)
	}

	got, ok, err := s.FindMembership(ctx, acme.ID, bob)
	want := domain.Member{ID: id, WorkspaceID: acme.ID, UserID: bob, Role: shared.WorkspaceGuest, CreatedAt: now()}
	if err != nil || !ok || got != want {
		t.Errorf("after the restore = %+v, %v, %v; want %+v, active", got, ok, err, want)
	}
	if r := readMember(t, pool, id); r.EndedAt != nil || r.UpdatedBy != bob || !r.UpdatedAt.Equal(later()) {
		t.Errorf("the row: %+v; want active, updated by bob at %v", r, later())
	}
}
