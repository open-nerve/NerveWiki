package httpadapter_test

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type fakeUpdateMe struct {
	got domain.UserPatch
	err error
}

func (f *fakeUpdateMe) Execute(_ context.Context, p domain.UserPatch) (domain.User, error) {
	f.got = p
	if f.err != nil {
		return domain.User{}, f.err
	}
	name := "alice"
	if p.DisplayName != nil {
		name = *p.DisplayName
	}
	return domain.User{ID: uuid.MustParse(userIDText), Email: "alice@corp.com", DisplayName: name}, nil
}

type fakeRecordStep struct {
	step string
	err  error
}

func (f *fakeRecordStep) Execute(_ context.Context, step string) (domain.User, error) {
	f.step = step
	if f.err != nil {
		return domain.User{}, f.err
	}
	return domain.User{ID: uuid.MustParse(userIDText), Email: "alice@corp.com", DisplayName: "alice", OnboardingSteps: []string{step}}, nil
}

type fakeChangePassword struct {
	got   app.ChangePasswordInput
	calls int
	err   error
}

func (f *fakeChangePassword) Execute(_ context.Context, in app.ChangePasswordInput) error {
	f.got = in
	f.calls++
	return f.err
}

func patchMe(body string) *http.Request {
	req := withToken(postJSON("/api/v0/me", body))
	req.Method = http.MethodPatch
	return req
}

func TestUpdateMe(t *testing.T) {
	uc := &fakeUpdateMe{}
	h := newServer(t, httpadapter.UseCases{UpdateMe: uc})

	res, body := do(t, h, patchMe(`{"display_name":"Alice Stone"}`))
	empty, _ := do(t, h, patchMe(`{}`))

	if res.StatusCode != http.StatusOK || !strings.Contains(body, `"display_name":"Alice Stone"`) || empty.StatusCode != http.StatusOK || uc.got.DisplayName != nil {
		t.Errorf("update = %d %s, empty = %d with %+v; want 200 with the account, then an empty patch", res.StatusCode, body, empty.StatusCode, uc.got)
	}
}

func TestUpdateMeInvalid(t *testing.T) {
	invalid := shared.Invalid(shared.FieldError{Field: "display_name", Code: shared.FieldRequired, Message: "is required"})
	h := newServer(t, httpadapter.UseCases{UpdateMe: &fakeUpdateMe{err: invalid}})

	res, body := do(t, h, patchMe(`{"display_name":" "}`))

	if res.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body, `"code":"validation_failed"`) {
		t.Errorf("update = %d %s, want 422 validation_failed", res.StatusCode, body)
	}
}

func TestRecordOnboardingStep(t *testing.T) {
	uc := &fakeRecordStep{}
	h := newServer(t, httpadapter.UseCases{RecordOnboardingStep: uc})

	res, body := do(t, h, withToken(postJSON("/api/v0/me/onboarding-steps", `{"step":"profile"}`)))
	invalid, invalidBody := do(t, newServer(t, httpadapter.UseCases{RecordOnboardingStep: &fakeRecordStep{err: domain.ErrTooManyOnboardingSteps}}),
		withToken(postJSON("/api/v0/me/onboarding-steps", `{"step":"one_more"}`)))

	if res.StatusCode != http.StatusOK || uc.step != "profile" || !strings.Contains(body, `"onboarding_steps":["profile"]`) {
		t.Errorf("record = %d %s, step %q; want 200 with the step", res.StatusCode, body, uc.step)
	}
	if invalid.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(invalidBody, `"code":"out_of_range"`) {
		t.Errorf("record beyond the bound = %d %s, want 422 out_of_range", invalid.StatusCode, invalidBody)
	}
}

func TestChangePassword(t *testing.T) {
	uc := &fakeChangePassword{}
	h := newServer(t, httpadapter.UseCases{ChangePassword: uc})

	res, body := do(t, h, withToken(postJSON("/api/v0/me/change-password", `{"current_password":"old","new_password":"N3w-Passw0rd!"}`)))

	if res.StatusCode != http.StatusNoContent || body != "" || uc.got != (app.ChangePasswordInput{Current: "old", New: "N3w-Passw0rd!"}) {
		t.Errorf("change = %d %q, use case got %+v; want 204 with both passwords passed", res.StatusCode, body, uc.got)
	}
}

func TestChangePasswordProblems(t *testing.T) {
	invalid := shared.Invalid(shared.FieldError{Field: "new_password", Code: shared.FieldTooShort, Message: "must be at least 8 characters"})
	for _, tt := range []struct {
		err    error
		status int
		code   string
	}{
		{invalid, 422, "validation_failed"},
		{domain.ErrCurrentPasswordIncorrect, 422, "identity.current_password_incorrect"},
		{shared.ServerBusy(time.Second), 503, "server_busy"},
	} {
		h := newServer(t, httpadapter.UseCases{ChangePassword: &fakeChangePassword{err: tt.err}})

		res, body := do(t, h, withToken(postJSON("/api/v0/me/change-password", `{"current_password":"x","new_password":"y"}`)))

		if res.StatusCode != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) {
			t.Errorf("change = %d %s, want %d %s", res.StatusCode, body, tt.status, tt.code)
		}
	}
}

// Changing the password takes a unit of password_user, as creating a token
// does: both verify the current password.
func TestChangePasswordIsLimitedPerAccount(t *testing.T) {
	uc := &fakeChangePassword{err: domain.ErrCurrentPasswordIncorrect}
	var logs bytes.Buffer
	h := limitedServer(t, httpadapter.UseCases{ChangePassword: uc, CreateAPIToken: &fakeCreateToken{}}, &logs)

	do(t, h, withToken(postJSON("/api/v0/me/change-password", `{"current_password":"x","new_password":"y"}`)))
	do(t, h, withToken(postJSON("/api/v0/me/api-tokens", `{"name":"CI","current_password":"x"}`)))
	res, _ := do(t, h, withToken(postJSON("/api/v0/me/change-password", `{"current_password":"x","new_password":"y"}`)))

	if res.StatusCode != http.StatusTooManyRequests || uc.calls != 1 {
		t.Errorf("third attempt = %d after %d calls; want 429, the bucket shared with creating a token", res.StatusCode, uc.calls)
	}
}

type fakeDeactivate struct{ calls int }

func (f *fakeDeactivate) Execute(context.Context) error {
	f.calls++
	return nil
}

func TestDeactivateMe(t *testing.T) {
	uc := &fakeDeactivate{}
	h := newServer(t, httpadapter.UseCases{Deactivate: uc})

	res, body := do(t, h, withToken(postJSON("/api/v0/me/deactivate", "")))

	if res.StatusCode != http.StatusNoContent || body != "" || uc.calls != 1 {
		t.Errorf("deactivate = %d %q after %d calls, want 204 once", res.StatusCode, body, uc.calls)
	}
}
