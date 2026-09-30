package app_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func newAuthenticate(cred app.SessionCredential, credErr error) (*app.Authenticate, *fakeTokens, *fakeStore) {
	tokens := newFakeTokens()
	store := &fakeStore{credential: cred, credErr: credErr}
	uc := app.NewAuthenticate(app.AuthenticateDeps{AccessTokens: tokens, Sessions: store, Clock: fixedClock(testNow())})
	return uc, tokens, store
}

func validCredential() app.SessionCredential {
	return app.SessionCredential{UserID: testUserID(), ExpiresAt: testNow().Add(time.Hour), UserActive: true}
}

func TestAuthenticateAValidToken(t *testing.T) {
	uc, tokens, store := newAuthenticate(validCredential(), nil)
	token, _ := tokens.Issue(app.AccessClaims{UserID: testUserID(), SessionID: testSessionID(), ExpiresAt: testNow().Add(time.Minute)})

	actor, err := uc.Execute(context.Background(), token)

	if err != nil || actor != (shared.Actor{UserID: testUserID(), SessionID: testSessionID()}) {
		t.Errorf("Execute() = %+v, %v", actor, err)
	}
	if len(store.credentials) != 1 || store.credentials[0] != testSessionID() {
		t.Errorf("sessions looked up = %v, want one lookup of %v", store.credentials, testSessionID())
	}
	if len(tokens.verifiedAt) != 1 || !tokens.verifiedAt[0].Equal(testNow()) {
		t.Errorf("token verified at %v, want once at the clock's %v", tokens.verifiedAt, testNow())
	}
}

func TestAuthenticateRejects(t *testing.T) {
	other := uuid.MustParse("0199a2b4-0000-7000-8000-000000000009")
	now := testNow()
	tests := []struct {
		name    string
		cred    func(*app.SessionCredential)
		credErr error
		token   string
		reason  string
	}{
		{"a token that is not ours", nil, nil, "garbage", "signature is invalid"},
		{"a refresh token as bearer", nil, nil, domain.RefreshTokenPrefix + "AAAA", "signature is invalid"},
		{"an expired access token", nil, nil, "expired", "access token expired"},
		{"an unknown session", nil, app.ErrNotFound, "", "session does not exist"},
		{"another account's session", func(c *app.SessionCredential) { c.UserID = other }, nil, "", "session belongs to another account"},
		{"a revoked session", func(c *app.SessionCredential) { c.Revoked = true }, nil, "", "session is revoked"},
		{"a session expiring now", func(c *app.SessionCredential) { c.ExpiresAt = now }, nil, "", "session has expired"},
		{"a deactivated account", func(c *app.SessionCredential) { c.UserActive = false }, nil, "", "account is deactivated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := validCredential()
			if tt.cred != nil {
				tt.cred(&cred)
			}
			uc, tokens, _ := newAuthenticate(cred, tt.credErr)
			token := tt.token
			switch token {
			case "":
				token, _ = tokens.Issue(app.AccessClaims{UserID: testUserID(), SessionID: testSessionID(), ExpiresAt: now.Add(time.Minute)})
			case "expired":
				token, _ = tokens.Issue(app.AccessClaims{UserID: testUserID(), SessionID: testSessionID(), ExpiresAt: now})
			}

			_, err := uc.Execute(context.Background(), token)

			var se *shared.Error
			if !errors.As(err, &se) || se.ProblemStatus() != 401 || se.Code != shared.CodeUnauthorized {
				t.Fatalf("Execute() = %v, want 401 unauthorized", err)
			}
			if want := "Authentication is required.: " + tt.reason; err.Error() != want {
				t.Errorf("error = %q, want %q", err, want)
			}
		})
	}
}

// Only a valid signature whose exp has come, at that instant or past it, is
// "expired": the client's cue to refresh.
func TestAuthenticateTellsAnExpiredAccessToken(t *testing.T) {
	for _, exp := range []time.Time{testNow(), testNow().Add(-time.Second)} {
		uc, tokens, _ := newAuthenticate(validCredential(), nil)
		old, _ := tokens.Issue(app.AccessClaims{UserID: testUserID(), SessionID: testSessionID(), ExpiresAt: exp})

		_, expired := uc.Execute(context.Background(), old)
		_, forged := uc.Execute(context.Background(), "forged")

		if !errors.Is(expired, app.ErrAccessTokenExpired) || errors.Is(forged, app.ErrAccessTokenExpired) {
			t.Errorf("exp %v: expired = %v, forged = %v; want only the first to be ErrAccessTokenExpired", exp, expired, forged)
		}
	}
}

func TestAuthenticateDatabaseFailureIsNot401(t *testing.T) {
	boom := errors.New("connection refused")
	uc, tokens, _ := newAuthenticate(app.SessionCredential{}, boom)
	token, _ := tokens.Issue(app.AccessClaims{UserID: testUserID(), SessionID: testSessionID(), ExpiresAt: testNow().Add(time.Minute)})

	_, err := uc.Execute(context.Background(), token)

	var se *shared.Error
	if !errors.Is(err, boom) || errors.As(err, &se) {
		t.Errorf("Execute() = %v, want the database error, not a problem", err)
	}
}
