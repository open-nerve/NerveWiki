//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The rollback's reads' guard's notebook: rollbackCostFolders folders at
// the root of rollbackCostPages pages each less one, 100,000 pages, its
// baseline; beside rollbackCostOthers notebooks of rollbackCostOtherPages
// pages each, each with another upgrade's baseline of them, the rows the
// reads should not read (decision 240).
const (
	rollbackCostFolders    = 100
	rollbackCostPages      = 1000
	rollbackCostOthers     = 9
	rollbackCostOtherPages = 50000
)

// rollbackLeftBound and rollbackReadsBound are the medians of three reads
// the test below allows in each state and plan: three times the larger of
// the medians of two local rounds, rounded up to 100 ms, RollbackLeft's
// within the request's 4 s too (decisions 226, 236). The rounds' largest
// medians were 413 and 429 ms of RollbackLeft (368 to 429 ms in the six
// states and plans), 1,449 and 1,514 ms of the synthesized pass's reads
// (885 to 1,514 ms: analyzed, and generic with the other upgrades'
// statistics alone, the slowest); the seed took 13.5 and 13.8 s, 2.4 s of
// it the notebook's. Read with joins and arrays of ids, the design's and a
// revert's statements, they took 127 to 462 ms and 534 to 1,242 ms, but
// read nodes, page_contents and upgrade_nodes whole in four of the six.
const (
	rollbackLeftBound  = 1300 * time.Millisecond
	rollbackReadsBound = 4600 * time.Millisecond
)

// The rollback's reads' cost guard (M11/P1 design 7.1, decisions 226,
// 236, 240): a notebook of 100,000 pages, the baseline of an upgrade of
// it, then every page below a folder moved under a new folder, renamed or
// reordered, a third each, and 10,000 written; beside nine notebooks of
// 50,000 pages, each with another upgrade's baseline. upgrade_nodes, nodes
// and page_contents never analyzed; then analyzed; then upgrade_nodes with
// the other upgrades' statistics alone. Plans first: in each, for their
// arguments and generic, RollbackLeft, and RollbackFreeForest,
// RollbackBaseline, RollbackHolders, RollbackNodesNow, RollbackContentsNow
// and ListRevertTree of the pass that synthesizes its plan, read as
// rollbackReadProblems says. Time
// second: in each state and plan, forced on a connection of its own as a
// cached statement gets it, RollbackLeft answers what is left, the median
// of three within rollbackLeftBound; the synthesized pass's reads answer
// the baseline, its 100 holders, no free node, the nodes and the contents,
// the median of three within rollbackReadsBound. -v prints the medians and
// the seed's time.
func TestARollbacksReadsTakeTheirTimeAtAnyStatistics(t *testing.T) {
	f := newFixture(t)
	for _, table := range rollbackCostTables() {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)")
	}
	upgrade := uuid.NewV7()
	started := time.Now()
	// The folders, f, and the agent's new ones beside them, h; then the
	// pages in their state now, each with its baseline row of its state
	// before: below its folder, its name, its order; the folders' rows.
	f.exec(t, `INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		SELECT gen_random_uuid(), $1, 'page', NULL, v.s || k, v.s || k, v.at + k, $2, $2, now(), now()
		FROM generate_series(0, $3::integer - 1) k, (VALUES ('f', 0), ('h', 1000)) v(s, at)`, f.eng, f.alice, rollbackCostFolders)
	f.exec(t, `WITH g AS MATERIALIZED (
			SELECT gen_random_uuid() AS id, d.id AS folder, h.id AS holder, d.name || '-' || i AS name, i,
				row_number() OVER () AS j
			FROM nodes d JOIN nodes h ON h.notebook_id = d.notebook_id AND h.parent_id IS NULL AND h.name = 'h' || substr(d.name, 2),
				generate_series(1, $4::integer - 1) i
			WHERE d.notebook_id = $1 AND d.parent_id IS NULL AND d.name LIKE 'f%'
		), made AS (
			INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
				updated_at)
			SELECT id, $1, 'page', CASE WHEN j % 3 = 0 THEN holder ELSE folder END, CASE WHEN j % 3 = 1 THEN 'r' || name ELSE name END,
				CASE WHEN j % 3 = 1 THEN 'r' || name ELSE name END, CASE WHEN j % 3 = 2 THEN i + 0.5 ELSE i END, $2, $2, now(), now()
			FROM g
		)
		INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT $3, id, $1, true, 'page', folder, name, i, 1, sha256('x'), 1, 0, sha256('x') FROM g`,
		f.eng, f.alice, upgrade, rollbackCostPages)
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT $1, id, notebook_id, true, 'page', NULL, name, sort_order, 1, sha256('x'), 1, 0, sha256('x') FROM nodes
		WHERE notebook_id = $2 AND parent_id IS NULL AND name LIKE 'f%'`, upgrade, f.eng)
	// Every page's content, the baseline's but 10,000 written.
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, CASE WHEN w THEN 'y' ELSE 'x' END, CASE WHEN w THEN 2 ELSE 1 END, sha256(CASE WHEN w THEN 'y' ELSE 'x' END::bytea), 1,
			created_by_id, now()
		FROM (SELECT id, created_by_id, parent_id IS NOT NULL AND row_number() OVER (ORDER BY parent_id IS NULL, id) <= 10000 AS w
			FROM nodes WHERE notebook_id = $1) n`, f.eng)
	notebook := time.Since(started)
	f.exec(t, `INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at)
		SELECT gen_random_uuid(), b.workspace_id, 'Other ' || g, b.created_by_id, b.created_by_id, now(), now()
		FROM notebooks b, generate_series(1, $2::integer) g WHERE b.id = $1`, f.eng, rollbackCostOthers)
	f.exec(t, `INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		SELECT gen_random_uuid(), o.id, 'page', NULL, 'o' || k, 'o' || k, k, $2, $2, now(), now()
		FROM notebooks o, generate_series(1, $1::integer) k WHERE o.name LIKE 'Other %'`, rollbackCostOtherPages, f.alice)
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, 'x', 1, sha256('x'), 1, created_by_id, now() FROM nodes WHERE notebook_id IN (SELECT id FROM notebooks WHERE name LIKE 'Other %')`)
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT u.id, n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision, c.content_hash, c.byte_size, 0,
			c.content_hash
		FROM (SELECT id AS notebook_id, gen_random_uuid() AS id FROM notebooks WHERE name LIKE 'Other %') u
		JOIN nodes n ON n.notebook_id = u.notebook_id JOIN page_contents c ON c.node_id = n.id`)
	seeded := time.Since(started)

	reads := rollbackCostReads{f: f, upgrade: upgrade}
	reads.load(t)
	if want := (domain.RollbackLeft{Nodes: rollbackCostFolders * (rollbackCostPages - 1), Pages: 10000, Created: rollbackCostFolders}); reads.left != want {
		t.Fatalf("the seed leaves %+v, want %+v", reads.left, want)
	}
	medians := map[string]any{}
	for _, statistics := range []string{"never analyzed", "analyzed", "the other upgrades' alone"} {
		switch statistics {
		case "never analyzed":
			for _, table := range rollbackCostTables() {
				f.exec(t, "SELECT pg_clear_relation_stats('public', '"+table+"')")
				f.exec(t, `SELECT pg_clear_attribute_stats('public', '`+table+`', attname, false)
					FROM pg_attribute WHERE attrelid = '`+table+`'::regclass AND attnum > 0 AND NOT attisdropped`)
			}
		case "analyzed":
			f.exec(t, "ANALYZE upgrade_nodes, nodes, page_contents")
		case "the other upgrades' alone":
			f.exec(t, "CREATE TABLE kept AS SELECT * FROM upgrade_nodes WHERE upgrade_id = $1", upgrade)
			f.exec(t, "DELETE FROM upgrade_nodes WHERE upgrade_id = $1", upgrade)
			f.exec(t, "ANALYZE upgrade_nodes")
			f.exec(t, "INSERT INTO upgrade_nodes SELECT * FROM kept")
		}
		for name, s := range reads.statements(t) {
			for _, generic := range []bool{false, true} {
				var plan planNode
				var raw string
				if generic {
					plan, raw = f.genericPlanTree(t, s.sql)
				} else {
					plan, raw = f.explain(t, `EXPLAIN (FORMAT JSON, VERBOSE, COSTS OFF) `+s.sql, s.args, nil)
				}
				if problems := rollbackReadProblems(&plan); len(problems) > 0 {
					t.Errorf("%s, %s, generic %v: %s\n%s\n%.3000s", statistics, name, generic, strings.Join(problems, "; "), planShape(&plan), raw)
				}
			}
		}
		for _, mode := range []string{"custom", "generic"} {
			left, synthesized := reads.timed(t, mode)
			medians[statistics+", "+mode] = map[string]int64{"left_ms": left.Milliseconds(), "reads_ms": synthesized.Milliseconds()}
			if left > rollbackLeftBound || synthesized > rollbackReadsBound {
				t.Errorf("%s, %s: the medians of three reads took %s for RollbackLeft and %s for the synthesized pass's, past %s or %s",
					statistics, mode, left, synthesized, rollbackLeftBound, rollbackReadsBound)
			}
		}
	}
	b, err := json.Marshal(map[string]any{"seed_ms": seeded.Milliseconds(), "notebook_ms": notebook.Milliseconds(), "medians": medians})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("rollback reads cost %s", b)
}

// rollbackCostTables are the tables whose statistics the guard sets, and
// autovacuum would not set behind it.
func rollbackCostTables() []string {
	return []string{"upgrade_nodes", "nodes", "page_contents"}
}

// rollbackCostReads are the rollback's reads of the guard's upgrade: what
// is left and the baseline's rows, as the seed made them.
type rollbackCostReads struct {
	f       fixture
	upgrade uuid.UUID
	left    domain.RollbackLeft
	rows    int // the baseline's
}

func (r *rollbackCostReads) load(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	var err error
	if r.left, err = r.f.s.RollbackLeft(ctx, r.upgrade, r.f.eng); err != nil {
		t.Fatal(err)
	}
	baseline, err := r.f.s.RollbackBaseline(ctx, r.upgrade)
	if err != nil || len(baseline) != rollbackCostFolders*rollbackCostPages {
		t.Fatalf("RollbackBaseline = %d rows, %v; want %d", len(baseline), err, rollbackCostFolders*rollbackCostPages)
	}
	r.rows = len(baseline)
}

// statements are the reads' statements with the guard's arguments.
func (r *rollbackCostReads) statements(t *testing.T) map[string]statement {
	t.Helper()
	ctx := context.Background()
	return map[string]statement{
		"RollbackLeft": statementOf(t, func(q *gen.Queries) error {
			_, err := q.RollbackLeft(ctx, gen.RollbackLeftParams{UpgradeID: r.upgrade, NotebookID: r.f.eng})
			return err
		}),
		"RollbackBaseline": statementOf(t, func(q *gen.Queries) error { _, err := q.RollbackBaseline(ctx, r.upgrade); return err }),
		"RollbackHolders":  statementOf(t, func(q *gen.Queries) error { _, err := q.RollbackHolders(ctx, r.upgrade); return err }),
		"RollbackFreeForest": statementOf(t, func(q *gen.Queries) error {
			_, err := q.RollbackFreeForest(ctx, gen.RollbackFreeForestParams{UpgradeID: r.upgrade, NotebookID: r.f.eng})
			return err
		}),
		"RollbackNodesNow":    statementOf(t, func(q *gen.Queries) error { _, err := q.RollbackNodesNow(ctx, r.upgrade); return err }),
		"RollbackContentsNow": statementOf(t, func(q *gen.Queries) error { _, err := q.RollbackContentsNow(ctx, r.upgrade); return err }),
		"ListRevertTree":      statementOf(t, func(q *gen.Queries) error { _, err := q.ListRevertTree(ctx, r.f.eng); return err }),
	}
}

// timed are the medians of three of RollbackLeft and of the synthesized
// pass's reads, on a connection of their own whose cached statements are
// planned as mode says, each answering as the seed has it.
func (r *rollbackCostReads) timed(t *testing.T, mode string) (left, reads time.Duration) {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: r.f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "SET plan_cache_mode = force_"+mode+"_plan"); err != nil {
		t.Fatal(err)
	}
	s := postgresadapter.New(pool)
	var lefts, synthesized []time.Duration
	for range 3 {
		began := time.Now()
		got, err := s.RollbackLeft(ctx, r.upgrade, r.f.eng)
		lefts = append(lefts, time.Since(began))
		if err != nil || got != r.left {
			t.Fatalf("%s: RollbackLeft = %+v, %v; want %+v", mode, got, err, r.left)
		}
		began = time.Now()
		err = rollbackSynthesizedReads(ctx, s, r)
		synthesized = append(synthesized, time.Since(began))
		if err != nil {
			t.Fatalf("%s: the synthesized pass's reads: %v", mode, err)
		}
	}
	slices.Sort(lefts)
	slices.Sort(synthesized)
	return lefts[1], synthesized[1]
}

// rollbackSynthesizedReads are the reads of the pass that synthesizes its
// plan, as PlanRollback makes them: the free nodes, then the baseline, the
// holders, the nodes and the contents now and the tree, each answering as
// the seed made them.
func rollbackSynthesizedReads(ctx context.Context, s *postgresadapter.Store, r *rollbackCostReads) error {
	forest, err := s.RollbackFreeForest(ctx, r.upgrade, r.f.eng)
	if err != nil {
		return err
	}
	baseline, err := s.RollbackBaseline(ctx, r.upgrade)
	if err != nil {
		return err
	}
	holders, err := s.RollbackHolders(ctx, r.upgrade)
	if err != nil {
		return err
	}
	now, contents, err := s.RollbackNow(ctx, r.upgrade)
	if err != nil {
		return err
	}
	tree, err := s.ListRevertTree(ctx, r.f.eng)
	if err != nil {
		return err
	}
	if len(forest) != 0 || len(baseline) != r.rows || len(holders) != rollbackCostFolders || len(now) != r.rows ||
		len(contents) != r.rows || len(tree) != r.rows+rollbackCostFolders {
		return fmt.Errorf("%d free nodes, %d baseline rows, %d holders, %d nodes and %d contents now, %d in the tree", len(forest),
			len(baseline), len(holders), len(now), len(contents), len(tree))
	}
	return nil
}

// rollbackJoined matches an index condition that compares the index's
// column with a column of a join's other side, alias.column: a read for
// one row of it.
var rollbackJoined = regexp.MustCompile(`= [a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*\)`)

// rollbackReadProblems are the problems of a plan of a rollback's read
// (M11/P1 design 7.1, decisions 236, 240): upgrade_nodes read other than by
// upgrade_nodes_pkey with upgrade_id in its condition; nodes other than by
// an index with notebook_id in its condition or by nodes_pkey on id;
// page_contents other than by page_contents_pkey on node_id; a read by a
// column of a join's other side not repeated on a Nested Loop's inner
// side, for each of its rows; any other scan, a Seq Scan, of them.
func rollbackReadProblems(plan *planNode) []string {
	var problems []string
	var visit func(n *planNode, repeated bool)
	visit = func(n *planNode, repeated bool) {
		if n.Type == "Materialize" || n.Type == "Hash" {
			repeated = false
		}
		switch n.Relation {
		case "upgrade_nodes", "nodes", "page_contents":
			problems = append(problems, rollbackScanProblems(n, repeated)...)
		}
		for i := range n.Plans {
			below := &n.Plans[i]
			visit(below, repeated || n.Type == "Nested Loop" && below.Parent == "Inner")
		}
	}
	visit(plan, false)
	return problems
}

// rollbackScanProblems are the problems of n, a scan of one of the
// rollback's tables, repeated or not.
func rollbackScanProblems(n *planNode, repeated bool) []string {
	if n.Type == "ModifyTable" {
		return nil
	}
	var scans []*planNode
	switch n.Type {
	case "Index Scan", "Index Only Scan":
		scans = []*planNode{n}
	case "Bitmap Heap Scan":
		scans = rollbackBitmaps(n)
	}
	if len(scans) == 0 {
		return []string{fmt.Sprintf("%s read by a %s", n.Relation, n.Type)}
	}
	var problems []string
	for _, ix := range scans {
		cond := ix.IndexCond
		var keyed bool
		switch n.Relation {
		case "upgrade_nodes":
			keyed = ix.Index == "upgrade_nodes_pkey" && strings.Contains(cond, "upgrade_id = ")
		case "nodes":
			keyed = strings.Contains(cond, "notebook_id = ") || ix.Index == "nodes_pkey" && strings.Contains(cond, "id = ")
		case "page_contents":
			keyed = ix.Index == "page_contents_pkey" && strings.Contains(cond, "node_id = ")
		}
		switch {
		case !keyed:
			problems = append(problems, fmt.Sprintf("%s read by %s on %q", n.Relation, ix.Index, cond))
		case !repeated && rollbackJoined.MatchString(cond):
			problems = append(problems, fmt.Sprintf("%s read by %s on %q once, not for each row on a Nested Loop's inner side",
				n.Relation, ix.Index, cond))
		}
	}
	return problems
}

// rollbackBitmaps are the Bitmap Index Scans a Bitmap Heap Scan reads by,
// through its BitmapAnds and BitmapOrs, not its subplans'.
func rollbackBitmaps(n *planNode) []*planNode {
	var out []*planNode
	for i := range n.Plans {
		below := &n.Plans[i]
		switch below.Type {
		case "Bitmap Index Scan":
			out = append(out, below)
		case "BitmapAnd", "BitmapOr":
			out = append(out, rollbackBitmaps(below)...)
		}
	}
	return out
}

// The rollback's reads' guard's check (M11/P1 design 7.1, decisions 236,
// 240) on plans written by hand, without a database, so that the short
// line (make test-go-short), which builds this file without race, runs
// it: a key read for each row on a Nested Loop's inner side, a read by
// the notebook's key, a read by an array of keys, and a Bitmap Heap
// Scan whose subplan reads another table are accepted; each other plan
// breaks one rule.
func TestTheRollbackGuardJudgesThePlans(t *testing.T) {
	scan := func(kind, relation, index, cond, parent string) string {
		return `{"Node Type":"` + kind + `","Relation Name":"` + relation + `","Index Name":"` + index + `","Index Cond":"` + cond +
			`","Parent Relationship":"` + parent + `"}`
	}
	baseline := scan("Index Scan", "upgrade_nodes", "upgrade_nodes_pkey", "(b.upgrade_id = $1)", "Outer")
	loop := func(inner string) string {
		return `{"Node Type":"Nested Loop","Plans":[` + baseline + `,{"Node Type":"Limit","Parent Relationship":"Inner","Plans":[` +
			inner + `]}]}`
	}
	for _, tt := range []struct {
		name, plan string
		ok         bool
	}{
		{"a node's key read for each baseline row", loop(scan("Index Scan", "nodes", "nodes_pkey", "(n.id = b.node_id)", "Outer")), true},
		{"the notebook's nodes", scan("Index Scan", "nodes", "nodes_notebook_id_name_key_idx", "(n.notebook_id = $2)", ""), true},
		{"contents by an array of keys", `{"Node Type":"Bitmap Heap Scan","Relation Name":"page_contents","Plans":[` +
			scan("Bitmap Index Scan", "", "page_contents_pkey", "(node_id = ANY ($1))", "Outer") + `]}`, true},
		{"a subplan's bitmap is its own", `{"Node Type":"Bitmap Heap Scan","Relation Name":"nodes","Plans":[` +
			scan("Bitmap Index Scan", "", "nodes_notebook_id_parent_id_idx", "(notebook_id = $1)", "Outer") +
			`,{"Node Type":"Bitmap Heap Scan","Relation Name":"changeset_items","Parent Relationship":"SubPlan","Plans":[` +
			scan("Bitmap Index Scan", "", "changeset_items_node_id_idx", "(node_id = n.id)", "Outer") + `]}]}`, true},
		{"a Seq Scan", scan("Seq Scan", "nodes", "", "", ""), false},
		{"upgrade_nodes by another index", scan("Index Scan", "upgrade_nodes", "upgrade_nodes_notebook_id_idx", "(notebook_id = $1)", ""),
			false},
		{"upgrade_nodes by its key without the upgrade", loop(scan("Index Scan", "upgrade_nodes", "upgrade_nodes_pkey",
			"(node_id = n.id)", "Outer")), false},
		{"an index of nodes whole", scan("Index Scan", "nodes", "nodes_parent_id_name_key_key", "", ""), false},
		{"a node's key read once", `{"Node Type":"Hash Join","Plans":[` + baseline + `,{"Node Type":"Hash","Parent Relationship":"Inner",` +
			`"Plans":[` + scan("Index Scan", "nodes", "nodes_pkey", "(n.id = b.node_id)", "Outer") + `]}]}`, false},
		{"contents by another index", loop(scan("Index Scan", "page_contents", "page_contents_deleted_at_idx", "(c.node_id = b.node_id)",
			"Outer")), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var plan planNode
			if err := json.Unmarshal([]byte(tt.plan), &plan); err != nil {
				t.Fatal(err)
			}
			if problems := rollbackReadProblems(&plan); (len(problems) == 0) != tt.ok {
				t.Errorf("%s: the check answered %v, want accepted %v", planShape(&plan), problems, tt.ok)
			}
		})
	}
}
