package app_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
)

// newLogout shares newRefresh's session: generation 3, secret 3.
func newLogout() (*app.Logout, *refreshFixture) {
	f := newRefresh(signedWith())
	return app.NewLogout(f.sessions, fixedClock(testNow()), slog.New(slog.NewJSONHandler(f.logs, nil))), f
}

func TestLogoutEndsTheSessionOfTheCurrentGeneration(t *testing.T) {
	uc, f := newLogout()

	err := uc.Execute(context.Background(), issued(signedWith(), 3, 3).String())

	if err != nil || !f.row.State.Revoked || f.row.reason != "logout" || !f.row.changedAt.Equal(testNow()) || f.tx.calls != 0 {
		t.Errorf("Execute() = %v, session %+v (%s at %v), %d transactions; want revoked for logout now, by one statement",
			err, f.row.State, f.row.reason, f.row.changedAt, f.tx.calls)
	}
	if want := `"msg":"signed out","session_id":"` + sessionIDText + `"`; !strings.Contains(f.logs.String(), want) {
		t.Errorf("logs = %s, want %s", f.logs, want)
	}
}

// Any other token changes nothing and is no error: the answer never tells
// what the token was, and an older generation is no reuse here (M1/P2
// design 3.6).
func TestLogoutLeavesTheSessionForAnyOtherToken(t *testing.T) {
	unknown := issued(signedWith(), 3, 3)
	unknown.SessionID = uuid.NewV7()
	tests := []struct {
		name  string
		token string
		row   func(*fakeSession)
	}{
		{"an older generation the session issued", issued(signedWith(), 2, 2).String(), nil},
		{"a forged older generation", forged(2).String(), nil},
		{"the current generation with another secret", issued(signedWith(), 3, 9).String(), nil},
		{"a newer generation", issued(signedWith(), 4, 4).String(), nil},
		{"an unknown session", unknown.String(), nil},
		{"a revoked session", issued(signedWith(), 3, 3).String(), func(s *fakeSession) { s.State.Revoked, s.reason = true, "reuse_detected" }},
		{"a session ending now", issued(signedWith(), 3, 3).String(), func(s *fakeSession) { s.State.ExpiresAt = testNow() }},
		{"a malformed token", "not a token", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, f := newLogout()
			if tt.row != nil {
				tt.row(f.row)
			}
			before := *f.row

			err := uc.Execute(context.Background(), tt.token)

			if err != nil || f.row.State.Revoked != before.State.Revoked || f.row.reason != before.reason || f.row.State.Generation != 3 {
				t.Errorf("Execute() = %v, session %+v (%s); want nil and the session unchanged", err, f.row.State, f.row.reason)
			}
			if f.logs.Len() != 0 {
				t.Errorf("logs = %s, want none", f.logs)
			}
		})
	}
}
