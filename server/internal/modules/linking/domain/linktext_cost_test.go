//go:build !race

package domain_test

import (
	"fmt"
	"runtime"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The link targets of a notebook whose ten thousand folders each hold a
// page of the same title, which only its path writes, are written in under
// two seconds and 64 MiB, though each one's path is read against all the
// others of its title (M6/P5 design 6): also when the title ends with
// ".md", whose writings are read against the pages of its stem instead
// when there are any, here none or one at the root, which has them
// written with ".md" after the path. Joining the pages of the two keys
// anew for each writing allocated GBs (review r1-3, r2-L3, c4). The race
// detector slows the code too much for a bound in time: make test-go runs
// it in a build without.
func TestTheLinktextsOfManyPagesOfOneTitleAreCheap(t *testing.T) {
	const folders = 10_000
	for _, tt := range []struct {
		title, root, want string
	}{
		{"index", "", "f%d/index"},
		{"index.md", "", "f%d/index.md"},
		{"x.md", "x", "f%d/x.md.md"},
	} {
		t.Run(tt.title, func(t *testing.T) {
			var nodes []domain.Node
			step := func(i int, name string) domain.Step {
				var id uuid.UUID
				id[0], id[1], id[2], id[3] = byte(len(nodes)>>16), byte(len(nodes)>>8), byte(len(nodes)), byte(i)
				return domain.Step{ID: id, Key: shared.TitleKey(name), Name: name}
			}
			if tt.root != "" {
				r := step(0, tt.root)
				nodes = append(nodes, domain.Node{ID: r.ID, Path: []domain.Step{r}})
			}
			first := len(nodes)
			for i := range folders {
				f := step(1, fmt.Sprintf("f%d", i))
				nodes = append(nodes, domain.Node{ID: f.ID, Path: []domain.Step{f}})
				p := step(2, tt.title)
				nodes = append(nodes, domain.Node{ID: p.ID, Path: []domain.Step{f, p}})
			}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			got := domain.Linktexts(nodes)
			took := time.Since(start)
			runtime.ReadMemStats(&after)
			if allocated := after.TotalAlloc - before.TotalAlloc; took > 2*time.Second || allocated > 64<<20 {
				t.Errorf("the linktexts took %s and %d MiB", took, allocated>>20)
			}
			for i := range folders {
				at := first + 2*i
				if got[at] != fmt.Sprintf("f%d", i) || got[at+1] != fmt.Sprintf(tt.want, i) {
					t.Fatalf("the linktexts of folder %d are %q, %q", i, got[at], got[at+1])
				}
			}
		})
	}
}
