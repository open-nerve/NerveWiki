package markdowntest

import (
	"context"
	"math"
	"runtime"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// k is how many times an ordinary document's time a pathological input of
// its size may take, kAlloc how many times its allocation it may allocate
// (M4/P3 design 3.10; measured in the P3 document's results: at most about
// 6 and 11, the most allocated by HTML blocks, each a tokenizer of 4 KB).
const (
	k      = 10
	kAlloc = 14
)

// fastest is the least of three runs of parsing and rendering src, the
// others carrying the machine's noise; a run past limit is not repeated.
func fastest(t *testing.T, m *markdown.Markdown, src string, limit time.Duration) time.Duration {
	t.Helper()
	content := []byte(src)
	best := time.Duration(math.MaxInt64)
	for range 3 {
		start := time.Now()
		if _, err := m.Render(context.Background(), m.Parse(content), markdown.Page{}); err != nil {
			t.Fatal(err)
		}
		if best = min(best, time.Since(start)); best > limit {
			break
		}
	}
	return best
}

// allocated is what one parse and rendering of content allocates, and the
// HTML.
func allocated(t *testing.T, m *markdown.Markdown, content []byte) (uint64, string) {
	t.Helper()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	out, err := m.Render(context.Background(), m.Parse(content), markdown.Page{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc, out
}

// CheckCosts checks what m costs (M4/P3 design 3.10): every pathological
// input at most k times what an ordinary document of its size costs, and
// twice the input about twice as much (goldmark's own parse takes seconds
// on most of these at 256 KB); an ordinary megabyte in a second, a floor
// against a slowdown by an order of magnitude. The half is tried first, so
// a slow input fails without trying the full size. What an input allocates
// is checked too, at most kAlloc times an ordinary document's: the count
// does not vary as the time does, and a cost that the garbage collector
// takes later shows in it (a tokenizer for each of thousands of tags
// allocates hundreds of megabytes, and takes only a few times longer).
// Each input's HTML is checked against CheckSize, which no machine's load
// sways, the Amplifying inputs' at AmplifyingSize.
// The race detector makes the code several times slower: under it the
// check is skipped, and make test-go runs it in a build without.
func CheckCosts(t *testing.T, m *markdown.Markdown) {
	t.Helper()
	if raceEnabled {
		t.Skip("costs are checked without the race detector (make test-go runs it)")
	}
	for _, in := range Amplifying() {
		content := []byte(in.Make(AmplifyingSize))
		_, out := allocated(t, m, content)
		if err := CheckSize(content, out); err != nil {
			t.Errorf("%s: %v", in.Name, err)
		}
	}
	const size = 256 << 10
	normal := fastest(t, m, Normal(size), time.Second)
	normalAlloc, _ := allocated(t, m, []byte(Normal(size)))
	t.Logf("ordinary, %d KB: %v, %d KB allocated", size>>10, normal, normalAlloc>>10)
	for _, in := range Pathological() {
		half := fastest(t, m, in.Make(size/2), k*normal)
		if half > k*normal {
			t.Errorf("%s: %v for %d KB, more than %d times an ordinary document's %v for twice as much",
				in.Name, half, size>>11, k, normal)
			continue
		}
		full := fastest(t, m, in.Make(size), k*normal)
		content := []byte(in.Make(size))
		alloc, out := allocated(t, m, content)
		t.Logf("%s: %v, %v (%.1f); allocated %.1f", in.Name, half, full, float64(full)/float64(normal),
			float64(alloc)/float64(normalAlloc))
		if err := CheckSize(content, out); err != nil {
			t.Errorf("%s: %v", in.Name, err)
		}
		if alloc > kAlloc*normalAlloc {
			t.Errorf("%s: %d KB allocated for %d KB, more than %d times an ordinary document's %d KB",
				in.Name, alloc>>10, size>>10, kAlloc, normalAlloc>>10)
		}
		switch {
		case full > k*normal:
			t.Errorf("%s: %v for %d KB, more than %d times an ordinary document's %v", in.Name, full, size>>10, k, normal)
		case full > 3*half+time.Millisecond:
			t.Errorf("%s: %v for %d KB, more than three times the %v for half of it", in.Name, full, size>>10, half)
		}
	}
	if d := fastest(t, m, Normal(1<<20), time.Second); d > time.Second {
		t.Errorf("an ordinary megabyte: %v", d)
	}
}
