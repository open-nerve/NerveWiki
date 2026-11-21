//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

const (
	widePlanPages   = 6000   // the notebook's pages, each with widePlanLinks links
	widePlanLinks   = 20     // links a page
	widePlanReached = 4096   // the reach's pages: their names' keys, their ids as targets and sources
	widePlanForeign = 200000 // the other notebook's links, to the same keys
	widePlanStride  = 293    // page i's link j leads to page (i+1+293j) mod 6000
)

// widePlanLocal is each cell's mean of two local runs of the fastest of
// its three reads (M12/P4/S7 design 4.2; a run's log has them; 8 cores,
// arm64, through heavy.sh): the bound is three times it and 100 ms, a
// backstop, not what tells a quadratic read, which the plan's shape and
// the rows its nodes read do.
func widePlanLocal() map[string]time.Duration {
	return map[string]time.Duration{
		"no_statistics/auto":                                69 * time.Millisecond,  // 69.0, 69.1
		"no_statistics/auto/64kB":                           109 * time.Millisecond, // 111.4, 106.5
		"no_statistics/force_custom_plan":                   69 * time.Millisecond,  // 69.8, 68.9
		"no_statistics/force_custom_plan/64kB":              107 * time.Millisecond, // 107.0, 107.0
		"no_statistics/force_generic_plan":                  63 * time.Millisecond,  // 63.1, 63.0
		"analyzed/auto":                                     61 * time.Millisecond,  // 61.7, 60.1
		"analyzed/auto/64kB":                                50 * time.Millisecond,  // 49.5, 49.8
		"analyzed/force_custom_plan":                        60 * time.Millisecond,  // 60.3, 60.5
		"analyzed/force_custom_plan/64kB":                   49 * time.Millisecond,  // 49.4, 48.6
		"analyzed/force_generic_plan":                       56 * time.Millisecond,  // 56.9, 55.7
		"app_rebuild_after_analyzed/auto":                   81 * time.Millisecond,  // 81.9, 80.3
		"app_rebuild_after_analyzed/auto/64kB":              90 * time.Millisecond,  // 90.7, 89.2
		"app_rebuild_after_analyzed/force_custom_plan":      81 * time.Millisecond,  // 82.9, 79.5
		"app_rebuild_after_analyzed/force_custom_plan/64kB": 91 * time.Millisecond,  // 90.9, 92.0
		"app_rebuild_after_analyzed/force_generic_plan":     99 * time.Millisecond,  // 98.7, 98.8
	}
}

// widePlanWorkMem is the cells under work_mem = 64kB: a constant array's
// hash does not look at it, a sub-select's does (design 1.3 item 3).
const widePlanWorkMem = "64kB"

// A reach of many keys and targets reads its links in one pass over the
// rows they meet, linear at any statistics and any plan_cache_mode (M12/P4/S7
// design 4.2): through Store.Links, LinksReachedWide, its arrays constants
// whose = ANY is hashed, or under force_generic_plan LinksReachedWideGeneric,
// whose sub-selects are hashed sub-plans. Each cell first explains the
// statement without running it and fails on a plan that compares a row
// with a parameter array (a filter, a join filter, a recheck of a lossy
// bitmap), on a sub-plan not hashed, an anti-join, a nested loop over an
// unnest; then reads the reach through Store.Links, the rows the
// arithmetic of the fixture writes, LinksReached's too where it is planned
// with its arguments, no DML, each scan of page_links reading no more rows
// than the notebook has (the other notebook, larger, holds the same keys:
// a whole table's scan reads 320,000), and the fastest of three reads
// within its bound.
func TestLinksReachedWideReadPlans(t *testing.T) {
	if testing.Short() {
		t.Skip("native PostgreSQL wide LinksReached qualification")
	}
	for _, state := range []string{"no_statistics", "analyzed", "app_rebuild_after_analyzed"} {
		t.Run(state, func(t *testing.T) {
			f := newWidePlanFixture(t, state)
			for _, cell := range []struct{ mode, workMem string }{
				{"auto", ""}, {"auto", widePlanWorkMem}, {"force_custom_plan", ""}, {"force_custom_plan", widePlanWorkMem},
				{"force_generic_plan", ""},
			} {
				name := cell.mode
				if cell.workMem != "" {
					name += "_work_mem_" + cell.workMem
				}
				t.Run(name, func(t *testing.T) { widePlanCell(t, f, cell.mode, cell.workMem) })
			}
		})
	}
}

type widePlanFixture struct {
	*stablePlanFixture
	pages []uuid.UUID
	names []string
	reach domain.Reach
	want  []app.Link
	own   int // the notebook's links
}

func widePlanTarget(i, j int) int { return (i + 1 + widePlanStride*j) % widePlanPages }

func widePlanContent(i int) string {
	var b strings.Builder
	for j := range widePlanLinks {
		if j > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "[[w%04d]]", widePlanTarget(i, j))
	}
	return b.String()
}

func newWidePlanFixture(t *testing.T, state string) *widePlanFixture {
	t.Helper()
	f := &widePlanFixture{stablePlanFixture: &stablePlanFixture{fixture: newFixture(t), state: state}, own: widePlanPages * widePlanLinks}
	f.eng, f.ops = stablePlanID(900001), stablePlanID(900002)
	ingestPlanSetup(t, f.fixture)
	for _, table := range []string{"page_tags", "page_aliases"} {
		lintExec(t, f.fixture, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	for i := range widePlanPages {
		f.pages = append(f.pages, stablePlanID(uint64(400000+i)))
		f.names = append(f.names, fmt.Sprintf("w%04d", i))
	}
	if !slices.IsSortedFunc(f.pages, uuid.UUID.Compare) {
		t.Fatal("the pages' ids must be sorted as the pages")
	}
	// The parser writes each page's links where the arithmetic says: the
	// rows below, and those a rebuild writes, are alike.
	for _, i := range []int{0, 4095, 4096, widePlanPages - 1} {
		facts := originFacts(t, widePlanContent(i))
		if len(facts.Links) != widePlanLinks {
			t.Fatalf("page %d parses into %d links", i, len(facts.Links))
		}
		for j, l := range facts.Links {
			name := fmt.Sprintf("w%04d", widePlanTarget(i, j))
			if l != (domain.Link{Kind: "wikilink", Target: name, Start: 10*j + 2, End: 10*j + 7}) || !slices.Equal(l.Keys(), []string{name}) {
				t.Fatalf("page %d's link %d parses into %+v", i, j, l)
			}
		}
	}
	lintExec(t, f.fixture, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
SELECT id,$1,1,2,true FROM unnest($2::uuid[]) p(id)`, f.eng, f.pages)
	lintExec(t, f.fixture, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,
resolved_id,ambiguous,resolved_asset,aliases)
SELECT p.id,10*j+2,10*j+7,$1,'wikilink',n.name,n.name,n.id,false,false,false
FROM unnest($2::uuid[]) WITH ORDINALITY p(id,ord)
CROSS JOIN generate_series(0,$4::int-1) j
JOIN unnest($3::text[],$2::uuid[]) WITH ORDINALITY n(name,id,ord) ON n.ord=(p.ord+$5::int*j) % $6::int + 1
ORDER BY p.ord,j`, f.eng, f.pages, f.names, widePlanLinks, widePlanStride, widePlanPages)
	// The other notebook: the same keys, its own pages, more rows.
	lintExec(t, f.fixture, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,
resolved_id,ambiguous,resolved_asset,aliases)
SELECT ('50000000-0000-0000-0000-'||lpad(to_hex(k/20),12,'0'))::uuid,10*(k%20)+2,10*(k%20)+7,$1,'wikilink',
($2::text[])[(k*7)%6000+1],($2::text[])[(k*7)%6000+1],('51000000-0000-0000-0000-'||lpad(to_hex((k*7)%6000),12,'0'))::uuid,false,false,false
FROM generate_series(0,$3::int-1) k`, f.ops, f.names, widePlanForeign)
	for _, table := range stablePlanTables() {
		lintExec(t, f.fixture, "VACUUM "+table) // Explicitly no ANALYZE here.
	}
	tr := &stablePlanTrace{}
	pool := lintPool(t, f.fixture, tr, 1)
	f.pool, f.s, f.tx, f.trace = pool, postgresadapter.New(pool), postgres.NewTxManager(pool, time.Second), tr
	switch state {
	case "no_statistics":
	case "analyzed", "app_rebuild_after_analyzed":
		if err := f.s.Analyze(t.Context()); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown wide fixture state %q", state)
	}
	if state == "app_rebuild_after_analyzed" {
		widePlanRebuild(t, f)
	}
	f.reach = domain.Reach{Keys: slices.Clone(f.names[:widePlanReached]), Targets: slices.Clone(f.pages[:widePlanReached]),
		Sources: slices.Clone(f.pages[:widePlanReached])}
	for i := range widePlanPages {
		for j := range widePlanLinks {
			to := widePlanTarget(i, j)
			if i < widePlanReached || to < widePlanReached {
				f.want = append(f.want, app.Link{SourceID: f.pages[i], Start: 10*j + 2, Target: f.names[to],
					Resolution: domain.Resolution{ID: f.pages[to]}})
			}
		}
	}
	var total, own int
	if err := f.pool.QueryRow(t.Context(), "SELECT count(*), count(*) FILTER (WHERE notebook_id=$1) FROM page_links", f.eng).Scan(&total, &own); err != nil ||
		total != f.own+widePlanForeign || own != f.own {
		t.Fatalf("the fixture's links: %d, the notebook's %d; want %d and %d (%v)", total, own, f.own+widePlanForeign, f.own, err)
	}
	return f
}

// widePlanRebuild rebuilds the notebook's index with the real Rebuild
// after the statistics: its rows are written anew, the statistics left
// as they were (the M10 plan tests' app_rebuild_after_analyzed).
func widePlanRebuild(t *testing.T, f *widePlanFixture) {
	t.Helper()
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	parser := markdownadapter.NewParser(md, markdown.NewBudget(8<<20, 2*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))))
	p := &stablePlanInputs{notebook: f.eng, nodes: map[uuid.UUID]domain.Node{}, byKey: map[string][]domain.Node{}}
	for i, id := range f.pages {
		p.specs = append(p.specs, stablePlanSpec{ID: id, Name: f.names[i], OldText: widePlanContent(i)})
		n := domain.Node{ID: id, Path: []domain.Step{{ID: id, Key: f.names[i], Name: f.names[i]}}}
		p.nodes[id], p.byKey[f.names[i]] = n, []domain.Node{n}
	}
	rebuild := app.Rebuild{Tx: postgres.NewTxManager(f.pool, time.Minute), Index: app.Index{Store: f.s, Pages: p, Publisher: p},
		Contents: p, Notebooks: p, Parser: parser, Writes: f.s, StagingLimit: 1}
	rebuilt, err := rebuild.Notebook(t.Context(), f.eng)
	if err != nil || rebuilt.Pages != widePlanPages || rebuilt.Links != f.own || rebuilt.Unresolved != 0 || p.locked != 1 || p.rekeyed != 1 {
		t.Fatalf("the real rebuild: %+v locks=%d rekeys=%d: %v", rebuilt, p.locked, p.rekeyed, err)
	}
}

// widePlanNode is an EXPLAIN's node, as the shapes below read it.
type widePlanNode struct {
	Type       string         `json:"Node Type"`
	Relation   string         `json:"Relation Name"`
	Parent     string         `json:"Parent Relationship"`
	JoinType   string         `json:"Join Type"`
	Function   string         `json:"Function Name"`
	Filter     string         `json:"Filter"`
	JoinFilter string         `json:"Join Filter"`
	Recheck    string         `json:"Recheck Cond"`
	IndexCond  string         `json:"Index Cond"`
	Rows       float64        `json:"Actual Rows"`
	Loops      float64        `json:"Actual Loops"`
	Removed    float64        `json:"Rows Removed by Filter"`
	JoinRemove float64        `json:"Rows Removed by Join Filter"`
	Rechecked  float64        `json:"Rows Removed by Index Recheck"`
	Lossy      float64        `json:"Lossy Heap Blocks"`
	Plans      []widePlanNode `json:"Plans"`
}

func (n widePlanNode) all() []widePlanNode {
	out := []widePlanNode{n}
	for _, child := range n.Plans {
		out = append(out, child.all()...)
	}
	return out
}

func widePlanTree(t *testing.T, raw string) widePlanNode {
	t.Helper()
	var plans []struct{ Plan widePlanNode }
	if err := json.Unmarshal([]byte(raw), &plans); err != nil || len(plans) != 1 {
		t.Fatalf("the JSON plan: %v", err)
	}
	return plans[0].Plan
}

// parameterArray is a comparison with a parameter's array: a row compared
// with each of its elements (design 1.3 items 3, 4).
var parameterArray = regexp.MustCompile(`ANY \(\$\d`)

// widePlanSubPlan is a sub-plan in a plan's JSON: its node's name, or an
// expression's use of it. PostgreSQL 18 names the node "SubPlan N" and
// says "hashed SubPlan N" where an expression uses it hashed.
var widePlanSubPlan = regexp.MustCompile(`("Subplan Name": ")?(hashed )?SubPlan (\d+)\b`)

// widePlanSubPlansHashed fails a sub-plan that an expression uses without
// its hash, or that none uses hashed (InitPlans run once and are not
// looked at).
func widePlanSubPlansHashed(t *testing.T, raw string) {
	t.Helper()
	named, hashed := map[string]bool{}, map[string]bool{}
	for _, m := range widePlanSubPlan.FindAllStringSubmatch(raw, -1) {
		switch {
		case m[1] != "":
			named[m[3]] = true
		case m[2] == "":
			t.Fatalf("an expression uses SubPlan %s without its hash\n%s", m[3], raw)
		default:
			hashed[m[3]] = true
		}
	}
	for n := range named {
		if !hashed[n] {
			t.Fatalf("the sub-plan SubPlan %s is not used hashed\n%s", n, raw)
		}
	}
}

// widePlanShape fails a plan that is not linear in the reach (design 4.2):
// a row compared with a parameter's array in a filter, a join filter or the
// recheck of a lossy bitmap; a sub-plan not hashed; an anti-join (the
// generic statement's exclusion must stay a hashed sub-plan, review A's
// N2); a nested loop whose inner side is a function's scan run more than
// once. analyzed says the plan ran: the recheck and the loops are known.
func widePlanShape(t *testing.T, raw string, analyzed bool) {
	t.Helper()
	root := widePlanTree(t, raw)
	widePlanSubPlansHashed(t, raw)
	for _, n := range root.all() {
		for _, cond := range []string{n.Filter, n.JoinFilter} {
			if parameterArray.MatchString(cond) {
				t.Fatalf("a %s compares each row with a parameter's array: %q\n%s", n.Type, cond, raw)
			}
		}
		if analyzed && (n.Lossy > 0 || n.Rechecked > 0) && parameterArray.MatchString(n.Recheck) {
			t.Fatalf("a %s rechecks a lossy bitmap against a parameter's array: %q\n%s", n.Type, n.Recheck, raw)
		}
		if n.JoinType == "Anti" || n.JoinType == "Semi" {
			t.Fatalf("a %s %s join\n%s", n.JoinType, n.Type, raw)
		}
		if n.Type == "Nested Loop" {
			for _, child := range n.Plans {
				if child.Parent == "Inner" && child.Type == "Function Scan" && (!analyzed || child.Loops > 1) {
					t.Fatalf("a nested loop over %s\n%s", child.Function, raw)
				}
			}
		}
	}
}

// widePlanRead fails a scan of page_links that read more rows than the
// notebook holds: of the other notebook's, or of a row more than once.
func widePlanRead(t *testing.T, raw string, own int) {
	t.Helper()
	for _, n := range widePlanTree(t, raw).all() {
		if n.Relation != "page_links" {
			continue
		}
		// Parallel nodes give each process's mean; one row of rounding.
		if read := (n.Rows + n.Removed + n.Rechecked) * max(n.Loops, 1); read > float64(own)+n.Loops {
			t.Fatalf("a %s of page_links read %.0f rows, more than the notebook's %d\n%s", n.Type, read, own, raw)
		}
	}
}

func widePlanCell(t *testing.T, f *widePlanFixture, mode, workMem string) {
	cell := f.state + "/" + mode
	if workMem != "" {
		cell += "/" + workMem
	}
	statement, sql := "LinksReachedWide", gen.LinksReachedWide
	if mode == "force_generic_plan" {
		statement, sql = "LinksReachedWideGeneric", gen.LinksReachedWideGeneric
	}
	args := []any{f.eng, f.reach.Keys, f.reach.Targets, f.reach.Sources}
	var reads []time.Duration
	var planned, analyzed string
	rollback := errors.New("the wide read's observation rolls back")
	err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
		tx, ok := postgres.TxFrom(ctx)
		if !ok || postgres.DB(ctx, f.pool) != tx {
			t.Fatal("the wide read lost its transaction")
		}
		stablePlanSettings(t, ctx, tx, mode)
		if workMem != "" {
			if _, err := tx.Exec(ctx, "SET LOCAL work_mem = '"+workMem+"'"); err != nil {
				return err
			}
		}
		// The shape first, not run: a quadratic plan of 8,192 keys and
		// targets may take many seconds (review C's N2).
		// pgx keeps the name prepared: Deallocate, not DEALLOCATE, forgets
		// it, else the next cell's Prepare is a no-op of a gone statement.
		const name = "links_reached_wide_observation"
		if _, err := tx.Conn().Prepare(ctx, name, sql); err != nil {
			return err
		}
		defer func() { _ = tx.Conn().Deallocate(context.WithoutCancel(ctx), name) }()
		execute := stablePlanExecute(t, name, args)
		if err := tx.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+execute).Scan(&planned); err != nil {
			return err
		}
		widePlanShape(t, planned, false)
		// Store.Links reads the statement of the mode, planned with the reach.
		work := stablePlanRelationWork(t, ctx, tx)
		f.trace.begin()
		got, err := f.s.Links(ctx, f.eng, f.reach)
		calls := f.trace.end()
		delta := stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
		if err != nil {
			return err
		}
		var names []string
		for _, c := range calls {
			names = append(names, c.Name)
		}
		if !slices.Equal(names, []string{"PlanCacheMode", statement}) ||
			!slices.Equal(calls[1].LeadingOptions, []string{fmt.Sprintf("leading=true mode=%v", pgx.QueryExecModeDescribeExec)}) {
			t.Fatalf("Store.Links read %v (%+v); want PlanCacheMode and %s planned with the reach", names, calls, statement)
		}
		if !slices.Equal(got, f.want) {
			t.Fatalf("Store.Links read %d rows, not the fixture's %d", len(got), len(f.want))
		}
		widePlanNoDML(t, delta)
		if mode != "force_generic_plan" {
			// LinksReached compares each row with every source in a
			// generic plan (design 1.3 item 5): planned with the reach alone.
			if narrow := widePlanNarrow(t, ctx, tx, args); !slices.Equal(narrow, f.want) {
				t.Fatalf("LinksReached read %d rows, not the wide read's %d", len(narrow), len(f.want))
			}
		}
		before := stablePlanCounter(t, ctx, tx, sql)
		work = stablePlanRelationWork(t, ctx, tx)
		if err := tx.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+execute).Scan(&analyzed); err != nil {
			return err
		}
		widePlanNoDML(t, stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx)))
		stablePlanCountDelta(t, before, stablePlanCounter(t, ctx, tx, sql), mode)
		if err := tx.Conn().Deallocate(ctx, name); err != nil {
			return err
		}
		widePlanShape(t, analyzed, true)
		widePlanRead(t, analyzed, f.own)
		for range 3 {
			began := time.Now()
			if _, err := f.s.Links(ctx, f.eng, f.reach); err != nil {
				return err
			}
			reads = append(reads, time.Since(began))
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("the wide read's observation: %v", err)
	}
	fastest := slices.Min(reads)
	t.Logf("%s: %s, the fastest of three reads %v (%v)", cell, statement, fastest, reads)
	if t.Failed() {
		return
	}
	local, ok := widePlanLocal()[cell]
	if !ok {
		t.Fatalf("%s: no local mean; plan %s", cell, analyzed)
	}
	if bound := 3*local + 100*time.Millisecond; fastest > bound {
		t.Fatalf("%s: the fastest of three wide reads took %v, past %v (three times the local %v and 100 ms)", cell, fastest, bound, local)
	}
}

func widePlanNoDML(t *testing.T, work map[string]stablePlanWork) {
	t.Helper()
	for relation, w := range work {
		if w.Inserted != 0 || w.Updated != 0 || w.Deleted != 0 {
			t.Fatalf("the wide read wrote %s: %+v", relation, w)
		}
	}
}

// widePlanNarrow is LinksReached's rows of the reach, read planned.
func widePlanNarrow(t *testing.T, ctx context.Context, tx pgx.Tx, args []any) []app.Link {
	t.Helper()
	rows, err := postgres.Planned(tx).Query(ctx, gen.LinksReached, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []app.Link
	for rows.Next() {
		var l app.Link
		var start int32
		var resolved pgtype.UUID
		if err := rows.Scan(&l.SourceID, &start, &l.Target, &resolved, &l.Resolution.Ambiguous, &l.Resolution.Asset, &l.Aliases); err != nil {
			t.Fatal(err)
		}
		l.Start, l.Resolution.ID = int(start), uuid.UUID(resolved.Bytes)
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
