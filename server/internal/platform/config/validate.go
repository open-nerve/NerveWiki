package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
)

// validate reports every invalid key at once, one "key: problem" line each.
func (c Config) validate() error {
	var errs []error
	fail := func(key, format string, args ...any) {
		errs = append(errs, fmt.Errorf("%s: "+format, append([]any{key}, args...)...))
	}

	if _, _, err := net.SplitHostPort(c.Server.Addr); err != nil {
		fail("server.addr", "must be host:port, e.g. \":8080\", got %q", c.Server.Addr)
	}
	switch {
	case c.Server.ReadHeaderTimeout <= 0:
		fail("server.read_header_timeout", "must be positive, got %s", c.Server.ReadHeaderTimeout)
	case c.Server.ReadTimeout > 0 && c.Server.ReadHeaderTimeout > c.Server.ReadTimeout:
		// read_timeout is the budget for the whole request: a longer header limit
		// would let the headers alone outlast it and leave the body no time.
		fail("server.read_header_timeout", "must not exceed server.read_timeout (%s), got %s", c.Server.ReadTimeout, c.Server.ReadHeaderTimeout)
	}
	if c.Server.ReadTimeout <= 0 {
		fail("server.read_timeout", "must be positive, got %s", c.Server.ReadTimeout)
	}
	if c.Server.WriteTimeout <= 0 {
		fail("server.write_timeout", "must be positive, got %s", c.Server.WriteTimeout)
	}
	if c.Server.ShutdownTimeout <= 0 {
		fail("server.shutdown_timeout", "must be positive, got %s", c.Server.ShutdownTimeout)
	}
	switch {
	case c.Server.RequestTimeout <= 0:
		fail("server.request_timeout", "must be positive, got %s", c.Server.RequestTimeout)
	case c.Server.WriteTimeout > 0 && c.Server.RequestTimeout >= c.Server.WriteTimeout:
		// A request that runs into its deadline still has to write its error
		// response before write_timeout cuts the connection.
		fail("server.request_timeout", "must be less than server.write_timeout (%s), got %s", c.Server.WriteTimeout, c.Server.RequestTimeout)
	}
	if c.Server.MaxBodyBytes < 1 {
		fail("server.max_body_bytes", "must be at least 1, got %d", c.Server.MaxBodyBytes)
	}
	if c.Database.URL == "" {
		fail("database.url", "is required")
	}
	if c.Database.MaxConns < 1 {
		fail("database.max_conns", "must be at least 1, got %d", c.Database.MaxConns)
	}
	if c.Database.CommitTimeout <= 0 {
		fail("database.commit_timeout", "must be positive, got %s", c.Database.CommitTimeout)
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.Log.Level)); err != nil {
		fail("log.level", "must be one of debug, info, warn, error, got %q", c.Log.Level)
	}
	if c.Log.Format != "text" && c.Log.Format != "json" {
		fail("log.format", "must be text or json, got %q", c.Log.Format)
	}
	return errors.Join(errs...)
}
