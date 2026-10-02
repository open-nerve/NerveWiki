package markdown_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

func TestTheCostsAreAboutTheSize(t *testing.T) {
	markdowntest.CheckCosts(t, newMarkdown(t))
}
