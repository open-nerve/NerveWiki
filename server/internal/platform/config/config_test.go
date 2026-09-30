package config

import (
	"bytes"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
)

func TestLogValueMasksDatabaseURL(t *testing.T) {
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("configuration loaded", "config", validConfig())

	out := buf.String()
	if strings.Contains(out, "secret") {
		t.Errorf("log output leaks a secret: %s", out)
	}
	for _, want := range []string{
		"config.env=test",
		"config.server.addr=:8080",
		"config.server.read_header_timeout=5s",
		"config.server.read_timeout=30s",
		"config.server.write_timeout=1m0s",
		"config.server.shutdown_timeout=20s",
		"config.server.request_timeout=15s",
		"config.server.max_body_bytes=1048576",
		`config.server.addr_file=""`,
		"config.database.url=xxxxx",
		"config.database.max_conns=10",
		"config.database.auto_migrate=true",
		"config.database.commit_timeout=2s",
		"config.auth.signup_enabled=false",
		"config.auth.access_token_ttl=15m0s",
		"config.auth.session_ttl=720h0m0s",
		"config.auth.refresh_deadline=4s",
		"config.auth.jwt.private_key_file_set=true",
		"config.auth.password.argon2_memory_kib=19456",
		"config.auth.password.argon2_iterations=2",
		"config.auth.password.argon2_parallelism=1",
		"config.auth.password.max_concurrent_hashes=4",
		"config.auth.password.max_wait=2s",
		"config.ratelimit.ipv6_prefix_len=64",
		"config.ratelimit.anonymous.per_minute=600",
		"config.ratelimit.anonymous.burst=100",
		"config.ratelimit.auth_failure.per_minute=60",
		"config.ratelimit.authenticated.burst=200",
		"config.ratelimit.login_ip.per_minute=30",
		"config.ratelimit.login_ip_email.burst=5",
		"config.ratelimit.register_ip.per_minute=10",
		"config.log.level=info",
		"config.log.format=json",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log output lacks %q: %s", want, out)
		}
	}
}

// The configuration names secret files but never shows where they are; the
// trusted proxies are listed.
func TestLogValueShowsWhetherTheSigningKeyIsSet(t *testing.T) {
	cfg := validConfig()
	cfg.Server.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("fd00::/8")}
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("configuration loaded", "config", cfg)

	out := buf.String()
	if strings.Contains(out, "jwt-key") || !strings.Contains(out, "config.auth.jwt.private_key_file_set=true") ||
		!strings.Contains(out, "config.server.trusted_proxies=10.0.0.0/8,fd00::/8") {
		t.Errorf("log output = %s, want the key file as set only and the proxies listed", out)
	}
}

// JSON logs render durations as "5s" too, not as nanoseconds.
func TestLogValueRendersReadableDurationsInJSON(t *testing.T) {
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("configuration loaded", "config", validConfig())

	for _, want := range []string{`"read_header_timeout":"5s"`, `"write_timeout":"1m0s"`, `"commit_timeout":"2s"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log output lacks %s: %s", want, buf.String())
		}
	}
}

// The URL is masked as a whole, whatever its form: pgx parses it with its own
// libpq-compatible grammar, which accepts forms other URL parsers misread.
func TestDatabaseConfigLogValueMasksTheWholeURL(t *testing.T) {
	for _, url := range []string{
		"postgres://nervewiki:secret@localhost:5432/nervewiki",
		"postgres://localhost/nervewiki?password=secret&sslmode=disable",
		"postgres://localhost/nervewiki?password=secret;more&sslmode=disable", // raw ';': net/url drops the pair, pgx keeps it
		"postgres://localhost/nervewiki?pass%77ord=secret",
		"postgres://localhost/nervewiki?sslpassword=secret",
		"postgres://nervewiki:pa@ss@localhost/nervewiki?password=a%ZZsecret",
		"host=localhost user=nervewiki password=secret",
	} {
		var buf bytes.Buffer
		slog.New(slog.NewTextHandler(&buf, nil)).Info("x", "db", DatabaseConfig{URL: url, MaxConns: 10})

		out := buf.String()
		if strings.Contains(out, "secret") || !strings.Contains(out, "db.url=xxxxx ") {
			t.Errorf("DatabaseConfig{URL: %q} logs %s; want db.url=xxxxx and no password", url, out)
		}
		for _, want := range []string{"db.max_conns=10", "db.auto_migrate=false"} {
			if !strings.Contains(out, want) {
				t.Errorf("log output lacks %q: %s", want, out)
			}
		}
	}
}

func TestDatabaseConfigLogValueShowsAnEmptyURL(t *testing.T) {
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("x", "db", DatabaseConfig{})

	if out := buf.String(); !strings.Contains(out, `db.url="" `) {
		t.Errorf("log output = %s, want an empty db.url", out)
	}
}
