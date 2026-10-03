package page_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The extension points on a real database (M4/P1 design 3.6), the module
// wired as bootstrap wires it, with registrants that stand in for M5's and
// M6's. The ports stand in for the workspace and notebook modules', which
// the page module does not import: they lock the same rows. The decision
// lets alice write eng and no one else see it.

// testNow is the clock's instant, in whole microseconds.
func testNow() time.Time { return time.Date(2026, 10, 2, 10, 0, 0, 123456000, time.UTC) }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return testNow() }

// sqlWorkspaces and sqlNotebooks stand in for the other modules' ports.
type sqlWorkspaces struct{ pool *pgxpool.Pool }

func (w sqlWorkspaces) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	return exists(ctx, w.pool, "SELECT 1 FROM workspaces WHERE id = $1 AND deleted_at IS NULL FOR SHARE", id)
}

type sqlNotebooks struct{ pool *pgxpool.Pool }

func (n sqlNotebooks) WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	var ws uuid.UUID
	err := postgres.DB(ctx, n.pool).QueryRow(ctx, "SELECT workspace_id FROM notebooks WHERE id = $1 AND deleted_at IS NULL", id).Scan(&ws)
	return ws, err == nil, nil
}

func (n sqlNotebooks) ShareByID(ctx context.Context, id uuid.UUID) (bool, error) {
	return exists(ctx, n.pool, "SELECT 1 FROM notebooks WHERE id = $1 AND deleted_at IS NULL FOR SHARE", id)
}

func (n sqlNotebooks) LockByID(ctx context.Context, id uuid.UUID) (bool, error) {
	return exists(ctx, n.pool, "SELECT 1 FROM notebooks WHERE id = $1 AND deleted_at IS NULL FOR NO KEY UPDATE", id)
}

// sqlNames stands in for identity's directory: the accounts' display
// names.
type sqlNames struct{ pool *pgxpool.Pool }

func (n sqlNames) DisplayNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	rows, err := postgres.DB(ctx, n.pool).Query(ctx, "SELECT id, display_name FROM users WHERE id = ANY($1)", ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID]string)
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

func exists(ctx context.Context, pool *pgxpool.Pool, query string, id uuid.UUID) (bool, error) {
	var one int
	err := postgres.DB(ctx, pool).QueryRow(ctx, query, id).Scan(&one)
	return err == nil, nil
}

// aliceWrites lets alice do anything in the notebooks; the rest see none.
type aliceWrites struct{ alice uuid.UUID }

func (a aliceWrites) Authorize(_ context.Context, actor shared.Actor, _ shared.Action, _ shared.Target) (shared.Grant, error) {
	if actor.UserID != a.alice {
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{WorkspaceRole: shared.WorkspaceMember, NotebookRole: shared.NotebookEditor}, nil
}

// tokenAuth takes "session:<account id>" for a sign-in session's access
// token and "pat:<account id>" for a personal access token.
type tokenAuth struct{}

func (tokenAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	kind, id, _ := strings.Cut(token, ":")
	user, err := uuid.Parse(id)
	if err != nil {
		return nil, "", shared.Unauthenticated()
	}
	actor := shared.Actor{UserID: user, SessionID: uuid.NewV7()}
	if kind == "pat" {
		actor = shared.Actor{UserID: user, APITokenID: uuid.NewV7()}
	}
	return shared.WithActor(ctx, actor), token, nil
}

// fixture is acme on a database of its own, with its notebook eng and
// eng's page Notes; alice writes it.
type fixture struct {
	pool             *pgxpool.Pool
	alice            uuid.UUID
	acme, eng, notes uuid.UUID
	md               *markdown.Markdown
	// The edit sessions' registrants serve wires.
	vetoers     []page.EditSessionVetoer
	subscribers []page.EditSessionSubscriber
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 8})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	md, err := markdown.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{pool: pool, alice: uuid.NewV7(), acme: uuid.NewV7(), eng: uuid.NewV7(), notes: uuid.NewV7(), md: md}
	before := testNow().Add(-time.Hour)
	f.exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'Alice', $2, $2)",
		f.alice, before)
	f.exec(t, "INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, $3, $3)",
		f.acme, f.alice, before)
	f.exec(t, "INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Eng', $3, $3, $4, $4)",
		f.eng, f.acme, f.alice, before)
	f.exec(t, "INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at) "+
		"VALUES ($1, $2, 'page', 'Notes', 'notes', 0, $3, $3, $4, $4)", f.notes, f.eng, f.alice, before)
	f.exec(t, "INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at) "+
		"VALUES ($1, '', 1, sha256(''), 0, $2, $3)", f.notes, f.alice, before)
	return f
}

func (f fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (f fixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// serve wires the module with the registrants and serves one request of
// alice's by token kind ("session" or "pat").
func (f fixture) serve(t *testing.T, kind, method, path, body string, guards []page.WriteGuard, participants []page.Participant,
	observers []page.PageObserver,
) *httptest.ResponseRecorder {
	t.Helper()
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	page.New(page.Deps{
		Pool: f.pool, Tx: postgres.NewTxManager(f.pool, 5*time.Second), Clock: fixedClock{}, Logger: slog.New(slog.DiscardHandler),
		Authorizer: aliceWrites{f.alice}, Workspaces: sqlWorkspaces{f.pool}, Notebooks: sqlNotebooks{f.pool}, Names: sqlNames{f.pool},
		Markdown: f.md, Guards: guards, Participants: participants, Observers: observers,
		EditSessionVetoers: f.vetoers, EditSessionSubscribers: f.subscribers, EditSessionCleanupInterval: time.Hour,
		ParseBudgetBytes: 8 << 20, ParseMaxWait: time.Second,
	}).Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: tokenAuth{}}))
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+kind+":"+f.alice.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func (f fixture) createPath() string { return "/api/v0/notebooks/" + f.eng.String() + "/pages" }
func (f fixture) renamePath() string { return "/api/v0/nodes/" + f.notes.String() }

// guard records what it sees, in the transaction, and answers err.
type guard struct {
	err   error
	steps []page.Step
	inTx  bool
}

func (g *guard) GuardWrite(ctx context.Context, s page.Step) error {
	g.steps = append(g.steps, s)
	g.inTx = postgres.InTx(ctx)
	return g.err
}

// observer records the events it follows and the changesets it sees in
// the transaction, and answers err.
type observer struct {
	f          fixture
	err        error
	events     []page.Event
	changesets int
}

func (o *observer) PagesChanged(ctx context.Context, e page.Event) error {
	o.events = append(o.events, e)
	if err := postgres.DB(ctx, o.f.pool).QueryRow(ctx, "SELECT count(*) FROM changesets WHERE id = $1", e.ChangesetID).Scan(&o.changesets); err != nil {
		return err
	}
	return o.err
}

// renamer renames Notes to Journal after each operation it follows.
type renamer struct {
	f     fixture
	calls int
}

func (r *renamer) Participate(ctx context.Context, _ page.Step, u page.Appender) error {
	r.calls++
	_, err := u.Rename(ctx, r.f.notes, "Journal")
	return err
}

// A guard's refusal rolls the unit back and is its answer. It runs in the
// unit's transaction and sees where the node was and would be.
func TestAGuardRefusesTheUnit(t *testing.T) {
	locked := shared.NewError(shared.KindConflict, "page.locked", "Locked.")
	for _, tt := range []struct {
		name, method, body string
		path               func(f fixture) string
		before, after      string
	}{
		{"a creation", http.MethodPost, `{"parent_id":null,"title":"New"}`, fixture.createPath, "", "New"},
		{"a rename", http.MethodPatch, `{"name":"Renamed"}`, fixture.renamePath, "Notes", "Renamed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			g := &guard{err: locked}
			rec := f.serve(t, "session", tt.method, tt.path(f), tt.body, []page.WriteGuard{g}, nil, nil)
			if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"code":"page.locked"`) {
				t.Errorf("%s = %d %s, want 409 page.locked", tt.name, rec.Code, rec.Body)
			}
			if len(g.steps) != 1 || !g.inTx || len(g.steps[0].Changes) != 1 {
				t.Fatalf("the guard saw %+v in a transaction %v, want one step", g.steps, g.inTx)
			}
			c := g.steps[0].Changes[0]
			before := ""
			if c.Before != nil {
				before = c.Before.Name
			}
			if before != tt.before || c.After.Name != tt.after {
				t.Errorf("the guard saw %q to %q, want %q to %q", before, c.After.Name, tt.before, tt.after)
			}
			if n := f.count(t, "SELECT count(*) FROM nodes WHERE name <> 'Notes'") + f.count(t, "SELECT count(*) FROM changesets"); n != 0 {
				t.Errorf("%d rows written beside the refusal, want none", n)
			}
		})
	}
}

// The observers follow a unit once, in its transaction, with its
// changeset; an observer's error rolls it back and is a 500.
func TestTheObserversFollowAUnit(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "the observer follows", true: "the observer fails"}[fail], func(t *testing.T) {
			f := newFixture(t)
			o := &observer{f: f}
			if fail {
				o.err = errors.New("the observer failed")
			}
			rec := f.serve(t, "session", http.MethodPost, f.createPath(), `{"parent_id":null,"title":"New"}`, nil, nil, []page.PageObserver{o})
			if len(o.events) != 1 || o.changesets != 1 || o.events[0].NotebookID != f.eng || o.events[0].By != f.alice ||
				len(o.events[0].Changes) != 1 || !o.events[0].At.Equal(testNow()) {
				t.Errorf("the observer followed %+v, seeing %d changesets; want one event of the creation, its changeset", o.events, o.changesets)
			}
			news := f.count(t, "SELECT count(*) FROM nodes WHERE name = 'New' AND created_at = $1", testNow())
			switch {
			case !fail && (rec.Code != http.StatusCreated || news != 1):
				t.Errorf("POST = %d %s with %d pages New, want 201 and the page", rec.Code, rec.Body, news)
			case fail && (rec.Code != http.StatusInternalServerError || news != 0 || f.count(t, "SELECT count(*) FROM changesets") != 0):
				t.Errorf("POST with a failing observer = %d with %d pages New, want 500 and nothing written", rec.Code, news)
			}
		})
	}
}

// A participant's rename runs through the guards into the unit's
// changeset and its one event; it calls no participant. The answer is
// the page as the unit left it.
func TestAParticipantAddsToTheUnit(t *testing.T) {
	f := newFixture(t)
	g, o, p := &guard{}, &observer{f: f}, &renamer{f: f}
	rec := f.serve(t, "session", http.MethodPost, f.createPath(), `{"parent_id":null,"title":"New"}`,
		[]page.WriteGuard{g}, []page.Participant{p}, []page.PageObserver{o})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s, want 201", rec.Code, rec.Body)
	}
	if p.calls != 1 || len(g.steps) != 2 || g.steps[1].Operation != domain.OpRename {
		t.Errorf("the participant ran %d times, the guard saw %+v; want once, and the creation then the rename", p.calls, g.steps)
	}
	if n := f.count(t, "SELECT count(*) FROM changesets"); n != 1 {
		t.Errorf("%d changesets, want 1", n)
	}
	if n := f.count(t, "SELECT count(*) FROM changeset_items i JOIN changesets s ON s.id = i.changeset_id "+
		"WHERE (i.node_id = $1 AND i.before_name = 'Notes' AND i.after_name = 'Journal') OR i.after_name = 'New'", f.notes); n != 2 {
		t.Errorf("%d items of the creation and the rename in the changeset, want 2", n)
	}
	if len(o.events) != 1 || len(o.events[0].Changes) != 2 {
		t.Errorf("events = %+v, want one with both changes", o.events)
	}
}

// The changeset's client is the credential's: a sign-in session's access
// token is the web, a personal access token the API.
func TestTheClientIsTheCredentials(t *testing.T) {
	for kind, want := range map[string]string{"session": "web", "pat": "api"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			if rec := f.serve(t, kind, http.MethodPatch, f.renamePath(), `{"name":"Renamed"}`, nil, nil, nil); rec.Code != http.StatusOK {
				t.Fatalf("PATCH = %d %s, want 200", rec.Code, rec.Body)
			}
			if n := f.count(t, "SELECT count(*) FROM changesets WHERE client = $1 AND created_by_id = $2", want, f.alice); n != 1 {
				t.Errorf("%d changesets of the client %s by alice, want 1", n, want)
			}
		})
	}
}

// A unit opens the outermost transaction: in one already, it refuses, and
// writes nothing.
func TestAUnitRefusesAnOuterTransaction(t *testing.T) {
	f := newFixture(t)
	tx := postgres.NewTxManager(f.pool, 5*time.Second)
	store := postgresadapter.New(f.pool)
	writer := app.NewWriter(app.WriterDeps{
		Tx: tx, Clock: fixedClock{}, Auth: aliceWrites{f.alice}, Workspaces: sqlWorkspaces{f.pool}, Notebooks: sqlNotebooks{f.pool},
		Nodes: store, NodeWriter: store, Changesets: store,
	})
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: f.alice, SessionID: uuid.NewV7()})
	err := tx.WithinTx(ctx, func(ctx context.Context) error {
		_, err := writer.Run(ctx, app.UnitSpec{NotebookID: f.eng, Action: domain.ActionCreate, Tree: true, Client: domain.ClientWeb,
			NotFound: domain.ErrNotebookNotFound}, func(ctx context.Context, u *app.Unit) error {
			_, err := u.CreatePage(ctx, app.PageDraft{Title: "New"})
			return err
		})
		return err
	})
	if err == nil || f.count(t, "SELECT count(*) FROM nodes WHERE name = 'New'") != 0 {
		t.Errorf("a unit in a transaction = %v, want an error and no page", err)
	}
}

// The registrant of the notebook module's deletion deletes the notebooks'
// pages, what follows them and their changesets at the deletion's time,
// and their edit sessions: the subscribers follow the end of the one alive
// then, not of the one expired.
func TestANotebookDeletionDeletesItsPages(t *testing.T) {
	f := newFixture(t)
	if rec := f.serve(t, "session", http.MethodPost, f.createPath(), `{"parent_id":"`+f.notes.String()+`","title":"Child"}`,
		nil, nil, nil); rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s, want 201", rec.Code, rec.Body)
	}
	alive := f.openSession(t)
	expired := f.openSession(t)
	f.exec(t, "UPDATE edit_sessions SET expires_at = created_at + interval '1 second' WHERE id = $1", expired)
	sub := &subscriber{}
	at := testNow().Add(30 * time.Second)
	err := postgres.NewTxManager(f.pool, 5*time.Second).WithinTx(context.Background(), func(ctx context.Context) error {
		return page.NewNotebookDeletion(f.pool, []page.EditSessionSubscriber{sub}).NotebookDeleted(ctx,
			page.NotebookDeleted{WorkspaceID: f.acme, NotebookIDs: []uuid.UUID{f.eng}, By: f.alice, At: at})
	})
	if err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int{"nodes": 2, "page_contents": 2, "page_revisions": 1, "changeset_items": 1, "changesets": 1} {
		if n := f.count(t, "SELECT count(*) FROM "+table+" WHERE deleted_at = $1", at); n != want {
			t.Errorf("%d rows of %s deleted at the deletion's time, want %d", n, table, want)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM edit_sessions"); n != 0 {
		t.Errorf("%d edit sessions left, want none", n)
	}
	want := []page.SessionEnded{{SessionID: alive, WorkspaceID: f.acme, NotebookID: f.eng, PageID: f.notes, UserID: f.alice, Reason: domain.EndedWithPage,
		By: f.alice, At: at}}
	if !reflect.DeepEqual(sub.ended, want) || !sub.inTx {
		t.Errorf("the subscriber followed %+v in a transaction %v, want %+v", sub.ended, sub.inTx, want)
	}
}

// openSession opens alice's edit session of Notes through the route.
func (f fixture) openSession(t *testing.T) uuid.UUID {
	t.Helper()
	rec := f.serve(t, "session", http.MethodPost, "/api/v0/pages/"+f.notes.String()+"/edit-sessions", "", nil, nil, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST an edit session = %d %s, want 201", rec.Code, rec.Body)
	}
	var s struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return s.ID
}

// subscriber records the openings and the ends it follows, and whether in
// a transaction.
type subscriber struct {
	opened []page.SessionOpened
	ended  []page.SessionEnded
	inTx   bool
}

func (s *subscriber) EditSessionOpened(ctx context.Context, o page.SessionOpened) error {
	s.opened = append(s.opened, o)
	s.inTx = postgres.InTx(ctx)
	return nil
}

func (s *subscriber) EditSessionEnded(ctx context.Context, e page.SessionEnded) error {
	s.ended = append(s.ended, e)
	s.inTx = postgres.InTx(ctx)
	return nil
}

// vetoer records the openings it sees, and whether the page's gate is
// held then, and refuses them with err.
type vetoer struct {
	f        fixture
	err      error
	openings []page.SessionOpening
	gated    bool
}

func (v *vetoer) VetoEditSession(ctx context.Context, o page.SessionOpening) error {
	v.openings = append(v.openings, o)
	// The test's own transaction would wait for the opening's lock: NOWAIT
	// fails at once instead.
	_, err := v.f.pool.Exec(context.Background(), "SELECT 1 FROM page_contents WHERE node_id = $1 FOR NO KEY UPDATE NOWAIT", o.PageID)
	v.gated = err != nil
	return v.err
}

// The edit sessions' registrants reach their paths through page.New: a
// vetoer sees an opening under the page's gate, and its refusal is the
// answer, with no session; an opening tells the subscribers, and an end by
// its owner and a page's deletion tell them with the reason and who, each
// in its transaction.
func TestTheEditSessionsReachTheirRegistrants(t *testing.T) {
	f := newFixture(t)
	locked := shared.NewError(shared.KindConflict, "page.locked", "Someone else is editing this page.")
	v := &vetoer{f: f, err: locked}
	f.vetoers = []page.EditSessionVetoer{v}
	rec := f.serve(t, "pat", http.MethodPost, "/api/v0/pages/"+f.notes.String()+"/edit-sessions", "", nil, nil, nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"code":"page.locked"`) || f.count(t, "SELECT count(*) FROM edit_sessions") != 0 {
		t.Errorf("a vetoed opening = %d %s, want 409 page.locked and no session", rec.Code, rec.Body)
	}
	if len(v.openings) != 1 || v.openings[0].PageID != f.notes || v.openings[0].By != f.alice || v.openings[0].Client != domain.ClientAPI ||
		!v.gated {
		t.Errorf("the vetoer saw %+v, gated %v; want alice's opening of Notes by the API, under the page's gate", v.openings, v.gated)
	}

	v.err = nil
	sub := &subscriber{}
	f.subscribers = []page.EditSessionSubscriber{sub}
	ended := f.openSession(t)
	if rec := f.serve(t, "session", http.MethodDelete, "/api/v0/edit-sessions/"+ended.String(), "", nil, nil, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE the session = %d %s, want 204", rec.Code, rec.Body)
	}
	deleted := f.openSession(t)
	if rec := f.serve(t, "session", http.MethodDelete, f.renamePath(), "", nil, nil, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE Notes = %d %s, want 204", rec.Code, rec.Body)
	}
	want := []page.SessionEnded{
		{SessionID: ended, WorkspaceID: f.acme, NotebookID: f.eng, PageID: f.notes, UserID: f.alice, Reason: domain.EndedByOwner, By: f.alice, At: testNow()},
		{SessionID: deleted, WorkspaceID: f.acme, NotebookID: f.eng, PageID: f.notes, UserID: f.alice, Reason: domain.EndedWithPage, By: f.alice, At: testNow()},
	}
	if !reflect.DeepEqual(sub.ended, want) || !sub.inTx || f.count(t, "SELECT count(*) FROM edit_sessions") != 0 {
		t.Errorf("the subscriber followed %+v in a transaction %v, want %+v and no session left", sub.ended, sub.inTx, want)
	}
	opened := []page.SessionOpened{
		{SessionID: ended, WorkspaceID: f.acme, NotebookID: f.eng, PageID: f.notes, UserID: f.alice, At: testNow()},
		{SessionID: deleted, WorkspaceID: f.acme, NotebookID: f.eng, PageID: f.notes, UserID: f.alice, At: testNow()},
	}
	if !reflect.DeepEqual(sub.opened, opened) {
		t.Errorf("the subscriber followed the openings %+v, want %+v", sub.opened, opened)
	}
}

// page.NewEditLock over the pool, as the opening's vetoer, refuses a
// second opening of a page: 409 page.locked with the lock member, its
// holder named by Deps.Names, and no second session.
func TestTheEditLockRefusesASecondOpening(t *testing.T) {
	f := newFixture(t)
	f.vetoers = []page.EditSessionVetoer{page.NewEditLock(f.pool, sqlNames{f.pool})}
	f.openSession(t)
	rec := f.serve(t, "pat", http.MethodPost, "/api/v0/pages/"+f.notes.String()+"/edit-sessions", "", nil, nil, nil)
	var p struct {
		Code string `json:"code"`
		Lock struct {
			PageID      uuid.UUID `json:"page_id"`
			UserID      uuid.UUID `json:"user_id"`
			DisplayName string    `json:"display_name"`
		} `json:"lock"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || rec.Code != http.StatusConflict || p.Code != "page.locked" ||
		p.Lock.PageID != f.notes || p.Lock.UserID != f.alice || p.Lock.DisplayName != "Alice" {
		t.Errorf("a second opening = %d %s, want 409 page.locked by Alice", rec.Code, rec.Body)
	}
	if n := f.count(t, "SELECT count(*) FROM edit_sessions"); n != 1 {
		t.Errorf("%d edit sessions, want the first alone", n)
	}
}

// A move and a deletion reach the unit through their routes: the guard and
// the observers see each in its transaction. The move's item has the page's
// places; the deletion's goes to the trash with its node; each changeset is
// of its credential's client, at the unit's time.
func TestTheTreesWritesReachTheUnit(t *testing.T) {
	f := newFixture(t)
	child := uuid.NewV7()
	f.exec(t, "INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at) "+
		"VALUES ($1, $2, $3, 'page', 'Child', 'child', 0, $4, $4, $5, $5)", child, f.eng, f.notes, f.alice, testNow().Add(-time.Hour))
	f.exec(t, "INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at) "+
		"VALUES ($1, '', 1, sha256(''), 0, $2, $3)", child, f.alice, testNow().Add(-time.Hour))
	g, o := &guard{}, &observer{f: f}
	moved := f.serve(t, "pat", http.MethodPost, "/api/v0/nodes/"+child.String()+"/move", `{"parent_id":null,"after_id":null}`,
		[]page.WriteGuard{g}, nil, []page.PageObserver{o})
	if moved.Code != http.StatusOK || !strings.Contains(moved.Body.String(), `"parent_id":null`) {
		t.Fatalf("POST move = %d %s, want 200 at the root", moved.Code, moved.Body)
	}
	if n := f.count(t, `SELECT count(*) FROM changeset_items i JOIN changesets s ON s.id = i.changeset_id
		WHERE i.node_id = $1 AND i.before_parent_id = $2 AND i.before_sort_order = 0 AND i.after_parent_id IS NULL
		AND i.after_sort_order = -1 AND s.client = 'api' AND s.created_at = $3`, child, f.notes, testNow()); n != 1 {
		t.Error("the move's item is not Child's places in an api changeset of the unit's time")
	}
	deleted := f.serve(t, "session", http.MethodDelete, f.renamePath(), "", []page.WriteGuard{g}, nil, []page.PageObserver{o})
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("DELETE = %d %s, want 204", deleted.Code, deleted.Body)
	}
	if n := f.count(t, `SELECT count(*) FROM changeset_items i JOIN changesets s ON s.id = i.changeset_id JOIN nodes n ON n.id = i.node_id
		WHERE i.node_id = $1 AND i.after_name IS NULL AND i.deleted_at = $2 AND n.deleted_at = $2 AND s.client = 'web'`, f.notes, testNow()); n != 1 {
		t.Error("the deletion's item is not in the trash with Notes, in a web changeset")
	}
	if n := f.count(t, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL", child); n != 1 {
		t.Error("Child, moved out of Notes, is deleted with it")
	}
	if len(g.steps) != 2 || g.steps[0].Operation != domain.OpMove || g.steps[1].Operation != domain.OpDelete || !g.inTx ||
		len(o.events) != 2 {
		t.Errorf("the guard saw %+v in a transaction %v, the observers %d events; want the move, then the deletion", g.steps, g.inTx, len(o.events))
	}
}

// pageMark is a Markdown extension's test double: it takes the content's
// size, fetches the page it renders for, and writes both in a mark before
// the body.
func pageMark() markdown.Extension {
	return markdown.Extension{
		Name:    "page-mark",
		Extract: func(_ gast.Node, content []byte) any { return len(content) },
		Fetch: func(_ context.Context, p markdown.Page, extracted any) (any, error) {
			return fmt.Sprintf("%s/%s:%d", p.NotebookID, p.PageID, extracted), nil
		},
		Renderer: func(data any) []util.PrioritizedValue {
			return []util.PrioritizedValue{util.Prioritized(markRenderer(data.(string)), 50)}
		},
		Markup: markdown.Markup{Elements: map[string][]string{"mark": {"data-page"}}},
	}
}

type markRenderer string

func (r markRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(gast.KindDocument, func(w util.BufWriter, _ []byte, _ gast.Node, entering bool) (gast.WalkStatus, error) {
		if entering {
			_, _ = w.WriteString(`<mark data-page="` + string(r) + `"></mark>`)
		}
		return gast.WalkContinue, nil
	})
}

// A registered extension reaches the reading view through page.New: what
// it fetched for this page, from what it took from the content, is in the
// HTML, with the content rendered at its revision.
func TestAnExtensionReachesTheReadingView(t *testing.T) {
	f := newFixture(t)
	md, err := markdown.New([]markdown.Extension{pageMark()})
	if err != nil {
		t.Fatal(err)
	}
	f.md = md
	f.exec(t, "UPDATE page_contents SET content = $2, byte_size = octet_length($2), content_hash = sha256(convert_to($2, 'UTF8')),"+
		" revision = 2 WHERE node_id = $1", f.notes, "# Hello")
	rec := f.serve(t, "session", http.MethodGet, "/api/v0/pages/"+f.notes.String()+"/view", "", nil, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d %s, want 200", rec.Code, rec.Body)
	}
	var view struct {
		HTML     string `json:"html"`
		Revision int    `json:"revision"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	want := `<mark data-page="` + f.eng.String() + "/" + f.notes.String() + `:7"></mark><h1 id="nw-hello">Hello</h1>` + "\n"
	if view.HTML != want || view.Revision != 2 {
		t.Errorf("view = %+v, want %q at revision 2", view, want)
	}
}

// A content's write and a page created with a content reach the unit
// through page.New: the write is the page's next revision, its version on
// its base in a changeset of the credentials' client; the guard and the
// observers get its parse by the composition's Markdown, which the
// registered extension took from the content.
func TestTheContentsWritesReachTheUnit(t *testing.T) {
	f := newFixture(t)
	md, err := markdown.New([]markdown.Extension{pageMark()})
	if err != nil {
		t.Fatal(err)
	}
	f.md = md
	g, o := &guard{}, &observer{f: f}
	marked := func(c domain.Change) any {
		d, _ := c.Parsed.(*markdown.Document)
		if d == nil {
			return nil
		}
		return d.Extracted("page-mark")
	}
	written := f.serve(t, "pat", http.MethodPut, "/api/v0/pages/"+f.notes.String()+"/content", `{"content":"# Hello","base_revision":1}`,
		[]page.WriteGuard{g}, nil, []page.PageObserver{o})
	if written.Code != http.StatusOK || !strings.Contains(written.Body.String(), `"revision":2`) {
		t.Fatalf("PUT = %d %s, want 200 at revision 2", written.Code, written.Body)
	}
	if n := f.count(t, `SELECT count(*) FROM page_contents c JOIN page_revisions r ON r.node_id = c.node_id JOIN changesets s ON s.id = r.changeset_id
		WHERE c.node_id = $1 AND c.content = '# Hello' AND c.revision = 2 AND c.byte_size = 7 AND c.content_hash = sha256('# Hello')
		AND c.updated_at = $2 AND r.base_revision = 1 AND r.revision = 2 AND s.client = 'api' AND s.created_at = $2`, f.notes, testNow()); n != 1 {
		t.Error("the content is not Notes' revision 2, with its version on 1 in an api changeset of the unit's time")
	}
	if len(g.steps) != 1 || g.steps[0].Operation != domain.OpContent || !g.inTx || marked(g.steps[0].Changes[0]) != 7 ||
		len(o.events) != 1 || marked(o.events[0].Changes[0]) != 7 {
		t.Errorf("the guard saw %+v in a transaction %v, the observers %+v; want the write with its parse", g.steps, g.inTx, o.events)
	}
	created := f.serve(t, "session", http.MethodPost, f.createPath(), `{"parent_id":null,"title":"New","content":"abc"}`,
		nil, nil, []page.PageObserver{o})
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"byte_size":3`) {
		t.Fatalf("POST = %d %s, want 201 with its 3 bytes", created.Code, created.Body)
	}
	if len(o.events) != 2 || marked(o.events[1].Changes[0]) != 3 {
		t.Errorf("the observers followed %+v, want the creation with its parse", o.events)
	}
}

// The module's larger bodies are its routes' (M4/P4 design 3.8): a key
// that names no route it registers would relax nothing.
func TestTheBodyLimitsAreTheModulesRoutes(t *testing.T) {
	md, err := markdown.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	m := page.New(page.Deps{Clock: fixedClock{}, Logger: slog.New(slog.DiscardHandler), Markdown: md, EditSessionCleanupInterval: time.Hour,
		ParseBudgetBytes: 8 << 20, ParseMaxWait: time.Second})
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	m.Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: tokenAuth{}, BodyLimits: m.BodyLimits()}))
	if len(m.BodyLimits()) == 0 {
		t.Fatal("no route's body limit")
	}
	for route := range m.BodyLimits() {
		if !slices.Contains(router.Patterns(), route) {
			t.Errorf("the body limit of %q, which is no route of the module's %q", route, router.Patterns())
		}
	}
}
