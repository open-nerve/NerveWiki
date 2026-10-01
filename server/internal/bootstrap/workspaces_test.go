package bootstrap

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// runWorkspaces runs cmd as nervewiki workspaces does, on the database of
// url, with workspace.creation_enabled off: the commands do not ask it.
func runWorkspaces(t *testing.T, url string, cmd WorkspaceCommand) (out, logs string, err error) {
	t.Helper()
	cfg := testConfig(t, url, false)
	cfg.Log.Level = "info"
	cfg.Workspace.CreationEnabled = false
	var stdout, stderr bytes.Buffer
	err = Workspaces(context.Background(), cfg, &stderr, &stdout, cmd)
	return stdout.String(), stderr.String(), err
}

// workspaces creates a workspace with an account named by its address as
// its admin, while creation is off, and prints it (M2/P4 design 3.3). What
// it refuses leaves the database as it was, with one line saying why.
func TestWorkspacesCreate(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	alice := createdAccount(t, url, pool, "alice@example.com")
	createdAccount(t, url, pool, "bob@example.com")
	if _, _, err := runUsers(t, url, DeactivateUser("bob@example.com")); err != nil {
		t.Fatal(err)
	}

	out, logs, err := runWorkspaces(t, url, CreateWorkspace("acme", "Acme", " Alice@Example.COM "))

	var id string
	if err := pool.QueryRow(context.Background(), "SELECT id FROM workspaces WHERE slug = 'acme' AND created_by_id = $1", alice).Scan(&id); err != nil {
		t.Fatalf("create = %q, %v: no acme created by alice: %v", out, err, err)
	}
	if want := "created workspace acme (" + id + ") with admin alice@example.com\n"; err != nil || out != want {
		t.Errorf("create = %q, %v; want %q", out, err, want)
	}
	if n := count(t, pool, "SELECT count(*) FROM workspace_members WHERE workspace_id = $1 AND user_id = $2 AND role = 'admin'", id, alice); n != 1 {
		t.Errorf("alice is the admin of acme %d times, want once", n)
	}
	if !strings.Contains(logs, `msg="workspace created"`) || !strings.Contains(logs, "by=cli") {
		t.Errorf("logs = %s, want the creation by=cli", logs)
	}

	for _, tt := range []struct {
		name string
		cmd  WorkspaceCommand
		want string
	}{
		{"a slug taken", CreateWorkspace("acme", "Another", "alice@example.com"), "The slug is taken."},
		{"no such account", CreateWorkspace("beta", "Beta", "carol@example.com"), "The account does not exist."},
		{"a deactivated account", CreateWorkspace("beta", "Beta", "bob@example.com"), "This account is deactivated."},
		{"invalid values", CreateWorkspace("Not A Slug", " ", "alice@example.com"), "--name "},
	} {
		out, _, err := runWorkspaces(t, url, tt.cmd)
		if err == nil || !strings.Contains(err.Error(), tt.want) || out != "" {
			t.Errorf("%s = %q, %v; want the refusal %q, no output", tt.name, out, err, tt.want)
		}
	}
	if n := count(t, pool, "SELECT count(*) FROM workspaces"); n != 1 {
		t.Errorf("%d workspaces, want acme alone: the refusals wrote nothing", n)
	}
}

// reactivate-member brings an ended membership back with its role, once
// the account is active again (M2/P4 design 3.3); it says so when the
// membership is active, and refuses what it cannot find.
func TestWorkspacesReactivateMember(t *testing.T) {
	tm := newAcmeTeam(t, "admin", "")
	if _, _, err := runUsers(t, tm.url, DeactivateUser("bob@example.com")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runWorkspaces(t, tm.url, ReactivateMember("acme", "bob@example.com")); err == nil ||
		!strings.Contains(err.Error(), "This account is deactivated.") {
		t.Errorf("reactivate-member of a deactivated account = %v, want the refusal", err)
	}
	if _, _, err := runUsers(t, tm.url, ActivateUser("bob@example.com")); err != nil {
		t.Fatal(err)
	}
	var ended time.Time
	if err := tm.pool.QueryRow(context.Background(), "SELECT ended_at FROM workspace_members WHERE id = $1", tm.members["bob"]).Scan(&ended); err != nil {
		t.Fatal(err)
	}

	out, logs, err := runWorkspaces(t, tm.url, ReactivateMember("acme", "Bob@Example.com"))

	want := "reactivated bob@example.com in acme as admin; the membership had ended at " + ended.UTC().Format(time.RFC3339) +
		"; ownerless notebooks returned: 0\n"
	if err != nil || out != want || !strings.Contains(logs, `msg="workspace membership reactivated"`) || !strings.Contains(logs, "by=cli") {
		t.Errorf("reactivate-member = %q, %v, logs %s; want %q, logged by=cli", out, err, logs, want)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM workspace_members WHERE id = $1 AND ended_at IS NULL AND role = 'admin'", tm.members["bob"]); n != 1 {
		t.Errorf("bob's membership is not active as an admin again")
	}
	if out, _, err := runWorkspaces(t, tm.url, ReactivateMember("acme", "bob@example.com")); err != nil || out != "bob@example.com is already a member of acme\n" {
		t.Errorf("reactivate-member again = %q, %v; want it said already", out, err)
	}
	for _, tt := range []struct {
		name string
		cmd  WorkspaceCommand
		want string
	}{
		{"no such workspace", ReactivateMember("beta", "bob@example.com"), "No such workspace."},
		{"never a member", ReactivateMember("acme", "dana@example.com"), "No such member."},
		{"no such account", ReactivateMember("acme", "erin@example.com"), "The account does not exist."},
	} {
		if out, _, err := runWorkspaces(t, tm.url, tt.cmd); err == nil || !strings.Contains(err.Error(), tt.want) || out != "" {
			t.Errorf("%s = %q, %v; want the refusal %q", tt.name, out, err, tt.want)
		}
	}
}
