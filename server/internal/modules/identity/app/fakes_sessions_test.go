package app_test

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
)

// fakeLogins is login's ports over one account, in memory.
type fakeLogins struct {
	account     app.LoginAccount // found by email
	email       string           // the account's normalized address
	active      bool
	hash        string      // the row's hash, as LockForCredentials reads it
	lookedUp    []string    // addresses FindLoginAccount was given
	locks       int         // LockForCredentials calls
	hashUpdates []string    // hashes UpdatePasswordHash wrote
	hashTimes   []time.Time // the times it was given
	sessions    []app.NewSession
	outsideTx   []string
}

func (f *fakeLogins) FindLoginAccount(_ context.Context, email string) (app.LoginAccount, error) {
	f.lookedUp = append(f.lookedUp, email)
	if email != f.email {
		return app.LoginAccount{}, app.ErrNotFound
	}
	return f.account, nil
}

func (f *fakeLogins) LockForCredentials(ctx context.Context, id uuid.UUID) (app.LockedAccount, error) {
	f.locks++
	if !inTx(ctx) {
		f.outsideTx = append(f.outsideTx, "lock")
	}
	if id != f.account.ID {
		return app.LockedAccount{}, app.ErrNotFound
	}
	return app.LockedAccount{PasswordHash: f.hash, Active: f.active}, nil
}

func (f *fakeLogins) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string, now time.Time) error {
	if !inTx(ctx) || id != f.account.ID {
		f.outsideTx = append(f.outsideTx, "password of "+id.String())
	}
	f.hashUpdates = append(f.hashUpdates, hash)
	f.hashTimes = append(f.hashTimes, now)
	f.hash = hash
	return nil
}

func (f *fakeLogins) CreateSession(ctx context.Context, n app.NewSession) error {
	if !inTx(ctx) {
		f.outsideTx = append(f.outsideTx, "session")
	}
	f.sessions = append(f.sessions, n)
	return nil
}
