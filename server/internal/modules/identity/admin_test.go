package identity_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock/clocktest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// newAdmin builds the administrator's commands from pool alone, as the
// command line does, with the deactivation's registrants given.
func newAdmin(pool *pgxpool.Pool, vetoers []identity.DeactivationVetoer, subscribers ...identity.DeactivationSubscriber) *identity.Admin {
	return identity.NewAdmin(identity.AdminDeps{
		Pool: pool, Tx: postgres.NewTxManager(pool, 2*time.Second), Clock: clocktest.At(testStart()),
		Logger: slog.New(slog.DiscardHandler), Password: testPassword(),
		DeactivationVetoers: vetoers, DeactivationSubscribers: subscribers,
	})
}

// codeOf is the problem code of err, "" when it has none.
func codeOf(err error) string {
	var se *shared.Error
	if errors.As(err, &se) {
		return se.Code
	}
	return ""
}

// An account the administrator creates signs in; its address cannot be
// created twice.
func TestAdminCreatesAnAccountThatSignsIn(t *testing.T) {
	h, pool := newServer(t)
	admin := newAdmin(pool, nil)

	created, err := admin.CreateUser(context.Background(), " Carol@Corp.com ", "correct horse battery")
	if err != nil || created.Email != "carol@corp.com" {
		t.Fatalf("CreateUser() = %+v, %v; want carol", created, err)
	}
	if rec, _ := login(h, "carol@corp.com", "correct horse battery"); rec.Code != http.StatusOK {
		t.Errorf("sign-in = %d %s, want 200", rec.Code, rec.Body)
	}
	if _, err := admin.CreateUser(context.Background(), "carol@corp.com", "correct horse battery"); codeOf(err) != "identity.email_taken" {
		t.Errorf("CreateUser() again = %v, want identity.email_taken", err)
	}
}

// A reset ends every session and personal access token of the account, and
// only of it: the old password no longer signs in, the new one does.
func TestAdminResetsAPassword(t *testing.T) {
	h, pool := newServer(t)
	_, first := register(h, "alice@corp.com")
	_, second := login(h, "alice@corp.com", "correct horse battery")
	token := createToken(t, h, first.AccessToken, "CI")
	_, bob := register(h, "bob@corp.com")

	result, err := newAdmin(pool, nil).ResetPassword(context.Background(), "alice@corp.com", "N3w-Passw0rd!")

	if err != nil || result != (identity.PasswordReset{Email: "alice@corp.com", Sessions: 2, APITokens: 1}) {
		t.Fatalf("ResetPassword() = %+v, %v; want 2 sessions and 1 token revoked", result, err)
	}
	for _, credential := range []string{first.AccessToken, second.AccessToken, token.Token} {
		if code := getMe(h, credential).Code; code != http.StatusUnauthorized {
			t.Errorf("a credential of before the reset = %d, want 401", code)
		}
	}
	if code := getMe(h, bob.AccessToken).Code; code != http.StatusOK {
		t.Errorf("another account's session = %d, want 200", code)
	}
	old, _ := login(h, "alice@corp.com", "correct horse battery")
	fresh, _ := login(h, "alice@corp.com", "N3w-Passw0rd!")
	if old.Code != http.StatusUnauthorized || fresh.Code != http.StatusOK {
		t.Errorf("sign-in with the old password = %d, the new = %d; want 401 and 200", old.Code, fresh.Code)
	}
	if reasons := sessionReasons(t, pool); !equalCounts(reasons, map[string]int{"password_reset": 2, "": 2}) {
		t.Errorf("sessions %q, want alice's two revoked for password_reset, bob's and the new one live", reasons)
	}
}

// equalCounts reports whether values hold each key of want that many times,
// and nothing else.
func equalCounts(values []string, want map[string]int) bool {
	got := map[string]int{}
	for _, v := range values {
		got[v]++
	}
	if len(got) != len(want) {
		return false
	}
	for k, n := range want {
		if got[k] != n {
			return false
		}
	}
	return true
}

// A new address ends every session; the tokens keep working; the account
// signs in with the new address only.
func TestAdminChangesAnAddress(t *testing.T) {
	h, pool := newServer(t)
	_, session := register(h, "alice@corp.com")
	token := createToken(t, h, session.AccessToken, "CI")
	register(h, "bob@corp.com")
	admin := newAdmin(pool, nil)

	result, err := admin.SetEmail(context.Background(), "alice@corp.com", "Alice@Example.org")

	if err != nil || result != (identity.EmailChange{Email: "alice@example.org", Sessions: 1}) {
		t.Fatalf("SetEmail() = %+v, %v; want the new address, 1 session revoked", result, err)
	}
	if getMe(h, session.AccessToken).Code != http.StatusUnauthorized || getMe(h, token.Token).Code != http.StatusOK {
		t.Error("after the change, want the session ended and the token working")
	}
	old, _ := login(h, "alice@corp.com", "correct horse battery")
	fresh, _ := login(h, "alice@example.org", "correct horse battery")
	if old.Code != http.StatusUnauthorized || fresh.Code != http.StatusOK {
		t.Errorf("sign-in with the old address = %d, the new = %d; want 401 and 200", old.Code, fresh.Code)
	}
	_, same := admin.SetEmail(context.Background(), "alice@example.org", "alice@example.org")
	_, taken := admin.SetEmail(context.Background(), "alice@example.org", "bob@corp.com")
	_, unknown := admin.SetEmail(context.Background(), "nobody@corp.com", "someone@corp.com")
	if codeOf(same) != "identity.email_unchanged" || codeOf(taken) != "identity.email_taken" || codeOf(unknown) != "identity.account_not_found" {
		t.Errorf("same = %v, taken = %v, unknown = %v; want email_unchanged, email_taken, account_not_found", same, taken, unknown)
	}
}

// The administrator's deactivation goes through the registrants as the
// caller's does: a vetoer refuses it, a subscriber follows it once. An
// activation brings the tokens back; the sessions stay ended, and the
// account signs in again.
func TestAdminDeactivatesAndActivates(t *testing.T) {
	pool := newPool(t)
	createMemberships(t, pool)
	var seen []identity.Deactivation
	var active []bool
	subscriber := endingSubscriber{pool: pool, seen: &seen, active: &active}
	h := serverWithRegistrants(t, pool, subscriber)
	_, session := register(h, "alice@corp.com")
	token := createToken(t, h, session.AccessToken, "CI")
	register(h, "bob@corp.com")
	execSQL(t, pool, `INSERT INTO memberships SELECT id FROM users WHERE email = 'bob@corp.com'`)
	admin := newAdmin(pool, []identity.DeactivationVetoer{membershipVetoer{pool}}, subscriber)
	ctx := context.Background()

	if _, err := admin.Deactivate(ctx, "bob@corp.com"); codeOf(err) != "test.has_memberships" {
		t.Errorf("Deactivate(bob) = %v, want the vetoer's refusal", err)
	}
	deactivated, err := admin.Deactivate(ctx, "alice@corp.com")
	if err != nil || deactivated != (identity.Deactivated{Email: "alice@corp.com", Sessions: 1}) || len(seen) != 1 {
		t.Fatalf("Deactivate(alice) = %+v, %v with %d subscriber calls; want 1 session revoked, the subscriber once", deactivated, err, len(seen))
	}
	again, err := admin.Deactivate(ctx, "alice@corp.com")
	if err != nil || !again.Already || len(seen) != 1 {
		t.Errorf("Deactivate(alice) again = %+v, %v with %d subscriber calls; want already, the subscriber not called again", again, err, len(seen))
	}
	if code := getMe(h, token.Token).Code; code != http.StatusUnauthorized {
		t.Errorf("the token while inactive = %d, want 401", code)
	}

	activated, err := admin.Activate(ctx, "alice@corp.com")
	if err != nil || activated != (identity.Activated{Email: "alice@corp.com", APITokens: 1}) {
		t.Fatalf("Activate() = %+v, %v; want 1 token usable again", activated, err)
	}
	signIn, _ := login(h, "alice@corp.com", "correct horse battery")
	if getMe(h, token.Token).Code != http.StatusOK || getMe(h, session.AccessToken).Code != http.StatusUnauthorized || signIn.Code != http.StatusOK {
		t.Error("after the activation, want the token working, the old session ended, the sign-in working")
	}
	if again, err := admin.Activate(ctx, "alice@corp.com"); err != nil || !again.Already {
		t.Errorf("Activate() again = %+v, %v; want already", again, err)
	}
	if _, err := admin.Activate(ctx, "nobody@corp.com"); codeOf(err) != "identity.account_not_found" {
		t.Errorf("Activate(unknown) = %v, want identity.account_not_found", err)
	}
}

// The administrator's commands lock the account row first, as the
// self-service ones do (M1 design 8): a transaction that shares the
// account makes the deactivation wait, and its vetoers then see what that
// transaction did. Without the lock the vetoers would run while the join
// is still open, find no membership, and the deactivation would go through
// once the join commits: the join's FOR SHARE would hold only its UPDATE.
func TestAnAdminDeactivationWaitsForATransactionThatSharesTheAccount(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	createMemberships(t, pool)
	admin := newAdmin(pool, []identity.DeactivationVetoer{membershipVetoer{pool}})
	if _, err := admin.CreateUser(ctx, "alice@corp.com", "correct horse battery"); err != nil {
		t.Fatal(err)
	}
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
	refused := async(func() error {
		_, err := admin.Deactivate(ctx, "alice@corp.com")
		return err
	})
	pgtest.WaitForLockWaits(t, pool, 1, waitLimit)
	g.open()
	joinErr := await(t, join)
	err := await(t, refused)

	if joinErr != nil || codeOf(err) != "test.has_memberships" || !isActive(t, pool) {
		t.Errorf("join %v; Deactivate() = %v, active %v; want the join, then the vetoer's refusal, the account active",
			joinErr, err, isActive(t, pool))
	}
}
