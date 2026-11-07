package obsidian_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

// Every written target is kept, in content order, across a large page. A
// shared reference definition is taken once, as its first use's kind;
// the frontmatter's link precedes the body's even though extracted last.
func TestTheLinksOfALargePageAreCompleteAndInContentOrder(t *testing.T) {
	var src strings.Builder
	src.WriteString("---\nsource: '[[front#top|Front]]'\n---\n")
	want := obsidian.Extracted{
		Links: []obsidian.Link{{
			Kind: obsidian.KindWikilink, Target: "front", Anchor: "top", Display: "Front", Key: "source",
			Range: markdown.Span{Start: 15, Stop: 20}, InTable: false,
		}},
	}
	tagAt := src.Len()
	src.WriteString("#kept\n\n")
	want.Tags = []obsidian.Tag{{Name: "kept", Range: markdown.Span{Start: tagAt, Stop: tagAt + len("#kept")}}}
	src.WriteString("[first][shared]\n\n")

	// Each expected range is fixed by the bytes written here. No parse,
	// extraction, or sorting is used to construct the expected facts.
	for i := range 513 {
		target := fmt.Sprintf("body-%03d", i)
		anchor := fmt.Sprintf("h-%03d", i)
		display := fmt.Sprintf("d-%03d", i)
		kind, wantDisplay, inTable := obsidian.KindWikilink, display, false
		prefix, suffix := "[[", "#"+anchor+"|"+display+"]]\n"
		switch i % 4 {
		case 1:
			kind = obsidian.KindEmbed
			prefix = "![["
		case 2:
			kind, wantDisplay = obsidian.KindLink, ""
			prefix, suffix = "["+display+"](", "#"+anchor+")\n"
		case 3:
			kind, wantDisplay = obsidian.KindImage, ""
			prefix, suffix = "!["+display+"](", "#"+anchor+")\n"
		}
		if i == 256 {
			src.WriteString("![again][shared]\n\n")
			prefix = "\n| h |\n| --- |\n| [["
			suffix = "#" + anchor + "\\|" + display + "]] |\n\n"
			inTable = true
		}
		at := src.Len() + len(prefix)
		src.WriteString(prefix)
		src.WriteString(target)
		src.WriteString(suffix)
		want.Links = append(want.Links, obsidian.Link{
			Kind: kind, Target: target, Anchor: anchor, Display: wantDisplay, Key: "",
			Range: markdown.Span{Start: at, Stop: at + len(target)}, InTable: inTable,
		})
	}

	const referencePrefix = "\n[shared]\n\n[shared]: <"
	referenceAt := src.Len() + len(referencePrefix)
	src.WriteString(referencePrefix + "shared-target#shared-anchor>\n")
	want.Links = append(want.Links, obsidian.Link{
		Kind: obsidian.KindLink, Target: "shared-target", Anchor: "shared-anchor", Display: "", Key: "",
		Range: markdown.Span{Start: referenceAt, Stop: referenceAt + len("shared-target")}, InTable: false,
	})

	got := extracted(newMarkdown(t), src.String())
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extracted %+v\nwant %+v", got, want)
	}
}
