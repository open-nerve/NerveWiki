//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand/v2"
	"runtime"
	"strings"
	"testing"
	"time"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

func lintToast(t *testing.T, f fixture, table string, read func(context.Context) error) int64 {
	t.Helper()
	var before, after int64
	const sql = `SELECT coalesce(sum(pg_stat_get_xact_blocks_fetched(r.oid)),0)::bigint FROM (
 SELECT reltoastrelid AS oid FROM pg_class WHERE oid=$1::regclass
 UNION ALL SELECT indexrelid FROM pg_index WHERE indrelid=(SELECT reltoastrelid FROM pg_class WHERE oid=$1::regclass)) r`
	err := postgres.NewTxManager(f.pool, time.Second).WithinSnapshot(t.Context(), func(ctx context.Context) error {
		db := postgres.DB(ctx, f.pool)
		if err := db.QueryRow(ctx, sql, table).Scan(&before); err != nil {
			return err
		}
		if err := read(ctx); err != nil {
			return err
		}
		return db.QueryRow(ctx, sql, table).Scan(&after)
	})
	if err != nil {
		t.Fatal(err)
	}
	return after - before
}

// The interval includes the real parser, writer and selected read/consumer
// subtests (including their independent assertions), but excludes constructing
// the handwritten input/oracle. Heap peak is sampled every ms.
// PG relation work is measured independently; these are Go process metrics.
func lintMeasured(t *testing.T, name string, run func()) {
	t.Helper()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	stopped, finished := make(chan struct{}), make(chan uint64, 1)
	go func() {
		peak := before.HeapAlloc
		tick := time.NewTicker(time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				peak = max(peak, m.HeapAlloc)
			case <-stopped:
				finished <- peak
				return
			}
		}
	}()
	started := time.Now()
	defer func() {
		elapsed := time.Since(started)
		runtime.ReadMemStats(&after)
		close(stopped)
		peak := max(<-finished, after.HeapAlloc)
		lintEvidence(t, name, map[string]any{"wall_ns": elapsed.Nanoseconds(), "go_total_alloc": after.TotalAlloc - before.TotalAlloc, "sampled_heap_peak": peak, "heap_at_start": before.HeapAlloc, "scope": "parser + production index write + selected bounded-read/rule-consumer subtests and independent assertions; excludes fixture strings and PostgreSQL memory"})
	}()
	run()
}

func TestLintJSONToast(t *testing.T) {
	if testing.Short() {
		t.Skip("TOAST and parser growth costs")
	}
	t.Run("target-and-JSON-physical-controls", func(t *testing.T) {
		f := newFixture(t)
		p := uuid.UUID{}
		rng := rand.New(rand.NewPCG(20261030, 1))
		raw := make([]byte, 768*1024)
		for i := range raw {
			raw[i] = byte(rng.Uint32())
		}
		target := base64.RawURLEncoding.EncodeToString(raw) + "/x"
		content := "---\nsources: '[[" + target + "]]'\n---\n[[" + target + "]]"
		facts := originFacts(t, content)
		if !facts.FrontmatterValid || len(facts.Links) != 2 {
			t.Fatalf("real TOAST fixture: valid=%t links=%d", facts.FrontmatterValid, len(facts.Links))
		}
		f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, facts)
		r := postgresadapter.NewLintReads(f.pool, 5*1024*1024)
		var heads []app.EdgeHead
		blocks := lintToast(t, f, "page_links", func(ctx context.Context) error {
			var err error
			heads, err = r.WantedEdgeHeads(ctx, f.eng, nil, nil, 512)
			return err
		})
		if len(heads) != 2 || blocks != 0 || heads[0].TargetBytes != int64(len(target)) {
			t.Fatalf("short target heads: %d rows, %d TOAST blocks", len(heads), blocks)
		}
		var exact []app.LintEdge
		positive := lintToast(t, f, "page_links", func(ctx context.Context) error {
			var err error
			exact, err = r.Edges(ctx, f.eng, []app.EdgePos{heads[0].EdgePos})
			return err
		})
		if positive <= 0 || len(exact) != 1 || exact[0].Target != target {
			t.Fatalf("target point read: %d rows, %d TOAST blocks", len(exact), positive)
		}
		var jsonHeads []app.SourcePropertyHead
		headJSON := lintToast(t, f, "page_properties", func(ctx context.Context) error {
			var err error
			jsonHeads, err = r.SourcePropertyHeads(ctx, f.eng, []uuid.UUID{p})
			return err
		})
		if len(jsonHeads) != 1 || jsonHeads[0].PageID != p || jsonHeads[0].Position != 0 || jsonHeads[0].ValueBytes != int64(len(target)+6) {
			t.Fatalf("nonempty first JSON head: %+v; expected position 0, bytes %d", jsonHeads, len(target)+6)
		}
		var values []app.SourceProperty
		exactJSON := lintToast(t, f, "page_properties", func(ctx context.Context) error {
			var err error
			values, err = r.SourceProperties(ctx, f.eng, []app.SourcePropertyPos{jsonHeads[0].SourcePropertyPos})
			return err
		})
		if headJSON <= 0 || exactJSON <= 0 || len(values) != 1 || string(values[0].JSON) != `"[[`+target+`]]"` {
			t.Fatalf("two necessary JSON reads: heads=%d, exact=%d, rows=%d", headJSON, exactJSON, len(values))
		}
		slotBlocks := lintToast(t, f, "page_links", func(ctx context.Context) error {
			slots, err := r.SourceSlotEdges(ctx, f.eng, []app.EdgePos{heads[0].EdgePos})
			if err == nil && (len(slots) != 1 || slots[0].Target != target) {
				return fmt.Errorf("slot exact target mismatch: %d rows", len(slots))
			}
			return err
		})
		if slotBlocks <= 0 {
			t.Fatal("slot target positive TOAST control read no blocks")
		}
		f.resolve(t, p, domain.Resolution{ID: uuid.NewV7()}, facts.Links[1].Start)
		for _, table := range []string{"page_links", "page_properties"} {
			bodyBlocks := lintToast(t, f, table, func(ctx context.Context) error {
				body, err := r.BodyReferences(ctx, f.eng, []uuid.UUID{p}, nil, 512)
				if err == nil && len(body) != 1 {
					return fmt.Errorf("short body positive witness: %d rows", len(body))
				}
				return err
			})
			if bodyBlocks != 0 {
				t.Fatalf("short body read %s TOAST: %d", table, bodyBlocks)
			}
		}
		lintEvidence(t, "toast-controls", map[string]any{"target_bytes": len(target), "target_head_blocks": blocks, "exact_target_blocks": positive, "JSON_head_blocks": headJSON, "JSON_exact_blocks": exactJSON})
	})
	t.Run("legal-escaped-target", func(t *testing.T) {
		const c = 5 * 1024 * 1024
		const n = 1800000
		content := "---\nsources: \"[[a" + strings.Repeat(`\L`, n) + "z/x]]\"\n---\n"
		want := "a" + strings.Repeat("\u2028", n) + "z/x"
		if len(content) >= c || len(want) <= c {
			t.Fatal("not a legal escaped-growth witness")
		}
		f := newFixture(t)
		p := uuid.UUID{}
		tr := &lintTrace{}
		pool := lintPool(t, f, tr, 1)
		r := postgresadapter.NewLintReads(pool, c)
		reads := app.NewLintReads(r, c)
		lintMeasured(t, "legal-target-parser-writer-reads", func() {
			facts := originFacts(t, content)
			if !facts.FrontmatterValid || len(facts.Links) != 1 || facts.Links[0].Target != want {
				t.Fatalf("parser-growth witness failed: valid=%t links=%d", facts.FrontmatterValid, len(facts.Links))
			}
			f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, facts)
			var stored, key string
			var resolved *uuid.UUID
			var start int32
			if err := f.pool.QueryRow(t.Context(), `SELECT target,target_key,resolved_id,range_start FROM page_links WHERE source_id=$1`, p).Scan(&stored, &key, &resolved, &start); err != nil {
				t.Fatal(err)
			}
			if stored != want || key != "x" || resolved != nil {
				t.Fatalf("persisted witness: target bytes=%d key=%q resolved=%v", len(stored), key, resolved)
			}
			t.Run("wanted", func(t *testing.T) {
				t.Run("bounded-reads", func(t *testing.T) {
					heads, err := r.WantedEdgeHeads(t.Context(), f.eng, nil, nil, 512)
					if err != nil || len(heads) != 1 || heads[0].Start != start || heads[0].TargetBytes != int64(len(want)) {
						t.Fatalf("nonempty real wanted heads: %+v %v", heads, err)
					}
					stream := reads.Wanted(f.eng, nil)
					edges, err := stream.Next(t.Context())
					if err != nil || len(edges) != 1 || edges[0].Target != want || edges[0].Start != start {
						t.Fatalf("complete wanted growth target: rows=%d %v", len(edges), err)
					}
					end, err := stream.Next(t.Context())
					if err != nil || len(end) != 0 {
						t.Fatalf("wanted does not advance: %d %v", len(end), err)
					}
				})
				t.Run("rule-consumer", func(t *testing.T) {
					consumer := f
					consumer.pool = pool
					assertLintPayloadRules(t, consumer, p, len(content), &want, want, false)
				})
			})
			t.Run("source-slot", func(t *testing.T) {
				t.Run("bounded-reads", func(t *testing.T) {
					// JSON retains the shared wide bound independently of the target check.
					values, err := reads.SourceValues(f.eng, []uuid.UUID{p}).Next(t.Context())
					if err != nil || len(values) != 1 {
						t.Fatalf("actual source JSON: %d %v", len(values), err)
					}
					specs := []app.SourceSpec{{PageID: p, Key: "sources"}}
					heads, err := r.SourceSlotHeads(t.Context(), f.eng, specs, nil, 512)
					if err != nil || len(heads) != 1 || heads[0].Start != start || heads[0].TargetBytes != int64(len(want)) {
						t.Fatalf("nonempty real slot heads: %+v %v", heads, err)
					}
					stream := reads.SourceSlots(f.eng, specs)
					edges, err := stream.Next(t.Context())
					if err != nil || len(edges) != 1 || edges[0].Target != want || edges[0].Start != start {
						t.Fatalf("complete source-slot growth target: rows=%d %v", len(edges), err)
					}
					end, err := stream.Next(t.Context())
					if err != nil || len(end) != 0 {
						t.Fatalf("slot does not advance: %d %v", len(end), err)
					}
				})
				t.Run("rule-consumer", func(t *testing.T) {
					consumer := f
					consumer.pool = pool
					assertLintPayloadRules(t, consumer, p, len(content), &want, want, false)
				})
			})
			lintEvidence(t, "legal-target-provenance", map[string]any{"source_bytes": len(content), "target_bytes": len(want), "target_key": key, "range_start": start, "trace": tr.names()})
		})
	})
	t.Run("JSON-derived-bound-and-single-large-progress", func(t *testing.T) {
		const c = 5 * 1024 * 1024
		cases := []struct{ name, content, want, invalid string }{
			{"numeric-exponent", "---\nsources: 5e-324\n---\n", "0." + strings.Repeat("0", 323) + "5", "0." + strings.Repeat("0", 323) + "5"},
			{"string-escapes", "---\nsources: \"\\L\\P\\0\\x01\"\n---\n", "\"\u2028\u2029\uFFFD\\u0001\"", `"\u2028\u2029` + "\uFFFD" + `\u0001"`},
			{"alias-expansion", "---\na: &a " + strings.Repeat("x", 50000) + "\nsources: [*a,*a]\n---\n", `["` + strings.Repeat("x", 50000) + `", "` + strings.Repeat("x", 50000) + `"]`, `"` + strings.Repeat("x", 50000) + `"`},
		}
		// One legal JSON payload larger than an ordinary batch: two aliases of
		// a YAML escaped scalar; repeated decoded bytes fit the parser's budget.
		escaped := strings.Repeat(`\x01`, 1100000)
		expanded := strings.Repeat(`\u0001`, 1100000)
		cases = append(cases, struct{ name, content, want, invalid string }{"over-8MiB", "---\na: &a \"" + escaped + "\"\nsources: [*a,*a]\n---\n", `["` + expanded + `", "` + expanded + `"]`, `"` + expanded + `"`})
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if len(tc.content) >= c {
					t.Fatalf("source bytes %d exceed fixed content bound", len(tc.content))
				}
				f := newFixture(t)
				p := uuid.UUID{}
				r := postgresadapter.NewLintReads(f.pool, c)
				reads := app.NewLintReads(r, c)
				lintMeasured(t, "JSON-parser-writer-reads", func() {
					facts := originFacts(t, tc.content)
					if !facts.FrontmatterValid {
						t.Fatal("not legal parser facts")
					}
					f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, facts)
					t.Run("bounded-reads", func(t *testing.T) {
						heads, err := r.SourcePropertyHeads(t.Context(), f.eng, []uuid.UUID{p})
						if err != nil || len(heads) != 1 || heads[0].ValueBytes != int64(len(tc.want)) || heads[0].ValueBytes > app.MaxLintPayloadBytes(c) {
							t.Fatalf("JSON derived bound: %+v %v, want %d bytes", heads, err, len(tc.want))
						}
						stream := reads.SourceValues(f.eng, []uuid.UUID{p})
						values, err := stream.Next(t.Context())
						if err != nil || len(values) != 1 || string(values[0].JSON) != tc.want {
							t.Fatalf("exact handwritten JSON: %d rows %v, wanted bytes %d", len(values), err, len(tc.want))
						}
						end, err := stream.Next(t.Context())
						if err != nil || len(end) != 0 {
							t.Fatalf("large JSON does not advance: %d %v", len(end), err)
						}
						if tc.name == "over-8MiB" && len(values[0].JSON) <= 8*1024*1024 {
							t.Fatal("large witness is not larger than ordinary batch")
						}
						lintEvidence(t, "JSON-provenance", map[string]any{"source_bytes": len(tc.content), "serialized_bytes": heads[0].ValueBytes, "bound": app.MaxLintPayloadBytes(c)})
					})
					t.Run("rule-consumer", func(t *testing.T) {
						assertLintPayloadRules(t, f, p, len(tc.content), nil, tc.invalid, true)
					})
				})
			})
		}
	})
	t.Run("illegal-persisted-target-is-sanitized", func(t *testing.T) {
		f := newFixture(t)
		p := uuid.UUID{}
		r := postgresadapter.NewLintReads(f.pool, 5*1024*1024)
		f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, originFacts(t, "---\nsources: '[[x]]'\n---\n"))
		lintExec(t, f, `UPDATE page_links SET target=repeat('secret',($2::integer/6)+1) WHERE source_id=$1`, p, app.MaxLintPayloadBytes(5*1024*1024))
		_, err := r.WantedEdgeHeads(t.Context(), f.eng, nil, nil, 512)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("unsafe or missing illegal target error: %v", err)
		}
		_, err = r.SourceSlotHeads(t.Context(), f.eng, []app.SourceSpec{{PageID: p, Key: "sources"}}, nil, 512)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("unsafe or missing illegal slot error: %v", err)
		}
		t.Logf("sanitized illegal-payload boundary: %s", fmt.Sprint(err))
	})
}
