package postgresadapter_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// now is in whole microseconds, as timestamptz stores them.
func now() time.Time { return time.Date(2026, 10, 2, 10, 0, 0, 123456000, time.UTC) }

// later is a time after now, in whole microseconds.
func later() time.Time { return now().Add(time.Hour) }

func newStore(t *testing.T) (*postgresadapter.Store, *pgxpool.Pool) {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return postgresadapter.New(pool), pool
}

// exec runs statements of the test's own: other modules' rows, and the
// writes no use case of this Phase makes.
func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// inTx runs fn in a transaction of the store's pool.
func inTx(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context) error) error {
	t.Helper()
	return postgres.NewTxManager(pool, 10*time.Second).WithinTx(context.Background(), fn)
}

// newAccount inserts an account into identity's table and returns its id.
func newAccount(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	exec(t, pool, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, $2, 'x', 'x', $3, $3)",
		id, email, now())
	return id
}

// newWorkspace inserts a workspace into the workspace module's table.
func newWorkspace(t *testing.T, pool *pgxpool.Pool, slug string, by uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	exec(t, pool, "INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, $2, $3, $3, $4, $4)",
		id, slug, by, now())
	return id
}

// newNotebook creates a notebook of workspaceID with by as its admin.
func newNotebook(t *testing.T, s *postgresadapter.Store, workspaceID uuid.UUID, name string, access shared.WorkspaceAccess, by uuid.UUID) domain.Notebook {
	t.Helper()
	n := domain.Notebook{ID: uuid.NewV7(), WorkspaceID: workspaceID, Name: name, Access: access, CreatedAt: now(), UpdatedAt: now()}
	if err := s.CreateNotebook(context.Background(), n, by); err != nil {
		t.Fatal(err)
	}
	addMember(t, s, n.ID, by, shared.NotebookAdmin, by)
	return n
}

func addMember(t *testing.T, s *postgresadapter.Store, notebookID, userID uuid.UUID, role shared.NotebookRole, by uuid.UUID) uuid.UUID {
	t.Helper()
	m := domain.Member{ID: uuid.NewV7(), NotebookID: notebookID, UserID: userID, Role: role, CreatedAt: now()}
	if err := s.AddMember(context.Background(), m, by); err != nil {
		t.Fatal(err)
	}
	return m.ID
}

// row is what a row of either table holds beside its key and its own
// columns.
type row struct {
	CreatedBy, UpdatedBy uuid.UUID
	CreatedAt, UpdatedAt time.Time
	DeletedAt            *time.Time
}

func readRow(t *testing.T, pool *pgxpool.Pool, table string, id uuid.UUID) row {
	t.Helper()
	var r row
	err := pool.QueryRow(context.Background(),
		"SELECT created_by_id, updated_by_id, created_at, updated_at, deleted_at FROM "+table+" WHERE id = $1", id).
		Scan(&r.CreatedBy, &r.UpdatedBy, &r.CreatedAt, &r.UpdatedAt, &r.DeletedAt)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCreateNotebookWritesTheAuditColumns(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	n := newNotebook(t, s, newWorkspace(t, pool, "acme", alice), "Engineering", shared.AccessViewer, alice)

	got, err := s.FindNotebook(ctx, n.ID)
	if err != nil || got != n {
		t.Errorf("FindNotebook() = %+v, %v; want %+v", got, err, n)
	}
	want := row{CreatedBy: alice, UpdatedBy: alice, CreatedAt: now(), UpdatedAt: now()}
	if r := readRow(t, pool, "notebooks", n.ID); !sameRow(r, want) {
		t.Errorf("the notebook's row = %+v, want %+v", r, want)
	}
	var memberID uuid.UUID
	var role string
	if err := pool.QueryRow(ctx, "SELECT id, role FROM notebook_members WHERE notebook_id = $1 AND user_id = $2", n.ID, alice).
		Scan(&memberID, &role); err != nil {
		t.Fatal(err)
	}
	if r := readRow(t, pool, "notebook_members", memberID); role != "admin" || !sameRow(r, want) {
		t.Errorf("the admin's row = %s, %+v; want admin, %+v", role, r, want)
	}
}

// sameRow compares two rows, their times by the instant.
func sameRow(a, b row) bool {
	deletedAlike := (a.DeletedAt == nil) == (b.DeletedAt == nil) && (a.DeletedAt == nil || a.DeletedAt.Equal(*b.DeletedAt))
	return a.CreatedBy == b.CreatedBy && a.UpdatedBy == b.UpdatedBy && a.CreatedAt.Equal(b.CreatedAt) &&
		a.UpdatedAt.Equal(b.UpdatedAt) && deletedAlike
}

func TestFindNotebookOfNoNotebook(t *testing.T) {
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	gone := newNotebook(t, s, newWorkspace(t, pool, "acme", alice), "Gone", shared.AccessNone, alice)
	exec(t, pool, "UPDATE notebooks SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	for _, id := range []uuid.UUID{gone.ID, uuid.NewV7()} {
		if got, err := s.FindNotebook(context.Background(), id); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("FindNotebook(%s) = %+v, %v; want ErrNotFound", id, got, err)
		}
	}
}

// The list shows what the caller sees (M3/P1 design 3.7): its notebooks,
// and, when reached, the ones open to the workspace; never a deleted one,
// another workspace's, or one of an ended membership.
func TestListNotebooks(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	carol := newAccount(t, pool, "carol@corp.com")
	acme := newWorkspace(t, pool, "acme", alice)
	mine := newNotebook(t, s, acme, "b mine", shared.AccessNone, alice)
	addMember(t, s, mine.ID, bob, shared.NotebookReader, alice)
	exec(t, pool, "UPDATE notebook_members SET ended_at = $2 WHERE id = $1", addMember(t, s, mine.ID, carol, shared.NotebookEditor, alice), now())
	viewer := newNotebook(t, s, acme, "A viewer", shared.AccessViewer, bob)
	editor := newNotebook(t, s, acme, "a editor", shared.AccessEditor, bob)
	newNotebook(t, s, acme, "closed", shared.AccessNone, bob)
	left := newNotebook(t, s, acme, "left", shared.AccessNone, bob)
	exec(t, pool, "UPDATE notebook_members SET ended_at = $2 WHERE id = $1", addMember(t, s, left.ID, alice, shared.NotebookAdmin, bob), now())
	gone := newNotebook(t, s, acme, "gone", shared.AccessEditor, alice)
	exec(t, pool, "UPDATE notebooks SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	newNotebook(t, s, newWorkspace(t, pool, "other", alice), "other", shared.AccessEditor, alice)

	for _, tt := range []struct {
		name    string
		reached bool
		want    []app.Listed
	}{
		{"reached", true, []app.Listed{
			{Notebook: editor, MemberCount: 1},
			{Notebook: viewer, MemberCount: 1},
			{Notebook: mine, Explicit: shared.NotebookAdmin, MemberCount: 2},
		}},
		{"not reached", false, []app.Listed{{Notebook: mine, Explicit: shared.NotebookAdmin, MemberCount: 2}}},
	} {
		got, err := s.ListNotebooks(ctx, acme, alice, tt.reached)
		if err != nil || !slices.Equal(got, tt.want) {
			t.Errorf("%s: ListNotebooks() = %+v, %v; want %+v", tt.name, got, err, tt.want)
		}
	}
	if n, err := s.CountMembers(ctx, mine.ID); err != nil || n != 2 {
		t.Errorf("CountMembers() = %d, %v; want 2: the ended member is not counted", n, err)
	}
}

// The lock waits for a transaction that holds the notebook's row, and then
// sees what it committed: a deletion leaves no row.
func TestLockNotebookWaitsForTheRowAndSeesADeletion(t *testing.T) {
	for _, commit := range []bool{true, false} {
		t.Run(map[bool]string{true: "deletion committed", false: "deletion rolled back"}[commit], func(t *testing.T) {
			ctx := context.Background()
			s, pool := newStore(t)
			alice := newAccount(t, pool, "alice@corp.com")
			n := newNotebook(t, s, newWorkspace(t, pool, "acme", alice), "Engineering", shared.AccessNone, alice)
			holder, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = holder.Rollback(ctx) }()
			if _, err := holder.Exec(ctx, "UPDATE notebooks SET deleted_at = $2 WHERE id = $1", n.ID, now()); err != nil {
				t.Fatal(err)
			}

			type locked struct {
				n   domain.Notebook
				err error
			}
			done := make(chan locked, 1)
			go func() {
				var got locked
				if err := inTx(t, pool, func(ctx context.Context) error {
					got.n, got.err = s.LockNotebook(ctx, n.ID)
					return nil
				}); err != nil {
					got.err = err
				}
				done <- got
			}()
			pgtest.WaitForLockWaitsOn(t, pool, "notebooks", 1, 10*time.Second)
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
				t.Errorf("lock after the deletion = %+v, %v; want ErrNotFound", got.n, got.err)
			case !commit && (got.err != nil || got.n != n):
				t.Errorf("lock after the rollback = %+v, %v; want %+v", got.n, got.err, n)
			}
		})
	}
}

func TestUpdateNotebookWritesTheAuditColumns(t *testing.T) {
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	n := newNotebook(t, s, newWorkspace(t, pool, "acme", alice), "Engineering", shared.AccessNone, alice)
	changed := n
	changed.Name, changed.Access, changed.UpdatedAt = "Eng", shared.AccessEditor, later()
	if err := s.UpdateNotebook(context.Background(), changed, bob); err != nil {
		t.Fatal(err)
	}
	if got, err := s.FindNotebook(context.Background(), n.ID); err != nil || got != changed {
		t.Errorf("FindNotebook() = %+v, %v; want %+v", got, err, changed)
	}
	want := row{CreatedBy: alice, UpdatedBy: bob, CreatedAt: now(), UpdatedAt: later()}
	if r := readRow(t, pool, "notebooks", n.ID); !sameRow(r, want) {
		t.Errorf("the notebook's row = %+v, want %+v", r, want)
	}
}

func TestDeleteNotebookAndItsMembers(t *testing.T) {
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	acme := newWorkspace(t, pool, "acme", alice)
	n := newNotebook(t, s, acme, "Engineering", shared.AccessNone, alice)
	member := addMember(t, s, n.ID, bob, shared.NotebookEditor, alice)
	ended := addMember(t, s, n.ID, newAccount(t, pool, "carol@corp.com"), shared.NotebookReader, alice)
	exec(t, pool, "UPDATE notebook_members SET ended_at = $2 WHERE id = $1", ended, now())
	other := newNotebook(t, s, acme, "Other", shared.AccessNone, bob)

	if err := inTx(t, pool, func(ctx context.Context) error { return s.DeleteNotebook(ctx, n.ID, bob, later()) }); err != nil {
		t.Fatal(err)
	}

	deleted := later()
	if r := readRow(t, pool, "notebooks", n.ID); !sameRow(r, row{CreatedBy: alice, UpdatedBy: bob, CreatedAt: now(), UpdatedAt: later(), DeletedAt: &deleted}) {
		t.Errorf("the notebook's row = %+v; want deleted at %v by bob", r, later())
	}
	var admin uuid.UUID
	if err := pool.QueryRow(context.Background(), "SELECT id FROM notebook_members WHERE notebook_id = $1 AND user_id = $2", n.ID, alice).Scan(&admin); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uuid.UUID{admin, member, ended} {
		if r := readRow(t, pool, "notebook_members", id); r.DeletedAt == nil || !r.DeletedAt.Equal(later()) || r.UpdatedBy != bob || !r.UpdatedAt.Equal(later()) {
			t.Errorf("member %s: %+v; want deleted at %v by bob", id, r, later())
		}
	}
	if _, err := s.FindNotebook(context.Background(), other.ID); err != nil {
		t.Errorf("another notebook: %v", err)
	}
	if n, err := s.CountMembers(context.Background(), other.ID); err != nil || n != 1 {
		t.Errorf("another notebook's members = %d, %v; want 1", n, err)
	}
}

// The facts of the access module's notebook level, in the caller's
// transaction.
func TestNotebookFacts(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	carol := newAccount(t, pool, "carol@corp.com")
	acme := newWorkspace(t, pool, "acme", alice)
	n := newNotebook(t, s, acme, "Engineering", shared.AccessViewer, alice)
	exec(t, pool, "UPDATE notebook_members SET ended_at = $2 WHERE id = $1", addMember(t, s, n.ID, bob, shared.NotebookEditor, alice), now())
	gone := newNotebook(t, s, acme, "Gone", shared.AccessEditor, alice)
	exec(t, pool, "UPDATE notebooks SET deleted_at = $2 WHERE id = $1", gone.ID, now())

	open := app.Fact{Found: true, WorkspaceID: acme, Access: shared.AccessViewer}
	admin := open
	admin.Role = shared.NotebookAdmin
	for _, tt := range []struct {
		name           string
		notebook, user uuid.UUID
		want           app.Fact
	}{
		{"its admin", n.ID, alice, admin},
		{"an ended member", n.ID, bob, open},
		{"no member", n.ID, carol, open},
		{"deleted", gone.ID, alice, app.Fact{}},
		{"no notebook", uuid.NewV7(), alice, app.Fact{}},
	} {
		if got, err := s.NotebookFacts(ctx, tt.notebook, tt.user); err != nil || got != tt.want {
			t.Errorf("%s: NotebookFacts() = %+v, %v; want %+v", tt.name, got, err, tt.want)
		}
	}

	reader := open
	reader.Role = shared.NotebookReader
	addMember(t, s, n.ID, carol, shared.NotebookReader, alice)
	err := inTx(t, pool, func(ctx context.Context) error {
		if _, err := postgres.DB(ctx, pool).Exec(ctx, "UPDATE notebooks SET workspace_access = 'editor' WHERE id = $1", n.ID); err != nil {
			return err
		}
		got, err := s.NotebookFacts(ctx, n.ID, carol)
		if want := (app.Fact{Found: true, WorkspaceID: acme, Access: shared.AccessEditor, Role: shared.NotebookReader}); err != nil || got != want {
			t.Errorf("NotebookFacts() in the transaction = %+v, %v; want %+v, what it wrote", got, err, want)
		}
		return errors.New("roll back")
	})
	if err == nil {
		t.Fatal("the transaction committed")
	}
	if got, err := s.NotebookFacts(ctx, n.ID, carol); err != nil || got != reader {
		t.Errorf("NotebookFacts() after the rollback = %+v, %v; want %+v", got, err, reader)
	}
}
