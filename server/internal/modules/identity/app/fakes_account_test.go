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

// fakeUsers is the test account's profile.
type fakeUsers struct {
	user  domain.User
	err   error
	names []string    // display names written
	steps []string    // steps recorded
	times []time.Time // at
	reads int
}

func (f *fakeUsers) UpdateDisplayName(_ context.Context, id uuid.UUID, name string, now time.Time) (domain.User, error) {
	if f.err != nil {
		return domain.User{}, f.err
	}
	f.names, f.times = append(f.names, name), append(f.times, now)
	f.user.DisplayName = name
	return f.user, nil
}

func (f *fakeUsers) RecordOnboardingStep(_ context.Context, id uuid.UUID, step string, now time.Time) (domain.User, error) {
	if f.err != nil {
		return domain.User{}, f.err
	}
	f.steps, f.times = append(f.steps, step), append(f.times, now)
	return f.user, nil
}

func (f *fakeUsers) GetUser(_ context.Context, id uuid.UUID) (domain.User, error) {
	f.reads++
	return f.user, f.err
}

// fakeSessionRevoker records the revocations.
type fakeSessionRevoker struct {
	keeps     []uuid.UUID
	reasons   []domain.RevokeReason
	outsideTx int
}

func (f *fakeSessionRevoker) RevokeSessions(ctx context.Context, _, keep uuid.UUID, reason domain.RevokeReason, _ time.Time) (int, error) {
	if !inTx(ctx) {
		f.outsideTx++
	}
	f.keeps, f.reasons = append(f.keeps, keep), append(f.reasons, reason)
	return 2, nil
}

// fakePasswordWriter records the hashes written.
type fakePasswordWriter struct {
	hashes    []string
	outsideTx int
}

func (f *fakePasswordWriter) UpdatePasswordHash(ctx context.Context, _ uuid.UUID, hash string, _ time.Time) error {
	if !inTx(ctx) {
		f.outsideTx++
	}
	f.hashes = append(f.hashes, hash)
	return nil
}
