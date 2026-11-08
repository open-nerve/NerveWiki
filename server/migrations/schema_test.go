package migrations_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
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

// 00042 takes the siblings' unique index (00012) over in two, the roots'
// and the children's, and drops it; its Down makes it again as 00012 did,
// and drops the two (M9/P1 S2 fix round 2).
func TestThePathKeysGoDownToTheSiblingsIndex(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewEmptyDatabase(t))
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	if _, err := m.Up(ctx); err != nil {
		t.Fatal(err)
	}
	indexes := func() []string {
		rows, err := pool.Query(ctx, `SELECT indexdef FROM pg_indexes WHERE tablename = 'nodes'
			AND indexdef LIKE 'CREATE UNIQUE INDEX %' AND indexdef LIKE '%name_key)%' ORDER BY indexname`)
		if err != nil {
			t.Fatal(err)
		}
		defs, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		return defs
	}
	up := []string{
		"CREATE UNIQUE INDEX nodes_notebook_id_name_key_root_key ON public.nodes USING btree (notebook_id, name_key) " +
			"WHERE ((parent_id IS NULL) AND (deleted_at IS NULL))",
		"CREATE UNIQUE INDEX nodes_parent_id_name_key_key ON public.nodes USING btree (parent_id, name_key) " +
			"WHERE ((parent_id IS NOT NULL) AND (deleted_at IS NULL))",
	}
	if got := indexes(); !slices.Equal(got, up) {
		t.Errorf("the name keys' unique indexes after Up are %q, want %q", got, up)
	}
	for {
		back, err := m.Down(ctx)
		if err != nil || back == nil {
			t.Fatalf("Down() = %+v, %v; want down past 00042", back, err)
		}
		if back.Version == 42 {
			break
		}
	}
	down := []string{"CREATE UNIQUE INDEX nodes_notebook_id_parent_id_name_key_idx ON public.nodes USING btree " +
		"(notebook_id, parent_id, name_key) NULLS NOT DISTINCT WHERE (deleted_at IS NULL)"}
	if got := indexes(); !slices.Equal(got, down) {
		t.Errorf("the name keys' unique indexes after 00042's Down are %q, want %q", got, down)
	}
}

// Constraint and index names are stable: errors are mapped by them, and later
// migrations drop them by name. Each index is pinned with what it is too,
// unique, partial and NULLS NOT DISTINCT or not, so a partial unique key
// cannot turn into a plain or a total one, nor let two NULLs by, unseen. The tables are every table of the schema but goose's
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
			|| CASE WHEN i.indnullsnotdistinct THEN 'n' ELSE '' END
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
	// DELETE CASCADE, r: RESTRICT, a: NO ACTION), c check. An index is i, then u when it is unique, w
	// when it is partial (has a WHERE) and n when its NULLs are not distinct.
	want := []string{
		"api_tokens_client_name_check c",
		"api_tokens_expires_at_check c",
		"api_tokens_name_check c",
		"api_tokens_pkey iu",
		"api_tokens_pkey p",
		"api_tokens_token_hash_check c",
		"api_tokens_token_hash_key iu",
		"api_tokens_token_hash_key u",
		"api_tokens_user_id_created_at_idx iw",
		"api_tokens_user_id_fkey f c",
		"asset_blobs_byte_size_check c",
		"asset_blobs_created_by_id_fkey f a",
		"asset_blobs_deleted_at_idx iw",
		"asset_blobs_height_check c",
		"asset_blobs_mime_check c",
		"asset_blobs_node_id_key iu",
		"asset_blobs_node_id_key u",
		"asset_blobs_notebook_id_idx iw",
		"asset_blobs_notebook_id_node_id_fkey f r",
		"asset_blobs_pkey iu",
		"asset_blobs_pkey p",
		"asset_blobs_sha256_check c",
		"asset_blobs_size_check c",
		"asset_blobs_width_check c",
		"auth_sessions_expires_at_idx i",
		"auth_sessions_generation_check c",
		"auth_sessions_pkey iu",
		"auth_sessions_pkey p",
		"auth_sessions_revoke_reason_check c",
		"auth_sessions_revoked_consistent_check c",
		"auth_sessions_token_hash_check c",
		"auth_sessions_user_id_fkey f c",
		"auth_sessions_user_id_idx i",
		"changeset_items_changeset_id_fkey f c",
		"changeset_items_changeset_id_node_id_key iu",
		"changeset_items_changeset_id_node_id_key u",
		"changeset_items_deleted_at_idx iw",
		"changeset_items_node_id_fkey f c",
		"changeset_items_node_id_idx i",
		"changeset_items_pkey iu",
		"changeset_items_pkey p",
		"changeset_items_restored_check c",
		"changeset_items_revision_check c",
		"changeset_items_revision_state_check c",
		"changeset_items_state_check c",
		"changeset_sources_changeset_id_node_id_key iu",
		"changeset_sources_changeset_id_node_id_key u",
		"changeset_sources_pkey iu",
		"changeset_sources_pkey p",
		"changesets_client_check c",
		"changesets_created_by_id_fkey f a",
		"changesets_deleted_at_idx iw",
		"changesets_ingest_count_check c",
		"changesets_kind_check c",
		"changesets_message_check c",
		"changesets_notebook_id_created_at_id_idx i",
		"changesets_notebook_id_created_at_ingest_idx iw",
		"changesets_notebook_id_created_by_id_created_at_id_idx iw",
		"changesets_notebook_id_created_by_id_created_at_ingest_idx iw",
		"changesets_notebook_id_fkey f r",
		"changesets_pkey iu",
		"changesets_pkey p",
		"changesets_reverts_check c",
		"changesets_reverts_id_idx iw",
		"changesets_trimmed_idx iw",
		"changesets_upgrade_id_idx iw",
		"edit_sessions_client_check c",
		"edit_sessions_ended_by_id_fkey f a",
		"edit_sessions_ended_check c",
		"edit_sessions_ended_reason_check c",
		"edit_sessions_expires_at_check c",
		"edit_sessions_node_id_idx i",
		"edit_sessions_notebook_id_idx i",
		"edit_sessions_pkey iu",
		"edit_sessions_pkey p",
		"edit_sessions_user_id_fkey f a",
		"edit_sessions_written_check c",
		"indexed_pages_extractor_check c",
		"indexed_pages_notebook_id_idx i",
		"indexed_pages_pkey iu",
		"indexed_pages_pkey p",
		"indexed_pages_revision_check c",
		"nodes_created_by_id_fkey f a",
		"nodes_deleted_at_idx iw",
		"nodes_kind_check c",
		"nodes_name_check c",
		"nodes_name_key_check c",
		"nodes_notebook_id_deleted_at_id_idx iw",
		"nodes_notebook_id_fkey f r",
		"nodes_notebook_id_id_key iu",
		"nodes_notebook_id_id_key u",
		"nodes_notebook_id_kind_source_hash_idx iw",
		"nodes_notebook_id_name_key_idx iw",
		"nodes_notebook_id_name_key_root_key iuw",
		"nodes_notebook_id_parent_id_fkey f r",
		"nodes_notebook_id_parent_id_idx i",
		"nodes_parent_check c",
		"nodes_parent_id_name_key_key iuw",
		"nodes_pkey iu",
		"nodes_pkey p",
		"nodes_sort_order_check c",
		"nodes_source_hash_check c",
		"nodes_trash_counts_check c",
		"nodes_trash_root_check c",
		"nodes_updated_by_id_fkey f a",
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
		"notebook_upgrades_approved_check c",
		"notebook_upgrades_baseline_check c",
		"notebook_upgrades_created_by_id_fkey f a",
		"notebook_upgrades_ending_check c",
		"notebook_upgrades_ending_state_check c",
		"notebook_upgrades_locale_check c",
		"notebook_upgrades_notebook_id_fkey f r",
		"notebook_upgrades_notebook_id_idx i",
		"notebook_upgrades_notebook_id_key iuw",
		"notebook_upgrades_pkey iu",
		"notebook_upgrades_pkey p",
		"notebook_upgrades_rejected_check c",
		"notebook_upgrades_report_check c",
		"notebook_upgrades_rollback_end_check c",
		"notebook_upgrades_rollback_job_check c",
		"notebook_upgrades_state_check c",
		"notebook_upgrades_submission_check c",
		"notebook_upgrades_submission_state_check c",
		"notebook_upgrades_submitted_by_id_fkey f a",
		"notebook_upgrades_updated_by_id_fkey f a",
		"notebooks_created_by_id_fkey f a",
		"notebooks_former_owner_id_fkey f a",
		"notebooks_mode_check c",
		"notebooks_name_check c",
		"notebooks_ownerless_check c",
		"notebooks_pkey iu",
		"notebooks_pkey p",
		"notebooks_source_root_check c",
		"notebooks_updated_by_id_fkey f a",
		"notebooks_upgrade_by_id_fkey f a",
		"notebooks_upgrade_check c",
		"notebooks_workspace_access_check c",
		"notebooks_workspace_id_fkey f r",
		"notebooks_workspace_id_idx i",
		"page_aliases_alias_check c",
		"page_aliases_alias_key_check c",
		"page_aliases_notebook_id_alias_key_idx i",
		"page_aliases_pkey iu",
		"page_aliases_pkey p",
		"page_contents_byte_size_check c",
		"page_contents_content_hash_check c",
		"page_contents_deleted_at_idx iw",
		"page_contents_node_id_fkey f c",
		"page_contents_pkey iu",
		"page_contents_pkey p",
		"page_contents_revision_check c",
		"page_contents_updated_by_id_fkey f a",
		"page_links_ambiguous_check c",
		"page_links_kind_check c",
		"page_links_notebook_id_source_id_range_start_idx i",
		"page_links_notebook_id_target_alt_key_idx iw",
		"page_links_notebook_id_target_key_idx i",
		"page_links_pkey iu",
		"page_links_pkey p",
		"page_links_range_check c",
		"page_links_range_start_check c",
		"page_links_resolved_asset_check c",
		"page_links_resolved_id_source_id_idx iw",
		"page_links_source_id_range_start_idx iw",
		"page_links_target_check c",
		"page_properties_notebook_id_idx i",
		"page_properties_pkey iu",
		"page_properties_pkey p",
		"page_properties_position_check c",
		"page_properties_sources_idx iw",
		"page_properties_summary_idx iw",
		"page_properties_type_idx iw",
		"page_revisions_byte_size_check c",
		"page_revisions_changeset_id_fkey f c",
		"page_revisions_changeset_id_node_id_key iu",
		"page_revisions_changeset_id_node_id_key u",
		"page_revisions_content_hash_check c",
		"page_revisions_deleted_at_idx iw",
		"page_revisions_node_id_fkey f c",
		"page_revisions_node_id_revision_key iu",
		"page_revisions_pkey iu",
		"page_revisions_pkey p",
		"page_revisions_revision_check c",
		"page_search_body_check c",
		"page_search_body_idx i",
		"page_search_partial_check c",
		"page_search_pkey iu",
		"page_search_pkey p",
		"page_search_queue_bulk_idx iw",
		"page_search_queue_notebook_id_bulk_idx i",
		"page_search_queue_pkey iu",
		"page_search_queue_pkey p",
		"page_search_revision_check c",
		"page_search_tags_pkey iu",
		"page_search_tags_pkey p",
		"page_search_tags_prefix_key iu",
		"page_search_tags_prefix_key u",
		"page_search_title_idx i",
		"page_tags_count_check c",
		"page_tags_notebook_id_tag_key_idx i",
		"page_tags_pkey iu",
		"page_tags_pkey p",
		"page_tags_tag_check c",
		"page_tags_tag_key_check c",
		"transfer_jobs_changeset_check c",
		"transfer_jobs_client_check c",
		"transfer_jobs_created_by_id_fkey f a",
		"transfer_jobs_deleted_at_idx iw",
		"transfer_jobs_expired_check c",
		"transfer_jobs_exporting_key iuw",
		"transfer_jobs_finished_at_idx iw",
		"transfer_jobs_finished_check c",
		"transfer_jobs_kind_check c",
		"transfer_jobs_message_check c",
		"transfer_jobs_name_check c",
		"transfer_jobs_notebook_id_created_at_idx iw",
		"transfer_jobs_notebook_id_fkey f r",
		"transfer_jobs_pkey iu",
		"transfer_jobs_pkey p",
		"transfer_jobs_progress_check c",
		"transfer_jobs_result_bytes_check c",
		"transfer_jobs_root_check c",
		"transfer_jobs_started_check c",
		"transfer_jobs_state_check c",
		"transfer_jobs_state_idx iw",
		"transfer_jobs_target_check c",
		"transfer_jobs_writing_key iuw",
		"upgrade_nodes_baseline_check c",
		"upgrade_nodes_kind_check c",
		"upgrade_nodes_notebook_id_idx i",
		"upgrade_nodes_page_check c",
		"upgrade_nodes_pkey iu",
		"upgrade_nodes_pkey p",
		"upgrade_nodes_wanted_check c",
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

// checkedRows writes a row of each table the CHECKs guard, as the domain
// writes them: the CHECKs accept them.
func checkedRows(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	const user = "'0199a2b4-0000-7000-8000-000000000001'"
	for _, stmt := range []string{
		"INSERT INTO users (id, email, password, display_name, onboarding_steps, created_at, updated_at) VALUES (" +
			user + ", 'élodie@exämple.com', 'x', 'élodie', '{profile,workspace_2}', now(), now())",
		"INSERT INTO auth_sessions (id, user_id, token_hash, expires_at, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000003', " + user + ", sha256('x'), now(), now(), now())",
		"UPDATE auth_sessions SET revoked_at = now(), revoke_reason = 'logout'",
		"INSERT INTO api_tokens (id, user_id, token_hash, name, expires_at, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000004', " + user + ", sha256('y'), 'CI', now() + interval '1 day', now(), now())",
		// A client's name is 128 characters at most, not bytes.
		"UPDATE api_tokens SET client_name = repeat('名', 128)",
		"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000005', 'acme_2-x', 'Acme 研发', " + user + ", " + user + ", now(), now())",
		"INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000006', '0199a2b4-0000-7000-8000-000000000005', " + user + ", 'guest', " +
			user + ", " + user + ", now(), now())",
		"INSERT INTO workspace_invitations (id, workspace_id, email, role, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000007', '0199a2b4-0000-7000-8000-000000000005', 'élodie@exämple.com', 'member', " +
			user + ", " + user + ", now(), now())",
		"UPDATE workspace_invitations SET accepted_at = now(), deleted_at = now()",
		// An LLM Wiki, inserted with its mode and its source root at once, as its creation does (M10/P1 design 3.4):
		// the root page below.
		"INSERT INTO notebooks (id, workspace_id, name, workspace_access, mode, source_root_id, created_by_id, updated_by_id, " +
			"created_at, updated_at) VALUES ('0199a2b4-0000-7000-8000-000000000008', '0199a2b4-0000-7000-8000-000000000005', '" +
			strings.Repeat("名", 85) + "', 'editor', 'llm_wiki', '0199a2b4-0000-7000-8000-000000000010', " + user + ", " + user + ", now(), now())",
		"UPDATE notebooks SET ownerless_since = now(), former_owner_id = " + user,
		// A standard notebook being upgraded, its four columns set together (M11/P1 design 3.2).
		"INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at, upgrade_id, upgrade_state, " +
			"upgrade_by_id, upgrade_at) VALUES ('0199a2b4-0000-7000-8000-000000000019', '0199a2b4-0000-7000-8000-000000000005', 'Upgrading', " +
			user + ", " + user + ", now(), now(), '0199a2b4-0000-7000-8000-000000000020', 'upgrading', " + user + ", now())",
		"INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000009', '0199a2b4-0000-7000-8000-000000000008', " + user + ", 'reader', " +
			user + ", " + user + ", now(), now())",
		// A root page of 255 bytes and its child, the root's content, a changeset that created the root, its
		// item and its version.
		"INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000010', '0199a2b4-0000-7000-8000-000000000008', NULL, 'page', '" + strings.Repeat("名", 85) + "', 'k', 0, " +
			user + ", " + user + ", now(), now()), " +
			"('0199a2b4-0000-7000-8000-000000000011', '0199a2b4-0000-7000-8000-000000000008', '0199a2b4-0000-7000-8000-000000000010', 'page', 'b', 'b', 1, " +
			user + ", " + user + ", now(), now())",
		// An attachment under the root, and its file.
		"INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000016', '0199a2b4-0000-7000-8000-000000000008', '0199a2b4-0000-7000-8000-000000000010', 'asset', 'a.png', 'a.png', 2, " +
			user + ", " + user + ", now(), now())",
		"INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, width, height, created_by_id, created_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000017', '0199a2b4-0000-7000-8000-000000000016', '0199a2b4-0000-7000-8000-000000000008', 'image/png', 3, " +
			"sha256('abc'), 1, 1, " + user + ", now())",
		"INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000010', 'x', 1, sha256('x'), 1, " + user + ", now())",
		"INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000012', '0199a2b4-0000-7000-8000-000000000008', 'edit', 'mcp:claude-code', " + user + ", now(), now())",
		"INSERT INTO changeset_items (id, changeset_id, node_id, after_name, after_sort_order, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000013', '0199a2b4-0000-7000-8000-000000000012', '0199a2b4-0000-7000-8000-000000000010', 'a', 0, now(), now())",
		"INSERT INTO page_revisions (id, changeset_id, node_id, revision, content, content_hash, byte_size, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000014', '0199a2b4-0000-7000-8000-000000000012', '0199a2b4-0000-7000-8000-000000000010', 1, 'x', sha256('x'), 1, now(), now())",
		"INSERT INTO edit_sessions (id, node_id, notebook_id, user_id, client, changeset_id, revision, created_at, expires_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000015', '0199a2b4-0000-7000-8000-000000000010', '0199a2b4-0000-7000-8000-000000000008', " +
			user + ", 'mcp:claude-code', '0199a2b4-0000-7000-8000-000000000012', 1, now(), now() + interval '1 minute')",
		// The root's index: its version, an ambiguous link in a property, a tag, a property and an alias.
		"INSERT INTO indexed_pages (node_id, notebook_id, revision, extractor, frontmatter_valid) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000010', '0199a2b4-0000-7000-8000-000000000008', 1, 1, true)",
		"INSERT INTO page_links (source_id, range_start, range_end, notebook_id, kind, property_key, target, target_key, " +
			"resolved_id, ambiguous) VALUES ('0199a2b4-0000-7000-8000-000000000010', 0, 1, '0199a2b4-0000-7000-8000-000000000008', " +
			"'wikilink', 'sources.0', 'b', 'b', '0199a2b4-0000-7000-8000-000000000011', true)",
		"INSERT INTO page_tags (source_id, tag_key, notebook_id, tag, count) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000010', 'tag', '0199a2b4-0000-7000-8000-000000000008', 'Tag', 1)",
		"INSERT INTO page_properties (source_id, position, notebook_id, key, value) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000010', 0, '0199a2b4-0000-7000-8000-000000000008', 'sources', '[\"[[b]]\"]')",
		"INSERT INTO page_aliases (source_id, alias_key, notebook_id, alias) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000010', 'al', '0199a2b4-0000-7000-8000-000000000008', 'Al')",
		// The search's rows of the root and the attachment, and the child in its queue.
		"INSERT INTO page_search (node_id, revision, name, tokenizer, built_at, title, body) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000010', 1, 'a', '1/17.0.0', now(), 'a', 'a x'), " +
			"('0199a2b4-0000-7000-8000-000000000016', NULL, 'a.png', '1/17.0.0', now(), 'a png', NULL)",
		"INSERT INTO page_search_queue (node_id, notebook_id, bulk) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000011', '0199a2b4-0000-7000-8000-000000000008', false)",
		// The notebook's prefix of the search's keys.
		"INSERT INTO page_search_tags (notebook_id, prefix) VALUES ('0199a2b4-0000-7000-8000-000000000008', '0#')",
		// The upgrade's baseline: a page at the root and an attachment under it, and the protected row of the source
		// root its start created, a page at the root (M11/P1 design 3.2).
		"INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision, " +
			"content_hash, byte_size, wanted, accepted_hash) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000020', '0199a2b4-0000-7000-8000-000000000021', '0199a2b4-0000-7000-8000-000000000019', true, " +
			"'page', NULL, 'Guide', 0, 1, sha256('x'), 1, 10000, sha256('x')), " +
			"('0199a2b4-0000-7000-8000-000000000020', '0199a2b4-0000-7000-8000-000000000022', '0199a2b4-0000-7000-8000-000000000019', true, " +
			"'asset', '0199a2b4-0000-7000-8000-000000000021', 'a.png', 1, NULL, NULL, NULL, NULL, NULL), " +
			"('0199a2b4-0000-7000-8000-000000000020', '0199a2b4-0000-7000-8000-000000000023', '0199a2b4-0000-7000-8000-000000000019', false, " +
			"'page', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL)",
		"UPDATE changesets SET upgrade_id = '0199a2b4-0000-7000-8000-000000000020'",
		// The wiki module's row of that upgrade (M11/P1 design 3.10): upgrading, its seeded source root the protected
		// row's page, its baseline's two nodes and one page, wanting the most it counts.
		"INSERT INTO notebook_upgrades (id, notebook_id, state, source_root_id, seeded_root, locale, baseline_nodes, baseline_pages, " +
			"baseline_wanted, baseline_wanted_capped, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"('0199a2b4-0000-7000-8000-000000000020', '0199a2b4-0000-7000-8000-000000000019', 'upgrading', " +
			"'0199a2b4-0000-7000-8000-000000000023', true, 'zh-CN', 2, 1, 10000, true, " + user + ", " + user + ", now(), now())",
		// An export of the notebook, running.
		"INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, progress_done, progress_total, " +
			"started_at, heartbeat_at, created_at) VALUES ('0199a2b4-0000-7000-8000-000000000018', '0199a2b4-0000-7000-8000-000000000008', " +
			"'export', 'running', 'n', " + user + ", 'web', 1, 2, now(), now(), now())",
	} {
		if _, err := pool.Exec(context.Background(), stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

// The CHECKs accept what the domain writes and reject what bypasses it.
func TestChecksRejectCounterexamples(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	checkedRows(t, pool)
	insertUser := func(email, displayName string) string {
		return "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES (gen_random_uuid(), " +
			email + ", 'x', " + displayName + ", now(), now())"
	}
	steps := func(ids string) string { return "UPDATE users SET onboarding_steps = " + ids }
	// The upgrade's row in the wiki module (M11/P1 design 3.10): upgrading, without a submission; rolling back and
	// rolled back as their steps set them; a submission's three columns.
	const (
		upgrade     = "UPDATE notebook_upgrades SET "
		rollingBack = "state = 'rolling_back', ending = 'abandoned', rollback_job_id = gen_random_uuid()"
		rolledBack  = "state = 'rolled_back', ending = 'abandoned', rollback_job_id = gen_random_uuid()"
		submitter   = "'0199a2b4-0000-7000-8000-000000000001'"
		submission  = "submission_id = gen_random_uuid(), submitted_at = now(), submitted_by_id = " + submitter
	)
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
		{"empty client name", "UPDATE api_tokens SET client_name = ''", "api_tokens_client_name_check"},
		{"client name of 129 characters", "UPDATE api_tokens SET client_name = repeat('名', 129)", "api_tokens_client_name_check"},
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
		{"a third mode", "UPDATE notebooks SET mode = 'upgrading'", "notebooks_mode_check"},
		{"an upper-case mode", "UPDATE notebooks SET mode = 'LLM_WIKI'", "notebooks_mode_check"},
		{"an LLM Wiki without its source root", "UPDATE notebooks SET source_root_id = NULL", "notebooks_source_root_check"},
		// An upgrade's four columns (M11/P1 design 3.2): each conjunct alone broken, each = both ways.
		{"an upgrade's state without its id", "UPDATE notebooks SET upgrade_id = NULL, upgrade_by_id = NULL, upgrade_at = NULL " +
			"WHERE upgrade_id IS NOT NULL", "notebooks_upgrade_check"},
		{"an upgrade without its state", "UPDATE notebooks SET upgrade_state = NULL WHERE upgrade_id IS NOT NULL", "notebooks_upgrade_check"},
		{"an upgrade without its initiator", "UPDATE notebooks SET upgrade_by_id = NULL WHERE upgrade_id IS NOT NULL", "notebooks_upgrade_check"},
		{"an initiator alone", "UPDATE notebooks SET upgrade_id = NULL, upgrade_state = NULL, upgrade_at = NULL WHERE upgrade_id IS NOT NULL",
			"notebooks_upgrade_check"},
		{"a start alone", "UPDATE notebooks SET upgrade_id = NULL, upgrade_state = NULL, upgrade_by_id = NULL WHERE upgrade_id IS NOT NULL",
			"notebooks_upgrade_check"},
		{"an upgrade without its start", "UPDATE notebooks SET upgrade_at = NULL WHERE upgrade_id IS NOT NULL", "notebooks_upgrade_check"},
		{"an upgrade's unknown state", "UPDATE notebooks SET upgrade_state = 'approved' WHERE upgrade_id IS NOT NULL", "notebooks_upgrade_check"},
		{"an LLM Wiki upgrading", "UPDATE notebooks SET upgrade_id = gen_random_uuid(), upgrade_state = 'upgrading', upgrade_by_id = " +
			"created_by_id, upgrade_at = now() WHERE mode = 'llm_wiki'", "notebooks_upgrade_check"},
		{"a notebook's fourth role", "UPDATE notebook_members SET role = 'owner'", "notebook_members_role_check"},
		{"a notebook's upper-case role", "UPDATE notebook_members SET role = 'Reader'", "notebook_members_role_check"},
		{"a third kind of node", "UPDATE nodes SET kind = 'folder'", "nodes_kind_check"},
		{"an upper-case kind", "UPDATE nodes SET kind = 'Page'", "nodes_kind_check"},
		{"empty node name", "UPDATE nodes SET name = '' WHERE parent_id IS NULL", "nodes_name_check"},
		{"node name of 256 bytes", "UPDATE nodes SET name = name || 'a' WHERE parent_id IS NULL", "nodes_name_check"},
		{"empty name key", "UPDATE nodes SET name_key = ''", "nodes_name_key_check"},
		{"its own parent", "UPDATE nodes SET parent_id = id WHERE parent_id IS NOT NULL", "nodes_parent_check"},
		{"an order of NaN", "UPDATE nodes SET sort_order = 'NaN'", "nodes_sort_order_check"},
		{"an infinite order", "UPDATE nodes SET sort_order = '-Infinity'", "nodes_sort_order_check"},
		{"a source hash of 31 bytes", "UPDATE nodes SET source_hash = decode(repeat('01', 31), 'hex')", "nodes_source_hash_check"},
		{"a source hash of 33 bytes", "UPDATE nodes SET source_hash = decode(repeat('01', 33), 'hex')", "nodes_source_hash_check"},
		{"a trash root alive", "UPDATE nodes SET trash_root = true, trash_pages = 1, trash_assets = 0, trash_height = 1 " +
			"WHERE parent_id IS NULL", "nodes_trash_root_check"},
		{"a trash root without its counts", "UPDATE nodes SET trash_root = true, deleted_at = now() WHERE parent_id IS NULL",
			"nodes_trash_counts_check"},
		{"a trash root without its height", "UPDATE nodes SET trash_root = true, deleted_at = now(), trash_pages = 1, " +
			"trash_assets = 0 WHERE parent_id IS NULL", "nodes_trash_counts_check"},
		{"counts on a node no root", "UPDATE nodes SET deleted_at = now(), trash_pages = 1, trash_assets = 0, trash_height = 1 " +
			"WHERE parent_id IS NULL", "nodes_trash_counts_check"},
		{"a trash root of no node", "UPDATE nodes SET trash_root = true, deleted_at = now(), trash_pages = 0, trash_assets = 0, " +
			"trash_height = 0 WHERE parent_id IS NULL", "nodes_trash_counts_check"},
		{"a trash root of -1 attachments", "UPDATE nodes SET trash_root = true, deleted_at = now(), trash_pages = 2, " +
			"trash_assets = -1, trash_height = 1 WHERE parent_id IS NULL", "nodes_trash_counts_check"},
		{"revision 0", "UPDATE page_contents SET revision = 0", "page_contents_revision_check"},
		{"a hash of 31 bytes", "UPDATE page_contents SET content_hash = substring(content_hash from 2)", "page_contents_content_hash_check"},
		{"a size other than the content's", "UPDATE page_contents SET byte_size = 2", "page_contents_byte_size_check"},
		{"content over 5 MB", "UPDATE page_contents SET content = repeat('a', 5242881), byte_size = 5242881", "page_contents_byte_size_check"},
		{"an unknown kind of changeset", "UPDATE changesets SET kind = 'restore'", "changesets_kind_check"},
		{"a revert that names no changeset", "UPDATE changesets SET kind = 'revert'", "changesets_reverts_check"},
		{"a changeset not a revert naming one", "UPDATE changesets SET reverts_id = id", "changesets_reverts_check"},
		{"an unknown client", "UPDATE changesets SET client = 'mobile'", "changesets_client_check"},
		{"an MCP client without a name", "UPDATE changesets SET client = 'mcp:'", "changesets_client_check"},
		{"an MCP client's name with a newline", `UPDATE changesets SET client = E'mcp:a\nb'`, "changesets_client_check"},
		{"an MCP client's name of 129 characters", "UPDATE changesets SET client = 'mcp:' || repeat('名', 129)", "changesets_client_check"},
		// The four characters of the M4/P1 hand-off: C1 controls and DEL, which [:cntrl:] took or refused by the
		// locale, and a line separator, which it took (M9/P2 design 3.3.2).
		{"an MCP client's name with U+0085", "UPDATE changesets SET client = 'mcp:a' || chr(133)", "changesets_client_check"},
		{"an MCP client's name with U+009F", "UPDATE changesets SET client = 'mcp:a' || chr(159)", "changesets_client_check"},
		{"an MCP client's name with U+007F", "UPDATE changesets SET client = 'mcp:a' || chr(127)", "changesets_client_check"},
		{"an MCP client's name with U+2028", "UPDATE changesets SET client = 'mcp:a' || chr(8232)", "changesets_client_check"},
		{"an empty message", "UPDATE changesets SET message = ''", "changesets_message_check"},
		{"a message of 4097 bytes", "UPDATE changesets SET message = repeat('a', 4097)", "changesets_message_check"},
		{"half a state after", "UPDATE changeset_items SET after_sort_order = NULL", "changeset_items_state_check"},
		{"no state before or after", "UPDATE changeset_items SET after_name = NULL, after_sort_order = NULL", "changeset_items_state_check"},
		{"a parent before without a state", "UPDATE changeset_items SET before_parent_id = node_id", "changeset_items_state_check"},
		{"a restore with a before", "UPDATE changeset_items SET restored = true, before_name = 'a', before_sort_order = 0",
			"changeset_items_restored_check"},
		{"a revision with a before and an after", "UPDATE changeset_items SET revision = 1, before_name = 'a', before_sort_order = 0",
			"changeset_items_revision_state_check"},
		{"a deletion restored", "UPDATE changeset_items SET restored = true, before_name = 'a', before_sort_order = 0, " +
			"after_name = NULL, after_sort_order = NULL", "changeset_items_restored_check"},
		{"an item's revision 0", "UPDATE changeset_items SET revision = 0", "changeset_items_revision_check"},
		{"a version of revision 0", "UPDATE page_revisions SET revision = 0", "page_revisions_revision_check"},
		{"a version based on itself", "UPDATE page_revisions SET base_revision = revision", "page_revisions_revision_check"},
		{"a version's hash of 31 bytes", "UPDATE page_revisions SET content_hash = substring(content_hash from 2)", "page_revisions_content_hash_check"},
		{"a version's size other than its content's", "UPDATE page_revisions SET byte_size = 2", "page_revisions_byte_size_check"},
		{"a version over 5 MB", "UPDATE page_revisions SET content = repeat('a', 5242881), byte_size = 5242881", "page_revisions_byte_size_check"},
		{"a session's unknown client", "UPDATE edit_sessions SET client = 'mobile'", "edit_sessions_client_check"},
		{"a session's MCP client without a name", "UPDATE edit_sessions SET client = 'mcp:'", "edit_sessions_client_check"},
		{"a session's MCP client's name with a newline", `UPDATE edit_sessions SET client = E'mcp:a\nb'`, "edit_sessions_client_check"},
		{"a session's MCP client's name of 129 characters", "UPDATE edit_sessions SET client = 'mcp:' || repeat('名', 129)",
			"edit_sessions_client_check"},
		{"a session's MCP client's name with U+0085", "UPDATE edit_sessions SET client = 'mcp:a' || chr(133)", "edit_sessions_client_check"},
		{"a session's MCP client's name with U+009F", "UPDATE edit_sessions SET client = 'mcp:a' || chr(159)", "edit_sessions_client_check"},
		{"a session's MCP client's name with U+007F", "UPDATE edit_sessions SET client = 'mcp:a' || chr(127)", "edit_sessions_client_check"},
		{"a session's MCP client's name with U+2028", "UPDATE edit_sessions SET client = 'mcp:a' || chr(8232)", "edit_sessions_client_check"},
		{"a session's changeset without its revision", "UPDATE edit_sessions SET revision = NULL", "edit_sessions_written_check"},
		{"a session's revision without its changeset", "UPDATE edit_sessions SET changeset_id = NULL", "edit_sessions_written_check"},
		{"a session's revision 0", "UPDATE edit_sessions SET revision = 0", "edit_sessions_written_check"},
		{"a session expiring when opened", "UPDATE edit_sessions SET expires_at = created_at", "edit_sessions_expires_at_check"},
		{"a session's end reason without who or when", "UPDATE edit_sessions SET ended_reason = 'unlocked'", "edit_sessions_ended_check"},
		{"a session's end without its reason", "UPDATE edit_sessions SET ended_by_id = user_id, ended_at = created_at", "edit_sessions_ended_check"},
		{"a session ended before it opened", "UPDATE edit_sessions SET ended_reason = 'unlocked', ended_by_id = user_id, " +
			"ended_at = created_at - interval '1 second'", "edit_sessions_ended_check"},
		{"a session's unknown end reason", "UPDATE edit_sessions SET ended_reason = 'expired', ended_by_id = user_id, ended_at = created_at",
			"edit_sessions_ended_reason_check"},
		{"an indexed revision 0", "UPDATE indexed_pages SET revision = 0", "indexed_pages_revision_check"},
		{"an extractor 0", "UPDATE indexed_pages SET extractor = 0", "indexed_pages_extractor_check"},
		{"a link before the content", "UPDATE page_links SET range_start = -1", "page_links_range_start_check"},
		{"an empty link range", "UPDATE page_links SET range_end = range_start", "page_links_range_check"},
		{"a fifth kind of link", "UPDATE page_links SET kind = 'tag'", "page_links_kind_check"},
		{"an empty target", "UPDATE page_links SET target = ''", "page_links_target_check"},
		{"ambiguous and resolved to nothing", "UPDATE page_links SET resolved_id = NULL", "page_links_ambiguous_check"},
		{"an attachment resolved to nothing", "UPDATE page_links SET resolved_id = NULL, ambiguous = false, resolved_asset = true",
			"page_links_resolved_asset_check"},
		{"an empty tag key", "UPDATE page_tags SET tag_key = ''", "page_tags_tag_key_check"},
		{"an empty tag", "UPDATE page_tags SET tag = ''", "page_tags_tag_check"},
		{"a tag written 0 times", "UPDATE page_tags SET count = 0", "page_tags_count_check"},
		{"a property before the first", "UPDATE page_properties SET position = -1", "page_properties_position_check"},
		{"an empty alias key", "UPDATE page_aliases SET alias_key = ''", "page_aliases_alias_key_check"},
		{"an empty alias", "UPDATE page_aliases SET alias = ''", "page_aliases_alias_check"},
		{"a search row of revision 0", "UPDATE page_search SET revision = 0 WHERE body IS NOT NULL", "page_search_revision_check"},
		{"an attachment's search row with a body", "UPDATE page_search SET body = title WHERE revision IS NULL", "page_search_body_check"},
		{"a page's search row without a body", "UPDATE page_search SET body = NULL WHERE revision IS NOT NULL", "page_search_body_check"},
		{"an attachment's search row partial", "UPDATE page_search SET partial = true WHERE body IS NULL", "page_search_partial_check"},
		{"a file of no type", "UPDATE asset_blobs SET mime = ''", "asset_blobs_mime_check"},
		{"a file of -1 bytes", "UPDATE asset_blobs SET byte_size = -1", "asset_blobs_byte_size_check"},
		{"a file's hash of 31 bytes", "UPDATE asset_blobs SET sha256 = substring(sha256 from 2)", "asset_blobs_sha256_check"},
		{"a width of 0", "UPDATE asset_blobs SET width = 0", "asset_blobs_width_check"},
		{"a height of 0", "UPDATE asset_blobs SET height = 0", "asset_blobs_height_check"},
		{"a width without its height", "UPDATE asset_blobs SET height = NULL", "asset_blobs_size_check"},
		{"a height without its width", "UPDATE asset_blobs SET width = NULL", "asset_blobs_size_check"},
		{"a sixth state of a job", "UPDATE transfer_jobs SET state = 'paused', finished_at = now(), report = '{}'", "transfer_jobs_state_check"},
		{"a fifth kind of job", "UPDATE transfer_jobs SET kind = 'copy', changeset_id = gen_random_uuid()", "transfer_jobs_kind_check"},
		{"a job of no name", "UPDATE transfer_jobs SET name = ''", "transfer_jobs_name_check"},
		{"a job's name of 256 bytes", "UPDATE transfer_jobs SET name = repeat('a', 256)", "transfer_jobs_name_check"},
		{"a job of the command line", "UPDATE transfer_jobs SET client = 'cli'", "transfer_jobs_client_check"},
		{"more done than all", "UPDATE transfer_jobs SET progress_done = 3", "transfer_jobs_progress_check"},
		{"less than nothing done", "UPDATE transfer_jobs SET progress_done = -1", "transfer_jobs_progress_check"},
		{"a running job that did not start", "UPDATE transfer_jobs SET started_at = NULL", "transfer_jobs_started_check"},
		{"a running job that never beat", "UPDATE transfer_jobs SET heartbeat_at = NULL", "transfer_jobs_started_check"},
		{"a queued job that started", "UPDATE transfer_jobs SET state = 'queued'", "transfer_jobs_started_check"},
		{"a cancel asked of a job that never started", "UPDATE transfer_jobs SET state = 'queued', started_at = NULL, heartbeat_at = NULL, " +
			"cancel_requested_at = now()", "transfer_jobs_started_check"},
		{"a running job that ended", "UPDATE transfer_jobs SET finished_at = now()", "transfer_jobs_finished_check"},
		{"a queued job's report", "UPDATE transfer_jobs SET state = 'queued', started_at = NULL, heartbeat_at = NULL, report = '{}'",
			"transfer_jobs_finished_check"},
		{"an ended job without its report", "UPDATE transfer_jobs SET state = 'failed', finished_at = now()", "transfer_jobs_finished_check"},
		{"an ended job without its time", "UPDATE transfer_jobs SET state = 'failed', report = '{}'", "transfer_jobs_finished_check"},
		{"an import expired", "UPDATE transfer_jobs SET kind = 'import', state = 'expired', finished_at = now(), report = '{}'",
			"transfer_jobs_expired_check"},
		{"an export succeeded without its bytes", "UPDATE transfer_jobs SET state = 'succeeded', finished_at = now(), report = '{}'",
			"transfer_jobs_result_bytes_check"},
		{"an export failed with bytes", "UPDATE transfer_jobs SET state = 'failed', finished_at = now(), report = '{}', result_bytes = 1",
			"transfer_jobs_result_bytes_check"},
		{"a running export's bytes", "UPDATE transfer_jobs SET result_bytes = 1", "transfer_jobs_result_bytes_check"},
		{"an archive of -1 bytes", "UPDATE transfer_jobs SET state = 'succeeded', finished_at = now(), report = '{}', result_bytes = -1",
			"transfer_jobs_result_bytes_check"},
		// An upgrade's baseline (M11/P1 design 3.2): each conjunct alone broken, each = both ways; the baseline page,
		// the baseline attachment and the protected row each broken where the shape is theirs.
		{"a baseline row of a folder", "UPDATE upgrade_nodes SET kind = 'folder' WHERE kind = 'asset'", "upgrade_nodes_kind_check"},
		{"a baseline row without its name", "UPDATE upgrade_nodes SET name = NULL, sort_order = NULL WHERE kind = 'asset'",
			"upgrade_nodes_baseline_check"},
		{"a protected row with a name", "UPDATE upgrade_nodes SET name = 'raw', sort_order = 0 WHERE NOT baseline",
			"upgrade_nodes_baseline_check"},
		{"a name without its order", "UPDATE upgrade_nodes SET sort_order = NULL WHERE kind = 'asset'", "upgrade_nodes_baseline_check"},
		{"an order without its name", "UPDATE upgrade_nodes SET sort_order = 0 WHERE NOT baseline", "upgrade_nodes_baseline_check"},
		{"a protected attachment", "UPDATE upgrade_nodes SET kind = 'asset' WHERE NOT baseline", "upgrade_nodes_baseline_check"},
		{"a protected row under a page", "UPDATE upgrade_nodes SET parent_id = gen_random_uuid() WHERE NOT baseline",
			"upgrade_nodes_baseline_check"},
		{"a baseline page without its version", "UPDATE upgrade_nodes SET revision = NULL, content_hash = NULL, byte_size = NULL, " +
			"wanted = NULL, accepted_hash = NULL WHERE baseline AND kind = 'page'", "upgrade_nodes_page_check"},
		{"a baseline attachment with a version", "UPDATE upgrade_nodes SET revision = 1, content_hash = sha256('x'), byte_size = 1, " +
			"wanted = 0, accepted_hash = sha256('x') WHERE kind = 'asset'", "upgrade_nodes_page_check"},
		{"a baseline page without its hash", "UPDATE upgrade_nodes SET content_hash = NULL WHERE baseline AND kind = 'page'",
			"upgrade_nodes_page_check"},
		{"a baseline page without its size", "UPDATE upgrade_nodes SET byte_size = NULL WHERE baseline AND kind = 'page'",
			"upgrade_nodes_page_check"},
		{"a baseline page without its wanted", "UPDATE upgrade_nodes SET wanted = NULL WHERE baseline AND kind = 'page'",
			"upgrade_nodes_page_check"},
		{"a baseline page without its accepted hash", "UPDATE upgrade_nodes SET accepted_hash = NULL WHERE baseline AND kind = 'page'",
			"upgrade_nodes_page_check"},
		{"a protected row with a hash", "UPDATE upgrade_nodes SET content_hash = sha256('x') WHERE NOT baseline", "upgrade_nodes_page_check"},
		{"a protected row with a size", "UPDATE upgrade_nodes SET byte_size = 1 WHERE NOT baseline", "upgrade_nodes_page_check"},
		{"a protected row with a wanted", "UPDATE upgrade_nodes SET wanted = 0 WHERE NOT baseline", "upgrade_nodes_page_check"},
		{"a protected row with an accepted hash", "UPDATE upgrade_nodes SET accepted_hash = sha256('x') WHERE NOT baseline",
			"upgrade_nodes_page_check"},
		{"a page wanting 10,001 targets", "UPDATE upgrade_nodes SET wanted = 10001 WHERE baseline AND kind = 'page'",
			"upgrade_nodes_wanted_check"},
		{"a page wanting -1 targets", "UPDATE upgrade_nodes SET wanted = -1 WHERE baseline AND kind = 'page'", "upgrade_nodes_wanted_check"},
		// An upgrade's row (M11/P1 design 3.10): each conjunct alone broken, an = broken each way.
		{"an upgrade's unknown state", upgrade + "state = 'done'", "notebook_upgrades_state_check"},
		{"an upgrade's unknown ending", upgrade + "state = 'rolling_back', ending = 'cancelled', rollback_job_id = gen_random_uuid()",
			"notebook_upgrades_ending_check"},
		{"an upgrade's unknown language", upgrade + "locale = 'fr'", "notebook_upgrades_locale_check"},
		{"a baseline of -1 nodes", upgrade + "baseline_nodes = -1", "notebook_upgrades_baseline_check"},
		{"more baseline pages than nodes", upgrade + "baseline_pages = 3", "notebook_upgrades_baseline_check"},
		{"10,001 wanted at the start", upgrade + "baseline_wanted = 10001, baseline_wanted_capped = false",
			"notebook_upgrades_baseline_check"},
		{"capped under 10,000 wanted", upgrade + "baseline_wanted = 9999", "notebook_upgrades_baseline_check"},
		{"10,000 wanted not capped", upgrade + "baseline_wanted_capped = false", "notebook_upgrades_baseline_check"},
		{"rolling back without its ending", upgrade + "state = 'rolling_back', rollback_job_id = gen_random_uuid()",
			"notebook_upgrades_ending_state_check"},
		{"an ending while upgrading", upgrade + "ending = 'abandoned'", "notebook_upgrades_ending_state_check"},
		{"a submission without its id", upgrade + rollingBack + ", submitted_at = now(), submitted_by_id = " + submitter,
			"notebook_upgrades_submission_check"},
		{"a submission without its time", upgrade + rollingBack + ", submission_id = gen_random_uuid(), submitted_by_id = " + submitter,
			"notebook_upgrades_submission_check"},
		{"a submission without its submitter", upgrade + rollingBack + ", submission_id = gen_random_uuid(), submitted_at = now()",
			"notebook_upgrades_submission_check"},
		{"a submission's id alone", upgrade + rollingBack + ", submission_id = gen_random_uuid()", "notebook_upgrades_submission_check"},
		{"a submission's submitter alone", upgrade + rollingBack + ", submitted_by_id = " + submitter,
			"notebook_upgrades_submission_check"},
		{"a submission while upgrading", upgrade + submission, "notebook_upgrades_submission_state_check"},
		{"reviewing without a submission", upgrade + "state = 'reviewing'", "notebook_upgrades_submission_state_check"},
		{"a report while upgrading", upgrade + submission + ", report = '{}'", "notebook_upgrades_report_check"},
		{"a report without a submission", upgrade + rollingBack + ", report = '{}'", "notebook_upgrades_report_check"},
		{"a report not an object", upgrade + "state = 'reviewing', " + submission + ", report = '[]'", "notebook_upgrades_report_check"},
		{"a report of more than 65,536 bytes", upgrade + "state = 'reviewing', " + submission +
			", report = jsonb_build_object('r', repeat('x', 65536))", "notebook_upgrades_report_check"},
		{"approved without its report", upgrade + "state = 'approved', " + submission, "notebook_upgrades_approved_check"},
		{"rejected without a submission", upgrade + "state = 'rolling_back', ending = 'rejected', rollback_job_id = gen_random_uuid()",
			"notebook_upgrades_rejected_check"},
		{"rolling back without its job", upgrade + "state = 'rolling_back', ending = 'abandoned'", "notebook_upgrades_rollback_job_check"},
		{"a rollback's job while upgrading", upgrade + "rollback_job_id = gen_random_uuid()", "notebook_upgrades_rollback_job_check"},
		{"rolled back without its end", upgrade + rolledBack, "notebook_upgrades_rollback_end_check"},
		{"a rollback's end while upgrading", upgrade + "rollback_complete = true", "notebook_upgrades_rollback_end_check"},
		{"a complete rollback with what it left", upgrade + rolledBack + ", rollback_complete = true, rollback_left_nodes = 0, " +
			"rollback_left_pages = 0, rollback_left_created = 0", "notebook_upgrades_rollback_end_check"},
		{"a short rollback without its nodes left", upgrade + rolledBack + ", rollback_complete = false, rollback_left_pages = 1, " +
			"rollback_left_created = 1", "notebook_upgrades_rollback_end_check"},
		{"a short rollback without its pages left", upgrade + rolledBack + ", rollback_complete = false, rollback_left_nodes = 1, " +
			"rollback_left_created = 1", "notebook_upgrades_rollback_end_check"},
		{"a short rollback without its new nodes left", upgrade + rolledBack + ", rollback_complete = false, rollback_left_nodes = 1, " +
			"rollback_left_pages = 1", "notebook_upgrades_rollback_end_check"},
		// The writing jobs' columns (M8/P4 design 3.2): each check alone broken, the others kept.
		{"an export's changeset", "UPDATE transfer_jobs SET changeset_id = gen_random_uuid()", "transfer_jobs_changeset_check"},
		{"a restore without its changeset", "UPDATE transfer_jobs SET kind = 'trash_restore', root_id = gen_random_uuid()",
			"transfer_jobs_changeset_check"},
		{"a revert without its changeset", "UPDATE transfer_jobs SET kind = 'revert', target_changeset_id = gen_random_uuid()",
			"transfer_jobs_changeset_check"},
		{"a revert of nothing", "UPDATE transfer_jobs SET kind = 'revert', changeset_id = gen_random_uuid()", "transfer_jobs_target_check"},
		{"a restore's target", "UPDATE transfer_jobs SET kind = 'trash_restore', root_id = gen_random_uuid(), " +
			"changeset_id = gen_random_uuid(), target_changeset_id = gen_random_uuid()", "transfer_jobs_target_check"},
		{"a restore's message", "UPDATE transfer_jobs SET kind = 'trash_restore', root_id = gen_random_uuid(), " +
			"changeset_id = gen_random_uuid(), message = 'm'", "transfer_jobs_message_check"},
		{"a revert's empty message", "UPDATE transfer_jobs SET kind = 'revert', changeset_id = gen_random_uuid(), " +
			"target_changeset_id = gen_random_uuid(), message = ''", "transfer_jobs_message_check"},
		{"a revert's message of 4,097 bytes", "UPDATE transfer_jobs SET kind = 'revert', changeset_id = gen_random_uuid(), " +
			"target_changeset_id = gen_random_uuid(), message = repeat('a', 4097)", "transfer_jobs_message_check"},
		{"a restore of no item", "UPDATE transfer_jobs SET kind = 'trash_restore', changeset_id = gen_random_uuid()",
			"transfer_jobs_root_check"},
		{"a revert under a page", "UPDATE transfer_jobs SET kind = 'revert', root_id = gen_random_uuid(), " +
			"changeset_id = gen_random_uuid(), target_changeset_id = gen_random_uuid()", "transfer_jobs_root_check"},
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
	// A notebook has one upgrade under way at most (M11/P1 design 3.10): a second is a unique violation of the
	// partial index; one that ended, approved, stays beside it.
	another := func(state string) string {
		report := "NULL"
		if state == "approved" {
			report = "'{}'"
		}
		return "INSERT INTO notebook_upgrades (id, notebook_id, state, source_root_id, seeded_root, locale, baseline_nodes, " +
			"baseline_pages, baseline_wanted, baseline_wanted_capped, submission_id, submitted_at, submitted_by_id, report, " +
			"created_by_id, updated_by_id, created_at, updated_at) VALUES (gen_random_uuid(), '0199a2b4-0000-7000-8000-000000000019', '" +
			state + "', gen_random_uuid(), false, 'en', 0, 0, 0, false, gen_random_uuid(), now(), " + submitter + ", " + report + ", " +
			submitter + ", " + submitter + ", now(), now())"
	}
	_, err := pool.Exec(ctx, another("reviewing"))
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "notebook_upgrades_notebook_id_key" {
		t.Errorf("a second upgrade under way = %v, want unique_violation (23505) of notebook_upgrades_notebook_id_key", err)
	}
	if _, err := pool.Exec(ctx, another("approved")); err != nil {
		t.Errorf("an approved upgrade beside the one under way = %v, want it kept", err)
	}
	// Each client check, as the database holds it, refuses exactly the code points shared.Unshowable refuses
	// in a name, but NUL, which text holds not (M9/P2 design 3.3.2): one statement a table, PostgreSQL reading
	// its own expression. A Go of another Unicode, whose set differs, fails here: a migration follows it
	// (v0.1 design 13.4 rule 13).
	var unshowable []int32
	for r := rune(1); r <= unicode.MaxRune; r++ {
		if (r < 0xd800 || r > 0xdfff) && shared.Unshowable(r) {
			unshowable = append(unshowable, r)
		}
	}
	for _, check := range []string{"changesets_client_check", "edit_sessions_client_check"} {
		var expr string
		if err := pool.QueryRow(ctx, "SELECT pg_get_expr(conbin, conrelid) FROM pg_constraint WHERE conname = $1", check).Scan(&expr); err != nil {
			t.Fatalf("%s: %v", check, err)
		}
		rows, err := pool.Query(ctx, "SELECT n FROM (SELECT n, 'mcp:a' || chr(n) AS client FROM generate_series(1, 1114111) n "+
			"WHERE n < 55296 OR n > 57343) s WHERE NOT ("+expr+") ORDER BY n")
		if err != nil {
			t.Fatalf("%s: %v", check, err)
		}
		refused, err := pgx.CollectRows(rows, pgx.RowTo[int32])
		if err != nil {
			t.Fatalf("%s: %v", check, err)
		}
		if !slices.Equal(refused, unshowable) {
			t.Errorf("%s refuses %d code points, shared.Unshowable %d; the first that differ: %v",
				check, len(refused), len(unshowable), firstDifferences(refused, unshowable))
		}
	}
}

// firstDifferences are the first few code points of one list and not the
// other, as U+XXXX.
func firstDifferences(a, b []int32) []string {
	var diff []string
	for _, pair := range [][2][]int32{{a, b}, {b, a}} {
		for _, r := range pair[0] {
			if _, found := slices.BinarySearch(pair[1], r); !found && len(diff) < 10 {
				diff = append(diff, fmt.Sprintf("U+%04X", r))
			}
		}
	}
	return diff
}

// The client checks take a name of what Client.Valid takes: a space, a
// no-break space, the format characters but the bidirectional controls (a
// soft hyphen, the zero-width space and joiners, a byte order mark, the
// separators U+2027 and U+202F, the invisible operators and the deprecated
// format characters), and 128 characters. Client.Valid is shared.Unshowable
// with mcp: and the length, checked by page/domain's tests; these import no
// module (v0.1 design 13.1 rule 11).
func TestTheClientChecksTakeWhatValidTakes(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	checkedRows(t, pool)
	for _, table := range []string{"changesets", "edit_sessions"} {
		for _, n := range []int{32, 160, 173, 8203, 8204, 8205, 8231, 8239, 8293, 8298, 65279} {
			if _, err := pool.Exec(ctx, "UPDATE "+table+" SET client = 'mcp:a' || chr($1)", n); err != nil {
				t.Errorf("%s's client with U+%04X: %v", table, n, err)
			}
		}
		if _, err := pool.Exec(ctx, "UPDATE "+table+" SET client = 'mcp:' || repeat('名', 128)"); err != nil {
			t.Errorf("%s's client of 128 characters: %v", table, err)
		}
	}
}
