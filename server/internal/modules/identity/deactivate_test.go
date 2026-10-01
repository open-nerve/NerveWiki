package identity_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock/clocktest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Deactivation and its extension point on a real database (M1/P3 design
// 3.6). The registrants here stand in for M2's: built from the pool alone,
// their statements reach the deactivation's transaction through the
// context.

// errHasMemberships is the code a registrant refuses with.
func errHasMemberships() *shared.Error {
	return shared.NewError(shared.KindConflict, "test.has_memberships", "The account has memberships.")
}

// createMemberships creates a registrant's tables: memberships, the test's
// stand-in for M2's workspace members, and ended, where the subscriber
// records the accounts whose memberships it ended.
func createMemberships(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	execSQL(t, pool, `CREATE TABLE memberships (user_id uuid NOT NULL)`)
	execSQL(t, pool, `CREATE TABLE ended (user_id uuid NOT NULL)`)
}

// membershipVetoer refuses the deactivation of an account that has a
// membership, as it reads under the account row lock.
type membershipVetoer struct{ pool *pgxpool.Pool }

func (v membershipVetoer) VetoDeactivation(ctx context.Context, d identity.Deactivation) error {
	var n int
	if err := postgres.DB(ctx, v.pool).QueryRow(ctx, `SELECT count(*) FROM memberships WHERE user_id = $1`, d.UserID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return errHasMemberships()
	}
	return nil
}

// endingSubscriber ends the account's memberships when it is deactivated,
// writing a row of ended, and records what it saw of the account in the
// deactivation's transaction.
type endingSubscriber struct {
	pool   *pgxpool.Pool
	seen   *[]identity.Deactivation
	active *[]bool
	fail   *bool // fail after the writes while true; nil never fails
}

func (s endingSubscriber) AccountDeactivated(ctx context.Context, d identity.Deactivation) error {
	db := postgres.DB(ctx, s.pool)
	var active bool
	if err := db.QueryRow(ctx, `SELECT is_active FROM users WHERE id = $1`, d.UserID).Scan(&active); err != nil {
		return err
	}
	*s.seen, *s.active = append(*s.seen, d), append(*s.active, active)
	if _, err := db.Exec(ctx, `DELETE FROM memberships WHERE user_id = $1`, d.UserID); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `INSERT INTO ended VALUES ($1)`, d.UserID); err != nil {
		return err
	}
	if s.fail != nil && *s.fail {
		return errors.New("the subscriber failed")
	}
	return nil
}

func deactivate(h http.Handler, credential string) *http.Response {
	return send(h, http.MethodPost, "/api/v0/me/deactivate", credential, "").Result()
}

func accountID(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT id FROM users`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func isActive(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()
	var active bool
	if err := pool.QueryRow(context.Background(), `SELECT is_active FROM users`).Scan(&active); err != nil {
		t.Fatal(err)
	}
	return active
}

// Deactivation ends every session, the caller's too, and stops the tokens:
// an access token, a refresh token and a personal access token all answer
// 401 afterwards, and sign-in answers 403 (M1/P3 design 3.6, P2 review N3).
func TestDeactivationStopsEveryCredential(t *testing.T) {
	for _, withToken := range []bool{false, true} {
		h, pool := newServer(t)
		_, first := register(h, "alice@corp.com")
		_, second := login(h, "alice@corp.com", "correct horse battery")
		pat := createToken(t, h, first.AccessToken, "agent")
		credential := second.AccessToken
		if withToken {
			credential = pat.Token
		}

		res := deactivate(h, credential)

		if res.StatusCode != http.StatusNoContent || isActive(t, pool) || !slices.Equal(sessionReasons(t, pool), []string{"deactivated", "deactivated"}) {
			t.Fatalf("deactivate with a token %v = %d, active %v, sessions %q; want 204, inactive, both revoked for deactivated",
				withToken, res.StatusCode, isActive(t, pool), sessionReasons(t, pool))
		}
		refreshed, _ := refresh(h, first.RefreshToken)
		signIn, _ := login(h, "alice@corp.com", "correct horse battery")
		if getMe(h, second.AccessToken).Code != http.StatusUnauthorized || refreshed.Code != http.StatusUnauthorized ||
			getMe(h, pat.Token).Code != http.StatusUnauthorized || signIn.Code != http.StatusForbidden {
			t.Errorf("after deactivation: access %d, refresh %d, token %d, sign-in %d; want 401, 401, 401, 403",
				getMe(h, second.AccessToken).Code, refreshed.Code, getMe(h, pat.Token).Code, signIn.Code)
		}
	}
}

// registrants serves the module with the membership vetoer and subscribers.
func serverWithRegistrants(t *testing.T, pool *pgxpool.Pool, subscribers ...identity.DeactivationSubscriber) http.Handler {
	t.Helper()
	return newServerWith(t, pool, clocktest.At(testStart()), func(d *identity.Deps) {
		d.DeactivationVetoers = []identity.DeactivationVetoer{membershipVetoer{pool}}
		d.DeactivationSubscribers = subscribers
	})
}

// A vetoer's *shared.Error is the answer, and nothing changes: the account
// stays active, its sessions live.
func TestAVetoerRefusesADeactivation(t *testing.T) {
	pool := newPool(t)
	createMemberships(t, pool)
	var seen []identity.Deactivation
	var active []bool
	h := serverWithRegistrants(t, pool, endingSubscriber{pool: pool, seen: &seen, active: &active})
	_, tokens := register(h, "alice@corp.com")
	execSQL(t, pool, `INSERT INTO memberships SELECT id FROM users`)

	rec := send(h, http.MethodPost, "/api/v0/me/deactivate", tokens.AccessToken, "")

	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"code":"test.has_memberships"`) {
		t.Errorf("deactivate = %d %s, want 409 test.has_memberships", rec.Code, rec.Body)
	}
	if !isActive(t, pool) || !slices.Equal(sessionReasons(t, pool), []string{""}) || len(seen) != 0 {
		t.Errorf("active %v, sessions %q, subscriber called %d times; want nothing changed", isActive(t, pool), sessionReasons(t, pool), len(seen))
	}
}

// The subscribers run after the writes, in the deactivation's transaction:
// they see the account inactive, with the address read under the lock and
// the deactivation's instant. One that fails rolls the whole deactivation
// back, its own writes too.
func TestSubscribersRunInTheTransaction(t *testing.T) {
	pool := newPool(t)
	createMemberships(t, pool)
	var seen []identity.Deactivation
	var active []bool
	fail := true
	h := serverWithRegistrants(t, pool,
		endingSubscriber{pool: pool, seen: &seen, active: &active},
		endingSubscriber{pool: pool, seen: &seen, active: &active, fail: &fail})
	_, tokens := register(h, "alice@corp.com")
	id := accountID(t, pool)

	failed := send(h, http.MethodPost, "/api/v0/me/deactivate", tokens.AccessToken, "")

	if failed.Code != http.StatusInternalServerError || !isActive(t, pool) || !slices.Equal(sessionReasons(t, pool), []string{""}) || ended(t, pool) != 0 {
		t.Errorf("with a failing subscriber: %d, active %v, sessions %q, %d ended; want 500 and nothing changed, the subscribers' writes neither",
			failed.Code, isActive(t, pool), sessionReasons(t, pool), ended(t, pool))
	}
	want := identity.Deactivation{UserID: id, Email: "alice@corp.com", At: testStart()}
	if len(seen) != 2 || seen[0] != want || !slices.Equal(active, []bool{false, false}) {
		t.Errorf("the subscribers saw %+v with the account active %v; want %+v twice, inactive", seen, active, want)
	}

	seen, active, fail = nil, nil, false
	if rec := send(h, http.MethodPost, "/api/v0/me/deactivate", tokens.AccessToken, ""); rec.Code != http.StatusNoContent || len(seen) != 2 ||
		isActive(t, pool) || ended(t, pool) != 2 {
		t.Errorf("with the subscribers succeeding: %d %s, called %d times, %d ended; want 204, both called and committed, the account inactive",
			rec.Code, rec.Body, len(seen), ended(t, pool))
	}
}

func ended(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM ended`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// A transaction that gives the account new access takes the shared lock
// first and holds it: a deactivation waits for it, and then its vetoer sees
// the membership that transaction committed (M1 design 8). The order is
// forced: the test commits only once the deactivation waits for the lock.
func TestADeactivationWaitsForATransactionThatSharesTheAccount(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	createMemberships(t, pool)
	h := serverWithRegistrants(t, pool)
	_, tokens := register(h, "alice@corp.com")
	id := accountID(t, pool)
	tx := postgres.NewTxManager(pool, 2*time.Second)
	g := newGate()

	join := async(func() error {
		return tx.WithinTx(ctx, func(ctx context.Context) error {
			if _, err := identity.NewAccounts(pool).ShareActiveAccount(ctx, id); err != nil {
				return err
			}
			if _, err := postgres.DB(ctx, pool).Exec(ctx, `INSERT INTO memberships VALUES ($1)`, id); err != nil {
				return err
			}
			return g.stop()
		})
	})
	g.await(t)
	refused := make(chan *http.Response, 1)
	go func() { refused <- deactivate(h, tokens.AccessToken) }()
	pgtest.WaitForLockWaits(t, pool, 1, waitLimit)
	g.open()
	joinErr := await(t, join)
	res := <-refused

	if joinErr != nil || res.StatusCode != http.StatusConflict || !isActive(t, pool) {
		t.Errorf("join %v; deactivate = %d, active %v; want the join, then 409, the account active", joinErr, res.StatusCode, isActive(t, pool))
	}
}

// The other way round: a deactivation that holds the account row lock
// makes a transaction that would share the account wait, and then refuse:
// the account is deactivated. The deactivation stops inside its
// transaction, in a subscriber.
func TestATransactionThatSharesTheAccountWaitsForADeactivation(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	createMemberships(t, pool)
	g := newGate()
	h := newServerWith(t, pool, clocktest.At(testStart()), func(d *identity.Deps) {
		d.DeactivationSubscribers = []identity.DeactivationSubscriber{gatedSubscriber{g}}
	})
	_, tokens := register(h, "alice@corp.com")
	id := accountID(t, pool)
	tx := postgres.NewTxManager(pool, 2*time.Second)

	done := make(chan *http.Response, 1)
	go func() { done <- deactivate(h, tokens.AccessToken) }()
	g.await(t)
	join := async(func() error {
		return tx.WithinTx(ctx, func(ctx context.Context) error {
			_, err := identity.NewAccounts(pool).ShareActiveAccount(ctx, id)
			return err
		})
	})
	pgtest.WaitForLockWaits(t, pool, 1, waitLimit)
	g.open()
	res := <-done
	joinErr := await(t, join)

	if res.StatusCode != http.StatusNoContent || !errors.Is(joinErr, domain.ErrAccountDeactivated) {
		t.Errorf("deactivate = %d, join = %v; want 204, then identity.account_deactivated", res.StatusCode, joinErr)
	}
	if err := tx.WithinTx(ctx, func(ctx context.Context) error {
		_, err := identity.NewAccounts(pool).ShareActiveAccount(ctx, uuid.NewV7())
		return err
	}); !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("ShareActiveAccount(unknown) = %v, want identity.account_not_found", err)
	}
}

// gatedSubscriber stops a deactivation inside its transaction.
type gatedSubscriber struct{ gate *gate }

func (s gatedSubscriber) AccountDeactivated(context.Context, identity.Deactivation) error {
	return s.gate.stop()
}
