package notebook_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The extension point and the registrant on a real database (M3/P1 design
// 3.8), the module wired as bootstrap wires it. The subscribers stand in for
// M4's: built from the pool alone, their statements reach the transaction
// through the context. The decision stands in for the access module's,
// which the notebook module does not import: it reads the same facts
// through NewFacts, in the transaction.

// testNow is the clock's instant, in whole microseconds.
func testNow() time.Time { return time.Date(2026, 10, 2, 10, 0, 0, 123456000, time.UTC) }

// tickingClock reads testNow first, then a microsecond later each time: a
// use case that read it twice for what must be one time would write two.
type tickingClock struct{ reads int }

func (c *tickingClock) Now() time.Time {
	c.reads++
	return testNow().Add(time.Duration(c.reads-1) * time.Microsecond)
}

// adminsAuthorizer lets a notebook's explicit admins act on it; the rest
// do not see it.
type adminsAuthorizer struct{ facts notebook.Facts }

func (a adminsAuthorizer) Authorize(ctx context.Context, actor shared.Actor, _ shared.Action, t shared.Target) (shared.Grant, error) {
	f, err := a.facts.NotebookFacts(ctx, t.NotebookID, actor.UserID)
	switch {
	case err != nil:
		return shared.Grant{}, err
	case f.Role != shared.NotebookAdmin || f.WorkspaceID != t.WorkspaceID:
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{WorkspaceRole: shared.WorkspaceMember, NotebookRole: f.Role}, nil
}

// sqlWorkspaces stands in for the workspace module's port.
type sqlWorkspaces struct{ pool *pgxpool.Pool }

func (w sqlWorkspaces) FindBySlug(ctx context.Context, slug string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := postgres.DB(ctx, w.pool).QueryRow(ctx, "SELECT id FROM workspaces WHERE slug = $1 AND deleted_at IS NULL", slug).Scan(&id)
	return id, err == nil, nil
}

func (w sqlWorkspaces) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	var found uuid.UUID
	err := postgres.DB(ctx, w.pool).QueryRow(ctx, "SELECT id FROM workspaces WHERE id = $1 AND deleted_at IS NULL FOR SHARE", id).Scan(&found)
	return err == nil, nil
}

// tokenAuth takes the bearer token for the caller's account id.
type tokenAuth struct{}

func (tokenAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	id, err := uuid.Parse(token)
	if err != nil {
		return nil, "", shared.Unauthenticated()
	}
	return shared.WithActor(ctx, shared.Actor{UserID: id, SessionID: uuid.NewV7()}), "session:" + token, nil
}

// fixture is acme on a database of its own, alice the admin of its
// notebooks eng and ops, bob a reader of eng; and other, a workspace with
// a notebook of its own.
type fixture struct {
	pool              *pgxpool.Pool
	acme, other       uuid.UUID
	alice, bob        uuid.UUID
	eng, ops, outside uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 8})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f := fixture{pool: pool, acme: uuid.NewV7(), other: uuid.NewV7(), alice: uuid.NewV7(), bob: uuid.NewV7(),
		eng: uuid.NewV7(), ops: uuid.NewV7(), outside: uuid.NewV7()}
	f.exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES "+
		"($1, 'alice@corp.com', 'x', 'Alice', $3, $3), ($2, 'bob@corp.com', 'x', 'Bob', $3, $3)", f.alice, f.bob, testNow())
	f.exec(t, "INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) "+
		"VALUES ($1, 'acme', 'Acme', $3, $3, $4, $4), ($2, 'other', 'Other', $3, $3, $4, $4)", f.acme, f.other, f.alice, testNow())
	for id, workspace := range map[uuid.UUID]uuid.UUID{f.eng: f.acme, f.ops: f.acme, f.outside: f.other} {
		f.exec(t, "INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) "+
			"VALUES ($1, $2, 'Notes', $3, $3, $4, $4)", id, workspace, f.alice, testNow().Add(-time.Hour))
		f.exec(t, "INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at) "+
			"VALUES ($1, $2, $3, 'admin', $3, $3, $4, $4)", uuid.NewV7(), id, f.alice, testNow().Add(-time.Hour))
	}
	f.exec(t, "INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at, ended_at) "+
		"VALUES ($1, $2, $3, 'reader', $4, $4, $5, $5, $5)", uuid.NewV7(), f.eng, f.bob, f.alice, testNow().Add(-time.Hour))
	return f
}

func (f fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// deletedAt is how many rows of notebook id were deleted at at: the
// notebook's, and its members'.
func (f fixture) deletedAt(t *testing.T, id uuid.UUID, at time.Time) (notebooks, members int) {
	t.Helper()
	if err := f.pool.QueryRow(context.Background(), `SELECT
		(SELECT count(*) FROM notebooks WHERE id = $1 AND deleted_at = $2),
		(SELECT count(*) FROM notebook_members WHERE notebook_id = $1 AND deleted_at = $2)`, id, at).Scan(&notebooks, &members); err != nil {
		t.Fatal(err)
	}
	return notebooks, members
}

// subscriber fails when fail is set, and records what it saw in the
// transaction: the deletions, and the notebooks' deletion times.
type subscriber struct {
	f       fixture
	fail    bool
	got     []notebook.NotebookDeletion
	inTx    bool
	deleted []time.Time
}

func (s *subscriber) NotebookDeleted(ctx context.Context, d notebook.NotebookDeletion) error {
	s.got = append(s.got, d)
	s.inTx = postgres.InTx(ctx)
	rows, err := postgres.DB(ctx, s.f.pool).Query(ctx, "SELECT deleted_at FROM notebooks WHERE id = ANY($1) ORDER BY id", d.NotebookIDs)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var at *time.Time
		if err := rows.Scan(&at); err != nil {
			return err
		}
		if at != nil {
			s.deleted = append(s.deleted, *at)
		}
	}
	if s.fail {
		return errors.New("the subscriber failed")
	}
	return rows.Err()
}

// The subscriber runs after the notebook and its members were deleted, in
// their transaction, with their time; its failure rolls everything back.
func TestANotebookDeletionPassesTheSubscriber(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "the subscriber follows", true: "the subscriber fails"}[fail], func(t *testing.T) {
			f := newFixture(t)
			s := &subscriber{f: f, fail: fail}
			router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
			notebook.New(notebook.Deps{
				Pool: f.pool, Tx: postgres.NewTxManager(f.pool, 5*time.Second), Clock: &tickingClock{},
				Logger: slog.New(slog.DiscardHandler), Authorizer: adminsAuthorizer{facts: notebook.NewFacts(f.pool)},
				Workspaces: sqlWorkspaces{f.pool}, DeletionSubscribers: []notebook.NotebookDeletionSubscriber{s},
			}).Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: tokenAuth{}}))

			req := httptest.NewRequest(http.MethodDelete, "/api/v0/notebooks/"+f.eng.String(), nil)
			req.Header.Set("Authorization", "Bearer "+f.alice.String())
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			notebooks, members := f.deletedAt(t, f.eng, testNow())
			want := notebook.NotebookDeletion{WorkspaceID: f.acme, NotebookIDs: []uuid.UUID{f.eng}, By: f.alice, At: testNow()}
			switch {
			case len(s.got) != 1 || !sameDeletion(s.got[0], want) || !s.inTx || !slices.Equal(s.deleted, []time.Time{testNow()}):
				t.Errorf("the subscriber got %+v in a transaction %v, saw %v; want %+v, the notebook deleted", s.got, s.inTx, s.deleted, want)
			case !fail && (rec.Code != http.StatusNoContent || notebooks != 1 || members != 2):
				t.Errorf("DELETE = %d %s, %d notebooks and %d members deleted at %v; want 204, eng and both its members", rec.Code, rec.Body, notebooks, members, testNow())
			case fail && (rec.Code != http.StatusInternalServerError || notebooks != 0 || members != 0):
				t.Errorf("DELETE with a failing subscriber = %d, %d notebooks and %d members deleted; want 500, none", rec.Code, notebooks, members)
			}
		})
	}
}

func sameDeletion(a, b notebook.NotebookDeletion) bool {
	return a.WorkspaceID == b.WorkspaceID && slices.Equal(a.NotebookIDs, b.NotebookIDs) && a.By == b.By && a.At.Equal(b.At)
}

// The registrant of the workspace module's deletion, in the deletion's
// transaction: the workspace's notebooks not deleted, with their members,
// at the deletion's time; a notebook deleted before keeps its own; another
// workspace's stays; the subscribers get every id once, in order.
func TestAWorkspaceDeletionDeletesItsNotebooks(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "the subscriber follows", true: "the subscriber fails"}[fail], func(t *testing.T) {
			f := newFixture(t)
			gone := testNow().Add(-time.Minute)
			f.exec(t, "UPDATE notebooks SET deleted_at = $2 WHERE id = $1", f.ops, gone)
			f.exec(t, "UPDATE notebook_members SET deleted_at = $2 WHERE notebook_id = $1", f.ops, gone)
			third := uuid.NewV7()
			f.exec(t, "INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) "+
				"VALUES ($1, $2, 'Third', $3, $3, $4, $4)", third, f.acme, f.alice, testNow())
			s := &subscriber{f: f, fail: fail}
			deletion := notebook.NewWorkspaceDeletion(f.pool, []notebook.NotebookDeletionSubscriber{s})

			err := postgres.NewTxManager(f.pool, 5*time.Second).WithinTx(context.Background(), func(ctx context.Context) error {
				return deletion.WorkspaceDeleted(ctx, notebook.WorkspaceDeleted{WorkspaceID: f.acme, By: f.bob, At: testNow()})
			})

			ids := []uuid.UUID{f.eng, third}
			slices.SortFunc(ids, uuid.UUID.Compare)
			want := notebook.NotebookDeletion{WorkspaceID: f.acme, NotebookIDs: ids, By: f.bob, At: testNow()}
			if len(s.got) != 1 || !sameDeletion(s.got[0], want) || !s.inTx {
				t.Errorf("the subscriber got %+v in a transaction %v; want %+v once", s.got, s.inTx, want)
			}
			engs, engMembers := f.deletedAt(t, f.eng, testNow())
			thirds, _ := f.deletedAt(t, third, testNow())
			opses, opsMembers := f.deletedAt(t, f.ops, gone)
			outsides, _ := f.deletedAt(t, f.outside, testNow())
			if opses != 1 || opsMembers != 1 || outsides != 0 {
				t.Errorf("ops %d with %d members at its own time, the other workspace's %d; want ops untouched, the other's too", opses, opsMembers, outsides)
			}
			switch {
			case !fail && (err != nil || engs != 1 || engMembers != 2 || thirds != 1):
				t.Errorf("WorkspaceDeleted() = %v; eng %d with %d members, third %d at %v; want eng with both members and third", err, engs, engMembers, thirds, testNow())
			case fail && (err == nil || engs != 0 || engMembers != 0 || thirds != 0):
				t.Errorf("WorkspaceDeleted() with a failing subscriber = %v; eng %d with %d members, third %d; want the failure, none", err, engs, engMembers, thirds)
			}
		})
	}
}
