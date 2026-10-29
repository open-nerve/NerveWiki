package notebook_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
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

// adminsAuthorizer lets a notebook's explicit admins act on it, and anyone
// handle the ownerless notebooks, as a workspace's admin; the rest do not
// see it.
type adminsAuthorizer struct{ facts notebook.Facts }

func (a adminsAuthorizer) Authorize(ctx context.Context, actor shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	if ownerlessAction(action) {
		return shared.Grant{WorkspaceRole: shared.WorkspaceAdmin}, nil
	}
	f, err := a.facts.NotebookFacts(ctx, t.NotebookID, actor.UserID)
	switch {
	case err != nil:
		return shared.Grant{}, err
	case f.Role != shared.NotebookAdmin || f.WorkspaceID != t.WorkspaceID:
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{WorkspaceRole: shared.WorkspaceMember, NotebookRole: f.Role}, nil
}

// ownerlessAction reports whether action is one of the ownerless
// notebooks', which a workspace's admins may do.
func ownerlessAction(action shared.Action) bool {
	return strings.HasPrefix(string(action), "notebook_ownerless.")
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

// orphanEng ends alice's membership of eng, its only admin's, and leaves
// it ownerless of her, a minute before the clock's instant.
func (f fixture) orphanEng(t *testing.T) {
	t.Helper()
	at := testNow().Add(-time.Minute)
	f.exec(t, "UPDATE notebook_members SET ended_at = $3, updated_at = $3 WHERE notebook_id = $1 AND user_id = $2", f.eng, f.alice, at)
	f.exec(t, "UPDATE notebooks SET ownerless_since = $2, former_owner_id = $3 WHERE id = $1", f.eng, at, f.alice)
}

// count is the one number query reads.
func (f fixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
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
// their transaction, with their time; its failure rolls everything back,
// the ownerless deletion's audit event too.
func TestANotebookDeletionPassesTheSubscriber(t *testing.T) {
	for _, d := range []struct {
		name      string
		ownerless bool
		path      func(f fixture) string
		by        func(f fixture) uuid.UUID
	}{
		{"by its admin", false, func(f fixture) string { return "/api/v0/notebooks/" + f.eng.String() },
			func(f fixture) uuid.UUID { return f.alice }},
		{"ownerless, by a workspace admin", true, func(f fixture) string { return "/api/v0/ownerless-notebooks/" + f.eng.String() },
			func(f fixture) uuid.UUID { return f.bob }},
	} {
		for _, fail := range []bool{false, true} {
			t.Run(d.name+map[bool]string{false: ", the subscriber follows", true: ", the subscriber fails"}[fail], func(t *testing.T) {
				f := newFixture(t)
				if d.ownerless {
					f.orphanEng(t)
				}
				s := &subscriber{f: f, fail: fail}
				router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
				notebook.New(notebook.Deps{
					Pool: f.pool, Tx: postgres.NewTxManager(f.pool, 5*time.Second), Clock: &tickingClock{},
					Logger: slog.New(slog.DiscardHandler), Authorizer: adminsAuthorizer{facts: notebook.NewFacts(f.pool)},
					Workspaces: sqlWorkspaces{f.pool}, DeletionSubscribers: []notebook.NotebookDeletionSubscriber{s},
					AccountWorkspaces: noAccountWorkspaces{}, Seeders: []notebook.Seeder{noSeed{}}, WritingJobs: noJobs{}, SourceHashes: noSourceHashes{},
				}).Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: tokenAuth{}}))

				req := httptest.NewRequest(http.MethodDelete, d.path(f), nil)
				req.Header.Set("Authorization", "Bearer "+d.by(f).String())
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)

				notebooks, members := f.deletedAt(t, f.eng, testNow())
				events := f.count(t, "SELECT count(*) FROM notebook_audit_events WHERE action = 'deleted' AND notebook_id = $1 "+
					"AND created_by_id = $2 AND created_at = $3", f.eng, d.by(f), testNow())
				wantEvents := 0
				if d.ownerless && !fail {
					wantEvents = 1
				}
				want := notebook.NotebookDeletion{WorkspaceID: f.acme, NotebookIDs: []uuid.UUID{f.eng}, By: d.by(f), At: testNow()}
				switch {
				case len(s.got) != 1 || !sameDeletion(s.got[0], want) || !s.inTx || !slices.Equal(s.deleted, []time.Time{testNow()}):
					t.Errorf("the subscriber got %+v in a transaction %v, saw %v; want %+v, the notebook deleted", s.got, s.inTx, s.deleted, want)
				case !fail && (rec.Code != http.StatusNoContent || notebooks != 1 || members != 2):
					t.Errorf("DELETE = %d %s, %d notebooks and %d members deleted at %v; want 204, eng and both its members", rec.Code, rec.Body,
						notebooks, members, testNow())
				case fail && (rec.Code != http.StatusInternalServerError || notebooks != 0 || members != 0):
					t.Errorf("DELETE with a failing subscriber = %d, %d notebooks and %d members deleted; want 500, none", rec.Code, notebooks, members)
				case events != wantEvents:
					t.Errorf("%d audit events of the deletion, want %d", events, wantEvents)
				}
			})
		}
	}
}

// creatorsAuthorizer lets anyone create a notebook, as a workspace's
// member, and decides the rest as adminsAuthorizer: the creator is the
// notebook's admin only once it is written.
type creatorsAuthorizer struct{ admins adminsAuthorizer }

func (a creatorsAuthorizer) Authorize(ctx context.Context, actor shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	if action == "notebook.create" {
		return shared.Grant{WorkspaceRole: shared.WorkspaceMember}, nil
	}
	return a.admins.Authorize(ctx, actor, action, t)
}

// seeder stands in for the wiki module's template: it records in order
// what it got, in the creation's transaction, whether it saw the notebook
// and its admin there, and answers what answer gives, or err.
type seeder struct {
	f      fixture
	name   string
	order  *[]string
	answer func(c notebook.NotebookCreation) uuid.UUID
	err    error
	got    []notebook.NotebookCreation
	inTx   bool
	saw    int
}

func (s *seeder) NotebookCreated(ctx context.Context, c notebook.NotebookCreation) (uuid.UUID, error) {
	*s.order = append(*s.order, s.name)
	s.got = append(s.got, c)
	s.inTx = postgres.InTx(ctx)
	if err := postgres.DB(ctx, s.f.pool).QueryRow(ctx, "SELECT count(*) FROM notebooks n JOIN notebook_members m ON m.notebook_id = n.id "+
		"WHERE n.id = $1 AND m.user_id = $2 AND m.role = 'admin'", c.NotebookID, c.By).Scan(&s.saw); err != nil {
		return uuid.UUID{}, err
	}
	if s.err != nil {
		return uuid.UUID{}, s.err
	}
	if s.answer == nil {
		return uuid.UUID{}, nil
	}
	return s.answer(c), nil
}

// The creation's seeders run in order in its transaction, after the
// notebook and its admin, which they see, with the creation as it was
// asked, at the clock's first read (M10/P1 design 3.5). Exactly one answers
// an LLM Wiki's source root, the one written in its row; none answers a
// standard notebook's: else the creation is a 500 and writes nothing. A
// seeder's failure stops the next one; its problem is a 500, never its
// code.
func TestACreationPassesTheSeeders(t *testing.T) {
	its := func(c notebook.NotebookCreation) uuid.UUID { return c.SourceRootID }
	another := func(notebook.NotebookCreation) uuid.UUID { return uuid.NewV7() }
	failure := errors.New("the seeder failed")
	protected := shared.NewError(shared.KindConflict, "page.protected", "An LLM Wiki keeps its AGENTS page.")
	for _, c := range []struct {
		name          string
		body          string
		first, second func(notebook.NotebookCreation) uuid.UUID
		firstErr      error
		code          int
		called        []string
	}{
		{"standard, both answer none", `{"name":"Wiki"}`, nil, nil, nil, http.StatusCreated, []string{"a", "b"}},
		{"an LLM Wiki, the first answers its root", `{"name":"Wiki","mode":"llm_wiki","template_locale":"zh-CN"}`, its, nil, nil,
			http.StatusCreated, []string{"a", "b"}},
		{"an LLM Wiki, both answer none", `{"name":"Wiki","mode":"llm_wiki","template_locale":"zh-CN"}`, nil, nil, nil,
			http.StatusInternalServerError, []string{"a", "b"}},
		{"an LLM Wiki, both answer its root", `{"name":"Wiki","mode":"llm_wiki","template_locale":"zh-CN"}`, its, its, nil,
			http.StatusInternalServerError, []string{"a", "b"}},
		{"an LLM Wiki, the first answers another id", `{"name":"Wiki","mode":"llm_wiki","template_locale":"zh-CN"}`, another, nil, nil,
			http.StatusInternalServerError, []string{"a", "b"}},
		{"standard, the first answers one", `{"name":"Wiki"}`, another, nil, nil, http.StatusInternalServerError, []string{"a", "b"}},
		{"the first fails", `{"name":"Wiki","mode":"llm_wiki","template_locale":"zh-CN"}`, its, nil, failure,
			http.StatusInternalServerError, []string{"a"}},
		{"the first refuses", `{"name":"Wiki","mode":"llm_wiki","template_locale":"zh-CN"}`, its, nil, protected,
			http.StatusInternalServerError, []string{"a"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			var order []string
			a := &seeder{f: f, name: "a", order: &order, answer: c.first, err: c.firstErr}
			b := &seeder{f: f, name: "b", order: &order, answer: c.second}
			router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
			notebook.New(notebook.Deps{
				Pool: f.pool, Tx: postgres.NewTxManager(f.pool, 5*time.Second), Clock: &tickingClock{},
				Logger: slog.New(slog.DiscardHandler), Authorizer: creatorsAuthorizer{adminsAuthorizer{facts: notebook.NewFacts(f.pool)}},
				Workspaces: sqlWorkspaces{f.pool}, AccountWorkspaces: noAccountWorkspaces{}, Seeders: []notebook.Seeder{a, b},
				WritingJobs: noJobs{}, SourceHashes: noSourceHashes{},
			}).Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: tokenAuth{}}))

			req := httptest.NewRequest(http.MethodPost, "/api/v0/workspaces/acme/notebooks", strings.NewReader(c.body))
			req.Header.Set("Authorization", "Bearer "+f.alice.String())
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != c.code || !slices.Equal(order, c.called) {
				t.Fatalf("POST = %d %s, the seeders called %q; want %d, %q", rec.Code, rec.Body, order, c.code, c.called)
			}
			if c.code == http.StatusInternalServerError && !strings.Contains(rec.Body.String(), `"code":"internal_error"`) {
				t.Errorf("POST = %s, want internal_error", rec.Body)
			}
			got := a.got[0]
			mode, locale := notebook.ModeStandard, "en"
			if strings.Contains(c.body, "llm_wiki") {
				mode, locale = notebook.ModeLLMWiki, "zh-CN"
			}
			if got.WorkspaceID != f.acme || got.Mode != mode || string(got.Locale) != locale || got.By != f.alice || got.Client != "web" ||
				!got.At.Equal(testNow()) || (got.SourceRootID == uuid.UUID{}) != (mode == notebook.ModeStandard) {
				t.Errorf("the first seeder got %+v; want acme's %s in %s by alice from the web at %v", got, mode, locale, testNow())
			}
			for _, s := range []*seeder{a, b}[:len(c.called)] {
				if !s.inTx || s.saw != 1 || s.got[0] != got {
					t.Errorf("seeder %s: in a transaction %v, saw the notebook's admin %d times, got %+v; want the same, once, in it",
						s.name, s.inTx, s.saw, s.got)
				}
			}
			var notebooks, members int
			var root *uuid.UUID
			if err := f.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM notebooks WHERE id = $1),
				(SELECT count(*) FROM notebook_members WHERE notebook_id = $1), (SELECT source_root_id FROM notebooks WHERE id = $1)`,
				got.NotebookID).Scan(&notebooks, &members, &root); err != nil {
				t.Fatal(err)
			}
			switch {
			case c.code == http.StatusCreated && (notebooks != 1 || members != 1 || (root != nil) != (mode == notebook.ModeLLMWiki) ||
				root != nil && *root != got.SourceRootID):
				t.Errorf("created %d notebooks with %d members, its source root %v; want it, its admin, the source root %s", notebooks, members,
					root, got.SourceRootID)
			case c.code != http.StatusCreated && (notebooks != 0 || members != 0):
				t.Errorf("a 500 left %d notebooks and %d members, want none", notebooks, members)
			}
		})
	}
}

// modeSubscriber stands in for the event stream's: it records in order
// what it got, in the downgrade's transaction, and the mode it read of the
// notebook's row there, and answers err.
type modeSubscriber struct {
	f     fixture
	name  string
	order *[]string
	err   error
	got   []notebook.ModeChange
	inTx  bool
	saw   string
}

func (s *modeSubscriber) ModeChanged(ctx context.Context, m notebook.ModeChange) error {
	*s.order = append(*s.order, s.name)
	s.got = append(s.got, m)
	s.inTx = postgres.InTx(ctx)
	if err := postgres.DB(ctx, s.f.pool).QueryRow(ctx, "SELECT mode FROM notebooks WHERE id = $1", m.NotebookID).Scan(&s.saw); err != nil {
		return err
	}
	return s.err
}

// The mode subscribers run in order in the downgrade's transaction, after
// the row was written standard by its caller, which they read, with the
// change at the row's time (M10/P1 design 3.5); the first's failure rolls
// everything back, and the second is not called. The row was last
// updated by bob, so alice's downgrade shows on it.
func TestADowngradePassesTheModeSubscribers(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "the subscribers follow", true: "the first fails"}[fail], func(t *testing.T) {
			f := newFixture(t)
			root := uuid.NewV7()
			f.exec(t, "UPDATE notebooks SET mode = 'llm_wiki', source_root_id = $2, updated_by_id = $3 WHERE id = $1", f.eng, root, f.bob)
			var order []string
			a := &modeSubscriber{f: f, name: "a", order: &order}
			if fail {
				a.err = errors.New("the subscriber failed")
			}
			b := &modeSubscriber{f: f, name: "b", order: &order}
			router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
			notebook.New(notebook.Deps{
				Pool: f.pool, Tx: postgres.NewTxManager(f.pool, 5*time.Second), Clock: &tickingClock{},
				Logger: slog.New(slog.DiscardHandler), Authorizer: adminsAuthorizer{facts: notebook.NewFacts(f.pool)},
				Workspaces: sqlWorkspaces{f.pool}, AccountWorkspaces: noAccountWorkspaces{}, Seeders: []notebook.Seeder{noSeed{}},
				ModeSubscribers: []notebook.ModeSubscriber{a, b}, WritingJobs: noJobs{}, SourceHashes: noSourceHashes{},
			}).Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: tokenAuth{}}))

			req := httptest.NewRequest(http.MethodPost, "/api/v0/notebooks/"+f.eng.String()+"/downgrade", nil)
			req.Header.Set("Authorization", "Bearer "+f.alice.String())
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			var mode string
			var sourceRoot *uuid.UUID
			var updated time.Time
			var updatedBy uuid.UUID
			if err := f.pool.QueryRow(context.Background(), "SELECT mode, source_root_id, updated_at, updated_by_id FROM notebooks WHERE id = $1",
				f.eng).Scan(&mode, &sourceRoot, &updated, &updatedBy); err != nil {
				t.Fatal(err)
			}
			want := notebook.ModeChange{WorkspaceID: f.acme, NotebookID: f.eng, From: notebook.ModeLLMWiki, To: notebook.ModeStandard,
				By: f.alice, At: testNow()}
			if len(a.got) != 1 || a.got[0].WorkspaceID != want.WorkspaceID || a.got[0].NotebookID != want.NotebookID ||
				a.got[0].From != want.From || a.got[0].To != want.To || a.got[0].By != want.By || !a.got[0].At.Equal(want.At) ||
				!a.inTx || a.saw != "standard" {
				t.Errorf("the first subscriber got %+v in a transaction %v, read %q; want %+v in it, the row standard", a.got, a.inTx, a.saw, want)
			}
			switch {
			case !fail && (rec.Code != http.StatusOK || !slices.Equal(order, []string{"a", "b"}) || mode != "standard" || sourceRoot != nil ||
				!updated.Equal(a.got[0].At) || updatedBy != f.alice || !b.inTx || b.saw != "standard" || len(b.got) != 1 || b.got[0] != a.got[0]):
				t.Errorf("POST = %d %s, called %q, the row %s with source root %v at %v by %s, the second got %+v; want 200, a then b, "+
					"standard without one at the change's time by alice, the same change", rec.Code, rec.Body, order, mode, sourceRoot, updated,
					updatedBy, b.got)
			case fail && (rec.Code != http.StatusInternalServerError || !slices.Equal(order, []string{"a"}) || mode != "llm_wiki" ||
				sourceRoot == nil || *sourceRoot != root || updatedBy != f.bob):
				t.Errorf("POST with a failing subscriber = %d, called %q, the row %s with source root %v by %s; want 500, a alone, "+
					"llm_wiki with its own by bob", rec.Code, order, mode, sourceRoot, updatedBy)
			}
		})
	}
}

func sameDeletion(a, b notebook.NotebookDeletion) bool {
	return a.WorkspaceID == b.WorkspaceID && slices.Equal(a.NotebookIDs, b.NotebookIDs) && a.By == b.By && a.At.Equal(b.At)
}

// The registrant of the workspace module's deletion, in the deletion's
// transaction: the workspace's notebooks not deleted, with their members,
// and its audit events, at the deletion's time; a notebook deleted before
// keeps its own; another workspace's stay; the subscribers get every id
// once, in order.
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
			acmes, others := f.auditEvent(t, f.acme), f.auditEvent(t, f.other)

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
			acmeDeleted, otherDeleted := f.auditDeletedBy(t, acmes, f.bob, testNow()), f.auditDeletedBy(t, others, f.bob, testNow())
			if acmeDeleted == fail || otherDeleted {
				t.Errorf("acme's audit event deleted %v, the other's %v; want acme's %v, the other's not", acmeDeleted, otherDeleted, !fail)
			}
		})
	}
}

// The registrant deletes the audit events of a workspace whose notebooks
// are all gone: an event outlives its notebook.
func TestAWorkspaceDeletionDeletesTheEventsOfGoneNotebooks(t *testing.T) {
	f := newFixture(t)
	f.exec(t, "DELETE FROM notebook_members WHERE notebook_id IN (SELECT id FROM notebooks WHERE workspace_id = $1)", f.acme)
	f.exec(t, "DELETE FROM notebooks WHERE workspace_id = $1", f.acme)
	event := f.auditEvent(t, f.acme)

	err := postgres.NewTxManager(f.pool, 5*time.Second).WithinTx(context.Background(), func(ctx context.Context) error {
		return notebook.NewWorkspaceDeletion(f.pool, nil).WorkspaceDeleted(ctx, notebook.WorkspaceDeleted{WorkspaceID: f.acme, By: f.bob, At: testNow()})
	})

	if err != nil || !f.auditDeletedBy(t, event, f.bob, testNow()) {
		t.Errorf("WorkspaceDeleted() = %v; want the event deleted by bob at %v", err, testNow())
	}
}

// auditEvent inserts an audit event of workspace, alice's notebook taken
// over by bob, and returns its id.
func (f fixture) auditEvent(t *testing.T, workspace uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	f.exec(t, "INSERT INTO notebook_audit_events (id, workspace_id, notebook_id, notebook_name, action, former_owner_id, "+
		"created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, $3, 'Notes', 'taken_over', $4, $5, $5, $6, $6)",
		id, workspace, uuid.NewV7(), f.alice, f.bob, testNow().Add(-time.Hour))
	return id
}

// auditDeletedBy is whether the audit event id was deleted by by at at.
func (f fixture) auditDeletedBy(t *testing.T, id, by uuid.UUID, at time.Time) bool {
	t.Helper()
	var deleted bool
	if err := f.pool.QueryRow(context.Background(), "SELECT deleted_at IS NOT DISTINCT FROM $2 AND updated_at = $2 AND updated_by_id = $3 "+
		"FROM notebook_audit_events WHERE id = $1", id, at, by).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	return deleted
}
