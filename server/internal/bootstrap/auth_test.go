package bootstrap

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A signing key file that cannot be read or parsed stops startup with an
// error that names the key, never the file's path or content.
func TestABadSigningKeyStopsStartup(t *testing.T) {
	dir := t.TempDir()
	garbage := filepath.Join(dir, "secret-dir", "garbage.pem")
	if err := os.MkdirAll(filepath.Dir(garbage), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(garbage, []byte("secret-looking-garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(dir, "secret-dir", "missing.pem"), garbage} {
		cfg := testConfig(t, unreachableDB, false)
		cfg.Auth.JWT.PrivateKeyFile = path

		_, err := newApp(context.Background(), cfg, slog.New(slog.DiscardHandler), sampleMigrations(), testWebUI())

		if err == nil || !strings.HasPrefix(err.Error(), "auth.jwt.private_key_file: ") ||
			strings.Contains(err.Error(), "secret-") {
			t.Errorf("newApp() with %s = %v, want an auth.jwt.private_key_file error without the path or the content", filepath.Base(path), err)
		}
	}
}

// A key file in the format of openssl genpkey signs the tokens.
func TestAnOpenSSLSigningKeyStartsTheApp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jwt.pem")
	pem := "-----BEGIN PRIVATE KEY-----\nMC4CAQAwBQYDK2VwBCIEIPneKoGsY0rpwLc94vhBW73igdoPaBAvGyjWyYTwGsUX\n-----END PRIVATE KEY-----\n"
	if err := os.WriteFile(path, []byte(pem), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t, unreachableDB, false)
	cfg.Auth.JWT.PrivateKeyFile = path

	buildApp(t, cfg, sampleMigrations())
}

// A server that is not prod and listens beyond loopback warns once at
// startup: most likely a deployment that forgot NWIKI_ENV=prod.
func TestWarnIfExposed(t *testing.T) {
	tests := []struct {
		env, addr string
		warned    bool
	}{
		{"test", ":8080", true},
		{"dev", "0.0.0.0:8080", true},
		{"dev", "[::]:8080", true},
		{"dev", "127.0.0.1:8080", false},
		{"dev", "[::1]:8080", false},
		{"dev", "localhost:8080", false},
		{"prod", ":8080", false},
	}
	for _, tt := range tests {
		var logs bytes.Buffer
		cfg := testConfig(t, unreachableDB, false)
		cfg.Env, cfg.Server.Addr = tt.env, tt.addr

		warnIfExposed(context.Background(), slog.New(slog.NewJSONHandler(&logs, nil)), cfg)

		warned := strings.Contains(logs.String(), `"msg":"not running as prod but listening beyond this machine`)
		if warned != tt.warned || (warned && !strings.Contains(logs.String(), `"signup_enabled":true,"ephemeral_signing_key":true`)) {
			t.Errorf("%s on %s: logs %q, want warned %v with what is exposed", tt.env, tt.addr, logs.String(), tt.warned)
		}
	}
}
