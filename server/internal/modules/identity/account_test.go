package identity_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The caller's account on a real database, through the HTTP operations
// (M1/P3 design 3.5).

// me is the User answer.
type me struct {
	DisplayName     string   `json:"display_name"`
	OnboardingSteps []string `json:"onboarding_steps"`
}

func decodeMe(t *testing.T, body []byte) me {
	t.Helper()
	var m me
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return m
}

func TestUpdateTheDisplayName(t *testing.T) {
	h, _ := newServer(t)
	_, tokens := register(h, "alice@corp.com")
	pat := createToken(t, h, tokens.AccessToken, "agent")

	rec := send(h, http.MethodPatch, "/api/v0/me", pat.Token, `{"display_name":"  Alice Stone "}`)
	bad := send(h, http.MethodPatch, "/api/v0/me", pat.Token, `{"display_name":"Alice\u0007"}`)
	empty := send(h, http.MethodPatch, "/api/v0/me", pat.Token, `{}`)

	if rec.Code != http.StatusOK || decodeMe(t, rec.Body.Bytes()).DisplayName != "Alice Stone" {
		t.Errorf("update = %d %s, want 200 with Alice Stone", rec.Code, rec.Body)
	}
	if bad.Code != http.StatusUnprocessableEntity || empty.Code != http.StatusOK || decodeMe(t, empty.Body.Bytes()).DisplayName != "Alice Stone" {
		t.Errorf("a bell in the name = %d, an empty patch = %d %s; want 422, then the account unchanged", bad.Code, empty.Code, empty.Body)
	}
}

func steps(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	var s []string
	if err := pool.QueryRow(context.Background(), `SELECT onboarding_steps FROM users`).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// A step is recorded once; 32 at most.
func TestRecordOnboardingSteps(t *testing.T) {
	h, pool := newServer(t)
	_, tokens := register(h, "alice@corp.com")
	record := func(step string) int {
		return send(h, http.MethodPost, "/api/v0/me/onboarding-steps", tokens.AccessToken, `{"step":"`+step+`"}`).Code
	}

	codes := []int{record("profile"), record("profile"), record("Profile")}
	for i := 1; i < 32; i++ {
		record(fmt.Sprintf("step_%d", i))
	}
	beyond := send(h, http.MethodPost, "/api/v0/me/onboarding-steps", tokens.AccessToken, `{"step":"one_more"}`)

	if want := []int{200, 200, 422}; !slices.Equal(codes, want) {
		t.Errorf("profile, profile again, Profile = %v, want %v", codes, want)
	}
	if got := steps(t, pool); len(got) != 32 || got[0] != "profile" || beyond.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(beyond.Body.String(), `"field":"step"`) {
		t.Errorf("steps = %q, the 33rd = %d %s; want 32 steps, profile first, then 422 on step", got, beyond.Code, beyond.Body)
	}
}

// sessionReasons are the revoke reasons of the account's sessions, by
// session id order: "" for a live one.
func sessionReasons(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT coalesce(revoke_reason, '') FROM auth_sessions ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var reasons []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			t.Fatal(err)
		}
		reasons = append(reasons, r)
	}
	return reasons
}

func changePassword(h http.Handler, credential, current, next string) int {
	return send(h, http.MethodPost, "/api/v0/me/change-password", credential,
		`{"current_password":"`+current+`","new_password":"`+next+`"}`).Code
}

// With a session, the other sessions end and the caller's goes on; the
// personal access tokens keep working; only the new password signs in.
func TestChangeThePasswordFromASession(t *testing.T) {
	h, pool := newServer(t)
	_, first := register(h, "alice@corp.com")
	_, second := login(h, "alice@corp.com", "correct horse battery")
	pat := createToken(t, h, first.AccessToken, "agent")

	code := changePassword(h, second.AccessToken, "correct horse battery", "N3w-Passw0rd!")

	if code != http.StatusNoContent || !slices.Equal(sessionReasons(t, pool), []string{"password_changed", ""}) {
		t.Errorf("change = %d, sessions %q; want 204, the first revoked for password_changed, the caller's live", code, sessionReasons(t, pool))
	}
	if getMe(h, first.AccessToken).Code != http.StatusUnauthorized || getMe(h, second.AccessToken).Code != http.StatusOK || getMe(h, pat.Token).Code != http.StatusOK {
		t.Error("want the other session refused, the caller's session and the token working")
	}
	old, _ := login(h, "alice@corp.com", "correct horse battery")
	fresh, _ := login(h, "alice@corp.com", "N3w-Passw0rd!")
	if old.Code != http.StatusUnauthorized || fresh.Code != http.StatusOK {
		t.Errorf("sign-in with the old password = %d, the new = %d; want 401, 200", old.Code, fresh.Code)
	}
}

// With a personal access token, every session ends; the token goes on.
func TestChangeThePasswordWithAToken(t *testing.T) {
	h, pool := newServer(t)
	_, first := register(h, "alice@corp.com")
	pat := createToken(t, h, first.AccessToken, "agent")

	wrong := changePassword(h, pat.Token, "not the password", "N3w-Passw0rd!")
	weak := changePassword(h, pat.Token, "correct horse battery", "alice123")
	code := changePassword(h, pat.Token, "correct horse battery", "N3w-Passw0rd!")

	if wrong != http.StatusUnprocessableEntity || weak != http.StatusUnprocessableEntity || code != http.StatusNoContent {
		t.Errorf("wrong current = %d, weak new = %d, change = %d; want 422, 422, 204", wrong, weak, code)
	}
	if !slices.Equal(sessionReasons(t, pool), []string{"password_changed"}) || getMe(h, pat.Token).Code != http.StatusOK {
		t.Errorf("sessions %q; want the only one revoked, the token working", sessionReasons(t, pool))
	}
}
