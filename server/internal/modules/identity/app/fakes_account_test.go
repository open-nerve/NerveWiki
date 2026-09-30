package app_test

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// fakeAccount is the test account's row as the operations that ask for the
// current password see it: read before the transaction, locked inside it.
type fakeAccount struct {
	email     string
	hash      string // the row's hash
	active    bool
	readErr   error
	locks     int
	outsideTx []string // statements made outside a transaction
}

func newFakeAccount(hash string) *fakeAccount {
	return &fakeAccount{email: "alice@corp.com", hash: hash, active: true}
}

func (f *fakeAccount) PasswordAccount(_ context.Context, id uuid.UUID) (app.PasswordAccount, error) {
	if f.readErr != nil {
		return app.PasswordAccount{}, f.readErr
	}
	if id != testUserID() {
		return app.PasswordAccount{}, app.ErrNotFound
	}
	return app.PasswordAccount{Email: f.email, PasswordHash: f.hash}, nil
}

func (f *fakeAccount) LockForCredentials(ctx context.Context, id uuid.UUID) (app.LockedAccount, error) {
	f.locks++
	if !inTx(ctx) {
		f.outsideTx = append(f.outsideTx, "lock")
	}
	if id != testUserID() {
		return app.LockedAccount{}, app.ErrNotFound
	}
	return app.LockedAccount{Email: f.email, PasswordHash: f.hash, Active: f.active}, nil
}

// currentPassword is app.CurrentPassword over account, the caller's live
// session or token, hasher and tx.
func currentPassword(account *fakeAccount, hasher *fakeHasher, tx *fakeTx) app.CurrentPassword {
	return app.CurrentPassword{
		Accounts: account, Verifier: hasher, Tx: tx,
		Lock: app.CredentialLock{Locker: account, Sessions: &fakeStore{credential: validCredential()}, APITokens: newFakeAPITokens(validToken())},
	}
}

// fakeTokenStore records the tokens inserted.
type fakeTokenStore struct {
	created   []app.NewAPIToken
	outsideTx []string
	listed    []uuid.UUID // accounts whose tokens were listed
	list      []domain.APIToken
	revoked   []uuid.UUID
	revokeAt  []time.Time
	revokes   bool // RevokeAPIToken's answer
	err       error
}

func (f *fakeTokenStore) CreateAPIToken(ctx context.Context, t app.NewAPIToken) error {
	if !inTx(ctx) {
		f.outsideTx = append(f.outsideTx, "token")
	}
	f.created = append(f.created, t)
	return f.err
}

func (f *fakeTokenStore) ListAPITokens(_ context.Context, userID uuid.UUID) ([]domain.APIToken, error) {
	f.listed = append(f.listed, userID)
	return f.list, f.err
}

func (f *fakeTokenStore) RevokeAPIToken(_ context.Context, id, userID uuid.UUID, now time.Time) (bool, error) {
	if userID != testUserID() {
		return false, nil
	}
	f.revoked = append(f.revoked, id)
	f.revokeAt = append(f.revokeAt, now)
	return f.revokes, f.err
}
