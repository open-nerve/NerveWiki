package domain

import (
	"errors"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestDecideAtTheWorkspaceLevel(t *testing.T) {
	adminsOnly := Rule{Level: LevelWorkspace, Workspace: []shared.WorkspaceRole{shared.WorkspaceAdmin}}
	everyone := Rule{Level: LevelWorkspace, Workspace: shared.WorkspaceRoles()}
	active := func(r shared.WorkspaceRole) Facts { return Facts{Workspace: Membership{Active: true, Role: r}} }
	tests := []struct {
		name  string
		rule  Rule
		facts Facts
		want  error // nil: granted with the facts' role
	}{
		{"admin, admins only", adminsOnly, active(shared.WorkspaceAdmin), nil},
		{"member, admins only", adminsOnly, active(shared.WorkspaceMember), shared.Forbidden()},
		{"guest, admins only", adminsOnly, active(shared.WorkspaceGuest), shared.Forbidden()},
		{"admin, everyone", everyone, active(shared.WorkspaceAdmin), nil},
		{"member, everyone", everyone, active(shared.WorkspaceMember), nil},
		{"guest, everyone", everyone, active(shared.WorkspaceGuest), nil},
		{"no active membership", everyone, Facts{}, shared.ErrNotVisible},
		// An ended membership keeps its role: the role does not make it seen.
		{"ended admin", everyone, Facts{Workspace: Membership{Role: shared.WorkspaceAdmin}}, shared.ErrNotVisible},
		{"a role outside the three", everyone, active("owner"), shared.Forbidden()},
		{"the empty role", everyone, active(""), shared.Forbidden()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grant, err := Decide(tt.rule, tt.facts)
			switch {
			case tt.want == nil && (err != nil || grant != shared.Grant{WorkspaceRole: tt.facts.Workspace.Role}):
				t.Errorf("Decide() = %+v, %v; want the grant of %q", grant, err, tt.facts.Workspace.Role)
			case tt.want != nil && (!errors.Is(err, tt.want) || grant != shared.Grant{}):
				t.Errorf("Decide() = %+v, %v; want %v", grant, err, tt.want)
			}
		})
	}
}

func TestDecideRefusesARuleOfUnknownLevel(t *testing.T) {
	_, err := Decide(Rule{Workspace: shared.WorkspaceRoles()}, Facts{Workspace: Membership{Active: true, Role: shared.WorkspaceAdmin}})
	var se *shared.Error
	if err == nil || errors.As(err, &se) {
		t.Errorf("Decide() = %v, want an internal error", err)
	}
}
