// Package config loads, validates and describes nervewiki's configuration.
package config

import (
	"log/slog"
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
	Env      string         `koanf:"-"`
	Server   ServerConfig   `koanf:"server"`
	Database DatabaseConfig `koanf:"database"`
	Log      LogConfig      `koanf:"log"`
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

// LogConfig configures the process logger.
type LogConfig struct {
	Level  string `koanf:"level"`  // debug, info, warn or error
	Format string `koanf:"format"` // text or json
}

// LogValue renders the configuration for logs with secrets masked, so the
// effective configuration can be logged at startup. Only the keys listed here
// reach the log. Durations are strings such as "5s", in JSON logs too.
func (c Config) LogValue() slog.Value {
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
		),
		slog.Any("database", c.Database),
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
