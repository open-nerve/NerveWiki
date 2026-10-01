package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// fakeInvitations admits with admit or fails with err, and records what it
// was asked.
type fakeInvitations struct {
	admit bool
	err   error
	asked []string
}

func (f *fakeInvitations) Admits(_ context.Context, id uuid.UUID, token, email string) (bool, error) {
	f.asked = append(f.asked, id.String()+" "+token+" "+email)
	return f.admit, f.err
}

// Open sign-up does not look at the invitation; closed sign-up opens only
// to an invitation the workspace module admits, for the address the
// attempt shows, and its failure is the policy's.
func TestSignupPolicy(t *testing.T) {
	id := uuid.NewV7()
	invited := identity.SignupAttempt{Email: "dana@corp.com", Invitation: &identity.SignupInvitation{ID: id, Token: "nwk_inv_x"}}
	plain := identity.SignupAttempt{Email: "dana@corp.com"}
	boom := errors.New("database down")
	for _, tt := range []struct {
		name    string
		open    bool
		check   fakeInvitations
		attempt identity.SignupAttempt
		allowed bool
		err     error
		asked   bool
	}{
		{"open, without an invitation", true, fakeInvitations{}, plain, true, nil, false},
		{"open, with one it would not admit", true, fakeInvitations{}, invited, true, nil, false},
		{"closed, without an invitation", false, fakeInvitations{admit: true}, plain, false, nil, false},
		{"closed, with one it admits", false, fakeInvitations{admit: true}, invited, true, nil, true},
		{"closed, with one it does not admit", false, fakeInvitations{}, invited, false, nil, true},
		{"closed, and the check fails", false, fakeInvitations{err: boom}, invited, false, boom, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			check := tt.check
			allowed, err := signupPolicy{open: tt.open, invitations: &check}.AllowSignup(context.Background(), tt.attempt)

			if allowed != tt.allowed || !errors.Is(err, tt.err) {
				t.Errorf("AllowSignup() = %v, %v; want %v, %v", allowed, err, tt.allowed, tt.err)
			}
			want := []string(nil)
			if tt.asked {
				want = []string{id.String() + " nwk_inv_x dana@corp.com"}
			}
			if len(check.asked) != len(want) || (len(want) == 1 && check.asked[0] != want[0]) {
				t.Errorf("the check was asked %q, want %q", check.asked, want)
			}
		})
	}
}

// The whole program, sign-up closed (M2/P3 design 3.6): only an invitation
// to the address opens it, by its link's token; registering does not
// accept it, the acceptance that follows does. Two apps share the database
// and the signing key: one open, where the admin invites, one closed.
func TestSignupClosedOpensToAnInvitation(t *testing.T) {
	contract := apitest.Load(t)
	url, keyFile := pgtest.NewDatabase(t), signingKeyFile(t)
	appWith := func(open bool) string {
		cfg := testConfig(t, url, false)
		cfg.Auth.JWT.PrivateKeyFile, cfg.Auth.SignupEnabled = keyFile, open
		return startApp(t, cfg, migrations.FS())
	}
	open := appWith(true)
	alice := registerAccount(t, contract, open, "alice@example.com").AccessToken
	if status, answer := ask(t, contract, http.MethodPost, open+"/api/v0/workspaces", alice, `{"name":"Acme","slug":"acme"}`); status != http.StatusCreated {
		t.Fatalf("create acme = %d %s", status, answer)
	}
	status, answer := ask(t, contract, http.MethodPost, open+"/api/v0/workspaces/acme/invitations", alice, `{"email":"Dana@Example.com","role":"member"}`)
	var inv struct{ ID, Token string }
	if decodeAnswer(t, answer, &inv); status != http.StatusCreated {
		t.Fatalf("invite dana = %d %s", status, answer)
	}
	closed := appWith(false)
	register := func(email, invitation string) (int, string) {
		body := `{"email":"` + email + `","password":"Tr0ub4dor&3"` + invitation + `}`
		return ask(t, contract, http.MethodPost, closed+"/api/v0/auth/register", "", body)
	}
	with := func(token string) string { return `,"invitation":{"id":"` + inv.ID + `","token":"` + token + `"}` }

	for _, tt := range []struct{ name, email, invitation string }{
		{"no invitation", "dana@example.com", ""},
		{"another address", "erin@example.com", with(inv.Token)},
		{"a wrong token", "dana@example.com", with(tampered(inv.Token))},
	} {
		if status, answer := register(tt.email, tt.invitation); status != http.StatusForbidden || problemCode(t, answer) != "identity.signup_disabled" {
			t.Errorf("register with %s = %d %s, want 403 identity.signup_disabled", tt.name, status, answer)
		}
	}
	status, answer = register(" DANA@example.com ", with(inv.Token))
	var dana authTokens
	if decodeAnswer(t, answer, &dana); status != http.StatusCreated {
		t.Fatalf("register with the invitation = %d %s, want 201", status, answer)
	}
	status, answer = ask(t, contract, http.MethodPost, closed+"/api/v0/workspace-invitations/"+inv.ID+"/accept", dana.AccessToken,
		`{"token":"`+inv.Token+`"}`)
	var joined struct{ Slug, Role string }
	if decodeAnswer(t, answer, &joined); status != http.StatusOK || joined.Slug != "acme" || joined.Role != "member" {
		t.Errorf("accept = %d %s, want 200, acme as a member", status, answer)
	}
}

// tampered is token with its last character changed.
func tampered(token string) string {
	last := "A"
	if strings.HasSuffix(token, "A") {
		last = "B"
	}
	return token[:len(token)-1] + last
}
