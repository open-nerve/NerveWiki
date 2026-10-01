package workspace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/mac"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The sign-up policy's check on a real database (M2/P3 design 3.6): only a
// pending invitation of a workspace not deleted, by its token, for the
// address it was sent to.
func TestInvitationCheckAdmits(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	alice := uuid.NewV7()
	exec(`INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', $2, $2)`, alice, testNow())
	invitation := func(slug, email string) uuid.UUID {
		t.Helper()
		ws, id := uuid.NewV7(), uuid.NewV7()
		exec(`INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, $2, $3, $3, $4, $4)`,
			ws, slug, alice, testNow())
		exec(`INSERT INTO workspace_invitations (id, workspace_id, email, role, created_by_id, updated_by_id, created_at, updated_at)
			VALUES ($1, $2, $3, 'member', $4, $4, $5, $5)`, id, ws, email, alice, testNow())
		return id
	}
	pending := invitation("acme", "dana@corp.com")
	deleted := invitation("beta", "dana@corp.com")
	exec(`UPDATE workspace_invitations SET deleted_at = $2 WHERE id = $1`, deleted, testNow())
	ofDeleted := invitation("gone", "dana@corp.com")
	exec(`UPDATE workspaces SET deleted_at = $2 WHERE id = (SELECT workspace_id FROM workspace_invitations WHERE id = $1)`, ofDeleted, testNow())
	key := bytes.Repeat([]byte{7}, 32)
	token := macadapter.New(key).Token
	check := workspace.NewInvitationCheck(pool, key)

	for _, tt := range []struct {
		name         string
		id           uuid.UUID
		token, email string
		want         bool
	}{
		{"its token, its address", pending, token(pending), "dana@corp.com", true},
		{"another invitation's token", pending, token(deleted), "dana@corp.com", false},
		{"another key's token", pending, macadapter.New(bytes.Repeat([]byte{8}, 32)).Token(pending), "dana@corp.com", false},
		{"another address", pending, token(pending), "erin@corp.com", false},
		{"deleted", deleted, token(deleted), "dana@corp.com", false},
		{"of a deleted workspace", ofDeleted, token(ofDeleted), "dana@corp.com", false},
		{"no such invitation", uuid.NewV7(), "", "dana@corp.com", false},
	} {
		if got, err := check.Admits(ctx, tt.id, tt.token, tt.email); err != nil || got != tt.want {
			t.Errorf("Admits(%s) = %v, %v; want %v", tt.name, got, err, tt.want)
		}
	}
	// A token checked first: a valid one reaches the database, whose
	// failure is the check's.
	pool.Close()
	if _, err := check.Admits(ctx, pending, token(pending), "dana@corp.com"); err == nil || errors.Is(err, context.Canceled) {
		t.Errorf("Admits() on a closed pool = %v, want its error", err)
	}
	if got, err := check.Admits(ctx, pending, token(deleted), "dana@corp.com"); err != nil || got {
		t.Errorf("Admits(a wrong token) on a closed pool = %v, %v; want false before any read", got, err)
	}
}

// invite inserts a pending invitation of workspaceID to email with role,
// and returns its id.
func (f fixture) invite(t *testing.T, workspaceID uuid.UUID, email, role string) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	f.exec(t, `INSERT INTO workspace_invitations (id, workspace_id, email, role, created_by_id, updated_by_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5, $6, $6)`, id, workspaceID, email, role, f.alice, testNow())
	return id
}

// invitationDeletedAt is when the invitation id stopped being pending, nil
// while it is.
func (f fixture) invitationDeletedAt(t *testing.T, id uuid.UUID) *time.Time {
	t.Helper()
	var at *time.Time
	if err := f.pool.QueryRow(context.Background(), "SELECT deleted_at FROM workspace_invitations WHERE id = $1", id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

// accept sends bob's acceptance of the invitation id with its token, and
// returns the status and the problem code, if any.
func accept(t *testing.T, h http.Handler, by, id uuid.UUID) (int, string) {
	t.Helper()
	body := `{"token":"` + macadapter.New(invitationKey()).Token(id) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v0/workspace-invitations/"+id.String()+"/accept", strings.NewReader(body))
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

// A membership's end deletes the pending invitations of the workspace to
// the account's address, at the end's time: none sent before brings it
// back (M2/P3 design 3.4).
func TestAMembershipEndDeletesTheInvitationsToTheAddress(t *testing.T) {
	f := newFixture(t)
	beta := uuid.NewV7()
	f.exec(t, "INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'beta', 'Beta', $2, $2, $3, $3)",
		beta, f.alice, testNow())
	toBob, toBobElsewhere, toErin := f.invite(t, f.acme, "bob@corp.com", "admin"), f.invite(t, beta, "bob@corp.com", "member"),
		f.invite(t, f.acme, "erin@corp.com", "member")

	status, code := call(t, f.serve(t, func(*workspace.Deps) {}), f.alice, http.MethodDelete, "/api/v0/workspace-members/"+f.bobMembership.String())

	if status != http.StatusNoContent {
		t.Fatalf("DELETE = %d %s, want 204", status, code)
	}
	if at := f.invitationDeletedAt(t, toBob); at == nil || !at.Equal(testNow()) {
		t.Errorf("bob's invitation to acme deleted at %v, want %v", at, testNow())
	}
	for name, id := range map[string]uuid.UUID{"bob's to beta": toBobElsewhere, "erin's to acme": toErin} {
		if at := f.invitationDeletedAt(t, id); at != nil {
			t.Errorf("%s deleted at %v, want it pending", name, at)
		}
	}
}

// A workspace's deletion deletes its pending invitations, at its time.
func TestAWorkspaceDeletionDeletesItsInvitations(t *testing.T) {
	f := newFixture(t)
	inv := f.invite(t, f.acme, "erin@corp.com", "member")

	status, code := call(t, f.serve(t, func(*workspace.Deps) {}), f.alice, http.MethodDelete, "/api/v0/workspaces/acme")

	if at := f.invitationDeletedAt(t, inv); status != http.StatusNoContent || at == nil || !at.Equal(testNow()) {
		t.Errorf("DELETE = %d %s, the invitation deleted at %v; want 204, at %v", status, code, at, testNow())
	}
}

// restorer fails when fail is set, and records what it saw in the
// transaction: the restore, and the membership's end and role then.
type restorer struct {
	f        fixture
	fail     bool
	restored []workspace.MembershipRestore
	endedAt  *time.Time
	role     string
}

func (r *restorer) MembershipRestored(ctx context.Context, e workspace.MembershipRestore) error {
	r.restored = append(r.restored, e)
	if err := postgres.DB(ctx, r.f.pool).QueryRow(ctx, "SELECT ended_at, role FROM workspace_members WHERE id = $1", r.f.bobMembership).
		Scan(&r.endedAt, &r.role); err != nil {
		return err
	}
	if r.fail {
		return errors.New("the subscriber failed")
	}
	return nil
}

// An accepted invitation restores an ended membership, and its subscriber
// reads it restored in the acceptance's transaction; its failure rolls the
// whole acceptance back.
func TestARestorePassesTheSubscriber(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "the subscriber follows", true: "the subscriber fails"}[fail], func(t *testing.T) {
			f := newFixture(t)
			f.exec(t, "UPDATE workspace_members SET ended_at = $2 WHERE id = $1", f.bobMembership, testNow())
			inv := f.invite(t, f.acme, "bob@corp.com", "guest")
			r := &restorer{f: f, fail: fail}

			status, code := accept(t, f.serve(t, func(d *workspace.Deps) {
				d.MembershipRestoreSubscribers = []workspace.MembershipRestoreSubscriber{r}
			}), f.bob, inv)

			want := []workspace.MembershipRestore{{WorkspaceID: f.acme, UserID: f.bob, Role: "guest", By: f.bob, At: testNow()}}
			if !slices.Equal(r.restored, want) || r.endedAt != nil || r.role != "guest" {
				t.Errorf("the subscriber saw %+v, the membership ended at %v as %s; want %+v, active, a guest", r.restored, r.endedAt, r.role, want)
			}
			ended, pending := f.endedAt(t, context.Background(), f.bobMembership), f.invitationDeletedAt(t, inv) == nil
			switch {
			case !fail && (status != http.StatusOK || ended != nil || pending):
				t.Errorf("accept = %d %s, ended %v, pending %v; want 200, restored, used up", status, code, ended, pending)
			case fail && (status != http.StatusInternalServerError || ended == nil || !pending):
				t.Errorf("accept = %d %s, ended %v, pending %v; want 500, still ended, still pending", status, code, ended, pending)
			}
		})
	}
}
