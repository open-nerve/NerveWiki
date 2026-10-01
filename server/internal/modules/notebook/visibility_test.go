package notebook_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The visibility event's triggers on a real database (M3/P2 design 3.6):
// the module wired as bootstrap wires it, each subscriber reading, in the
// write's transaction, what the write did.

// rolesAuthorizer lets anyone create a notebook, and lets a notebook's
// explicit members act on it with their role; the rest do not see it. The
// rule table's sets are the access module's to prove: here every role may
// do every action.
type rolesAuthorizer struct{ facts notebook.Facts }

func (a rolesAuthorizer) Authorize(ctx context.Context, actor shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	if action == "notebook.create" {
		return shared.Grant{WorkspaceRole: shared.WorkspaceMember}, nil
	}
	f, err := a.facts.NotebookFacts(ctx, t.NotebookID, actor.UserID)
	switch {
	case err != nil:
		return shared.Grant{}, err
	case f.Role == "" || f.WorkspaceID != t.WorkspaceID:
		return shared.Grant{}, shared.ErrNotVisible
	}
	return shared.Grant{WorkspaceRole: shared.WorkspaceMember, NotebookRole: f.Role}, nil
}

// sqlWorkspaceMembers stands in for the workspace module's memberships.
type sqlWorkspaceMembers struct{ pool *pgxpool.Pool }

func (m sqlWorkspaceMembers) RoleOf(ctx context.Context, workspaceID, userID uuid.UUID) (shared.WorkspaceRole, bool, error) {
	var role string
	err := postgres.DB(ctx, m.pool).QueryRow(ctx, "SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2 "+
		"AND ended_at IS NULL AND deleted_at IS NULL", workspaceID, userID).Scan(&role)
	return shared.WorkspaceRole(role), err == nil, nil
}

// sqlProfiles stands in for identity's directory.
type sqlProfiles struct{ pool *pgxpool.Pool }

func (p sqlProfiles) MemberProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]notebook.Profile, error) {
	rows, err := postgres.DB(ctx, p.pool).Query(ctx, "SELECT id, display_name, email FROM users WHERE id = ANY($1)", ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]notebook.Profile{}
	for rows.Next() {
		var id uuid.UUID
		var pr notebook.Profile
		if err := rows.Scan(&id, &pr.DisplayName, &pr.Email); err != nil {
			return nil, err
		}
		out[id] = pr
	}
	return out, rows.Err()
}

// watcher fails when fail is set, and records the changes it was told and
// what probe, a query of one text, read in their transaction.
type watcher struct {
	pool  *pgxpool.Pool
	probe string
	fail  bool
	got   []notebook.VisibilityChange
	seen  []string
}

func (w *watcher) VisibilityChanged(ctx context.Context, v notebook.VisibilityChange) error {
	w.got = append(w.got, v)
	var s string
	if err := postgres.DB(ctx, w.pool).QueryRow(ctx, w.probe).Scan(&s); err != nil {
		return err
	}
	w.seen = append(w.seen, s)
	if w.fail {
		return errors.New("the subscriber failed")
	}
	return nil
}

// serveWith wires the module on the fixture's pool, w its visibility's
// subscriber.
func (f fixture) serveWith(t *testing.T, w *watcher) http.Handler {
	t.Helper()
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	notebook.New(notebook.Deps{
		Pool: f.pool, Tx: postgres.NewTxManager(f.pool, 5*time.Second), Clock: &tickingClock{}, Logger: slog.New(slog.DiscardHandler),
		Authorizer: rolesAuthorizer{facts: notebook.NewFacts(f.pool)}, Workspaces: sqlWorkspaces{f.pool},
		WorkspaceMembers: sqlWorkspaceMembers{f.pool}, Profiles: sqlProfiles{f.pool},
		VisibilitySubscribers: []notebook.VisibilitySubscriber{w},
	}).Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: tokenAuth{}}))
	return router
}

// send sends method path with body as the account by, and returns the
// status.
func send(h http.Handler, by uuid.UUID, method, path, body string) int {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+by.String())
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

// withCarol makes carol, with bob, a member of acme, and carol an active
// reader of eng, whose membership's id it returns.
func (f fixture) withCarol(t *testing.T) (carol, membership uuid.UUID) {
	t.Helper()
	carol, membership = uuid.NewV7(), uuid.NewV7()
	f.exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'carol@corp.com', 'x', 'Carol', $2, $2)",
		carol, testNow())
	for _, user := range []uuid.UUID{f.alice, f.bob, carol} {
		f.exec(t, "INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at) "+
			"VALUES ($1, $2, $3, 'member', $3, $3, $4, $4)", uuid.NewV7(), f.acme, user, testNow())
	}
	f.exec(t, "INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at) "+
		"VALUES ($1, $2, $3, 'reader', $4, $4, $5, $5)", membership, f.eng, carol, f.alice, testNow().Add(-time.Hour))
	return carol, membership
}

// Each trigger tells its subscriber after its write, in its transaction,
// with its value; the subscriber's failure rolls the write back.
func TestTheVisibilityTriggers(t *testing.T) {
	for _, tt := range []struct {
		name string
		// request is the write, by an account of f, and the value it tells.
		request func(f fixture, carol, carolsMembership uuid.UUID) (by uuid.UUID, method, path, body string, want notebook.VisibilityChange)
		status  int
		// probe reads the write's effect: its value after the write, and
		// before it (when the write rolled back).
		probe        func(f fixture, carol uuid.UUID) string
		after, prior string
	}{
		{"a notebook created open", func(f fixture, _, _ uuid.UUID) (uuid.UUID, string, string, string, notebook.VisibilityChange) {
			return f.alice, http.MethodPost, "/api/v0/workspaces/acme/notebooks", `{"name":"Plans","workspace_access":"viewer"}`,
				notebook.VisibilityChange{WorkspaceID: f.acme, UserIDs: []uuid.UUID{f.alice}, Reached: true, At: testNow()}
		}, http.StatusCreated, func(fixture, uuid.UUID) string {
			return "SELECT count(*)::text FROM notebooks WHERE name = 'Plans'"
		}, "1", "0"},
		{"the access crossing none", func(f fixture, _, _ uuid.UUID) (uuid.UUID, string, string, string, notebook.VisibilityChange) {
			return f.alice, http.MethodPatch, "/api/v0/notebooks/" + f.eng.String(), `{"workspace_access":"editor"}`,
				notebook.VisibilityChange{WorkspaceID: f.acme, Reached: true, At: testNow()}
		}, http.StatusOK, func(f fixture, _ uuid.UUID) string {
			return "SELECT workspace_access FROM notebooks WHERE id = '" + f.eng.String() + "'"
		}, "editor", "none"},
		{"a member once added back", func(f fixture, _, _ uuid.UUID) (uuid.UUID, string, string, string, notebook.VisibilityChange) {
			return f.alice, http.MethodPost, "/api/v0/notebooks/" + f.eng.String() + "/members", `{"user_id":"` + f.bob.String() + `","role":"editor"}`,
				notebook.VisibilityChange{WorkspaceID: f.acme, UserIDs: []uuid.UUID{f.bob}, At: testNow()}
		}, http.StatusCreated, func(f fixture, _ uuid.UUID) string {
			return "SELECT role || ' ' || (ended_at IS NULL)::text FROM notebook_members WHERE notebook_id = '" + f.eng.String() +
				"' AND user_id = '" + f.bob.String() + "'"
		}, "editor true", "reader false"},
		{"a member removed", func(f fixture, carol, m uuid.UUID) (uuid.UUID, string, string, string, notebook.VisibilityChange) {
			return f.alice, http.MethodDelete, "/api/v0/notebook-members/" + m.String(), "",
				notebook.VisibilityChange{WorkspaceID: f.acme, UserIDs: []uuid.UUID{carol}, At: testNow()}
		}, http.StatusNoContent, endedProbe, "true", "false"},
		{"a member leaving", func(f fixture, carol, _ uuid.UUID) (uuid.UUID, string, string, string, notebook.VisibilityChange) {
			return carol, http.MethodPost, "/api/v0/notebooks/" + f.eng.String() + "/leave", "",
				notebook.VisibilityChange{WorkspaceID: f.acme, UserIDs: []uuid.UUID{carol}, At: testNow()}
		}, http.StatusNoContent, endedProbe, "true", "false"},
	} {
		for _, fail := range []bool{false, true} {
			t.Run(tt.name+map[bool]string{false: "", true: ", the subscriber fails"}[fail], func(t *testing.T) {
				f := newFixture(t)
				carol, membership := f.withCarol(t)
				by, method, path, body, want := tt.request(f, carol, membership)
				probe := tt.probe(f, carol)
				w := &watcher{pool: f.pool, probe: probe, fail: fail}

				status := send(f.serveWith(t, w), by, method, path, body)

				if len(w.got) != 1 || !sameChange(w.got[0], want) || len(w.seen) != 1 || w.seen[0] != tt.after {
					t.Errorf("told %+v, seeing %q; want %+v, seeing %q", w.got, w.seen, want, tt.after)
				}
				committed, wantStatus := tt.prior, http.StatusInternalServerError
				if !fail {
					committed, wantStatus = tt.after, tt.status
				}
				var got string
				if err := f.pool.QueryRow(context.Background(), probe).Scan(&got); err != nil {
					t.Fatal(err)
				}
				if status != wantStatus || got != committed {
					t.Errorf("%s %s = %d, then %q; want %d, %q", method, path, status, got, wantStatus, committed)
				}
			})
		}
	}
}

// endedProbe reads whether carol's membership of eng ended.
func endedProbe(f fixture, carol uuid.UUID) string {
	return "SELECT (ended_at IS NOT NULL)::text FROM notebook_members WHERE notebook_id = '" + f.eng.String() +
		"' AND user_id = '" + carol.String() + "'"
}

// sameChange compares two visibility changes, the time by the instant.
func sameChange(a, b notebook.VisibilityChange) bool {
	if a.WorkspaceID != b.WorkspaceID || a.Reached != b.Reached || !a.At.Equal(b.At) || len(a.UserIDs) != len(b.UserIDs) {
		return false
	}
	for i := range a.UserIDs {
		if a.UserIDs[i] != b.UserIDs[i] {
			return false
		}
	}
	return true
}
