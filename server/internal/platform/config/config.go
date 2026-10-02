// Package config loads, validates and describes nervewiki's configuration.
package config

import (
	"log/slog"
	"net/netip"
	"strings"
	"time"
)

// Profiles, selected with NWIKI_ENV.
const (
	EnvDev  = "dev"
	EnvTest = "test"
	EnvProd = "prod"
)

// Config is the effective configuration of a nervewiki process.
type Config struct {
	// Env is the profile the configuration was loaded for. It comes from
	// NWIKI_ENV and is not a configuration key.
	Env       string          `koanf:"-"`
	Server    ServerConfig    `koanf:"server"`
	Database  DatabaseConfig  `koanf:"database"`
	Auth      AuthConfig      `koanf:"auth"`
	RateLimit RateLimitConfig `koanf:"ratelimit"`
	Workspace WorkspaceConfig `koanf:"workspace"`
	Page      PageConfig      `koanf:"page"`
	Jobs      JobsConfig      `koanf:"jobs"`
	Log       LogConfig       `koanf:"log"`
}

// ServerConfig configures the HTTP server. The timeouts bound the reads and
// writes on a connection: reading the request headers, reading the whole
// request (headers and body), and writing the response; idle keep-alive
// connections have a fixed timeout in httpserver. RequestTimeout bounds each
// API operation's context, which write_timeout does not cancel.
type ServerConfig struct {
	Addr              string        `koanf:"addr"`
	ReadHeaderTimeout time.Duration `koanf:"read_header_timeout"`
	ReadTimeout       time.Duration `koanf:"read_timeout"`
	WriteTimeout      time.Duration `koanf:"write_timeout"`
	ShutdownTimeout   time.Duration `koanf:"shutdown_timeout"`
	RequestTimeout    time.Duration `koanf:"request_timeout"`
	MaxBodyBytes      int64         `koanf:"max_body_bytes"`
	// AddrFile, when set, receives the address the server listens on once it
	// does, e.g. for addr ":0".
	AddrFile string `koanf:"addr_file"`
	// TrustedProxies are the reverse proxies whose X-Forwarded-For names the
	// client (M1/P1 design 3.5). Empty: the client is the connection's peer.
	TrustedProxies []netip.Prefix `koanf:"trusted_proxies"`
}

// DatabaseConfig configures the PostgreSQL pool and schema migrations.
type DatabaseConfig struct {
	URL         string `koanf:"url"`
	MaxConns    int32  `koanf:"max_conns"`
	AutoMigrate bool   `koanf:"auto_migrate"`
	// CommitTimeout bounds COMMIT and ROLLBACK, which the request deadline
	// does not cancel.
	CommitTimeout time.Duration `koanf:"commit_timeout"`
}

// AuthConfig configures accounts and credentials (M1/P1 design 3.6, M1/P2
// design 3.7).
type AuthConfig struct {
	// SignupEnabled opens registration to anyone who reaches the server.
	SignupEnabled  bool          `koanf:"signup_enabled"`
	AccessTokenTTL time.Duration `koanf:"access_token_ttl"`
	SessionTTL     time.Duration `koanf:"session_ttl"`
	// RefreshDeadline bounds a refresh or a logout; with
	// database.commit_timeout it must end before the web client gives up on
	// a refresh (M1/P2 design 3.5).
	RefreshDeadline time.Duration `koanf:"refresh_deadline"`
	// SessionCleanupInterval is how often the expired sessions are deleted
	// (M1/P4 design 3.5).
	SessionCleanupInterval time.Duration  `koanf:"session_cleanup_interval"`
	JWT                    JWTConfig      `koanf:"jwt"`
	Password               PasswordConfig `koanf:"password"`
}

// JWTConfig locates the Ed25519 signing key.
type JWTConfig struct {
	// PrivateKeyFile is a PKCS#8 PEM Ed25519 private key. Required in prod;
	// empty elsewhere means an ephemeral key generated at startup.
	PrivateKeyFile string `koanf:"private_key_file"`
}

// PasswordConfig configures argon2id and how many hashes run at once.
type PasswordConfig struct {
	Argon2MemoryKiB     uint32        `koanf:"argon2_memory_kib"`
	Argon2Iterations    uint32        `koanf:"argon2_iterations"`
	Argon2Parallelism   uint8         `koanf:"argon2_parallelism"`
	MaxConcurrentHashes int           `koanf:"max_concurrent_hashes"`
	MaxWait             time.Duration `koanf:"max_wait"`
}

// RateLimitConfig sizes the rate-limit buckets (M1/P2 design 3.2).
type RateLimitConfig struct {
	// IPv6PrefixLen is how much of an IPv6 client address the per-IP buckets
	// count by: a host usually holds a whole /64.
	IPv6PrefixLen int `koanf:"ipv6_prefix_len"`
	// Anonymous limits the public operations, by client IP.
	Anonymous BucketConfig `koanf:"anonymous"`
	// AuthFailure is the gate before authentication: requests whose token
	// fails, by client IP.
	AuthFailure BucketConfig `koanf:"auth_failure"`
	// Authenticated limits the operations that need a token, by credential.
	Authenticated BucketConfig `koanf:"authenticated"`
	// LoginIP and LoginIPEmail limit sign-in by client IP, and by client IP
	// and address; RegisterIP limits sign-up by client IP.
	LoginIP      BucketConfig `koanf:"login_ip"`
	LoginIPEmail BucketConfig `koanf:"login_ip_email"`
	RegisterIP   BucketConfig `koanf:"register_ip"`
	// PasswordUser limits the authenticated operations that verify the
	// current password (changing it, creating a token), by account.
	PasswordUser BucketConfig `koanf:"password_user"`
}

// BucketConfig is a token bucket: it holds at most Burst units and gains
// PerMinute units a minute.
type BucketConfig struct {
	PerMinute int `koanf:"per_minute"`
	Burst     int `koanf:"burst"`
}

// LogValue renders the bucket as its two settings.
func (b BucketConfig) LogValue() slog.Value {
	return slog.GroupValue(slog.Int("per_minute", b.PerMinute), slog.Int("burst", b.Burst))
}

// WorkspaceConfig configures the workspaces (M2/P1 design 3.8).
type WorkspaceConfig struct {
	// CreationEnabled lets every account create workspaces; when off, the
	// server's administrator creates them (nervewiki workspaces create).
	CreationEnabled bool `koanf:"creation_enabled"`
}

// PageConfig configures the pages (M4/P4 design 3.5).
type PageConfig struct {
	// EditSessionCleanupInterval is how often the edit sessions expired
	// and not ended are deleted.
	EditSessionCleanupInterval time.Duration `koanf:"edit_session_cleanup_interval"`
	// ParseBudgetBytes is the bytes of content parsed and rendered at once
	// (M4/P4 review P2): the largest content's parse can hold some 300
	// times its size in memory. At least MinParseBudgetBytes.
	ParseBudgetBytes int `koanf:"parse_budget_bytes"`
	// ParseMaxWait is how long a request waits for its share of the budget
	// before it is answered 503 server_busy.
	ParseMaxWait time.Duration `koanf:"parse_max_wait"`
}

// MinParseBudgetBytes is the smallest parse budget: one page's largest
// content, the page module's domain.MaxContentBytes, which bootstrap's
// test checks it against.
const MinParseBudgetBytes = 5 << 20

// JobsConfig configures the background jobs (M1/P4 design 3.3).
type JobsConfig struct {
	// ShutdownTimeout is how long a stop lets the running jobs finish
	// before it cancels them.
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
	// PurgeInterval is how often the purge runs (M2/P4 design 3.4).
	PurgeInterval time.Duration `koanf:"purge_interval"`
	// PurgeRetention is how long a soft-deleted row is kept before the
	// purge deletes it (v0.1 design 7.1).
	PurgeRetention time.Duration `koanf:"purge_retention"`
}

// LogConfig configures the process logger.
type LogConfig struct {
	Level  string `koanf:"level"`  // debug, info, warn or error
	Format string `koanf:"format"` // text or json
}

// LogValue renders the configuration for logs with secrets masked, so the
// effective configuration can be logged at startup. Only the keys listed here
// reach the log; a secret's *_file key logs whether it is set, never its
// path. Durations are strings such as "5s", in JSON logs too.
func (c Config) LogValue() slog.Value {
	proxies := make([]string, len(c.Server.TrustedProxies))
	for i, p := range c.Server.TrustedProxies {
		proxies[i] = p.String()
	}
	return slog.GroupValue(
		slog.String("env", c.Env),
		slog.Group("server",
			slog.String("addr", c.Server.Addr),
			duration("read_header_timeout", c.Server.ReadHeaderTimeout),
			duration("read_timeout", c.Server.ReadTimeout),
			duration("write_timeout", c.Server.WriteTimeout),
			duration("shutdown_timeout", c.Server.ShutdownTimeout),
			duration("request_timeout", c.Server.RequestTimeout),
			slog.Int64("max_body_bytes", c.Server.MaxBodyBytes),
			slog.String("addr_file", c.Server.AddrFile),
			slog.String("trusted_proxies", strings.Join(proxies, ",")),
		),
		slog.Any("database", c.Database),
		slog.Group("auth",
			slog.Bool("signup_enabled", c.Auth.SignupEnabled),
			duration("access_token_ttl", c.Auth.AccessTokenTTL),
			duration("session_ttl", c.Auth.SessionTTL),
			duration("refresh_deadline", c.Auth.RefreshDeadline),
			duration("session_cleanup_interval", c.Auth.SessionCleanupInterval),
			slog.Group("jwt",
				slog.Bool("private_key_file_set", c.Auth.JWT.PrivateKeyFile != ""),
			),
			slog.Group("password",
				slog.Uint64("argon2_memory_kib", uint64(c.Auth.Password.Argon2MemoryKiB)),
				slog.Uint64("argon2_iterations", uint64(c.Auth.Password.Argon2Iterations)),
				slog.Uint64("argon2_parallelism", uint64(c.Auth.Password.Argon2Parallelism)),
				slog.Int("max_concurrent_hashes", c.Auth.Password.MaxConcurrentHashes),
				duration("max_wait", c.Auth.Password.MaxWait),
			),
		),
		slog.Group("ratelimit",
			slog.Int("ipv6_prefix_len", c.RateLimit.IPv6PrefixLen),
			slog.Any("anonymous", c.RateLimit.Anonymous),
			slog.Any("auth_failure", c.RateLimit.AuthFailure),
			slog.Any("authenticated", c.RateLimit.Authenticated),
			slog.Any("login_ip", c.RateLimit.LoginIP),
			slog.Any("login_ip_email", c.RateLimit.LoginIPEmail),
			slog.Any("register_ip", c.RateLimit.RegisterIP),
			slog.Any("password_user", c.RateLimit.PasswordUser),
		),
		slog.Group("workspace",
			slog.Bool("creation_enabled", c.Workspace.CreationEnabled),
		),
		slog.Group("page",
			duration("edit_session_cleanup_interval", c.Page.EditSessionCleanupInterval),
			slog.Int("parse_budget_bytes", c.Page.ParseBudgetBytes),
			duration("parse_max_wait", c.Page.ParseMaxWait),
		),
		slog.Group("jobs",
			duration("shutdown_timeout", c.Jobs.ShutdownTimeout),
			duration("purge_interval", c.Jobs.PurgeInterval),
			duration("purge_retention", c.Jobs.PurgeRetention),
		),
		slog.Group("log",
			slog.String("level", c.Log.Level),
			slog.String("format", c.Log.Format),
		),
	)
}

// redacted stands in for a secret in log output.
const redacted = "xxxxx"

// LogValue renders the database settings with the URL masked as a whole, so
// they are safe to log on their own too. pgx parses the URL with its own
// libpq-compatible grammar, which accepts forms that other parsers read
// differently, so masking only the password another parser finds could leak
// the rest; log the target from the parsed pool configuration instead.
func (d DatabaseConfig) LogValue() slog.Value {
	url := ""
	if d.URL != "" {
		url = redacted
	}
	return slog.GroupValue(
		slog.String("url", url),
		slog.Int("max_conns", int(d.MaxConns)),
		slog.Bool("auto_migrate", d.AutoMigrate),
		duration("commit_timeout", d.CommitTimeout),
	)
}

// duration renders d as "5s" rather than slog's nanoseconds in JSON.
func duration(key string, d time.Duration) slog.Attr {
	return slog.String(key, d.String())
}
