package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// planNode is a node of a plan, as EXPLAIN (FORMAT JSON) gives it.
type planNode struct {
	Type      string     `json:"Node Type"`
	Relation  string     `json:"Relation Name"`
	Index     string     `json:"Index Name"`
	IndexCond string     `json:"Index Cond"`
	Plans     []planNode `json:"Plans"`
}

// all are the nodes of the plan below n, n too, that match.
func (n *planNode) all(match func(*planNode) bool) []*planNode {
	var out []*planNode
	if match(n) {
		out = append(out, n)
	}
	for i := range n.Plans {
		out = append(out, n.Plans[i].all(match)...)
	}
	return out
}

// indexes are the indexes a scan reads, its own or its bitmaps', and
// whether each is read by a condition on column, not walked whole.
func (n *planNode) indexes(column string) ([]string, bool) {
	var out []string
	keyed := true
	for _, m := range n.all(func(m *planNode) bool { return m.Index != "" }) {
		out = append(out, m.Index)
		keyed = keyed && strings.Contains(m.IndexCond, column)
	}
	return out, keyed && len(out) > 0
}

// unlimited are the reads of table below n, n too, that no Limit node
// is above: a lateral's LIMIT 1 is one, which stops at its first row.
func (n *planNode) unlimited(table string) []*planNode {
	if n.Type == "Limit" {
		return nil
	}
	var out []*planNode
	if n.Relation == table {
		out = append(out, n)
	}
	for i := range n.Plans {
		out = append(out, n.Plans[i].unlimited(table)...)
	}
	return out
}

// readsOf matches the scans of table: an index's, a bitmap's heap scan or
// a scan of it whole, never a bitmap's index scan alone.
func readsOf(table string) func(*planNode) bool {
	return func(n *planNode) bool { return n.Relation == table }
}

// planTree is the plan of s, planned with its arguments, or, generic, for
// any values of its parameters as a plan cache's generic plan, the
// statement explained unbound; and its JSON.
func (f fixture) planTree(t *testing.T, s statement, generic bool) (planNode, string) {
	t.Helper()
	ctx := context.Background()
	var raw string
	err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
		if !generic {
			return tx.QueryRow(ctx, `EXPLAIN (FORMAT JSON, COSTS OFF) `+s.sql, s.args...).Scan(&raw)
		}
		results, err := tx.Conn().PgConn().Exec(ctx, `EXPLAIN (GENERIC_PLAN, FORMAT JSON, COSTS OFF) `+s.sql).ReadAll()
		if err != nil {
			return err
		}
		if len(results) != 1 || len(results[0].Rows) != 1 {
			return fmt.Errorf("%d results", len(results))
		}
		raw = string(results[0].Rows[0][0])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var plans []struct {
		Plan planNode `json:"Plan"`
	}
	if err := json.Unmarshal([]byte(raw), &plans); err != nil || len(plans) != 1 {
		t.Fatalf("the plan %s: %v", raw, err)
	}
	return plans[0].Plan, raw
}

// index's statements read through their indexes (M9/P4 design 3.9.3;
// v0.1 design 13.4, item 9): eng holds 1,000 pages of five properties,
// type and summary among them, three tags and links, and 200 pages of
// 1,000 properties without either (review B-I2); ops holds 50,000 pages of
// five properties. Without statistics, analyzed, and planned for any
// arguments: IndexPages reads eng's pages by indexed_pages_notebook_id_idx
// (an index's or a bitmap's scan: on ops, most of the rows, a scan of the
// table whole is the plan, review B-M4) and their types by
// page_properties_type_idx (00047), one probe a page; IndexItems reads its
// pages by their key, their types and summaries by the two partial
// indexes, their tags by page_tags' key; InboundCounts finds each source
// on page_links_resolved_id_source_id_idx alone. Without 00047 the
// types are read by page_properties' key, every property of a page
// without a type (mutant 184). Each read of page_properties is below its
// lateral's Limit, the first row of a page alone: without LIMIT 1 the
// planner keeps its probe a page, the plan otherwise the same, and reads
// each of a page's rows of the key (mutant 173). So is each read of
// page_tags below its subquery's LIMIT 21 (S2 review B-n1): without
// statistics the plan reads a page's tags by a bitmap and sorts them, at
// most MaxNames, before it keeps 21; analyzed it probes the primary key in
// its order and stops at the 21st.
func TestIndexPlans(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	for _, table := range []string{"indexed_pages", "page_properties", "page_tags", "page_links"} {
		if _, err := f.pool.Exec(ctx, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)"); err != nil {
			t.Fatal(err)
		}
	}
	// seed writes pages of notebook, each of props properties, the second
	// its type and the fourth its summary when typed; the pages of a seed
	// are of its own revision.
	seed := func(notebook uuid.UUID, revision, pages, props int, typed bool) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, `INSERT INTO indexed_pages (node_id, notebook_id, revision, extractor, frontmatter_valid)
			SELECT uuidv7(), $1, $2, 1, true FROM generate_series(1, $3::integer)`, notebook, revision, pages); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Exec(ctx, `INSERT INTO page_properties (source_id, position, notebook_id, key, value)
			SELECT p.node_id, k, $1, CASE WHEN $3 AND k = 1 THEN 'type' WHEN $3 AND k = 3 THEN 'Summary' ELSE 'p' || k END,
				to_jsonb('v' || k)
			FROM indexed_pages p CROSS JOIN generate_series(0, $4::integer - 1) k
			WHERE p.notebook_id = $1 AND p.revision = $2`, notebook, revision, typed, props); err != nil {
			t.Fatal(err)
		}
	}
	seed(f.eng, 1, 1_000, 5, true)
	seed(f.eng, 2, 200, 1_000, false)
	seed(f.ops, 1, 50_000, 5, true)
	for _, stmt := range []string{
		`INSERT INTO page_tags (source_id, tag_key, notebook_id, tag, count)
			SELECT p.node_id, 't' || k, p.notebook_id, 'T' || k, 1 FROM indexed_pages p CROSS JOIN generate_series(1, 3) k
			WHERE p.notebook_id = $1 AND p.revision = 1`,
		`INSERT INTO page_links (source_id, range_start, range_end, notebook_id, kind, target, resolved_id, ambiguous)
			SELECT s.node_id, 0, 1, $1, 'wikilink', 'x', t.node_id, false
			FROM (SELECT node_id, row_number() OVER (ORDER BY node_id) r FROM indexed_pages WHERE notebook_id = $1) s
			JOIN (SELECT node_id, row_number() OVER (ORDER BY node_id) r FROM indexed_pages WHERE notebook_id = $1) t
				ON t.r = s.r % 50 + 1`,
	} {
		if _, err := f.pool.Exec(ctx, stmt, f.eng); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	rows, err := f.pool.Query(ctx, `SELECT node_id FROM indexed_pages WHERE notebook_id = $1 ORDER BY node_id LIMIT 100`, f.eng)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		t.Fatal(err)
	}

	// only is a check that each scan of table reads index (or one of
	// index) by a condition on column: a probe, never the index whole.
	only := func(table, column string, index ...string) func(root *planNode) []string {
		return func(root *planNode) []string {
			scans := root.all(readsOf(table))
			if len(scans) == 0 {
				return []string{"does not read " + table}
			}
			var out []string
			for _, s := range scans {
				got, keyed := s.indexes(column)
				if !keyed || slices.ContainsFunc(got, func(i string) bool { return !slices.Contains(index, i) }) {
					out = append(out, fmt.Sprintf("reads %s by %v, not by %v alone on %s", table, got, index, column))
				}
			}
			return out
		}
	}
	both := func(checks ...func(*planNode) []string) func(*planNode) []string {
		return func(root *planNode) []string {
			var out []string
			for _, c := range checks {
				out = append(out, c(root)...)
			}
			return out
		}
	}
	properties := func(index ...string) func(root *planNode) []string {
		return func(root *planNode) []string {
			out := only("page_properties", "source_id", index...)(root)
			if n := len(root.all(readsOf("page_properties"))); n != len(index) {
				out = append(out, fmt.Sprintf("reads page_properties %d times, want %d", n, len(index)))
			}
			if n := len(root.unlimited("page_properties")); n > 0 {
				out = append(out, fmt.Sprintf("reads page_properties %d times without a Limit above it", n))
			}
			return out
		}
	}
	project := "project"
	statements := []struct {
		name string
		s    statement
		want func(root *planNode) []string
	}{
		{"IndexPages", statementOf(t, func(q *gen.Queries) error {
			_, err := q.IndexPages(ctx, gen.IndexPagesParams{NotebookID: f.eng})
			return err
		}), both(only("indexed_pages", "notebook_id", "indexed_pages_notebook_id_idx"), properties("page_properties_type_idx"))},
		{"IndexPages of a type", statementOf(t, func(q *gen.Queries) error {
			_, err := q.IndexPages(ctx, gen.IndexPagesParams{NotebookID: f.eng, TypeIs: &project})
			return err
		}), both(only("indexed_pages", "notebook_id", "indexed_pages_notebook_id_idx"), properties("page_properties_type_idx"))},
		{"IndexItems", statementOf(t, func(q *gen.Queries) error {
			_, err := q.IndexItems(ctx, ids)
			return err
		}), both(only("indexed_pages", "node_id", "indexed_pages_pkey"),
			properties("page_properties_type_idx", "page_properties_summary_idx"), only("page_tags", "source_id", "page_tags_pkey"),
			func(root *planNode) []string {
				if n := len(root.unlimited("page_tags")); n > 0 {
					return []string{fmt.Sprintf("reads page_tags %d times without a Limit above it", n)}
				}
				return nil
			})},
		{"InboundCounts", statementOf(t, func(q *gen.Queries) error {
			_, err := q.InboundCounts(ctx, gen.InboundCountsParams{Ids: ids, MaxCount: domain.MaxInbound})
			return err
		}), func(root *planNode) []string {
			out := only("page_links", "resolved_id", "page_links_resolved_id_source_id_idx")(root)
			if n := len(root.all(func(n *planNode) bool { return n.Relation == "page_links" && n.Type == "Index Only Scan" })); n != 2 {
				out = append(out, fmt.Sprintf("%d index-only scans of page_links, want 2", n))
			}
			return out
		}},
	}
	for _, stage := range []string{"without statistics", "analyzed"} {
		if stage == "analyzed" {
			if _, err := f.pool.Exec(ctx, "ANALYZE indexed_pages, page_properties, page_tags, page_links"); err != nil {
				t.Fatal(err)
			}
		} else {
			var n int
			if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stats WHERE tablename IN ('indexed_pages', 'page_properties',
				'page_tags', 'page_links')`).Scan(&n); err != nil || n != 0 {
				t.Fatalf("the tables have statistics of %d columns, %v", n, err)
			}
		}
		for _, s := range statements {
			for _, generic := range []bool{false, true} {
				root, raw := f.planTree(t, s.s, generic)
				if problems := s.want(&root); len(problems) > 0 {
					t.Errorf("%s, %s, generic %v: %s:\n%s", s.name, stage, generic, strings.Join(problems, "; "), raw)
				}
			}
		}
	}
}
