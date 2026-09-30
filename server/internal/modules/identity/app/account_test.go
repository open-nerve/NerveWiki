package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func asCaller(actor shared.Actor) context.Context {
	return shared.WithActor(context.Background(), actor)
}

// A display name is stored trimmed; a patch without a field writes nothing.
func TestUpdateMe(t *testing.T) {
	users := &fakeUsers{user: domain.User{ID: testUserID(), DisplayName: "alice"}}
	uc := app.NewUpdateMe(users, users, fixedClock(testNow()))
	name := "  Alice Stone "

	got, err := uc.Execute(asCaller(sessionActor()), domain.UserPatch{DisplayName: &name})

	if err != nil || got.DisplayName != "Alice Stone" || !slices.Equal(users.names, []string{"Alice Stone"}) || !users.times[0].Equal(testNow()) {
		t.Errorf("Execute() = %+v, %v; written %q; want Alice Stone at the clock's now", got, err, users.names)
	}
	if got, err := uc.Execute(asCaller(sessionActor()), domain.UserPatch{}); err != nil || got.DisplayName != "Alice Stone" ||
		len(users.names) != 1 || users.reads != 1 {
		t.Errorf("Execute(empty) = %+v, %v; %d writes, %d reads; want the account read, nothing written", got, err, len(users.names), users.reads)
	}
	empty := ""
	var se *shared.Error
	if _, err := uc.Execute(asCaller(sessionActor()), domain.UserPatch{DisplayName: &empty}); !errors.As(err, &se) || len(users.names) != 1 {
		t.Errorf("Execute(empty name) = %v, want 422 and no write", err)
	}
}

// An account gone since authentication is 401.
func TestUpdateMeOfAnAccountGone(t *testing.T) {
	users := &fakeUsers{err: app.ErrNotFound}
	name := "Alice"
	_, err := app.NewUpdateMe(users, users, fixedClock(testNow())).Execute(asCaller(sessionActor()), domain.UserPatch{DisplayName: &name})
	var se *shared.Error
	if !errors.As(err, &se) || se.ProblemStatus() != 401 {
		t.Errorf("Execute() = %v, want 401", err)
	}
}

func TestRecordOnboardingStep(t *testing.T) {
	users := &fakeUsers{user: domain.User{ID: testUserID()}}
	uc := app.NewRecordOnboardingStep(users, fixedClock(testNow()))

	_, err := uc.Execute(asCaller(tokenActor()), "profile")
	_, bad := uc.Execute(asCaller(tokenActor()), "Profile")

	var se *shared.Error
	if err != nil || !slices.Equal(users.steps, []string{"profile"}) || !errors.As(bad, &se) || se.Code != shared.CodeValidationFailed {
		t.Errorf("Execute() = %v, then %v; recorded %q; want profile recorded, then 422 before the store", err, bad, users.steps)
	}
	full := &fakeUsers{err: domain.ErrTooManyOnboardingSteps}
	if _, err := app.NewRecordOnboardingStep(full, fixedClock(testNow())).Execute(asCaller(tokenActor()), "profile"); !errors.Is(err, domain.ErrTooManyOnboardingSteps) {
		t.Errorf("Execute() beyond the bound = %v, want the store's 422", err)
	}
}

type changePasswordFixture struct {
	account  *fakeAccount
	hasher   *fakeHasher
	tx       *fakeTx
	written  *fakePasswordWriter
	sessions *fakeSessionRevoker
	logs     *bytes.Buffer
	uc       *app.ChangePassword
}

func newChangePassword() *changePasswordFixture {
	f := &changePasswordFixture{
		account: newFakeAccount("hashed:Tr0ub4dor&3"), hasher: &fakeHasher{}, tx: &fakeTx{},
		written: &fakePasswordWriter{}, sessions: &fakeSessionRevoker{}, logs: &bytes.Buffer{},
	}
	f.uc = app.NewChangePassword(app.ChangePasswordDeps{
		Password: currentPassword(f.account, f.hasher, f.tx), Rules: domain.NewPasswordRules(), Hasher: f.hasher,
		Passwords: f.written, Sessions: f.sessions, Clock: fixedClock(testNow()), Logger: slog.New(slog.NewJSONHandler(f.logs, nil)),
	})
	return f
}

// The new hash and the revocation of the other sessions are written under
// the lock: a session keeps itself, a personal access token keeps none.
func TestChangePassword(t *testing.T) {
	for _, tt := range []struct {
		actor shared.Actor
		keep  uuid.UUID
	}{{sessionActor(), testSessionID()}, {tokenActor(), uuid.Nil()}} {
		f := newChangePassword()

		err := f.uc.Execute(asCaller(tt.actor), app.ChangePasswordInput{Current: "Tr0ub4dor&3", New: "N3w-Passw0rd!"})

		if err != nil || !slices.Equal(f.written.hashes, []string{"hashed:N3w-Passw0rd!"}) || f.written.outsideTx != 0 || f.hasher.calls != 1 {
			t.Errorf("Execute(%+v) = %v; hashes %q (%d outside a transaction), %d hashings; want the new hash once, inside", tt.actor, err, f.written.hashes, f.written.outsideTx, f.hasher.calls)
		}
		if !slices.Equal(f.sessions.keeps, []uuid.UUID{tt.keep}) || f.sessions.reasons[0] != domain.RevokePasswordChanged || f.sessions.outsideTx != 0 {
			t.Errorf("Execute(%+v) revoked keeping %v for %v; want keeping %v for password_changed, inside the transaction", tt.actor, f.sessions.keeps, f.sessions.reasons, tt.keep)
		}
		logs := f.logs.String()
		if !strings.Contains(logs, `"msg":"password changed"`) || !strings.Contains(logs, `"revoked_sessions":2`) || strings.Contains(logs, "N3w-Passw0rd!") {
			t.Errorf("logs = %s, want the change with the count and no password", logs)
		}
	}
}

// The rules come first, with the account's address: a weak new password
// costs no argon2; a wrong current password hashes nothing and writes
// nothing.
func TestChangePasswordRefuses(t *testing.T) {
	f := newChangePassword()
	var se *shared.Error
	err := f.uc.Execute(asCaller(sessionActor()), app.ChangePasswordInput{Current: "Wr0ng-password", New: "alice2026"})
	if !errors.As(err, &se) || len(se.Fields) != 1 || se.Fields[0].Field != "new_password" || len(f.hasher.verified) != 0 {
		t.Errorf("Execute(weak new password) = %v after %d verifications, want 422 on new_password before any", err, len(f.hasher.verified))
	}
	err = f.uc.Execute(asCaller(sessionActor()), app.ChangePasswordInput{Current: "Wr0ng-password", New: "N3w-Passw0rd!"})
	if !errors.Is(err, domain.ErrCurrentPasswordIncorrect) || f.hasher.calls != 0 || len(f.written.hashes) != 0 || len(f.sessions.keeps) != 0 {
		t.Errorf("Execute(wrong current) = %v; %d hashings, %d writes; want current_password_incorrect and nothing done", err, f.hasher.calls, len(f.written.hashes))
	}
}
