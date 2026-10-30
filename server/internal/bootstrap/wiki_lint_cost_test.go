//go:build !race

package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

// This is the actual bootstrap Module.Execute, with the installed authority,
// snapshot, source-area, metadata, linking, asset and final Paths ports. Seed
// and validation are outside the measured calls. CPU/heap are the Go process,
// including the server and observation overhead; neither is PostgreSQL RSS.
func TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("100k true wiki runtime cost runs alone without race")
	}
	f := newWikiCostFixture(t)
	actor := f.actor(t, "alice")
	stopped := false
	for _, operation := range []string{"summary", "wanted"} {
		if stopped {
			return
		}
		t.Run(operation, func(t *testing.T) {
			var samples [3]wikiCostSample
			for sample := 1; sample <= 3; sample++ {
				finished := false
				t.Run(fmt.Sprintf("sample-%d", sample), func(t *testing.T) {
					f.sampleCleanup(t, operation, sample, &finished, f.event)
					samples[sample-1] = f.measure(t, actor, operation, sample)
				})
				if !finished {
					stopped = true
					return
				}
			}
			if !t.Failed() {
				checkWikiCost(t, operation, samples)
			}
		})
	}
}

// Cleanup runs after measure has logged its fixed Execute/CPU/heap sample,
// including when measure uses Fatal. Created is synchronous before worker
// launch, so a missing queued Created must stop sampling without a new wait.
func (f *wikiCostFixture) sampleCleanup(t *testing.T, operation string, sample int, finished *bool,
	wait func(*testing.T, wiki.RunEventKind, uint64) wiki.RunEvent,
) {
	t.Helper()
	t.Cleanup(func() {
		start := time.Now()
		var created wiki.RunEvent
		defer func() {
			lintCostLog(t, "wiki-cleanup", struct {
				Operation string
				Sample    int
				FlightID  uint64
				Created   bool
				Finished  bool
				WaitNS    int64
			}{operation, sample, created.FlightID, created.Kind == wiki.RunCreated, *finished, int64(time.Since(start))})
		}()
		for created.Kind != wiki.RunCreated {
			select {
			case event := <-f.events:
				if event.Kind == wiki.RunCreated {
					created = event
				}
			default:
				t.Error("wiki cost sample has no queued RunCreated; refusing the next reset")
				return
			}
		}
		if created.NotebookID != uuid.MustParse(f.book.id) || created.FlightID == 0 {
			t.Error("wiki cost sample received another notebook's or invalid flight; refusing the next reset")
			return
		}
		wait(t, wiki.RunFinished, created.FlightID)
		*finished = true
	})
}

type wikiCostSample struct {
	WallNS, CPUNS, AllocatedBytes, PeakIncrementBytes uint64
}

// Three successful runs of the unchanged 3315-file input each supplied three
// samples. For each metric: 3 * the largest round median, rounded up to 100ms
// or 1MiB. These development-calibrated guards leave the real 2s worker and
// 4s caller unchanged; the summary wall guard is therefore not extra runtime.
func checkWikiCost(t *testing.T, operation string, samples [3]wikiCostSample) {
	t.Helper()
	limits := map[string]wikiCostSample{
		"summary": {uint64(5200 * time.Millisecond), uint64(2100 * time.Millisecond), 1571 << 20, 404 << 20},
		"wanted":  {uint64(1400 * time.Millisecond), uint64(1500 * time.Millisecond), 1104 << 20, 348 << 20},
	}
	limit, ok := limits[operation]
	if !ok {
		t.Fatalf("no calibrated cost limits for %s", operation)
	}
	for _, metric := range []struct {
		name   string
		values [3]uint64
		limit  uint64
	}{
		{"wall-ns", [3]uint64{samples[0].WallNS, samples[1].WallNS, samples[2].WallNS}, limit.WallNS},
		{"go-process-cpu-ns", [3]uint64{samples[0].CPUNS, samples[1].CPUNS, samples[2].CPUNS}, limit.CPUNS},
		{"go-total-alloc-bytes", [3]uint64{samples[0].AllocatedBytes, samples[1].AllocatedBytes, samples[2].AllocatedBytes}, limit.AllocatedBytes},
		{"sampled-heap-increment-bytes", [3]uint64{samples[0].PeakIncrementBytes, samples[1].PeakIncrementBytes, samples[2].PeakIncrementBytes}, limit.PeakIncrementBytes},
	} {
		slices.Sort(metric.values[:])
		median := metric.values[1]
		t.Logf("wiki cost %s median %s=%d bound=%d", operation, metric.name, median, metric.limit)
		if median > metric.limit {
			t.Errorf("wiki cost %s median %s=%d exceeds calibrated bound %d", operation, metric.name, median, metric.limit)
		}
	}
}

func (f *wikiCostFixture) measure(t *testing.T, actor context.Context, operation string, sample int) wikiCostSample {
	t.Helper()
	query := wiki.LintQuery{}
	if operation == "wanted" {
		rule, limit := "wanted", 50
		query.Rule = &rule
		query.Limit = &limit
	}
	f.trace.reset()
	f.lintRuntimeFixture.trace.mu.Lock()
	f.lintRuntimeFixture.trace.queries = nil
	f.lintRuntimeFixture.trace.observations = nil
	f.lintRuntimeFixture.trace.mu.Unlock()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	stop, peak := make(chan struct{}), make(chan uint64, 1)
	finish := sync.OnceFunc(func() { close(stop) })
	defer finish()
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		high := before.HeapAlloc
		for {
			var now runtime.MemStats
			runtime.ReadMemStats(&now)
			high = max(high, now.HeapAlloc)
			select {
			case <-stop:
				peak <- high
				return
			case <-ticker.C:
			}
		}
	}()
	ctx, cancel := context.WithTimeout(actor, 4*time.Second)
	defer cancel()
	cpu, start := originCPU(t), time.Now()
	result, err := f.module.Execute(ctx, wiki.LintScope{NotebookID: uuid.MustParse(f.book.id)}, query)
	elapsed := time.Since(start)
	cpu = originCPU(t) - cpu
	finish()
	high := <-peak
	runtime.ReadMemStats(&after)
	queries, stages, timings, pathIDs := f.trace.stop()
	var sqlNS int64
	for _, q := range queries {
		sqlNS += q.ElapsedNS
	}
	encoded, jsonErr := json.Marshal(result)
	if jsonErr != nil {
		t.Fatal(jsonErr)
	}
	lintCostLog(t, "wiki-execute", struct {
		Operation                                                                   string
		Sample                                                                      int
		ElapsedNS, ProcessCPUNS, ObservedSQLScanNS, OtherWallNS                     int64
		AllocatedBytes, BaselineHeapBytes, SampledPeakHeapBytes, PeakIncrementBytes uint64
		SamplePeriodNS                                                              int64
		ResultBytes                                                                 int
		Failed                                                                      bool
		Queries                                                                     []wikiCostSQL
		Stages                                                                      []wikiCostStage
		Timings                                                                     *wiki.PreparationTimings
	}{operation, sample, int64(elapsed), cpu, sqlNS, int64(elapsed) - sqlNS,
		after.TotalAlloc - before.TotalAlloc, before.HeapAlloc, high, high - before.HeapAlloc,
		int64(time.Millisecond), len(encoded), err != nil, queries, stages, timings})
	// A ByKeys regression is diagnosed even if its full ancestor materializing
	// work exhausts the real 2s worker: expiry is not the mutation oracle.
	for _, q := range queries {
		if q.Name == "LinkTargetsByKeys" {
			t.Fatalf("S49 wanted consumer read full candidate ancestors through LinkTargetsByKeys (rows=%d stage=%s)", q.Rows, q.Stage)
		}
	}
	if err != nil {
		t.Fatalf("actual %s Execute under unchanged 4s request/2s worker: %v", operation, err)
	}
	if sqlNS > int64(elapsed) {
		t.Fatal("observed sequential SQL+scan wall exceeds Execute; observation accounting is invalid")
	}
	f.checkResult(t, result, operation)
	f.checkTrace(t, queries, timings, pathIDs, result, operation)
	return wikiCostSample{uint64(elapsed), uint64(cpu), after.TotalAlloc - before.TotalAlloc, high - before.HeapAlloc}
}

func (f *wikiCostFixture) checkResult(t *testing.T, result wiki.LintResult, operation string) {
	t.Helper()
	wantRules := 8
	if operation == "wanted" {
		wantRules = 1
	}
	if result.Mode != wiki.ModeLLMWiki || result.StalePages != 0 || len(result.Rules) != wantRules {
		t.Fatalf("actual %s summary mode/stale/rules=%s/%d/%d", operation, result.Mode, result.StalePages, len(result.Rules))
	}
	wanted := lintResultRule(t, result, wiki.RuleWanted)
	if wanted.Count.Value != 2 || wanted.Count.Capped || len(wanted.Items) != 2 || wanted.NextCursor != nil || wanted.StartCursor == nil {
		t.Fatalf("independent wanted count/items=%+v", wanted)
	}
	for i, target := range []string{"missing/needle.js", "truly-wanted"} {
		item := wanted.Items[i]
		display := target
		if i == 0 {
			display += ".md"
		}
		key := fmt.Sprintf("w:%x", sha256.Sum256([]byte(target)))
		if item.ItemKey != key || item.Target != display || item.TargetCut || item.Pages != 1 || item.PageID != nil || item.NodeID != nil || item.Path != nil || item.Cursor == "" {
			t.Fatalf("wanted item %d=%+v, want independent %s/%s/page spelling", i, item, key, display)
		}
	}
	if operation == "wanted" {
		return
	}
	for _, rule := range []wiki.LintRule{wiki.RuleAmbiguous, wiki.RuleInvalidFrontmatter, wiki.RuleMissingType, wiki.RuleMissingSummary, wiki.RuleInvalidSources} {
		got := lintResultRule(t, result, rule)
		if got.Count.Value != 0 || got.Count.Capped || got.Items == nil || len(got.Items) != 0 || got.NextCursor != nil || got.StartCursor != nil {
			t.Fatalf("independent empty %s=%+v", rule, got)
		}
	}
	orphans := lintResultRule(t, result, wiki.RuleOrphans)
	if orphans.Count.Value != 10000 || !orphans.Count.Capped || len(orphans.Items) != 5 || orphans.NextCursor == nil {
		t.Fatalf("capped orphan preview=%+v", orphans)
	}
	path := "branch00000"
	for i, item := range orphans.Items {
		if i > 0 {
			path += fmt.Sprintf("/level%d", i)
		}
		id := wikiCostID(0, i)
		if item.PageID == nil || *item.PageID != id || item.Path == nil || *item.Path != path+".md" || item.ItemKey != "p:"+id.String() {
			t.Fatalf("orphan %d=%+v, independent id/path=%s/%s.md", i, item, id, path)
		}
	}
	pending := lintResultRule(t, result, wiki.RuleUndigestedSources)
	if pending.Count.Value != 2 || pending.Count.Capped || len(pending.Items) != 2 || pending.NextCursor != nil {
		t.Fatalf("independent pending=%+v", pending)
	}
	for i, want := range []struct {
		id, path string
		kind     wiki.NodeKind
		bytes    int64
		created  time.Time
	}{
		{f.source, "raw/Source.md", wiki.NodePage, int64(len("source payload")), f.sourceCreated},
		{f.pending, "raw/pending.txt", wiki.NodeAsset, int64(len("pending bytes")), f.assetCreated},
	} {
		item := pending.Items[i]
		if item.NodeID == nil || item.NodeID.String() != want.id || item.Path == nil || *item.Path != want.path || item.Kind != want.kind || item.Bytes != want.bytes || !item.CreatedAt.Equal(want.created) {
			t.Fatalf("pending %d=%+v, want independent %+v", i, item, want)
		}
	}
}

func (f *wikiCostFixture) checkTrace(t *testing.T, queries []wikiCostSQL, timings *wiki.PreparationTimings, pathIDs []uuid.UUID, result wiki.LintResult, operation string) {
	t.Helper()
	counts := map[string]int64{}
	calls := map[string]int{}
	var first, last string
	var pid uint32
	var snapshot uint64
	for _, q := range queries {
		if q.Name == "" {
			continue
		}
		counts[q.Name] += q.Rows
		calls[q.Name]++
		if first == "" {
			first = q.Name
			pid = q.PID
			snapshot = q.Snapshot
		}
		last = q.Name
		if q.PID != pid || q.Snapshot != snapshot || snapshot == 0 || q.Failed {
			t.Fatalf("real payload query escaped/failed snapshot: %+v", q)
		}
	}
	if operation == "summary" {
		if calls["LintFrontmatterCount"] != 0 {
			t.Errorf("known-valid frontmatter queried Count: calls=%d, want none", calls["LintFrontmatterCount"])
		}
		for _, name := range []string{"LintFrontmatterItemsFirst", "LintMissingTypeItemsFirst", "LintMissingSummaryItemsFirst"} {
			if calls[name] != 0 {
				t.Errorf("zero-count rule queried Items: %s calls=%d, want none", name, calls[name])
			}
		}
		if calls["LintOrphanCount"] != 1 || calls["LintOrphanItemsFirst"] != 1 {
			t.Errorf("positive orphan count lost its real Items query: calls=%v", calls)
		}
	}
	if calls["LintNodeMetadata"] != 1 || counts["LintNodeMetadata"] != int64(100000+f.bulkAssets+2) {
		t.Fatalf("metadata must be read once: calls=%d rows=%d; want 1/%d", calls["LintNodeMetadata"], counts["LintNodeMetadata"], 100000+f.bulkAssets+2)
	}
	if first != "FindNotebook" || counts["LintHeads"] != 100000 || counts["LintWantedEdgeHeadsFirst"] != 3 || counts["LintEdgesByPositions"] != 3 {
		t.Fatalf("real nonempty compact/wanted trace=%v first=%s", counts, first)
	}
	var wantedIDs []uuid.UUID
	for _, rule := range result.Rules {
		for _, item := range rule.Items {
			if item.PageID != nil {
				wantedIDs = append(wantedIDs, *item.PageID)
			}
			if item.NodeID != nil {
				wantedIDs = append(wantedIDs, *item.NodeID)
			}
		}
	}
	slices.SortFunc(wantedIDs, uuid.UUID.Compare)
	wantedIDs = slices.Compact(wantedIDs)
	slices.SortFunc(pathIDs, uuid.UUID.Compare)
	if !slices.Equal(pathIDs, wantedIDs) {
		t.Fatalf("final Paths ids=%v, want only actual output ids=%v", pathIDs, wantedIDs)
	}
	for _, q := range queries {
		if q.Name == "LinkTargetsByIDs" {
			ids := slices.Clone(q.PathIDs)
			slices.SortFunc(ids, uuid.UUID.Compare)
			if !slices.Equal(ids, wantedIDs) {
				t.Fatalf("display SQL ids=%v, want final output ids=%v", ids, wantedIDs)
			}
		}
	}
	if operation == "summary" && (last != "LinkTargetsByIDs" || counts["AttachmentInfos"] != 1 || len(pathIDs) != 7) {
		t.Fatalf("summary final output/asset SQL=%v paths=%v last=%s", counts, pathIDs, last)
	}
	if operation == "wanted" && (counts["LinkTargetsByIDs"] != 0 || len(pathIDs) != 0) {
		t.Fatal("wanted materialized display paths despite having no page/node IDs")
	}
	if timings == nil || len(timings.Rules) != len(result.Rules) || operation == "summary" && timings.SharedSources == nil || operation == "wanted" && timings.SharedSources != nil {
		t.Fatalf("actual preparation timing availability=%+v", timings)
	}
	_, observations := f.lintRuntimeFixture.trace.copy()
	if len(observations) == 0 || observations[0].Phase != "state" {
		t.Fatal("no real StateOf snapshot observation")
	}
	for _, got := range observations {
		if got.PID != pid || got.Snapshot != snapshot || got.Isolation != "repeatable read" || got.ReadOnly != "on" || got.View != observations[0].View {
			t.Fatalf("actual snapshot changed: %+v", observations)
		}
	}
}

// The child deliberately calls the real measure Fatal path. Its overall exit
// is always 1; only the two named child outcomes establish cleanup ordering.
func TestWikiCostSamplesWaitForFinished(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: needs Docker and an expected-failure child")
	}
	const name = "TestWikiCostSamplesWaitForFinished"
	if os.Getenv("NERVEWIKI_WIKI_COST_CLEANUP_CHILD") != "1" {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+name+"$", "-test.v", "-test.count=1", "-test.timeout=10m")
		cmd.Env = append(os.Environ(), "NERVEWIKI_WIKI_COST_CLEANUP_CHILD=1")
		output, err := cmd.CombinedOutput()
		exit := 0
		var childError *exec.ExitError
		if errors.As(err, &childError) {
			exit = childError.ExitCode()
		}
		pid := 0
		if cmd.Process != nil {
			pid = cmd.Process.Pid
		}
		t.Logf("wiki-cleanup child test=%s pid=%d argv=%q exit=%d error=%v\n%s", name, pid, cmd.Args, exit, err, output)
		text := string(output)
		for _, forbidden := range []string{"panic:", "WARNING: DATA RACE", "test timed out", "fatal error:"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("unexpected child failure %q", forbidden)
			}
		}
		if !errors.As(err, &childError) || exit != 1 || pid == 0 ||
			strings.Count(text, "=== RUN   "+name+"/sample-failure\n") != 1 ||
			strings.Count(text, "=== RUN   "+name+"/next-reset\n") != 1 ||
			!strings.Contains(text, "--- FAIL: "+name+"/sample-failure ") ||
			!strings.Contains(text, "actual summary Execute under unchanged 4s request/2s worker: context canceled") {
			t.Fatalf("fresh cost child did not reach the designated real measure Fatal and two actual nodes: %v", err)
		}
		if !strings.Contains(text, "--- PASS: "+name+"/next-reset ") ||
			strings.Contains(text, "--- FAIL: "+name+"/next-reset ") ||
			!strings.Contains(text, "CLEANUP_BEFORE_RESET_OK") {
			t.Fatal("fresh cost child reset before the previous real worker finished; expected next-reset PASS and CLEANUP_BEFORE_RESET_OK")
		}
		return
	}

	barrier := &wikiCostQueryBarrier{entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(barrier.release) }) }
	trace := &wikiCostTrace{previous: barrier}
	var rulesDone atomic.Bool
	var createdID, finishedID atomic.Uint64
	finishedEvent := make(chan struct{})
	r := newLintRuntime(t, func(d *wiki.Deps) {
		d.Sources = wikiCostSources{d.Sources, trace}
		d.Nodes = wikiCostNodes{d.Nodes, trace}
		d.Rules = wikiCostCompletionRules{rules: wikiCostRules{d.Rules, trace}, done: &rulesDone}
		d.Paths = wikiCostPaths{d.Paths, trace}
		observe := d.RunObserver
		d.RunObserver = func(event wiki.RunEvent) {
			if event.Kind == wiki.RunCreated {
				createdID.Store(event.FlightID)
			}
			if event.Kind == wiki.RunFinished {
				finishedID.Store(event.FlightID)
				close(finishedEvent)
			}
			observe(event)
		}
	})
	barrier.previous = r.trace.previous
	r.trace.previous = trace
	book, _, _ := r.seedPending(t)
	f := &wikiCostFixture{lintRuntimeFixture: r, book: book, trace: trace}
	// This independent outer cleanup also releases the deliberately broken
	// implementation, so its failure cannot strand the real pool or worker.
	t.Cleanup(func() {
		release()
		if createdID.Load() == 0 {
			return
		}
		select {
		case <-finishedEvent:
		case <-time.After(interleavingWait):
			t.Error("instrumentation outer cleanup did not observe actual RunFinished")
		}
	})
	actor, cancel := context.WithCancel(f.actor(t, "alice"))
	defer cancel()
	cancelDone := make(chan bool, 1)
	go func() {
		select {
		case <-barrier.entered:
			cancel()
			cancelDone <- true
		case <-time.After(interleavingWait):
			cancel()
			cancelDone <- false
		}
	}()
	wait := func(t *testing.T, kind wiki.RunEventKind, flight uint64) wiki.RunEvent {
		if kind == wiki.RunFinished {
			release()
		}
		return f.event(t, kind, flight)
	}
	completed := false
	t.Run("sample-failure", func(t *testing.T) {
		f.sampleCleanup(t, "summary", 1, &completed, wait)
		f.measure(t, actor, "summary", 1)
		t.Fatal("the real canceled measure unexpectedly returned")
	})
	if !<-cancelDone {
		t.Fatal("real LintHeads QueryEnd never reached the cancellation barrier")
	}
	t.Run("next-reset", func(t *testing.T) {
		flight, snapshot := createdID.Load(), barrier.snapshot.Load()
		if flight == 0 || snapshot == 0 {
			t.Fatal("catcher did not observe an actual flight and SQL snapshot")
		}
		ready := completed && finishedID.Load() == flight && barrier.ended.Load() && rulesDone.Load()
		f.trace.reset()
		if !ready {
			wait(t, wiki.RunFinished, flight)
			queries, stages, _, _ := f.trace.stop()
			lateQuery, lateStage := false, false
			for _, query := range queries {
				lateQuery = lateQuery || query.Name == "LintHeads" && query.Snapshot == snapshot
			}
			for _, stage := range stages {
				lateStage = lateStage || stage.Name == "rules"
			}
			if !lateQuery || !lateStage || !barrier.ended.Load() || !rulesDone.Load() {
				t.Fatalf("catcher failed to witness actual old SQL/stage after reset: query=%t stage=%t", lateQuery, lateStage)
			}
			t.Fatalf("sample reset preceded previous RunFinished/trace completion: flight=%d snapshot=%d late_query=%t late_stage=%t", flight, snapshot, lateQuery, lateStage)
		}
		queries, stages, _, _ := f.trace.stop()
		if len(queries) != 0 || len(stages) != 0 {
			t.Fatalf("completed worker contaminated the next sample: queries=%+v stages=%+v", queries, stages)
		}
		t.Logf("CLEANUP_BEFORE_RESET_OK flight=%d snapshot=%d", flight, snapshot)
	})
}

type wikiCostQueryBarrier struct {
	previous         pgx.QueryTracer
	entered, release chan struct{}
	once             sync.Once
	ended            atomic.Bool
	snapshot         atomic.Uint64
}

func (b *wikiCostQueryBarrier) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if b.previous != nil {
		return b.previous.TraceQueryStart(ctx, conn, data)
	}
	return ctx
}
func (b *wikiCostQueryBarrier) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if b.previous != nil {
		b.previous.TraceQueryEnd(ctx, conn, data)
	}
	start, ok := ctx.Value(wikiCostSQLKey{}).(wikiCostSQLStart)
	if ok && start.row.Name == "LintHeads" {
		b.once.Do(func() {
			b.snapshot.Store(start.row.Snapshot)
			close(b.entered)
			<-b.release
			b.ended.Store(true)
		})
	}
}

type wikiCostCompletionRules struct {
	rules wiki.Rules
	done  *atomic.Bool
}

func (r wikiCostCompletionRules) PrepareRules(ctx context.Context, in wiki.ReadInput, query wiki.RuleQuery) (wiki.PreparedRules, error) {
	prepared, err := r.rules.PrepareRules(ctx, in, query)
	r.done.Store(true)
	return prepared, err
}
