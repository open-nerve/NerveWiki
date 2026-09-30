package postgresadapter_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

func TestFindLoginAccount(t *testing.T) {
	s, _ := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)

	got, err := s.FindLoginAccount(context.Background(), "alice@corp.com")

	if err != nil || got != (app.LoginAccount{ID: u.ID, PasswordHash: u.PasswordHash}) {
		t.Errorf("FindLoginAccount() = %+v, %v; want the account's id and hash", got, err)
	}
	if _, err := s.FindLoginAccount(context.Background(), "bob@corp.com"); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("FindLoginAccount(unknown) = %v, want app.ErrNotFound", err)
	}
}

// The account row lock reads the hash and whether the account is active,
// and holds the row until the transaction ends: a second lock waits for it.
func TestLockForCredentials(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	exec(t, pool, "UPDATE users SET is_active = false")
	tx := postgres.NewTxManager(pool, time.Second)
	holding, release := make(chan struct{}), make(chan struct{})
	locked := make(chan error, 1)
	go func() {
		locked <- tx.WithinTx(ctx, func(ctx context.Context) error {
			got, err := s.LockForCredentials(ctx, u.ID)
			if err == nil && (got.Email != u.Email || got.PasswordHash != u.PasswordHash || got.Active) {
				err = errors.New("the lock read the wrong row")
			}
			close(holding)
			select { // a failed test never releases: give up rather than hold the pool's close
			case <-release:
			case <-time.After(10 * time.Second):
			}
			return err
		})
	}()
	<-holding

	second := make(chan error, 1)
	go func() {
		second <- tx.WithinTx(ctx, func(ctx context.Context) error {
			_, err := s.LockForCredentials(ctx, u.ID)
			return err
		})
	}()
	pgtest.WaitForLockWaits(t, pool, 1, 10*time.Second)
	close(release)

	if err := <-locked; err != nil {
		t.Errorf("the first lock: %v", err)
	}
	if err := <-second; err != nil {
		t.Errorf("the second lock, after the first: %v", err)
	}
	err := tx.WithinTx(ctx, func(ctx context.Context) error {
		_, err := s.LockForCredentials(ctx, uuid.NewV7())
		return err
	})
	if !errors.Is(err, app.ErrNotFound) {
		t.Errorf("LockForCredentials(unknown) = %v, want app.ErrNotFound", err)
	}
}

func TestUpdatePasswordHash(t *testing.T) {
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)

	if err := s.UpdatePasswordHash(context.Background(), u.ID, "$argon2id$new", later()); err != nil {
		t.Fatal(err)
	}

	var hash string
	var updated time.Time
	if err := pool.QueryRow(context.Background(), "SELECT password, updated_at FROM users").Scan(&hash, &updated); err != nil {
		t.Fatal(err)
	}
	if hash != "$argon2id$new" || !updated.Equal(later()) {
		t.Errorf("row = %q updated at %v, want the new hash at %v", hash, updated, later())
	}
}
