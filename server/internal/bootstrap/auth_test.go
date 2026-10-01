package bootstrap

import (
	"bytes"
	"context"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
)

// openSSLKey is an Ed25519 key in the format of openssl genpkey.
const openSSLKey = "-----BEGIN PRIVATE KEY-----\nMC4CAQAwBQYDK2VwBCIEIPneKoGsY0rpwLc94vhBW73igdoPaBAvGyjWyYTwGsUX\n-----END PRIVATE KEY-----\n"

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
	if err := os.WriteFile(path, []byte(openSSLKey), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t, unreachableDB, false)
	cfg.Auth.JWT.PrivateKeyFile = path

	buildApp(t, cfg, sampleMigrations())
}

// The invitations' MAC key is the one a signing key derives for them
// (M2/P3 design 3.2): a key no other use shares, and one that changing
// InvitationKeyInfo would change, ending every link pending as a new
// signing key does. So the key a known signing key derives is pinned;
// TestTokenKnownAnswer pins the token a key makes.
func TestTheInvitationKeyIsPinned(t *testing.T) {
	keys, err := identity.LoadSigningKeys([]byte(openSSLKey), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	const want = "c0783e2fa68061920382ee164bf763501d6d86756f892f96bb2881119619c6ee"
	if got := hex.EncodeToString(keys.Derive(workspace.InvitationKeyInfo)); got != want {
		t.Errorf("the invitations' key = %s, want %s", got, want)
	}
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
