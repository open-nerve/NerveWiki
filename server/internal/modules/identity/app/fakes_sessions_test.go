package app_test

import (
	"bytes"
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
)

// fakeLogins is login's ports over one account, in memory.
type fakeLogins struct {
	account     app.LoginAccount // found by email
	email       string           // the account's normalized address
	renamed     string           // when set, the address LockForCredentials reads: changed since the lookup
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
	email := f.email
	if f.renamed != "" {
		email = f.renamed
	}
	return app.LockedAccount{ID: id, Email: email, PasswordHash: f.hash, Active: f.active}, nil
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

// fakeSession is a session row as refresh and logout see it.
type fakeSession struct {
	app.RefreshSession
	reason    string    // revoke_reason
	changedAt time.Time // updated_at, which each write sets
}

// fakeSessions is refresh's and logout's ports, in memory. RotateSession and
// EndSession apply their conditions the way the SQL does.
type fakeSessions struct {
	rows        map[uuid.UUID]*fakeSession
	beforeWrite func() // runs before each conditional write: a transaction that commits first
	afterWrite  func() // runs after each conditional write
	reads       int
	outsideTx   []string
}

func (f *fakeSessions) SessionForRefresh(_ context.Context, id uuid.UUID) (app.RefreshSession, error) {
	f.reads++
	s, ok := f.rows[id]
	if !ok {
		return app.RefreshSession{}, app.ErrNotFound
	}
	return app.RefreshSession{UserID: s.UserID, State: s.State}, nil
}

// at reports whether the session is still at g: the WHERE of rotation and
// logout.
func (f *fakeSessions) at(g app.SessionGeneration) (*fakeSession, bool) {
	if f.beforeWrite != nil {
		f.beforeWrite()
	}
	if f.afterWrite != nil {
		defer f.afterWrite()
	}
	s, ok := f.rows[g.ID]
	return s, ok && s.State.Generation == g.Generation && bytes.Equal(s.State.TokenHash, g.TokenHash) &&
		!s.State.Revoked && g.Now.Before(s.State.ExpiresAt)
}

func (f *fakeSessions) RotateSession(ctx context.Context, g app.SessionGeneration, newHash []byte) (bool, error) {
	if !inTx(ctx) {
		f.outsideTx = append(f.outsideTx, "rotate")
	}
	s, ok := f.at(g)
	if ok {
		s.State.Generation++
		s.State.TokenHash = newHash
		s.changedAt = g.Now
	}
	return ok, nil
}

func (f *fakeSessions) RevokeForReuse(ctx context.Context, id uuid.UUID, now time.Time) error {
	if !inTx(ctx) {
		f.outsideTx = append(f.outsideTx, "revoke")
	}
	if s, ok := f.rows[id]; ok && !s.State.Revoked {
		s.State.Revoked, s.reason, s.changedAt = true, "reuse_detected", now
	}
	return nil
}

func (f *fakeSessions) EndSession(_ context.Context, g app.SessionGeneration) (bool, error) {
	s, ok := f.at(g)
	if ok {
		s.State.Revoked, s.reason, s.changedAt = true, "logout", g.Now
	}
	return ok, nil
}
