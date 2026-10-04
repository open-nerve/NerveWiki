package obsidian_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

// brief is a link in a line: kind, target, anchor, display, key and the
// bytes its range covers in content.
func brief(content string, l obsidian.Link) string {
	return fmt.Sprintf("%s %q #%q |%q @%q %q", l.Kind, l.Target, l.Anchor, l.Display, l.Key,
		content[l.Range.Start:l.Range.Stop])
}

// What the fixtures leave out: a property's value and its writing, a
// reference definition, a comment, an image's caption, the footnotes'
// order (rules 4, 6, 8, 10).
func TestTheLinksOfAPage(t *testing.T) {
	m := newMarkdown(t)
	for _, tt := range []struct {
		name, content string
		want          []string
	}{
		{
			"a property's escapes and quotes", "---\na: \"[[\\x41b]]\"\nb: '[[it''s|x]]'\nc: [[n]]\nd: \"[t](<a b.md>)\"\n---\n",
			[]string{
				`wikilink "Ab" #"" |"" @"a" "\\x41b"`,
				`wikilink "it's" #"" |"x" @"b" "it''s"`,
				`link "a b.md" #"" |"" @"d" "a b.md"`,
			},
		},
		{
			"a property that is not one link", "---\na: \"[[x]](y.md)\"\nb: \"![t](i.png)\"\nc: \"[[x]] [[y]]\"\n" +
				"d: \"[t](https://x.com)\"\ne: \"[[#h]]\"\nf: \"$[[x]]$\"\ng: \" [[x]]\"\nh: \"[[x]] \"\n---\n",
			nil,
		},
		{
			"a definition two links and an image use is one link", "[a][r] ![b][r] [r]\n\n[r]: <t.md#h>\n[u]: u.md\n",
			[]string{`link "t.md" #"h" |"" @"" "t.md"`},
		},
		{
			"a comment's links are the page's", "%%[[a]] [t](b.md)%%\n\n%%\n[[c]]\n",
			[]string{`wikilink "a" #"" |"" @"" "a"`, `link "b.md" #"" |"" @"" "b.md"`, `wikilink "c" #"" |"" @"" "c"`},
		},
		{
			"an image's caption is not, a link's text is", "![[[a]] #t](i.png) [[[b]] #u](l.md)\n",
			[]string{
				`image "i.png" #"" |"" @"" "i.png"`, `wikilink "b" #"" |"" @"" "b"`, `link "l.md" #"" |"" @"" "l.md"`,
			},
		},
		{
			"a footnote's in the content's order", "x[^2] y[^1] [[z]]\n\n[^1]: [[one]]\n[^2]: [[two]]\n",
			[]string{`wikilink "z" #"" |"" @"" "z"`, `wikilink "one" #"" |"" @"" "one"`, `wikilink "two" #"" |"" @"" "two"`},
		},
		{
			"a callout's and a formula's", "> [!note] [[t]] $[[m]]$\n> [[b]]\n\n$$\n[[blk]]\n$$\n",
			[]string{`wikilink "t" #"" |"" @"" "t"`, `wikilink "b" #"" |"" @"" "b"`},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, l := range extracted(m, tt.content).Links {
				got = append(got, brief(tt.content, l))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("links\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

// Every tag counts, in the content's order, each where it is written.
func TestTheTagsOfAPage(t *testing.T) {
	m := newMarkdown(t)
	content := "x[^1] #b\n\n[^1]: #a\n\n%%#c%% `#no` $#no$\n"
	var got []string
	for _, tg := range extracted(m, content).Tags {
		got = append(got, tg.Name+"="+content[tg.Range.Start:tg.Range.Stop])
	}
	if want := []string{"b=#b", "a=#a", "c=#c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tags %q, want %q", got, want)
	}
}

// A comment changes what shows, not what the page holds: a task item it
// hides is the page's, wherever the comment ends (rules 6, 11).
func TestACommentsTaskItemsAreThePages(t *testing.T) {
	m := newMarkdown(t)
	for _, content := range []string{"%%\n- [ ] task %%\n  more\n", "%%\n- [ ] task\n  more %%\n"} {
		got := m.Parse([]byte(content)).Extracted(tasks.Name)
		if want := []tasks.Task{{Offset: 6}}; !reflect.DeepEqual(got, want) {
			t.Errorf("%q: tasks %+v, want %+v", content, got, want)
		}
	}
}
