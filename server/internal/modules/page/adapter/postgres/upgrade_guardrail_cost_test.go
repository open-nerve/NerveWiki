//go:build !race

package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The guardrails' reads' guard's rows (decision 240): eng's upgrade's
// baseline of guardrailCostPages pages, each of a content of
// guardrailContentBytes; another upgrade's of as many rows, of ops; and
// guardrailCostOthers notebooks of guardrailCostOtherPages pages each,
// with their upgrades' baselines, the rows the reads should not read. The
// guard reads and writes guardrailCostTimes of eng's pages.
const (
	guardrailCostPages      = 100000
	guardrailContentBytes   = 1024
	guardrailCostOthers     = 9
	guardrailCostOtherPages = 50000
	guardrailCostTimes      = 1000
)

// guardrailReadBound and guardrailWriteBound are the medians of
// guardrailCostTimes reads of UpgradeBaselineContent and writes of
// AcceptUpgradeContent the test below allows in each state and plan: three
// times the larger of the medians of two local rounds, rounded up to 5 ms
// (decisions 236, 240). The two rounds' medians, in the three states and
// two plans each, were 0.12 to 0.16 ms of a read and 0.15 to 0.19 ms of a
// write, both rounds alike; three times 0.19 ms is within 5 ms. The seed
// took 12.7 s.
const (
	guardrailReadBound  = 5 * time.Millisecond
	guardrailWriteBound = 5 * time.Millisecond
)

// The guardrails' reads' cost guard (M11/P2 design 7.1, decisions 236,
// 240, 244): the body's guardrail's read of a baseline page's content
// (UpgradeBaselineContent) and the ledger's write (AcceptUpgradeContent),
// each of a baseline page of an upgrade of 100,000, beside another
// upgrade's 100,000 rows and nine notebooks of 50,000 pages with their
// upgrades'. upgrade_nodes and page_contents never analyzed; then
// analyzed; then upgrade_nodes with the other upgrades' statistics alone,
// this one's written after them, as when a start's ANALYZE failed. In each
// state, for their arguments and generic, as a cached statement gets
// either (the generic plan the one force_generic_plan gives), the plans
// read upgrade_nodes by upgrade_nodes_pkey alone, upgrade_id and node_id
// both in its condition, and page_contents by page_contents_pkey on a
// Nested Loop's inner side, by the outer row's node_id (the LATERAL's
// LIMIT 1 keeps it from being pulled up into a join), as
// rollbackReadProblems judges them, no part of a table read whole; a
// problem stops the test before the reads run. Then, on a connection of
// their own forced to each plan, 1,000 reads answer each page's content and
// 1,000 writes take each page's hash over the one accepted last, the
// medians within guardrailReadBound and guardrailWriteBound. -v prints the
// medians and the seed's time.
func TestUpgradeGuardrailReadsTakeTheirTimeAtAnyStatistics(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	for _, table := range rollbackCostTables() {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)")
	}
	upgrade, other := uuid.NewV7(), uuid.NewV7()
	started := time.Now()
	f.exec(t, `WITH g AS MATERIALIZED (
			SELECT gen_random_uuid() AS id, g, repeat(md5(g::text), $5::integer / 32) AS content FROM generate_series(1, $4::integer) g
		), made AS (
			INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
				updated_at)
			SELECT id, $1, 'page', NULL, 'p' || g, 'p' || g, g, $3, $3, now(), now() FROM g
		), contents AS (
			INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
			SELECT id, content, 1, sha256(convert_to(content, 'UTF8')), octet_length(content), $3, now() FROM g
		)
		INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT $2, id, $1, true, 'page', NULL, 'p' || g, g, 1, sha256(convert_to(content, 'UTF8')), octet_length(content), 0,
			sha256(convert_to(content, 'UTF8'))
		FROM g`, f.eng, upgrade, f.alice, guardrailCostPages, guardrailContentBytes)
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT $1, gen_random_uuid(), $2, true, 'page', NULL, 'p' || g, g, 1, sha256(g::text::bytea), 1, 0, sha256(g::text::bytea)
		FROM generate_series(1, $3::integer) g`, other, f.ops, guardrailCostPages)
	f.exec(t, `INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at)
		SELECT gen_random_uuid(), b.workspace_id, 'Other ' || g, b.created_by_id, b.created_by_id, now(), now()
		FROM notebooks b, generate_series(1, $2::integer) g WHERE b.id = $1`, f.eng, guardrailCostOthers)
	f.exec(t, `INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		SELECT gen_random_uuid(), o.id, 'page', NULL, 'o' || k, 'o' || k, k, $2, $2, now(), now()
		FROM notebooks o, generate_series(1, $1::integer) k WHERE o.name LIKE 'Other %'`, guardrailCostOtherPages, f.alice)
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, 'x', 1, sha256('x'), 1, created_by_id, now() FROM nodes WHERE notebook_id IN (SELECT id FROM notebooks WHERE name LIKE 'Other %')`)
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT u.id, n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision, c.content_hash, c.byte_size, 0,
			c.content_hash
		FROM (SELECT id AS notebook_id, gen_random_uuid() AS id FROM notebooks WHERE name LIKE 'Other %') u
		JOIN nodes n ON n.notebook_id = u.notebook_id JOIN page_contents c ON c.node_id = n.id`)
	seeded := time.Since(started)

	// Every hundredth of eng's baseline pages, and the hash the ledger
	// accepted of each, which the writes take over.
	accepted := map[uuid.UUID][]byte{}
	var ids []uuid.UUID
	rows, err := f.pool.Query(ctx, `SELECT node_id, accepted_hash FROM (
			SELECT node_id, accepted_hash, row_number() OVER (ORDER BY node_id) AS r FROM upgrade_nodes WHERE upgrade_id = $1
		) b WHERE r % ($2::integer / $3::integer) = 0`, upgrade, guardrailCostPages, guardrailCostTimes)
	if err == nil {
		for rows.Next() {
			var id uuid.UUID
			var hash []byte
			if err = rows.Scan(&id, &hash); err != nil {
				break
			}
			ids, accepted[id] = append(ids, id), hash
		}
		rows.Close()
		if err == nil {
			err = rows.Err()
		}
	}
	if err != nil || len(ids) != guardrailCostTimes {
		t.Fatalf("the pages read: %d, %v", len(ids), err)
	}

	statements := map[string]statement{
		"UpgradeBaselineContent": statementOf(t, func(q *gen.Queries) error {
			_, err := q.UpgradeBaselineContent(ctx, gen.UpgradeBaselineContentParams{UpgradeID: upgrade, NodeID: ids[0]})
			return err
		}),
		"AcceptUpgradeContent": statementOf(t, func(q *gen.Queries) error {
			_, err := q.AcceptUpgradeContent(ctx, gen.AcceptUpgradeContentParams{UpgradeID: upgrade, NodeID: ids[0],
				After: accepted[ids[0]], Before: accepted[ids[0]]})
			return err
		}),
	}
	round := 0
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
		var planned bool
		for name, s := range statements {
			for _, generic := range []bool{false, true} {
				var plan planNode
				var raw string
				if generic {
					plan, raw = f.genericPlanTree(t, s.sql)
				} else {
					plan, raw = f.explain(t, `EXPLAIN (FORMAT JSON, VERBOSE, COSTS OFF) `+s.sql, s.args, nil)
				}
				if problems := guardrailReadProblems(&plan, name); len(problems) > 0 {
					planned = true
					t.Errorf("%s, %s, generic %v: %s\n%s\n%.3000s", statistics, name, generic, strings.Join(problems, "; "),
						planShape(&plan), raw)
				}
			}
		}
		if planned {
			t.FailNow()
		}
		for _, mode := range []string{"custom", "generic"} {
			round++
			read, write := guardrailTimed(t, f, mode, upgrade, ids, accepted, round)
			medians[statistics+", "+mode] = map[string]float64{"read_ms": ms(read), "write_ms": ms(write)}
			if read > guardrailReadBound || write > guardrailWriteBound {
				t.Errorf("%s, %s: the medians of %d reads and writes took %s and %s, past %s or %s", statistics, mode, len(ids), read, write,
					guardrailReadBound, guardrailWriteBound)
			}
		}
	}
	b, err := json.Marshal(map[string]any{"seed_ms": seeded.Milliseconds(), "medians": medians})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("guardrails' reads cost %s", b)
}

// ms is d in milliseconds, to a hundredth.
func ms(d time.Duration) float64 { return float64(d.Microseconds()/10) / 100 }

// guardrailTimed are the medians of a read of each page's content and a
// write of each page's hash to the ledger, over the hash it accepted last,
// on a connection of their own whose cached statements are planned as mode
// says; each read answers the page's content, each write takes, round
// telling the hashes apart.
func guardrailTimed(t *testing.T, f fixture, mode string, upgrade uuid.UUID, ids []uuid.UUID, accepted map[uuid.UUID][]byte,
	round int,
) (read, write time.Duration) {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "SET plan_cache_mode = force_"+mode+"_plan"); err != nil {
		t.Fatal(err)
	}
	s := postgresadapter.New(pool)
	var reads, writes []time.Duration
	for _, id := range ids {
		began := time.Now()
		content, ok, err := s.BaselineContent(ctx, upgrade, id)
		reads = append(reads, time.Since(began))
		if err != nil || !ok || len(content) != guardrailContentBytes {
			t.Fatalf("%s: BaselineContent of %s = %d bytes, %v, %v; want its content", mode, id, len(content), ok, err)
		}
	}
	for _, id := range ids {
		sum := sha256.Sum256(fmt.Appendf(nil, "%d %s", round, id))
		began := time.Now()
		err := s.AcceptUpgradeContent(ctx, upgrade, id, accepted[id], sum[:])
		writes = append(writes, time.Since(began))
		if err != nil {
			t.Fatalf("%s: AcceptUpgradeContent of %s: %v", mode, id, err)
		}
		accepted[id] = sum[:]
	}
	slices.Sort(reads)
	slices.Sort(writes)
	return reads[len(reads)/2], writes[len(writes)/2]
}

// guardrailReadProblems are the problems of a plan of the guardrails' read
// or of the ledger's write (M11/P2 design 7.1): rollbackReadProblems',
// and upgrade_nodes read other than with both of its key's columns in its
// index's condition, a page's row by its key alone; the read's
// page_contents read but once, on the Nested Loop's inner side.
func guardrailReadProblems(plan *planNode, name string) []string {
	problems := rollbackReadProblems(plan)
	for _, n := range plan.all(func(n *planNode) bool { return n.Relation == "upgrade_nodes" && n.Type != "ModifyTable" }) {
		cond := n.IndexCond
		for _, bitmap := range n.all(func(m *planNode) bool { return m.Index == "upgrade_nodes_pkey" }) {
			cond += " " + bitmap.IndexCond
		}
		if !strings.Contains(cond, "upgrade_id = ") || !strings.Contains(cond, "node_id = ") {
			problems = append(problems, fmt.Sprintf("upgrade_nodes read on %q, not its key's two columns", cond))
		}
	}
	contents := plan.all(func(n *planNode) bool { return n.Relation == "page_contents" })
	switch {
	case name == "UpgradeBaselineContent" && len(contents) != 1:
		problems = append(problems, fmt.Sprintf("page_contents read %d times, want once", len(contents)))
	case name != "UpgradeBaselineContent" && len(contents) != 0:
		problems = append(problems, "page_contents read")
	}
	return problems
}
