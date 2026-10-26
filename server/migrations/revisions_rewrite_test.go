package migrations_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// 00049 keeps old and unmarked versions ordinary. Down drops only the
// marker; another Up must conservatively forget a previous true marker,
// while preserving the versions, their contents and their bases.
func TestSourceRevisionRewriteMigration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewDatabase(t))
	m, err := postgres.NewMigrator(pool, migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up(): %v", err)
	}
	column := func(want int) {
		t.Helper()
		var count int
		var typ, nullable, defaultValue string
		if err := pool.QueryRow(ctx, `SELECT count(*), coalesce(max(data_type), ''), coalesce(max(is_nullable), ''),
			coalesce(max(column_default), '') FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'page_revisions' AND column_name = 'rewrite'`).
			Scan(&count, &typ, &nullable, &defaultValue); err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Fatalf("page_revisions.rewrite column count = %d, want %d", count, want)
		}
		if want == 1 && (typ != "boolean" || nullable != "NO" || defaultValue != "false") {
			t.Fatalf("rewrite column = type %q, nullable %q, default %q; want boolean, NO, false", typ, nullable, defaultValue)
		}
	}
	// This assertion precedes any use of rewrite or Down(49), so a missing
	// migration fails as a contract assertion, not an undefined-column error.
	column(1)
	downTo(t, m, 49)
	column(0)

	user, workspace, notebook, node := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	at := time.Date(2026, 10, 26, 1, 2, 3, 0, time.UTC)
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, stmt, args...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	exec(`INSERT INTO users (id, email, password, display_name, created_at, updated_at)
		VALUES ($1, 'rewrite@corp.com', 'x', 'rewrite', $2, $2)`, user, at)
	exec(`INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at)
		VALUES ($1, 'rewrite', 'Rewrite', $2, $2, $3, $3)`, workspace, user, at)
	exec(`INSERT INTO notebooks (id, workspace_id, name, workspace_access, created_by_id, updated_by_id, created_at, updated_at)
		VALUES ($1, $2, 'Versions', 'editor', $3, $3, $4, $4)`, notebook, workspace, user, at)
	exec(`INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
		VALUES ($1, $2, 'page', 'Page', 'page', 0, $3, $3, $4, $4)`, node, notebook, user, at)
	exec(`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		VALUES ($1, '旧正文' || chr(10), 1, sha256(convert_to('旧正文' || chr(10), 'UTF8')), 10, $2, $3)`, node, user, at)
	changeset := func() uuid.UUID {
		t.Helper()
		id := uuid.NewV7()
		exec(`INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
			VALUES ($1, $2, 'edit', 'web', $3, $4, $4)`, id, notebook, user, at)
		return id
	}
	insert := func(id, cs uuid.UUID, base *int, revision int, content string, mark bool, rewrite any) error {
		t.Helper()
		sum := sha256.Sum256([]byte(content))
		stmt := `INSERT INTO page_revisions (id, changeset_id, node_id, base_revision, revision, content, content_hash,
			byte_size, created_at, updated_at`
		values := ` VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9`
		args := []any{id, cs, node, base, revision, content, sum[:], len(content), at}
		if mark {
			stmt += ", rewrite"
			values += ", $10"
			args = append(args, rewrite)
		}
		_, err := pool.Exec(ctx, stmt+")"+values+")", args...)
		return err
	}
	oldID, oldCS := uuid.NewV7(), changeset()
	if err := insert(oldID, oldCS, nil, 1, "旧正文\n", false, nil); err != nil {
		t.Fatalf("old version at schema 48: %v", err)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up with an old version: %v", err)
	}
	column(1)
	marker := func(id uuid.UUID, want bool) {
		t.Helper()
		var got bool
		if err := pool.QueryRow(ctx, "SELECT rewrite FROM page_revisions WHERE id = $1", id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("version %s rewrite = %t, want %t", id, got, want)
		}
	}
	marker(oldID, false)
	defaultID, defaultCS := uuid.NewV7(), changeset()
	if err := insert(defaultID, defaultCS, new(1), 2, "默认正文\n", false, nil); err != nil {
		t.Fatalf("new version omitting rewrite: %v", err)
	}
	marker(defaultID, false)
	trueID, trueCS := uuid.NewV7(), changeset()
	if err := insert(trueID, trueCS, new(2), 3, "系统正文\n", true, true); err != nil {
		t.Fatalf("new version explicitly rewritten: %v", err)
	}
	marker(trueID, true)
	var pgErr *pgconn.PgError
	if err := insert(uuid.NewV7(), changeset(), new(3), 4, "null", true, nil); !errors.As(err, &pgErr) ||
		pgErr.Code != "23502" || pgErr.TableName != "page_revisions" || pgErr.ColumnName != "rewrite" {
		t.Errorf("NULL rewrite = %v, want page_revisions.rewrite's not-null violation", err)
	}

	type version struct {
		ID        uuid.UUID
		Changeset uuid.UUID
		Node      uuid.UUID
		Base      *int
		Revision  int
		Content   string
		Hash      []byte
		Bytes     int
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	read := func() []version {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT id, changeset_id, node_id, base_revision, revision, content, content_hash,
			byte_size, created_at, updated_at FROM page_revisions ORDER BY revision`)
		if err != nil {
			t.Fatal(err)
		}
		got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[version])
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	before := read()
	if len(before) != 3 {
		t.Fatalf("versions before Down = %+v, want the old, default and true versions", before)
	}
	for i, want := range []struct {
		id, cs  uuid.UUID
		content string
		bytes   int
	}{
		{oldID, oldCS, "旧正文\n", 10},
		{defaultID, defaultCS, "默认正文\n", 13},
		{trueID, trueCS, "系统正文\n", 13},
	} {
		g := before[i]
		sum := sha256.Sum256([]byte(want.content))
		baseOK := (i == 0 && g.Base == nil) || (i > 0 && g.Base != nil && *g.Base == i)
		if g.ID != want.id || g.Changeset != want.cs || g.Node != node || !baseOK || g.Revision != i+1 ||
			g.Content != want.content || !bytes.Equal(g.Hash, sum[:]) || g.Bytes != want.bytes ||
			!g.CreatedAt.Equal(at) || !g.UpdatedAt.Equal(at) {
			t.Errorf("version %d before Down = %+v, want its original ids, base and content", i+1, g)
		}
	}
	downTo(t, m, 49)
	column(0)
	if got := read(); !reflect.DeepEqual(got, before) {
		t.Errorf("Down changed version data: got %+v, want %+v", got, before)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("Up after losing true markers: %v", err)
	}
	column(1)
	if got := read(); !reflect.DeepEqual(got, before) {
		t.Errorf("another Up changed version data: got %+v, want %+v", got, before)
	}
	for _, id := range []uuid.UUID{oldID, defaultID, trueID} {
		marker(id, false)
	}
}
