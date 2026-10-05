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
