package httpadapter_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// memberID is the id of every membership the fakes answer; bobID its
// account's.
func memberID() uuid.UUID { return uuid.MustParse("0199a2b4-0000-7000-8000-00000000000c") }
func bobID() uuid.UUID    { return uuid.MustParse("0199a2b4-0000-7000-8000-00000000000d") }

// bob is the membership the fakes answer, with his email when shown.
func bob(shown bool) app.ListedMember {
	m := app.ListedMember{
		Member: domain.Member{ID: memberID(), NotebookID: notebookID(), UserID: bobID(), Role: shared.NotebookEditor,
			CreatedAt: time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)},
		DisplayName: "Bob",
	}
	if shown {
		email := "bob@corp.com"
		m.Email = &email
	}
	return m
}

func bobJSON(email string) string {
	return `{"created_at":"2026-10-02T10:00:00Z","display_name":"Bob","email":` + email +
		`,"id":"0199a2b4-0000-7000-8000-00000000000c","role":"editor","user_id":"0199a2b4-0000-7000-8000-00000000000d"}`
}

type fakeListMembers struct{ fakeUseCase }

func (f *fakeListMembers) Execute(_ context.Context, id uuid.UUID) ([]app.ListedMember, error) {
	f.got = []any{id}
	if f.err != nil {
		return nil, f.err
	}
	return []app.ListedMember{bob(true), bob(false)}, nil
}

type fakeAddMember struct{ fakeUseCase }

func (f *fakeAddMember) Execute(_ context.Context, id, userID uuid.UUID, role string) (app.ListedMember, error) {
	f.got = []any{id, userID, role}
	return bob(true), f.err
}

type fakeUpdateMember struct{ fakeUseCase }

func (f *fakeUpdateMember) Execute(_ context.Context, id uuid.UUID, role string) (app.ListedMember, error) {
	f.got = []any{id, role}
	return bob(false), f.err
}

type fakeRemoveMember struct{ fakeUseCase }

func (f *fakeRemoveMember) Execute(_ context.Context, id uuid.UUID) error {
	f.got = []any{id}
	return f.err
}

type fakeLeave struct{ fakeUseCase }

func (f *fakeLeave) Execute(_ context.Context, id uuid.UUID) error {
	f.got = []any{id}
	return f.err
}

// memberFakes are the member operations' use cases, every one answering
// err.
type memberFakes struct {
	listMembers  *fakeListMembers
	addMember    *fakeAddMember
	updateMember *fakeUpdateMember
	removeMember *fakeRemoveMember
	leave        *fakeLeave
}

func newMemberFakes(err error) memberFakes {
	u := fakeUseCase{err: err}
	return memberFakes{&fakeListMembers{u}, &fakeAddMember{u}, &fakeUpdateMember{u}, &fakeRemoveMember{u}, &fakeLeave{u}}
}

const (
	membersPath = notebookPath + "/members"
	memberPath  = "/api/v0/notebook-members/0199a2b4-0000-7000-8000-00000000000c"
	leavePath   = notebookPath + "/leave"
)

func TestTheMemberOperationsAnswerTheUseCases(t *testing.T) {
	f := newFakes(nil)
	h := f.serve(t)
	for _, tt := range []struct {
		method, path, body string
		status             int
		want               string
		got                func() []any
		wantGot            []any
	}{
		{http.MethodGet, membersPath, "", http.StatusOK, `{"data":[` + bobJSON(`"bob@corp.com"`) + "," + bobJSON("null") + "]}",
			func() []any { return f.listMembers.got }, []any{notebookID()}},
		{http.MethodPost, membersPath, `{"user_id":"0199a2b4-0000-7000-8000-00000000000d","role":"editor"}`, http.StatusCreated,
			bobJSON(`"bob@corp.com"`), func() []any { return f.addMember.got }, []any{notebookID(), bobID(), "editor"}},
		{http.MethodPatch, memberPath, `{"role":"reader"}`, http.StatusOK, bobJSON("null"),
			func() []any { return f.updateMember.got }, []any{memberID(), "reader"}},
		{http.MethodDelete, memberPath, "", http.StatusNoContent, "", func() []any { return f.removeMember.got }, []any{memberID()}},
		{http.MethodPost, leavePath, "", http.StatusNoContent, "", func() []any { return f.leave.got }, []any{notebookID()}},
	} {
		status, body := call(t, h, tt.method, tt.path, tt.body)
		want := tt.want
		if want != "" {
			want += "\n"
		}
		if status != tt.status || body != want {
			t.Errorf("%s %s %s = %d %s, want %d %s", tt.method, tt.path, tt.body, status, body, tt.status, want)
		}
		if got := tt.got(); !sameArgs(got, tt.wantGot) {
			t.Errorf("%s %s %s: the use case got %v, want %v", tt.method, tt.path, tt.body, got, tt.wantGot)
		}
	}
}

// Every code the member operations declare, as the use cases answer it.
func TestTheMemberOperationsAnswerEachProblem(t *testing.T) {
	invalid := shared.Invalid(shared.FieldError{Field: "user_id", Code: shared.FieldNotAllowed, Message: "must be an active member"})
	add := `{"user_id":"0199a2b4-0000-7000-8000-00000000000d","role":"reader"}`
	for _, tt := range []struct {
		method, path, body string
		err                error
		status             int
		code               string
	}{
		{http.MethodGet, membersPath, "", domain.ErrNotFound, http.StatusNotFound, "notebook.not_found"},
		{http.MethodPost, membersPath, add, domain.ErrNotFound, http.StatusNotFound, "notebook.not_found"},
		{http.MethodPost, membersPath, add, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPost, membersPath, add, invalid, http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodPatch, memberPath, `{"role":"owner"}`, domain.ErrMemberNotFound, http.StatusNotFound, "notebook.member_not_found"},
		{http.MethodPatch, memberPath, `{"role":"owner"}`, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPatch, memberPath, `{"role":"owner"}`, invalid, http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodPatch, memberPath, `{"role":"reader"}`, domain.ErrOwnMembership, http.StatusConflict, "notebook.own_membership"},
		{http.MethodDelete, memberPath, "", domain.ErrMemberNotFound, http.StatusNotFound, "notebook.member_not_found"},
		{http.MethodDelete, memberPath, "", shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodDelete, memberPath, "", domain.ErrOwnMembership, http.StatusConflict, "notebook.own_membership"},
		{http.MethodPost, leavePath, "", domain.ErrNotFound, http.StatusNotFound, "notebook.not_found"},
		{http.MethodPost, leavePath, "", domain.ErrMemberNotFound, http.StatusNotFound, "notebook.member_not_found"},
		{http.MethodPost, leavePath, "", domain.ErrSoleAdmin, http.StatusConflict, "notebook.sole_admin"},
	} {
		status, body := call(t, newFakes(tt.err).serve(t), tt.method, tt.path, tt.body)
		if status != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) {
			t.Errorf("%s %s: %d %s, want %d %s", tt.method, tt.path, status, body, tt.status, tt.code)
		}
	}
}

// A membership's id and an account's id are uuids: anything else is the
// boundary's 400, before any use case.
func TestAMemberIDThatIsNoUUID(t *testing.T) {
	f := newFakes(nil)
	h := f.serve(t)
	if status, body := call(t, h, http.MethodDelete, "/api/v0/notebook-members/bob", ""); status != http.StatusBadRequest ||
		!strings.Contains(body, `"code":"bad_request"`) || f.removeMember.got != nil {
		t.Errorf("DELETE = %d %s, use case got %v; want 400 bad_request, the use case not called", status, body, f.removeMember.got)
	}
	if status, body := call(t, h, http.MethodPost, membersPath, `{"user_id":"bob","role":"reader"}`); status != http.StatusBadRequest ||
		f.addMember.got != nil {
		t.Errorf("POST = %d %s, use case got %v; want 400, the use case not called", status, body, f.addMember.got)
	}
}
