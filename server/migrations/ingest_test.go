package migrations_test

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// Schema 53 must preserve nonempty old history, start without marks, and
// discard only its derived marks on Down. Catalog checks precede new SQL.
func TestIngestSchemaAndHistoryDefaults(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewEmptyDatabase(t))
	oldFiles, newFiles := fstest.MapFS{}, fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS(), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() > "00053_zzzz.sql" {
			continue
		}
		data, err := fs.ReadFile(migrations.FS(), entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		newFiles[entry.Name()] = &fstest.MapFile{Data: data, Mode: 0o444}
		if entry.Name() <= "00052_zzzz.sql" {
			oldFiles[entry.Name()] = newFiles[entry.Name()]
		}
	}
	old, err := postgres.NewMigrator(pool, oldFiles)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = old.Close() })
	up, err := old.Up(ctx)
	if err != nil || len(up) != 52 {
		t.Fatalf("old schema Up = %d, %v; want 52", len(up), err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	seedSourceHashMigration(t, pool)
	before := ingestMigrationData(t, pool)
	m, err := postgres.NewMigrator(pool, newFiles)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	up, err = m.Up(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var table, column bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.changeset_sources') IS NOT NULL, EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='changesets' AND column_name='ingest_count')").Scan(&table, &column); err != nil {
		t.Fatal(err)
	}
	if !table || !column {
		t.Fatalf("ingest schema present: marks table=%v, count column=%v; want both", table, column)
	}
	if len(up) != 1 || up[0].Version != 53 {
		t.Fatalf("Up after 52 = %+v, want 53 alone", up)
	}
	check := func() {
		t.Helper()
		var sets, counted, marks int
		if err := pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM changesets), (SELECT count(*) FROM changesets WHERE ingest_count <> 0), (SELECT count(*) FROM changeset_sources)").Scan(&sets, &counted, &marks); err != nil {
			t.Fatal(err)
		}
		if sets == 0 || counted != 0 || marks != 0 {
			t.Fatalf("old history=%d, nonzero counts=%d, marks=%d; want nonempty/0/0", sets, counted, marks)
		}
		if after := ingestMigrationData(t, pool); !reflect.DeepEqual(before, after) {
			t.Fatalf("ingest migration changed old data: got %v, want %v", after, before)
		}
	}
	check()
	if _, err := pool.Exec(ctx, "INSERT INTO changeset_sources SELECT id, 1, gen_random_uuid() FROM changesets LIMIT 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE changesets SET ingest_count=1 WHERE id IN (SELECT changeset_id FROM changeset_sources)"); err != nil {
		t.Fatal(err)
	}
	down, err := m.Down(ctx)
	if err != nil || down == nil || down.Version != 53 {
		t.Fatalf("Down = %+v, %v; want 53 alone", down, err)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatal(err)
	}
	check()
}

func TestIngestSchemaConstraints(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	seedSourceHashMigration(t, pool)
	var cs string
	if err := pool.QueryRow(ctx, "SELECT id::text FROM changesets ORDER BY id LIMIT 1").Scan(&cs); err != nil {
		t.Fatal(err)
	}
	var columns string
	if err := pool.QueryRow(ctx, "SELECT string_agg(attname || ':' || format_type(atttypid,atttypmod) || ':' || attnotnull::text, ',' ORDER BY attnum) FROM pg_attribute WHERE attrelid='changeset_sources'::regclass AND attnum>0 AND NOT attisdropped").Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != "changeset_id:uuid:true,seq:integer:true,node_id:uuid:true" {
		t.Errorf("source columns=%s", columns)
	}
	var forbidden int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_constraint WHERE conrelid='changeset_sources'::regclass AND contype IN ('f','c')").Scan(&forbidden); err != nil {
		t.Fatal(err)
	}
	if forbidden != 0 {
		t.Errorf("source foreign keys/checks=%d, want none", forbidden)
	}
	for name, want := range map[string]string{
		"changesets_notebook_id_created_at_ingest_idx":               "CREATE INDEX changesets_notebook_id_created_at_ingest_idx ON public.changesets USING btree (notebook_id, created_at DESC, id DESC) WHERE (ingest_count > 0)",
		"changesets_notebook_id_created_by_id_created_at_ingest_idx": "CREATE INDEX changesets_notebook_id_created_by_id_created_at_ingest_idx ON public.changesets USING btree (notebook_id, created_by_id, created_at DESC, id DESC) WHERE ((deleted_at IS NULL) AND (ingest_count > 0))",
	} {
		var got string
		if err := pool.QueryRow(ctx, "SELECT pg_get_indexdef(to_regclass($1))", name).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("index %s = %s; want %s", name, got, want)
		}
	}
	for _, tt := range []struct{ name, stmt, code, constraint string }{
		{"negative", "UPDATE changesets SET ingest_count=-1 WHERE id=$1", "23514", "changesets_ingest_count_check"},
		{"seq", "INSERT INTO changeset_sources VALUES ($1,1,'00000000-0000-7000-8000-000000000002')", "23505", "changeset_sources_pkey"},
		{"source", "INSERT INTO changeset_sources VALUES ($1,2,'00000000-0000-7000-8000-000000000001')", "23505", "changeset_sources_changeset_id_node_id_key"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, "DELETE FROM changeset_sources"); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, "INSERT INTO changeset_sources VALUES ($1,1,'00000000-0000-7000-8000-000000000001')", cs); err != nil {
				t.Fatal(err)
			}
			_, err := pool.Exec(ctx, tt.stmt, cs)
			var pg *pgconn.PgError
			if !errors.As(err, &pg) || pg.Code != tt.code || pg.ConstraintName != tt.constraint {
				t.Errorf("%s=%v; want %s/%s", tt.name, err, tt.code, tt.constraint)
			}
		})
	}
	if _, err := pool.Exec(ctx, "UPDATE changesets SET ingest_count=10001 WHERE id=$1", cs); err != nil {
		t.Fatalf("database must not duplicate the app cap: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO changeset_sources VALUES (gen_random_uuid(),0,gen_random_uuid())"); err != nil {
		t.Fatalf("no seq-positive check or FK is intended: %v", err)
	}
}

func ingestMigrationData(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range []string{"notebooks", "nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "asset_blobs"} {
		value := "to_jsonb(x)"
		if table == "changesets" {
			value += " - 'ingest_count'"
		}
		var data string
		query := "SELECT coalesce(string_agg(v, E'\\n' ORDER BY v), '') FROM (SELECT (" + value + ")::text AS v FROM " + table + " x) s"
		if err := pool.QueryRow(context.Background(), query).Scan(&data); err != nil {
			t.Fatal(err)
		}
		out[table] = data
	}
	return out
}
