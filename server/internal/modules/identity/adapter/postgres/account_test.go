package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

func updatedAt(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) time.Time {
	t.Helper()
	var at time.Time
	if err := pool.QueryRow(context.Background(), `SELECT updated_at FROM users WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func TestUpdateDisplayName(t *testing.T) {
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)

	got, err := s.UpdateDisplayName(context.Background(), u.ID, "Alice Stone", later())

	if err != nil || got.ID != u.ID || got.Email != u.Email || got.DisplayName != "Alice Stone" || !updatedAt(t, pool, u.ID).Equal(later()) {
		t.Errorf("UpdateDisplayName() = %+v, %v; want Alice Stone, updated at %v", got, err, later())
	}
	if _, err := s.UpdateDisplayName(context.Background(), uuid.NewV7(), "x", later()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("UpdateDisplayName(unknown) = %v, want app.ErrNotFound", err)
	}
}

// A step is appended once, in the order recorded; recording it again
// leaves the row, updated_at included, as it was.
func TestRecordOnboardingStep(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)

	if _, err := s.RecordOnboardingStep(ctx, u.ID, "profile", later()); err != nil {
		t.Fatal(err)
	}
	got, err := s.RecordOnboardingStep(ctx, u.ID, "workspace", later())
	again, againErr := s.RecordOnboardingStep(ctx, u.ID, "profile", later().Add(time.Hour))

	if err != nil || againErr != nil || !slices.Equal(got.OnboardingSteps, []string{"profile", "workspace"}) ||
		!slices.Equal(again.OnboardingSteps, got.OnboardingSteps) || !updatedAt(t, pool, u.ID).Equal(later()) {
		t.Errorf("steps = %q, then %q (%v, %v), updated at %v; want profile and workspace once, updated at %v",
			got.OnboardingSteps, again.OnboardingSteps, err, againErr, updatedAt(t, pool, u.ID), later())
	}
	if _, err := s.RecordOnboardingStep(ctx, uuid.NewV7(), "profile", later()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("RecordOnboardingStep(unknown) = %v, want app.ErrNotFound", err)
	}
}

// The 33rd step breaks the CHECK: the store answers the domain's 422, and
// a step recorded already still answers.
func TestRecordOnboardingStepBeyondTheBound(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	for i := range domain.MaxOnboardingSteps {
		if _, err := s.RecordOnboardingStep(ctx, u.ID, fmt.Sprintf("s%d", i), later()); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}

	_, err := s.RecordOnboardingStep(ctx, u.ID, "one_more", later())
	got, known := s.RecordOnboardingStep(ctx, u.ID, "s0", later())

	if !errors.Is(err, domain.ErrTooManyOnboardingSteps) || known != nil || len(got.OnboardingSteps) != domain.MaxOnboardingSteps {
		t.Errorf("the 33rd step = %v, a known one = %v with %d steps; want ErrTooManyOnboardingSteps, then 32", err, known, len(got.OnboardingSteps))
	}
}

func TestDeactivateUser(t *testing.T) {
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)

	err := s.DeactivateUser(context.Background(), u.ID, later())

	var active bool
	if qerr := pool.QueryRow(context.Background(), `SELECT is_active FROM users`).Scan(&active); qerr != nil {
		t.Fatal(qerr)
	}
	if err != nil || active || !updatedAt(t, pool, u.ID).Equal(later()) {
		t.Errorf("DeactivateUser() = %v; active %v; want inactive, updated at %v", err, active, later())
	}
}

// ShareAccount reads is_active and the address under FOR SHARE: two of
// them hold the row together; the account row lock waits for them.
func TestShareAccount(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	tx := postgres.NewTxManager(pool, time.Second)
	holding, release := make(chan struct{}), make(chan struct{})
	first := make(chan error, 1)
	go func() {
		first <- tx.WithinTx(ctx, func(ctx context.Context) error {
			got, err := s.ShareAccount(ctx, u.ID)
			if err != nil {
				return err
			}
			if got != (app.SharedAccount{ID: u.ID, Active: true, Email: "alice@corp.com"}) {
				return fmt.Errorf("ShareAccount() = %+v, want active, alice@corp.com", got)
			}
			close(holding)
			select { // a failed test never releases: give up rather than hold the pool's close
			case <-release:
			case <-time.After(10 * time.Second):
			}
			return nil
		})
	}()
	awaitHolding(t, holding, first)

	second := tx.WithinTx(ctx, func(ctx context.Context) error {
		_, err := s.ShareAccount(ctx, u.ID)
		return err
	})
	locked := make(chan error, 1)
	go func() {
		locked <- tx.WithinTx(ctx, func(ctx context.Context) error {
			_, err := s.LockForCredentials(ctx, u.ID)
			return err
		})
	}()
	pgtest.WaitForLockWaits(t, pool, 1, 10*time.Second)
	close(release)

	if err := errors.Join(second, <-first, <-locked); err != nil {
		t.Errorf("a second share while the first held: %v; want it through, then the lock after both", err)
	}
	err := tx.WithinTx(ctx, func(ctx context.Context) error {
		_, err := s.ShareAccount(ctx, uuid.NewV7())
		return err
	})
	if !errors.Is(err, app.ErrNotFound) {
		t.Errorf("ShareAccount(unknown) = %v, want app.ErrNotFound", err)
	}
}

// A deactivated account's share reads it inactive, with its address.
func TestShareAccountOfADeactivatedAccount(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	if err := s.DeactivateUser(ctx, u.ID, later()); err != nil {
		t.Fatal(err)
	}
	var got app.SharedAccount
	err := postgres.NewTxManager(pool, time.Second).WithinTx(ctx, func(ctx context.Context) error {
		var err error
		got, err = s.ShareAccount(ctx, u.ID)
		return err
	})
	if err != nil || got != (app.SharedAccount{ID: u.ID, Active: false, Email: "alice@corp.com"}) {
		t.Errorf("ShareAccount() = %+v, %v; want inactive, alice@corp.com", got, err)
	}
}

// ShareAccount outside a transaction is a fault (M1 handoff to M2, item
// 3): its FOR SHARE would end with the statement, and the access it guards
// would no longer wait for a deactivation.
func TestShareAccountRefusesToRunOutsideATransaction(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	if _, err := s.ShareAccount(ctx, u.ID); err == nil || errors.Is(err, app.ErrNotFound) {
		t.Errorf("ShareAccount() outside a transaction = %v, want a fault", err)
	}
}

// ShareAccountByEmail is ShareAccount by the address (M2/P4 design 3.3): it
// holds the row FOR SHARE, which the account row lock waits for and another
// share does not. An address changed while it waited names no account: the
// condition is checked again on the row the change committed.
func TestShareAccountByEmail(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	tx := postgres.NewTxManager(pool, time.Second)
	nowait := func(lock string) error {
		other, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = other.Rollback(ctx) }()
		_, err = other.Exec(ctx, "SELECT 1 FROM users WHERE id = $1 FOR "+lock+" NOWAIT", u.ID)
		return err
	}

	err := tx.WithinTx(ctx, func(ctx context.Context) error {
		got, err := s.ShareAccountByEmail(ctx, "alice@corp.com")
		if err != nil {
			return err
		}
		if got != (app.SharedAccount{ID: u.ID, Active: true, Email: "alice@corp.com"}) {
			t.Errorf("ShareAccountByEmail() = %+v, want alice's, active", got)
		}
		if nowait("SHARE") != nil || nowait("NO KEY UPDATE") == nil {
			t.Error("want the row held FOR SHARE: another share through, the account row lock held back")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ShareAccountByEmail(ctx, "alice@corp.com"); err == nil || errors.Is(err, app.ErrNotFound) {
		t.Errorf("ShareAccountByEmail() outside a transaction = %v, want a fault", err)
	}

	changer, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = changer.Rollback(ctx) }()
	if _, err := changer.Exec(ctx, "UPDATE users SET email = 'alice@example.com' WHERE id = $1", u.ID); err != nil {
		t.Fatal(err)
	}
	shared := make(chan error, 1)
	go func() {
		shared <- tx.WithinTx(ctx, func(ctx context.Context) error {
			_, err := s.ShareAccountByEmail(ctx, "alice@corp.com")
			return err
		})
	}()
	pgtest.WaitForLockWaits(t, pool, 1, 10*time.Second)
	if err := changer.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-shared; !errors.Is(err, app.ErrNotFound) {
		t.Errorf("ShareAccountByEmail() of the address changed meanwhile = %v, want app.ErrNotFound", err)
	}
}
