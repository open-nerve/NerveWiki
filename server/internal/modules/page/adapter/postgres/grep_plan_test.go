package postgresadapter_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// grep's statements read through their indexes (M9/P4 design 6.1, review
// C-M6: on empty tables a plan is anything): ops, a large notebook, holds
// 50,000 nodes and 100,000 nodes queued in bulk; eng, the reader's, 1,000
// nodes and 3 queued exact. Without statistics, analyzed, and planned for
// any arguments: GrepCandidates's two branches of page_search_hits are
// the function's (whose own settings leave its statements bitmaps of the
// GIN indexes, checkHits), its queue's branch reads
// page_search_queue_notebook_id_bulk_idx by the notebook; GrepContents,
// of a batch of MaxSearchWalk pages, grep's most (M9 Codex review fix
// round 1, B-M1), reads page_contents once, by its key, for the sizes and
// the contents within its bound; ListNodes reads nodes by an index that
// leads with the notebook. None scans a table whole.
func TestGrepPlans(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	for _, table := range []string{"nodes", "page_contents", "page_search", "page_search_queue"} {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)")
	}
	seed := func(notebook uuid.UUID, nodes int) {
		f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
				created_at, updated_at)
			SELECT uuidv7(), $1, NULL, 'page', 'n' || k, 'n' || k, k, $2, $2, $3, $3 FROM generate_series(1, $4::integer) k`,
			notebook, f.alice, now(), nodes)
		f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
			SELECT id, 'a line', 1, sha256('a line'), 6, $2, $3 FROM nodes WHERE notebook_id = $1`, notebook, f.alice, now())
		f.exec(t, `INSERT INTO page_search_tags (notebook_id, prefix) VALUES ($1, page_search_prefix(nextval('page_search_tags_seq')))`,
			notebook)
	}
	seed(f.ops, 50_000)
	seed(f.eng, 1_000)
	f.exec(t, `INSERT INTO page_search_queue (node_id, notebook_id, bulk) SELECT uuidv7(), $1, true FROM generate_series(1, 100000)`,
		f.ops)
	f.exec(t, `INSERT INTO page_search_queue (node_id, notebook_id, bulk) SELECT id, notebook_id, false FROM nodes
		WHERE notebook_id = $1 ORDER BY id LIMIT 3`, f.eng)
	var ids []uuid.UUID
	rows, err := f.pool.Query(ctx, `SELECT id FROM nodes WHERE notebook_id = $1 ORDER BY id LIMIT $2`, f.eng, domain.MaxSearchWalk)
	if err != nil {
		t.Fatal(err)
	}
	if ids, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID]); err != nil {
		t.Fatal(err)
	}
	statements := []struct {
		name string
		s    statement
		want func(root *planNode) []string
	}{
		{"GrepCandidates", statementOf(t, func(g *gen.Queries) error {
			_, err := g.GrepCandidates(ctx, gen.GrepCandidatesParams{NotebookID: f.eng, Keys: []string{"abc", "bcd"}})
			return err
		}), func(root *planNode) []string {
			var out []string
			if n := len(root.all(func(n *planNode) bool { return n.Function == "page_search_hits" })); n != 2 {
				out = append(out, "reads page_search_hits other than twice")
			}
			queue := root.all(func(n *planNode) bool { return n.Relation == "page_search_queue" })
			if len(queue) != 1 || !reads("page_search_queue", "page_search_queue_notebook_id_bulk_idx", "")(queue[0]) ||
				!strings.Contains(queue[0].scanConditions(), "notebook_id") {
				out = append(out, "does not read the queue by page_search_queue_notebook_id_bulk_idx and the notebook")
			}
			return out
		}},
		{"GrepContents", statementOf(t, func(g *gen.Queries) error {
			_, err := g.GrepContents(ctx, gen.GrepContentsParams{NodeIds: ids, MaxBytes: domain.GrepBatchBytes})
			return err
		}), func(root *planNode) []string {
			// Its sizes and the contents within the bound, in one read by
			// the key (M9 Codex review D2).
			all := root.all(func(n *planNode) bool { return n.Relation == "page_contents" })
			if len(all) != 1 || len(root.all(reads("page_contents", "page_contents_pkey", ""))) != 1 {
				return []string{"does not read page_contents once, by its key"}
			}
			return nil
		}},
		{"ListNodes", statementOf(t, func(g *gen.Queries) error {
			_, err := g.ListNodes(ctx, f.eng)
			return err
		}), func(root *planNode) []string {
			byNotebook := func(n *planNode) bool {
				return n.Relation == "nodes" && strings.Contains(n.scanConditions(), "notebook_id") &&
					slices.ContainsFunc(n.indexes(), func(i string) bool { return strings.HasPrefix(i, "nodes_notebook_id_") })
			}
			if len(root.all(byNotebook)) != 1 {
				return []string{"does not read nodes by an index of the notebook"}
			}
			return nil
		}},
	}

	for _, stage := range []string{"without statistics", "analyzed"} {
		if stage == "analyzed" {
			f.exec(t, "ANALYZE nodes, page_contents, page_search, page_search_queue")
		} else if n := f.count(t, `SELECT count(*) FROM pg_stats WHERE tablename IN ('nodes', 'page_contents', 'page_search_queue')`); n != 0 {
			t.Fatalf("the tables have statistics of %d columns", n)
		}
		for _, s := range statements {
			for _, generic := range []bool{false, true} {
				root, raw := f.planTreeWith(t, s.s)
				if generic {
					root, raw = f.genericPlanTree(t, s.s.sql)
				}
				problems := s.want(&root)
				if len(root.all(func(n *planNode) bool { return n.Type == "Seq Scan" })) > 0 {
					problems = append(problems, "scans a table whole")
				}
				if len(problems) > 0 {
					t.Errorf("%s, %s, generic %v: %s:\n%s", s.name, stage, generic, strings.Join(problems, "; "), raw)
				}
			}
		}
	}
}
