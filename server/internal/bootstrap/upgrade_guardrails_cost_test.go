//go:build !race

package bootstrap

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// guardrailUnitDeadline is the most a unit of the shapes below may hold
// the notebook's row (decisions 153, 243): within the bench's request of
// 15 s, a second clear of it. guardrailUnitBounds are the medians of three
// the guard allows each shape while upgrading, three times the larger of
// the medians of two local rounds, rounded up to 100 ms, its regression's
// bound: the unit's (unit_ms) for a to d, the call's for e, which its plan
// refuses before any unit. The rounds' medians, upgrading and not: a 1,701
// and 1,712 ms, 1,686 and 1,647 not upgraded; b 343 and 348, 293 and 294;
// c 47 and 44, 35 and 36; d 23 and 26, 15 and 17; e 2 and 3, 251 and 252
// (its 100 writes done); the seed 4.4 s.
const guardrailUnitDeadline = 14 * time.Second

func guardrailUnitBounds() map[string]time.Duration {
	return map[string]time.Duration{"a": 5200 * time.Millisecond, "b": 1100 * time.Millisecond, "c": 200 * time.Millisecond,
		"d": 100 * time.Millisecond, "e": 100 * time.Millisecond}
}

// The guardrails' units at the batch's bounds (M11/P2 design 7.1, review
// P2A-M4), the initiator's agent's batches in a notebook being upgraded,
// every page of the baseline, on the batch bench (production's bounds and
// request):
//
//	a. a rename whose rewrite writes MaxBatchRewrites pages of the full
//	   load (84 KB, 200 links): the system's, unread by the guard, a
//	   ledger's write a page;
//	b. 100 edits, each taking a frontmatter line of some 62 KB off a page
//	   of some 145 KB, the pages' heads together just within
//	   MaxBatchReadBytes, their final contents within MaxUnitBytes: the
//	   plan reads them whole, the guard again, each hashed twice (the
//	   design's 140 KB edits of the frontmatter alone are past
//	   MaxUnitBytes, CheckUnit's, at their final size);
//	c. an edit of the frontmatter of a page of MaxContentBytes;
//	d. blind writes of three pages, two of MaxContentBytes and one of some
//	   4 MiB, their heads together just within MaxBatchReadBytes, each
//	   taking a frontmatter off: the largest unit a batch writes while
//	   upgrading, the guard reading each page whole;
//	e. 100 blind writes, each taking a frontmatter of some 5 MiB off a
//	   page: refused by the plan, page.batch_too_large at ops[2], nothing
//	   written, no unit run (decision 266).
//
// Each three times; each unit of a to d within guardrailUnitDeadline, the
// medians within guardrailUnitBounds, e's call's too. The same shapes in
// the notebook not upgraded, three times each, give the medians the
// guardrails add to; e is done there. The pages of b to e are written by
// SQL, each written back between runs, the ledger with them; a and c
// write back by their next run. -v prints every median.
func TestUpgradeGuardrailsAtTheBatchBoundTakeTheirTime(t *testing.T) {
	if testing.Short() {
		t.Skip("a bound of time against a database")
	}
	started := time.Now()
	bench := newBatchBench(t, 0, 0, 0)
	bench.rewritten(t, 0, domain.MaxBatchRewrites-1) // P1 links to T0 too: MaxBatchRewrites pages
	random := rand.New(rand.NewPCG(11, 21))
	text := func(n int) string {
		b := make([]byte, (n+1)/2)
		for i := range b {
			b[i] = byte(random.Uint32())
		}
		return hex.EncodeToString(b)[:n]
	}
	// b: frontmatter lines of 62,000 bytes off pages of 145,023.
	var pages []sqlPage
	var edits []map[string]any
	for i := range domain.MaxBatchOps {
		line := "k: " + text(62000) + "\n"
		pages = append(pages, sqlPage{"", fmt.Sprint("B", i), "---\ntype: note\n" + line + "---\n" + text(83000) + "\n"})
		edits = append(edits, opEdit(fmt.Sprint("B", i, ".md"), line, ""))
	}
	// c: a page of MaxContentBytes.
	pages = append(pages, sqlPage{"", "C", "---\ntype: a\n---\n" + text(page.MaxContentBytes-len("---\ntype: a\n---\n")-1) + "\n"})
	// d: three pages whose heads are within MaxBatchReadBytes by 1,000.
	big := func(n int) string {
		return "---\nk: " + text(n-len("---\nk: ")-len("\n---\nBody.\n")) + "\n---\nBody.\n"
	}
	five := page.MaxContentBytes - 100
	for i, n := range []int{five, five, domain.MaxBatchReadBytes - 2*five - 1000} {
		pages = append(pages, sqlPage{"", fmt.Sprint("D", i), big(n)})
	}
	seedPagesSQL(t, bench.pool, bench.nb.String(), pages)
	// e: 100 pages of a frontmatter of some 5 MiB, of one hash repeated:
	// the plan reads their heads alone.
	bench.exec(t, `WITH e AS MATERIALIZED (
			SELECT gen_random_uuid() AS id, 'E' || g AS name, g,
				'---' || chr(10) || 'k: ' || repeat(md5(g::text), $2::integer) || chr(10) || '---' || chr(10) || 'Body.' || chr(10) AS content
			FROM generate_series(0, $3::integer - 1) g
		), changeset AS (
			INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
			SELECT gen_random_uuid(), id, 'edit', 'web', created_by_id, now(), now() FROM notebooks WHERE id = $1 RETURNING id
		), made AS (
			INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
				updated_at)
			SELECT e.id, b.id, NULL, 'page', e.name, lower(e.name), 5000 + e.g, b.created_by_id, b.created_by_id, now(), now()
			FROM e, notebooks b WHERE b.id = $1
		), contents AS (
			INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
			SELECT e.id, e.content, 1, sha256(convert_to(e.content, 'UTF8')), octet_length(e.content), b.created_by_id, now()
			FROM e, notebooks b WHERE b.id = $1
		)
		INSERT INTO page_revisions (id, changeset_id, node_id, revision, content, content_hash, byte_size, created_at, updated_at)
		SELECT gen_random_uuid(), c.id, e.id, 1, e.content, sha256(convert_to(e.content, 'UTF8')), octet_length(e.content), now(), now()
		FROM e, changeset c`, bench.nb, (page.MaxContentBytes-200)/32, domain.MaxBatchOps)
	bench.exec(t, `CREATE TABLE kept AS SELECT c.node_id, n.name, c.content, c.content_hash, c.byte_size FROM page_contents c
		JOIN nodes n ON n.id = c.node_id WHERE n.notebook_id = $1 AND (n.name LIKE 'B%' OR n.name LIKE 'D%' OR n.name LIKE 'E%')`, bench.nb)
	upgrade := uuid.NewV7()
	bench.exec(t, `UPDATE notebooks SET upgrade_id = $2, upgrade_state = 'upgrading', upgrade_by_id = $3, upgrade_at = now() WHERE id = $1`,
		bench.nb, upgrade, bench.alice)
	bench.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT $2, n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision, c.content_hash, c.byte_size,
			CASE WHEN n.kind = 'page' THEN 0 END, c.content_hash
		FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL
		WHERE n.notebook_id = $1 AND n.deleted_at IS NULL`, bench.nb, upgrade)
	bench.exec(t, "VACUUM ANALYZE")
	seeded := time.Since(started)

	// written back is the pages of the prefix as the seed wrote them, the
	// ledger with them.
	writtenBack := func(prefix string) {
		bench.exec(t, `UPDATE page_contents c SET content = k.content, content_hash = k.content_hash, byte_size = k.byte_size
			FROM kept k WHERE c.node_id = k.node_id AND k.name LIKE $1 || '%'`, prefix)
		bench.exec(t, `UPDATE upgrade_nodes u SET accepted_hash = c.content_hash FROM page_contents c
			WHERE u.upgrade_id = $1 AND c.node_id = u.node_id`, upgrade)
	}
	versions := func(prefix string, n int) []string {
		got := queryStrings(t, bench.pool, `SELECT n.id::text || ':' || c.revision FROM nodes n JOIN page_contents c ON c.node_id = n.id
			WHERE n.notebook_id = $1 AND n.name LIKE $2 || '%' ORDER BY n.sort_order`, bench.nb, prefix)
		if len(got) != n {
			t.Fatalf("the pages %s…: %d, want %d", prefix, len(got), n)
		}
		return got
	}
	blind := func(prefix string, n int) []map[string]any {
		var ops []map[string]any
		for i, v := range versions(prefix, n) {
			ops = append(ops, opWriteOn(fmt.Sprint(prefix, i, ".md"), "---\ntype: x\n---\nBody.\n", v))
		}
		return ops
	}
	unitOf := func(a mcpAnswer, rewrote int) time.Duration {
		return time.Duration(bench.applied(t, a, rewrote)["unit_ms"].(float64)) * time.Millisecond
	}
	unaccepted := func() int {
		return count(t, bench.pool, `SELECT count(*) FROM upgrade_nodes u JOIN page_contents c ON c.node_id = u.node_id
			WHERE u.upgrade_id = $1 AND u.accepted_hash IS DISTINCT FROM c.content_hash`, upgrade)
	}
	// run is shape's i-th run, its unit's time, or e's call's.
	run := func(shape string, i int, upgrading bool) time.Duration {
		switch shape {
		case "a":
			from, to := "T0.md", "T1.md"
			if i%2 == 1 {
				from, to = to, from
			}
			a, _ := bench.batch(t, opMove(from, to))
			return unitOf(a, domain.MaxBatchRewrites)
		case "b":
			a, _ := bench.batch(t, edits...)
			unit := unitOf(a, 0)
			if got := queryStrings(t, bench.pool, `SELECT content FROM page_contents c JOIN nodes n ON n.id = c.node_id
				WHERE n.notebook_id = $1 AND n.name = 'B0'`, bench.nb); len(got) != 1 || strings.Contains(got[0], "\nk: ") {
				t.Errorf("b: B0 kept its frontmatter's line")
			}
			writtenBack("B")
			return unit
		case "c":
			from, to := "type: a", "type: b"
			if i%2 == 1 {
				from, to = to, from
			}
			a, _ := bench.batch(t, opEdit("C.md", from, to))
			return unitOf(a, 0)
		case "d":
			defer writtenBack("D")
			return unitOf(func() mcpAnswer { a, _ := bench.batch(t, blind("D", 3)...); return a }(), 0)
		}
		before := count(t, bench.pool, `SELECT count(*) FROM page_contents c JOIN kept k ON k.node_id = c.node_id
			WHERE k.name LIKE 'E%' AND c.content_hash = k.content_hash`)
		a, took := bench.batch(t, blind("E", domain.MaxBatchOps)...)
		if !upgrading {
			defer writtenBack("E")
			unitOf(a, 0)
			return took
		}
		if member := a.toolError(t); member["code"] != "page.batch_too_large" || member["op"] != 2.0 || before != domain.MaxBatchOps ||
			count(t, bench.pool, `SELECT count(*) FROM page_contents c JOIN kept k ON k.node_id = c.node_id
				WHERE k.name LIKE 'E%' AND c.content_hash = k.content_hash`) != before {
			t.Errorf("e: 100 blind writes of pages of 5 MiB = %.300s, want page.batch_too_large at ops[2], nothing written", a.body)
		}
		return took
	}

	shapes := []string{"a", "b", "c", "d", "e"}
	runs := map[string]int{} // a's and c's write back by their next
	medians := map[string]any{}
	for _, upgrading := range []bool{true, false} {
		if !upgrading {
			bench.exec(t, `UPDATE notebooks SET upgrade_id = NULL, upgrade_state = NULL, upgrade_by_id = NULL, upgrade_at = NULL
				WHERE id = $1`, bench.nb)
		}
		for _, shape := range shapes {
			var took []time.Duration
			for range 3 {
				d := run(shape, runs[shape], upgrading)
				runs[shape]++
				took = append(took, d)
				if upgrading && shape != "e" && d >= guardrailUnitDeadline {
					t.Errorf("%s: its unit held the notebook %s, want less than %s", shape, d, guardrailUnitDeadline)
				}
			}
			if upgrading {
				if n := unaccepted(); n != 0 {
					t.Errorf("%s: %d pages of the baseline whose content the ledger did not accept", shape, n)
				}
				if last := queryStrings(t, bench.pool, `SELECT coalesce(upgrade_id::text, '') FROM changesets WHERE notebook_id = $1
					ORDER BY created_at DESC, id DESC LIMIT 1`, bench.nb); shape != "e" && (len(last) != 1 || last[0] != upgrade.String()) {
					t.Errorf("%s: the last changeset is of the upgrade %v, want %s: the initiator's agent's, past the freeze", shape, last,
						upgrade)
				}
			}
			slices.Sort(took)
			key := shape + ", upgrading"
			if !upgrading {
				key = shape + ", not upgraded"
			}
			medians[key] = took[1].Milliseconds()
			if bound := guardrailUnitBounds()[shape]; upgrading && took[1] > bound {
				t.Errorf("%s: the median of three took %s, past %s", shape, took[1], bound)
			}
		}
	}
	b, err := json.Marshal(map[string]any{"seed_ms": seeded.Milliseconds(), "medians_ms": medians})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("the guardrails' units %s", b)
}

// exec runs a statement of the test's own on the bench's database.
func (b batchBench) exec(tb testing.TB, sql string, args ...any) {
	tb.Helper()
	if _, err := b.pool.Exec(context.Background(), sql, args...); err != nil {
		tb.Fatalf("%s: %v", sql, err)
	}
}
