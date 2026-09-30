package postgresadapter_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// newToken inserts a token of account userID, created at now, expiring a
// day later, named name.
func newToken(t *testing.T, s *postgresadapter.Store, userID uuid.UUID, name string) (app.NewAPIToken, domain.PAT) {
	t.Helper()
	var pat domain.PAT
	copy(pat[:], name)
	expires := now().Add(24 * time.Hour)
	n := app.NewAPIToken{ID: uuid.NewV7(), UserID: userID, TokenHash: pat.Hash(), Name: name, ExpiresAt: &expires, Now: now()}
	if err := s.CreateAPIToken(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	return n, pat
}

// tokenRow is what the tests read back of a token.
type tokenRow struct {
	name                   string
	expires, used, revoked *time.Time
	created, updated       time.Time
}

func readToken(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) tokenRow {
	t.Helper()
	var r tokenRow
	err := pool.QueryRow(context.Background(),
		`SELECT name, expires_at, last_used_at, revoked_at, created_at, updated_at FROM api_tokens WHERE id = $1`, id).
		Scan(&r.name, &r.expires, &r.used, &r.revoked, &r.created, &r.updated)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// A token reads back by its hash and by its id, with its account's state.
func TestCreateAndReadAPIToken(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	n, pat := newToken(t, s, u.ID, "CI")

	byHash, err := s.APITokenByHash(ctx, pat.Hash())
	want := app.APITokenCredential{ID: n.ID, UserID: u.ID, ExpiresAt: n.ExpiresAt, UserActive: true}
	if err != nil || !sameCredential(byHash, want) {
		t.Errorf("APITokenByHash() = %+v, %v; want %+v", byHash, err, want)
	}
	if byID, err := s.APITokenByID(ctx, n.ID); err != nil || !sameCredential(byID, want) {
		t.Errorf("APITokenByID() = %+v, %v; want %+v", byID, err, want)
	}
	if r := readToken(t, pool, n.ID); r.name != "CI" || !r.created.Equal(now()) || !r.updated.Equal(now()) || r.used != nil || r.revoked != nil {
		t.Errorf("row = %+v, want CI created and updated at %v, never used, live", r, now())
	}

	exec(t, pool, "UPDATE api_tokens SET revoked_at = $1, last_used_at = $1", later())
	exec(t, pool, "UPDATE users SET is_active = false")
	got, err := s.APITokenByID(ctx, n.ID)
	if err != nil || !got.Revoked || got.UserActive || got.LastUsedAt == nil || !got.LastUsedAt.Equal(later()) {
		t.Errorf("APITokenByID() after a change = %+v, %v; want it revoked, used at %v, the account inactive", got, err, later())
	}
}

func sameCredential(a, b app.APITokenCredential) bool {
	sameTime := func(x, y *time.Time) bool { return (x == nil) == (y == nil) && (x == nil || x.Equal(*y)) }
	return a.ID == b.ID && a.UserID == b.UserID && sameTime(a.ExpiresAt, b.ExpiresAt) && sameTime(a.LastUsedAt, b.LastUsedAt) &&
		a.Revoked == b.Revoked && a.UserActive == b.UserActive
}

func TestUnknownAPIToken(t *testing.T) {
	s, _ := newStore(t)
	var pat domain.PAT
	if _, err := s.APITokenByHash(context.Background(), pat.Hash()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("APITokenByHash(unknown) = %v, want app.ErrNotFound", err)
	}
	if _, err := s.APITokenByID(context.Background(), uuid.NewV7()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("APITokenByID(unknown) = %v, want app.ErrNotFound", err)
	}
}

// last_used_at is written when it is unset or older than staleBefore; the
// token's updated_at stays.
func TestTouchAPIToken(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	n, _ := newToken(t, s, u.ID, "CI")
	first, second, third := later(), later().Add(30*time.Second), later().Add(2*time.Minute)

	for _, touch := range []struct{ now, staleBefore, want time.Time }{
		{first, first.Add(-time.Minute), first},   // unset
		{second, second.Add(-time.Minute), first}, // used 30 seconds before
		{third, third.Add(-time.Minute), third},   // used 2 minutes before
		{third.Add(time.Second), first, third},    // exactly at staleBefore: not older
	} {
		if err := s.TouchAPIToken(ctx, n.ID, touch.now, touch.staleBefore); err != nil {
			t.Fatal(err)
		}
		if r := readToken(t, pool, n.ID); r.used == nil || !r.used.Equal(touch.want) || !r.updated.Equal(now()) {
			t.Errorf("after TouchAPIToken(%v, %v): row %+v, want last used %v, updated at %v", touch.now, touch.staleBefore, r, touch.want, now())
		}
	}
}

// A touch skips a row that another transaction holds instead of waiting for
// it; once the row is free, the next touch writes.
func TestTouchAPITokenSkipsARowInUse(t *testing.T) {
	s, pool := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	n, _ := newToken(t, s, u.ID, "CI")
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(context.Background(), "SELECT 1 FROM api_tokens WHERE id = $1 FOR UPDATE", n.ID); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.TouchAPIToken(ctx, n.ID, later(), later().Add(-time.Minute)); err != nil {
		t.Fatalf("touch while the row is held = %v, want no wait", err)
	}
	if r := readToken(t, pool, n.ID); r.used != nil {
		t.Errorf("last used %v while the row is held, want unset", r.used)
	}

	if err := tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.TouchAPIToken(context.Background(), n.ID, later(), later().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if r := readToken(t, pool, n.ID); r.used == nil || !r.used.Equal(later()) {
		t.Errorf("last used %v once the row is free, want %v", r.used, later())
	}
}

// RevokeSessions revokes the account's live sessions but keep, and leaves
// revoked and expired ones, and other accounts', as they are.
func TestRevokeSessions(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u, other := newUser("alice@corp.com"), newUser("bob@corp.com")
	mustCreate(t, s, u)
	mustCreate(t, s, other)
	session := func(userID uuid.UUID, expires time.Time) uuid.UUID {
		n := app.NewSession{ID: uuid.NewV7(), UserID: userID, TokenHash: secretHash(), ExpiresAt: expires, Now: now()}
		if err := s.CreateSession(ctx, n); err != nil {
			t.Fatal(err)
		}
		return n.ID
	}
	keep, live, revoked, expired, others := session(u.ID, sessionEnd()), session(u.ID, sessionEnd()),
		session(u.ID, sessionEnd()), session(u.ID, later()), session(other.ID, sessionEnd())
	exec(t, pool, "UPDATE auth_sessions SET revoked_at = $1, revoke_reason = 'logout' WHERE id = $2", now(), revoked)

	n, err := s.RevokeSessions(ctx, u.ID, keep, domain.RevokePasswordChanged, later())

	if err != nil || n != 1 {
		t.Fatalf("RevokeSessions() = %d, %v; want 1", n, err)
	}
	for _, want := range []struct {
		id     uuid.UUID
		reason string
	}{{keep, ""}, {live, "password_changed"}, {revoked, "logout"}, {expired, ""}, {others, ""}} {
		r := readSession(t, pool, want.id)
		if got := deref(r.reason); got != want.reason {
			t.Errorf("session %v: revoke_reason %q, want %q", want.id, got, want.reason)
		}
	}
	if r := readSession(t, pool, live); !r.revoked.Equal(later()) || !r.updated.Equal(later()) {
		t.Errorf("the revoked session = %+v, want revoked and updated at %v", r, later())
	}
	if n, err := s.RevokeSessions(ctx, u.ID, uuid.Nil(), domain.RevokeDeactivated, later()); err != nil || n != 1 {
		t.Errorf("RevokeSessions() keeping none = %d, %v; want the one kept before", n, err)
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Every reason the domain has is one the column accepts.
func TestEveryRevokeReasonIsStorable(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)
	for _, reason := range []domain.RevokeReason{
		domain.RevokeLogout, domain.RevokePasswordChanged, domain.RevokePasswordReset,
		domain.RevokeEmailChanged, domain.RevokeDeactivated, domain.RevokeReuseDetected,
	} {
		n := app.NewSession{ID: uuid.NewV7(), UserID: u.ID, TokenHash: secretHash(), ExpiresAt: sessionEnd(), Now: now()}
		if err := s.CreateSession(ctx, n); err != nil {
			t.Fatal(err)
		}
		if got, err := s.RevokeSessions(ctx, u.ID, uuid.Nil(), reason, later()); err != nil || got != 1 {
			t.Errorf("RevokeSessions(%s) = %d, %v; want 1", reason, got, err)
		}
	}
}

// The list is the account's unrevoked tokens, expired ones too, newest
// first and then by id; another account's never.
func TestListAPITokens(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u, other := newUser("alice@corp.com"), newUser("bob@corp.com")
	mustCreate(t, s, u)
	mustCreate(t, s, other)
	older, _ := newToken(t, s, u.ID, "older")
	exec(t, pool, "UPDATE api_tokens SET created_at = created_at - interval '1 hour'")
	newToken(t, s, u.ID, "first")
	newToken(t, s, u.ID, "second") // the same instant, a later v7 id: first by id, descending
	revoked, _ := newToken(t, s, u.ID, "revoked")
	newToken(t, s, other.ID, "bob's")
	exec(t, pool, "UPDATE api_tokens SET revoked_at = now() WHERE id = $1", revoked.ID)
	exec(t, pool, "UPDATE api_tokens SET expires_at = created_at + interval '1 second' WHERE id = $1", older.ID)

	got, err := s.ListAPITokens(ctx, u.ID)

	var names []string
	for _, tok := range got {
		names = append(names, tok.Name)
	}
	if want := []string{"second", "first", "older"}; err != nil || !slices.Equal(names, want) {
		t.Fatalf("ListAPITokens() = %q, %v; want %q", names, err, want)
	}
	if got[2].ExpiresAt == nil || got[0].LastUsedAt != nil || !got[0].CreatedAt.Equal(now()) {
		t.Errorf("ListAPITokens() = %+v, want the fields as stored", got)
	}
	if none, err := s.ListAPITokens(ctx, uuid.NewV7()); err != nil || len(none) != 0 {
		t.Errorf("ListAPITokens(no account) = %v, %v; want none", none, err)
	}
}

// Revoking hits only the account's own unrevoked token, once.
func TestRevokeAPIToken(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	u, other := newUser("alice@corp.com"), newUser("bob@corp.com")
	mustCreate(t, s, u)
	mustCreate(t, s, other)
	mine, _ := newToken(t, s, u.ID, "mine")
	theirs, _ := newToken(t, s, other.ID, "theirs")

	if ok, err := s.RevokeAPIToken(ctx, theirs.ID, u.ID, later()); ok || err != nil {
		t.Errorf("RevokeAPIToken(another account's) = %v, %v; want false", ok, err)
	}
	if ok, err := s.RevokeAPIToken(ctx, mine.ID, u.ID, later()); !ok || err != nil {
		t.Errorf("RevokeAPIToken(mine) = %v, %v; want true", ok, err)
	}
	if ok, err := s.RevokeAPIToken(ctx, mine.ID, u.ID, later().Add(time.Minute)); ok || err != nil {
		t.Errorf("RevokeAPIToken(mine) again = %v, %v; want false", ok, err)
	}
	if r := readToken(t, pool, mine.ID); r.revoked == nil || !r.revoked.Equal(later()) || !r.updated.Equal(later()) {
		t.Errorf("my token = %+v, want revoked and updated at %v, the first time", r, later())
	}
	if r := readToken(t, pool, theirs.ID); r.revoked != nil {
		t.Errorf("their token = %+v, want it live", r)
	}
}

func TestPasswordAccount(t *testing.T) {
	s, _ := newStore(t)
	u := newUser("alice@corp.com")
	mustCreate(t, s, u)

	got, err := s.PasswordAccount(context.Background(), u.ID)

	if err != nil || got != (app.PasswordAccount{Email: u.Email, PasswordHash: u.PasswordHash}) {
		t.Errorf("PasswordAccount() = %+v, %v; want the address and the hash", got, err)
	}
	if _, err := s.PasswordAccount(context.Background(), uuid.NewV7()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("PasswordAccount(unknown) = %v, want app.ErrNotFound", err)
	}
}
