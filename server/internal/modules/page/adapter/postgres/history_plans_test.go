package postgresadapter_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
)

// The log's reads, the revert's look-ups and the purge's statements read
// through their indexes (M8/P3 design 3.11; v0.1 design 13.4, item 9),
// whole tables' scans off and the arguments bound as the store binds them:
// the log and one actor's a range of their own index each, already in its
// order; one page's its items' and versions' by node; a changeset's
// changes and counts its two keys, its moves the changesets' key (M9/P3
// design 3.20.1); a trash root's deletion by the
// revert of its restore the items' index on node_id, by the items' own
// times (M8/P4 design 3.7), the changesets' key and the items' keys, the
// deletions between the items' index on node_id; what reverted a
// changeset the partial index on reverts_id; the purge's the partial
// indexes on deleted_at and the changesets' key, its deletion of the
// emptied ones the partial index on trimmed; and the notebooks' foreign
// key's check, which reads changesets by notebook_id alone, the log's.
func TestTheLogReadsUseTheirIndexes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	id, actor := f.logOfSomeSize(t)
	ids := []uuid.UUID{id, uuid.NewV7()}
	var roots []uuid.UUID
	if err := f.pool.QueryRow(ctx, "SELECT array_agg(id) FROM nodes WHERE trash_root").Scan(&roots); err != nil || len(roots) == 0 {
		t.Fatalf("the log's trash roots = %v, %v", roots, err)
	}
	for _, tt := range []struct {
		name    string
		s       statement
		indexes []string
		sorted  bool // the statement's order is its index's: no sort
	}{
		{"ListChangesets", statementOf(t, func(q *gen.Queries) error {
			_, err := q.ListChangesets(ctx, gen.ListChangesetsParams{NotebookID: id, BelowAt: now(), BelowID: uuid.Max(), Client: new("web"),
				MaxRows: 101})
			return err
		}), []string{"changesets_notebook_id_created_at_id_idx", "changesets_pkey"}, true},
		{"ListChangesetsBy", statementOf(t, func(q *gen.Queries) error {
			_, err := q.ListChangesetsBy(ctx, gen.ListChangesetsByParams{NotebookID: id, Actor: actor, BelowAt: now(), BelowID: uuid.Max(),
				AllMcp: true, MaxRows: 101})
			return err
		}), []string{"changesets_notebook_id_created_by_id_created_at_id_idx", "changesets_pkey"}, true},
		{"ListChangesetsOfPage", statementOf(t, func(q *gen.Queries) error {
			_, err := q.ListChangesetsOfPage(ctx, gen.ListChangesetsOfPageParams{NotebookID: id, PageID: actor, BelowAt: now(),
				BelowID: uuid.Max(), MaxRows: 101})
			return err
		}), []string{"changeset_items_node_id_idx", "page_revisions_node_id_revision_key", "changesets_pkey"}, false},
		{"FindChangeset", statementOf(t, func(q *gen.Queries) error {
			_, err := q.FindChangeset(ctx, id)
			return err
		}), []string{"changesets_pkey"}, false},
		{"ChangesetFacts", statementOf(t, func(q *gen.Queries) error {
			_, err := q.ChangesetFacts(ctx, ids)
			return err
		}), []string{"changeset_items_changeset_id_node_id_key", "page_revisions_changeset_id_node_id_key", "changesets_reverts_id_idx"}, false},
		{"ChangesetMoves", statementOf(t, func(q *gen.Queries) error {
			_, err := q.ChangesetMoves(ctx, id)
			return err
		}), []string{"changesets_pkey"}, false},
		{"ListChanges", statementOf(t, func(q *gen.Queries) error {
			_, err := q.ListChanges(ctx, gen.ListChangesParams{ID: id, After: uuid.UUID{}, MaxRows: 100})
			return err
		}), []string{"changeset_items_changeset_id_node_id_key", "page_revisions_changeset_id_node_id_key", "nodes_pkey"}, false},
		{"FirstChanges", statementOf(t, func(q *gen.Queries) error {
			_, err := q.FirstChanges(ctx, gen.FirstChangesParams{Ids: ids, N: 3})
			return err
		}), []string{"changeset_items_changeset_id_node_id_key", "page_revisions_changeset_id_node_id_key", "nodes_pkey"}, false},
		{"CountItems", statementOf(t, func(q *gen.Queries) error {
			_, err := q.CountItems(ctx, gen.CountItemsParams{ID: id, Lim: 10_001})
			return err
		}), []string{"changeset_items_changeset_id_node_id_key"}, false},
		{"ChangesetItems", statementOf(t, func(q *gen.Queries) error {
			_, err := q.ChangesetItems(ctx, id)
			return err
		}), []string{"changeset_items_changeset_id_node_id_key"}, false},
		{"ChangesetVersions", statementOf(t, func(q *gen.Queries) error {
			_, err := q.ChangesetVersions(ctx, id)
			return err
		}), []string{"page_revisions_changeset_id_node_id_key", "page_revisions_node_id_revision_key"}, false},
		{"NodesNow", statementOf(t, func(q *gen.Queries) error {
			_, err := q.NodesNow(ctx, gen.NodesNowParams{Ids: roots, Reverted: id})
			return err
		}), []string{"changeset_items_node_id_idx", "changesets_pkey", "changeset_items_changeset_id_node_id_key"}, false},
		{"LockChangeset", statementOf(t, func(q *gen.Queries) error {
			_, err := q.LockChangeset(ctx, id)
			return err
		}), []string{"changesets_pkey", "changesets_reverts_id_idx"}, false},
		{"PurgeChangesetItems", statementOf(t, func(q *gen.Queries) error {
			_, err := q.PurgeChangesetItems(ctx, gen.PurgeChangesetItemsParams{Before: now(), Batch: 1000})
			return err
		}), []string{"changeset_items_deleted_at_idx", "changesets_pkey"}, false},
		{"PurgePageRevisions", statementOf(t, func(q *gen.Queries) error {
			_, err := q.PurgePageRevisions(ctx, gen.PurgePageRevisionsParams{Before: now(), Batch: 1000})
			return err
		}), []string{"page_revisions_deleted_at_idx", "changesets_pkey"}, false},
		{"PurgeTrimmedChangesets", statementOf(t, func(q *gen.Queries) error {
			_, err := q.PurgeTrimmedChangesets(ctx, 1000)
			return err
		}), []string{"changesets_trimmed_idx"}, false},
		{"the notebooks' foreign key's check", statement{sql: `SELECT 1 FROM changesets WHERE notebook_id = $1`, args: []any{id}},
			[]string{"changesets_notebook_id_created_at_id_idx"}, true},
	} {
		plan := f.planOf(t, tt.s)
		text := strings.Join(plan, "\n")
		for _, index := range tt.indexes {
			if !slices.ContainsFunc(plan, func(line string) bool { return slices.Contains(strings.Fields(line), index) }) {
				t.Errorf("%s does not read %s:\n%s", tt.name, index, text)
			}
		}
		if strings.Contains(text, "Seq Scan") {
			t.Errorf("%s scans a table whole:\n%s", tt.name, text)
		}
		if tt.sorted && strings.Contains(text, "Sort") {
			t.Errorf("%s sorts:\n%s", tt.name, text)
		}
	}
}

// logOfSomeSize gives eng a log the planner weighs as a notebook's, with
// its statistics: 2,000 changesets of three actors from four clients, 100
// of them reverts, 20 trimmed; ten items of each on 1,000 pages, and
// 5,000 versions of them, some deleted; 100 of the pages trash items'
// roots, each deleted at a changeset's time; and 500 changesets of ops,
// deleted with it. It answers one of eng's changesets and an actor.
func (f fixture) logOfSomeSize(t *testing.T) (uuid.UUID, uuid.UUID) {
	t.Helper()
	bob, carol := f.user(t, "bob@corp.com"), f.user(t, "carol@corp.com")
	pages := f.nodes(t, f.eng, "P", 1000)
	f.exec(t, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
		SELECT uuidv7(), $1, 'edit', (ARRAY['web', 'api', 'mcp:a', 'mcp:b'])[1 + k % 4], ($2::uuid[])[1 + k % 3],
			$3::timestamptz + k * interval '1 second', $3::timestamptz + k * interval '1 second'
		FROM generate_series(1, 1900) k`, f.eng, []uuid.UUID{f.alice, bob, carol}, now())
	f.exec(t, `INSERT INTO changesets (id, notebook_id, kind, reverts_id, client, created_by_id, created_at, updated_at)
		SELECT uuidv7(), $1, 'revert', c.id, 'web', c.created_by_id, c.created_at + interval '1 hour', c.created_at + interval '1 hour'
		FROM (SELECT id, created_by_id, created_at FROM changesets WHERE notebook_id = $1 ORDER BY id LIMIT 100) c`, f.eng)
	f.exec(t, `UPDATE changesets SET trimmed = true WHERE id IN (SELECT id FROM changesets ORDER BY id DESC LIMIT 20)`)
	f.exec(t, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at, deleted_at)
		SELECT uuidv7(), $1, 'edit', 'web', $2, $3, $3, $3 FROM generate_series(1, 500) k`, f.ops, f.alice, now())
	f.exec(t, `INSERT INTO changeset_items (id, changeset_id, node_id, after_name, after_sort_order, created_at, updated_at, deleted_at)
		SELECT uuidv7(), c.id, ($2::uuid[])[1 + (c.k * 10 + j) % 1000], 'N', 0, $3::timestamptz, $3::timestamptz,
			CASE WHEN j = 0 AND c.k % 4 = 0 THEN $3::timestamptz END
		FROM (SELECT id, row_number() OVER (ORDER BY id) AS k FROM changesets WHERE notebook_id = $1) c, generate_series(0, 9) j`,
		f.eng, pages, now())
	f.exec(t, `INSERT INTO page_revisions (id, changeset_id, node_id, revision, content, content_hash, byte_size, created_at, updated_at,
			deleted_at)
		SELECT uuidv7(), c.id, ($2::uuid[])[1 + c.k % 1000], 1 + c.k / 1000, '', sha256(''), 0, $3::timestamptz,
			$3::timestamptz, CASE WHEN c.k % 10 = 0 THEN $3::timestamptz END
		FROM (SELECT id, row_number() OVER (ORDER BY id) AS k FROM changesets WHERE notebook_id = $1) c`, f.eng, pages, now())
	f.exec(t, `UPDATE nodes SET deleted_at = $2::timestamptz + interval '1 second' * (n.k * 10), trash_root = true, trash_pages = 1,
			trash_assets = 0, trash_height = 1
		FROM (SELECT id, row_number() OVER (ORDER BY id) AS k FROM unnest($1::uuid[]) id) n WHERE nodes.id = n.id AND n.k <= 100`,
		pages, now())
	f.exec(t, "ANALYZE")
	var id uuid.UUID
	if err := f.pool.QueryRow(context.Background(), `SELECT id FROM changesets WHERE notebook_id = $1 AND kind = 'edit' ORDER BY id LIMIT 1`,
		f.eng).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id, bob
}

// statement is the SQL and the arguments a query's call sends, caught
// before they reach a database (linking's adapter's tests' own).
type statement struct {
	sql  string
	args []any
}

// errCaught ends a call whose statement is caught.
var errCaught = errors.New("caught")

func (s *statement) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	s.sql, s.args = sql, args
	return pgconn.CommandTag{}, errCaught
}

func (s *statement) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	s.sql, s.args = sql, args
	return nil, errCaught
}

func (s *statement) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	s.sql, s.args = sql, args
	return caughtRow{}
}

func (s *statement) SendBatch(_ context.Context, b *pgx.Batch) pgx.BatchResults {
	s.sql = b.QueuedQueries[0].SQL
	s.args = b.QueuedQueries[0].Arguments
	return nil
}

// caughtRow is the row of a caught statement.
type caughtRow struct{}

func (caughtRow) Scan(...any) error { return errCaught }

// statementOf is the statement call sends.
func statementOf(t *testing.T, call func(q *gen.Queries) error) statement {
	t.Helper()
	var s statement
	if err := call(gen.New(&s)); !errors.Is(err, errCaught) {
		t.Fatalf("the call: %v", err)
	}
	return s
}

// planOf is the plan of s, scans of whole tables off.
func (f fixture) planOf(t *testing.T, s statement) []string {
	t.Helper()
	ctx := context.Background()
	var plan []string
	err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `EXPLAIN (COSTS OFF) `+s.sql, s.args...)
		if err != nil {
			return err
		}
		plan, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}
