//go:build !race

package app_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// A page that writes one link many times resolves it once against the
// pages of its name, not once a link, however each is written (P3B review
// M1 and its fix check): 100 000 links to a name a thousand pages have,
// each in a case of its own, resolve in well under a second, where one
// resolution a link took five. The race detector slows the code too much
// for a bound in time: make test-go runs it in a build without.
func TestALinkWrittenManyTimesResolvesOnce(t *testing.T) {
	const name = "abcdefghijklmnopq"
	w := newWorld(t, "src")
	for i := range 1000 {
		w.tree.add(fmt.Sprintf("f%d", i))
		w.tree.add(fmt.Sprintf("f%d/%s", i, name))
	}
	targets := make([]string, 100_000)
	for i := range targets {
		// The letters whose bits i has upper-case: 2^17 cases.
		var b strings.Builder
		for bit, c := range name {
			if i&(1<<bit) != 0 {
				c -= 'a' - 'A'
			}
			b.WriteRune(c)
		}
		targets[i] = b.String()
	}
	start := time.Now()
	got := w.view("src", 0, targets...)
	if took := time.Since(start); took > time.Second {
		t.Errorf("resolving took %s", took)
	}
	first := got[0]
	if !first.Ambiguous || len(got) != 100_000 {
		t.Fatalf("the first link resolves to %+v, of %d", first, len(got))
	}
	for at, r := range got {
		if r != first {
			t.Fatalf("the link at %d resolves to %+v, the first to %+v", at, r, first)
		}
	}
}

// Links to distinct paths of pages of one title each look only at the
// pages whose path ends as theirs (M6 closeout FA-I1): 50,000 pages named
// x, each under a page of its own, and a page with a link to each by its
// path resolve in well under a second, where each link read every page
// named x: seconds every time the page was read.
func TestDistinctPathsToPagesOfOneTitleResolveInTimeAsLongAsThey(t *testing.T) {
	const n = 50_000
	w := newWorld(t, "src")
	targets := make([]string, n)
	for i := range n {
		w.tree.add(fmt.Sprintf("g%d", i))
		w.tree.add(fmt.Sprintf("g%d/x", i))
		targets[i] = fmt.Sprintf("g%d/x", i)
	}
	start := time.Now()
	got := w.view("src", 0, targets...)
	if took := time.Since(start); took > time.Second {
		t.Errorf("resolving took %s", took)
	}
	for i := range n {
		if want := w.id(fmt.Sprintf("g%d/x", i)); got[10*i].ID != want || got[10*i].Ambiguous {
			t.Fatalf("[[g%d/x]] resolves to %+v, want %v", i, got[10*i], want)
		}
	}
}
