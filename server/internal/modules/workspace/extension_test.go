package workspace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The extension points on a real database (M2/P2 design 3.4, 3.5), the
// module wired as bootstrap wires it and called over HTTP. The registrants
// here stand in for M3's: built from the pool alone, their statements reach
// the transaction through the context. The decision stands in for the
// access module's, which the workspace module does not import: it reads the
// same fact through NewMemberships, in the transaction.

// testNow is the clock's instant, in whole microseconds.
func testNow() time.Time { return time.Date(2026, 10, 1, 10, 0, 0, 123456000, time.UTC) }

// tickingClock reads testNow first, then a microsecond later each time: a
// use case that read it twice for what must be one time would write two.
type tickingClock struct{ reads int }

func (c *tickingClock) Now() time.Time {
	c.reads++
	return testNow().Add(time.Duration(c.reads-1) * time.Microsecond)
}

// errVetoed is the code the test's vetoer refuses with.
func errVetoed() *shared.Error {
	return shared.NewError(shared.KindConflict, "test.vetoed", "The membership cannot end.")
}

// rolesAuthorizer decides as the access module's rule table does for the
// workspace module's actions: admins only for the changes, every role for
// the rest; no active membership is not visible.
type rolesAuthorizer struct{ facts workspace.Memberships }

func (a rolesAuthorizer) Authorize(ctx context.Context, actor shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	role, ok, err := a.facts.RoleOf(ctx, t.WorkspaceID, actor.UserID)
	switch {
	case err != nil:
		return shared.Grant{}, err
	case !ok:
		return shared.Grant{}, shared.ErrNotVisible
	}
	adminsOnly := map[shared.Action]bool{"workspace.update": true, "workspace.delete": true,
		"workspace_member.update": true, "workspace_member.remove": true,
		"workspace_invitation.list": true, "workspace_invitation.create": true, "workspace_invitation.delete": true}
	if adminsOnly[action] && role != shared.WorkspaceAdmin {
		return shared.Grant{}, shared.Forbidden()
	}
	return shared.Grant{WorkspaceRole: role}, nil
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

// sqlAccounts stands in for identity's Accounts: it shares the account's
// row and reads its address, in the transaction.
type sqlAccounts struct{ pool *pgxpool.Pool }

func (a sqlAccounts) ShareActiveAccount(ctx context.Context, id uuid.UUID) (string, error) {
	var email string
	err := postgres.DB(ctx, a.pool).QueryRow(ctx, "SELECT email FROM users WHERE id = $1 FOR SHARE", id).Scan(&email)
	return email, err
}

// sqlDirectory stands in for identity's directory: unlocked reads of the
// accounts' rows.
type sqlDirectory struct{ pool *pgxpool.Pool }

func (d sqlDirectory) MemberProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]workspace.Profile, error) {
	rows, err := postgres.DB(ctx, d.pool).Query(ctx, "SELECT id, display_name, email FROM users WHERE id = ANY($1)", ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := map[uuid.UUID]workspace.Profile{}
	for rows.Next() {
		var id uuid.UUID
		var p workspace.Profile
		if err := rows.Scan(&id, &p.DisplayName, &p.Email); err != nil {
			return nil, err
		}
		profiles[id] = p
	}
	return profiles, rows.Err()
}

func (d sqlDirectory) AccountIDByEmail(ctx context.Context, email string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := postgres.DB(ctx, d.pool).QueryRow(ctx, "SELECT id FROM users WHERE email = $1", email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, false, nil
	}
	return id, err == nil, err
}

// invitationKey is the MAC key the tests' module signs its invitations with.
func invitationKey() []byte { return bytes.Repeat([]byte{7}, 32) }

// fixture is acme on a database of its own: alice its admin, bob a member.
type fixture struct {
	pool          *pgxpool.Pool
	acme          uuid.UUID
	alice, bob    uuid.UUID
	bobMembership uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 8})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f := fixture{pool: pool, acme: uuid.NewV7(), alice: uuid.NewV7(), bob: uuid.NewV7(), bobMembership: uuid.NewV7()}
	f.exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES "+
		"($1, 'alice@corp.com', 'x', 'Alice', $3, $3), ($2, 'bob@corp.com', 'x', 'Bob', $3, $3)", f.alice, f.bob, testNow())
	f.exec(t, "INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) "+
		"VALUES ($1, 'acme', 'Acme', $2, $2, $3, $3)", f.acme, f.alice, testNow())
	f.exec(t, "INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at) "+
		"VALUES ($1, $2, $3, 'admin', $3, $3, $5, $5), ($4, $2, $6, 'member', $3, $3, $5, $5)",
		uuid.NewV7(), f.acme, f.alice, f.bobMembership, testNow(), f.bob)
	return f
}

func (f fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// serve wires the module on the fixture's pool with the registrants.
func (f fixture) serve(t *testing.T, change func(*workspace.Deps)) http.Handler {
	t.Helper()
	d := workspace.Deps{
		Pool: f.pool, Tx: postgres.NewTxManager(f.pool, 5*time.Second), Clock: &tickingClock{},
		Logger: slog.New(slog.DiscardHandler), Authorizer: rolesAuthorizer{facts: workspace.NewMemberships(f.pool)},
		Accounts: sqlAccounts{f.pool}, Directory: sqlDirectory{f.pool}, InvitationKey: invitationKey(), CreationEnabled: true,
	}
	change(&d)
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	workspace.New(d).Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: tokenAuth{}}))
	return router
}

// call sends method path as the account by, and returns the status and the
// problem code, if any.
func call(t *testing.T, h http.Handler, by uuid.UUID, method, path string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+by.String())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	return rec.Code, p.Code
}

// endedAt is when the membership id ended, nil while it is active.
func (f fixture) endedAt(t *testing.T, ctx context.Context, id uuid.UUID) *time.Time {
	t.Helper()
	var at *time.Time
	if err := postgres.DB(ctx, f.pool).QueryRow(ctx, "SELECT ended_at FROM workspace_members WHERE id = $1", id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

// workspaceLocked reports whether another transaction holds acme's row:
// NOWAIT fails at once rather than waiting for it.
func (f fixture) workspaceLocked(ctx context.Context) (bool, error) {
	_, err := f.pool.Exec(ctx, "SELECT 1 FROM workspaces WHERE id = $1 FOR NO KEY UPDATE NOWAIT", f.acme)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
		return true, nil
	}
	return false, err
}

// vetoer refuses when refuse is set, and records what it saw under the
// lock: whether the workspace's row was held, and the membership's end.
type vetoer struct {
	t       *testing.T
	f       fixture
	refuse  bool
	called  int
	locked  bool
	endedAt *time.Time
	inTx    bool
}

func (v *vetoer) VetoMembershipEnd(ctx context.Context, e workspace.MembershipEnd) error {
	v.called++
	v.inTx = postgres.InTx(ctx)
	locked, err := v.f.workspaceLocked(context.WithoutCancel(ctx))
	if err != nil {
		return err
	}
	v.locked = locked
	v.endedAt = v.f.endedAt(v.t, ctx, v.f.bobMembership)
	if v.refuse {
		return errVetoed()
	}
	return nil
}

// subscriber fails when fail is set, and records what it read in the
// transaction: the membership's end, and the workspace's deletion.
type subscriber struct {
	t         *testing.T
	f         fixture
	fail      bool
	ended     []workspace.MembershipEnd
	endedAt   *time.Time
	deletedAt *time.Time
}

func (s *subscriber) MembershipEnded(ctx context.Context, e workspace.MembershipEnd) error {
	s.ended = append(s.ended, e)
	s.endedAt = s.f.endedAt(s.t, ctx, s.f.bobMembership)
	if s.fail {
		return errors.New("the subscriber failed")
	}
	return nil
}

func (s *subscriber) WorkspaceDeleted(ctx context.Context, d workspace.WorkspaceDeletion) error {
	if err := postgres.DB(ctx, s.f.pool).QueryRow(ctx, "SELECT deleted_at FROM workspaces WHERE id = $1", d.WorkspaceID).
		Scan(&s.deletedAt); err != nil {
		return err
	}
	if s.fail {
		return errors.New("the subscriber failed")
	}
	return nil
}

func withEnd(v *vetoer, s *subscriber) func(*workspace.Deps) {
	return func(d *workspace.Deps) {
		d.MembershipEndVetoers = []workspace.MembershipEndVetoer{v}
		d.MembershipEndSubscribers = []workspace.MembershipEndSubscriber{s}
		d.DeletionSubscribers = []workspace.WorkspaceDeletionSubscriber{s}
	}
}

// The vetoer runs in the transaction, with the workspace's row held, before
// the membership is written; the subscriber after, in the same transaction.
func TestAMembershipEndPassesTheVetoerThenTheSubscriber(t *testing.T) {
	for _, path := range []struct {
		name         string
		by           func(f fixture) uuid.UUID
		method, path string
	}{
		{"removed", func(f fixture) uuid.UUID { return f.alice }, http.MethodDelete, "/api/v0/workspace-members/"},
		{"left", func(f fixture) uuid.UUID { return f.bob }, http.MethodPost, "/api/v0/workspaces/acme/leave"},
	} {
		t.Run(path.name, func(t *testing.T) {
			f := newFixture(t)
			v, s := &vetoer{t: t, f: f}, &subscriber{t: t, f: f}
			target := path.path
			if path.name == "removed" {
				target += f.bobMembership.String()
			}

			status, code := call(t, f.serve(t, withEnd(v, s)), path.by(f), path.method, target)

			if status != http.StatusNoContent {
				t.Fatalf("%s = %d %s, want 204", target, status, code)
			}
			if v.called != 1 || !v.inTx || !v.locked || v.endedAt != nil {
				t.Errorf("the vetoer: called %d, in tx %v, row held %v, saw the end %v; want once, in it, held, before the end",
					v.called, v.inTx, v.locked, v.endedAt)
			}
			if len(s.ended) != 1 || s.endedAt == nil || !s.endedAt.Equal(testNow()) {
				t.Fatalf("the subscriber: %d calls, saw the end %v; want one, at %v", len(s.ended), s.endedAt, testNow())
			}
			if e := s.ended[0]; string(e.Cause) != path.name || e.UserID != f.bob || e.By != path.by(f) || len(e.WorkspaceIDs) != 1 || e.WorkspaceIDs[0] != f.acme {
				t.Errorf("the end = %+v, want bob's of acme, %s, by %s", e, path.name, path.by(f))
			}
		})
	}
}

// A refusal answers its code and changes nothing; so does a subscriber's
// failure, after the write.
func TestAMembershipEndRollsBack(t *testing.T) {
	for _, tt := range []struct {
		name   string
		refuse bool
		fail   bool
		status int
		code   string
	}{
		{"vetoed", true, false, http.StatusConflict, "test.vetoed"},
		{"the subscriber fails", false, true, http.StatusInternalServerError, "internal_error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			v, s := &vetoer{t: t, f: f, refuse: tt.refuse}, &subscriber{t: t, f: f, fail: tt.fail}

			status, code := call(t, f.serve(t, withEnd(v, s)), f.alice, http.MethodDelete, "/api/v0/workspace-members/"+f.bobMembership.String())

			if status != tt.status || code != tt.code {
				t.Errorf("DELETE = %d %s, want %d %s", status, code, tt.status, tt.code)
			}
			if at := f.endedAt(t, context.Background(), f.bobMembership); at != nil {
				t.Errorf("bob's membership ended at %v, want it active", at)
			}
		})
	}
}

// The deletion's subscriber reads the workspace deleted, in the deletion's
// transaction, at the time of the members' deletion; its failure deletes
// nothing.
func TestAWorkspaceDeletionPassesTheSubscriber(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "the subscriber follows", true: "the subscriber fails"}[fail], func(t *testing.T) {
			f := newFixture(t)
			s := &subscriber{t: t, f: f, fail: fail}

			status, code := call(t, f.serve(t, withEnd(&vetoer{t: t, f: f}, s)), f.alice, http.MethodDelete, "/api/v0/workspaces/acme")

			// The rows deleted at the clock's first read: the members and
			// the workspace.
			var members, workspaces int
			if err := f.pool.QueryRow(context.Background(), `SELECT
				(SELECT count(*) FROM workspace_members WHERE workspace_id = $1 AND deleted_at = $2),
				(SELECT count(*) FROM workspaces WHERE id = $1 AND deleted_at = $2)`, f.acme, testNow()).Scan(&members, &workspaces); err != nil {
				t.Fatal(err)
			}
			switch {
			case !fail && (status != http.StatusNoContent || s.deletedAt == nil || !s.deletedAt.Equal(testNow()) || members != 2 || workspaces != 1):
				t.Errorf("DELETE = %d %s, the subscriber saw %v, %d members and %d workspaces deleted at %v; want 204, it, both, acme",
					status, code, s.deletedAt, members, workspaces, testNow())
			case fail && (status != http.StatusInternalServerError || members != 0 || workspaces != 0):
				t.Errorf("DELETE with a failing subscriber = %d %s, %d members and %d workspaces deleted; want 500, none", status, code, members, workspaces)
			}
		})
	}
}
