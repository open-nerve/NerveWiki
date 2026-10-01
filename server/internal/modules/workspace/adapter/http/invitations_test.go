package httpadapter_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// dana's is the invitation the fakes answer.
func danas() app.ListedInvitation {
	return app.ListedInvitation{
		Invitation: domain.Invitation{
			ID: uuid.MustParse("0199a2b4-0000-7000-8000-0000000000d1"), WorkspaceID: uuid.MustParse("0199a2b4-0000-7000-8000-00000000000a"),
			Email: "dana@corp.com", Role: shared.WorkspaceMember, CreatedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		},
		Token: "nwk_inv_x",
	}
}

const (
	danaPath = "/api/v0/workspace-invitations/0199a2b4-0000-7000-8000-0000000000d1"
	danaJSON = `{"created_at":"2026-10-01T12:00:00Z","email":"dana@corp.com","id":"0199a2b4-0000-7000-8000-0000000000d1",` +
		`"role":"member","token":"nwk_inv_x"}`
)

// fakeInvitations is each of the invitations' use cases: it answers err, or
// dana's invitation, and records what it got.
type fakeInvitations struct {
	err error
	got []string
}

func (f *fakeInvitations) answer(got ...string) error {
	f.got = got
	return f.err
}

type listInvitations struct{ *fakeInvitations }

func (f listInvitations) Execute(_ context.Context, slug string) ([]app.ListedInvitation, error) {
	return []app.ListedInvitation{danas()}, f.answer(slug)
}

type createInvitation struct{ *fakeInvitations }

func (f createInvitation) Execute(_ context.Context, slug, email, role string) (app.ListedInvitation, error) {
	return danas(), f.answer(slug, email, role)
}

type deleteInvitation struct{ *fakeInvitations }

func (f deleteInvitation) Execute(_ context.Context, id uuid.UUID) error {
	return f.answer(id.String())
}

type previewInvitation struct{ *fakeInvitations }

func (f previewInvitation) Execute(_ context.Context, id uuid.UUID, token string) (app.InvitationPreview, error) {
	return app.InvitationPreview{Workspace: membership("acme", "").Workspace, Role: shared.WorkspaceGuest}, f.answer(id.String(), token)
}

type acceptInvitation struct{ *fakeInvitations }

func (f acceptInvitation) Execute(_ context.Context, id uuid.UUID, token string) (app.Membership, error) {
	return membership("acme", shared.WorkspaceMember), f.answer(id.String(), token)
}

func invitationUseCases(f *fakeInvitations) httpadapter.UseCases {
	return httpadapter.UseCases{
		ListInvitations: listInvitations{f}, CreateInvitation: createInvitation{f}, DeleteInvitation: deleteInvitation{f},
		PreviewInvitation: previewInvitation{f}, AcceptInvitation: acceptInvitation{f},
	}
}

func TestTheInvitationOperations(t *testing.T) {
	for _, tt := range []struct {
		method, path, body string
		status             int
		want               string
		got                []string
	}{
		{http.MethodGet, "/api/v0/workspaces/acme/invitations", "", 200, `{"data":[` + danaJSON + `]}`, []string{"acme"}},
		{http.MethodPost, "/api/v0/workspaces/acme/invitations", `{"email":" Dana@Corp.com","role":"member"}`, 201, danaJSON,
			[]string{"acme", " Dana@Corp.com", "member"}},
		{http.MethodDelete, danaPath, "", 204, "", []string{"0199a2b4-0000-7000-8000-0000000000d1"}},
		{http.MethodPost, danaPath + "/accept", `{"token":"nwk_inv_x"}`, 200, strings.Replace(acmeJSON, "%s", "member", 1),
			[]string{"0199a2b4-0000-7000-8000-0000000000d1", "nwk_inv_x"}},
	} {
		f := &fakeInvitations{}
		status, body := call(t, serve(t, invitationUseCases(f)), tt.method, tt.path, tt.body)

		want := tt.want
		if want != "" {
			want += "\n"
		}
		if status != tt.status || body != want || strings.Join(f.got, "|") != strings.Join(tt.got, "|") {
			t.Errorf("%s %s = %d %s, the use case got %q; want %d %s, %q", tt.method, tt.path, status, body, f.got, tt.status, tt.want, tt.got)
		}
	}
}

// The preview is public: it answers without a token, through the platform
// that takes the module's public operations.
func TestThePreviewIsPublic(t *testing.T) {
	f := &fakeInvitations{}
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	httpadapter.Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{
		Authenticator: fakeAuth{}, PublicOperations: httpadapter.PublicOperations(),
	}), invitationUseCases(f))
	req := httptest.NewRequest(http.MethodPost, danaPath+"/preview", strings.NewReader(`{"token":"nwk_inv_x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	res := rec.Result()
	apitest.Load(t).CheckResponse(t, req, res)
	body, _ := io.ReadAll(res.Body)
	if want := `{"role":"guest","workspace":{"name":"Acme","slug":"acme"}}` + "\n"; res.StatusCode != http.StatusOK || string(body) != want {
		t.Errorf("POST preview without a token = %d %s, want 200 %s", res.StatusCode, body, want)
	}
	if strings.Join(f.got, "|") != "0199a2b4-0000-7000-8000-0000000000d1|nwk_inv_x" {
		t.Errorf("the use case got %q", f.got)
	}
}

// Each code the five operations declare, answered once (v0.1 design 13.1,
// item 8).
func TestTheInvitationOperationsAnswerEachProblem(t *testing.T) {
	invalid := shared.Invalid(shared.FieldError{Field: "email", Code: shared.FieldDuplicate, Message: "is invited"})
	deactivated := shared.NewError(shared.KindForbidden, "identity.account_deactivated", "Deactivated.")
	const invitations = "/api/v0/workspaces/acme/invitations"
	for _, tt := range []struct {
		err          error
		method, path string
		body         string
		status       int
		code         string
	}{
		{domain.ErrNotFound, http.MethodGet, invitations, "", 404, "workspace.not_found"},
		{shared.Forbidden(), http.MethodGet, invitations, "", 403, "forbidden"},
		{domain.ErrNotFound, http.MethodPost, invitations, `{"email":"dana@corp.com","role":"member"}`, 404, "workspace.not_found"},
		{shared.Forbidden(), http.MethodPost, invitations, `{"email":"dana@corp.com","role":"member"}`, 403, "forbidden"},
		{invalid, http.MethodPost, invitations, `{"email":"dana@corp.com","role":"member"}`, 422, "validation_failed"},
		{domain.ErrInvitationNotFound, http.MethodDelete, danaPath, "", 404, "workspace.invitation_not_found"},
		{shared.Forbidden(), http.MethodDelete, danaPath, "", 403, "forbidden"},
		{domain.ErrInvitationNotFound, http.MethodPost, danaPath + "/preview", `{"token":"x"}`, 404, "workspace.invitation_not_found"},
		{domain.ErrInvitationNotFound, http.MethodPost, danaPath + "/accept", `{"token":"x"}`, 404, "workspace.invitation_not_found"},
		{domain.ErrInvitationEmailMismatch, http.MethodPost, danaPath + "/accept", `{"token":"x"}`, 403, "workspace.invitation_email_mismatch"},
		{deactivated, http.MethodPost, danaPath + "/accept", `{"token":"x"}`, 403, "identity.account_deactivated"},
	} {
		status, body := call(t, serve(t, invitationUseCases(&fakeInvitations{err: tt.err})), tt.method, tt.path, tt.body)
		if status != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) {
			t.Errorf("%s %s answering %s: %d %s, want %d", tt.method, tt.path, tt.code, status, body, tt.status)
		}
	}
}
