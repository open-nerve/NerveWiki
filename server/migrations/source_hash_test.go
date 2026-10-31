package migrations_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// 00050 derives hashes from existing live LLM source descendants, including
// empty attachments, without changing their bytes or history. This starts at
// a real schema 49: it never creates 50 and drops it to manufacture the old DB.
func TestSourceHashSchemaAndBackfill(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewEmptyDatabase(t))
	oldFiles := fstest.MapFS{}
	through50Files := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS(), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() > "00050_zzzz.sql" {
			continue
		}
		data, err := fs.ReadFile(migrations.FS(), e.Name())
		if err != nil {
			t.Fatal(err)
		}
		through50Files[e.Name()] = &fstest.MapFile{Data: data, Mode: 0o444}
		if e.Name() <= "00049_zzzz.sql" {
			oldFiles[e.Name()] = through50Files[e.Name()]
		}
	}
	old, err := postgres.NewMigrator(pool, oldFiles)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = old.Close() })
	if up, err := old.Up(ctx); err != nil || len(up) != 49 || up[len(up)-1].Version != 49 {
		t.Fatalf("initial Up = %+v, %v; want exactly migrations 1 through 49", up, err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	assertSourceHashColumn(t, pool, false)
	want := seedSourceHashMigration(t, pool)
	before := sourceMigrationData(t, pool)
	// Keep this historical Up/Down/Up scoped to 50 as later migrations are added.
	m, err := postgres.NewMigrator(pool, through50Files)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	up, err := m.Up(ctx)
	if err != nil {
		t.Fatalf("Up with existing LLM data: %v", err)
	}
	// Check existence before querying the new column or rolling back 50. The
	// missing implementation must fail here, not with an undefined-column error.
	assertSourceHashColumn(t, pool, true)
	if len(up) != 1 || up[0].Version != 50 {
		t.Fatalf("Up from 49 = %+v, want migration 50 alone", up)
	}
	assertSourceHashCatalog(t, pool)
	assertSourceMigrationHashes(t, pool, want)
	if got := sourceMigrationData(t, pool); !reflect.DeepEqual(got, before) {
		t.Fatalf("backfill changed existing tree, mode, bytes or history: got %v, want %v", got, before)
	}
	// Both ends of the CHECK matter; NULL and duplicate 32-byte values are
	// legal for different node IDs. The catalog assertion independently checks
	// that the four-column index is nonunique; UNIQUE including id would still
	// allow these equal three-column source keys.
	var page uuid.UUID
	for id, hash := range want {
		if hash != nil {
			page = id
			break
		}
	}
	for _, size := range []int{31, 33} {
		_, err := pool.Exec(ctx, "UPDATE nodes SET source_hash = $2 WHERE id = $1", page, bytes.Repeat([]byte{1}, size))
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "nodes_source_hash_check" {
			t.Errorf("source hash of %d bytes = %v, want nodes_source_hash_check", size, err)
		}
	}
	if _, err := pool.Exec(ctx, "UPDATE nodes SET source_hash = NULL WHERE id = $1", page); err != nil {
		t.Fatalf("NULL source hash: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE nodes SET source_hash = $2 WHERE id = $1", page, want[page]); err != nil {
		t.Fatal(err)
	}
	if back, err := m.Down(ctx); err != nil || back == nil || back.Version != 50 {
		t.Fatalf("Down = %+v, %v, want migration 50 alone", back, err)
	}
	assertSourceHashColumn(t, pool, false)
	if got := sourceMigrationData(t, pool); !reflect.DeepEqual(got, before) {
		t.Fatalf("Down changed existing tree, mode, bytes or history: got %v, want %v", got, before)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up after Down: %v", err)
	}
	assertSourceHashColumn(t, pool, true)
	assertSourceHashCatalog(t, pool)
	assertSourceMigrationHashes(t, pool, want)
	if got := sourceMigrationData(t, pool); !reflect.DeepEqual(got, before) {
		t.Fatalf("second backfill changed existing tree, mode, bytes or history: got %v, want %v", got, before)
	}
}

func assertSourceHashColumn(t *testing.T, pool *pgxpool.Pool, present bool) {
	t.Helper()
	var count int
	var typ, nullable, defaultValue string
	if err := pool.QueryRow(context.Background(), `SELECT count(*), coalesce(max(data_type), ''),
		coalesce(max(is_nullable), ''), coalesce(max(column_default), '') FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'nodes' AND column_name = 'source_hash'`).
		Scan(&count, &typ, &nullable, &defaultValue); err != nil {
		t.Fatal(err)
	}
	if (count == 1) != present {
		t.Fatalf("nodes.source_hash column count = %d, want present %t", count, present)
	}
	if present && (typ != "bytea" || nullable != "YES" || defaultValue != "") {
		t.Errorf("nodes.source_hash = type %q, nullable %q, default %q; want bytea, YES, no default", typ, nullable, defaultValue)
	}
}

func assertSourceHashCatalog(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var unique bool
	var keys []string
	var predicate string
	if err := pool.QueryRow(context.Background(), `SELECT i.indisunique,
		ARRAY(SELECT pg_get_indexdef(i.indexrelid, key, true) FROM generate_series(1, i.indnkeyatts) key),
		pg_get_expr(i.indpred, i.indrelid)
		FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
		WHERE c.relname = 'nodes_notebook_id_kind_source_hash_idx' AND i.indrelid = 'nodes'::regclass`).
		Scan(&unique, &keys, &predicate); err != nil {
		t.Fatal(err)
	}
	if unique || !reflect.DeepEqual(keys, []string{"notebook_id", "kind", "source_hash", "id"}) ||
		predicate != "((source_hash IS NOT NULL) AND (deleted_at IS NULL))" {
		t.Errorf("source index = unique %t, keys %q, predicate %q; want nonunique notebook/kind/hash/id, live nonNULL", unique, keys, predicate)
	}
	var checks int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_constraint
		WHERE conrelid = 'nodes'::regclass AND conname = 'nodes_source_hash_check' AND contype = 'c'`).Scan(&checks); err != nil {
		t.Fatal(err)
	}
	if checks != 1 {
		t.Errorf("nodes_source_hash_check count = %d, want 1", checks)
	}
}

// The expectations are assigned per fixture row, independently of the
// migration's recursive SQL. Two source pages deliberately have equal hashes.
func seedSourceHashMigration(t *testing.T, pool *pgxpool.Pool) map[uuid.UUID][]byte {
	t.Helper()
	ctx := context.Background()
	user, workspace := uuid.NewV7(), uuid.NewV7()
	at := time.Date(2026, 10, 27, 1, 2, 3, 0, time.UTC)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	exec(`INSERT INTO users (id, email, password, display_name, created_at, updated_at)
		VALUES ($1, 'source-migration@corp.com', 'x', 'Source', $2, $2)`, user, at)
	exec(`INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at)
		VALUES ($1, 'source-migration', 'Source', $2, $2, $3, $3)`, workspace, user, at)
	want := map[uuid.UUID][]byte{}
	newNotebook := func(name string) uuid.UUID {
		id := uuid.NewV7()
		exec(`INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at)
			VALUES ($1, $2, $3, 'editor', $4, $4, $5, $5)`, id, workspace, name, user, at)
		return id
	}
	node := func(nb uuid.UUID, parent *uuid.UUID, name, kind, content string, deleted, source, metadata bool) uuid.UUID {
		id := uuid.NewV7()
		var gone *time.Time
		if deleted {
			gone = &at
		}
		exec(`INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order,
			created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			VALUES ($1, $2, $3, $4, $5, $5, 0, $6, $6, $7, $7, $8)`, id, nb, parent, kind, name, user, at, gone)
		want[id] = nil
		sum := sha256.Sum256([]byte(content))
		if metadata && kind == "page" {
			exec(`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at, deleted_at)
				VALUES ($1, $2, 1, $3, $4, $5, $6, $7)`, id, content, sum[:], len(content), user, at, gone)
			cs := uuid.NewV7()
			exec(`INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
				VALUES ($1, $2, 'edit', 'api', $3, $4, $4)`, cs, nb, user, at)
			exec(`INSERT INTO page_revisions (id, changeset_id, node_id, revision, content, content_hash, byte_size,
				created_at, updated_at, deleted_at, rewrite) VALUES ($1, $2, $3, 1, $4, $5, $6, $7, $7, $8, true)`,
				uuid.NewV7(), cs, id, content, sum[:], len(content), at, gone)
		}
		if metadata && kind == "asset" {
			exec(`INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at, deleted_at)
				VALUES ($1, $2, $3, 'text/plain', $4, $5, $6, $7, $8)`, uuid.NewV7(), id, nb, len(content), sum[:], user, at, gone)
		}
		if source {
			want[id] = sum[:]
		}
		return id
	}
	wiki := newNotebook("wiki")
	root := node(wiki, nil, "raw", "page", "root explanation", false, false, true)
	exec("UPDATE notebooks SET mode = 'llm_wiki', source_root_id = $2 WHERE id = $1", wiki, root)
	node(wiki, nil, "agents", "page", "instructions", false, false, true)
	parent := node(wiki, &root, "source", "page", "same source", false, true, true)
	node(wiki, &parent, "nested", "page", "same source", false, true, true)
	node(wiki, &root, "empty", "page", "", false, false, true)
	node(wiki, &parent, "file", "asset", "file bytes", false, true, true)
	node(wiki, &root, "zero", "asset", "", false, true, true)
	node(wiki, nil, "outside", "page", "outside", false, false, true)
	node(wiki, nil, "outside-file", "asset", "outside bytes", false, false, true)
	trash := node(wiki, &root, "trash", "page", "trash", true, false, true)
	node(wiki, &trash, "under-trash", "page", "invalid live descendant", false, false, true)
	node(wiki, &root, "trash-file", "asset", "old file", true, false, true)
	node(wiki, &root, "missing-content", "page", "", false, false, false)
	node(wiki, &root, "missing-blob", "asset", "", false, false, false)
	standard := newNotebook("standard")
	standardRoot := node(standard, nil, "raw", "page", "root", false, false, true)
	node(standard, &standardRoot, "child", "page", "standard child", false, false, true)
	exec("UPDATE notebooks SET source_root_id = $2 WHERE id = $1", standard, standardRoot)
	deleted := newNotebook("deleted")
	deletedRoot := node(deleted, nil, "raw", "page", "root", false, false, true)
	node(deleted, &deletedRoot, "child", "page", "deleted notebook child", false, false, true)
	exec("UPDATE notebooks SET mode = 'llm_wiki', source_root_id = $2, deleted_at = $3 WHERE id = $1", deleted, deletedRoot, at)
	foreign := newNotebook("foreign-root")
	node(foreign, nil, "own", "page", "not the other notebook's source", false, false, true)
	exec("UPDATE notebooks SET mode = 'llm_wiki', source_root_id = $2 WHERE id = $1", foreign, root)
	missing := newNotebook("missing-root")
	node(missing, nil, "own", "page", "missing root", false, false, true)
	exec("UPDATE notebooks SET mode = 'llm_wiki', source_root_id = $2 WHERE id = $1", missing, uuid.NewV7())
	return want
}

func assertSourceMigrationHashes(t *testing.T, pool *pgxpool.Pool, want map[uuid.UUID][]byte) {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT id, source_hash FROM nodes")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen, sources := 0, 0
	for rows.Next() {
		var id uuid.UUID
		var hash []byte
		if err := rows.Scan(&id, &hash); err != nil {
			t.Fatal(err)
		}
		expected, ok := want[id]
		if !ok || !bytes.Equal(hash, expected) || (hash == nil) != (expected == nil) {
			t.Errorf("node %s source hash = %x, want %x (known %t)", id, hash, expected, ok)
		}
		seen++
		if hash != nil {
			sources++
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if seen != len(want) || sources != 4 {
		t.Errorf("backfill checked %d rows and %d sources, want %d rows and 4 nonempty-page/any-asset sources", seen, sources, len(want))
	}
}

func sourceMigrationData(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range []string{"notebooks", "nodes", "page_contents", "page_revisions", "asset_blobs"} {
		value := "to_jsonb(x)"
		if table == "nodes" {
			value += " - 'source_hash'"
		}
		query := "SELECT coalesce(string_agg(v, E'\\n' ORDER BY v), '') FROM (SELECT (" + value + ")::text AS v FROM " + table + " x) s"
		var data string
		if err := pool.QueryRow(context.Background(), query).Scan(&data); err != nil {
			t.Fatal(err)
		}
		out[table] = data
	}
	return out
}
