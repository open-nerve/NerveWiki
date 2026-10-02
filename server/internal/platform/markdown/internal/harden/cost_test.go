package harden_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/yuin/goldmark"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/internal/harden"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

// fastest is the least of five runs of converting src, the others carrying
// the machine's noise; a run past limit is not repeated.
func fastest(t *testing.T, m goldmark.Markdown, src string, limit time.Duration) time.Duration {
	t.Helper()
	best := time.Duration(1 << 62)
	for range 5 {
		var b bytes.Buffer
		start := time.Now()
		if err := m.Convert([]byte(src), &b); err != nil {
			t.Fatal(err)
		}
		if best = min(best, time.Since(start)); best > limit {
			break
		}
	}
	return best
}

// Every pathological input costs about what an ordinary document of its
// size costs, and twice the input about twice as much: goldmark's own parse
// takes seconds on most of these at 256 KB. The half is tried first, so a
// slow input fails without trying the full size.
func TestThePathologicalInputsCostAboutTheirSize(t *testing.T) {
	if raceEnabled {
		t.Skip("costs are checked without the race detector (make test runs it)")
	}
	const size = 256 << 10
	m := harden.Hardened()
	normal := fastest(t, m, markdowntest.Normal(size), time.Second)
	t.Logf("ordinary, %d KB: %v", size>>10, normal)
	for _, in := range markdowntest.Pathological() {
		half := fastest(t, m, in.Make(size/2), 10*normal)
		if half > 10*normal {
			t.Errorf("%s: %v for %d KB, more than ten times an ordinary document's %v for twice as much",
				in.Name, half, size>>11, normal)
			continue
		}
		full := fastest(t, m, in.Make(size), 10*normal)
		t.Logf("%s: %v, %v", in.Name, half, full)
		switch {
		case full > 10*normal:
			t.Errorf("%s: %v for %d KB, more than ten times an ordinary document's %v", in.Name, full, size>>10, normal)
		case full > 3*half+time.Millisecond:
			t.Errorf("%s: %v for %d KB, more than three times the %v for half of it", in.Name, full, size>>10, half)
		}
	}
}
