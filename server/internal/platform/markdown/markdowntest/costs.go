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
// 4.4 and 10.3, the most allocated by HTML blocks, each a tokenizer of
// 4 KB).
const (
	k      = 10
	kAlloc = 14
)

// factsSlack is the heap a parse's Facts may keep beyond their Limit: the
// noise of the heap's accounting, some hundreds of bytes once two
// collections have emptied the pools (kept).
const factsSlack = 1 << 10

// factsSizes are the sizes the facts of every pathological input are
// measured at: below the YAML's limit of values, where a frontmatter keeps
// the most for its size (M6/P2 fix check M-1), and where a small content's
// slices grow by doubling (M6/P2 fix check 2 L2).
var factsSizes = []int{16 << 10, 1 << 10} //nolint:gochecknoglobals // read only

// sweep are the sizes the dense inputs' facts are measured at too: from 256
// bytes to 8 KB, each 5% more than the last, where a slice that appending
// left up to twice as long as what it holds would show (M6/P2 fix check 3
// M1).
func sweep() []int {
	var out []int
	for n := 256.0; n <= 8<<10; n *= 1.05 {
		out = append(out, int(n))
	}
	return out
}

// fastest is the least of three runs of parsing and rendering src, the
// others carrying the machine's noise; a run past limit is not repeated.
// Each starts from a collected heap: what the collector of the one before
// left to do is no part of it.
func fastest(t *testing.T, m *markdown.Markdown, src string, limit time.Duration) time.Duration {
	t.Helper()
	content := []byte(src)
	best := time.Duration(math.MaxInt64)
	for range 3 {
		runtime.GC()
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

// checkKept checks that the facts of content keep at most their Limit, and
// answers what part of it they keep.
func checkKept(t *testing.T, m *markdown.Markdown, name string, content []byte) float64 {
	t.Helper()
	f, limit := kept(m, content)
	if f > uint64(limit)+factsSlack {
		t.Errorf("%s: its facts keep %d bytes for %d, more than their limit of %d", name, f, len(content), limit)
	}
	return float64(f) / float64(limit)
}

// logKept checks the facts of content as checkKept does, and logs them.
func logKept(t *testing.T, m *markdown.Markdown, name string, content []byte) {
	t.Helper()
	f, limit := kept(m, content)
	if f > uint64(limit)+factsSlack {
		t.Errorf("%s: its facts keep %d bytes for %d, more than their limit of %d", name, f, len(content), limit)
		return
	}
	t.Logf("%s, %d bytes: its facts keep %d bytes, %.1f times the content, %.2f of their limit; %d to spare", name,
		len(content), f, float64(f)/float64(len(content)), float64(f)/float64(limit), int64(limit)-int64(f))
}

// CheckCosts checks what m costs (M4/P3 design 3.10): every pathological
// input of 512 KB at most k times what an ordinary document of its size
// costs (goldmark's own parse takes seconds on most of these at 256 KB),
// and at most eight times what a quarter of it costs: a linear cost is
// four times, a quadratic one sixteen. A linear cost grows faster than
// its size on small inputs, which the machine's load sways the most:
// twice 128 KB took three times as long, against a bound of three, where
// four times 128 KB takes at most about five and a half. An ordinary
// megabyte in a second is a floor against a slowdown by an order of
// magnitude. The
// quarter is tried first, so a slow input fails without trying the full
// size. What an input allocates
// is checked too, at most kAlloc times an ordinary document's: the count
// does not vary as the time does, and a cost that the garbage collector
// takes later shows in it (a tokenizer for each of thousands of tags
// allocates hundreds of megabytes, and takes only a few times longer).
// Each input's HTML is checked against CheckSize, which no machine's load
// sways, the Amplifying inputs' at AmplifyingSize; and what its Facts keep
// once the parse is done, at most their Limit, which the budget counts them
// as (M6/P2 review M1): the pathological inputs' at 512 KB and at
// factsSizes, the Amplifying inputs' at AmplifyingSize.
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
		logKept(t, m, in.Name, content)
	}
	const size = 512 << 10
	normal := fastest(t, m, Normal(size), time.Second)
	normalAlloc, _ := allocated(t, m, []byte(Normal(size)))
	t.Logf("ordinary, %d KB: %v, %d KB allocated", size>>10, normal, normalAlloc>>10)
	for _, in := range Pathological() {
		quarter := fastest(t, m, in.Make(size/4), k*normal)
		if quarter > k*normal {
			t.Errorf("%s: %v for %d KB, more than %d times an ordinary document's %v for four times as much",
				in.Name, quarter, size>>12, k, normal)
			continue
		}
		full := fastest(t, m, in.Make(size), k*normal)
		content := []byte(in.Make(size))
		alloc, out := allocated(t, m, content)
		t.Logf("%s: %v, %v (%.1f); allocated %.1f", in.Name, quarter, full, float64(full)/float64(normal),
			float64(alloc)/float64(normalAlloc))
		if err := CheckSize(content, out); err != nil {
			t.Errorf("%s: %v", in.Name, err)
		}
		logKept(t, m, in.Name, content)
		for _, n := range factsSizes {
			logKept(t, m, in.Name, []byte(in.Make(n)))
		}
		if alloc > kAlloc*normalAlloc {
			t.Errorf("%s: %d KB allocated for %d KB, more than %d times an ordinary document's %d KB",
				in.Name, alloc>>10, size>>10, kAlloc, normalAlloc>>10)
		}
		switch {
		case full > k*normal:
			t.Errorf("%s: %v for %d KB, more than %d times an ordinary document's %v", in.Name, full, size>>10, k, normal)
		case full > 8*quarter+time.Millisecond:
			t.Errorf("%s: %v for %d KB, more than eight times the %v for a quarter of it", in.Name, full, size>>10, quarter)
		}
	}
	for _, in := range dense() {
		worst, at := 0.0, 0
		for _, n := range sweep() {
			content := []byte(in.Make(n))
			if part := checkKept(t, m, in.Name, content); part > worst {
				worst, at = part, len(content)
			}
		}
		t.Logf("%s, from 256 bytes to 8 KB: its facts keep at most %.2f of their limit, at %d bytes", in.Name, worst, at)
	}
	if d := fastest(t, m, Normal(1<<20), time.Second); d > time.Second {
		t.Errorf("an ordinary megabyte: %v", d)
	}
}
