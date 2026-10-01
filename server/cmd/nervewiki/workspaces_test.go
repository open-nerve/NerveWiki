package main

import (
	"context"
	"strings"
	"testing"
)

// nervewiki workspaces through its flags (M2/P4 design 3.3): each prints
// its line, or one line saying why it failed, with exit code 1.
func TestWorkspacesCommands(t *testing.T) {
	environ, _ := usersDatabase(t)
	if code, _, stderr := executeWithInput(context.Background(), environ, "Tr0ub4dor&3\n", "users", "create", "--email", "ada@corp.com"); code != 0 {
		t.Fatalf("users create = %d: %s", code, stderr)
	}
	closed := append(environ, "NWIKI_WORKSPACE__CREATION_ENABLED=false")

	code, stdout, stderr := execute(context.Background(), closed, "workspaces", "create", "--slug", "acme", "--name", "Acme", "--admin", "ada@corp.com")
	if code != 0 || !strings.HasPrefix(stdout, "created workspace acme (") || !strings.HasSuffix(stdout, ") with admin ada@corp.com\n") {
		t.Errorf("workspaces create = %d, %q, %s; want the workspace created while creation is off", code, stdout, stderr)
	}
	code, stdout, stderr = execute(context.Background(), closed, "workspaces", "reactivate-member", "--workspace", "acme", "--email", "ada@corp.com")
	if code != 0 || stdout != "ada@corp.com is already a member of acme\n" {
		t.Errorf("workspaces reactivate-member = %d, %q, %s; want ada already a member", code, stdout, stderr)
	}

	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"no admin", []string{"workspaces", "create", "--slug", "beta", "--name", "Beta"}, "nervewiki: required flag(s) \"admin\" not set\n"},
		{"no workspace", []string{"workspaces", "reactivate-member", "--email", "ada@corp.com"}, "nervewiki: required flag(s) \"workspace\" not set\n"},
		{"invalid values", []string{"workspaces", "create", "--slug", "Beta!", "--name", "Beta", "--admin", "ada@corp.com"}, "nervewiki: --slug "},
		{"an unknown account", []string{"workspaces", "create", "--slug", "beta", "--name", "Beta", "--admin", "may@corp.com"},
			"nervewiki: The account does not exist.\n"},
		{"an unknown command", []string{"workspaces", "delete"}, "nervewiki: unknown command \"delete\" for \"nervewiki workspaces\"\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := execute(context.Background(), closed, tt.args...)
			if code != 1 || stdout != "" || !strings.Contains(stderr, tt.want) || strings.Count(stderr, "nervewiki: ") != 1 {
				t.Errorf("nervewiki %s = %d, stdout %q, stderr %q; want 1 and %q", strings.Join(tt.args, " "), code, stdout, stderr, tt.want)
			}
		})
	}
}

// The logs, at every level, name the accounts and workspaces by id, never
// by address: it goes to the administrator's own output only.
func TestWorkspacesLogNoAddress(t *testing.T) {
	environ, _ := usersDatabase(t)
	environ = append(environ, "NWIKI_LOG__LEVEL=debug")
	if code, _, stderr := executeWithInput(context.Background(), environ, "Tr0ub4dor&3\n", "users", "create", "--email", "ada@corp.com"); code != 0 {
		t.Fatalf("users create = %d: %s", code, stderr)
	}
	var logs strings.Builder
	for _, step := range []struct {
		args   []string
		logged string
	}{
		{[]string{"workspaces", "create", "--slug", "acme", "--name", "Acme", "--admin", "ada@corp.com"}, `msg="workspace created"`},
		{[]string{"users", "deactivate", "--email", "ada@corp.com"}, `msg="account deactivated"`},
		{[]string{"users", "activate", "--email", "ada@corp.com"}, `msg="account activated"`},
		{[]string{"workspaces", "reactivate-member", "--workspace", "acme", "--email", "ada@corp.com"}, `msg="workspace membership reactivated"`},
	} {
		code, _, stderr := execute(context.Background(), environ, step.args...)
		if code != 0 || !strings.Contains(stderr, step.logged) {
			t.Fatalf("nervewiki %s = %d (stderr %q), want 0 and the log %s", strings.Join(step.args, " "), code, stderr, step.logged)
		}
		logs.WriteString(stderr)
	}
	if !strings.Contains(logs.String(), "workspace_id=") || strings.Contains(logs.String(), "@corp.com") {
		t.Errorf("the logs name an address or no workspace:\n%s", logs.String())
	}
}

func TestBareWorkspacesPrintsHelp(t *testing.T) {
	code, stdout, stderr := execute(context.Background(), nil, "workspaces")

	if code != 0 || !strings.Contains(stdout, "\n  create ") || !strings.Contains(stdout, "\n  reactivate-member ") || stderr != "" {
		t.Errorf("nervewiki workspaces = %d, stdout %q, stderr %q; want 0 and the help, with both commands", code, stdout, stderr)
	}
}
