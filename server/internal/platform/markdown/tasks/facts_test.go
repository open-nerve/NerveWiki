package tasks_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

// The task items outlive the tree: a write keeps them through its unit
// (M6 design 4.7).
func TestTheTasksOutliveTheTree(t *testing.T) {
	markdowntest.CheckFacts(t, []markdown.Extension{tasks.Extension()})
}
