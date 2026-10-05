package domain_test

import (
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

// The rules of writing a link again the fixture set's cases leave (M6/P4
// design 2, 3).
func TestARewriteWritesEachLinkAsTheRulesSay(t *testing.T) {
	m, err := markdown.New([]markdown.Extension{tasks.Extension(), obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	stolen := renameCase{Pages: []string{"P", "Q", "src"}, Aliases: map[string][]string{"P": {"x"}}, From: "Q", To: "x"}
	deep := renameCase{Pages: []string{"F", "F/Deep", "src"}, From: "F/Deep", To: "F/Deeper"}
	for _, tt := range []struct {
		name          string
		c             renameCase
		content, want string
	}{
		{
			"a title that ends with .md, its stem another page's title: a wikilink its path and .md, which a Markdown link's .md is",
			renameCase{Pages: []string{"A", "A/x.md", "B", "B/x", "C", "src"}, From: "A/x.md", To: "C/x.md"},
			"[[A/x.md.md]] [t](A/x.md.md)\n", "[[C/x.md.md]] [t](x.md.md)\n",
		},
		{
			"an alias in a table's cell: its display text after \\|",
			stolen, "| h |\n| --- |\n| [[x]] |\n", "| h |\n| --- |\n| [[P\\|x]] |\n",
		},
		{
			"an alias with an anchor: the display text after it; an embed's has none",
			stolen, "[[x#h]] ![[x]] [t](x)\n", "[[P#h|x]] ![[P]] [t](P.md)\n",
		},
		{
			"a display text in single quotes: each ' written twice",
			renameCase{Pages: deep.Pages, From: "F/Deep", To: "F/Deep's"},
			"---\nr: '[[F/Deep|Deep]]'\n---\n", "---\nr: '[[Deep''s|Deep''s]]'\n---\n",
		},
		{
			"a Markdown link's text that is the page's path or title: its new title",
			deep, "[F/Deep](F/Deep.md) [Deep](F/Deep.md) [ Deep ](F/Deep.md) [x](F/Deep.md)\n",
			"[Deeper](Deeper.md) [Deeper](Deeper.md) [ Deeper ](Deeper.md) [x](Deeper.md)\n",
		},
		{
			"a rename of the case only: a link by an alias, and one written as now, as they were",
			renameCase{Pages: []string{"Old", "src"}, Aliases: map[string][]string{"Old": {"al"}}, From: "Old", To: "old"},
			"[[al]] [[old]] [[Old]]\n", "[[al]] [[old]] [[old]]\n",
		},
		{
			"a link that was ambiguous, and leads to its page after: as it was",
			renameCase{Pages: []string{"A", "A/x", "B", "B/x", "C", "C/y", "src"}, From: "C/y", To: "C/z"},
			"[[x]]\n", "[[x]]\n",
		},
		{
			"a link that leads to its page after, but ambiguously: its page's path",
			renameCase{Pages: []string{"A", "A/x", "C", "C/y", "src"}, From: "C/y", To: "C/x"},
			"[[x]] [t](x.md)\n", "[[A/x]] [t](A/x.md)\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got, left := rewrite(t, m, tt.c, tt.content); got != tt.want || len(left) > 0 {
				t.Errorf("written\n%q\nwant\n%q\nleft %+v", got, tt.want, left)
			}
		})
	}
}

// A link whose page the tree lacks is left, as it is: the caller logs it.
func TestALinkWhosePageTheTreeLacksIsLeft(t *testing.T) {
	var id uuid.UUID
	id[15] = 1
	link := domain.Link{Kind: "wikilink", Target: "x", Start: 2, End: 3}
	edits, left := domain.Rewrite("[[x]]\n", nil, []domain.Resolved{{Link: link, Before: domain.Resolution{ID: id}}}, domain.Tree{}, uuid.UUID{})
	if len(edits) != 0 || len(left) != 1 || left[0] != link {
		t.Errorf("edits %+v, left %+v", edits, left)
	}
}
