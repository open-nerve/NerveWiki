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

// objectsQuery lists the schema's objects: tables but goose's own;
// extensions but the built-in plpgsql; types and functions (River creates
// both) but the extensions' own and the types that stand for a table or an
// array.
const objectsQuery = `
	SELECT 'table ' || table_name FROM information_schema.tables
	WHERE table_schema = 'public' AND table_name <> 'goose_db_version'
	UNION ALL
	SELECT 'extension ' || extname FROM pg_extension WHERE extname <> 'plpgsql'
	UNION ALL
	SELECT 'type ' || t.typname FROM pg_type t
	WHERE t.typnamespace = 'public'::regnamespace AND t.typrelid = 0 AND t.typelem = 0
		AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid = 'pg_type'::regclass AND d.objid = t.oid AND d.deptype = 'e')
	UNION ALL
	SELECT 'function ' || p.oid::regprocedure::text FROM pg_proc p
	WHERE p.pronamespace = 'public'::regnamespace
		AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid = 'pg_proc'::regclass AND d.objid = p.oid AND d.deptype = 'e')
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
	if !slices.Contains(schema, "extension pg_trgm") || !slices.Contains(schema, "type river_job_state") {
		t.Errorf("after Up, the schema holds %q, want pg_trgm and River's job state among it", schema)
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
// and River's own, read from the catalog: a new table's constraints and
// indexes fail here until they are in want. River names its own, and its
// migration is checked against River's SQL (river_test.go).
func TestConstraintAndIndexNames(t *testing.T) {
	pool := newPool(t, pgtest.NewDatabase(t))
	rows, err := pool.Query(context.Background(), `
		WITH tables AS (
			SELECT oid FROM pg_class
			WHERE relnamespace = 'public'::regnamespace AND relkind IN ('r', 'p')
				AND relname <> 'goose_db_version' AND relname NOT LIKE 'river\_%'
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
	// DELETE CASCADE, r: RESTRICT, a: NO ACTION), c check. An index is i, then u when it is unique and w
	// when it is partial (has a WHERE).
	want := []string{
		"api_tokens_expires_at_check c",
		"api_tokens_name_check c",
		"api_tokens_pkey iu",
		"api_tokens_pkey p",
		"api_tokens_token_hash_check c",
		"api_tokens_token_hash_key iu",
		"api_tokens_token_hash_key u",
		"api_tokens_user_id_created_at_idx iw",
		"api_tokens_user_id_fkey f c",
		"auth_sessions_expires_at_idx i",
		"auth_sessions_generation_check c",
		"auth_sessions_pkey iu",
		"auth_sessions_pkey p",
		"auth_sessions_revoke_reason_check c",
		"auth_sessions_revoked_consistent_check c",
		"auth_sessions_token_hash_check c",
		"auth_sessions_user_id_fkey f c",
		"auth_sessions_user_id_idx i",
		"notebook_audit_events_action_check c",
		"notebook_audit_events_created_by_id_fkey f a",
		"notebook_audit_events_former_owner_id_fkey f a",
		"notebook_audit_events_pkey iu",
		"notebook_audit_events_pkey p",
		"notebook_audit_events_updated_by_id_fkey f a",
		"notebook_audit_events_workspace_id_created_at_idx iw",
		"notebook_audit_events_workspace_id_fkey f r",
		"notebook_audit_events_workspace_id_idx i",
		"notebook_members_created_by_id_fkey f a",
		"notebook_members_notebook_id_fkey f c",
		"notebook_members_notebook_id_idx i",
		"notebook_members_notebook_id_user_id_key iuw",
		"notebook_members_pkey iu",
		"notebook_members_pkey p",
		"notebook_members_role_check c",
		"notebook_members_updated_by_id_fkey f a",
		"notebook_members_user_id_fkey f a",
		"notebook_members_user_id_idx iw",
		"notebooks_created_by_id_fkey f a",
		"notebooks_former_owner_id_fkey f a",
		"notebooks_name_check c",
		"notebooks_ownerless_check c",
		"notebooks_pkey iu",
		"notebooks_pkey p",
		"notebooks_updated_by_id_fkey f a",
		"notebooks_workspace_access_check c",
		"notebooks_workspace_id_fkey f r",
		"notebooks_workspace_id_idx i",
		"users_display_name_check c",
		"users_email_check c",
		"users_email_key iu",
		"users_email_key u",
		"users_onboarding_steps_check c",
		"users_pkey iu",
		"users_pkey p",
		"workspace_invitations_accepted_check c",
		"workspace_invitations_created_by_id_fkey f a",
		"workspace_invitations_email_check c",
		"workspace_invitations_pkey iu",
		"workspace_invitations_pkey p",
		"workspace_invitations_role_check c",
		"workspace_invitations_updated_by_id_fkey f a",
		"workspace_invitations_workspace_id_email_key iuw",
		"workspace_invitations_workspace_id_fkey f c",
		"workspace_invitations_workspace_id_idx i",
		"workspace_members_created_by_id_fkey f a",
		"workspace_members_pkey iu",
		"workspace_members_pkey p",
		"workspace_members_role_check c",
		"workspace_members_updated_by_id_fkey f a",
		"workspace_members_user_id_fkey f a",
		"workspace_members_user_id_idx iw",
		"workspace_members_workspace_id_fkey f c",
		"workspace_members_workspace_id_idx i",
		"workspace_members_workspace_id_user_id_key iuw",
		"workspaces_created_by_id_fkey f a",
		"workspaces_name_check c",
		"workspaces_pkey iu",
		"workspaces_pkey p",
		"workspaces_slug_check c",
		"workspaces_slug_key iuw",
		"workspaces_updated_by_id_fkey f a",
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
		"INSERT INTO api_tokens (id, user_id, token_hash, name, expires_at, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000004', " + user + ", sha256('y'), 'CI', now() + interval '1 day', now(), now())",
		"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000005', 'acme_2-x', 'Acme 研发', " + user + ", " + user + ", now(), now())",
		"INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000006', '0199a2b4-0000-7000-8000-000000000005', " + user + ", 'guest', " +
			user + ", " + user + ", now(), now())",
		"INSERT INTO workspace_invitations (id, workspace_id, email, role, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000007', '0199a2b4-0000-7000-8000-000000000005', 'élodie@exämple.com', 'member', " +
			user + ", " + user + ", now(), now())",
		"UPDATE workspace_invitations SET accepted_at = now(), deleted_at = now()",
		"INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000008', '0199a2b4-0000-7000-8000-000000000005', '" + strings.Repeat("名", 85) + "', 'editor', " +
			user + ", " + user + ", now(), now())",
		"UPDATE notebooks SET ownerless_since = now(), former_owner_id = " + user,
		"INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000009', '0199a2b4-0000-7000-8000-000000000008', " + user + ", 'reader', " +
			user + ", " + user + ", now(), now())",
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
		{"two steps in one element", steps(`'{"profile,workspace"}'`), "users_onboarding_steps_check"},
		{"40 steps in one element", steps("ARRAY[(SELECT string_agg('s' || i, ',') FROM generate_series(1, 40) i)]"), "users_onboarding_steps_check"},
		{"token hash not 32 bytes", "UPDATE auth_sessions SET token_hash = '\\x00'", "auth_sessions_token_hash_check"},
		{"negative generation", "UPDATE auth_sessions SET generation = -1", "auth_sessions_generation_check"},
		{"unknown revoke reason", "UPDATE auth_sessions SET revoke_reason = 'expired'", "auth_sessions_revoke_reason_check"},
		{"revoked without a reason", "UPDATE auth_sessions SET revoke_reason = NULL", "auth_sessions_revoked_consistent_check"},
		{"a reason without revoked_at", "UPDATE auth_sessions SET revoked_at = NULL", "auth_sessions_revoked_consistent_check"},
		{"PAT hash not 32 bytes", "UPDATE api_tokens SET token_hash = '\\x00'", "api_tokens_token_hash_check"},
		{"empty PAT name", "UPDATE api_tokens SET name = ''", "api_tokens_name_check"},
		{"PAT expiring when created", "UPDATE api_tokens SET expires_at = created_at", "api_tokens_expires_at_check"},
		{"upper-case slug", "UPDATE workspaces SET slug = 'Acme'", "workspaces_slug_check"},
		{"slug with a space", "UPDATE workspaces SET slug = 'acme corp'", "workspaces_slug_check"},
		{"slug with a dot", "UPDATE workspaces SET slug = 'acme.corp'", "workspaces_slug_check"},
		{"non-ASCII slug", "UPDATE workspaces SET slug = 'café'", "workspaces_slug_check"},
		{"empty slug", "UPDATE workspaces SET slug = ''", "workspaces_slug_check"},
		{"empty workspace name", "UPDATE workspaces SET name = ''", "workspaces_name_check"},
		{"a fourth role", "UPDATE workspace_members SET role = 'owner'", "workspace_members_role_check"},
		{"an upper-case role", "UPDATE workspace_members SET role = 'Admin'", "workspace_members_role_check"},
		{"an upper-case invited address", "UPDATE workspace_invitations SET email = 'Bob@corp.com'", "workspace_invitations_email_check"},
		{"an invited address with a space", "UPDATE workspace_invitations SET email = 'bob@corp.com '", "workspace_invitations_email_check"},
		{"an invitation's fourth role", "UPDATE workspace_invitations SET role = 'owner'", "workspace_invitations_role_check"},
		{"accepted and not deleted", "UPDATE workspace_invitations SET deleted_at = NULL", "workspace_invitations_accepted_check"},
		{"accepted before its deletion", "UPDATE workspace_invitations SET accepted_at = deleted_at - interval '1 second'", "workspace_invitations_accepted_check"},
		{"empty notebook name", "UPDATE notebooks SET name = ''", "notebooks_name_check"},
		{"notebook name of 256 bytes", "UPDATE notebooks SET name = name || 'a'", "notebooks_name_check"},
		{"a fourth access", "UPDATE notebooks SET workspace_access = 'public'", "notebooks_workspace_access_check"},
		{"an upper-case access", "UPDATE notebooks SET workspace_access = 'Viewer'", "notebooks_workspace_access_check"},
		{"ownerless without its former owner", "UPDATE notebooks SET former_owner_id = NULL", "notebooks_ownerless_check"},
		{"a former owner without ownerless", "UPDATE notebooks SET ownerless_since = NULL", "notebooks_ownerless_check"},
		{"a notebook's fourth role", "UPDATE notebook_members SET role = 'owner'", "notebook_members_role_check"},
		{"a notebook's upper-case role", "UPDATE notebook_members SET role = 'Reader'", "notebook_members_role_check"},
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
