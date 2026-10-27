//go:build !race

package bootstrap

import (
	"context"
	"encoding/json"
	"runtime"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// This is the successful source-entry plan of a real deletion's revert,
// not the cheaper source-out plan of an import's deletion. The existing
// full SQL corpus keeps pages, versions, items and links; its attachments
// have distinct valid byte/hash metadata but no files. Planning reads only
// metadata, so these measurements make no file-operation cost claim.
// All 18 initial samples took at most 1.661868s. The 5s bound rounds up
// three times that slowest successful sample; every later sample must pass.
func TestSourceBackgroundPlanAtMaxJobItemsCostsItsWork(t *testing.T) {
	probe := &sourcePlanCostProbe{trace: &sourceEntryTrace{}, assets: &sourceEntryAttachments{}}
	var commitTimeout time.Duration
	bench := newTrashBenchOf(t, func(c *config.Config) {
		commitTimeout = c.Database.CommitTimeout
	}, func(d *page.Deps) {
		cfg := d.Pool.Config()
		probe.trace.previous, cfg.ConnConfig.Tracer = cfg.ConnConfig.Tracer, probe.trace
		pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		tx := postgres.NewTxManager(pool, commitTimeout)
		d.Pool, d.Tx, d.Snapshots, probe.pool = pool, tx, tx, pool
		probe.assets.AttachmentInfos, d.Attachments = d.Attachments, probe.assets
	})
	reverted := sourcePlanCostFixture(t, bench)
	var pgVersion string
	if err := bench.pool.QueryRow(t.Context(), "SHOW server_version").Scan(&pgVersion); err != nil {
		t.Fatal(err)
	}
	t.Logf("source plan corpus: nodes=%d pages=%d assets=%d bytes/page=%d links/page=%d Go=%s GOOS=%s GOARCH=%s CPUs=%d PG=%s",
		page.MaxJobItems, page.MaxJobItems-page.MaxJobItems/10, page.MaxJobItems/10, jobContent, jobLinks(t),
		runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), pgVersion)
	for _, stage := range []string{"no_statistics", "analyzed", "generic"} {
		switch stage {
		case "analyzed":
			bench.analyze(t)
		case "generic":
			bench.generic(t)
			// The traced page pool also has to pick up the owned database's
			// new default. The production ports retain Planned execution.
			probe.pool.Reset()
		}
		stats := count(t, bench.pool, "SELECT count(*) FROM pg_stats WHERE schemaname='public' AND tablename='nodes'")
		if stage == "no_statistics" && stats != 0 || stage != "no_statistics" && stats == 0 {
			t.Fatalf("stage %s has %d nodes statistics, want its stated statistics state", stage, stats)
		}
		var mode string
		if err := probe.pool.QueryRow(t.Context(), "SHOW plan_cache_mode").Scan(&mode); err != nil {
			t.Fatal(err)
		}
		if stage == "generic" && mode != "force_generic_plan" || stage != "generic" && mode != "auto" {
			t.Fatalf("stage %s actual page pool plan_cache_mode=%s", stage, mode)
		}
		for _, route := range []string{"check", "plan"} {
			t.Run(stage+"/"+route, func(t *testing.T) {
				var samples []time.Duration
				for sample := 1; sample <= 3; sample++ {
					e := sourcePlanCostSample(t, bench, probe, reverted, route)
					e.Stage, e.Route, e.Sample, e.PlanCacheMode = stage, route, sample, mode
					encoded, err := json.Marshal(e)
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("source plan sample %s", encoded)
					sourceCostWithin(t, "complete60k "+stage+"/"+route, time.Duration(e.ElapsedNS), 5*time.Second)
					samples = append(samples, time.Duration(e.ElapsedNS))
				}
				t.Logf("%s %s successful 60k source-entry samples=%v minimum=%s maximum=%s; each sample bound=5s",
					stage, route, samples, slices.Min(samples), slices.Max(samples))
			})
		}
	}
	// All setup and independent database invariants are outside every timer.
	if got := count(t, bench.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND deleted_at IS NOT NULL", bench.nb); got != page.MaxJobItems {
		t.Fatalf("read-only samples changed trash count to %d", got)
	}
	if got := count(t, bench.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", bench.nb); got != 4 {
		t.Fatalf("read-only samples left %d changesets, want import/raw/AGENTS/deletion", got)
	}
	checkPages(t, bench.pool)
	assertSourceHashesMatchRebuild(t, t.Context(), bench.pool)
}

type sourcePlanCostProbe struct {
	trace  *sourceEntryTrace
	assets *sourceEntryAttachments
	pool   *pgxpool.Pool
}

type sourcePlanQueryWork struct {
	Calls int64 `json:"calls"`
	Rows  int64 `json:"rows"`
}

type sourcePlanSample struct {
	Stage         string                         `json:"stage"`
	Route         string                         `json:"route"`
	Sample        int                            `json:"sample"`
	PlanCacheMode string                         `json:"plan_cache_mode"`
	ElapsedNS     int64                          `json:"elapsed_ns"`
	Allocated     uint64                         `json:"allocated_bytes"`
	Steps         int                            `json:"steps"`
	Cuts          int                            `json:"nominal_cuts"`
	AssetCalls    int64                          `json:"asset_metadata_calls"`
	Queries       map[string]sourcePlanQueryWork `json:"queries"`
}

// Each callback follows the existing complete-segment 100-node/8MiB cut,
// not allJobBatches. The restore-only fixture must produce 600 cuts.
func sourcePlanCostBatches(steps []page.JobStep) [][2]int {
	var out [][2]int
	for from := 0; from < len(steps); {
		to := cut(steps, from)
		out, from = append(out, [2]int{from, to}), to
	}
	return out
}

func sourcePlanCostSample(t *testing.T, bench trashBench, probe *sourcePlanCostProbe, reverted uuid.UUID, route string) sourcePlanSample {
	t.Helper()
	var plan page.JobPlan
	var before, after runtime.MemStats
	var cuts [][2]int
	callbacks, steps, admitted := 0, 0, 0
	batches := func(s []page.JobStep) [][2]int {
		callbacks, steps = callbacks+1, len(s)
		cuts = sourcePlanCostBatches(s)
		return cuts
	}
	ctx, spec := bench.asJob(), page.JobSpec{NotebookID: bench.nb, Client: page.Client("web"), Changeset: uuid.NewV7()}
	if route == "check" {
		ctx = bench.asAlice()
	}
	probe.assets.calls.Store(0)
	probe.assets.active.Store(true)
	probe.trace.start()
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	var err error
	if route == "check" {
		_, err = bench.jobs.CheckRevert(ctx, bench.nb, reverted, nil, func(context.Context) error { admitted++; return nil }, batches)
	} else {
		plan, err = bench.jobs.PlanRevert(ctx, spec, reverted, batches)
	}
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	queries := probe.trace.stop()
	probe.assets.active.Store(false)
	if err != nil {
		t.Fatalf("successful source-entry %s: %v", route, err)
	}
	if callbacks != 1 || steps != page.MaxJobItems || len(cuts) != page.MaxJobItems/100 || route == "check" && admitted != 1 {
		t.Fatalf("%s callbacks=%d steps=%d cuts=%d admits=%d, want one full production nominal plan", route, callbacks, steps, len(cuts), admitted)
	}
	for i, bounds := range cuts {
		if bounds != [2]int{i * 100, (i + 1) * 100} {
			t.Fatalf("nominal cut %d=%v, want [%d,%d)", i, bounds, i*100, (i+1)*100)
		}
	}
	if route == "plan" {
		if plan.Total() != page.MaxJobItems || len(plan.Steps()) != page.MaxJobItems {
			t.Fatalf("successful plan total=%d steps=%d, want %d", plan.Total(), len(plan.Steps()), page.MaxJobItems)
		}
		for i, step := range plan.Steps() {
			if step.Nodes != 1 || step.Joined || step.Bytes != jobContent && step.Bytes != 0 {
				t.Fatalf("restore step %d=%+v, want one full-load page or asset with no parsed body bytes", i, step)
			}
		}
	}
	if got := queries["SourceHeads"]; got.calls != 6 || got.rows != page.MaxJobItems {
		t.Fatalf("%s source metadata work=%+v, want six real chunks covering 60000 IDs", route, got)
	}
	if got := queries["ProbeSources"]; got.calls != 6 || got.rows != 0 {
		t.Fatalf("%s source probes=%+v, want six full key batches and no live outside candidate", route, got)
	}
	if probe.assets.calls.Load() != 1 {
		t.Fatalf("%s asset metadata calls=%d, want one 6000-asset batch", route, probe.assets.calls.Load())
	}
	for _, name := range []string{"PageContent", "Contents", "VersionContents", "TrashContents", "TrashContentsWithin", "SetSourceHashes"} {
		if got := queries[name]; got.calls != 0 {
			t.Fatalf("preview performed body/write work %s=%+v", name, got)
		}
	}
	e := sourcePlanSample{ElapsedNS: elapsed.Nanoseconds(), Allocated: after.TotalAlloc - before.TotalAlloc, Steps: steps,
		Cuts: len(cuts), AssetCalls: probe.assets.calls.Load(), Queries: map[string]sourcePlanQueryWork{}}
	for name, work := range queries {
		e.Queries[name] = sourcePlanQueryWork{Calls: work.calls, Rows: work.rows}
	}
	runtime.KeepAlive(plan)
	return e
}

func sourcePlanCostFixture(t *testing.T, bench trashBench) uuid.UUID {
	t.Helper()
	const n = page.MaxJobItems
	imported := bench.importOf(t, shapeDeep, n)
	// These unrelated creations follow importOf so its creation-item SELECT
	// cannot include them. Keep the LLM root and AGENTS outside the group.
	raw := bench.page(t, "raw")
	bench.page(t, "AGENTS")
	top := md5ID(bench.nb, "0")
	bench.exec(t, "UPDATE nodes SET parent_id=$2 WHERE id=$1", top, raw)
	bench.exec(t, "UPDATE changeset_items SET after_parent_id=$3 WHERE changeset_id=$1 AND node_id=$2", imported, top, raw)
	// A blob's UUID bytes are a distinct 16-byte payload. This DB-only cost
	// fixture retains exact size/hash facts, without claiming physical files.
	bench.exec(t, "UPDATE asset_blobs SET byte_size=16, sha256=sha256(uuid_send(node_id)) WHERE notebook_id=$1", bench.nb)
	bench.exec(t, "UPDATE notebooks SET mode='llm_wiki', source_root_id=$2 WHERE id=$1", bench.nb, raw)
	bench.exec(t, `UPDATE nodes n SET source_hash=c.content_hash FROM page_contents c
		WHERE c.node_id=n.id AND n.id IN (SELECT node_id FROM changeset_items WHERE changeset_id=$1)`, imported)
	bench.exec(t, `UPDATE nodes n SET source_hash=b.sha256 FROM asset_blobs b
		WHERE b.node_id=n.id AND n.notebook_id=$1`, bench.nb)
	bench.exec(t, `INSERT INTO page_search_queue(node_id,notebook_id,bulk)
		SELECT id,notebook_id,true FROM nodes WHERE notebook_id=$1 ON CONFLICT(node_id) DO NOTHING`, bench.nb)
	for _, c := range []struct {
		name, sql string
		args      []any
		want      int
	}{
		{"all nodes", "SELECT count(*) FROM nodes WHERE notebook_id=$1", []any{bench.nb}, n + 2},
		{"import items", "SELECT count(*) FROM changeset_items WHERE changeset_id=$1", []any{imported}, n},
		{"full page versions", "SELECT count(*) FROM page_revisions WHERE changeset_id=$1 AND byte_size=2048 AND octet_length(content_hash)=32", []any{imported}, n - n/10},
		{"distinct full page bodies", "SELECT count(DISTINCT content_hash) FROM page_revisions WHERE changeset_id=$1", []any{imported}, n - n/10},
		{"distinct valid assets", "SELECT count(DISTINCT sha256) FROM asset_blobs WHERE notebook_id=$1 AND byte_size=16 AND sha256=sha256(uuid_send(node_id))", []any{bench.nb}, n / 10},
		{"resolved links", "SELECT count(*) FROM page_links WHERE notebook_id=$1 AND resolved_id IS NOT NULL", []any{bench.nb}, (n - n/10) * jobLinks(t)},
		{"eligible hashes", "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND source_hash IS NOT NULL", []any{bench.nb}, n},
	} {
		if got := count(t, bench.pool, c.sql, c.args...); got != c.want {
			t.Fatalf("source plan fixture %s=%d, want %d", c.name, got, c.want)
		}
	}
	checkPages(t, bench.pool)
	assertSourceHashesMatchRebuild(t, t.Context(), bench.pool)
	ctx, spec := bench.asJob(), page.JobSpec{NotebookID: bench.nb, Client: page.Client("web"), Changeset: uuid.NewV7()}
	plan, err := bench.jobs.PlanRevert(ctx, spec, imported, sourcePlanCostBatches)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Total() != n || len(plan.Steps()) != 1 || plan.Steps()[0].Nodes != n {
		t.Fatalf("setup deletion plan total=%d steps=%+v, want one real 60k subtree deletion", plan.Total(), plan.Steps())
	}
	if done, err := bench.jobs.RunBatch(ctx, spec, plan, 0, 1); err != nil || done.Deleted != n {
		t.Fatalf("setup deletion=%+v, %v", done, err)
	}
	if got := count(t, bench.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND deleted_at IS NOT NULL AND source_hash IS NULL", bench.nb); got != n {
		t.Fatalf("setup deletion left %d cleared trash nodes, want %d", got, n)
	}
	if got := count(t, bench.pool, "SELECT count(*) FROM changeset_items WHERE changeset_id=$1 AND deleted_at IS NOT NULL", spec.Changeset); got != n {
		t.Fatalf("actual deletion recorded %d trash items, want %d", got, n)
	}
	t.Logf("source plan fixture setup: actual import %s reverted by %s; 60000 source nodes now trash, only raw/AGENTS live", imported, spec.Changeset)
	return spec.Changeset
}
