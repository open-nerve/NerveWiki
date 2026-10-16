//go:build !race

package bootstrap

import (
	"cmp"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	pagepostgres "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	pageapp "github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// What a background revert and restore cost (M8/P4 design 3.11): the
// measurements MaxJobItems is taken from, run by hand through heavy.sh,
//
//	NW_JOB_SIZES=60000,70000 go test -run '^$' -bench 'BenchmarkStartRevert|BenchmarkBackground' -benchtime 1x \
//		-timeout 6h ./internal/bootstrap/
//
// and the guards that hold a job at the bound to its time. The bench is
// the trash's (trash_cost_test.go): the page module wired as serve wires
// it, the bound raised for the measurement; a job's batches cut as the
// transfer module's run cuts them, with its bounds.

// The shapes of an import's changeset the measurements revert: its pages
// at the root; under one top, five levels below it; in folders, a page of
// each six at the root and the five after it under it.
const (
	shapeFlat    = "flat"
	shapeDeep    = "deep"
	shapeFolders = "folders"
)

// shapeGroups is the revert of the revert of an import of attachments at
// the root, which restores a group of each (revertOfAttachments): many
// groups, a request about as long as the revert's revert of one group of
// every node (BenchmarkBackgroundSteps).
const shapeGroups = "groups"

// The load of an imported changeset's pages, the design's full load
// MaxJobItems is measured at (M8/P4 design 3.11, fixed in its review's fix
// round): a content of 2 KiB, which its row keeps out of line, with
// jobLinks links to the import's other pages, resolved; and an attachment
// under a page of each nine, a tenth of its nodes.
const jobContent = 2 << 10

// jobLinks are the links of each imported page: NW_JOB_LINKS, or 20, the
// full load's.
func jobLinks(tb testing.TB) int {
	tb.Helper()
	n, err := strconv.Atoi(cmp.Or(os.Getenv("NW_JOB_LINKS"), "20"))
	if err != nil || n < 1 || n*12 > jobContent {
		tb.Fatalf("NW_JOB_LINKS %q: from 1 to %d", os.Getenv("NW_JOB_LINKS"), jobContent/12)
	}
	return n
}

// jobSizes are the sizes measured: NW_JOB_SIZES, or 50,000, 60,000 and
// 70,000.
func jobSizes(tb testing.TB) []int {
	tb.Helper()
	list := os.Getenv("NW_JOB_SIZES")
	if list == "" {
		return []int{50_000, 60_000, 70_000}
	}
	var out []int
	for s := range strings.SplitSeq(list, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			tb.Fatalf("NW_JOB_SIZES %q: %v", list, err)
		}
		out = append(out, n)
	}
	return out
}

// newJobBench is the bench, a background revert's changeset bounded at
// maxItems, 0 the composition root's.
func newJobBench(tb testing.TB, maxItems int) trashBench {
	tb.Helper()
	return newTrashBenchWith(tb, func(d *page.Deps) {
		if maxItems > 0 {
			d.MaxJobItems = maxItems
		}
	})
}

// exec runs the statement with args, failing tb on its error.
func (b trashBench) exec(tb testing.TB, sql string, args ...any) {
	tb.Helper()
	if _, err := b.pool.Exec(context.Background(), sql, args...); err != nil {
		tb.Fatalf("%s: %v", sql, err)
	}
}

// fanout is the children of each page of the deep shape of n pages: a
// top and five levels below it.
func fanout(n int) int {
	return int(math.Ceil(math.Pow(float64(n), 1.0/5)))
}

// md5ID is the id the bench's SQL gives the node of key in the notebook
// nb: md5 of the notebook and key.
func md5ID(nb uuid.UUID, key string) uuid.UUID {
	return uuid.UUID(md5.Sum([]byte(nb.String() + ":" + key)))
}

// importOf writes, through SQL, an import's changeset of n nodes of shape
// into the bench's notebook as an import writes it, of the load: each
// page's node, content, version and creation's item, its row in the index
// and its links; each attachment's node, row and item. A page's id is
// md5ID of its number, its name "p" and the number. It answers the
// changeset.
func (b trashBench) importOf(tb testing.TB, shape string, n int) uuid.UUID {
	tb.Helper()
	c, pages, links := uuid.NewV7(), n-n/10, jobLinks(tb)
	b.exec(tb, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
	VALUES ($1, $2, 'import', 'web', $3, now(), now())`, c, b.nb, b.alice)
	parent, args := "NULL::uuid", []any{b.nb, b.alice, pages}
	switch shape {
	case shapeDeep:
		parent, args = "CASE WHEN g = 0 THEN NULL ELSE md5($1::uuid::text || ':' || ((g - 1) / $4::integer))::uuid END", append(args, fanout(pages))
	case shapeFolders:
		parent = "CASE WHEN g % 6 = 0 THEN NULL ELSE md5($1::uuid::text || ':' || (g / 6 * 6))::uuid END"
	}
	b.exec(tb, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
	SELECT md5($1::uuid::text || ':' || g)::uuid, $1, `+parent+`, 'page', 'p' || g, 'p' || g, g, $2, $2, now(), now()
	FROM generate_series(0, $3::integer - 1) g`, args...)
	b.exec(tb, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
	SELECT md5($1::uuid::text || ':a' || g)::uuid, $1, md5($1::uuid::text || ':' || (g * 9 % $3::integer))::uuid, 'asset', 'a' || g || '.png',
		'a' || g || '.png', g, $2, $2, now(), now()
	FROM generate_series(0, $4::integer - 1) g`, b.nb, b.alice, pages, n/10)
	b.exec(tb, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
	SELECT md5($1::uuid::text || ':' || g)::uuid, x.content, 1, sha256(convert_to(x.content, 'UTF8')), octet_length(x.content), $2, now()
	FROM generate_series(0, $3::integer - 1) g, LATERAL (
		SELECT rpad((SELECT string_agg('[[p' || ((g + k) % $3::integer) || ']]', E'\n') FROM generate_series(1, $4::integer) k) || E'\n',
			$5::integer, 'x') AS content
	) x`, b.nb, b.alice, pages, links, jobContent)
	b.exec(tb, `INSERT INTO page_revisions (id, changeset_id, node_id, revision, content, content_hash, byte_size, created_at, updated_at)
	SELECT gen_random_uuid(), $2, c.node_id, 1, c.content, c.content_hash, c.byte_size, now(), now()
	FROM generate_series(0, $3::integer - 1) g JOIN page_contents c ON c.node_id = md5($1::uuid::text || ':' || g)::uuid`, b.nb, c, pages)
	b.exec(tb, `INSERT INTO changeset_items (id, changeset_id, node_id, after_parent_id, after_name, after_sort_order, revision, created_at, updated_at)
	SELECT gen_random_uuid(), $2, n.id, n.parent_id, n.name, n.sort_order, CASE WHEN n.kind = 'page' THEN 1 END, now(), now()
	FROM nodes n WHERE n.notebook_id = $1`, b.nb, c)
	b.exec(tb, `INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at)
	SELECT gen_random_uuid(), md5($1::uuid::text || ':a' || g)::uuid, $1, 'image/png', 3, sha256('abc'), $2, now()
	FROM generate_series(0, $3::integer - 1) g`, b.nb, b.alice, n/10)
	b.exec(tb, `INSERT INTO indexed_pages (node_id, notebook_id, revision, extractor, frontmatter_valid)
	SELECT md5($1::uuid::text || ':' || g)::uuid, $1, 1, 1, true FROM generate_series(0, $2::integer - 1) g`, b.nb, pages)
	b.exec(tb, `INSERT INTO page_links (source_id, range_start, range_end, notebook_id, kind, target, target_key, resolved_id, ambiguous)
	SELECT md5($1::uuid::text || ':' || g)::uuid, k, k + 1, $1, 'wikilink', 'p' || t, 'p' || t, md5($1::uuid::text || ':' || t)::uuid, false
	FROM generate_series(0, $2::integer - 1) g, generate_series(1, $3::integer) k, LATERAL (SELECT (g + k) % $2::integer AS t) x`,
		b.nb, pages, links)
	return c
}

// importedAttachments writes, through SQL, an import's changeset of n
// attachments at the notebook's root, each with its row and its item; it
// answers the changeset.
func (b trashBench) importedAttachments(tb testing.TB, n int) uuid.UUID {
	tb.Helper()
	c := uuid.NewV7()
	b.exec(tb, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
	VALUES ($1, $2, 'import', 'web', $3, now(), now())`, c, b.nb, b.alice)
	b.exec(tb, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
	SELECT md5($1::uuid::text || ':a' || g)::uuid, $1, NULL, 'asset', 'a' || g || '.png', 'a' || g || '.png', g, $2, $2, now(), now()
	FROM generate_series(0, $3::integer - 1) g`, b.nb, b.alice, n)
	b.exec(tb, `INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at)
	SELECT gen_random_uuid(), md5($1::uuid::text || ':a' || g)::uuid, $1, 'image/png', 3, sha256('abc'), $2, now()
	FROM generate_series(0, $3::integer - 1) g`, b.nb, b.alice, n)
	b.exec(tb, `INSERT INTO changeset_items (id, changeset_id, node_id, after_parent_id, after_name, after_sort_order, created_at, updated_at)
	SELECT gen_random_uuid(), $2, md5($1::uuid::text || ':a' || g)::uuid, NULL, 'a' || g || '.png', g, now(), now()
	FROM generate_series(0, $3::integer - 1) g`, b.nb, c, n)
	return c
}

// revertOfAttachments writes, through SQL, the revert of the import c of
// n attachments at the notebook's root (importedAttachments) as its
// background job leaves it: each attachment deleted, a trash item of its
// own, in batches of 100 a millisecond apart, the revert's item of it and
// the import's in the trash at its batch's time; the job's changeset made
// at its first batch, written last at its last. At 300 attachments its
// rows read as the job's do, row by row. It answers the revert.
func (b trashBench) revertOfAttachments(tb testing.TB, c uuid.UUID, n int) uuid.UUID {
	tb.Helper()
	r, start := uuid.NewV7(), time.Now()
	b.exec(tb, `INSERT INTO changesets (id, notebook_id, kind, reverts_id, client, created_by_id, created_at, updated_at)
	VALUES ($1, $2, 'revert', $3, 'web', $4, $5, $5::timestamptz + ($6::integer - 1) / 100 * interval '1 millisecond')`, r, b.nb, c,
		b.alice, start, n)
	b.exec(tb, `UPDATE nodes n SET deleted_at = t.at, updated_by_id = $2, updated_at = t.at, trash_root = true, trash_pages = 0,
		trash_assets = 1, trash_height = 0
	FROM (SELECT md5($1::uuid::text || ':a' || g)::uuid AS id, $4::timestamptz + (g / 100) * interval '1 millisecond' AS at
		FROM generate_series(0, $3::integer - 1) g) t
	WHERE n.id = t.id`, b.nb, b.alice, n, start)
	b.exec(tb, `UPDATE changeset_items i SET deleted_at = n.deleted_at FROM nodes n
	WHERE i.changeset_id = $2 AND n.id = i.node_id AND n.notebook_id = $1`, b.nb, c)
	b.exec(tb, `INSERT INTO changeset_items (id, changeset_id, node_id, before_parent_id, before_name, before_sort_order, created_at,
		updated_at, deleted_at)
	SELECT gen_random_uuid(), $2, n.id, NULL, n.name, n.sort_order, n.deleted_at, n.deleted_at, n.deleted_at
	FROM nodes n WHERE n.notebook_id = $1 AND n.deleted_at IS NOT NULL`, b.nb, r)
	return r
}

// analyze refreshes the statistics of every table, as a job does the
// module's every 10,000 nodes it writes.
func (b trashBench) analyze(tb testing.TB) {
	tb.Helper()
	b.exec(tb, "ANALYZE")
}

// asAlice is a context of a session of alice's; asJob, of a job of hers.
func (b trashBench) asAlice() context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: b.alice, SessionID: uuid.NewV7()})
}

func (b trashBench) asJob() context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: b.alice, JobID: uuid.NewV7()})
}

// startRevert times a background revert's request of c as startRevert's
// check makes it, its admit doing nothing: CheckRevert.
func (b trashBench) startRevert(tb testing.TB, c uuid.UUID) time.Duration {
	tb.Helper()
	at := time.Now()
	if _, err := b.jobs.CheckRevert(b.asAlice(), b.nb, c, nil, func(context.Context) error { return nil }); err != nil {
		tb.Fatalf("CheckRevert(%s): %v", c, err)
	}
	return time.Since(at)
}

// jobRun is a background job's run as the bench measures it: its plan's
// time, the heap it holds and its steps; its batches' times; its
// changeset.
type jobRun struct {
	plan      time.Duration
	planBytes uint64
	steps     int
	batches   []time.Duration
	total     time.Duration
	changeset uuid.UUID
}

// String tells the run in milliseconds: the plan, its heap and steps; the
// batches, the first, the largest, the 95th percentile, the mean of the
// first ten and of the last ten (does a batch slow as the job goes); and
// the whole.
func (r jobRun) String() string {
	ms := func(d time.Duration) int64 { return d.Milliseconds() }
	if len(r.batches) == 0 {
		return fmt.Sprintf("plan %d ms (%d MiB, %d steps), no batch", ms(r.plan), r.planBytes>>20, r.steps)
	}
	s := slices.Sorted(slices.Values(r.batches))
	mean := func(ds []time.Duration) int64 {
		var sum time.Duration
		for _, d := range ds {
			sum += d
		}
		return ms(sum / time.Duration(len(ds)))
	}
	k := min(10, len(r.batches))
	return fmt.Sprintf("plan %d ms (%d MiB, %d steps); %d batches: first %d, max %d, p95 %d, first ten %d, last ten %d; total %d ms",
		ms(r.plan), r.planBytes>>20, r.steps, len(s), ms(r.batches[0]), ms(s[len(s)-1]), ms(s[len(s)*95/100]),
		mean(r.batches[:k]), mean(r.batches[len(r.batches)-k:]), ms(r.total))
}

// maxBatch is the run's largest batch.
func (r jobRun) maxBatch() time.Duration {
	return slices.Max(append([]time.Duration{0}, r.batches...))
}

// runJob plans a job with plan and writes its batches as the transfer
// module's run cuts them with the import's bounds: up to 100 nodes and
// 8 MiB, a step past them alone; a batch past a unit's history or links
// halved; a busy server waited for; the statistics refreshed every 10,000
// nodes written.
func (b trashBench) runJob(tb testing.TB, plan func(ctx context.Context, spec page.JobSpec) (page.JobPlan, error)) jobRun {
	tb.Helper()
	ctx := b.asJob()
	p, spec, r := b.jobPlanned(tb, ctx, plan)
	steps := p.Steps()
	start, unanalyzed := time.Now(), 0
	for from := 0; from < len(steps); {
		to := cut(steps, from)
		for {
			at := time.Now()
			_, err := b.jobs.RunBatch(ctx, spec, p, from, to)
			var e *shared.Error
			if errors.Is(err, page.ErrBatchTooLarge) && to-from > 1 {
				to = from + (to-from)/2
				continue
			}
			if errors.As(err, &e) && e.Kind == shared.KindUnavailable {
				time.Sleep(time.Second)
				continue
			}
			if err != nil {
				tb.Fatalf("the batch of steps %d to %d: %v", from, to, err)
			}
			r.batches = append(r.batches, time.Since(at))
			break
		}
		for _, s := range steps[from:to] {
			unanalyzed += s.Nodes
		}
		if unanalyzed >= 10_000 {
			b.analyze(tb)
			unanalyzed = 0
		}
		from = to
	}
	r.total = time.Since(start)
	runtime.KeepAlive(p)
	return r
}

// cut is the end of the batch from the step from, as the transfer
// module's run cuts it with the import's bounds: up to 100 nodes and
// 8 MiB, a step past them alone.
func cut(steps []page.JobStep, from int) int {
	to, nodes, bytes := from+1, steps[from].Nodes, steps[from].Bytes
	for ; to < len(steps) && nodes+steps[to].Nodes <= 100 && bytes+steps[to].Bytes <= 8<<20; to++ {
		nodes, bytes = nodes+steps[to].Nodes, bytes+steps[to].Bytes
	}
	return to
}

// revertJob runs the background revert of c.
func (b trashBench) revertJob(tb testing.TB, c uuid.UUID) jobRun {
	tb.Helper()
	return b.runJob(tb, func(ctx context.Context, spec page.JobSpec) (page.JobPlan, error) {
		return b.jobs.PlanRevert(ctx, spec, c)
	})
}

// restoreJob runs the background restore of the trash item of root.
func (b trashBench) restoreJob(tb testing.TB, root uuid.UUID) jobRun {
	tb.Helper()
	return b.runJob(tb, func(ctx context.Context, spec page.JobSpec) (page.JobPlan, error) {
		return b.jobs.PlanRestore(ctx, spec, root)
	})
}

// BenchmarkStartRevert measures a background revert's request of an
// import's changeset by its shape and size (M8/P4 design 3.11): its pages
// at the root, under a top five levels deep, and in folders; and of the
// revert of the revert of an import of attachments at the root, which
// restores a group of each (shapeGroups); each on a database of its own
// without statistics, then analyzed. It logs each, ms.
//
// Measured on an Apple M5 Max shared with other work (load 5-14), at the
// full load, three runs at MaxJobItems, ms, without statistics / analyzed:
//
//	nodes      flat          deep          folders       many groups
//	 60,000    324-358 /     417-663 /     338-386 /     744-836 /
//	           273-310       353-592       292-308       736-807
//
// The request grows with the changeset alone, a fifth of the costliest
// step at MaxJobItems (BenchmarkBackgroundSteps): the first round's, at
// ten links a page, 0.5 s at 90,000, 1.0 s at 200,000, 3.3 s at 550,000,
// flat, analyzed. An import makes MaxJobItems nodes at most (M8/P4 design
// 2.2 (a)).
func BenchmarkStartRevert(b *testing.B) {
	for _, n := range jobSizes(b) {
		for _, shape := range []string{shapeFlat, shapeDeep, shapeFolders, shapeGroups} {
			b.Run(fmt.Sprintf("%s/%d", shape, n), func(b *testing.B) {
				bench := newJobBench(b, 1<<30)
				var c uuid.UUID
				if shape == shapeGroups {
					c = bench.revertOfAttachments(b, bench.importedAttachments(b, n), n)
				} else {
					c = bench.importOf(b, shape, n)
				}
				raw := bench.startRevert(b, c)
				bench.analyze(b)
				analyzed := bench.startRevert(b, c)
				b.ReportMetric(float64(raw.Milliseconds()), "nostats-ms")
				b.ReportMetric(float64(analyzed.Milliseconds()), "analyzed-ms")
				b.Logf("the request of the revert of an import of %d nodes, %s: %d ms without statistics, %d ms analyzed", n, shape,
					raw.Milliseconds(), analyzed.Milliseconds())
			})
		}
	}
}

// BenchmarkBackgroundJobs measures the background jobs whole (M8/P4
// design 3.11), each on a database of its own, its tables without
// statistics or analyzed before it starts: the revert of an import of
// each shape and of attachments at the root; the request of the revert
// of that revert, which restores many groups (one of a top, or of a
// folder, its pages and attachments), or one group of every node (the
// deep shape), and that revert; for the deep shape, then the top's
// deletion, the request of its restore, and its item's background
// restore. Each run tells its plan, its heap and its batches
// (jobRun.String). It logs each, ms.
//
// Measured in the first round, at 90,000 nodes of pages of ten links,
// before MaxJobItems was taken at the full load (60,000), on an Apple M5
// Max shared with other work (load 5-8), analyzed: the import's revert,
// its batches' p95 and the whole; the revert of that revert, the same;
// seconds unless ms:
//
//	shape          the import's revert       its revert's revert
//	flat           325 ms, 191 s             411 ms, 250 s
//	deep           one step, 5.2 s           252 ms, 171 s
//	folders        228 ms, 120 s             357 ms, 240 s
//	attachments    266 ms, 173 s             184 ms, 108 s
//
// Without statistics a batch is three to four times slower until the
// job's first ANALYZE, then as analyzed. The plans hold 18 MiB at most;
// the deep top's item's restore, 900 batches, p95 310 ms, 191 s. A batch
// grows with the notebook by its re-plan's read of the whole live tree
// alone (24 ms at 50,000 nodes), not by its tops' subtrees (M8/P4 design
// 3.11's Subtrees, not needed).
func BenchmarkBackgroundJobs(b *testing.B) {
	for _, n := range jobSizes(b) {
		for _, shape := range []string{shapeFlat, shapeDeep, shapeFolders, "attachments"} {
			for _, stats := range []string{"nostats", "analyzed"} {
				b.Run(fmt.Sprintf("%s/%d/%s", shape, n, stats), func(b *testing.B) {
					bench := newJobBench(b, 1<<30)
					var c uuid.UUID
					if shape == "attachments" {
						c = bench.importedAttachments(b, n)
					} else {
						c = bench.importOf(b, shape, n)
					}
					if stats == "analyzed" {
						bench.analyze(b)
					}
					run := bench.revertJob(b, c)
					b.ReportMetric(float64(run.maxBatch().Milliseconds()), "revert-max-batch-ms")
					b.Logf("%s, %d nodes, %s: the import's revert: %s", shape, n, stats, run)
					request := bench.startRevert(b, run.changeset)
					b.ReportMetric(float64(request.Milliseconds()), "revert-of-revert-request-ms")
					b.Logf("%s, %d nodes, %s: the request of the revert of its revert %d ms", shape, n, stats, request.Milliseconds())
					back := bench.revertJob(b, run.changeset)
					b.ReportMetric(float64(back.maxBatch().Milliseconds()), "revert-of-revert-max-batch-ms")
					b.Logf("%s, %d nodes, %s: the revert of its revert: %s", shape, n, stats, back)
					if shape != shapeDeep {
						return
					}
					top := md5ID(bench.nb, "0")
					at := time.Now()
					if status, answer := bench.serve("DELETE", "/api/v0/nodes/"+top.String(), ""); status != 204 {
						b.Fatalf("delete the top = %d %s", status, answer)
					}
					b.Logf("%s, %d nodes, %s: the top's deletion %d ms", shape, n, stats, time.Since(at).Milliseconds())
					at = time.Now()
					if _, err := bench.jobs.CheckRestore(bench.asAlice(), bench.nb, top); err != nil {
						b.Fatal(err)
					}
					b.Logf("%s, %d nodes, %s: the request of its restore %d ms", shape, n, stats, time.Since(at).Milliseconds())
					restore := bench.restoreJob(b, top)
					b.ReportMetric(float64(restore.maxBatch().Milliseconds()), "restore-max-batch-ms")
					b.Logf("%s, %d nodes, %s: its item's restore: %s", shape, n, stats, restore)
				})
			}
		}
	}
}

// BenchmarkBackgroundSteps measures the steps no batch splits (M8/P4
// design 3.11): the costliest a revert's job cuts, by the size of the
// changeset, the largest of them all, the first batch of the revert of
// the import of a top and the pages under it, which deletes them, and the
// first batch of that revert's revert, which reads the group whole, its
// items, nodes, hashes and the tree, plans it again, and restores the
// first of it; each request too. Then a page's background restore beside
// its deletion, the page of a million rows of history; the first batch of
// the restore of a page with 5,000 attachments under it, which marks the
// 4,901 it leaves trash items of their own (markFrontier); and the
// arrays' statements at 10,000 and 50,000 ids, planned with their
// arguments and for any (P2A's lesson). It logs each, ms.
//
// Measured on an Apple M5 Max shared with other work (load 5-14), at the
// full load, ms, without statistics / analyzed, three runs each: the
// deletion of the top, MaxJobItems the largest size of ten thousands it
// takes 5 s at most at in every run; the first batch of the group's
// restore; the request of the revert's revert:
//
//	nodes     the top's deletion          the group's first batch       the request
//	60,000    3,544-3,981 / 4,220-4,642   1,281-1,555 / 1,295-2,203     628-737 / 620-1,231
//	70,000    3,973-6,907 / 4,267-4,564   1,561-3,210 / 1,490-1,670     688-1,636 / 645-662
//	80,000    4,278-4,361 / 5,946-6,108   1,968-2,328 / 2,065-2,695     730-896 / 884-1,007
//
// 70,000's 6,907 ms run was a busy machine's (every step of it slower),
// but a run past 5 s is past the bound (P3A's rule: every run, not the
// median); 90,000 took 6,465 ms (the review's run) and 6,508 ms.
//
// The deletion grows with the links of the pages as much as with the
// nodes, about 1.6 µs a link: 60,000 nodes, analyzed, 3,227-3,317 ms at
// ten links a page, 4,220-4,642 at 20, 5,864-6,244 at 40 (the group's
// first batch 927-1,106, 1,295-2,203, 2,798-3,773); past 20 a page the
// bound does not hold the step to 5 s, as it does not a user's deletion of
// the same top, the same unit. At 90,000 and 20 links its statements:
// DeleteNodes 2.2 s, LinksReached 1.4 s, RecordItems 0.9 s, DeleteLinksOf
// 0.9 s. LinksReached reads every link the deletion deleted, which meet
// its keys and targets (an import's links point at its own pages) in the
// unit that deleted them: read without the deleted pages as sources, 1.34
// against 1.38 s, so it is as it was.
//
// Then, in the first round, at ten links a page and load 8-10: the
// plans of a deep import's revert and of the
// restore of its top's item, 488 ms with 3 MiB and 198 ms with 2 MiB at
// 90,000 nodes, 3,662 ms with 18 MiB and 1,045 ms with 15 MiB at 550,000
// (the revert's revert's of one group of every node, 7 MiB at 50,000,
// 14 MiB at 100,000). A page of a million rows of history: its deletion
// 6,235 ms, its restore 8,925 ms, 1.43 times. A page with 5,000
// attachments: its restore's first batch, which marks the 4,901 left,
// 50 ms. The arrays' statements at 50,000 ids, planned with them / for
// any: TrashNodesAt 135 / 188, TrashGroups 154 / 210, ChangesetItemsOf
// 56 / 45, ChangesetVersionsOf 62 / 56, NodesNow 270 / 252, MarkTrashRoots
// 14 / 54; none of them slow for any. NodesNow stays a cached statement,
// as P3 left it, its plan for any no slower; the others run planned.
func BenchmarkBackgroundSteps(b *testing.B) {
	for _, n := range jobSizes(b) {
		for _, stats := range []string{"nostats", "analyzed"} {
			b.Run(fmt.Sprintf("top/%d/%s", n, stats), func(b *testing.B) {
				bench := newJobBench(b, 1<<30)
				c := bench.importOf(b, shapeDeep, n)
				if stats == "analyzed" {
					bench.analyze(b)
				}
				request := bench.startRevert(b, c)
				first, r := bench.firstBatch(b, c)
				// The job refreshes the statistics past 10,000 nodes written.
				bench.analyze(b)
				again := bench.startRevert(b, r)
				group, _ := bench.firstBatch(b, r)
				b.ReportMetric(float64(first.Milliseconds()), "top-ms")
				b.ReportMetric(float64(group.Milliseconds()), "group-ms")
				b.Logf("a top of %d nodes, %d links a page, %s: the request of its import's revert %d ms, its deletion %d ms; "+
					"the request of that revert's revert %d ms, its first batch %d ms", n, jobLinks(b), stats, request.Milliseconds(),
					first.Milliseconds(), again.Milliseconds(), group.Milliseconds())
			})
		}
		// The plans' time and heap: the revert's of the import, and the
		// restore's of the trash item its first batch made of the top.
		b.Run(fmt.Sprintf("plans/%d", n), func(b *testing.B) {
			bench := newJobBench(b, 1<<30)
			c := bench.importOf(b, shapeDeep, n)
			bench.analyze(b)
			revert := bench.planOnly(b, func(ctx context.Context, spec page.JobSpec) (page.JobPlan, error) {
				return bench.jobs.PlanRevert(ctx, spec, c)
			})
			bench.firstBatch(b, c)
			bench.analyze(b)
			restore := bench.planOnly(b, func(ctx context.Context, spec page.JobSpec) (page.JobPlan, error) {
				return bench.jobs.PlanRestore(ctx, spec, md5ID(bench.nb, "0"))
			})
			b.Logf("%d nodes: the revert's plan %s, the restore's of its top %s", n, revert, restore)
		})
	}
	b.Run("history", func(b *testing.B) {
		bench := newJobBench(b, 0)
		id := bench.page(b, "Old")
		bench.exec(b, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
		SELECT md5($1::uuid::text || ':c' || g)::uuid, $1, 'edit', 'web', $2, now() - interval '1 day', now() - interval '1 day'
		FROM generate_series(1, 1000000) g`, bench.nb, bench.alice)
		bench.exec(b, `INSERT INTO changeset_items (id, changeset_id, node_id, before_parent_id, before_name, before_sort_order,
			after_parent_id, after_name, after_sort_order, created_at, updated_at)
		SELECT gen_random_uuid(), md5($1::uuid::text || ':c' || g)::uuid, $2, NULL, 'Old', 0, NULL, 'Old', 0, now() - interval '1 day',
			now() - interval '1 day'
		FROM generate_series(1, 1000000) g`, bench.nb, id)
		bench.analyze(b)
		at := time.Now()
		if status, answer := bench.serve("DELETE", "/api/v0/nodes/"+id.String(), ""); status != 204 {
			b.Fatalf("delete Old = %d %s", status, answer)
		}
		deleted := time.Since(at)
		run := bench.restoreJob(b, id)
		b.Logf("a page of 1,000,000 rows of history: its deletion %d ms, its background restore: %s", deleted.Milliseconds(), run)
	})
	b.Run("frontier", func(b *testing.B) {
		bench := newJobBench(b, 0)
		id := bench.page(b, "Root")
		c := uuid.NewV7()
		bench.exec(b, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
		VALUES ($1, $2, 'import', 'web', $3, now(), now())`, c, bench.nb, bench.alice)
		bench.exec(b, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
			created_at, updated_at)
		SELECT md5($1::uuid::text || ':a' || g)::uuid, $1, $3, 'asset', 'a' || g || '.png', 'a' || g || '.png', g, $2, $2, now(), now()
		FROM generate_series(0, 4999) g`, bench.nb, bench.alice, id)
		bench.exec(b, `INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at)
		SELECT gen_random_uuid(), md5($1::uuid::text || ':a' || g)::uuid, $1, 'image/png', 3, sha256('abc'), $2, now()
		FROM generate_series(0, 4999) g`, bench.nb, bench.alice)
		bench.exec(b, `INSERT INTO changeset_items (id, changeset_id, node_id, after_parent_id, after_name, after_sort_order, created_at,
			updated_at)
		SELECT gen_random_uuid(), $2, md5($1::uuid::text || ':a' || g)::uuid, $3, 'a' || g || '.png', g, now(), now()
		FROM generate_series(0, 4999) g`, bench.nb, c, id)
		if status, answer := bench.serve("DELETE", "/api/v0/nodes/"+id.String(), ""); status != 204 {
			b.Fatalf("delete Root = %d %s", status, answer)
		}
		bench.analyze(b)
		run := bench.restoreJob(b, id)
		b.Logf("a page with 5,000 attachments under it: its background restore: %s", run)
	})
	b.Run("arrays", func(b *testing.B) {
		for _, n := range []int{10_000, 50_000} {
			bench := newJobBench(b, 0)
			c := bench.importOf(b, shapeFlat, n)
			bench.analyze(b)
			run := bench.revertJob(b, c)
			bench.analyze(b)
			for _, generic := range []bool{false, true} {
				if generic {
					bench.generic(b)
				}
				b.Logf("%d nodes, planned for any arguments %v: %s", n, generic, bench.arrays(b, c, run.changeset))
			}
		}
	})
}

// jobPlanned plans a job of a changeset of its own with plan as ctx's
// actor, and answers the plan, its spec, and the run so far: the plan's
// time and the heap it holds after a collection.
func (b trashBench) jobPlanned(tb testing.TB, ctx context.Context, plan func(ctx context.Context, spec page.JobSpec) (page.JobPlan, error)) (
	page.JobPlan, page.JobSpec, jobRun,
) {
	tb.Helper()
	spec := page.JobSpec{NotebookID: b.nb, Client: domain.ClientWeb, Changeset: uuid.NewV7()}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	at := time.Now()
	p, err := plan(ctx, spec)
	if err != nil {
		tb.Fatalf("the plan: %v", err)
	}
	r := jobRun{plan: time.Since(at), changeset: spec.Changeset}
	runtime.GC()
	runtime.ReadMemStats(&after)
	r.planBytes, r.steps = after.HeapAlloc-min(before.HeapAlloc, after.HeapAlloc), len(p.Steps())
	return p, spec, r
}

// planOnly is a job's plan alone, as runJob measures it.
func (b trashBench) planOnly(tb testing.TB, plan func(ctx context.Context, spec page.JobSpec) (page.JobPlan, error)) jobRun {
	tb.Helper()
	p, _, r := b.jobPlanned(tb, b.asJob(), plan)
	runtime.KeepAlive(p)
	return r
}

// firstBatch plans the background revert of c and writes its first batch
// alone: it answers how long the batch took, and the job's changeset.
func (b trashBench) firstBatch(tb testing.TB, c uuid.UUID) (time.Duration, uuid.UUID) {
	tb.Helper()
	spec, ctx := page.JobSpec{NotebookID: b.nb, Client: domain.ClientWeb, Changeset: uuid.NewV7()}, b.asJob()
	p, err := b.jobs.PlanRevert(ctx, spec, c)
	if err != nil {
		tb.Fatalf("PlanRevert(%s): %v", c, err)
	}
	at := time.Now()
	if _, err := b.jobs.RunBatch(ctx, spec, p, 0, cut(p.Steps(), 0)); err != nil {
		tb.Fatalf("the first batch of the revert of %s: %v", c, err)
	}
	return time.Since(at), spec.Changeset
}

// page creates a page of the title at the notebook's root through the
// module and answers its id.
func (b trashBench) page(tb testing.TB, title string) uuid.UUID {
	tb.Helper()
	status, answer := b.serve("POST", "/api/v0/notebooks/"+b.nb.String()+"/pages", `{"parent_id":null,"title":"`+title+`","content":"x"}`)
	if status != 201 {
		tb.Fatalf("create %s = %d %s", title, status, answer)
	}
	var id uuid.UUID
	if err := b.pool.QueryRow(context.Background(), "SELECT id FROM nodes WHERE notebook_id = $1 AND name = $2", b.nb, title).Scan(
		&id); err != nil {
		tb.Fatal(err)
	}
	return id
}

// arrays times, once each, the background jobs' statements of arrays of
// ids on the trash the revert r of the import c left: the nodes of every
// id at its deletion, the groups of the trash items (their levels'
// children), the frontier's mark (rolled back), the items and the
// versions of c, and the nodes now with the exemption of r's revert.
func (b trashBench) arrays(tb testing.TB, c, r uuid.UUID) string {
	tb.Helper()
	ctx := context.Background()
	store := pagepostgres.New(b.pool)
	at, roots := map[uuid.UUID]time.Time{}, map[uuid.UUID]time.Time{}
	rows, err := b.pool.Query(ctx, "SELECT id, deleted_at, trash_root FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL", b.nb)
	if err != nil {
		tb.Fatal(err)
	}
	var ids []uuid.UUID
	var marks []pageapp.TrashMark
	for rows.Next() {
		var id uuid.UUID
		var t time.Time
		var root bool
		if err := rows.Scan(&id, &t, &root); err != nil {
			tb.Fatal(err)
		}
		at[id], ids = t, append(ids, id)
		if root {
			roots[id], marks = t, append(marks, pageapp.TrashMark{ID: id, Pages: 1})
		}
	}
	if err := rows.Err(); err != nil {
		tb.Fatal(err)
	}
	var out []string
	timed := func(name string, f func() error) {
		start := time.Now()
		if err := f(); err != nil {
			tb.Fatalf("%s: %v", name, err)
		}
		out = append(out, fmt.Sprintf("%s %d", name, time.Since(start).Milliseconds()))
	}
	timed("TrashNodesAt", func() error { _, err := store.TrashNodesAt(ctx, b.nb, at); return err })
	timed("TrashGroups", func() error { _, err := store.TrashGroups(ctx, b.nb, roots); return err })
	timed("ChangesetItemsOf", func() error { _, err := store.ChangesetItemsOf(ctx, c, ids); return err })
	timed("ChangesetVersionsOf", func() error { _, err := store.ChangesetVersionsOf(ctx, c, ids); return err })
	timed("NodesNow", func() error { _, err := store.NodesNow(ctx, ids, r); return err })
	rollback := errors.New("rolled back")
	timed("MarkTrashRoots", func() error {
		err := postgres.NewTxManager(b.pool, time.Minute).WithinTx(ctx, func(ctx context.Context) error {
			if _, err := store.MarkTrashRoots(ctx, marks, at[marks[0].ID]); err != nil {
				return err
			}
			return rollback
		})
		if errors.Is(err, rollback) {
			return nil
		}
		return err
	})
	return strings.Join(out, ", ") + " ms"
}

// The guards of a background revert at MaxJobItems (M8/P4 design 3.11),
// each a line of its own in make test-go-guards, with -v, so that CI's
// runs log their times: their limits are three times the times measured
// here, CI being about 2.1 times slower (M8/P4 review B-M11). A request's
// time is the least of three runs (fastestOf): a runner's slow moment once
// took one past its limit (M8/P5B review, decision 10; M8 closeout A-1).
// The bench keeps the composition root's bound: one far from the domain's
// fails here, and TestPageDepsLeaveTheUnitsBoundsTheDomains pins it
// exactly.

// A background revert's request of an import of MaxJobItems nodes,
// analyzed, is answered in its time: of the import, its pages at the root
// (BenchmarkStartRevert's other shapes take about as long); and of the
// revert of the revert of an import of as many attachments at the root,
// which restores as many groups (shapeGroups; M8/P4A review C-M5), the
// revert written through SQL as its job writes it. A request reads
// NodesNow and TrashGroups of every item; the batches' arrays' statements
// at scale, the changeset's items and versions of every node of a top and
// the mark of a group's frontier, TestARevertJobOfAnImportAtMaxJobItems
// times in its largest steps (M8/P4A fix check K-4). One
// more node in the import is page.revert_too_large before its items are
// read, admit not asked.
func TestStartRevertAtMaxJobItems(t *testing.T) {
	t.Run("many groups", func(t *testing.T) {
		bench := newJobBench(t, 0)
		r := bench.revertOfAttachments(t, bench.importedAttachments(t, page.MaxJobItems), page.MaxJobItems)
		bench.analyze(t)
		took := fastestOf(t, fmt.Sprintf("the request of the revert of a revert of %d groups", page.MaxJobItems),
			func() time.Duration { return bench.startRevert(t, r) })
		if took > manyGroupsLimit {
			t.Errorf("the request of the revert of a revert of %d groups took %s, want %s at most", page.MaxJobItems, took, manyGroupsLimit)
		}
	})
	bench := newJobBench(t, 0)
	c := bench.importOf(t, shapeFlat, page.MaxJobItems)
	bench.analyze(t)
	took := fastestOf(t, fmt.Sprintf("the request of the revert of an import of %d nodes", page.MaxJobItems),
		func() time.Duration { return bench.startRevert(t, c) })
	if took > startRevertLimit {
		t.Errorf("the request of the revert of an import of %d nodes took %s, want %s at most", page.MaxJobItems, took, startRevertLimit)
	}
	bench.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at)
		VALUES (md5($1::uuid::text || ':extra')::uuid, $1, NULL, 'asset', 'extra.png', 'extra.png', -1, $2, $2, now(), now())`,
		bench.nb, bench.alice)
	bench.exec(t, `INSERT INTO changeset_items (id, changeset_id, node_id, after_parent_id, after_name, after_sort_order, created_at,
		updated_at)
		VALUES (gen_random_uuid(), $2, md5($1::uuid::text || ':extra')::uuid, NULL, 'extra.png', -1, now(), now())`, bench.nb, c)
	admitted := false
	_, err := bench.jobs.CheckRevert(bench.asAlice(), bench.nb, c, nil, func(context.Context) error { admitted = true; return nil })
	var e *shared.Error
	if !errors.As(err, &e) || e.Code != "page.revert_too_large" || admitted {
		t.Errorf("the request of the revert of %d nodes = %v, admitted %v; want page.revert_too_large, not admitted", page.MaxJobItems+1,
			err, admitted)
	}
}

// A background revert of an import of guardJobItems nodes in folders, as
// a vault's, analyzed as an import's end leaves its tables, ends in its
// time, the trash listing its tops, a folder's page of each six; and the
// largest steps any revert's job takes take their time: the first batch
// of the revert of an import of a top and every node under it, which
// deletes them all in one unit, and the first batch of that revert's
// revert, which plans the group of every node again (BenchmarkBackgroundSteps). The invariants are
// read analyzed, as the job leaves the tables past 10,000 nodes: a group
// of every node, its statistics read before its deletion, makes their
// recursive read of the groups read every node of its time for each.
func TestARevertJobOfAnImportAtMaxJobItems(t *testing.T) {
	t.Run("in folders", func(t *testing.T) {
		bench := newJobBench(t, 0)
		c := bench.importOf(t, shapeFolders, guardJobItems)
		bench.analyze(t)
		run := bench.revertJob(t, c)
		took := run.plan + run.total
		t.Logf("the revert of an import of %d nodes in folders: %s", guardJobItems, run)
		if took > revertJobLimit {
			t.Errorf("the revert of an import of %d nodes in folders took %s, want %s at most", guardJobItems, took, revertJobLimit)
		}
		pages := guardJobItems - guardJobItems/10
		if n := count(t, bench.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND trash_root", bench.nb); n != (pages+5)/6 {
			t.Errorf("%d trash items, want the import's %d tops", n, (pages+5)/6)
		}
		if n := count(t, bench.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL", bench.nb); n != 0 {
			t.Errorf("%d nodes alive, want none", n)
		}
		checkPages(t, bench.pool)
	})
	t.Run("its largest step", func(t *testing.T) {
		bench := newJobBench(t, 0)
		c := bench.importOf(t, shapeDeep, page.MaxJobItems)
		bench.analyze(t)
		took, r := bench.firstBatch(t, c)
		t.Logf("the deletion of a top of %d nodes took %s", page.MaxJobItems, took)
		if took > largestStepLimit {
			t.Errorf("the deletion of a top of %d nodes took %s, want %s at most", page.MaxJobItems, took, largestStepLimit)
		}
		if n := count(t, bench.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL", bench.nb); n != 0 {
			t.Errorf("%d nodes alive, want none", n)
		}
		bench.analyze(t)
		checkPages(t, bench.pool)
		// The first batch of that deletion's revert: the group of every
		// node planned again, its first members restored, the rest marked
		// trash items of their own (M8/P4A fix check K-4).
		group, _ := bench.firstBatch(t, r)
		t.Logf("the first batch of the restore of a group of %d nodes took %s", page.MaxJobItems, group)
		if group > groupBatchLimit {
			t.Errorf("the first batch of the restore of a group of %d nodes took %s, want %s at most", page.MaxJobItems, group,
				groupBatchLimit)
		}
		bench.analyze(t)
		checkPages(t, bench.pool)
	})
}

// guardJobItems are the nodes of the import whose revert's job the guard
// runs whole: a job of a size near MaxJobItems takes minutes (120 s in
// folders at 90,000, the first round's), past the minute a guard takes at
// most (M8/P4 design 3.11), so 10,000; BenchmarkBackgroundJobs measures
// the jobs whole.
const guardJobItems = 10_000

// fastestOf is the least of three runs of what, which run times, each
// logged: a slower runner's moment slows a run, never speeds one, so that
// the least of them is the request's own time, and a real regression slows
// all three (M8 closeout A-1). The limits stay as they were.
func fastestOf(tb testing.TB, what string, run func() time.Duration) time.Duration {
	tb.Helper()
	runs := []time.Duration{run(), run(), run()}
	tb.Logf("%s took %s, the least of %v", what, slices.Min(runs), runs)
	return slices.Min(runs)
}

// The guards' limits, three times the times measured here at the full
// load (load 6-8): the request 0.3 s, of many groups 0.7 s, the job of
// 10,000 nodes 5.5-6.5 s; the largest steps three times the bench's
// slowest runs (load 6-13), the deletion 3.6-4.7 s here, 4.6 s there, the
// group's first batch 0.75-1.0 s here, 2.2 s there.
const (
	startRevertLimit = 1 * time.Second
	manyGroupsLimit  = 2100 * time.Millisecond
	revertJobLimit   = 17 * time.Second
	largestStepLimit = 14 * time.Second
	groupBatchLimit  = 6600 * time.Millisecond
)
