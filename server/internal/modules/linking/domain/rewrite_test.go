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
			"a rename of the case only of a title that ends with .md: a link written as now, as it was",
			renameCase{Pages: []string{"x.md", "src"}, From: "x.md", To: "X.md"},
			"[[X.md]] [[x.md]] [t](X.md.md)\n", "[[X.md]] [[X.md]] [t](X.md.md)\n",
		},
		{
			"a title that holds a comment's %%, new or old: a Markdown link's text left, a wikilink's display followed",
			renameCase{Pages: []string{"F", "F/Old", "src"}, From: "F/Old", To: "F/a %% b"},
			"[Old](F/Old.md) [[F/Old|Old]] %% hidden %% shown\n", "[Old](a%20%25%25%20b.md) [[a %% b|a %% b]] %% hidden %% shown\n",
		},
		{
			"a title that held a comment's %%: a Markdown link's text left",
			renameCase{From: "a %% b", To: "New"}, "[a %% b](a%20%25%25%20b.md) %% hidden %%\n", "[a %% b](New.md) %% hidden %%\n",
		},
		{
			"a link that was ambiguous, and leads to its page after: as it was",
			renameCase{Pages: []string{"A", "A/x", "B", "B/x", "C", "C/y", "src"}, From: "C/y", To: "C/z"},
			"[[x]]\n", "[[x]]\n",
		},
		{
			"a destination between angle brackets: its '%' escaped, which the reading decodes",
			renameCase{From: "Old", To: "x%41"}, "[t](<Old.md>) [t](Old.md)\n", "[t](<x%2541.md>) [t](x%2541.md)\n",
		},
		{
			"a display past an escape of a double-quoted string: left, the string's closing too",
			stolen, "---\nr: \"[[x\\x5D\\x5D\"\ns: \"[[x\\x7Cshown]]\"\n---\nSee [[y]].\n",
			"---\nr: \"[[P\\x5D\\x5D\"\ns: \"[[P\\x7Cshown]]\"\n---\nSee [[y]].\n",
		},
		{
			"a Markdown link's text past an escaped bracket: not its title",
			renameCase{From: "Old", To: "New"}, "[x\\[Old](Old.md)\n", "[x\\[Old](New.md)\n",
		},
		{
			"an embed's display, its size or caption: left",
			deep, "![[F/Deep|Deep]] ![[F/Deep|300]]\n", "![[Deeper|Deep]] ![[Deeper|300]]\n",
		},
		{
			"an empty display of a link by an alias: the alias",
			stolen, "[[x|]] [[x| ]]\n", "[[P|x]] [[P| x]]\n",
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

// A rename of the case only writes no link that names the page by its
// title as now written, a title that ends with ".md" too, with ".md" after
// it or not (M6/P4 fix check c1-1): the page is not written again, nor the
// rename refused for its being edited.
func TestACaseOnlyRenameRewritesNoLinkWrittenAsNow(t *testing.T) {
	id := uuid.NewV7()
	at := domain.Resolution{ID: id}
	for _, recased := range []struct {
		name    string
		targets map[string]bool
	}{
		{"X.md", map[string]bool{"X.md": false, "X.md.md": false, "A/X.md": false, "x.md": true, "x.md.md": true, "x.MD": true}},
		{"X", map[string]bool{"X": false, "X.md": false, "X.MD": false, "x": true, "x.md": true}},
	} {
		for target, want := range recased.targets {
			if got := domain.Rewrites(domain.Link{Target: target}, at, at, domain.Recased{ID: id, Name: recased.name}); got != want {
				t.Errorf("a rename to %q: [[%s]] rewrites %t, want %t", recased.name, target, got, want)
			}
		}
	}
}

// A writing read back (Written): a title that the Markdown around a link
// pairs with a '$' or a '`' before it loses links; the text of a Markdown
// link that follows the title is left then, the targets written alone; and
// when that loses links too, the content is left as it is (M6/P4 review
// R1-1). So is one whose writing changes the page's aliases, or the
// targets of its body are written alone, its property links left.
func TestAWritingIsReadBack(t *testing.T) {
	m, err := markdown.New([]markdown.Extension{tasks.Extension(), obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		to, content, want string
		left              int // property links left, the body written
	}{
		{"A$B", "[Old](Old.md)\n", "[Old](A$B.md)\n", 0},
		{"Don`t", "[Old](Old.md) [[Old]]\n", "[Old](Don`t.md) [[Don`t]]\n", 0},
		{"US$", "costs $5 [[Other]] [[Old]]\n", "", 0},
		{"Don`t", "x ` [[Other]] [[Old]]\n", "", 0},
		// A value of the aliases a YAML alias repeats from another key (M6/P4
		// fix check c3 F1): the link is that key's, and writing it changes
		// the aliases; the body is written alone (c5-1).
		{"New", "---\nx: &x '[[Old]]'\naliases: *x\n---\n", "", 0},
		{"New", "---\nx: &x ['[[Old]]']\naliases: *x\n---\n", "", 0},
		{
			"New", "---\nx: &x '[[Old]]'\naliases: [nick, *x]\nup: '[[Old]]'\n---\n[[Old]] [Old](Old.md)\n",
			"---\nx: &x '[[Old]]'\naliases: [nick, *x]\nup: '[[Old]]'\n---\n[[New]] [Old](New.md)\n", 2,
		},
		{"New", "---\naliases: &a ['[[Old]]']\ny: *a\n---\n[[Old]]\n", "---\naliases: &a ['[[Old]]']\ny: *a\n---\n[[New]]\n", 0},
	} {
		c := renameCase{Pages: []string{"Old", "Other", "src"}, From: "Old", To: tt.to}
		got, left := rewrite(t, m, c, tt.content)
		if tt.want == "" {
			tt.want = tt.content
		}
		if got != tt.want || len(left) != tt.left {
			t.Errorf("%q renamed %s: written %q, want %q; left %+v, want %d", tt.content, tt.to, got, tt.want, left, tt.left)
		}
	}
}

// Kept holds each link a writing does not write again as it was, its
// display too, each it writes again to its page, and the page's aliases.
func TestKeptHoldsTheLinksInTheirPlaces(t *testing.T) {
	a := domain.Node{ID: uuid.UUID{15: 1}, Path: []domain.Step{{ID: uuid.UUID{15: 1}, Key: "a", Name: "A"}}}
	tree := domain.Tree{Named: map[string][]domain.Node{"a": {a}}}
	w := domain.Rewriting{Leads: map[int]domain.Node{2: a}}
	aliases := []domain.Alias{{Key: "al", Name: "Al"}}
	was := domain.Facts{
		Links:   []domain.Link{{Kind: "wikilink", Target: "Old", Start: 2}, {Kind: "wikilink", Target: "x", Display: "t", Start: 12}},
		Aliases: aliases,
	}
	for _, tt := range []struct {
		name    string
		now     []domain.Link
		aliases []domain.Alias
		kept    bool
	}{
		{"as written", []domain.Link{{Kind: "wikilink", Target: "A"}, {Kind: "wikilink", Target: "x", Display: "t"}}, aliases, true},
		{"the other's display changed", []domain.Link{{Kind: "wikilink", Target: "A"}, {Kind: "wikilink", Target: "x", Display: "u"}}, aliases, false},
		{"the other's target changed", []domain.Link{{Kind: "wikilink", Target: "A"}, {Kind: "wikilink", Target: "y", Display: "t"}}, aliases, false},
		{"leading elsewhere", []domain.Link{{Kind: "wikilink", Target: "B"}, {Kind: "wikilink", Target: "x", Display: "t"}}, aliases, false},
		{"of another kind", []domain.Link{{Kind: "embed", Target: "A"}, {Kind: "wikilink", Target: "x", Display: "t"}}, aliases, false},
		{"one fewer", []domain.Link{{Kind: "wikilink", Target: "A"}}, aliases, false},
		{"the aliases changed", []domain.Link{{Kind: "wikilink", Target: "A"}, {Kind: "wikilink", Target: "x", Display: "t"}}, nil, false},
	} {
		if got := w.Kept(was, domain.Facts{Links: tt.now, Aliases: tt.aliases}, nil, tree); got != tt.kept {
			t.Errorf("%s: kept %t, want %t", tt.name, got, tt.kept)
		}
	}
}

// A link whose page the tree lacks is left, as it is: the caller logs it.
func TestALinkWhosePageTheTreeLacksIsLeft(t *testing.T) {
	var id uuid.UUID
	id[15] = 1
	link := domain.Link{Kind: "wikilink", Target: "x", Start: 2, End: 3}
	w := domain.Rewrite("[[x]]\n", nil, []domain.Resolved{{Link: link, Before: domain.Resolution{ID: id}}}, domain.Tree{}, domain.Recased{})
	if len(w.Edits) != 0 || len(w.Left) != 1 || w.Left[0] != link {
		t.Errorf("edits %+v, left %+v", w.Edits, w.Left)
	}
}
