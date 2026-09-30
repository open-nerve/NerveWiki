package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type createTokenFixture struct {
	account *fakeAccount
	hasher  *fakeHasher
	tx      *fakeTx
	tokens  *fakeTokenStore
	logs    *bytes.Buffer
	uc      *app.CreateAPIToken
}

func newCreateToken() *createTokenFixture {
	f := &createTokenFixture{
		account: newFakeAccount("hashed:Tr0ub4dor&3"), hasher: &fakeHasher{}, tx: &fakeTx{},
		tokens: &fakeTokenStore{}, logs: &bytes.Buffer{},
	}
	f.uc = app.NewCreateAPIToken(app.CreateAPITokenDeps{
		Password: currentPassword(f.account, f.hasher, f.tx), Tokens: f.tokens,
		Clock: fixedClock(testNow()), Logger: slog.New(slog.NewJSONHandler(f.logs, nil)),
	})
	return f
}

func (f *createTokenFixture) create(actor shared.Actor, in app.CreateAPITokenInput) (app.CreatedAPIToken, error) {
	return f.uc.Execute(shared.WithActor(context.Background(), actor), in)
}

// The token is inserted under the lock with the hash of the token answered,
// this once; the row and the answer hold the spec as the check returns it.
// A personal access token may create one too.
func TestCreateAPIToken(t *testing.T) {
	for _, actor := range []shared.Actor{sessionActor(), tokenActor()} {
		f := newCreateToken()
		expires := testNow().Add(24*time.Hour + 999*time.Nanosecond)

		got, err := f.create(actor, app.CreateAPITokenInput{
			Spec: domain.APITokenSpec{Name: "  CI ", ExpiresAt: &expires}, CurrentPassword: "Tr0ub4dor&3",
		})

		if err != nil || len(f.tokens.created) != 1 || len(f.tokens.outsideTx) != 0 {
			t.Fatalf("Execute(%+v) = %v; inserted %+v (outside a transaction: %v); want one token inside it", actor, err, f.tokens.created, f.tokens.outsideTx)
		}
		row := f.tokens.created[0]
		pat, ok := domain.ParsePAT(got.Token)
		stored := testNow().Add(24 * time.Hour)
		if !ok || !bytes.Equal(pat.Hash(), row.TokenHash) || row.UserID != testUserID() || row.Name != "CI" ||
			!row.ExpiresAt.Equal(stored) || !row.Now.Equal(testNow()) || row.ID != got.ID {
			t.Errorf("row %+v for token %q, want the hash of the token answered, CI, expiring at %v", row, got.Token, stored)
		}
		if got.Name != "CI" || !got.ExpiresAt.Equal(stored) || !got.CreatedAt.Equal(testNow()) || got.LastUsedAt != nil {
			t.Errorf("answer = %+v, want the row's fields, never used", got.APIToken)
		}
		logs := f.logs.String()
		if !strings.Contains(logs, `"msg":"API token created"`) || !strings.Contains(logs, `"token_id":"`+row.ID.String()+`"`) ||
			strings.Contains(logs, got.Token) {
			t.Errorf("logs = %s, want the token's id and never the token", logs)
		}
	}
}

// The spec is checked before the password: a bad name costs no argon2.
func TestCreateAPITokenChecksTheSpecFirst(t *testing.T) {
	f := newCreateToken()

	_, err := f.create(sessionActor(), app.CreateAPITokenInput{Spec: domain.APITokenSpec{Name: " "}, CurrentPassword: "Wr0ng-password"})

	var se *shared.Error
	if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed || len(f.hasher.verified) != 0 {
		t.Errorf("Execute() = %v after %d verifications, want validation_failed before any", err, len(f.hasher.verified))
	}
}

func TestCreateAPITokenWithAWrongPassword(t *testing.T) {
	f := newCreateToken()

	_, err := f.create(sessionActor(), app.CreateAPITokenInput{Spec: domain.APITokenSpec{Name: "CI"}, CurrentPassword: "Wr0ng-password"})

	if !errors.Is(err, domain.ErrCurrentPasswordIncorrect) || len(f.tokens.created) != 0 || f.tx.calls != 0 {
		t.Errorf("Execute() = %v, inserted %d, %d transactions; want current_password_incorrect and nothing", err, len(f.tokens.created), f.tx.calls)
	}
}

func TestListAPITokens(t *testing.T) {
	store := &fakeTokenStore{list: []domain.APIToken{{ID: testTokenID(), Name: "CI"}}}
	uc := app.NewListAPITokens(store)

	got, err := uc.Execute(shared.WithActor(context.Background(), sessionActor()))

	if err != nil || len(got) != 1 || got[0].Name != "CI" || !slices.Equal(store.listed, []uuid.UUID{testUserID()}) {
		t.Errorf("Execute() = %+v, %v; listed %v; want the caller's tokens", got, err, store.listed)
	}
	if _, err := uc.Execute(context.Background()); err == nil {
		t.Error("Execute() without a caller succeeded, want 401")
	}
}

func TestRevokeAPIToken(t *testing.T) {
	var logs bytes.Buffer
	store := &fakeTokenStore{revokes: true}
	uc := app.NewRevokeAPIToken(store, fixedClock(testNow()), slog.New(slog.NewJSONHandler(&logs, nil)))

	err := uc.Execute(shared.WithActor(context.Background(), tokenActor()), testTokenID())

	if err != nil || !slices.Equal(store.revoked, []uuid.UUID{testTokenID()}) || !store.revokeAt[0].Equal(testNow()) {
		t.Errorf("Execute() = %v; revoked %v at %v; want the token, at the clock's now", err, store.revoked, store.revokeAt)
	}
	if !strings.Contains(logs.String(), `"msg":"API token revoked"`) || !strings.Contains(logs.String(), tokenIDText) {
		t.Errorf("logs = %s, want the revocation with the token's id", logs.String())
	}
}

// Nothing revoked (unknown, revoked already, another account's) is one
// answer: identity.api_token_not_found.
func TestRevokeAPITokenNotFound(t *testing.T) {
	uc := app.NewRevokeAPIToken(&fakeTokenStore{revokes: false}, fixedClock(testNow()), slog.New(slog.DiscardHandler))

	err := uc.Execute(shared.WithActor(context.Background(), sessionActor()), testTokenID())

	if !errors.Is(err, domain.ErrAPITokenNotFound) {
		t.Errorf("Execute() = %v, want identity.api_token_not_found", err)
	}
}
