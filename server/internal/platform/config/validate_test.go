package config

import (
	"net/netip"
	"strings"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		Env: EnvTest,
		Server: ServerConfig{
			Addr:              ":8080",
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			ShutdownTimeout:   20 * time.Second,
			RequestTimeout:    15 * time.Second,
			MaxBodyBytes:      1 << 20,
		},
		Database: DatabaseConfig{
			URL:           "postgres://nervewiki:secret@localhost:5432/nervewiki",
			MaxConns:      10,
			AutoMigrate:   true,
			CommitTimeout: 2 * time.Second,
		},
		Auth: AuthConfig{
			AccessTokenTTL:         15 * time.Minute,
			SessionTTL:             720 * time.Hour,
			RefreshDeadline:        4 * time.Second,
			SessionCleanupInterval: time.Hour,
			JWT:                    JWTConfig{PrivateKeyFile: "/run/secrets/jwt-key.pem"},
			Password: PasswordConfig{
				Argon2MemoryKiB: 19456, Argon2Iterations: 2, Argon2Parallelism: 1,
				MaxConcurrentHashes: 4, MaxWait: 2 * time.Second,
			},
		},
		RateLimit: RateLimitConfig{
			IPv6PrefixLen: 64,
			Anonymous:     BucketConfig{PerMinute: 600, Burst: 100},
			AuthFailure:   BucketConfig{PerMinute: 60, Burst: 60},
			Authenticated: BucketConfig{PerMinute: 1200, Burst: 200},
			LoginIP:       BucketConfig{PerMinute: 30, Burst: 10},
			LoginIPEmail:  BucketConfig{PerMinute: 10, Burst: 5},
			RegisterIP:    BucketConfig{PerMinute: 10, Burst: 5},
			PasswordUser:  BucketConfig{PerMinute: 5, Burst: 5},
		},
		Workspace: WorkspaceConfig{CreationEnabled: true},
		Page:      PageConfig{EditSessionCleanupInterval: 10 * time.Minute, ParseBudgetBytes: 8 << 20, ParseMaxWait: 2 * time.Second},
		Jobs:      JobsConfig{ShutdownTimeout: 10 * time.Second, PurgeInterval: time.Hour, PurgeRetention: 1440 * time.Hour},
		Log:       LogConfig{Level: "info", Format: "json"},
	}
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	if err := validConfig().validate(); err != nil {
		t.Fatalf("validate() = %v, want nil", err)
	}
}

func TestValidateReportsEveryInvalidKey(t *testing.T) {
	cfg := Config{
		Env:      EnvProd,
		Server:   ServerConfig{Addr: "8080", ReadHeaderTimeout: 0, ReadTimeout: 0, WriteTimeout: -time.Second, ShutdownTimeout: -time.Second},
		Database: DatabaseConfig{URL: "", MaxConns: 0},
		Log:      LogConfig{Level: "verbose", Format: "xml"},
	}
	err := cfg.validate()
	if err == nil {
		t.Fatal("validate() = nil, want errors")
	}
	want := []string{
		`server.addr: must be host:port, e.g. ":8080", got "8080"`,
		"server.read_header_timeout: must be positive, got 0s",
		"server.read_timeout: must be positive, got 0s",
		"server.write_timeout: must be positive, got -1s",
		"server.shutdown_timeout: must be positive, got -1s",
		"server.request_timeout: must be positive, got 0s",
		"server.max_body_bytes: must be at least 1, got 0",
		"database.url: is required",
		"database.max_conns: must be at least 1, got 0",
		"database.commit_timeout: must be positive, got 0s",
		"auth.access_token_ttl: must be positive, got 0s",
		"auth.session_ttl: must be positive, got 0s",
		"auth.session_cleanup_interval: must be at least 1s, got 0s",
		"auth.jwt.private_key_file: is required in prod: a PKCS#8 PEM Ed25519 private key, e.g. from openssl genpkey -algorithm ed25519",
		"auth.password.argon2_iterations: must be at least 1, got 0",
		"auth.password.argon2_parallelism: must be at least 1, got 0",
		"auth.password.argon2_memory_kib: must be at least 8 per lane (8), got 0",
		"auth.password.max_concurrent_hashes: must be at least 1, got 0",
		"auth.password.max_wait: must be positive, got 0s",
		"auth.refresh_deadline: must be positive, got 0s",
		"ratelimit.ipv6_prefix_len: must be from 1 to 128, got 0",
		"ratelimit.anonymous.per_minute: must be at least 1, got 0",
		"ratelimit.anonymous.burst: must be at least 1, got 0",
		"ratelimit.auth_failure.per_minute: must be at least 1, got 0",
		"ratelimit.auth_failure.burst: must be at least 1, got 0",
		"ratelimit.authenticated.per_minute: must be at least 1, got 0",
		"ratelimit.authenticated.burst: must be at least 1, got 0",
		"ratelimit.login_ip.per_minute: must be at least 1, got 0",
		"ratelimit.login_ip.burst: must be at least 1, got 0",
		"ratelimit.login_ip_email.per_minute: must be at least 1, got 0",
		"ratelimit.login_ip_email.burst: must be at least 1, got 0",
		"ratelimit.register_ip.per_minute: must be at least 1, got 0",
		"ratelimit.register_ip.burst: must be at least 1, got 0",
		"ratelimit.password_user.per_minute: must be at least 1, got 0",
		"ratelimit.password_user.burst: must be at least 1, got 0",
		"page.edit_session_cleanup_interval: must be at least 1s, got 0s",
		"page.parse_budget_bytes: must be at least 5242880, a page's largest content, got 0",
		"page.parse_max_wait: must be positive, got 0s",
		"jobs.shutdown_timeout: must be positive, got 0s",
		"jobs.purge_interval: must be at least 1s, got 0s",
		"jobs.purge_retention: must be at least 1h, got 0s",
		`log.level: must be one of debug, info, warn, error, got "verbose"`,
		`log.format: must be text or json, got "xml"`,
	}
	if got := strings.Split(err.Error(), "\n"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("validate() =\n%s\nwant\n%s", err, strings.Join(want, "\n"))
	}
}

func TestValidateCrossKeyRules(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string // "" means valid
	}{
		{
			name:   "headers may take the whole read budget",
			mutate: func(c *Config) { c.Server.ReadHeaderTimeout = c.Server.ReadTimeout },
		},
		{
			name:   "headers may not outlast the whole request",
			mutate: func(c *Config) { c.Server.ReadHeaderTimeout = c.Server.ReadTimeout + time.Second },
			want:   "server.read_header_timeout: must not exceed server.read_timeout (30s), got 31s",
		},
		{
			name: "a request may take less than the write budget",
			mutate: func(c *Config) {
				c.Server.ReadHeaderTimeout, c.Server.ReadTimeout = time.Second, time.Second
				c.Server.RequestTimeout = c.Server.WriteTimeout - 2*time.Second
			},
		},
		{
			name:   "a content's body and its request within the write budget",
			mutate: func(c *Config) { c.Server.RequestTimeout = c.Server.WriteTimeout - c.Server.ReadTimeout - time.Second },
		},
		{
			name:   "a content's body and its request beyond the write budget",
			mutate: func(c *Config) { c.Server.RequestTimeout = c.Server.WriteTimeout - c.Server.ReadTimeout },
			want:   "server.request_timeout: plus server.read_timeout (30s) must be less than server.write_timeout (1m0s), got 30s",
		},
		{
			name:   "a request must leave time to write its answer",
			mutate: func(c *Config) { c.Server.RequestTimeout = c.Server.WriteTimeout },
			want:   "server.request_timeout: must be less than server.write_timeout (1m0s), got 1m0s",
		},
		{
			name:   "a host is optional in the address",
			mutate: func(c *Config) { c.Server.Addr = "127.0.0.1:0" },
		},
		{
			name:   "a session must outlive its access tokens",
			mutate: func(c *Config) { c.Auth.SessionTTL = c.Auth.AccessTokenTTL },
			want:   "auth.session_ttl: must be longer than auth.access_token_ttl (15m0s), got 15m0s",
		},
		{
			name:   "argon2 needs 8 KiB per lane",
			mutate: func(c *Config) { c.Auth.Password.Argon2Parallelism, c.Auth.Password.Argon2MemoryKiB = 4, 31 },
			want:   "auth.password.argon2_memory_kib: must be at least 8 per lane (32), got 31",
		},
		{
			name:   "argon2 at 8 KiB per lane",
			mutate: func(c *Config) { c.Auth.Password.Argon2Parallelism, c.Auth.Password.Argon2MemoryKiB = 4, 32 },
		},
		{
			name:   "dev and test may sign with an ephemeral key",
			mutate: func(c *Config) { c.Env, c.Auth.JWT.PrivateKeyFile = EnvDev, "" },
		},
		{
			name: "a refresh ends before the web client gives up",
			mutate: func(c *Config) {
				c.Auth.RefreshDeadline, c.Database.CommitTimeout = 5*time.Second, 3*time.Second-time.Millisecond
			},
		},
		{
			name: "a refresh that the web client may give up on",
			mutate: func(c *Config) {
				c.Auth.RefreshDeadline, c.Database.CommitTimeout = 5*time.Second, 3*time.Second
			},
			want: "auth.refresh_deadline: plus database.commit_timeout (3s) must be less than 8s, the web client's refresh timeout, got 5s",
		},
		{
			name:   "a refresh deadline as long as the request timeout",
			mutate: func(c *Config) { c.Auth.RefreshDeadline, c.Server.RequestTimeout = 4*time.Second, 4*time.Second },
		},
		{
			name:   "a refresh deadline beyond the request timeout",
			mutate: func(c *Config) { c.Auth.RefreshDeadline, c.Server.RequestTimeout = 4*time.Second, 3*time.Second },
			want:   "auth.refresh_deadline: must be at most server.request_timeout (3s), got 4s",
		},
		{
			name:   "a parse waits less than its request may take",
			mutate: func(c *Config) { c.Page.ParseMaxWait = c.Server.RequestTimeout - time.Millisecond },
		},
		{
			name:   "a parse that waits as long as its request may take",
			mutate: func(c *Config) { c.Page.ParseMaxWait = c.Server.RequestTimeout },
			want:   "page.parse_max_wait: must be less than server.request_timeout (15s), got 15s",
		},
		{
			name:   "an IPv6 prefix of a single address",
			mutate: func(c *Config) { c.RateLimit.IPv6PrefixLen = 128 },
		},
		{
			name:   "an IPv6 prefix longer than an address",
			mutate: func(c *Config) { c.RateLimit.IPv6PrefixLen = 129 },
			want:   "ratelimit.ipv6_prefix_len: must be from 1 to 128, got 129",
		},
		{
			name:   "trusted proxies",
			mutate: func(c *Config) { c.Server.TrustedProxies = prefixes("10.0.0.0/8", "fd00::/8", "192.0.2.7/32") },
		},
		{
			name:   "a proxy prefix that trusts everyone",
			mutate: func(c *Config) { c.Server.TrustedProxies = prefixes("10.0.0.0/8", "::/0") },
			want:   "server.trusted_proxies: ::/0 trusts every address, so any client could choose its own IP; list only your proxies' addresses",
		},
		{
			name:   "an IPv4-mapped proxy prefix",
			mutate: func(c *Config) { c.Server.TrustedProxies = prefixes("::ffff:10.0.0.0/104") },
			want:   "server.trusted_proxies: ::ffff:10.0.0.0/104 is an IPv4-mapped IPv6 prefix, which no address matches: write the IPv4 prefix",
		},
		{
			name:   "an edit sessions' cleanup every second",
			mutate: func(c *Config) { c.Page.EditSessionCleanupInterval = time.Second },
		},
		{
			name:   "an edit sessions' cleanup more often",
			mutate: func(c *Config) { c.Page.EditSessionCleanupInterval = time.Second - time.Millisecond },
			want:   "page.edit_session_cleanup_interval: must be at least 1s, got 999ms",
		},
		{
			name:   "a parse budget of one largest content",
			mutate: func(c *Config) { c.Page.ParseBudgetBytes = 5 << 20 },
		},
		{
			name:   "a parse budget smaller than the largest content",
			mutate: func(c *Config) { c.Page.ParseBudgetBytes = 5<<20 - 1 },
			want:   "page.parse_budget_bytes: must be at least 5242880, a page's largest content, got 5242879",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)
			err := cfg.validate()
			switch {
			case tt.want == "" && err != nil:
				t.Errorf("validate() = %v, want nil", err)
			case tt.want != "" && (err == nil || err.Error() != tt.want):
				t.Errorf("validate() = %v, want %q", err, tt.want)
			}
		})
	}
}

func prefixes(ss ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}
