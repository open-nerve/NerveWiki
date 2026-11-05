//go:build !race

package postgresadapter_test

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
)

const bufferedRebuildState = "buffered_rebuild_after_analyzed"

type bufferedPlanBoundaryKey struct{}
type bufferedPlanBoundary struct {
	SQL, CommandTag, Error string
	PID                    uint32
}

// These fixtures use the real Store, Parser, TxManager and index lock, with
// small notebook/content/path ports. This is not a bootstrap cost/row-lock test.
// The seed's live version plus a single final INSERT gives a two-version upper
// bound. Pruning may remove a dead entry; it cannot create an UPDATE version.
func bufferedRebuildHistory(t *testing.T, trace *stablePlanTrace, calls []stablePlanCall, pageRows map[uuid.UUID]int) {
	t.Helper()
	trace.mu.Lock()
	boundaries := slices.Clone(trace.boundaries)
	trace.mu.Unlock()
	rows, counts := map[string]int64{}, map[string]int{}
	var commands []map[string]any
	for _, c := range calls {
		rows[c.Name] += c.RowsAffected
		counts[c.Name]++
		commands = append(commands, map[string]any{"name": c.Name, "kind": c.Kind, "sql_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(c.SQL))), "argument_sha256": c.ArgumentSHA, "argument_types": c.ArgumentTypes, "leading_options": c.LeadingOptions, "default_mode": c.DefaultMode, "statement_capacity": c.StatementCapacity, "description_capacity": c.DescriptionCapacity, "pid": c.PID, "command_tag": c.CommandTag, "rows": c.RowsAffected, "error": c.Error})
	}
	links := 0
	for _, n := range pageRows {
		links += n
	}
	// All completed query identities/tags precede assertions; no large arrays
	// are printed. Existing literal post-images independently check their values.
	lintEvidence(t, "buffered-rebuild-history", map[string]any{"state": bufferedRebuildState, "pages": len(pageRows), "literal_links": links, "source_versions_upper": 2 * links, "source_versions_derivation": "one live seed + one final INSERT per key; no resolution UPDATE", "commands": commands, "boundaries": boundaries, "call_counts": counts, "affected_rows": rows})
	if len(boundaries) != 2 || boundaries[0].SQL != "begin" || boundaries[0].CommandTag != "BEGIN" || boundaries[1].SQL != "commit" || boundaries[1].CommandTag != "COMMIT" || boundaries[0].Error != "" || boundaries[1].Error != "" || boundaries[0].PID == 0 || boundaries[0].PID != boundaries[1].PID || !slices.Equal(trace.terminal(), []string{"commit"}) {
		t.Fatalf("BUFFERED_HISTORY actual transaction incomplete: boundaries=%+v terminal=%v", boundaries, trace.terminal())
	}
	wantNames := []string{"LockNotebook", "DeleteNotebooksLinks", "DeleteNotebooksTags", "DeleteNotebooksProperties", "DeleteNotebooksAliases", "DeleteNotebooksIndexedPages"}
	for range len(pageRows) {
		wantNames = append(wantNames, "InsertIndexedPage", "InsertTags", "InsertProperties", "InsertAliases")
	}
	wantNames = append(wantNames, "AliasesByKeys")
	for range len(pageRows) {
		wantNames = append(wantNames, "InsertRebuiltLinks")
	}
	var names []string
	for _, c := range calls {
		names = append(names, c.Name)
		if c.Error != "" || c.CommandTag == "" || c.PID != boundaries[0].PID {
			t.Fatalf("BUFFERED_HISTORY query=%s pid=%d tag=%q error=%s", c.Name, c.PID, c.CommandTag, c.Error)
		}
	}
	if !slices.Equal(names, wantNames) || rows["DeleteNotebooksLinks"] != int64(links) || rows["DeleteNotebooksIndexedPages"] != int64(len(pageRows)) || rows["InsertIndexedPage"] != int64(len(pageRows)) || rows["InsertRebuiltLinks"] != int64(links) {
		t.Fatalf("BUFFERED_HISTORY single final insert: counts=%v rows=%v expected_pages=%d links=%d exact_sequence=%t", counts, rows, len(pageRows), links, slices.Equal(names, wantNames))
	}
	heads, inserted := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, c := range calls {
		if c.Name != "InsertIndexedPage" && c.Name != "InsertRebuiltLinks" {
			continue
		}
		if len(c.Args) == 0 {
			t.Fatal("BUFFERED_HISTORY missing source argument")
		}
		id, ok := c.Args[0].(uuid.UUID)
		n, exists := pageRows[id]
		if !ok || !exists {
			t.Fatalf("BUFFERED_HISTORY unexpected source %v in %s", c.Args[0], c.Name)
		}
		if c.Name == "InsertIndexedPage" {
			if heads[id] || c.RowsAffected != 1 || c.CommandTag != "INSERT 0 1" {
				t.Fatalf("BUFFERED_HISTORY repeated/missing head %s", id)
			}
			heads[id] = true
			continue
		}
		if len(heads) != len(pageRows) || inserted[id] || len(c.Args) != 16 || c.RowsAffected != int64(n) || c.CommandTag != fmt.Sprintf("INSERT 0 %d", n) {
			t.Fatalf("BUFFERED_HISTORY final page=%s args=%d tag=%q rows=%d want=%d heads=%d/%d repeated=%t", id, len(c.Args), c.CommandTag, c.RowsAffected, n, len(heads), len(pageRows), inserted[id])
		}
		for i, arg := range c.Args[2:] {
			v := reflect.ValueOf(arg)
			if v.Kind() != reflect.Slice || v.Len() != n {
				t.Fatalf("BUFFERED_HISTORY page=%s array%d=%T want_length=%d", id, i+3, arg, n)
			}
		}
		inserted[id] = true
	}
	if len(inserted) != len(pageRows) {
		t.Fatalf("BUFFERED_HISTORY final sources=%d want=%d", len(inserted), len(pageRows))
	}
}

// Derive the new tuple ledger from the fixture's independently built complete
// literal links (already compared to actual Store.Links), not measured work.
func bufferedNovelLedger(t *testing.T, links []app.Link, q app.IngestCandidateQuery) {
	t.Helper()
	wanted := map[uuid.UUID]bool{novelRangeID(1): true, novelRangeID(2): true, novelRangeID(3): true, novelRangeID(4): true}
	eligible := map[uuid.UUID]bool{}
	for _, id := range q.ParentIDs {
		wanted[id] = true
	}
	for _, id := range q.EligibleEmitters {
		eligible[id] = true
	}
	followed, relevant, sources := 0, 0, 0
	for _, l := range links {
		if l.Resolution.ID == novelRangeID(1) || l.Resolution.ID == novelRangeID(2) {
			followed++
		}
		if wanted[l.Resolution.ID] {
			relevant++
		}
		if eligible[l.SourceID] {
			sources++
		}
	}
	// Eight foreign reverse entries remain once; the main notebook is rebuilt.
	bound := 2*followed + 8 + 2*relevant + 8 + 2*sources
	if len(links) != 52117 || followed != 3 || relevant != 404 || sources != 916 || bound != 2662 {
		t.Fatalf("BUFFERED_NOVEL literal range ledger links/followed/relevant/sources/bound=%d/%d/%d/%d/%d", len(links), followed, relevant, sources, bound)
	}
	lintEvidence(t, "buffered-novel-version-derivation", map[string]int{"main_links": len(links), "main_followed": followed, "main_relevant": relevant, "main_complete_sources": sources, "foreign_reverse_per_scope": 8, "followed_versions": 2*followed + 8, "relevant_versions": 2*relevant + 8, "source_versions": 2 * sources, "total_versions_upper": bound, "planning_endpoint_versions_upper": 4})
}

// Only the new heap history is added: the existing no-statistics, analyzed,
// streaming-history, boundary, TOAST and prepared-history matrices stay intact.
func TestRebuildBufferedPhysicalPlans(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL default buffered rebuild physical observation")
	}
	t.Run("set", func(t *testing.T) {
		first, tail := setPlanParameters(t)
		for _, mode := range []string{"auto", "force_custom_plan", "force_generic_plan"} {
			t.Run(mode, func(t *testing.T) {
				for _, shape := range []struct {
					name string
					s    statement
				}{{"first4096", first}, {"tail1", tail}} {
					t.Run(shape.name, func(t *testing.T) {
						for _, operation := range []string{"exec", "explain"} {
							t.Run(operation, func(t *testing.T) { setPlanRun(t, bufferedRebuildState, mode, operation, shape.s) })
						}
					})
				}
			})
		}
	})
	t.Run("stable", func(t *testing.T) {
		specs := stablePlanMainSpecs()
		captured := stablePlanCapture(t, specs)
		for _, mode := range []string{"auto", "force_custom_plan", "force_generic_plan"} {
			t.Run(mode, func(t *testing.T) {
				for _, operation := range []string{"execute", "explain"} {
					t.Run(operation, func(t *testing.T) {
						var real *stablePlanFixture
						if operation == "execute" {
							real = newStablePlanFixture(t, bufferedRebuildState, specs, true)
						}
						kernel := newStablePlanFixture(t, bufferedRebuildState, specs, true)
						for i, s := range specs {
							t.Run(s.Shape, func(t *testing.T) {
								if real != nil {
									stablePlanRunStore(t, real, real.specs[i], mode, captured[s.Name], true, true, nil)
								}
								stablePlanRunKernel(t, kernel, kernel.specs[i], mode, operation, captured[s.Name], false, false, nil)
							})
						}
					})
				}
			})
		}
	})
	// lintExecutedMode's existing statistics guard derives analyzed state
	// from this path component. No generic helper or old selector is changed.
	t.Run("novel/analyzed", func(t *testing.T) { novelRangeRun(t, bufferedRebuildState) })
}
