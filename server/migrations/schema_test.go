package migrations_test

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// objectsQuery lists the schema's objects: tables but goose's own, and
// extensions but the built-in plpgsql.
const objectsQuery = `
	SELECT 'table ' || table_name FROM information_schema.tables
	WHERE table_schema = 'public' AND table_name <> 'goose_db_version'
	UNION ALL
	SELECT 'extension ' || extname FROM pg_extension WHERE extname <> 'plpgsql'
	ORDER BY 1`

func newPool(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: url, MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func objects(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), objectsQuery)
	if err != nil {
		t.Fatal(err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// Every migration goes up, and down again to an empty schema; going up once
// more rebuilds the same schema.
func TestMigrationsGoUpDownAndUpAgain(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewEmptyDatabase(t))
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	files, err := fs.ReadDir(migrations.FS(), ".")
	if err != nil {
		t.Fatal(err)
	}

	up, err := m.Up(ctx)
	if err != nil || len(up) != len(files) {
		t.Fatalf("Up() = %d migrations, %v; want all %d", len(up), err, len(files))
	}
	schema := objects(t, pool)
	if !slices.Contains(schema, "extension pg_trgm") {
		t.Errorf("after Up, the schema holds %q, want pg_trgm among it", schema)
	}
	for range up {
		if _, err := m.Down(ctx); err != nil {
			t.Fatalf("Down() error = %v", err)
		}
	}
	if left := objects(t, pool); len(left) != 0 {
		t.Errorf("after every Down, the schema holds %q, want nothing", left)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up() again: %v", err)
	}
	if again := objects(t, pool); !slices.Equal(again, schema) {
		t.Errorf("after Up again, the schema holds %q, want %q as after the first Up", again, schema)
	}
}

// Constraint and index names are stable: errors are mapped by them, and later
// migrations drop them by name. Each index is pinned with what it is too,
// unique and partial or not, so a partial unique key cannot turn into a plain
// or a total one unseen. The tables are every table of the schema but goose's
// own, read from the catalog: a new table's constraints and indexes fail here
// until they are in want.
func TestConstraintAndIndexNames(t *testing.T) {
	pool := newPool(t, pgtest.NewDatabase(t))
	rows, err := pool.Query(context.Background(), `
		WITH tables AS (
			SELECT oid FROM pg_class
			WHERE relnamespace = 'public'::regnamespace AND relkind IN ('r', 'p')
				AND relname <> 'goose_db_version'
		)
		SELECT conname || ' ' || contype::text || CASE WHEN contype = 'f' THEN ' ' || confdeltype::text ELSE '' END FROM pg_constraint
		WHERE conrelid IN (SELECT oid FROM tables)
			AND contype <> 'n' -- PG 18 lists NOT NULL as constraints too
		UNION ALL
		SELECT c.relname || ' i' || CASE WHEN i.indisunique THEN 'u' ELSE '' END || CASE WHEN i.indpred IS NOT NULL THEN 'w' ELSE '' END
		FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
		WHERE i.indrelid IN (SELECT oid FROM tables)
		ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	// contype: p primary key, u unique, f foreign key (confdeltype c: ON
	// DELETE CASCADE), c check. An index is i, then u when it is unique and w
	// when it is partial (has a WHERE).
	want := []string{
		"auth_sessions_expires_at_idx i",
		"auth_sessions_generation_check c",
		"auth_sessions_pkey iu",
		"auth_sessions_pkey p",
		"auth_sessions_revoke_reason_check c",
		"auth_sessions_revoked_consistent_check c",
		"auth_sessions_token_hash_check c",
		"auth_sessions_user_id_fkey f c",
		"auth_sessions_user_id_idx i",
		"users_display_name_check c",
		"users_email_check c",
		"users_email_key iu",
		"users_email_key u",
		"users_onboarding_steps_check c",
		"users_pkey iu",
		"users_pkey p",
	}
	if !slices.Equal(got, want) {
		t.Errorf("constraints and indexes =\n%q\nwant\n%q", got, want)
	}
}

// The CHECKs accept what the domain writes and reject what bypasses it.
func TestChecksRejectCounterexamples(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	const user = "'0199a2b4-0000-7000-8000-000000000001'"
	for _, stmt := range []string{
		"INSERT INTO users (id, email, password, display_name, onboarding_steps, created_at, updated_at) VALUES (" +
			user + ", 'élodie@exämple.com', 'x', 'élodie', '{profile,workspace_2}', now(), now())",
		"INSERT INTO auth_sessions (id, user_id, token_hash, expires_at, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000003', " + user + ", sha256('x'), now(), now(), now())",
		"UPDATE auth_sessions SET revoked_at = now(), revoke_reason = 'logout'",
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	insertUser := func(email, displayName string) string {
		return "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES (gen_random_uuid(), " +
			email + ", 'x', " + displayName + ", now(), now())"
	}
	steps := func(ids string) string { return "UPDATE users SET onboarding_steps = " + ids }
	tests := []struct{ name, stmt, constraint string }{
		{"upper-case ASCII e-mail", insertUser("'Bob@corp.com'", "'b'"), "users_email_check"},
		{"upper-case non-ASCII e-mail", insertUser("'Élodie@corp.com'", "'e'"), "users_email_check"},
		{"leading space", insertUser("' carol@corp.com'", "'c'"), "users_email_check"},
		{"trailing tab", insertUser(`E'carol@corp.com\t'`, "'c'"), "users_email_check"},
		{"trailing newline", insertUser(`E'carol@corp.com\n'`, "'c'"), "users_email_check"},
		{"inner space", insertUser("'car ol@corp.com'", "'c'"), "users_email_check"},
		{"leading U+3000", insertUser(`U&'\3000carol@corp.com'`, "'c'"), "users_email_check"},
		{"trailing U+2028", insertUser(`U&'carol@corp.com\2028'`, "'c'"), "users_email_check"},
		{"empty display name", insertUser("'dave@corp.com'", "''"), "users_display_name_check"},
		{"step with an upper-case letter", steps("'{Profile}'"), "users_onboarding_steps_check"},
		{"step starting with a digit", steps("'{2fa}'"), "users_onboarding_steps_check"},
		{"step with a hyphen", steps("'{first-notebook}'"), "users_onboarding_steps_check"},
		{"empty step", steps(`'{""}'`), "users_onboarding_steps_check"},
		{"empty step last", steps(`'{profile,""}'`), "users_onboarding_steps_check"},
		{"empty step first", steps(`'{"",profile}'`), "users_onboarding_steps_check"},
		{"step of 33 characters", steps("ARRAY['" + strings.Repeat("a", 33) + "']"), "users_onboarding_steps_check"},
		{"NULL step", steps("ARRAY['profile', NULL]"), "users_onboarding_steps_check"},
		{"33 steps", steps("(SELECT array_agg('s' || i) FROM generate_series(1, 33) i)"), "users_onboarding_steps_check"},
		{"token hash not 32 bytes", "UPDATE auth_sessions SET token_hash = '\\x00'", "auth_sessions_token_hash_check"},
		{"negative generation", "UPDATE auth_sessions SET generation = -1", "auth_sessions_generation_check"},
		{"unknown revoke reason", "UPDATE auth_sessions SET revoke_reason = 'expired'", "auth_sessions_revoke_reason_check"},
		{"revoked without a reason", "UPDATE auth_sessions SET revoke_reason = NULL", "auth_sessions_revoked_consistent_check"},
		{"a reason without revoked_at", "UPDATE auth_sessions SET revoked_at = NULL", "auth_sessions_revoked_consistent_check"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tt.stmt)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != tt.constraint {
				t.Errorf("%s = %v, want check_violation (23514) of %s", tt.stmt, err, tt.constraint)
			}
		})
	}
}
