package postgresadapter_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"net/netip"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// now is in whole microseconds, as timestamptz stores them, so the audit
// columns read back equal to it.
func now() time.Time { return time.Date(2026, 9, 25, 10, 0, 0, 123456000, time.UTC) }

func newStore(t *testing.T) (*postgresadapter.Store, *pgxpool.Pool) {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return postgresadapter.New(pool), pool
}

func newUser(email string) app.NewUser {
	return app.NewUser{ID: uuid.NewV7(), Email: email, PasswordHash: "$argon2id$v=19$m=64,t=1,p=1$c2FsdA$a2V5", DisplayName: "alice", Now: now()}
}

func mustCreate(t *testing.T, s *postgresadapter.Store, u app.NewUser) {
	t.Helper()
	if err := s.CreateUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAndGetUser(t *testing.T) {
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)

	got, err := s.GetUser(context.Background(), u.ID)

	if err != nil || got.ID != u.ID || got.Email != u.Email || got.DisplayName != "alice" || len(got.OnboardingSteps) != 0 {
		t.Errorf("GetUser() = %+v, %v; want the account, with no onboarding step done", got, err)
	}
	var password string
	var active bool
	var created, updated time.Time
	if err := pool.QueryRow(context.Background(), "SELECT password, is_active, created_at, updated_at FROM users WHERE id = $1", u.ID).
		Scan(&password, &active, &created, &updated); err != nil {
		t.Fatal(err)
	}
	if password != u.PasswordHash || !active || !created.Equal(now()) || !updated.Equal(now()) {
		t.Errorf("row = %q active=%v created %v updated %v; want the hash, active, and both audit columns at %v", password, active, created, updated, now())
	}
}

func TestGetUserReadsTheOnboardingSteps(t *testing.T) {
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	if _, err := pool.Exec(context.Background(), "UPDATE users SET onboarding_steps = '{profile,workspace}' WHERE id = $1", u.ID); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetUser(context.Background(), u.ID)

	if err != nil || !slices.Equal(got.OnboardingSteps, []string{"profile", "workspace"}) {
		t.Errorf("GetUser() = %+v, %v; want the steps in their order", got, err)
	}
}

func TestGetUnknownUser(t *testing.T) {
	s, _ := newStore(t)
	if _, err := s.GetUser(context.Background(), uuid.NewV7()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("GetUser() = %v, want app.ErrNotFound", err)
	}
}

// The unique constraint turns a race between two registrations into 409.
func TestCreateUserWithATakenAddress(t *testing.T) {
	s, pool := newStore(t)
	mustCreate(t, s, newUser("alice@corp.com"))
	tx := postgres.NewTxManager(pool, 2*time.Second)

	err := tx.WithinTx(context.Background(), func(ctx context.Context) error {
		return s.CreateUser(ctx, newUser("alice@corp.com"))
	})

	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Errorf("CreateUser() = %v, want identity.email_taken", err)
	}
}

// The domain validates every value first, so a CHECK violation is a bug
// that got past it: an internal error, never a domain error.
func TestCreateUserBreakingACheckIsInternal(t *testing.T) {
	s, _ := newStore(t)

	err := s.CreateUser(context.Background(), newUser("Alice@corp.com"))

	var se *shared.Error
	var pgErr *pgconn.PgError
	if errors.As(err, &se) || !errors.As(err, &pgErr) || pgErr.ConstraintName != "users_email_check" {
		t.Errorf("CreateUser() = %v, want the check_violation of users_email_check, not a domain error", err)
	}
}

func newSession(userID uuid.UUID, ip netip.Addr) app.NewSession {
	hash := sha256.Sum256([]byte("secret"))
	return app.NewSession{
		ID: uuid.NewV7(), UserID: userID, TokenHash: hash[:], UserAgent: "Firefox",
		IP: ip, ExpiresAt: now().Add(720 * time.Hour), Now: now(),
	}
}

func TestCreateSession(t *testing.T) {
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	for _, tt := range []struct {
		name string
		ip   netip.Addr
		want string
	}{
		{"IPv4", netip.MustParseAddr("203.0.113.7"), "203.0.113.7"},
		{"IPv6", netip.MustParseAddr("2001:db8::1"), "2001:db8::1"},
		{"unknown", netip.Addr{}, "NULL"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			n := newSession(u.ID, tt.ip)

			if err := s.CreateSession(context.Background(), n); err != nil {
				t.Fatal(err)
			}

			var userID uuid.UUID
			var hash []byte
			var generation int
			var agent, ip string
			var expires, created, updated time.Time
			err := pool.QueryRow(context.Background(), `SELECT user_id, token_hash, generation, user_agent, coalesce(host(ip), 'NULL'),
				expires_at, created_at, updated_at FROM auth_sessions WHERE id = $1`, n.ID).
				Scan(&userID, &hash, &generation, &agent, &ip, &expires, &created, &updated)
			if err != nil {
				t.Fatal(err)
			}
			if userID != u.ID || !bytes.Equal(hash, n.TokenHash) || generation != 0 || agent != "Firefox" || ip != tt.want ||
				!expires.Equal(n.ExpiresAt) || !created.Equal(now()) || !updated.Equal(now()) {
				t.Errorf("session = %v %x gen %d %q %s %v %v %v", userID, hash, generation, agent, ip, expires, created, updated)
			}
		})
	}
}

func TestSessionCredential(t *testing.T) {
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	n := newSession(u.ID, netip.Addr{})
	if err := s.CreateSession(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		change string
		want   app.SessionCredential
	}{
		{"usable", "", app.SessionCredential{UserID: u.ID, ExpiresAt: n.ExpiresAt, UserActive: true}},
		{"account deactivated", "UPDATE users SET is_active = false",
			app.SessionCredential{UserID: u.ID, ExpiresAt: n.ExpiresAt}},
		{"revoked", "UPDATE auth_sessions SET revoked_at = now(), revoke_reason = 'logout'",
			app.SessionCredential{UserID: u.ID, ExpiresAt: n.ExpiresAt, Revoked: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.change != "" {
				if _, err := pool.Exec(context.Background(), tt.change); err != nil {
					t.Fatal(err)
				}
			}

			got, err := s.SessionCredential(context.Background(), n.ID)

			if err != nil || got.UserID != tt.want.UserID || !got.ExpiresAt.Equal(tt.want.ExpiresAt) ||
				got.Revoked != tt.want.Revoked || got.UserActive != tt.want.UserActive {
				t.Errorf("SessionCredential() = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}

func TestUnknownSessionCredential(t *testing.T) {
	s, _ := newStore(t)
	if _, err := s.SessionCredential(context.Background(), uuid.NewV7()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("SessionCredential() = %v, want app.ErrNotFound", err)
	}
}

// awaitHolding waits until a transaction running in the background holds
// its lock and closes holding; it fails the test when the transaction ends
// first, with done, or takes too long.
func awaitHolding(t *testing.T, holding <-chan struct{}, done <-chan error) {
	t.Helper()
	select {
	case <-holding:
	case err := <-done:
		t.Fatalf("the transaction ended before it held the lock: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("the transaction did not hold the lock within 10s")
	}
}
