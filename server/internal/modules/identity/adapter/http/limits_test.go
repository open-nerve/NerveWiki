package httpadapter_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/ratelimit"
)

// tightLimits are small buckets on a limiter whose clock stands still, so
// nothing refills during a test: login_ip 3, login_ip_email 2,
// register_ip 1, each regaining a unit a minute.
func tightLimits() httpadapter.Limits {
	limiter := ratelimit.New(created)
	return httpadapter.Limits{
		Limiter:      limiter,
		LoginIP:      limiter.Bucket("login_ip", ratelimit.Rate{PerMinute: 1, Burst: 3}),
		LoginIPEmail: limiter.Bucket("login_ip_email", ratelimit.Rate{PerMinute: 1, Burst: 2}),
		RegisterIP:   limiter.Bucket("register_ip", ratelimit.Rate{PerMinute: 1, Burst: 1}),
	}
}

func limitedServer(t *testing.T, uc httpadapter.UseCases, logs *bytes.Buffer) http.Handler {
	t.Helper()
	return serverWithLimits(t, uc, tightLimits(), logs)
}

func serverWithLimits(t *testing.T, uc httpadapter.UseCases, limits httpadapter.Limits, logs *bytes.Buffer) http.Handler {
	t.Helper()
	return serverWith(t, uc, httpadapter.Settings{Limits: limits, RefreshDeadline: refreshDeadline, Logger: slog.New(slog.NewJSONHandler(logs, nil))})
}

// rateLimitLogs are the "rate limited" entries of logs.
func rateLimitLogs(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()
	var entries []map[string]any
	for line := range strings.Lines(logs.String()) {
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		if e["msg"] == "rate limited" {
			entries = append(entries, e)
		}
	}
	return entries
}

// Login takes a unit of login_ip and of login_ip_email together, or of
// neither (M1/P2 design 3.2): one address fails twice and is refused; the
// refusal leaves login_ip as it was, so another address from the client
// gets its last unit, then login_ip refuses.
func TestLoginLimitsByIPAndByIPWithAddress(t *testing.T) {
	var logs bytes.Buffer
	login := &fakeLogin{err: domain.ErrInvalidCredentials}
	h := limitedServer(t, httpadapter.UseCases{Login: login}, &logs)

	var statuses []int
	var retryAfter []string
	// " Alice@Corp.com" normalizes to alice@corp.com: the same bucket.
	for _, email := range []string{"alice@corp.com", " Alice@Corp.com", "alice@corp.com", "bob@corp.com", "carol@corp.com"} {
		res, _ := do(t, h, postJSON("/api/v0/auth/login", `{"email":"`+email+`","password":"x"}`))
		statuses = append(statuses, res.StatusCode)
		retryAfter = append(retryAfter, res.Header.Get("Retry-After"))
	}

	if want := []int{401, 401, 429, 401, 429}; !slices.Equal(statuses, want) || login.calls != 3 {
		t.Errorf("statuses = %v after %d logins, want %v after 3", statuses, login.calls, want)
	}
	if retryAfter[2] != "60" || retryAfter[4] != "60" {
		t.Errorf("Retry-After = %q, want 60 on both refusals", retryAfter)
	}
	entries := rateLimitLogs(t, &logs)
	if len(entries) != 2 || entries[0]["bucket"] != "login_ip_email" || entries[1]["bucket"] != "login_ip" ||
		entries[0]["ip"] != "203.0.113.7" || entries[0]["request_id"] == nil {
		t.Errorf("rate limited logs = %v, want login_ip_email then login_ip, with the client and the request", entries)
	}
	if strings.Contains(logs.String(), "alice") {
		t.Errorf("logs name the address: %s", logs.String())
	}
}

func TestRegisterLimitsByIP(t *testing.T) {
	var logs bytes.Buffer
	h := limitedServer(t, httpadapter.UseCases{Register: &fakeRegister{err: domain.ErrEmailTaken}}, &logs)

	first, _ := do(t, h, registerRequest(`{"email":"a@b.co","password":"x"}`))
	second, body := do(t, h, registerRequest(`{"email":"c@d.co","password":"x"}`))

	if first.StatusCode != 409 || second.StatusCode != 429 || !strings.Contains(body, `"code":"rate_limited"`) || second.Header.Get("Retry-After") != "60" {
		t.Errorf("responses = %d, %d %s Retry-After %q; want 409, then 429 rate_limited Retry-After 60",
			first.StatusCode, second.StatusCode, body, second.Header.Get("Retry-After"))
	}
	if entries := rateLimitLogs(t, &logs); len(entries) != 1 || entries[0]["bucket"] != "register_ip" {
		t.Errorf("rate limited logs = %v, want register_ip", entries)
	}
}

// The module's buckets count a client by its IP key, so an IPv6 /64 is one
// client; login_ip_email counts that key and the address together, so
// another client trying the same address has a bucket of its own.
func TestModuleLimitsCountByTheIPKey(t *testing.T) {
	var logs bytes.Buffer
	uc := httpadapter.UseCases{Login: &fakeLogin{err: domain.ErrInvalidCredentials}, Register: &fakeRegister{err: domain.ErrEmailTaken}}
	h := limitedServer(t, uc, &logs)
	from := func(peer, path string) int {
		req := postJSON(path, `{"email":"alice@corp.com","password":"x"}`)
		req.RemoteAddr = peer
		res, _ := do(t, h, req)
		return res.StatusCode
	}

	statuses := []int{
		from("[2001:db8:1:2::7]:5555", "/api/v0/auth/login"),
		from("[2001:db8:1:2::8]:5555", "/api/v0/auth/login"),
		from("[2001:db8:1:2::9]:5555", "/api/v0/auth/login"), // the /64 has used login_ip_email's 2
		from("[2001:db8:1:3::7]:5555", "/api/v0/auth/login"), // another /64
		from("[2001:db8:1:2::7]:5555", "/api/v0/auth/register"),
		from("[2001:db8:1:2::8]:5555", "/api/v0/auth/register"), // the /64 has used register_ip's 1
	}

	if want := []int{401, 401, 429, 401, 409, 429}; !slices.Equal(statuses, want) {
		t.Errorf("statuses = %v, want %v", statuses, want)
	}
}

// Refresh and logout take no unit of the module's buckets: only the
// platform's anonymous bucket limits them (M1/P2 design 3.2), so a client
// locked out of sign-in still keeps its sessions alive and can end them.
func TestRefreshAndLogoutTakeNoUnitOfTheModule(t *testing.T) {
	limits := tightLimits()
	for range 3 {
		limits.Limiter.AllowAll(ratelimit.Check{Bucket: limits.LoginIP, Key: "203.0.113.7"})
	}
	limits.Limiter.AllowAll(ratelimit.Check{Bucket: limits.RegisterIP, Key: "203.0.113.7"})
	var logs bytes.Buffer
	h := serverWithLimits(t, httpadapter.UseCases{Refresh: &fakeRefresh{tokens: sampleTokens()}}, limits, &logs)

	login, _ := do(t, h, postJSON("/api/v0/auth/login", `{"email":"a@b.co","password":"x"}`))
	register, _ := do(t, h, registerRequest(`{"email":"a@b.co","password":"x"}`))
	refresh, _ := do(t, h, postJSON("/api/v0/auth/refresh", `{"refresh_token":"nwk_rt_x"}`))
	logout, _ := do(t, h, postJSON("/api/v0/auth/logout", `{"refresh_token":"nwk_rt_x"}`))

	if login.StatusCode != 429 || register.StatusCode != 429 || refresh.StatusCode != 200 || logout.StatusCode != 204 {
		t.Errorf("login %d, register %d, refresh %d, logout %d; want 429, 429, 200, 204", login.StatusCode, register.StatusCode, refresh.StatusCode, logout.StatusCode)
	}
}
