package configs_test

import (
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/configs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

func TestBuiltInProfiles(t *testing.T) {
	const devURL = "postgres://nervewiki:nervewiki@localhost:55433/nervewiki?sslmode=disable"
	tests := []struct {
		env         string
		addr        string // expected server.addr
		url         string // expected database.url
		autoMigrate bool
		signup      bool
		argon2      config.PasswordConfig
		limits      config.RateLimitConfig
		keyFile     string
		cleanup     time.Duration
		purge       time.Duration // expected jobs.purge_interval
		level       string
		format      string
	}{
		{env: "dev", addr: "127.0.0.1:8080", url: devURL, autoMigrate: true, signup: true, argon2: owasp(), limits: defaultLimits(), cleanup: time.Hour, purge: time.Hour, level: "debug", format: "text"},
		{env: "test", addr: ":8080", url: "postgres://from-env", autoMigrate: true, signup: true, argon2: cheap(), limits: unlimited(), cleanup: 2 * time.Second, purge: 2 * time.Second, level: "warn", format: "text"},
		{env: "prod", addr: ":8080", url: "postgres://from-env", autoMigrate: false, signup: false, argon2: owasp(), limits: defaultLimits(), keyFile: "/run/secrets/jwt.pem", cleanup: time.Hour, purge: time.Hour, level: "info", format: "json"},
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			environ := []string{"NWIKI_ENV=" + tt.env}
			if tt.env != "dev" {
				environ = append(environ, "NWIKI_DATABASE__URL=postgres://from-env")
			}
			if tt.keyFile != "" {
				environ = append(environ, "NWIKI_AUTH__JWT__PRIVATE_KEY_FILE="+tt.keyFile)
			}
			cfg, err := config.Load(config.Sources{Embedded: configs.FS(), Environ: environ})
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			want := config.Config{
				Env: tt.env,
				Server: config.ServerConfig{
					Addr:              tt.addr,
					ReadHeaderTimeout: 5 * time.Second,
					ReadTimeout:       30 * time.Second,
					WriteTimeout:      60 * time.Second,
					ShutdownTimeout:   20 * time.Second,
					RequestTimeout:    15 * time.Second,
					MaxBodyBytes:      1 << 20,
					TrustedProxies:    []netip.Prefix{},
				},
				Database: config.DatabaseConfig{URL: tt.url, MaxConns: 10, AutoMigrate: tt.autoMigrate, CommitTimeout: 2 * time.Second},
				Auth: config.AuthConfig{
					SignupEnabled:          tt.signup,
					AccessTokenTTL:         15 * time.Minute,
					SessionTTL:             30 * 24 * time.Hour,
					RefreshDeadline:        4 * time.Second,
					SessionCleanupInterval: tt.cleanup,
					JWT:                    config.JWTConfig{PrivateKeyFile: tt.keyFile},
					Password:               tt.argon2,
				},
				RateLimit: tt.limits,
				Workspace: config.WorkspaceConfig{CreationEnabled: true},
				Page:      config.PageConfig{EditSessionCleanupInterval: 10 * time.Minute, ParseBudgetBytes: 8 << 20, ParseMaxWait: 2 * time.Second},
				Events:    config.EventsConfig{HeartbeatInterval: 20 * time.Second},
				Jobs:      config.JobsConfig{ShutdownTimeout: 10 * time.Second, PurgeInterval: tt.purge, PurgeRetention: 60 * 24 * time.Hour},
				Log:       config.LogConfig{Level: tt.level, Format: tt.format},
			}
			if !reflect.DeepEqual(cfg, want) {
				t.Errorf("Load() =\n%+v\nwant\n%+v", cfg, want)
			}
		})
	}
}

// owasp is OWASP's minimum for argon2id, the default.
func owasp() config.PasswordConfig {
	return config.PasswordConfig{Argon2MemoryKiB: 19456, Argon2Iterations: 2, Argon2Parallelism: 1, MaxConcurrentHashes: 4, MaxWait: 2 * time.Second}
}

// cheap is the test profile's argon2id: tests register many accounts.
func cheap() config.PasswordConfig {
	p := owasp()
	p.Argon2MemoryKiB, p.Argon2Iterations = 64, 1
	return p
}

// defaultLimits are the buckets of M1/P2 design 3.7.
func defaultLimits() config.RateLimitConfig {
	return config.RateLimitConfig{
		IPv6PrefixLen: 64,
		Anonymous:     config.BucketConfig{PerMinute: 600, Burst: 100},
		AuthFailure:   config.BucketConfig{PerMinute: 60, Burst: 60},
		Authenticated: config.BucketConfig{PerMinute: 1200, Burst: 200},
		LoginIP:       config.BucketConfig{PerMinute: 30, Burst: 10},
		LoginIPEmail:  config.BucketConfig{PerMinute: 10, Burst: 5},
		RegisterIP:    config.BucketConfig{PerMinute: 10, Burst: 5},
		PasswordUser:  config.BucketConfig{PerMinute: 5, Burst: 5},
	}
}

// unlimited is the test profile's buckets: tests sign up and in a lot.
func unlimited() config.RateLimitConfig {
	huge := config.BucketConfig{PerMinute: 600000, Burst: 100000}
	return config.RateLimitConfig{
		IPv6PrefixLen: 64, Anonymous: huge, AuthFailure: huge, Authenticated: huge,
		LoginIP: huge, LoginIPEmail: huge, RegisterIP: huge, PasswordUser: huge,
	}
}

// prod has no ephemeral signing key: a restart would sign every user out.
func TestProdRequiresASigningKey(t *testing.T) {
	_, err := config.Load(config.Sources{Embedded: configs.FS(), Environ: []string{"NWIKI_ENV=prod", "NWIKI_DATABASE__URL=postgres://x"}})
	if err == nil || !strings.Contains(err.Error(), "auth.jwt.private_key_file: is required in prod") {
		t.Fatalf("Load() error = %v, want auth.jwt.private_key_file to be required", err)
	}
}

func TestProdRequiresDatabaseURL(t *testing.T) {
	_, err := config.Load(config.Sources{Embedded: configs.FS(), Environ: []string{"NWIKI_ENV=prod"}})
	if err == nil || !strings.Contains(err.Error(), "database.url: is required") {
		t.Fatalf("Load() error = %v, want database.url to be required", err)
	}
}
