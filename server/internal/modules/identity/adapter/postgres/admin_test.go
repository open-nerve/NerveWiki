package postgresadapter_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The administrator's lock finds the account by its normalized address and
// reads it under the lock; no account has an address it does not know.
func TestLockAccountByEmail(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	exec(t, pool, "UPDATE users SET is_active = false")

	var got app.LockedAccount
	err := postgres.NewTxManager(pool, time.Second).WithinTx(ctx, func(ctx context.Context) error {
		var err error
		got, err = s.LockAccountByEmail(ctx, "alice@corp.com")
		return err
	})
	if err != nil || got != (app.LockedAccount{ID: u.ID, Email: u.Email, PasswordHash: u.PasswordHash, Active: false}) {
		t.Errorf("LockAccountByEmail() = %+v, %v; want alice's row, inactive", got, err)
	}
	if _, err := s.LockAccountByEmail(ctx, "bob@corp.com"); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("LockAccountByEmail(unknown) = %v, want app.ErrNotFound", err)
	}
}

// A new address replaces the old; one another account has is
// identity.email_taken, and changes nothing.
func TestChangeEmail(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u, other := newUser("alice@corp.com"), newUser("bob@corp.com")
	mustCreate(t, s, u)
	mustCreate(t, s, other)

	if err := s.ChangeEmail(ctx, u.ID, "alice@example.org", later()); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangeEmail(ctx, u.ID, "bob@corp.com", later().Add(time.Minute)); !errors.Is(err, domain.ErrEmailTaken) {
		t.Errorf("ChangeEmail(taken) = %v, want identity.email_taken", err)
	}
	var email string
	if err := pool.QueryRow(ctx, "SELECT email FROM users WHERE id = $1", u.ID).Scan(&email); err != nil {
		t.Fatal(err)
	}
	if email != "alice@example.org" || !updatedAt(t, pool, u.ID).Equal(later()) {
		t.Errorf("address %q updated at %v, want alice@example.org at %v", email, updatedAt(t, pool, u.ID), later())
	}
}

func TestActivateUser(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	if err := s.DeactivateUser(ctx, u.ID, now()); err != nil {
		t.Fatal(err)
	}

	err := s.ActivateUser(ctx, u.ID, later())

	var active bool
	if err := pool.QueryRow(ctx, "SELECT is_active FROM users WHERE id = $1", u.ID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if err != nil || !active || !updatedAt(t, pool, u.ID).Equal(later()) {
		t.Errorf("ActivateUser() = %v; active %v; want active, updated at %v", err, active, later())
	}
}

// Revoking every token of an account hits the live and the expired ones,
// leaves a revoked one as it was and another account's alone; the usable
// tokens are those neither revoked nor expired.
func TestRevokeAllAndCountUsableAPITokens(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u, other := newUser("alice@corp.com"), newUser("bob@corp.com")
	mustCreate(t, s, u)
	mustCreate(t, s, other)
	live, _ := newToken(t, s, u.ID, "live")
	expired, _ := newToken(t, s, u.ID, "expired")
	revoked, _ := newToken(t, s, u.ID, "revoked")
	others, _ := newToken(t, s, other.ID, "other")
	exec(t, pool, "UPDATE api_tokens SET expires_at = $1 WHERE id = $2", later(), expired.ID)
	exec(t, pool, "UPDATE api_tokens SET revoked_at = $1 WHERE id = $2", later(), revoked.ID)
	at := later().Add(time.Hour) // expired has expired

	usable, err := s.CountUsableAPITokens(ctx, u.ID, at)
	if err != nil || usable != 1 {
		t.Errorf("CountUsableAPITokens() = %d, %v; want 1, the live one", usable, err)
	}
	n, err := s.RevokeAllAPITokens(ctx, u.ID, at)
	if err != nil || n != 2 {
		t.Fatalf("RevokeAllAPITokens() = %d, %v; want 2, the live and the expired one", n, err)
	}
	for _, tt := range []struct {
		name string
		row  tokenRow
		want time.Time
	}{
		{"live", readToken(t, pool, live.ID), at},
		{"expired", readToken(t, pool, expired.ID), at},
		{"revoked", readToken(t, pool, revoked.ID), later()},
	} {
		if tt.row.revoked == nil || !tt.row.revoked.Equal(tt.want) {
			t.Errorf("%s: revoked at %v, want %v", tt.name, tt.row.revoked, tt.want)
		}
	}
	if r := readToken(t, pool, others.ID); r.revoked != nil {
		t.Errorf("another account's token revoked at %v, want live", r.revoked)
	}
	if usable, err := s.CountUsableAPITokens(ctx, other.ID, at); err != nil || usable != 1 {
		t.Errorf("CountUsableAPITokens(other) = %d, %v; want 1", usable, err)
	}
}
