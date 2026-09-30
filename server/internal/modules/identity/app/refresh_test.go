package app_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func clientIP() netip.Addr  { return netip.MustParseAddr("203.0.113.7") }
func sessionEnd() time.Time { return testNow().Add(time.Hour) }
func signedWith() fakeMAC   { return fakeMAC{key: "the signing key"} }

// issued is generation g of the test session with secret byte s, tagged
// under mac: a token the server issued.
func issued(mac fakeMAC, g uint32, s byte) domain.RefreshToken {
	t := domain.RefreshToken{SessionID: testSessionID(), Generation: g, Secret: [32]byte{s}}
	t.Tag = mac.Tag(t.MACMessage())
	return t
}

// forged is generation g of the test session with a random secret and tag.
func forged(g uint32) domain.RefreshToken {
	t := domain.RefreshToken{SessionID: testSessionID(), Generation: g}
	_, _ = rand.Read(t.Secret[:])
	_, _ = rand.Read(t.Tag[:])
	return t
}

type refreshFixture struct {
	sessions *fakeSessions
	row      *fakeSession
	tx       *fakeTx
	tokens   *fakeTokens
	logs     *bytes.Buffer
	uc       *app.Refresh
}

// newRefresh has one session of the test account at generation 3, secret 3,
// until sessionEnd; the use case tags under mac.
func newRefresh(mac fakeMAC) *refreshFixture {
	row := &fakeSession{RefreshSession: app.RefreshSession{
		UserID: testUserID(),
		State:  domain.SessionState{Generation: 3, TokenHash: issued(signedWith(), 3, 3).SecretHash(), ExpiresAt: sessionEnd()},
	}}
	f := &refreshFixture{
		sessions: &fakeSessions{rows: map[uuid.UUID]*fakeSession{testSessionID(): row}},
		row:      row,
		tx:       &fakeTx{},
		tokens:   newFakeTokens(),
		logs:     &bytes.Buffer{},
	}
	f.uc = app.NewRefresh(app.RefreshDeps{
		Sessions: f.sessions,
		Tx:       f.tx,
		Issuance: testIssuance(f.tokens, mac),
		Clock:    fixedClock(testNow()),
		Logger:   slog.New(slog.NewJSONHandler(f.logs, nil)),
	})
	return f
}

func TestRefreshRotates(t *testing.T) {
	f := newRefresh(signedWith())

	tokens, err := f.uc.Execute(context.Background(), issued(signedWith(), 3, 3).String(), clientIP())
	if err != nil {
		t.Fatal(err)
	}

	next, ok := domain.ParseRefreshToken(tokens.RefreshToken)
	if !ok || next.SessionID != testSessionID() || next.Generation != 4 || !signedWith().Verify(next.MACMessage(), next.Tag) {
		t.Fatalf("refresh token = %+v, %v; want generation 4 of the session, tagged", next, ok)
	}
	if f.row.State.Generation != 4 || !bytes.Equal(f.row.State.TokenHash, next.SecretHash()) || f.row.State.Revoked ||
		!f.row.State.ExpiresAt.Equal(sessionEnd()) || !f.row.changedAt.Equal(testNow()) {
		t.Errorf("session = %+v, changed at %v; want generation 4 with the new secret's hash, its end unchanged, changed now", f.row.State, f.row.changedAt)
	}
	want := app.AccessClaims{UserID: testUserID(), SessionID: testSessionID(), ExpiresAt: accessExpiry()}
	if !slices.Equal(f.tokens.issued, []app.AccessClaims{want}) || !tokens.RefreshExpiresAt.Equal(sessionEnd()) || tokens.AccessExpiresIn != 15*time.Minute {
		t.Errorf("tokens = %+v, claims %+v; want claims %+v and the session's end", tokens, f.tokens.issued, want)
	}
	if f.tx.calls != 1 || len(f.sessions.outsideTx) != 0 {
		t.Errorf("transactions %d, writes outside one %q; want one", f.tx.calls, f.sessions.outsideTx)
	}
}

// Each refresh uses the token the last one returned; the session's end
// never moves.
func TestRefreshEveryGenerationInTurn(t *testing.T) {
	f := newRefresh(signedWith())
	token := issued(signedWith(), 3, 3).String()
	for g := uint32(4); g <= 6; g++ {
		tokens, err := f.uc.Execute(context.Background(), token, clientIP())
		if err != nil || f.row.State.Generation != g || !tokens.RefreshExpiresAt.Equal(sessionEnd()) {
			t.Fatalf("refresh to %d: %v, session at %d, end %v", g, err, f.row.State.Generation, tokens.RefreshExpiresAt)
		}
		token = tokens.RefreshToken
	}
}

// An older generation that the session issued comes back: someone else has
// a copy. The session is revoked, and every token of it fails from now on.
func TestRefreshDetectsReuse(t *testing.T) {
	f := newRefresh(signedWith())

	_, err := f.uc.Execute(context.Background(), issued(signedWith(), 2, 2).String(), clientIP())

	if !errors.Is(err, domain.ErrRefreshTokenInvalid) {
		t.Errorf("Execute() = %v, want identity.refresh_token_invalid", err)
	}
	if !f.row.State.Revoked || f.row.reason != "reuse_detected" || !f.row.changedAt.Equal(testNow()) || f.row.State.Generation != 3 || len(f.sessions.outsideTx) != 0 {
		t.Errorf("session = %+v (%s at %v), writes outside the transaction %q; want revoked for reuse_detected now, in it",
			f.row.State, f.row.reason, f.row.changedAt, f.sessions.outsideTx)
	}
	want := `"level":"WARN","msg":"refresh token reused: session revoked","user_id":"` + userIDText +
		`","session_id":"` + sessionIDText + `","ip":"203.0.113.7"`
	if !strings.Contains(f.logs.String(), want) {
		t.Errorf("logs = %s, want %s", f.logs, want)
	}
	if _, err := f.uc.Execute(context.Background(), issued(signedWith(), 3, 3).String(), clientIP()); !errors.Is(err, domain.ErrRefreshTokenInvalid) {
		t.Errorf("the current generation after the reuse: %v, want identity.refresh_token_invalid", err)
	}
}

// Everything else is 401 and leaves the session as it is: only a token the
// session did issue can end it (M1/P2 design 3.5).
func TestRefreshRejectsWithoutRevoking(t *testing.T) {
	otherSecret := issued(signedWith(), 3, 9)
	unknown := issued(signedWith(), 3, 3)
	unknown.SessionID = uuid.NewV7()
	tests := []struct {
		name  string
		token string
		row   func(*fakeSession)
		reads int
	}{
		{"a forged older generation", forged(2).String(), nil, 1},
		{"the session id with generation 0 and a forged tag", forged(0).String(), nil, 1},
		{"the current generation with another secret", otherSecret.String(), nil, 1},
		{"a newer generation", issued(signedWith(), 4, 4).String(), nil, 1},
		{"an unknown session", unknown.String(), nil, 1},
		{"a revoked session", issued(signedWith(), 3, 3).String(), func(s *fakeSession) { s.State.Revoked, s.reason = true, "logout" }, 1},
		{"a session ending now", issued(signedWith(), 3, 3).String(), func(s *fakeSession) { s.State.ExpiresAt = testNow() }, 1},
		{"a malformed token", "nwk_rt_garbage", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newRefresh(signedWith())
			if tt.row != nil {
				tt.row(f.row)
			}
			before := *f.row

			_, err := f.uc.Execute(context.Background(), tt.token, clientIP())

			if !errors.Is(err, domain.ErrRefreshTokenInvalid) || f.sessions.reads != tt.reads {
				t.Errorf("Execute() = %v after %d reads, want identity.refresh_token_invalid after %d", err, f.sessions.reads, tt.reads)
			}
			if f.row.State.Revoked != before.State.Revoked || f.row.reason != before.reason || f.row.State.Generation != 3 {
				t.Errorf("session = %+v (%s), want it unchanged", f.row.State, f.row.reason)
			}
			if strings.Contains(f.logs.String(), "WARN") {
				t.Errorf("logs = %s, want no reuse warning", f.logs)
			}
		})
	}
}

// After the signing key changed, an older generation's tag no longer
// holds: treated as forged, 401 without revoking. The current generation
// rotates as before: its stored hash proves it (M1/P2 design 3.5).
func TestRefreshAfterTheSigningKeyChanged(t *testing.T) {
	newKey := fakeMAC{key: "the new signing key"}
	f := newRefresh(newKey)

	_, oldGeneration := f.uc.Execute(context.Background(), issued(signedWith(), 2, 2).String(), clientIP())
	tokens, current := f.uc.Execute(context.Background(), issued(signedWith(), 3, 3).String(), clientIP())

	if !errors.Is(oldGeneration, domain.ErrRefreshTokenInvalid) || f.row.reason != "" {
		t.Errorf("older generation: %v, revoke reason %q; want 401 without revoking", oldGeneration, f.row.reason)
	}
	next, _ := domain.ParseRefreshToken(tokens.RefreshToken)
	if current != nil || f.row.State.Generation != 4 || !newKey.Verify(next.MACMessage(), next.Tag) {
		t.Errorf("current generation: %v, session at %d; want it rotated to 4 and tagged under the new key", current, f.row.State.Generation)
	}
}

// Another refresh with the same token commits between the read and the
// conditional UPDATE: the UPDATE misses, the row is read again, and the
// token is now an older generation the session issued: reuse.
func TestRefreshJudgesAgainWhenAConcurrentRefreshWins(t *testing.T) {
	f := newRefresh(signedWith())
	f.sessions.beforeWrite = func() {
		f.sessions.beforeWrite = nil
		f.row.State.Generation, f.row.State.TokenHash = 4, issued(signedWith(), 4, 4).SecretHash()
	}

	_, err := f.uc.Execute(context.Background(), issued(signedWith(), 3, 3).String(), clientIP())

	if !errors.Is(err, domain.ErrRefreshTokenInvalid) || f.sessions.reads != 2 || f.tx.calls != 1 {
		t.Errorf("Execute() = %v after %d reads in %d transactions; want 401 after a second read in the same one", err, f.sessions.reads, f.tx.calls)
	}
	if !f.row.State.Revoked || f.row.reason != "reuse_detected" {
		t.Errorf("session = %+v (%s), want revoked for reuse_detected", f.row.State, f.row.reason)
	}
}

// A revocation commits first: the second read rejects, and the
// revocation keeps its reason.
func TestRefreshJudgesAgainWhenARevocationWins(t *testing.T) {
	f := newRefresh(signedWith())
	f.sessions.beforeWrite = func() {
		f.sessions.beforeWrite = nil
		f.row.State.Revoked, f.row.reason = true, "logout"
	}

	_, err := f.uc.Execute(context.Background(), issued(signedWith(), 3, 3).String(), clientIP())

	if !errors.Is(err, domain.ErrRefreshTokenInvalid) || f.sessions.reads != 2 || f.row.reason != "logout" {
		t.Errorf("Execute() = %v after %d reads, reason %q; want 401 after a second read, logout kept", err, f.sessions.reads, f.row.reason)
	}
}

// The rotation cannot miss twice (a session never goes back to a
// generation); if it does, that is a fault, not an answer.
func TestRefreshGivesUpWhenTheRotationMissesTwice(t *testing.T) {
	f := newRefresh(signedWith())
	state := f.row.State
	f.sessions.beforeWrite = func() { f.row.State.Revoked = true }
	f.sessions.afterWrite = func() { f.row.State = state }

	_, err := f.uc.Execute(context.Background(), issued(signedWith(), 3, 3).String(), clientIP())

	var se *shared.Error
	if err == nil || errors.As(err, &se) || f.sessions.reads != 2 {
		t.Errorf("Execute() = %v after %d reads, want a fault after two", err, f.sessions.reads)
	}
}

// Neither a token nor its hash reaches the log.
func TestRefreshLogsNoSecret(t *testing.T) {
	f := newRefresh(signedWith())
	tokens, err := f.uc.Execute(context.Background(), issued(signedWith(), 3, 3).String(), clientIP())
	if err != nil {
		t.Fatal(err)
	}
	old := issued(signedWith(), 3, 3)
	_, _ = f.uc.Execute(context.Background(), old.String(), clientIP())

	logs := f.logs.String()
	assertNoSecret(t, logs, "reused refresh token", []byte(old.String()))
	assertNoSecret(t, logs, "reused token's hash", old.SecretHash())
	assertNoSecret(t, logs, "refresh token", []byte(tokens.RefreshToken))
	assertNoSecret(t, logs, "access token", []byte(tokens.AccessToken))
	assertNoSecret(t, logs, "token hash", f.row.State.TokenHash)
}
