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

func TestDecideAtTheNotebookLevel(t *testing.T) {
	admin, editor, reader := shared.NotebookAdmin, shared.NotebookEditor, shared.NotebookReader
	adminsOnly := Rule{Level: LevelNotebook, Notebook: []shared.NotebookRole{admin}}
	readers := Rule{Level: LevelNotebook, Notebook: shared.NotebookRoles()}
	facts := func(ws shared.WorkspaceRole, access shared.WorkspaceAccess, role shared.NotebookRole) Facts {
		return Facts{Workspace: Membership{Active: true, Role: ws}, Notebook: Notebook{Found: true, Access: access, Role: role}}
	}
	tests := []struct {
		name  string
		rule  Rule
		facts Facts
		want  shared.NotebookRole // "": the error
		err   error
	}{
		{"its admin", adminsOnly, facts(shared.WorkspaceMember, shared.AccessNone, admin), admin, nil},
		{"its editor, admins only", adminsOnly, facts(shared.WorkspaceMember, shared.AccessNone, editor), "", shared.Forbidden()},
		{"its reader reads", readers, facts(shared.WorkspaceMember, shared.AccessNone, reader), reader, nil},
		// The default role, and the higher of the two.
		{"a member by the access", readers, facts(shared.WorkspaceMember, shared.AccessViewer, ""), reader, nil},
		{"a reader raised by the access", readers, facts(shared.WorkspaceMember, shared.AccessEditor, reader), editor, nil},
		{"the workspace's admin by the access", adminsOnly, facts(shared.WorkspaceAdmin, shared.AccessEditor, ""), "", shared.Forbidden()},
		// Not visible: no role, to the workspace's admins too.
		{"a member, closed", readers, facts(shared.WorkspaceMember, shared.AccessNone, ""), "", shared.ErrNotVisible},
		{"the workspace's admin, closed", readers, facts(shared.WorkspaceAdmin, shared.AccessNone, ""), "", shared.ErrNotVisible},
		{"a guest, open", readers, facts(shared.WorkspaceGuest, shared.AccessEditor, ""), "", shared.ErrNotVisible},
		{"a guest, its reader", readers, facts(shared.WorkspaceGuest, shared.AccessEditor, reader), reader, nil},
		// Not found: the authorizer's answer for another workspace's notebook,
		// whose role in it counts for nothing here.
		{"not found", readers, Facts{
			Workspace: Membership{Active: true, Role: shared.WorkspaceMember}, Notebook: Notebook{Access: shared.AccessEditor, Role: admin},
		}, "", shared.ErrNotVisible},
		// A notebook role needs the workspace: an ended membership sees nothing.
		{"its admin, out of the workspace", readers, Facts{
			Workspace: Membership{Role: shared.WorkspaceMember}, Notebook: Notebook{Found: true, Access: shared.AccessEditor, Role: admin},
		}, "", shared.ErrNotVisible},
		{"a role outside the three", readers, facts(shared.WorkspaceMember, shared.AccessNone, "owner"), "", shared.ErrNotVisible},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grant, err := Decide(tt.rule, tt.facts)
			switch {
			case tt.err == nil && (err != nil || grant != shared.Grant{WorkspaceRole: tt.facts.Workspace.Role, NotebookRole: tt.want}):
				t.Errorf("Decide() = %+v, %v; want the grant of %q", grant, err, tt.want)
			case tt.err != nil && (!errors.Is(err, tt.err) || grant != shared.Grant{}):
				t.Errorf("Decide() = %+v, %v; want %v", grant, err, tt.err)
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
