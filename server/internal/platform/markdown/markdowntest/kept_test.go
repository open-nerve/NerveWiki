package markdowntest

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// kept reads what the facts keep (M6/P2 fix check 4 L3): an extension that
// takes 64 KB for each parse keeps them, which a reading that lost the facts
// before the heap with them is read would not see.
func TestKeptReadsWhatTheFactsKeep(t *testing.T) {
	const ballast = 64 << 10
	m, err := markdown.New([]markdown.Extension{{Name: "ballast", Extract: func(markdown.Tree) any {
		return make([]byte, ballast)
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if f, _ := kept(m, []byte("a\n")); f < ballast || f > ballast+8<<10 {
		t.Errorf("the facts of an extension keeping 64 KB read %d bytes", f)
	}
}
