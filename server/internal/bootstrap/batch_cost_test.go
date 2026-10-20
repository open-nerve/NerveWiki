//go:build !race

package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// What a batch's deferred rewrite costs, by the pages it writes (M9/P3
// design 3.8.2): the measurement MaxBatchRewrites is taken from, run by
// hand,
//
//	go test -run '^$' -bench 'BenchmarkBatchRewrites' -benchtime 3x ./internal/bootstrap/
//
// and the guard that holds a batch at the bound to its time. The bench is
// the trash bench's, the MCP server mounted on its router, alice's calls
// in process with a personal access token of hers (the page module hands
// the use case to its MCP tool alone); its page module logs to a buffer,
// whose "batch applied" line tells the unit's time (unit_ms), timed
// around Writer.Run: the batch's own reads and plan, and the MCP tool's
// decoding and answer, are outside it.

// batchBench is the trash bench with MCP's tools on its router, the page
// module's lines in logs.
type batchBench struct {
	trashBench
	logs *syncBuffer
}

// newBatchBench is the bench, the batch's deferred rewrite bounded at
// maxRewrites pages, maxLinks links and maxBytes bytes of its pages, 0 the
// composition root's.
func newBatchBench(tb testing.TB, maxRewrites, maxLinks, maxBytes int) batchBench {
	tb.Helper()
	return newBatchBenchOf(tb, func(*config.Config) {}, maxRewrites, maxLinks, maxBytes)
}

// newBatchBenchOf is newBatchBench, its configuration as configure leaves
// the trash bench's.
func newBatchBenchOf(tb testing.TB, configure func(c *config.Config), maxRewrites, maxLinks, maxBytes int) batchBench {
	tb.Helper()
	logs := &syncBuffer{}
	b := newTrashBenchOf(tb, configure, func(d *page.Deps) {
		d.Logger = slog.New(slog.NewJSONHandler(logs, nil))
		d.MaxBatchRewrites, d.MaxBatchRewriteLinks, d.MaxBatchRewriteBytes = maxRewrites, maxLinks, maxBytes
	})
	mountMCP(b.mux, b.api, mcpServer(b.cfg, slog.New(slog.DiscardHandler), b.api, b.pg, benchNames{},
		b.pg.Tools(page.ToolDeps{Attachments: benchAttachments{}})))
	return batchBench{b, logs}
}

// benchNames remembers no client's name.
type benchNames struct{}

func (benchNames) Clean(name string) string { return shared.CleanClientName(name) }

func (benchNames) Recorded(context.Context, uuid.UUID) (string, error) { return "", nil }

func (benchNames) Record(context.Context, uuid.UUID, string) (bool, error) { return false, nil }

// benchAttachments reads no attachment.
type benchAttachments struct{}

func (benchAttachments) Read(context.Context, uuid.UUID) (page.Attachment, error) {
	return page.Attachment{}, page.ErrNotFound
}

func (benchAttachments) Content(context.Context, page.Attachment, int64) ([]byte, int, int, error) {
	return nil, 0, 0, page.ErrNotFound
}

// rewritten makes, through SQL, the notebook's page T0, the pages P1 and P2
// a batch edits, of "[[T0]] p1 0" and "p2 0", and n pages R0 … of the full
// load (trash_cost_test.go's 84 KB and 200 links, M8's), their first link
// to T0; then rebuilds the notebook's index, which parses each once. P1,
// written before T0's rename, is one of the pages the rename's rewrite
// counts (M9/P3 design 3.7.2: a written page whose own changes move
// something) and writes: a batch of renamed's rewrites n+1 pages.
func (b batchBench) rewritten(tb testing.TB, from, n int) {
	tb.Helper()
	var pages []sqlPage
	if from == 0 {
		pages = []sqlPage{{"", "T0", ""}, {"", "P1", "[[T0]] p1 0"}, {"", "P2", "p2 0"}}
	}
	for i := from; i < from+n; i++ {
		_, rest, _ := strings.Cut(benchLoads()[1].content(i), "\n")
		pages = append(pages, sqlPage{"", fmt.Sprint("R", i), "[[T0]]\n" + rest})
	}
	seedPagesSQL(tb, b.pool, b.nb.String(), pages)
	admin, err := reindexAdmin(b.pool, b.cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := admin.Rebuild(context.Background(), b.nb); err != nil {
		tb.Fatal(err)
	}
}

// batch is alice's batch of ops in the bench's notebook, through MCP's
// tool in process, and how long the call took.
func (b batchBench) batch(tb testing.TB, ops ...map[string]any) (mcpAnswer, time.Duration) {
	tb.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call",
		"params": map[string]any{"name": "batch", "arguments": batched(ops...)}})
	if err != nil {
		tb.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, atNotebook(b.nb.String()), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer pat")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", mcpVersion)
	rec := httptest.NewRecorder()
	at := time.Now()
	b.router.ServeHTTP(rec, req)
	return mcpAnswer{status: rec.Code, header: rec.Header(), body: rec.Body.Bytes(), tool: "batch"}, time.Since(at)
}

// renamed is the batch's ops of its n-th round: P1 and P2 edited, and T
// renamed between them, from Tn to Tn+1.
func renamed(n int) []map[string]any {
	return []map[string]any{opEdit("P1.md", fmt.Sprint("p1 ", n), fmt.Sprint("p1 ", n+1)), opMove(fmt.Sprintf("T%d.md", n),
		fmt.Sprintf("T%d.md", n+1)), opEdit("P2.md", fmt.Sprint("p2 ", n), fmt.Sprint("p2 ", n+1))}
}

// applied is the line of the last batch done: failing tb unless a is a
// result and its deferred rewrite wrote rewrote pages.
func (b batchBench) applied(tb testing.TB, a mcpAnswer, rewrote int) map[string]any {
	tb.Helper()
	if a.result(tb)["isError"] == true {
		tb.Fatalf("the batch answered %.500s", a.body)
	}
	line := (&mcpApp{logs: b.logs}).loggedLast(tb, "batch applied")
	if line == nil || line["rewritten"] != float64(rewrote) {
		tb.Fatalf("the batch's line = %v, want %d pages rewritten", line, rewrote)
	}
	return line
}

// mixed makes, through SQL, the worst mix of a batch at its rewrites' bound
// (M9/P3 review B-M3): the folder Top/F of X0 … X{folder-1}, G, O0 … O99,
// and written pages P0 … and indexed ones R… of some 76 KB, each with 200
// links, its first 100 into distinct X by their full paths, the rest to
// the O; then rebuilds the notebook's index. Its ops edit each P, then
// move F under G, with update_links: the rewrite writes every P and R.
func (b batchBench) mixed(tb testing.TB, written, indexed, folder int) []map[string]any {
	tb.Helper()
	nb := b.nb.String()
	id := func(name string) string {
		return queryStrings(tb, b.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = $2", nb, name)[0]
	}
	seedPagesSQL(tb, b.pool, nb, []sqlPage{{"", "Top", ""}, {"", "G", ""}})
	seedPagesSQL(tb, b.pool, nb, []sqlPage{{id("Top"), "F", ""}})
	f := id("F")
	var xs, others, pages []sqlPage
	for k := range folder {
		xs = append(xs, sqlPage{f, fmt.Sprint("X", k), ""})
	}
	for k := range 100 {
		others = append(others, sqlPage{"", fmt.Sprint("O", k), ""})
	}
	page := func(name string, p int) sqlPage {
		var c strings.Builder
		c.WriteString("MARK\n")
		for k := range 200 {
			target := fmt.Sprintf("O%d", k%100)
			if k < 100 {
				target = fmt.Sprintf("Top/F/X%d", (p*100+k)%folder)
			}
			fmt.Fprintf(&c, "Line %d of page %d: %s [[%s]]\n", k, p, strings.Repeat("lorem ipsum dolor sit amet ", 13), target)
		}
		return sqlPage{"", name, c.String()}
	}
	var ops []map[string]any
	for p := range written {
		pages = append(pages, page(fmt.Sprint("P", p), p))
		ops = append(ops, opEdit(fmt.Sprint("P", p, ".md"), "MARK", "MARK2"))
	}
	for p := range indexed {
		pages = append(pages, page(fmt.Sprint("R", written+p), written+p))
	}
	seedPagesSQL(tb, b.pool, nb, xs)
	seedPagesSQL(tb, b.pool, nb, others)
	seedPagesSQL(tb, b.pool, nb, pages)
	admin, err := reindexAdmin(b.pool, b.cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := admin.Rebuild(context.Background(), b.nb); err != nil {
		tb.Fatal(err)
	}
	return append(ops, opMove("Top/F.md", "G/F.md"))
}

// dense makes, through SQL, the dense links' shape (M9/P3 review B1-M3):
// the folder Top/F of X0 … X{folder-1}, G, and pages R0 … of links lines
// each, a bare link alone, into distinct X by their full paths, some 18
// bytes a link, the first extra pages a line more, to Top/F/Extra; then
// rebuilds the notebook's index. Its op moves F under G, with
// update_links: the rewrite writes every link of every R.
func (b batchBench) dense(tb testing.TB, pages, links, folder, extra int) []map[string]any {
	tb.Helper()
	nb := b.nb.String()
	seedPagesSQL(tb, b.pool, nb, []sqlPage{{"", "Top", ""}, {"", "G", ""}})
	f := queryStrings(tb, b.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = 'Top'", nb)[0]
	seedPagesSQL(tb, b.pool, nb, []sqlPage{{f, "F", ""}})
	f = queryStrings(tb, b.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = 'F'", nb)[0]
	var xs, rs []sqlPage
	for k := range folder {
		xs = append(xs, sqlPage{f, fmt.Sprint("X", k), ""})
	}
	if extra > 0 {
		xs = append(xs, sqlPage{f, "Extra", ""})
	}
	for p := range pages {
		var c strings.Builder
		for k := range links {
			fmt.Fprintf(&c, "[[Top/F/X%d]]\n", (p*links+k)%folder)
		}
		if p < extra {
			c.WriteString("[[Top/F/Extra]]\n")
		}
		rs = append(rs, sqlPage{"", fmt.Sprint("R", p), c.String()})
	}
	seedPagesSQL(tb, b.pool, nb, xs)
	seedPagesSQL(tb, b.pool, nb, rs)
	admin, err := reindexAdmin(b.pool, b.cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := admin.Rebuild(context.Background(), b.nb); err != nil {
		tb.Fatal(err)
	}
	return []map[string]any{opMove("Top/F.md", "G/F.md")}
}

// heavy makes, through SQL, the heavy pages' shape (M9/P3 fix check 2,
// B2-M1): the folder Top/F of X0 … X{folder-1}, G, and pages R0 … of
// sizes, in their order, each a link into a distinct X by its full path,
// then plain text; then rebuilds the notebook's index. Its op moves F
// under G, with update_links: the rewrite writes every R, one link each,
// in their order, and keeps the facts of each to the unit's end, some
// tenth of its bytes.
func (b batchBench) heavy(tb testing.TB, sizes []int, folder int) []map[string]any {
	tb.Helper()
	nb := b.nb.String()
	seedPagesSQL(tb, b.pool, nb, []sqlPage{{"", "Top", ""}, {"", "G", ""}})
	f := queryStrings(tb, b.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = 'Top'", nb)[0]
	seedPagesSQL(tb, b.pool, nb, []sqlPage{{f, "F", ""}})
	f = queryStrings(tb, b.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = 'F'", nb)[0]
	var xs []sqlPage
	for k := range folder {
		xs = append(xs, sqlPage{f, fmt.Sprint("X", k), ""})
	}
	seedPagesSQL(tb, b.pool, nb, xs)
	const line = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore.\n"
	for from := 0; from < len(sizes); from += 50 {
		var rs []sqlPage
		for p := from; p < min(from+50, len(sizes)); p++ {
			head := fmt.Sprintf("[[Top/F/X%d]]\n\n", p%folder)
			text := head + strings.Repeat(line, (sizes[p]-len(head))/len(line))
			rs = append(rs, sqlPage{"", fmt.Sprint("R", p), text + strings.Repeat("x", sizes[p]-len(text))})
		}
		seedPagesSQL(tb, b.pool, nb, rs)
	}
	admin, err := reindexAdmin(b.pool, b.cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := admin.Rebuild(context.Background(), b.nb); err != nil {
		tb.Fatal(err)
	}
	return []map[string]any{opMove("Top/F.md", "G/F.md")}
}

// longPaths makes, through SQL, the long paths' shape (M9/P3 fix check 3,
// B3-M1): the folder Top/F of X0 … X999, a chain of seven folders of
// 250-byte names, and MaxBatchRewrites pages R0 … of links relative
// Markdown links each, "[r](./Top/F/Xk.md)", into distinct Xs; then
// rebuilds the notebook's index. Its op moves F to the chain's end, with
// update_links: each link's writing grows by the chain's path, some 1,760
// bytes, and a page's facts' share of the parse budget with it.
func (b batchBench) longPaths(tb testing.TB, links int) []map[string]any {
	tb.Helper()
	nb := b.nb.String()
	id := func(name string) string {
		return queryStrings(tb, b.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = $2", nb, name)[0]
	}
	seedPagesSQL(tb, b.pool, nb, []sqlPage{{"", "Top", ""}})
	seedPagesSQL(tb, b.pool, nb, []sqlPage{{id("Top"), "F", ""}})
	var chain []string
	parent := ""
	for d := range 7 {
		name := fmt.Sprint("D", d) + strings.Repeat("x", 248)
		seedPagesSQL(tb, b.pool, nb, []sqlPage{{parent, name, ""}})
		parent = id(name)
		chain = append(chain, name)
	}
	f := id("F")
	var xs, rs []sqlPage
	for k := range 1000 {
		xs = append(xs, sqlPage{f, fmt.Sprint("X", k), ""})
	}
	for p := range domain.MaxBatchRewrites {
		var c strings.Builder
		for k := range links {
			fmt.Fprintf(&c, "[r](./Top/F/X%d.md)\n", (p*links+k)%1000)
		}
		rs = append(rs, sqlPage{"", fmt.Sprint("R", p), c.String()})
	}
	seedPagesSQL(tb, b.pool, nb, xs)
	seedPagesSQL(tb, b.pool, nb, rs)
	admin, err := reindexAdmin(b.pool, b.cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := admin.Rebuild(context.Background(), b.nb); err != nil {
		tb.Fatal(err)
	}
	return []map[string]any{opMove("Top/F.md", strings.Join(chain, "/")+"/F.md")}
}

// spread is n sizes of total bytes in all, as even as they go, the first
// the larger.
func spread(total, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = total / n
		if i < total%n {
			out[i]++
		}
	}
	return out
}

// heaviestLast is 400 sizes of total bytes in all: a page's largest last,
// the rest as even as they go.
func heaviestLast(total int) []int {
	return append(spread(total-domain.MaxContentBytes, domain.MaxBatchRewrites-1), domain.MaxContentBytes)
}

// BenchmarkBatchRewrites times a batch that rewrites n pages, for each tier
// n (M9/P3 design 3.8.2): two edits and a rename of the page they link to,
// between them, n-1 pages of the full load and P1 rewritten; each
// iteration after a vacuum, untimed. Then the worst mix at the bound
// (mixed; M9/P3 review B-M3): 99 pages written, the most a batch's 100 ops
// write with the move, each counting WrittenRewriteWeight, and the 202 of
// the index that fill the bound, the move of a folder of 10,000 after the
// edits; and the written pages alone and 400 of the index alone, whose
// costs a page are the ratio a written page counts for; each iteration on
// a bench of its own, made untimed. Then the dense links' tiers (dense;
// M9/P3 review B1-M3), 400 pages of the index of 100 to 2,500 bare links
// each into a folder of 10,000 moved alone, the measurement
// MaxBatchRewriteLinks is taken from, each iteration on a bench of its
// own; a tier past the request's deadline is logged and skipped. Then the
// bytes' tiers (heavy; M9/P3 fix round 5, B2-M1), 400 pages of the index
// of 76 KiB to 1 MiB each, one link each into a folder of 10,000 moved
// alone, the measurement MaxBatchRewriteBytes is taken from, and two of
// its pages' bytes with a page's largest rewritten last (heaviestLast),
// the bound's, and that of the 96 KiB tier, past what the parse budget
// keeps room for; each iteration on a bench of its own; a tier that
// answers otherwise than done, server_busy past the parse budget, is
// logged and skipped. It reports the median of the unit and of the call,
// in milliseconds. A tier's bench is made in its run, so that -bench
// names the tiers to run.
func BenchmarkBatchRewrites(b *testing.B) {
	for _, n := range []int{100, 200, 250, 300, 400, 500, 1000} {
		var bench batchBench
		round := 0
		b.Run(fmt.Sprint(n, " pages"), func(b *testing.B) {
			if bench.logs == nil {
				b.StopTimer()
				bench = newBatchBench(b, 1<<20, 1<<30, 1<<40)
				bench.item(b, benchLoads()[1], 0)
				bench.rewritten(b, 0, n-1)
				b.StartTimer()
			}
			var units, calls []float64
			for range b.N {
				b.StopTimer()
				bench.vacuumed(b)
				b.StartTimer()
				a, took := bench.batch(b, renamed(round)...)
				b.StopTimer()
				round++
				units = append(units, bench.applied(b, a, n)["unit_ms"].(float64))
				calls = append(calls, float64(took)/float64(time.Millisecond))
				b.StartTimer()
			}
			b.ReportMetric(median(units), "unit-ms")
			b.ReportMetric(median(calls), "call-ms")
		})
	}
	const written = 99
	mixes := []struct{ written, indexed int }{
		{written, domain.MaxBatchRewrites - written*domain.WrittenRewriteWeight}, {written, 0}, {0, domain.MaxBatchRewrites},
	}
	for _, m := range mixes {
		b.Run(fmt.Sprintf("mixed: %d written, %d indexed, a folder of 10,000 moved", m.written, m.indexed), func(b *testing.B) {
			var units, calls []float64
			for range b.N {
				b.StopTimer()
				bench := newBatchBench(b, 1<<20, 1<<30, 1<<40)
				ops := bench.mixed(b, m.written, m.indexed, 10000)
				bench.vacuumed(b)
				b.StartTimer()
				a, took := bench.batch(b, ops...)
				b.StopTimer()
				units = append(units, bench.applied(b, a, m.written+m.indexed)["unit_ms"].(float64))
				calls = append(calls, float64(took)/float64(time.Millisecond))
				b.StartTimer()
			}
			b.ReportMetric(median(units), "unit-ms")
			b.ReportMetric(median(calls), "call-ms")
		})
	}
	for _, links := range []int{100, 250, 300, 375, 500, 750, 1000, 1500, 2000, 2500} {
		b.Run(fmt.Sprintf("dense: %d indexed of %d links each, %d links, a folder of 10,000 moved", domain.MaxBatchRewrites, links,
			domain.MaxBatchRewrites*links), func(b *testing.B) {
			var units, calls []float64
			for range b.N {
				b.StopTimer()
				bench := newBatchBench(b, 1<<20, 1<<30, 1<<40)
				ops := bench.dense(b, domain.MaxBatchRewrites, links, 10000, 0)
				bench.vacuumed(b)
				b.StartTimer()
				a, took := bench.batch(b, ops...)
				b.StopTimer()
				if a.status != http.StatusOK || a.result(b)["isError"] == true {
					b.Logf("%d links each: the batch answered %d in %s %.300s", links, a.status, took, a.body)
					b.SkipNow()
				}
				units = append(units, bench.applied(b, a, domain.MaxBatchRewrites)["unit_ms"].(float64))
				calls = append(calls, float64(took)/float64(time.Millisecond))
				b.StartTimer()
			}
			b.ReportMetric(median(units), "unit-ms")
			b.ReportMetric(median(calls), "call-ms")
		})
	}
	type heavyTier struct {
		name  string
		sizes []int
	}
	var heavies []heavyTier
	for _, kib := range []int{76, 84, 96, 112, 128, 160, 192, 256, 512, 1024} {
		heavies = append(heavies, heavyTier{fmt.Sprintf("bytes: %d indexed of %d KiB, one link each, %d KiB, a folder of 10,000 moved",
			domain.MaxBatchRewrites, kib, domain.MaxBatchRewrites*kib), spread(domain.MaxBatchRewrites*kib<<10, domain.MaxBatchRewrites)})
	}
	for _, total := range []int{domain.MaxBatchRewriteBytes, domain.MaxBatchRewrites * 96 << 10} {
		heavies = append(heavies, heavyTier{fmt.Sprintf("bytes, a page's largest last: %d indexed of %d bytes in all, one link each, "+
			"a folder of 10,000 moved", domain.MaxBatchRewrites, total), heaviestLast(total)})
	}
	for _, tier := range heavies {
		b.Run(tier.name, func(b *testing.B) {
			var units, calls []float64
			for range b.N {
				b.StopTimer()
				bench := newBatchBench(b, 1<<20, 1<<30, 1<<40)
				ops := bench.heavy(b, tier.sizes, 10000)
				bench.vacuumed(b)
				b.StartTimer()
				a, took := bench.batch(b, ops...)
				b.StopTimer()
				if a.status != http.StatusOK || a.result(b)["isError"] == true {
					b.Logf("%s: the batch answered %d in %s %.300s", tier.name, a.status, took, a.body)
					b.SkipNow()
				}
				units = append(units, bench.applied(b, a, domain.MaxBatchRewrites)["unit_ms"].(float64))
				calls = append(calls, float64(took)/float64(time.Millisecond))
				b.StartTimer()
			}
			b.ReportMetric(median(units), "unit-ms")
			b.ReportMetric(median(calls), "call-ms")
		})
	}
}

// median is the middle of xs, the lower of the two middles of an even
// count.
func median(xs []float64) float64 {
	s := slices.Sorted(slices.Values(xs))
	return s[(len(s)-1)/2]
}

// A batch at the bound of its deferred rewrite takes its time (M9/P3 design
// 3.8.2): pages of the full load, each with a link to T0, which a batch
// renames between two edits, the first of P1, which links to T0 too: the
// rename's rewrite counts P1 with them as WrittenRewriteWeight pages, one
// past the bound, page.batch_too_large at the rename saying so, counted
// before any page is parsed: T0, the pages and the changesets as they
// were. Once one of them is deleted, the batch rewrites the bound's pages,
// P1 among them, its unit in 11 s at most, three times the 3.56 s
// BenchmarkBatchRewrites measured at 400 pages (MaxBatchRewrites' comment;
// the time logged for CI's runs: make test-go runs it with -v). The bench
// keeps the composition root's bound, which the module hands the batch:
// one far from the domain's fails here.
func TestABatchAtItsRewriteBoundTakesItsTime(t *testing.T) {
	if testing.Short() {
		t.Skip("a bound of time against a database")
	}
	// The R pages and P1, written, past the bound by one (M9/P3 review B-M3).
	r := domain.MaxBatchRewrites - domain.WrittenRewriteWeight + 1
	bench := newBatchBench(t, 0, 0, 0)
	bench.rewritten(t, 0, r)
	state := func() string {
		return strings.Join(queryStrings(t, bench.pool, `SELECT n.name || ' ' || md5(c.content) FROM nodes n
			JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL WHERE n.deleted_at IS NULL ORDER BY n.name`), "\n") +
			fmt.Sprint(count(t, bench.pool, "SELECT count(*) FROM changesets"))
	}
	was := state()
	a, _ := bench.batch(t, renamed(0)...)
	past := fmt.Sprintf("%d pages whose links the batch's renames and moves would rewrite, each page the batch wrote counting %d, "+
		"at most %d", domain.MaxBatchRewrites+1, domain.WrittenRewriteWeight, domain.MaxBatchRewrites)
	if member := a.toolError(t); member["code"] != "page.batch_too_large" || member["op"] != 1.0 ||
		!strings.Contains(fmt.Sprint(member["message"]), past) || state() != was {
		t.Errorf("the batch counting %d pages = %.400s, want page.batch_too_large at ops[1] saying %q, nothing written",
			domain.MaxBatchRewrites+1, a.body, past)
	}

	extra := queryStrings(t, bench.pool, "SELECT id::text FROM nodes WHERE name = $1", fmt.Sprint("R", r-1))[0]
	bench.timed(t, http.MethodDelete, "/api/v0/nodes/"+extra, http.StatusNoContent)
	bench.vacuumed(t)
	a, took := bench.batch(t, renamed(0)...)
	unit := time.Duration(bench.applied(t, a, r)["unit_ms"].(float64)) * time.Millisecond
	t.Logf("a batch at its rewrites' bound, %d pages rewritten: its unit took %s, the call %s", r, unit, took)
	if unit > 11*time.Second {
		t.Errorf("a batch at its rewrites' bound took %s in its unit, want 11 s at most", unit)
	}
	if got := queryStrings(t, bench.pool, `SELECT count(*) || ' ' || count(*) FILTER (WHERE c.revision = 2)
		FROM nodes n JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL
		WHERE n.deleted_at IS NULL AND n.name LIKE 'R%' AND c.content LIKE '[[T1]]%'`); got[0] != fmt.Sprintf("%d %d", r-1, r-1) {
		t.Errorf("the R pages linking to T1 at their second revision: %s, want all %d", got[0], r-1)
	}
	if p1 := queryStrings(t, bench.pool, `SELECT c.content || ' ' || c.revision FROM nodes n JOIN page_contents c ON c.node_id = n.id
		WHERE n.name = 'P1'`); p1[0] != "[[T1]] p1 1 3" {
		t.Errorf("P1 = %q, want its edit, then its link rewritten, at 3", p1[0])
	}
	checkPages(t, bench.pool)
}

// A batch's rewrite counts the links it would write against
// MaxBatchRewriteLinks before any page is parsed (M9/P3 review B1-M3): 400
// pages of the index of 300 bare links each into a folder of 10,000 that a
// batch moves, and one link more, to F/Extra, one past the bound:
// page.batch_too_large at the move saying so, the pages and the
// changesets as they were. With F/Extra deleted first, its link left
// uncounted, the batch at the bound rewrites the 400 pages. The bench
// keeps the composition root's bounds, which the module hands the batch.
// Not a guard of time: the unit's is logged (make test-go runs it with -v
// beside the pages' guard).
func TestABatchPastItsLinksBoundIsRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("a bound against a database")
	}
	bench := newBatchBench(t, 0, 0, 0)
	per := domain.MaxBatchRewriteLinks / domain.MaxBatchRewrites
	ops := bench.dense(t, domain.MaxBatchRewrites, per, 10000, 1)
	state := func() string {
		return strings.Join(queryStrings(t, bench.pool, `SELECT n.name || ' ' || md5(c.content) FROM nodes n
			JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL WHERE n.deleted_at IS NULL ORDER BY n.name`), "\n") +
			fmt.Sprint(count(t, bench.pool, "SELECT count(*) FROM changesets"))
	}
	was := state()
	a, _ := bench.batch(t, ops...)
	past := fmt.Sprintf("%d links the batch's renames and moves would rewrite, at most %d: move with update_links false, or in "+
		"separate calls", domain.MaxBatchRewriteLinks+1, domain.MaxBatchRewriteLinks)
	if member := a.toolError(t); member["code"] != "page.batch_too_large" || member["op"] != 0.0 ||
		!strings.Contains(fmt.Sprint(member["message"]), past) || state() != was {
		t.Errorf("the batch of %d links = %.400s, want page.batch_too_large at ops[0] saying %q, nothing written",
			domain.MaxBatchRewriteLinks+1, a.body, past)
	}
	bench.vacuumed(t)
	a, took := bench.batch(t, append([]map[string]any{opDelete("Top/F/Extra.md")}, ops...)...)
	line := bench.applied(t, a, domain.MaxBatchRewrites)
	t.Logf("a batch at its links' bound, %d links of %d pages rewritten: its unit took %v ms, the call %s",
		domain.MaxBatchRewriteLinks, domain.MaxBatchRewrites, line["unit_ms"], took)
	checkPages(t, bench.pool)
}

// A batch's rewrite counts the bytes of the pages it would write against
// MaxBatchRewriteBytes before any page is parsed (M9/P3 fix round 5,
// B2-M1: the facts of each page it writes keep some tenth of its bytes of
// the parse budget to the unit's end): 400 pages of the index of
// MaxBatchRewriteBytes in all and a byte more, each of one link into a
// folder of 10,000 that a batch moves, the last of a page's largest
// content: page.batch_too_large at the move saying so, the pages and the
// changesets as they were. Once a batch has edited R0 a byte shorter, the
// batch at the bound rewrites the 400 pages, the last parsed beside the
// shares the 399 before it keep, within the configuration's parse budget.
// The bench keeps the composition root's bounds, which the module hands
// the batch. Not a guard of time: the unit's is logged (make test-go runs
// it with -v beside the pages' guard).
func TestABatchPastItsBytesBoundIsRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("a bound against a database")
	}
	bench := newBatchBench(t, 0, 0, 0)
	ops := bench.heavy(t, heaviestLast(domain.MaxBatchRewriteBytes+1), 10000)
	state := func() string {
		return strings.Join(queryStrings(t, bench.pool, `SELECT n.name || ' ' || md5(c.content) FROM nodes n
			JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL WHERE n.deleted_at IS NULL ORDER BY n.name`), "\n") +
			fmt.Sprint(count(t, bench.pool, "SELECT count(*) FROM changesets"))
	}
	was := state()
	a, took := bench.batch(t, ops...)
	past := fmt.Sprintf("%d bytes of pages the batch's renames and moves would rewrite, at most %d: move with update_links false, "+
		"or in separate calls", domain.MaxBatchRewriteBytes+1, domain.MaxBatchRewriteBytes)
	if member := a.toolError(t); member["code"] != "page.batch_too_large" || member["op"] != 0.0 ||
		!strings.Contains(fmt.Sprint(member["message"]), past) || state() != was {
		t.Errorf("the batch of %d bytes = %.400s, want page.batch_too_large at ops[0] saying %q, nothing written",
			domain.MaxBatchRewriteBytes+1, a.body, past)
	}
	t.Logf("a batch past its bytes' bound by one: refused in %s", took)
	bench.applied(t, func() mcpAnswer {
		a, _ := bench.batch(t, opEdit("R0.md", "[[Top/F/X0]]\n\n", "[[Top/F/X0]]\n"))
		return a
	}(), 0)
	bench.vacuumed(t)
	a, took = bench.batch(t, ops...)
	line := bench.applied(t, a, domain.MaxBatchRewrites)
	t.Logf("a batch at its bytes' bound, %d bytes of %d pages rewritten: its unit took %v ms, the call %s",
		domain.MaxBatchRewriteBytes, domain.MaxBatchRewrites, line["unit_ms"], took)
	checkPages(t, bench.pool)
}

// A batch's rewrite whose pages keep more of the parse budget than it has,
// within its three bounds, is refused at the parse that would not fit
// beside their shares, at once (M9/P3 fix round 6, B3-M1): 400 pages of the
// index of 150 relative links each, into a folder that the batch moves
// under seven folders of 250-byte names, each link written longer by the
// path, page.batch_too_large at the move saying so, the pages and the
// changesets as they were. The budget's wait is a minute, past the
// request's 15 s: a refusal that waited first would come at the request's
// end. With half the budget held by another, the same batch is
// server_busy at once: others' shares stand in its way, not its own. With
// a budget of 16 MiB it rewrites the 400 pages. The bench keeps the
// composition root's bounds, which the module hands the batch. Not a guard
// of time: the unit's is logged (make test-go runs it with -v beside the
// pages' guard).
func TestABatchWhosePagesOutgrowTheParseBudgetIsRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("a bound against a database")
	}
	bench := newBatchBenchOf(t, func(c *config.Config) { c.Page.ParseMaxWait = time.Minute }, 0, 0, 0)
	ops := bench.longPaths(t, 150)
	state := func() string {
		return strings.Join(queryStrings(t, bench.pool, `SELECT n.name || ' ' || md5(c.content) FROM nodes n
			JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL WHERE n.deleted_at IS NULL ORDER BY n.name`), "\n") +
			fmt.Sprint(count(t, bench.pool, "SELECT count(*) FROM changesets"))
	}
	was := state()
	bench.vacuumed(t)
	a, took := bench.batch(t, ops...)
	past := fmt.Sprintf("bytes of the parse budget the pages the batch's renames and moves rewrite would hold, at most %d: "+
		"move with update_links false, or in separate calls", bench.budget.Size())
	if member := a.toolError(t); member["code"] != "page.batch_too_large" || member["op"] != 0.0 ||
		!strings.Contains(fmt.Sprint(member["message"]), past) || state() != was || took >= bench.cfg.Server.RequestTimeout {
		t.Errorf("the batch of long paths = %.400s in %s, want page.batch_too_large at ops[0] saying %q, at once, nothing written",
			a.body, took, past)
	}
	t.Logf("a batch whose pages outgrow the parse budget: refused in %s", took)

	held, err := bench.budget.TakeNow(context.Background(), bench.budget.Size()/2)
	if err != nil {
		t.Fatal(err)
	}
	a, took = bench.batch(t, ops...)
	held.Release()
	if member := a.toolError(t); member["code"] != "server_busy" || member["op"] != 0.0 || state() != was ||
		took >= bench.cfg.Server.RequestTimeout {
		t.Errorf("the batch beside another's half of the budget = %.400s in %s, want server_busy at ops[0] at once, nothing written",
			a.body, took)
	}
	t.Logf("the same batch beside another's half of the budget: server_busy in %s", took)

	large := newBatchBenchOf(t, func(c *config.Config) { c.Page.ParseBudgetBytes = 16 << 20 }, 0, 0, 0)
	ops = large.longPaths(t, 150)
	large.vacuumed(t)
	a, took = large.batch(t, ops...)
	line := large.applied(t, a, domain.MaxBatchRewrites)
	t.Logf("the same batch in a budget of 16 MiB, %d pages rewritten: its unit took %v ms, the call %s",
		domain.MaxBatchRewrites, line["unit_ms"], took)
	checkPages(t, large.pool)
}
