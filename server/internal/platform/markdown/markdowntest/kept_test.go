package markdowntest

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// kept reads what the facts keep (M6/P2 fix check 4 L3): an extension that
// takes 64 KB for each parse keeps them, which a reading that lost the facts
// before the heap with them is read would not see. Each OS thread the
// runtime starts between the readings keeps some 5 KB, most of them in a
// process's first collections (fix checks 5 M1, 6 L1): a first reading is
// thrown away, and the rest are within 8 KB either way.
func TestKeptReadsWhatTheFactsKeep(t *testing.T) {
	const ballast = 64 << 10
	m, err := markdown.New([]markdown.Extension{{Name: "ballast", Extract: func(markdown.Tree) any {
		return make([]byte, ballast)
	}}})
	if err != nil {
		t.Fatal(err)
	}
	kept(m, []byte("a\n"))
	if f, _ := kept(m, []byte("a\n")); f < ballast-8<<10 || f > ballast+8<<10 {
		t.Errorf("the facts of an extension keeping 64 KB read %d bytes", f)
	}
}
