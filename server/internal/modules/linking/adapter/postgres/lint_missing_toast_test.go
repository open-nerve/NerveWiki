//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"runtime"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func TestLintJSONToastMissingProperties(t *testing.T) {
	if testing.Short() {
		t.Skip("necessary type/summary values and physical TOAST controls")
	}
	// Incompressible-enough whitespace stays a real external TOAST value under
	// production EXTENDED storage. The last character can decide a non-result.
	trim := []rune{'\t', '\n', '\v', '\f', '\r', ' ', 0x00a0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff}
	rng := rand.New(rand.NewPCG(20261030, 2))
	var white strings.Builder
	white.WriteString(strings.Repeat(" ", 257))
	for range 128 * 1024 {
		white.WriteRune(trim[rng.IntN(len(trim))])
	}
	quote := func(s string) string {
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	for _, tc := range []struct {
		name, payload                 string
		missing, necessary, unrelated bool
	}{
		{"prefix-sufficient", "x" + white.String(), false, false, false},
		{"long-whitespace-then-text", white.String() + "x", false, true, false},
		{"long-whitespace-only", white.String(), true, true, false},
		{"unrelated-only", white.String() + "x", true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			literal := quote(tc.payload)
			content := "---\ntype: " + literal + "\nsummary: " + literal + "\nother: " + literal + "\n---\n"
			if tc.unrelated {
				content = "---\nother: " + literal + "\n---\n"
			}
			if len(content) >= 5*1024*1024 {
				t.Fatalf("not a legal page: %d bytes", len(content))
			}
			facts := originFacts(t, content)
			if !facts.FrontmatterValid || len(facts.Properties) == 0 {
				t.Fatal("real parser did not produce valid properties")
			}
			var ids []uuid.UUID
			for n := uint64(0); n < 8; n++ {
				id := lintID(n)
				ids = append(ids, id)
				f.replace(t, app.Page{ID: id, NotebookID: f.eng, Revision: 1}, facts)
			}
			var storedJSON, storedValue string
			if err := f.pool.QueryRow(t.Context(), `SELECT value::text,value #>> '{}' FROM page_properties WHERE source_id=$1 AND position=0`, ids[0]).Scan(&storedJSON, &storedValue); err != nil {
				t.Fatal(err)
			}
			if storedValue != tc.payload {
				t.Fatalf("parser/writer changed payload: %d bytes want %d", len(storedValue), len(tc.payload))
			}
			var toastBytes int64
			if err := f.pool.QueryRow(t.Context(), `SELECT pg_relation_size(reltoastrelid) FROM pg_class WHERE oid='page_properties'::regclass`).Scan(&toastBytes); err != nil {
				t.Fatal(err)
			}
			if toastBytes == 0 {
				t.Fatal("fixture has no physically stored external property values")
			}
			for _, calls := range lintPageRules(f.s)[2:] {
				t.Run(calls.name, func(t *testing.T) {
					for _, part := range []string{"count", "first", "after"} {
						t.Run(part, func(t *testing.T) {
							in := app.PageRuleInput{NotebookID: f.eng, CandidateIDs: ids}
							expected := 0
							if tc.missing {
								expected = 8
							}
							if part == "after" && tc.missing {
								expected = 4
							}
							var before, after runtime.MemStats
							runtime.ReadMemStats(&before)
							started := time.Now()
							blocks := lintToast(t, f, "page_properties", func(ctx context.Context) error {
								if part == "count" {
									n, err := calls.count(ctx, in)
									if err == nil && n != (domain.Count{Value: int64(expected)}) {
										return fmt.Errorf("actual count %+v want %d", n, expected)
									}
									return err
								}
								var cursor *uuid.UUID
								if part == "after" {
									cursor = &ids[3]
								}
								got, err := calls.items(ctx, in, cursor, 101)
								if err == nil && len(got) != expected {
									return fmt.Errorf("actual %s rows %d want %d", part, len(got), expected)
								}
								return err
							})
							elapsed := time.Since(started)
							runtime.ReadMemStats(&after)
							if tc.necessary && blocks <= 0 {
								t.Fatal("necessary full JSON branch read no actual TOAST blocks")
							}
							if !tc.necessary && blocks != 0 {
								t.Fatalf("short prefix/absent key touched unnecessary TOAST: %d blocks", blocks)
							}
							rows := 8
							if part == "after" {
								rows = 4
							}
							required := 0
							if tc.necessary {
								required = rows * len(storedJSON)
							}
							lintEvidence(t, "missing-property-read", map[string]any{"rule": calls.name, "statement": part, "page_source_bytes": len(content), "single_JSON_bytes": len(storedJSON), "single_value_UTF8_bytes": len(storedValue), "necessary_JSON_bytes": required, "reader_pages": rows, "returned": expected, "stored_TOAST_bytes": toastBytes, "read_TOAST_blocks": blocks, "wall_ns": elapsed.Nanoseconds(), "go_total_alloc": after.TotalAlloc - before.TotalAlloc, "scope": "reader-only count/items in caller snapshot on warm production-write fixture; parser/production writer/fixture strings excluded; PostgreSQL memory not measured"})
						})
					}
				})
			}
		})
	}
}
