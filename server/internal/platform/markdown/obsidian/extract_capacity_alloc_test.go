//go:build !race

package obsidian

import (
	"runtime"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// These byte ceilings bound the actual extractor on dense facts and on the
// shapes where speculative capacity must not amplify allocation. Each sample
// first checks every fact. This !race guard also runs under -short.
func TestExtractCapacityAllocationBudget(t *testing.T) {
	const n = 10000
	const header = "---\np: '[[prop]]'\n---\n"
	propertyWant := capacityDenseFacts(n, len(header))
	propertyWant.Links = append([]Link{{Kind: KindWikilink, Target: "prop", Key: "p", Range: markdown.Span{Start: 10, Stop: 14}}}, propertyWant.Links...)
	for _, tc := range []struct {
		name, content string
		want          Extracted
		maxBytes      uint64
	}{
		{"dense", strings.Repeat("[](aa.a)", n), capacityDenseFacts(n, 0), 1_500_000},
		{"external", strings.Repeat("[](https://example.test)", n), Extracted{}, 400_000},
		{"anchor_only", strings.Repeat("[](#here)", n), Extracted{}, 128_000},
		{"reference_duplicates", strings.Repeat("[a][r]\n", n) + "\n[r]: target\n", Extracted{Links: []Link{
			{Kind: KindLink, Target: "target", Range: markdown.Span{Start: 7*n + 6, Stop: 7*n + 12}},
		}}, 180_000},
		{"one_property_after_dense", header + strings.Repeat("[](aa.a)", n), propertyWant, 3_500_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, values, original := extractCapacityTree(t, tc.content)
			equalCapacityFacts(t, original, tc.want)
			for sample := range 3 {
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				got := extract(tree, values).(Extracted)
				runtime.ReadMemStats(&after)
				equalCapacityFacts(t, got, tc.want)
				runtime.KeepAlive(got)
				t.Logf("EXTRACTCAPACITY case=%s sample=%d bytes=%d allocs=%d facts=%d cap=%d", tc.name, sample, after.TotalAlloc-before.TotalAlloc, after.Mallocs-before.Mallocs, len(got.Links), cap(got.Links))
				if used := after.TotalAlloc - before.TotalAlloc; used > tc.maxBytes {
					t.Errorf("complete extraction %s sample %d allocated %d bytes, exceeds %d", tc.name, sample, used, tc.maxBytes)
				}
			}
			runtime.KeepAlive(tree)
			if string(tree.Content) != tc.content {
				t.Fatal("measurement changed source bytes")
			}
		})
	}
}
