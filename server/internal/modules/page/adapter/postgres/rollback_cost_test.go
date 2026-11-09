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

// The rollback's reads' guard's notebooks (decisions 240, 244): eng,
// timed, of rollbackCostFolders folders at the root of rollbackCostPages
// pages each less one, 100,000 pages, its baseline, many parents of a few
// pages each, as a notebook has them; ops, its plans checked, not timed,
// of rollbackWideFolders folders of rollbackWidePages pages, as many
// pages below few parents; beside them rollbackCostOthers notebooks of
// rollbackCostOtherPages pages each, each with another upgrade's baseline
// of them, the rows the reads should not read.
const (
	rollbackCostFolders    = 10000
	rollbackCostPages      = 10
	rollbackWideFolders    = 100
	rollbackWidePages      = 1000
	rollbackCostOthers     = 9
	rollbackCostOtherPages = 50000
)

// rollbackLeftBound and rollbackReadsBound are the medians of three reads
// the test below allows in each state and plan: three times the larger of
// the medians of two local rounds, rounded up to 100 ms, RollbackLeft's
// within the request's 4 s too (decisions 226, 236, 244). The rounds'
// largest medians were 370 and 376 ms of RollbackLeft (330 to 376 ms in
// the six states and plans), 1,011 and 992 ms of the synthesized pass's
// reads (961 to 1,011 ms); the seed took 14.8 and 14.7 s, 2.4 and 2.5 s
// of it the first notebook's. On 100 folders of 1,000 pages alone, with
// RollbackFreeForest's NOT IN, the reads took up to 1,514 ms, the slowest
// three where its SubPlan was not hashed (decision 240's rounds).
const (
	rollbackLeftBound  = 1200 * time.Millisecond
	rollbackReadsBound = 3100 * time.Millisecond
)

// The rollback's reads' cost guard (M11/P1 design 7.1, decisions 226,
// 236, 240, 244): a notebook of 100,000 pages below 10,000 folders, the
// baseline of an upgrade of it, then every page below a folder moved under
// a new folder beside it, renamed or reordered, a third each, and 10,000
// written; another of 100,000 pages below 100 folders, as changed, with
// its own upgrade; beside them nine notebooks of 50,000 pages, each with
// another upgrade's baseline. upgrade_nodes, nodes and page_contents never
// analyzed; then analyzed; then upgrade_nodes with the other upgrades'
// statistics alone. Plans first, of both upgrades: in each, for their
// arguments and generic, RollbackLeft, and RollbackFreeForest,
// RollbackBaseline, RollbackHolders, RollbackNodesNow, RollbackContentsNow
// and ListRevertTree of the pass that synthesizes its plan, read as
// rollbackReadProblems says; a problem stops the test before the reads
// run, which with such a plan may take minutes. Time second, of the
// first notebook's: in each state and plan, forced on a connection of its
// own as a cached statement gets it, RollbackLeft answers what is left,
// the median of three within rollbackLeftBound; the synthesized pass's
// reads answer the baseline, its 10,000 holders, no free node, the nodes
// and the contents, the median of three within rollbackReadsBound. -v
// prints the medians and the seed's time.
//
// The first notebook's many parents are the shape the plans' costs of
// rows times rows grow with: as many holders as folders, a set the reads
// must not read again for each node (M11/P1 S3 review B-I1: RollbackFreeForest
// with NOT IN took 26.8 s on it, 0.3 s with NOT EXISTS). The second's few
// parents of many pages each, the guard's shape before, are checked for
// their plans, which differ, without doubling the guard's time.
func TestARollbacksReadsTakeTheirTimeAtAnyStatistics(t *testing.T) {
	f := newFixture(t)
	for _, table := range rollbackCostTables() {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)")
	}
	reads := rollbackCostReads{f: f, upgrade: uuid.NewV7(), notebook: f.eng, folders: rollbackCostFolders, pages: rollbackCostPages}
	wide := rollbackCostReads{f: f, upgrade: uuid.NewV7(), notebook: f.ops, folders: rollbackWideFolders, pages: rollbackWidePages}
	started := time.Now()
	reads.seed(t)
	notebook := time.Since(started)
	wide.seed(t)
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
			f.exec(t, "CREATE TABLE kept AS SELECT * FROM upgrade_nodes WHERE upgrade_id IN ($1, $2)", reads.upgrade, wide.upgrade)
			f.exec(t, "DELETE FROM upgrade_nodes WHERE upgrade_id IN ($1, $2)", reads.upgrade, wide.upgrade)
			f.exec(t, "ANALYZE upgrade_nodes")
			f.exec(t, "INSERT INTO upgrade_nodes SELECT * FROM kept")
		}
		var planned bool
		for shape, r := range map[string]*rollbackCostReads{"many parents": &reads, "few parents": &wide} {
			for name, s := range r.statements(t) {
				for _, generic := range []bool{false, true} {
					var plan planNode
					var raw string
					if generic {
						plan, raw = f.genericPlanTree(t, s.sql)
					} else {
						plan, raw = f.explain(t, `EXPLAIN (FORMAT JSON, VERBOSE, COSTS OFF) `+s.sql, s.args, nil)
					}
					if problems := rollbackReadProblems(&plan); len(problems) > 0 {
						planned = true
						t.Errorf("%s, %s, %s, generic %v: %s\n%s\n%.3000s", statistics, shape, name, generic, strings.Join(problems, "; "),
							planShape(&plan), raw)
					}
				}
			}
		}
		if planned {
			t.FailNow()
		}
		// What the seed left, read once its plans are checked: a read
		// whose plan is wrong may take minutes.
		if statistics == "never analyzed" {
			for _, r := range []*rollbackCostReads{&reads, &wide} {
				r.load(t)
				if want := (domain.RollbackLeft{Nodes: r.folders * (r.pages - 1), Pages: 10000, Created: r.folders}); r.left != want {
					t.Fatalf("the seed leaves %+v, want %+v", r.left, want)
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

// rollbackCostReads are the rollback's reads of one of the guard's
// upgrades, of notebook's folders of pages each: what is left and the
// baseline's rows, as the seed made them.
type rollbackCostReads struct {
	f        fixture
	upgrade  uuid.UUID
	notebook uuid.UUID
	folders  int
	pages    int
	left     domain.RollbackLeft
	rows     int // the baseline's
}

// seed makes the notebook: the folders, f, and the agent's new ones beside
// them, h; then the pages in their state now, each with its baseline row
// of its state before: below its folder, its name, its order; the
// folders' rows; every page's content, the baseline's but 10,000 written.
func (r *rollbackCostReads) seed(t *testing.T) {
	t.Helper()
	f := r.f
	f.exec(t, `WITH k AS MATERIALIZED (
			SELECT k, gen_random_uuid() AS folder, gen_random_uuid() AS holder FROM generate_series(0, $3::integer - 1) k
		), folders AS (
			INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
				updated_at)
			SELECT CASE v.s WHEN 'f' THEN folder ELSE holder END, $1, 'page', NULL, v.s || k, v.s || k, v.at + k, $2, $2, now(), now()
			FROM k, (VALUES ('f', 0), ('h', $3::integer)) v(s, at)
		), g AS MATERIALIZED (
			SELECT gen_random_uuid() AS id, folder, holder, 'f' || k || '-' || i AS name, i, row_number() OVER () AS j
			FROM k, generate_series(1, $4::integer - 1) i
		), made AS (
			INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
				updated_at)
			SELECT id, $1, 'page', CASE WHEN j % 3 = 0 THEN holder ELSE folder END, CASE WHEN j % 3 = 1 THEN 'r' || name ELSE name END,
				CASE WHEN j % 3 = 1 THEN 'r' || name ELSE name END, CASE WHEN j % 3 = 2 THEN i + 0.5 ELSE i END, $2, $2, now(), now()
			FROM g
		)
		INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT $5, id, $1, true, 'page', folder, name, i, 1, sha256('x'), 1, 0, sha256('x') FROM g`,
		r.notebook, f.alice, r.folders, r.pages, r.upgrade)
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT $1, id, notebook_id, true, 'page', NULL, name, sort_order, 1, sha256('x'), 1, 0, sha256('x') FROM nodes
		WHERE notebook_id = $2 AND parent_id IS NULL AND name LIKE 'f%'`, r.upgrade, r.notebook)
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, CASE WHEN w THEN 'y' ELSE 'x' END, CASE WHEN w THEN 2 ELSE 1 END, sha256(CASE WHEN w THEN 'y' ELSE 'x' END::bytea), 1,
			created_by_id, now()
		FROM (SELECT id, created_by_id, parent_id IS NOT NULL AND row_number() OVER (ORDER BY parent_id IS NULL, id) <= 10000 AS w
			FROM nodes WHERE notebook_id = $1) n`, r.notebook)
}

func (r *rollbackCostReads) load(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	var err error
	if r.left, err = r.f.s.RollbackLeft(ctx, r.upgrade, r.notebook); err != nil {
		t.Fatal(err)
	}
	baseline, err := r.f.s.RollbackBaseline(ctx, r.upgrade)
	if err != nil || len(baseline) != r.folders*r.pages {
		t.Fatalf("RollbackBaseline = %d rows, %v; want %d", len(baseline), err, r.folders*r.pages)
	}
	r.rows = len(baseline)
}

// statements are the reads' statements with the guard's arguments.
func (r *rollbackCostReads) statements(t *testing.T) map[string]statement {
	t.Helper()
	ctx := context.Background()
	return map[string]statement{
		"RollbackLeft": statementOf(t, func(q *gen.Queries) error {
			_, err := q.RollbackLeft(ctx, gen.RollbackLeftParams{UpgradeID: r.upgrade, NotebookID: r.notebook})
			return err
		}),
		"RollbackBaseline": statementOf(t, func(q *gen.Queries) error { _, err := q.RollbackBaseline(ctx, r.upgrade); return err }),
		"RollbackHolders":  statementOf(t, func(q *gen.Queries) error { _, err := q.RollbackHolders(ctx, r.upgrade); return err }),
		"RollbackFreeForest": statementOf(t, func(q *gen.Queries) error {
			_, err := q.RollbackFreeForest(ctx, gen.RollbackFreeForestParams{UpgradeID: r.upgrade, NotebookID: r.notebook})
			return err
		}),
		"RollbackNodesNow":    statementOf(t, func(q *gen.Queries) error { _, err := q.RollbackNodesNow(ctx, r.upgrade); return err }),
		"RollbackContentsNow": statementOf(t, func(q *gen.Queries) error { _, err := q.RollbackContentsNow(ctx, r.upgrade); return err }),
		"ListRevertTree":      statementOf(t, func(q *gen.Queries) error { _, err := q.ListRevertTree(ctx, r.notebook); return err }),
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
		got, err := s.RollbackLeft(ctx, r.upgrade, r.notebook)
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
	forest, err := s.RollbackFreeForest(ctx, r.upgrade, r.notebook)
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
	tree, err := s.ListRevertTree(ctx, r.notebook)
	if err != nil {
		return err
	}
	if len(forest) != 0 || len(baseline) != r.rows || len(holders) != r.folders || len(now) != r.rows ||
		len(contents) != r.rows || len(tree) != r.rows+r.folders {
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
// (M11/P1 design 7.1, decisions 236, 240, 244): upgrade_nodes read other
// than by upgrade_nodes_pkey with upgrade_id in its condition; nodes other
// than by an index with notebook_id in its condition or by nodes_pkey on
// id; page_contents other than by page_contents_pkey on node_id; a read by
// a column of a join's other side not repeated on a Nested Loop's inner
// side, for each of its rows; any other scan, a Seq Scan, of them. And
// what is read again for each row, on a Nested Loop's inner side or under
// a SubPlan that is not hashed (run for each row of the node it filters):
// a scan of them by no column of the outer row, its rows the same each
// time; a CTE Scan, a WorkTable Scan or a Materialize, a set read whole
// each time (M11/P1 S3 review B-I1, B-M1, A-N2). Each is a cost of rows
// times rows; the plan, not the time on one seed's shape, tells it.
func rollbackReadProblems(plan *planNode) []string {
	var problems []string
	var visit func(n *planNode, repeated bool)
	visit = func(n *planNode, repeated bool) {
		switch {
		case repeated && (n.Type == "CTE Scan" || n.Type == "WorkTable Scan" || n.Type == "Materialize"):
			problems = append(problems, fmt.Sprintf("a %s %s read whole for each row", n.Type, n.CTEName))
			repeated = false
		case n.Type == "Materialize" || n.Type == "Hash":
			repeated = false
		}
		switch n.Relation {
		case "upgrade_nodes", "nodes", "page_contents":
			problems = append(problems, rollbackScanProblems(n, repeated)...)
		}
		for i := range n.Plans {
			below := &n.Plans[i]
			visit(below, repeated || n.Type == "Nested Loop" && below.Parent == "Inner" ||
				below.Parent == "SubPlan" && !rollbackHashed(n, below.SubplanName))
		}
	}
	visit(plan, false)
	return problems
}

// rollbackHashed tells whether n's SubPlan name is hashed: run once into a
// hash table its expressions probe, "(hashed SubPlan 2)", not again for
// each row, "(SubPlan 2)" (PostgreSQL 17 on; "hashed SubPlan 2" its name
// before).
func rollbackHashed(n *planNode, name string) bool {
	if strings.HasPrefix(name, "hashed ") {
		return true
	}
	if name == "" {
		return false
	}
	exprs := strings.Join(append([]string{n.Filter, n.JoinFilter, n.IndexCond, n.RecheckCond, n.HashCond, n.MergeCond}, n.Output...), " ")
	return strings.Contains(exprs, "(hashed "+name+")") && !strings.Contains(exprs, "("+name+")")
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
		case repeated && !rollbackJoined.MatchString(cond):
			problems = append(problems, fmt.Sprintf("%s read by %s on %q for each row, by no column of it", n.Relation, ix.Index, cond))
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
// 240, 244) on plans written by hand, without a database, so that the
// short line (make test-go-short), which builds this file without race,
// runs it: a key read for each row on a Nested Loop's inner side, a read
// by the notebook's key, a read by an array of keys, a Bitmap Heap Scan
// whose subplan reads another table and a hashed SubPlan's CTE Scan are
// accepted; each other plan breaks one rule: among them a set read again
// for each row by no key of it, reviewer A's three and reviewer B's
// (M11/P1 S3 review A-N2, B-M1), and a CTE read whole for each row by a
// SubPlan not hashed, the NOT IN of RollbackFreeForest once analyzed
// (B-I1).
func TestTheRollbackGuardJudgesThePlans(t *testing.T) {
	scan := func(kind, relation, index, cond, parent string) string {
		return `{"Node Type":"` + kind + `","Relation Name":"` + relation + `","Index Name":"` + index + `","Index Cond":"` + cond +
			`","Parent Relationship":"` + parent + `"}`
	}
	filtered := func(kind, relation, index, cond, filter, parent string) string {
		return `{"Node Type":"` + kind + `","Relation Name":"` + relation + `","Index Name":"` + index + `","Index Cond":"` + cond +
			`","Filter":"` + filter + `","Parent Relationship":"` + parent + `"}`
	}
	notIn := func(hashed string) string {
		return `{"Node Type":"Index Scan","Relation Name":"nodes","Index Name":"nodes_notebook_id_parent_id_idx",` +
			`"Index Cond":"(n.notebook_id = $2)","Filter":"((n.deleted_at IS NULL) AND (NOT (ANY (n.id = (` + hashed + `SubPlan 2).col1))))",` +
			`"Plans":[{"Node Type":"CTE Scan","CTE Name":"holders","Parent Relationship":"SubPlan","Subplan Name":"SubPlan 2"}]}`
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
		{"the notebook's nodes read for each baseline row", loop(filtered("Index Scan", "nodes", "nodes_notebook_id_name_key_idx",
			"(n.notebook_id = $2)", "(n.id = b.node_id)", "Outer")), false},
		{"the notebook's nodes by a bitmap for each baseline row", loop(`{"Node Type":"Bitmap Heap Scan","Relation Name":"nodes",` +
			`"Parent Relationship":"Outer","Filter":"(n.id = b.node_id)","Plans":[` +
			scan("Bitmap Index Scan", "", "nodes_notebook_id_parent_id_idx", "(notebook_id = $2)", "Outer") + `]}`), false},
		{"the upgrade's rows read for each baseline row", loop(filtered("Index Scan", "upgrade_nodes", "upgrade_nodes_pkey",
			"(u.upgrade_id = $1)", "(u.node_id = b.node_id)", "Outer")), false},
		{"the upgrade's rows for each row, the node's key lost", loop(scan("Index Scan", "upgrade_nodes", "upgrade_nodes_pkey",
			"(b.upgrade_id = $1)", "Outer")), false},
		{"a CTE read whole for each row, a SubPlan not hashed", notIn(""), false},
		{"a CTE read once, a hashed SubPlan", notIn("hashed "), true},
		{"a set materialized, read whole for each row", `{"Node Type":"Nested Loop","Plans":[` + baseline +
			`,{"Node Type":"Materialize","Parent Relationship":"Inner","Plans":[` +
			scan("Index Scan", "nodes", "nodes_notebook_id_name_key_idx", "(n.notebook_id = $2)", "Outer") + `]}]}`, false},
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
