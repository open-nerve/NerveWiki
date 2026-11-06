package migrations_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// 00054 chooses LZ4 for new compressible content in both tables. Neither Up
// nor Down may rewrite existing pglz, LZ4 or short values. Down restores the
// default policy; it does not remove the need to read already stored LZ4.
func TestPageContentCompressionMigration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewEmptyDatabase(t))
	oldFiles, through54Files := fstest.MapFS{}, fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS(), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() > "00054_zzzz.sql" {
			continue
		}
		data, err := fs.ReadFile(migrations.FS(), entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		through54Files[entry.Name()] = &fstest.MapFile{Data: data, Mode: 0o444}
		if entry.Name() <= "00053_zzzz.sql" {
			oldFiles[entry.Name()] = through54Files[entry.Name()]
		}
	}
	old, err := postgres.NewMigrator(pool, oldFiles)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = old.Close() })
	if up, err := old.Up(ctx); err != nil || len(up) != 53 || up[len(up)-1].Version != 53 {
		t.Fatalf("initial Up = %+v, %v; want exactly migrations 1 through 53", up, err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	assertContentCompressionAttributes(t, pool, "schema 53", "")

	// Only the old-data transaction fixes a session default. It constructs
	// real pglz values without setting either product column's compression.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	exec("SET LOCAL default_toast_compression = 'pglz'")
	user, workspace, notebook := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	created := time.Date(2026, 11, 6, 1, 2, 3, 456000, time.UTC)
	updated := created.Add(time.Hour)
	exec(`INSERT INTO users (id, email, password, display_name, created_at, updated_at)
		VALUES ($1, 'compression@corp.com', 'x', 'Compression', $2, $3)`, user, created, updated)
	exec(`INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at)
		VALUES ($1, 'compression', 'Compression', $2, $2, $3, $4)`, workspace, user, created, updated)
	exec(`INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at)
		VALUES ($1, $2, 'Compression', 'editor', $3, $3, $4, $5)`, notebook, workspace, user, created, updated)

	want := map[string]contentCompressionValue{}
	insert := func(name, current, history, compression string) {
		t.Helper()
		node, changeset := uuid.NewV7(), uuid.NewV7()
		exec(`INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order,
			created_by_id, updated_by_id, created_at, updated_at)
			VALUES ($1, $2, 'page', $3, $3, 0, $4, $4, $5, $6)`, node, notebook, name, user, created, updated)
		exec(`INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
			VALUES ($1, $2, 'edit', 'api', $3, $4, $5)`, changeset, notebook, user, created, updated)
		sum := sha256.Sum256([]byte(current))
		exec(`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
			VALUES ($1, $2, 3, $3, $4, $5, $6)`, node, current, sum[:], len(current), user, updated)
		sum = sha256.Sum256([]byte(history))
		exec(`INSERT INTO page_revisions (id, changeset_id, node_id, base_revision, revision,
			content, content_hash, byte_size, created_at, updated_at, rewrite)
			VALUES ($1, $2, $3, 1, 2, $4, $5, $6, $7, $8, true)`,
			uuid.NewV7(), changeset, node, history, sum[:], len(history), created, updated)
		want["page_contents/"+node.String()] = contentCompressionValue{current, compression}
		want["page_revisions/"+node.String()] = contentCompressionValue{history, compression}
	}
	lines := make([]string, 2048)
	for i := range lines {
		lines[i] = fmt.Sprintf("第%04d行: [asset-%04x](asset-%04x) café e\u0301\r\n", i, i, i)
	}
	large := strings.Join(lines, "")
	insert("old-large", "旧当前\n"+large+"current end", "旧历史\r\n"+large+"history end", "pglz")
	insert("old-short", "短正文 é e\u0301\r\n", "短历史\n", "")
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	before := readContentCompressionRows(t, pool, want)

	// Keep this historical roundtrip scoped to 54 when later migrations exist.
	m, err := postgres.NewMigrator(pool, through54Files)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	up, err := m.Up(ctx)
	if err != nil {
		t.Fatalf("Up with old content: %v", err)
	}
	// The missing migration must fail on its observable policy, before the
	// migration-count assertion or Down could fail for an unrelated reason.
	assertContentCompressionAttributes(t, pool, "Up from 53", "l")
	if len(up) != 1 || up[0].Version != 54 {
		t.Fatalf("Up from 53 = %+v, want migration 54 alone", up)
	}
	assertContentCompressionUnchanged(t, "Up", before, readContentCompressionRows(t, pool, want))

	newTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tx = newTx
	// Send fresh Go strings as parameters, never INSERT SELECT an already
	// compressed datum. Small content must remain valid without compression.
	insert("new-large", "新当前\n"+large+"new current end", "新历史\r\n"+large+"new history end", "lz4")
	insert("new-short", "新短正文\r\n", "新短历史 é\n", "")
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	beforeDown := readContentCompressionRows(t, pool, want)
	if back, err := m.Down(ctx); err != nil || back == nil || back.Version != 54 {
		t.Fatalf("Down = %+v, %v; want migration 54 alone", back, err)
	}
	assertContentCompressionAttributes(t, pool, "Down", "")
	assertContentCompressionUnchanged(t, "Down", beforeDown, readContentCompressionRows(t, pool, want))
	up, err = m.Up(ctx)
	if err != nil {
		t.Fatalf("Up after Down: %v", err)
	}
	assertContentCompressionAttributes(t, pool, "second Up", "l")
	if len(up) != 1 || up[0].Version != 54 {
		t.Fatalf("second Up = %+v, want migration 54 alone", up)
	}
	assertContentCompressionUnchanged(t, "second Up", beforeDown, readContentCompressionRows(t, pool, want))
}

func assertContentCompressionAttributes(t *testing.T, pool *pgxpool.Pool, phase, compression string) {
	t.Helper()
	// Include any other explicitly compressed user-table column, so a broad
	// policy change cannot satisfy the two-column contract unnoticed.
	rows, err := pool.Query(context.Background(), `SELECT c.relname || '.' || a.attname
		|| ' compression=' || quote_literal(a.attcompression::text)
		|| ' storage=' || quote_literal(a.attstorage::text)
		FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid
		WHERE c.relnamespace = 'public'::regnamespace AND c.relkind IN ('r', 'p')
		AND a.attnum > 0 AND NOT a.attisdropped
		AND (a.attcompression <> ''::"char" OR
			(c.relname IN ('page_contents', 'page_revisions') AND a.attname = 'content'))
		ORDER BY c.relname, a.attname`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"page_contents.content compression='" + compression + "' storage='x'",
		"page_revisions.content compression='" + compression + "' storage='x'",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%s column attributes = %q; want %q", phase, got, want)
	}
}

type contentCompressionValue struct {
	content, compression string
}

type contentCompressionStorage struct {
	compression, fileNode, ctid, xmin string
	bytes                             int
}

type contentCompressionRow struct {
	logical string
	storage contentCompressionStorage
}

func readContentCompressionRows(t *testing.T, pool *pgxpool.Pool, want map[string]contentCompressionValue) map[string]contentCompressionRow {
	t.Helper()
	got := map[string]contentCompressionRow{}
	for _, table := range []string{"page_contents", "page_revisions"} {
		rows, err := pool.Query(context.Background(), `SELECT node_id::text, to_jsonb(p)::text,
			content, content_hash, byte_size, coalesce(pg_column_compression(content), ''),
			pg_column_size(content), pg_relation_filenode(tableoid)::text, ctid::text, xmin::text
			FROM `+table+` p ORDER BY node_id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var node, content string
			var hash []byte
			var byteSize int
			var row contentCompressionRow
			if err := rows.Scan(&node, &row.logical, &content, &hash, &byteSize, &row.storage.compression,
				&row.storage.bytes, &row.storage.fileNode, &row.storage.ctid, &row.storage.xmin); err != nil {
				t.Fatal(err)
			}
			key := table + "/" + node
			if _, duplicate := got[key]; duplicate {
				t.Fatalf("unexpected additional content row for %s", key)
			}
			expected, ok := want[key]
			if !ok {
				t.Fatalf("unexpected content row %s", key)
			}
			sum := sha256.Sum256([]byte(expected.content))
			if content != expected.content || !bytes.Equal(hash, sum[:]) || byteSize != len(expected.content) {
				t.Fatalf("%s content/hash/byte_size differ from the full original UTF-8 input", key)
			}
			if row.storage.compression != expected.compression {
				t.Fatalf("%s compression = %q; want %q", key, row.storage.compression, expected.compression)
			}
			if expected.compression != "" && (row.storage.bytes <= 0 || row.storage.bytes >= byteSize) {
				t.Fatalf("%s stored bytes = %d, logical bytes = %d; want actual compression", key, row.storage.bytes, byteSize)
			}
			got[key] = row
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}
	if len(got) != len(want) || len(want) == 0 {
		t.Fatalf("content rows = %d, want %d nonempty fixture rows", len(got), len(want))
	}
	return got
}

func assertContentCompressionUnchanged(t *testing.T, phase string, before, after map[string]contentCompressionRow) {
	t.Helper()
	if len(after) != len(before) {
		t.Fatalf("%s row count = %d, want %d", phase, len(after), len(before))
	}
	for key, old := range before {
		current, ok := after[key]
		if !ok || current.logical != old.logical {
			t.Errorf("%s changed the complete logical row %s (present %t)", phase, key, ok)
		}
		if current.storage != old.storage {
			t.Errorf("%s rewrote %s: storage = %+v, want %+v", phase, key, current.storage, old.storage)
		}
	}
}
