package httpadapter_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// errUseCase answers err, or nil, for any of P2's use cases.
type errUseCase struct{ err error }

func (f errUseCase) Execute(context.Context, string) error { return f.err }

type fakeRename struct{ err error }

func (f fakeRename) Execute(_ context.Context, slug, name string) (app.Membership, error) {
	if f.err != nil {
		return app.Membership{}, f.err
	}
	m := membership(slug, shared.WorkspaceAdmin)
	m.Workspace.Name = name
	return m, nil
}

type fakeMembers struct{ list []app.ListedMember }

func (f fakeMembers) Execute(context.Context, string) ([]app.ListedMember, error) { return f.list, nil }

type fakeUpdateMember struct {
	got string
	err error
}

func (f *fakeUpdateMember) Execute(_ context.Context, _ uuid.UUID, role string) (app.ListedMember, error) {
	f.got = role
	if f.err != nil {
		return app.ListedMember{}, f.err
	}
	m := bob(nil)
	m.Role = shared.WorkspaceRole(role)
	return m, nil
}

type fakeRemove struct{ err error }

func (f fakeRemove) Execute(context.Context, uuid.UUID) error { return f.err }

// bob is a member as the use cases list him, with email when not nil.
func bob(email *string) app.ListedMember {
	return app.ListedMember{
		Member: domain.Member{
			ID: uuid.MustParse("0199a2b4-0000-7000-8000-00000000000b"), WorkspaceID: uuid.MustParse("0199a2b4-0000-7000-8000-00000000000a"),
			UserID: uuid.MustParse("0199a2b4-0000-7000-8000-00000000000c"), Role: shared.WorkspaceMember,
			CreatedAt: time.Date(2026, 10, 1, 11, 0, 0, 0, time.UTC),
		},
		DisplayName: "Bob", Email: email,
	}
}

const bobPath = "/api/v0/workspace-members/0199a2b4-0000-7000-8000-00000000000b"

func bobJSON(role, email string) string {
	return `{"created_at":"2026-10-01T11:00:00Z","display_name":"Bob","email":` + email +
		`,"id":"0199a2b4-0000-7000-8000-00000000000b","role":"` + role + `","user_id":"0199a2b4-0000-7000-8000-00000000000c"}`
}

func TestUpdateWorkspace(t *testing.T) {
	status, body := call(t, serve(t, httpadapter.UseCases{UpdateWorkspace: fakeRename{}}), http.MethodPatch, "/api/v0/workspaces/acme",
		`{"name":"Acme Labs"}`)
	want := strings.Replace(strings.Replace(acmeJSON, "%s", "admin", 1), `"name":"Acme"`, `"name":"Acme Labs"`, 1)
	if status != http.StatusOK || body != want+"\n" {
		t.Errorf("PATCH = %d %s, want 200 %s", status, body, want)
	}
}

func TestDeleteWorkspaceAndLeave(t *testing.T) {
	h := serve(t, httpadapter.UseCases{DeleteWorkspace: errUseCase{}, LeaveWorkspace: errUseCase{}})
	for _, req := range [][2]string{{http.MethodDelete, "/api/v0/workspaces/acme"}, {http.MethodPost, "/api/v0/workspaces/acme/leave"}} {
		if status, body := call(t, h, req[0], req[1], ""); status != http.StatusNoContent || body != "" {
			t.Errorf("%s %s = %d %q, want 204 and no body", req[0], req[1], status, body)
		}
	}
}

func TestListWorkspaceMembers(t *testing.T) {
	email := "bob@corp.com"
	for _, tt := range []struct {
		name string
		list []app.ListedMember
		want string
	}{
		{"with emails", []app.ListedMember{bob(&email)}, `{"data":[` + bobJSON("member", `"bob@corp.com"`) + `]}`},
		{"to a guest", []app.ListedMember{bob(nil)}, `{"data":[` + bobJSON("member", "null") + `]}`},
	} {
		status, body := call(t, serve(t, httpadapter.UseCases{ListMembers: fakeMembers{list: tt.list}}), http.MethodGet,
			"/api/v0/workspaces/acme/members", "")
		if status != http.StatusOK || body != tt.want+"\n" {
			t.Errorf("%s: %d %s, want 200 %s", tt.name, status, body, tt.want)
		}
	}
}

func TestUpdateWorkspaceMember(t *testing.T) {
	update := &fakeUpdateMember{}
	status, body := call(t, serve(t, httpadapter.UseCases{UpdateMember: update}), http.MethodPatch, bobPath, `{"role":"guest"}`)
	if want := bobJSON("guest", "null"); status != http.StatusOK || body != want+"\n" || update.got != "guest" {
		t.Errorf("PATCH = %d %s, the use case got %q; want 200 %s", status, body, update.got, want)
	}
}

func TestRemoveWorkspaceMember(t *testing.T) {
	if status, body := call(t, serve(t, httpadapter.UseCases{RemoveMember: fakeRemove{}}), http.MethodDelete, bobPath, ""); status != http.StatusNoContent || body != "" {
		t.Errorf("DELETE = %d %q, want 204 and no body", status, body)
	}
}

// Each code the six operations declare, answered once (v0.1 design 13.1,
// item 8).
func TestTheMemberOperationsAnswerEachProblem(t *testing.T) {
	invalid := func(field string) error {
		return shared.Invalid(shared.FieldError{Field: field, Code: shared.FieldInvalidFormat, Message: "is wrong"})
	}
	for _, tt := range []struct {
		uc           httpadapter.UseCases
		method, path string
		body         string
		status       int
		code         string
	}{
		{httpadapter.UseCases{UpdateWorkspace: fakeRename{err: domain.ErrNotFound}}, http.MethodPatch, "/api/v0/workspaces/acme", `{"name":"x"}`, 404, "workspace.not_found"},
		{httpadapter.UseCases{UpdateWorkspace: fakeRename{err: shared.Forbidden()}}, http.MethodPatch, "/api/v0/workspaces/acme", `{"name":"x"}`, 403, "forbidden"},
		{httpadapter.UseCases{UpdateWorkspace: fakeRename{err: invalid("name")}}, http.MethodPatch, "/api/v0/workspaces/acme", `{"name":" "}`, 422, "validation_failed"},
		{httpadapter.UseCases{DeleteWorkspace: errUseCase{err: domain.ErrNotFound}}, http.MethodDelete, "/api/v0/workspaces/acme", "", 404, "workspace.not_found"},
		{httpadapter.UseCases{DeleteWorkspace: errUseCase{err: shared.Forbidden()}}, http.MethodDelete, "/api/v0/workspaces/acme", "", 403, "forbidden"},
		{httpadapter.UseCases{ListMembers: fakeListErr{err: domain.ErrNotFound}}, http.MethodGet, "/api/v0/workspaces/acme/members", "", 404, "workspace.not_found"},
		{httpadapter.UseCases{UpdateMember: &fakeUpdateMember{err: invalid("role")}}, http.MethodPatch, bobPath, `{"role":"owner"}`, 422, "validation_failed"},
		{httpadapter.UseCases{UpdateMember: &fakeUpdateMember{err: domain.ErrMemberNotFound}}, http.MethodPatch, bobPath, `{"role":"guest"}`, 404, "workspace.member_not_found"},
		{httpadapter.UseCases{UpdateMember: &fakeUpdateMember{err: shared.Forbidden()}}, http.MethodPatch, bobPath, `{"role":"guest"}`, 403, "forbidden"},
		{httpadapter.UseCases{UpdateMember: &fakeUpdateMember{err: domain.ErrOwnMembership}}, http.MethodPatch, bobPath, `{"role":"guest"}`, 409, "workspace.own_membership"},
		{httpadapter.UseCases{RemoveMember: fakeRemove{err: domain.ErrMemberNotFound}}, http.MethodDelete, bobPath, "", 404, "workspace.member_not_found"},
		{httpadapter.UseCases{RemoveMember: fakeRemove{err: shared.Forbidden()}}, http.MethodDelete, bobPath, "", 403, "forbidden"},
		{httpadapter.UseCases{RemoveMember: fakeRemove{err: domain.ErrOwnMembership}}, http.MethodDelete, bobPath, "", 409, "workspace.own_membership"},
		{httpadapter.UseCases{LeaveWorkspace: errUseCase{err: domain.ErrNotFound}}, http.MethodPost, "/api/v0/workspaces/acme/leave", "", 404, "workspace.not_found"},
		{httpadapter.UseCases{LeaveWorkspace: errUseCase{err: domain.ErrSoleAdmin}}, http.MethodPost, "/api/v0/workspaces/acme/leave", "", 409, "workspace.sole_admin"},
		// The notebook module's rule two, a vetoer's refusal: its code is
		// spelt out here, the workspace module does not import it.
		{httpadapter.UseCases{LeaveWorkspace: errUseCase{err: shared.NewError(shared.KindConflict, "notebook.sole_admin", "…")}},
			http.MethodPost, "/api/v0/workspaces/acme/leave", "", 409, "notebook.sole_admin"},
	} {
		status, body := call(t, serve(t, tt.uc), tt.method, tt.path, tt.body)
		if status != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) {
			t.Errorf("%s %s answering %s: %d %s, want %d", tt.method, tt.path, tt.code, status, body, tt.status)
		}
	}
}

type fakeListErr struct{ err error }

func (f fakeListErr) Execute(context.Context, string) ([]app.ListedMember, error) { return nil, f.err }
