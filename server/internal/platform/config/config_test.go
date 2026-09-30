package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLogValueMasksDatabaseURL(t *testing.T) {
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("configuration loaded", "config", validConfig())

	out := buf.String()
	if strings.Contains(out, "secret") {
		t.Errorf("log output leaks the password: %s", out)
	}
	for _, want := range []string{
		"config.env=test",
		"config.server.addr=:8080",
		"config.server.read_header_timeout=5s",
		"config.server.read_timeout=30s",
		"config.server.write_timeout=1m0s",
		"config.server.shutdown_timeout=20s",
		"config.server.addr_file_set=false",
		"config.database.url=xxxxx",
		"config.database.max_conns=10",
		"config.database.auto_migrate=true",
		"config.database.commit_timeout=2s",
		"config.log.level=info",
		"config.log.format=json",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log output lacks %q: %s", want, out)
		}
	}
}

// Every *_file key logs whether it is set, never the path: a path can tell
// where secrets live.
func TestLogValueHidesFilePaths(t *testing.T) {
	cfg := validConfig()
	cfg.Server.AddrFile = "/run/nervewiki/addr-secret-dir"
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("configuration loaded", "config", cfg)

	out := buf.String()
	if strings.Contains(out, "secret-dir") {
		t.Errorf("log output shows a file path: %s", out)
	}
	if !strings.Contains(out, "config.server.addr_file_set=true") {
		t.Errorf("log output lacks config.server.addr_file_set=true: %s", out)
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
