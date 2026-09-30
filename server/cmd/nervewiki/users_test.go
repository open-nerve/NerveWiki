package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	argon2adapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/argon2"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// usersDatabase is a migrated database of its own, the environment that
// points nervewiki at it, and a pool on it for the assertions. The
// environment sets auth.password's iterations apart from the test
// profile's, so that the stored hash shows the configuration reached the
// hasher.
func usersDatabase(t *testing.T) ([]string, *pgxpool.Pool) {
	t.Helper()
	url := pgtest.NewDatabase(t)
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=" + url, "NWIKI_AUTH__PASSWORD__ARGON2_ITERATIONS=2"}, pool
}

// storedHash is the password hash of the account with email.
func storedHash(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var hash string
	if err := pool.QueryRow(context.Background(), "SELECT password FROM users WHERE email = $1", email).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	return hash
}

// hasPassword reports whether the account with email has password, hashed
// with auth.password's argon2 parameters: the test profile's memory and
// parallelism, and the iterations usersDatabase sets.
func hasPassword(t *testing.T, pool *pgxpool.Pool, email, password string) bool {
	t.Helper()
	hash := storedHash(t, pool, email)
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=64,t=2,p=1$") {
		t.Errorf("hash %s, want auth.password's parameters m=64,t=2,p=1", hash)
	}
	hasher := argon2adapter.New(argon2adapter.Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, MaxConcurrent: 1, MaxWait: time.Second},
		slog.New(slog.DiscardHandler))
	ok, _, err := hasher.Verify(context.Background(), password, hash)
	return err == nil && ok
}

// Without a terminal, the password is one line of standard input: only its
// line ending, \n or \r\n, is removed, and a last line without one counts
// too (M1/P4 design 3.7). Spaces and a lone \r are part of the password.
func TestUsersReadThePasswordFromStandardInput(t *testing.T) {
	environ, pool := usersDatabase(t)
	tests := []struct {
		email, input, password string
	}{
		{"ida@corp.com", "Tr0ub4dor&3\n", "Tr0ub4dor&3"},
		{"jan@corp.com", "Pass word1!\r\nignored\n", "Pass word1!"},
		{"kim@corp.com", " Tr0ub4dor&3 ", " Tr0ub4dor&3 "},
		{"lou@corp.com", "Tr0ub4dor&3\r", "Tr0ub4dor&3\r"},
		{"max@corp.com", "Tr0ub4dor&3\r\r\n", "Tr0ub4dor&3\r"},
	}
	for _, tt := range tests {
		code, stdout, stderr := executeWithInput(context.Background(), environ, tt.input, "users", "create", "--email", tt.email)
		if code != 0 || !strings.HasPrefix(stdout, "created "+tt.email+" (") || !hasPassword(t, pool, tt.email, tt.password) {
			t.Errorf("create %s from %q = %d %q (stderr %q); want the account with password %q", tt.email, tt.input, code, stdout, stderr, tt.password)
		}
	}

	code, stdout, _ := executeWithInput(context.Background(), environ, "N3w-Passw0rd!\n", "users", "reset-password", "--email", "ida@corp.com")
	if code != 0 || stdout != "password reset for ida@corp.com: revoked 0 sessions, 0 API tokens\n" || !hasPassword(t, pool, "ida@corp.com", "N3w-Passw0rd!") {
		t.Errorf("reset-password = %d %q, want ida's new password", code, stdout)
	}
}

// The commands that set no password do not read standard input: empty
// input is no error for them. Each runs its own use case and prints its
// line.
func TestUsersCommandsWithoutAPassword(t *testing.T) {
	environ, _ := usersDatabase(t)
	if code, _, stderr := executeWithInput(context.Background(), environ, "Tr0ub4dor&3\n", "users", "create", "--email", "lee@corp.com"); code != 0 {
		t.Fatalf("create = %d: %s", code, stderr)
	}
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"users", "deactivate", "--email", "lee@corp.com"}, "deactivated lee@corp.com: revoked 0 sessions\n"},
		{[]string{"users", "activate", "--email", "lee@corp.com"}, "activated lee@corp.com: 0 API tokens are usable again\n"},
		{[]string{"users", "set-email", "--email", "lee@corp.com", "--new-email", "lee@new.example"},
			"e-mail changed to lee@new.example: revoked 0 sessions\n"},
	} {
		if code, stdout, stderr := execute(context.Background(), environ, tt.args...); code != 0 || stdout != tt.want {
			t.Errorf("nervewiki %s = %d %q (stderr %q), want 0 and %q", strings.Join(tt.args, " "), code, stdout, stderr, tt.want)
		}
	}
}

// A refused command exits 1 with one line on stderr and nothing on stdout.
func TestUsersCommandsFail(t *testing.T) {
	environ, _ := usersDatabase(t)
	if code, _, stderr := executeWithInput(context.Background(), environ, "Tr0ub4dor&3\n", "users", "create", "--email", "nia@corp.com"); code != 0 {
		t.Fatalf("create = %d: %s", code, stderr)
	}
	unreachable := []string{"NWIKI_ENV=test", "NWIKI_DATABASE__URL=postgres://nervewiki@127.0.0.1:1/nervewiki?connect_timeout=1"}
	tests := []struct {
		name    string
		environ []string
		input   string
		args    []string
		want    string
	}{
		{"no password", environ, "", []string{"users", "create", "--email", "may@corp.com"}, "nervewiki: read the password from standard input: EOF\n"},
		{"an empty password", environ, "\n", []string{"users", "create", "--email", "may@corp.com"}, "nervewiki: the password is required\n"},
		{"no address", environ, "Tr0ub4dor&3\n", []string{"users", "create"}, "nervewiki: required flag(s) \"email\" not set\n"},
		{"no address to change", environ, "", []string{"users", "set-email", "--new-email", "may@new.example"}, "nervewiki: required flag(s) \"email\" not set\n"},
		{"no new address", environ, "", []string{"users", "set-email", "--email", "may@corp.com"}, "nervewiki: required flag(s) \"new-email\" not set\n"},
		{"the same address", environ, "", []string{"users", "set-email", "--email", "nia@corp.com", "--new-email", "NIA@corp.com"},
			"nervewiki: The account has this e-mail address already.\n"},
		{"an unknown account", environ, "", []string{"users", "activate", "--email", "may@corp.com"}, "nervewiki: The account does not exist.\n"},
		{"an unknown command", environ, "", []string{"users", "delete"}, "nervewiki: unknown command \"delete\" for \"nervewiki users\"\n"},
		{"an argument", environ, "", []string{"users", "activate", "--email", "nia@corp.com", "now"}, "nervewiki: unknown command \"now\" for \"nervewiki users activate\"\n"},
		{"an unreachable database", unreachable, "N3w-Passw0rd!\n", []string{"users", "reset-password", "--email", "nia@corp.com"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := executeWithInput(context.Background(), tt.environ, tt.input, tt.args...)
			if code != 1 || stdout != "" || !strings.HasSuffix(stderr, tt.want) || strings.Count(stderr, "nervewiki: ") != 1 {
				t.Errorf("nervewiki %s = %d, stdout %q, stderr %q; want 1 and %q", strings.Join(tt.args, " "), code, stdout, stderr, tt.want)
			}
			if tt.want == "" && !strings.HasPrefix(stderr, "nervewiki: database unreachable (waited up to 10s): ") {
				t.Errorf("stderr %q, want the database unreachable", stderr)
			}
		})
	}
}

// failingReader fails the test when read.
type failingReader struct{ t *testing.T }

func (r failingReader) Read([]byte) (int, error) {
	r.t.Error("standard input was read")
	return 0, errors.New("read")
}

// An invalid configuration is reported before the password is asked for:
// the administrator does not type it for nothing.
func TestUsersReportAnInvalidConfigurationFirst(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run(context.Background(), []string{"users", "create", "--email", "may@corp.com"}, []string{"NWIKI_ENV=test"},
		failingReader{t}, &stdout, &stderr)

	if code != 1 || stderr.String() != "nervewiki: invalid configuration:\ndatabase.url: is required\n" {
		t.Errorf("nervewiki users create = %d %q, want 1 and the invalid key", code, stderr.String())
	}
}

// Neither the output nor the logs, at every level, hold a password, a
// stored hash or the hash's derived key, in any spelling a log would give
// them (M1/P4 design 3.7). The logs hold no e-mail address either: it goes
// to the administrator's own output only.
func TestUsersPrintAndLogNoSecret(t *testing.T) {
	environ, pool := usersDatabase(t)
	environ = append(environ, "NWIKI_LOG__LEVEL=debug")
	var printed, logs strings.Builder
	secrets := map[string][]byte{}
	for _, step := range []struct {
		password string
		args     []string
		logged   string
	}{
		{"Tr0ub4dor&3", []string{"users", "create", "--email", "oli@corp.com"}, `msg="account created"`},
		{"N3w-Passw0rd!", []string{"users", "reset-password", "--email", "oli@corp.com"}, `msg="password reset"`},
	} {
		code, stdout, stderr := executeWithInput(context.Background(), environ, step.password+"\n", step.args...)
		if code != 0 || !strings.Contains(stderr, step.logged) {
			t.Fatalf("nervewiki %s = %d (stderr %q), want 0 and the log %s", strings.Join(step.args, " "), code, stderr, step.logged)
		}
		printed.WriteString(stdout + stderr)
		logs.WriteString(stderr)
		hash := storedHash(t, pool, "oli@corp.com")
		secrets["password "+step.password] = []byte(step.password)
		secrets["hash "+hash] = []byte(hash)
		secrets["derived key of "+hash] = derivedKey(t, hash)
	}
	for name, secret := range secrets {
		assertNoSecret(t, printed.String(), name, secret)
	}
	for _, args := range [][]string{
		{"users", "set-email", "--email", "oli@corp.com", "--new-email", "oli@new.example"},
		{"users", "deactivate", "--email", "oli@new.example"},
		{"users", "activate", "--email", "oli@new.example"},
	} {
		code, _, stderr := execute(context.Background(), environ, args...)
		if code != 0 {
			t.Fatalf("nervewiki %s = %d: %s", strings.Join(args, " "), code, stderr)
		}
		logs.WriteString(stderr)
	}
	if !strings.Contains(logs.String(), "user_id=") || strings.Contains(logs.String(), "oli@") {
		t.Errorf("the logs name an address or no account:\n%s", logs.String())
	}
}

// derivedKey is the key a PHC string holds, $argon2id$v=19$<params>$<salt>$<key>,
// as the raw bytes it encodes.
func derivedKey(t *testing.T, hash string) []byte {
	t.Helper()
	parts := strings.Split(hash, "$")
	key, err := base64.RawStdEncoding.DecodeString(parts[len(parts)-1])
	if len(parts) != 6 || err != nil || len(key) == 0 {
		t.Fatalf("hash %s holds no key (%v)", hash, err)
	}
	return key
}

// assertNoSecret fails when text holds secret: as is, escaped as slog's
// text handler writes bytes, in hex of either case, or in either base64
// alphabet. The unpadded spellings also find the padded ones.
func assertNoSecret(t *testing.T, text, name string, secret []byte) {
	t.Helper()
	for _, spelling := range []string{
		string(secret),
		strings.Trim(strconv.Quote(string(secret)), `"`),
		hex.EncodeToString(secret),
		strings.ToUpper(hex.EncodeToString(secret)),
		base64.RawStdEncoding.EncodeToString(secret),
		base64.RawURLEncoding.EncodeToString(secret),
	} {
		if strings.Contains(text, spelling) {
			t.Errorf("the output or the logs hold the %s as %q:\n%s", name, spelling, text)
		}
	}
}

func TestBareUsersPrintsHelp(t *testing.T) {
	code, stdout, stderr := execute(context.Background(), nil, "users")

	if code != 0 || !strings.Contains(stdout, "reset-password") || stderr != "" {
		t.Errorf("nervewiki users = %d, stdout %q, stderr %q; want 0 and the help", code, stdout, stderr)
	}
}
