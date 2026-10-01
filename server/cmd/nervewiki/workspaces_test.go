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
