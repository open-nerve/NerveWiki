package obsidian_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

// What the dialect takes outlives the tree: a write keeps it through its
// unit (M6 design 4.7).
func TestTheDialectsFactsOutliveTheTree(t *testing.T) {
	markdowntest.CheckFacts(t, []markdown.Extension{obsidian.Extension()})
}
