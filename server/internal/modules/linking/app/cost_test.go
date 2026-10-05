//go:build !race

package app_test

import (
	"fmt"
	"slices"
	"testing"
	"time"
)

// A page that writes one link many times resolves it once against the
// pages of its name, not once a link (P3B review M1): 100 000 links to a
// name a thousand pages have resolve in well under a second, where one
// resolution a link took five. The race detector slows the code too much
// for a bound in time: make test-go runs it in a build without.
func TestALinkWrittenManyTimesResolvesOnce(t *testing.T) {
	w := newWorld(t, "src")
	for i := range 1000 {
		w.tree.add(fmt.Sprintf("f%d", i))
		w.tree.add(fmt.Sprintf("f%d/Index", i))
	}
	start := time.Now()
	got := w.view("src", 0, slices.Repeat([]string{"Index"}, 100_000)...)
	if took := time.Since(start); took > time.Second {
		t.Errorf("resolving took %s", took)
	}
	first := got[0]
	if !first.Ambiguous || len(got) != 100_000 {
		t.Fatalf("the first link resolves to %+v, of %d", first, len(got))
	}
	for start, r := range got {
		if r != first {
			t.Fatalf("the link at %d resolves to %+v, the first to %+v", start, r, first)
		}
	}
}
