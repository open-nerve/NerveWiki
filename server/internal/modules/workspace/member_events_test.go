package workspace_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The addition and the role change on a real database (M3/P2 design 3.6):
// the module wired as bootstrap wires it, each subscriber reading the
// membership as its transaction has it.

// memberWatcher fails when fail is set, and records what it was told and
// the role of the membership it was told about, read in the transaction:
// "" when there is no active one.
type memberWatcher struct {
	f       fixture
	fail    bool
	added   []workspace.MembershipAddition
	changed []workspace.MemberRoleChange
	roles   []string
}

func (w *memberWatcher) MembershipAdded(ctx context.Context, a workspace.MembershipAddition) error {
	w.added = append(w.added, a)
	return w.read(ctx, a.WorkspaceID, a.UserID)
}

func (w *memberWatcher) MemberRoleChanged(ctx context.Context, c workspace.MemberRoleChange) error {
	w.changed = append(w.changed, c)
	return w.read(ctx, c.WorkspaceID, c.UserID)
}

func (w *memberWatcher) read(ctx context.Context, workspaceID, userID uuid.UUID) error {
	var role string
	if err := postgres.DB(ctx, w.f.pool).QueryRow(ctx, "SELECT coalesce(max(role), '') FROM workspace_members "+
		"WHERE workspace_id = $1 AND user_id = $2 AND ended_at IS NULL AND deleted_at IS NULL", workspaceID, userID).Scan(&role); err != nil {
		return err
	}
	w.roles = append(w.roles, role)
	if w.fail {
		return errors.New("the subscriber failed")
	}
	return nil
}

// roleOf is the role of the account's active membership of acme, "" for
// none, as committed.
func (f fixture) roleOf(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	var role string
	if err := f.pool.QueryRow(context.Background(), "SELECT coalesce(max(role), '') FROM workspace_members "+
		"WHERE workspace_id = $1 AND user_id = $2 AND ended_at IS NULL AND deleted_at IS NULL", f.acme, userID).Scan(&role); err != nil {
		t.Fatal(err)
	}
	return role
}

// An accepted invitation adds a new membership, and its subscriber reads it
// in the acceptance's transaction; its failure rolls the whole acceptance
// back.
func TestAnAdditionPassesTheSubscriber(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "the subscriber follows", true: "the subscriber fails"}[fail], func(t *testing.T) {
			f := newFixture(t)
			dana := uuid.NewV7()
			f.exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) "+
				"VALUES ($1, 'dana@corp.com', 'x', 'Dana', $2, $2)", dana, testNow())
			inv := f.invite(t, f.acme, "dana@corp.com", "member")
			w := &memberWatcher{f: f, fail: fail}

			status, code := accept(t, f.serve(t, func(d *workspace.Deps) {
				d.MembershipAdditionSubscribers = []workspace.MembershipAdditionSubscriber{w}
			}), dana, inv)

			want := []workspace.MembershipAddition{{WorkspaceID: f.acme, UserID: dana, Role: "member", By: dana, At: testNow()}}
			if !slices.Equal(w.added, want) || !slices.Equal(w.roles, []string{"member"}) {
				t.Errorf("the subscriber saw %+v, the membership as %q; want %+v, a member", w.added, w.roles, want)
			}
			role, pending := f.roleOf(t, dana), f.invitationDeletedAt(t, inv) == nil
			switch {
			case !fail && (status != http.StatusOK || role != "member" || pending):
				t.Errorf("accept = %d %s, role %q, pending %v; want 200, a member, used up", status, code, role, pending)
			case fail && (status != http.StatusInternalServerError || role != "" || !pending):
				t.Errorf("accept = %d %s, role %q, pending %v; want 500, no membership, still pending", status, code, role, pending)
			}
		})
	}
}

// patchRole sends the admin by's change of the membership id to role, and
// returns the status and the problem code, if any.
func patchRole(t *testing.T, h http.Handler, by, id uuid.UUID, role string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/v0/workspace-members/"+id.String(), strings.NewReader(`{"role":"`+role+`"}`))
	req.Header.Set("Authorization", "Bearer "+by.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	return rec.Code, p.Code
}

// A role changed by an admin passes its subscriber, which reads the new
// role in the change's transaction; its failure rolls the change back. The
// same role again tells no one.
func TestARoleChangePassesTheSubscriber(t *testing.T) {
	for _, tt := range []struct {
		name, role string
		fail       bool
		status     int
		told       bool
		after      string
	}{
		{"the subscriber follows", "guest", false, http.StatusOK, true, "guest"},
		{"the subscriber fails", "guest", true, http.StatusInternalServerError, true, "member"},
		{"the same role", "member", false, http.StatusOK, false, "member"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			w := &memberWatcher{f: f, fail: tt.fail}

			status, code := patchRole(t, f.serve(t, func(d *workspace.Deps) {
				d.MemberRoleChangeSubscribers = []workspace.MemberRoleChangeSubscriber{w}
			}), f.alice, f.bobMembership, tt.role)

			var want []workspace.MemberRoleChange
			if tt.told {
				want = []workspace.MemberRoleChange{{WorkspaceID: f.acme, UserID: f.bob, From: "member", To: "guest", By: f.alice, At: testNow()}}
			}
			if !slices.Equal(w.changed, want) || (tt.told && !slices.Equal(w.roles, []string{"guest"})) {
				t.Errorf("the subscriber saw %+v, the membership as %q; want %+v", w.changed, w.roles, want)
			}
			if after := f.roleOf(t, f.bob); status != tt.status || after != tt.after {
				t.Errorf("PATCH = %d %s, bob's role %q; want %d, %q", status, code, after, tt.status, tt.after)
			}
		})
	}
}
