package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type patAuth struct {
	uc     *app.Authenticate
	tokens *fakeAPITokens
	store  *fakeStore
	logs   *bytes.Buffer
}

func newPATAuth(token app.APITokenCredential) patAuth {
	f := patAuth{tokens: newFakeAPITokens(token), store: &fakeStore{}, logs: &bytes.Buffer{}}
	f.uc = app.NewAuthenticate(app.AuthenticateDeps{
		AccessTokens: newFakeTokens(), Sessions: f.store, APITokens: f.tokens, Touch: f.tokens,
		Clock: fixedClock(testNow()), Logger: slog.New(slog.NewJSONHandler(f.logs, nil)),
	})
	return f
}

// A personal access token is found by its hash, without a session, and its
// use is recorded. One without an expiry never expires; one with expires
// then.
func TestAuthenticateAPersonalAccessToken(t *testing.T) {
	f := newPATAuth(validToken())

	auth, err := f.uc.Execute(context.Background(), testPAT().String())

	if err != nil || auth != (app.Authenticated{Actor: shared.Actor{UserID: testUserID(), APITokenID: testTokenID()}}) {
		t.Errorf("Execute() = %+v, %v; want the account with the token, never expiring", auth, err)
	}
	expiring := validToken()
	at := testNow().Add(time.Hour)
	expiring.ExpiresAt = &at
	if auth, err := newPATAuth(expiring).uc.Execute(context.Background(), testPAT().String()); err != nil || !auth.ExpiresAt.Equal(at) {
		t.Errorf("Execute() of a token expiring at %v = %+v, %v", at, auth, err)
	}
	if len(f.store.credentials) != 0 {
		t.Errorf("sessions looked up = %v, want none", f.store.credentials)
	}
	if len(f.tokens.touches) != 1 || !f.tokens.touches[0].Equal(testNow()) || !f.tokens.stale[0].Equal(testNow().Add(-time.Minute)) {
		t.Errorf("touches = %v, stale before %v; want one at the clock's now, a minute's staleness", f.tokens.touches, f.tokens.stale)
	}
}

// last_used_at is written at most once a minute: not when the last use is
// within it.
func TestAuthenticateTouchesATokenAtMostOnceAMinute(t *testing.T) {
	for _, tt := range []struct {
		name     string
		lastUsed time.Time
		touched  bool
	}{
		{"used a minute and a microsecond ago", testNow().Add(-time.Minute - time.Microsecond), true},
		{"used a minute ago", testNow().Add(-time.Minute), false},
		{"used a second ago", testNow().Add(-time.Second), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			token := validToken()
			token.LastUsedAt = &tt.lastUsed
			f := newPATAuth(token)

			if _, err := f.uc.Execute(context.Background(), testPAT().String()); err != nil {
				t.Fatal(err)
			}

			if touched := len(f.tokens.touches) == 1; touched != tt.touched {
				t.Errorf("touched = %v, want %v", touched, tt.touched)
			}
		})
	}
}

// Recording the use is best effort: the token still authenticates, and the
// failure is a warning with the token's id.
func TestAuthenticateWhenLastUsedIsNotWritten(t *testing.T) {
	f := newPATAuth(validToken())
	f.tokens.touchErr = errors.New("connection reset")

	auth, err := f.uc.Execute(context.Background(), testPAT().String())

	if err != nil || auth.Actor.APITokenID != testTokenID() {
		t.Errorf("Execute() = %+v, %v; want the token's actor", auth, err)
	}
	logs := f.logs.String()
	for _, want := range []string{`"level":"WARN"`, `"token_id":"` + tokenIDText + `"`, "connection reset"} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs lack %s:\n%s", want, logs)
		}
	}
	if strings.Contains(logs, testPAT().String()) {
		t.Errorf("logs hold the token:\n%s", logs)
	}
}

func TestAuthenticateRejectsPersonalAccessTokens(t *testing.T) {
	now := testNow()
	other := testPAT()
	other[0] = 0xff
	tests := []struct {
		name   string
		token  string
		cred   func(*app.APITokenCredential)
		reason string
	}{
		{"a malformed token", domain.PATPrefix + "AAAA", nil, "personal access token is malformed"},
		{"an unknown token", other.String(), nil, "personal access token does not exist"},
		{"a revoked token", "", func(c *app.APITokenCredential) { c.Revoked = true }, "personal access token is revoked"},
		{"a token expiring now", "", func(c *app.APITokenCredential) { c.ExpiresAt = &now }, "personal access token has expired"},
		{"a deactivated account", "", func(c *app.APITokenCredential) { c.UserActive = false }, "account is deactivated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := validToken()
			if tt.cred != nil {
				tt.cred(&cred)
			}
			f := newPATAuth(cred)
			token := tt.token
			if token == "" {
				token = testPAT().String()
			}

			_, err := f.uc.Execute(context.Background(), token)

			var se *shared.Error
			if !errors.As(err, &se) || se.ProblemStatus() != 401 || errors.Is(err, app.ErrAccessTokenExpired) {
				t.Fatalf("Execute() = %v, want 401 unauthorized, not an expired access token", err)
			}
			if want := "Authentication is required.: " + tt.reason; err.Error() != want {
				t.Errorf("error = %q, want %q", err, want)
			}
			if len(f.tokens.touches) != 0 {
				t.Errorf("a refused token was touched")
			}
		})
	}
}

func TestAuthenticatePATDatabaseFailureIsNot401(t *testing.T) {
	boom := errors.New("connection refused")
	f := newPATAuth(validToken())
	f.tokens.readErr = boom

	_, err := f.uc.Execute(context.Background(), testPAT().String())

	var se *shared.Error
	if !errors.Is(err, boom) || errors.As(err, &se) {
		t.Errorf("Execute() = %v, want the database error, not a problem", err)
	}
}

// fakeLocker is the account row lock over one account.
type fakeLocker struct {
	account app.LockedAccount
	err     error
	locked  []uuid.UUID
	inTx    []bool
}

func (f *fakeLocker) LockForCredentials(ctx context.Context, id uuid.UUID) (app.LockedAccount, error) {
	f.locked = append(f.locked, id)
	f.inTx = append(f.inTx, inTx(ctx))
	return f.account, f.err
}

func activeAccount() app.LockedAccount {
	return app.LockedAccount{Email: "alice@corp.com", PasswordHash: "hashed:Tr0ub4dor&3", Active: true}
}

func sessionActor() shared.Actor {
	return shared.Actor{UserID: testUserID(), SessionID: testSessionID()}
}

func tokenActor() shared.Actor { return shared.Actor{UserID: testUserID(), APITokenID: testTokenID()} }

// The lock returns the account once the caller's credential holds, read
// after the lock: a session, or a personal access token.
func TestCredentialLockChecksTheCallersCredential(t *testing.T) {
	for _, actor := range []shared.Actor{sessionActor(), tokenActor()} {
		locker := &fakeLocker{account: activeAccount()}
		store := &fakeStore{credential: validCredential()}
		tokens := newFakeAPITokens(validToken())
		lock := app.CredentialLock{Locker: locker, Sessions: store, APITokens: tokens}

		got, err := lock.Lock(context.Background(), actor, testNow())

		if err != nil || got != activeAccount() || len(locker.locked) != 1 || locker.locked[0] != testUserID() {
			t.Errorf("Lock(%+v) = %+v, %v; locked %v; want the account, locked once", actor, got, err, locker.locked)
		}
		sessionReads, tokenReads := len(store.credentials), len(tokens.byID)
		if actor.APITokenID != uuid.Nil() && (sessionReads != 0 || tokenReads != 1) ||
			actor.SessionID != uuid.Nil() && (sessionReads != 1 || tokenReads != 0) {
			t.Errorf("Lock(%+v) read %d sessions and %d tokens, want only the actor's credential", actor, sessionReads, tokenReads)
		}
	}
}

func TestCredentialLockRefuses(t *testing.T) {
	now := testNow()
	other := uuid.MustParse("0199a2b4-0000-7000-8000-000000000009")
	tests := []struct {
		name    string
		actor   shared.Actor
		account func(*app.LockedAccount)
		lockErr error
		session func(*app.SessionCredential)
		token   func(*app.APITokenCredential)
		reason  string
	}{
		{"no account", sessionActor(), nil, app.ErrNotFound, nil, nil, "account does not exist"},
		{"a deactivated account", sessionActor(), func(a *app.LockedAccount) { a.Active = false }, nil, nil, nil, "account is deactivated"},
		{"a revoked session", sessionActor(), nil, nil, func(c *app.SessionCredential) { c.Revoked = true }, nil, "session is revoked"},
		{"an expired session", sessionActor(), nil, nil, func(c *app.SessionCredential) { c.ExpiresAt = now }, nil, "session has expired"},
		{"a revoked token", tokenActor(), nil, nil, nil, func(c *app.APITokenCredential) { c.Revoked = true }, "personal access token is revoked"},
		{"an expired token", tokenActor(), nil, nil, nil, func(c *app.APITokenCredential) { c.ExpiresAt = &now }, "personal access token has expired"},
		{"another account's token", tokenActor(), nil, nil, nil, func(c *app.APITokenCredential) { c.UserID = other }, "personal access token belongs to another account"},
		{"an unknown token", shared.Actor{UserID: testUserID(), APITokenID: other}, nil, nil, nil, nil, "personal access token does not exist"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account, session, token := activeAccount(), validCredential(), validToken()
			apply(tt.account, &account)
			apply(tt.session, &session)
			apply(tt.token, &token)
			lock := app.CredentialLock{
				Locker:   &fakeLocker{account: account, err: tt.lockErr},
				Sessions: &fakeStore{credential: session}, APITokens: newFakeAPITokens(token),
			}

			_, err := lock.Lock(context.Background(), tt.actor, now)

			var se *shared.Error
			if !errors.As(err, &se) || se.ProblemStatus() != 401 {
				t.Fatalf("Lock() = %v, want 401", err)
			}
			if want := "Authentication is required.: " + tt.reason; err.Error() != want {
				t.Errorf("error = %q, want %q", err, want)
			}
		})
	}
}

// apply makes change to v, when there is one.
func apply[T any](change func(*T), v *T) {
	if change != nil {
		change(v)
	}
}
