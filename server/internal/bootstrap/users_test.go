package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// runUsers runs cmd on the database at url and returns its line, its logs
// and its error.
func runUsers(t *testing.T, url string, cmd UserCommand) (out, logs string, err error) {
	t.Helper()
	cfg := testConfig(t, url, false)
	cfg.Log.Level = "info"
	var stdout, stderr bytes.Buffer
	err = Users(context.Background(), cfg, &stderr, &stdout, cmd)
	return stdout.String(), stderr.String(), err
}

// createdAccount creates email through the command and returns its id.
func createdAccount(t *testing.T, url string, pool *pgxpool.Pool, email string) string {
	t.Helper()
	if _, _, err := runUsers(t, url, CreateUser(email, "Tr0ub4dor&3")); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := pool.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// accountState is what the commands change of an account: its address,
// whether it is active, its sessions by revoke reason ("live" for none) and
// its tokens, "live" or "revoked".
type accountState struct {
	email            string
	active           bool
	sessions, tokens string
}

func stateOf(t *testing.T, pool *pgxpool.Pool, id string) accountState {
	t.Helper()
	var s accountState
	err := pool.QueryRow(context.Background(), `SELECT u.email, u.is_active,
		coalesce((SELECT string_agg(coalesce(revoke_reason, 'live'), ',' ORDER BY coalesce(revoke_reason, 'live')) FROM auth_sessions WHERE user_id = u.id), ''),
		coalesce((SELECT string_agg(CASE WHEN revoked_at IS NULL THEN 'live' ELSE 'revoked' END, ',' ORDER BY revoked_at IS NULL) FROM api_tokens WHERE user_id = u.id), '')
		FROM users u WHERE u.id = $1`, id).Scan(&s.email, &s.active, &s.sessions, &s.tokens)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// addSessions gives the account id n live sessions.
func addSessions(t *testing.T, pool *pgxpool.Pool, id string, n int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO auth_sessions (id, user_id, token_hash, expires_at, created_at, updated_at)
		SELECT gen_random_uuid(), $1, sha256(gen_random_uuid()::text::bytea), now() + interval '1 hour', now(), now()
		FROM generate_series(1, $2)`, id, n); err != nil {
		t.Fatal(err)
	}
}

// addTokens gives the account id live personal access tokens that never
// expire and expired ones.
func addTokens(t *testing.T, pool *pgxpool.Pool, id string, live, expired int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO api_tokens (id, user_id, token_hash, name, expires_at, created_at, updated_at)
		SELECT gen_random_uuid(), $1, sha256(gen_random_uuid()::text::bytea), 'CI',
			CASE WHEN i > $2::int THEN now() - interval '1 day' END, now() - interval '2 days', now() - interval '2 days'
		FROM generate_series(1, $2::int + $3::int) AS i`, id, live, expired); err != nil {
		t.Fatal(err)
	}
}

// The five commands run on the command line's composition, one after
// another on one account, each printing its line (M1/P4 design 3.7); a
// second account shows that nothing reaches beyond the one named. The
// composition has no jobs client: nothing is enqueued.
func TestUsersCommands(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	bob := createdAccount(t, url, pool, "bob@corp.com")
	addSessions(t, pool, bob, 2)
	addTokens(t, pool, bob, 2, 1)

	out, logs, err := runUsers(t, url, CreateUser(" Carol@Corp.COM ", "Tr0ub4dor&3"))
	if err != nil || !strings.Contains(logs, `msg="account created"`) {
		t.Fatalf("create = %q, %v, logs %s", out, err, logs)
	}
	var carol string
	if err := pool.QueryRow(context.Background(), "SELECT id FROM users WHERE email = 'carol@corp.com'").Scan(&carol); err != nil {
		t.Fatal(err)
	}
	if want := "created carol@corp.com (" + carol + ")\n"; out != want {
		t.Errorf("create = %q, want %q: the normalized address and the id", out, want)
	}
	addSessions(t, pool, carol, 2)
	addTokens(t, pool, carol, 2, 1)

	// Each step after the reset first gives carol what it is to revoke, or
	// not: a live session, and before the activation live tokens again.
	steps := []struct {
		name    string
		prepare func()
		cmd     UserCommand
		out     string
		want    accountState
	}{
		{"reset-password", func() {}, ResetPassword("CAROL@corp.com", "N3w-Passw0rd!"),
			"password reset for carol@corp.com: revoked 2 sessions, 3 API tokens\n",
			accountState{"carol@corp.com", true, "password_reset,password_reset", "revoked,revoked,revoked"}},
		{"set-email", func() { addSessions(t, pool, carol, 1) }, SetEmail("carol@corp.com", "Carol@New.Example"),
			"e-mail changed to carol@new.example: revoked 1 session\n",
			accountState{"carol@new.example", true, "email_changed,password_reset,password_reset", "revoked,revoked,revoked"}},
		{"deactivate", func() { addSessions(t, pool, carol, 1); addTokens(t, pool, carol, 2, 1) }, DeactivateUser("carol@new.example"),
			"deactivated carol@new.example: revoked 1 session\n",
			accountState{"carol@new.example", false, "deactivated,email_changed,password_reset,password_reset", "revoked,revoked,revoked,live,live,live"}},
		{"deactivate again", func() {}, DeactivateUser("carol@new.example"),
			"carol@new.example is already deactivated\n",
			accountState{"carol@new.example", false, "deactivated,email_changed,password_reset,password_reset", "revoked,revoked,revoked,live,live,live"}},
		{"activate", func() {}, ActivateUser("carol@new.example"),
			"activated carol@new.example: 2 API tokens are usable again\n",
			accountState{"carol@new.example", true, "deactivated,email_changed,password_reset,password_reset", "revoked,revoked,revoked,live,live,live"}},
		{"activate again", func() {}, ActivateUser("carol@new.example"),
			"carol@new.example is already active\n",
			accountState{"carol@new.example", true, "deactivated,email_changed,password_reset,password_reset", "revoked,revoked,revoked,live,live,live"}},
	}
	for _, step := range steps {
		step.prepare()
		out, _, err := runUsers(t, url, step.cmd)
		if got := stateOf(t, pool, carol); err != nil || out != step.out || got != step.want {
			t.Errorf("%s = %q, %v leaving %+v; want %q leaving %+v", step.name, out, err, got, step.out, step.want)
		}
	}
	if got, want := stateOf(t, pool, bob), (accountState{"bob@corp.com", true, "live,live", "live,live,live"}); got != want {
		t.Errorf("bob = %+v, want %+v: untouched", got, want)
	}
	if jobs := count(t, pool, "SELECT count(*) FROM river_job"); jobs != 0 {
		t.Errorf("river_job holds %d rows, want none: the commands have no jobs client", jobs)
	}
}

// One API token usable again is said in the singular.
func TestActivateSaysOneToken(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	dave := createdAccount(t, url, pool, "dave@corp.com")
	addTokens(t, pool, dave, 1, 1)
	if _, _, err := runUsers(t, url, DeactivateUser("dave@corp.com")); err != nil {
		t.Fatal(err)
	}

	out, _, err := runUsers(t, url, ActivateUser("dave@corp.com"))

	if err != nil || out != "activated dave@corp.com: 1 API token is usable again\n" {
		t.Errorf("activate = %q, %v; want 1 usable token", out, err)
	}
}

// A refused command prints no line and says why in one line: the invalid
// fields by the names the command line knows, or the error's detail. The
// accounts stay as they were.
func TestUsersCommandErrors(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	erin := createdAccount(t, url, pool, "erin@corp.com")
	createdAccount(t, url, pool, "frank@corp.com")
	before := stateOf(t, pool, erin)
	tests := []struct {
		name string
		cmd  UserCommand
		want string
	}{
		{"a taken address", CreateUser("erin@corp.com", "Tr0ub4dor&3"), "An account with this e-mail address already exists."},
		{"a bad address and a short password", CreateUser("nobody", "short"),
			"--email is not a valid e-mail address; the password must be at least 8 characters"},
		{"an unknown account", ResetPassword("nobody@corp.com", "N3w-Passw0rd!"), "The account does not exist."},
		{"a common password", ResetPassword("erin@corp.com", "Password1!"), "the password is too common or too close to the e-mail address"},
		{"another account's address", SetEmail("erin@corp.com", "frank@corp.com"), "An account with this e-mail address already exists."},
		{"the same address", SetEmail("erin@corp.com", "ERIN@corp.com"), "The account has this e-mail address already."},
		{"a bad new address", SetEmail("erin@corp.com", "erin"), "--new-email is not a valid e-mail address"},
		{"deactivating nobody", DeactivateUser("nobody@corp.com"), "The account does not exist."},
		{"activating nobody", ActivateUser("nobody@corp.com"), "The account does not exist."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _, err := runUsers(t, url, tt.cmd)
			if err == nil || err.Error() != tt.want || out != "" {
				t.Errorf("= %q, %v; want no line and %q", out, err, tt.want)
			}
		})
	}
	if after := stateOf(t, pool, erin); after != before {
		t.Errorf("erin = %+v, want %+v: unchanged", after, before)
	}
	if n := count(t, pool, "SELECT count(*) FROM users"); n != 2 {
		t.Errorf("%d accounts, want the two created", n)
	}
}

func TestCommandErrorKeepsOtherErrors(t *testing.T) {
	boom := errors.New("connection refused")
	if err := commandError(boom); !errors.Is(err, boom) {
		t.Errorf("commandError(%v) = %v, want it unchanged", boom, err)
	}
	if err := commandError(domain.ErrAccountNotFound); !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("commandError(%v) = %v, want it unchanged", domain.ErrAccountNotFound, err)
	}
	unknown := shared.Invalid(shared.FieldError{Field: "display_name", Message: "is required"})
	if err := commandError(unknown); err == nil || err.Error() != "display_name is required" {
		t.Errorf("commandError(%v) = %v, want the field's own name", unknown, err)
	}
	workspaceFields := shared.Invalid(shared.FieldError{Field: "name", Message: "is required"}, shared.FieldError{Field: "slug", Message: "is reserved"})
	if err := commandError(workspaceFields); err == nil || err.Error() != "--name is required; --slug is reserved" {
		t.Errorf("commandError(%v) = %v, want the flags of nervewiki workspaces create", workspaceFields, err)
	}
}
