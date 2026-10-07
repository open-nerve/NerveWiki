package markdownadapter_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/bits"
	"reflect"
	"strings"
	"testing"
	"time"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func factsOf(t *testing.T, content string) domain.Facts {
	t.Helper()
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	f, err := markdownadapter.PageFacts(md.Parse([]byte(content)).Facts())
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// A page's facts are its links as the extension took them, the
// frontmatter's tags before the body's, by key with how often, its aliases
// and its properties in order, a mapping an object.
func TestAPagesFactsAreWhatTheIndexKeeps(t *testing.T) {
	content := "---\nsrc: \"[[Other#P|o]]\"\nTags: [ToDo, \"#x\"]\naliases: [Al, al, \" Spaced \"]\nn: {a: 1, b: [true, null]}\n---\n" +
		"[[A/Note#Part|the note]] ![[Pic.png]] [m](Straße.md) ![i](a.png) #todo #y/z\n"
	got := factsOf(t, content)
	span := func(target string) (int, int) {
		at := strings.Index(content, target)
		return at, at + len(target)
	}
	link := func(kind, property, target, anchor, display, written string) domain.Link {
		start, end := span(written)
		return domain.Link{Kind: kind, Property: property, Target: target, Anchor: anchor, Display: display, Start: start, End: end}
	}
	quoted := link("wikilink", "src", "Other", "P", "o", "Other")
	quoted.Quote = '"'
	want := domain.Facts{
		FrontmatterValid: true,
		Links: []domain.Link{
			quoted,
			link("wikilink", "", "A/Note", "Part", "the note", "A/Note"),
			link("embed", "", "Pic.png", "", "", "Pic.png"),
			link("link", "", "Straße.md", "", "", "Straße.md"),
			link("image", "", "a.png", "", "", "a.png"),
		},
		Tags: []domain.Tag{{Key: "todo", Name: "ToDo", Count: 2}, {Key: "x", Name: "x", Count: 1}, {Key: "y/z", Name: "y/z", Count: 1}},
		Properties: []domain.Property{
			{Key: "src", Value: []byte(`"[[Other#P|o]]"`)},
			{Key: "Tags", Value: []byte(`["ToDo","#x"]`)},
			{Key: "aliases", Value: []byte(`["Al","al"," Spaced "]`)},
			{Key: "n", Value: []byte(`{"a":1,"b":[true,null]}`)},
		},
		Aliases: []domain.Alias{{Key: "al", Name: "Al"}, {Key: "spaced", Name: "Spaced"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("facts = %+v\nwant %+v", got, want)
	}
}

// What a rewrite reads of a link beside (M6/P4 design 3): a property
// link's quote; whether it is a value of the aliases, the first key that is
// "aliases" but for ASCII case, its string or its list's, not a key's
// that holds a '.' or a mapping's; whether a wikilink is in a table's
// cell. A body's link has no quote.
func TestALinkTellsWhatARewriteReads(t *testing.T) {
	content := "---\naliases: [\"[[A]]\", '[[B]]']\nAliases: \"[[C]]\"\nr: '[[D]]'\nn:\n  aliases: \"[[E]]\"\naliases.1: \"[[F]]\"\n---\n" +
		"| h |\n| --- |\n| [[G]] |\n\n[[H]] [i](I.md)\n"
	type read struct {
		Quote            byte
		Aliases, InTable bool
	}
	var got []read
	for _, l := range factsOf(t, content).Links {
		got = append(got, read{l.Quote, l.Aliases, l.InTable})
	}
	want := []read{
		{'"', true, false}, {'\'', true, false}, {'"', false, false}, {'\'', false, false}, {'"', false, false}, {'"', false, false},
		{0, false, true}, {0, false, false}, {0, false, false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("read %+v\nwant %+v", got, want)
	}
	for frontmatter, want := range map[string]bool{
		"ALIASES: '[[A]]'":               true,
		"aliases: {\"0\": \"[[A]]\"}":    false,
		"aliases: [[\"[[A]]\"]]":         false,
		"aliases: {x: [\"[[A]]\"]}":      false,
		"aliases: x\naliases.0: '[[A]]'": false,
	} {
		links := factsOf(t, "---\n"+frontmatter+"\n---\n").Links
		if len(links) != 1 || links[0].Aliases != want {
			t.Errorf("%q: links %+v, want one whose aliases is %t", frontmatter, links, want)
		}
	}
}

// The frontmatter's tags and aliases are read as Obsidian reads them: the
// first key that is "tags" or "aliases" but for ASCII case, a string one
// (commas and all), a list's strings, trimmed as JavaScript trims; an empty
// one is none, and a tag must be one the tag pane counts, after a '#' it may
// start with. "alias" and "tag" are other keys, as is a key that is one but for
// a case other than ASCII's (a long s); a first key with no string is
// none, not the next key.
func TestTheFrontmattersTagsAndAliasesAreReadAsObsidianReadsThem(t *testing.T) {
	tests := []struct {
		frontmatter string
		tags        []string
		aliases     []string
	}{
		{"aliases: \"One, Two\"\ntags: \"a, b\"", nil, []string{"One, Two"}},
		{"aliases: [31, true, Al, [Nest], \"\", \" \"]\ntags: [123, \"1x\", a b, a.b, \"##c\", \"#d\", \"\"]", []string{"1x", "d"}, []string{"Al"}},
		{"ALIASES: Up\nTAGS: up", []string{"up"}, []string{"Up"}},
		{"alias: Single\ntag: single", nil, nil},
		{"aliases: [First]\nAliases: [Second]\ntags: first\nTags: second", []string{"first"}, []string{"First"}},
		{"aliases: 3.5\ntags: 42", nil, nil},
		{"aliases: \"\u00a0\u3000Wide\u2028\ufeff\"\ntags: \"\\Nx\"", []string{"\u0085x"}, []string{"Wide"}},
		{"aliases: \u212aelvin", nil, []string{"\u212aelvin"}},
		{"alia\u017fes: [No]\ntag\u017f: no", nil, nil},
		{"aliases: 3\nAliases: [Second]\ntags:\nTags: [second]", nil, nil},
	}
	for _, tt := range tests {
		got := factsOf(t, "---\n"+tt.frontmatter+"\n---\n")
		var tags, aliases []string
		for _, tag := range got.Tags {
			tags = append(tags, tag.Name)
		}
		for _, a := range got.Aliases {
			aliases = append(aliases, a.Name)
		}
		if !reflect.DeepEqual(tags, tt.tags) || !reflect.DeepEqual(aliases, tt.aliases) {
			t.Errorf("%q: tags %q, aliases %q; want %q, %q", tt.frontmatter, tags, aliases, tt.tags, tt.aliases)
		}
	}
}

// The tags are those Obsidian's tag pane counts, the frontmatter's and the
// body's: without one '/' they end with, and none with a character it
// refuses.
func TestTheTagsAreThoseTheTagPaneCounts(t *testing.T) {
	got := factsOf(t, "---\ntags: [a/, a😀, a→b, /, a—b, a。b]\n---\n#y/ #z #/\n")
	var tags []string
	for _, tag := range got.Tags {
		tags = append(tags, tag.Name)
	}
	if want := []string{"a", "a😀", "a→b", "a。b", "y", "z"}; !reflect.DeepEqual(tags, want) {
		t.Errorf("tags %q, want %q", tags, want)
	}
}

// A U+0000, which PostgreSQL's text does not hold, written by a Markdown
// link's %00 or a YAML string's escape, is U+FFFD in every fact.
func TestU0000IsWrittenAsTheReplacementCharacter(t *testing.T) {
	got := factsOf(t, "---\nn: \"a\\0b\"\n\"k\\0\": [\"\\0\", {\"m\\0\": \"v\\0\"}]\naliases: [\"x\\0\"]\n"+
		"tags: [\"t\\0\"]\nsrc: \"[[a\\0b|d\\0]]\"\n\"l\\0\": \"[[y]]\"\n---\n[x](a%00b.md#p%00q)\n")
	r := "\uFFFD"
	want := domain.Facts{
		FrontmatterValid: true,
		Links: []domain.Link{
			{Kind: "wikilink", Property: "src", Target: "a" + r + "b", Display: "d" + r, Quote: '"'},
			{Kind: "wikilink", Property: "l" + r, Target: "y", Quote: '"'},
			{Kind: "link", Target: "a" + r + "b.md", Anchor: "p" + r + "q"},
		},
		Tags:    []domain.Tag{{Key: "t" + r, Name: "t" + r, Count: 1}},
		Aliases: []domain.Alias{{Key: "x" + r, Name: "x" + r}},
		Properties: []domain.Property{
			{Key: "n", Value: []byte(`"a` + r + `b"`)},
			{Key: "k" + r, Value: []byte(`["` + r + `",{"m` + r + `":"v` + r + `"}]`)},
			{Key: "aliases", Value: []byte(`["x` + r + `"]`)},
			{Key: "tags", Value: []byte(`["t` + r + `"]`)},
			{Key: "src", Value: []byte(`"[[a` + r + `b|d` + r + `]]"`)},
			{Key: "l" + r, Value: []byte(`"[[y]]"`)},
		},
	}
	for i := range got.Links {
		got.Links[i].Start, got.Links[i].End = 0, 0
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("facts = %+v\nwant %+v", got, want)
	}
}

// A frontmatter that is not valid has nothing; none is valid; facts that
// are not the Markdown's, or that the obsidian extension did not take, are
// an error.
func TestAFrontmatterNotValidHasNothing(t *testing.T) {
	if got := factsOf(t, "---\n[a\n---\n#t"); got.FrontmatterValid || got.Properties != nil || len(got.Tags) != 1 {
		t.Errorf("an invalid frontmatter's facts = %+v", got)
	}
	if got := factsOf(t, "#t"); !got.FrontmatterValid {
		t.Errorf("no frontmatter's facts = %+v", got)
	}
	if _, err := markdownadapter.PageFacts("not facts"); err == nil {
		t.Error("facts of another kind were read")
	}
	md, err := markdown.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := markdownadapter.PageFacts(md.Parse([]byte("[[A]]")).Facts()); err == nil {
		t.Error("facts without the obsidian extension's were read")
	}
}

// The rebuild's parse takes its share of the parse budget: with none left
// within its wait, it is refused.
func TestTheParserTakesTheBudget(t *testing.T) {
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	budget := markdown.NewBudget(64<<10, 10*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	parser := markdownadapter.NewParser(md, budget)
	if f, err := parser.Facts(context.Background(), "[[A]]"); err != nil || len(f.Links) != 1 {
		t.Fatalf("Facts = %+v, %v; want the link", f, err)
	}
	hold, err := budget.Take(context.Background(), 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Release()
	if _, err := parser.Facts(context.Background(), "[[A]]"); !errors.Is(err, markdown.ErrBusy) {
		t.Errorf("Facts with the budget taken = %v, want ErrBusy", err)
	}
}

// A parse now holds its facts' share of the budget, taken now, until
// released, with the facts the page module writes; a budget not free now
// is server_busy at once, whatever its wait.
func TestAParseNowHoldsItsShareTakenNow(t *testing.T) {
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	const size = 64 << 10
	budget := markdown.NewBudget(size, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	parser := markdownadapter.NewParser(md, budget)
	content := strings.Repeat("[[A]] ", 4000)
	parsed, err := parser.ParseNow(context.Background(), content)
	if err != nil || len(parsed.Facts.Links) != 4000 {
		t.Fatalf("ParseNow = %d links, %v; want 4000", len(parsed.Facts.Links), err)
	}
	if _, ok := parsed.Written.(markdown.Facts); !ok {
		t.Errorf("the facts written are %T, want the platform's", parsed.Written)
	}
	share := (parsed.Written.(markdown.Facts).Limit(len(content)) + 299) / 300
	if hold, err := budget.TakeNow(context.Background(), size-share+1); err == nil {
		hold.Release()
		t.Errorf("all but the share %d taken beside it, and one byte more", share)
	}
	parsed.Release()
	all, err := budget.TakeNow(context.Background(), size)
	if err != nil {
		t.Fatalf("the budget after the release: %v", err)
	}
	defer all.Release()
	at := time.Now()
	var busy *shared.Error
	if _, err := parser.ParseNow(context.Background(), "[[A]]"); !errors.As(err, &busy) || busy.Code != "server_busy" || time.Since(at) > time.Second {
		t.Errorf("ParseNow with the budget taken = %v after %s, want server_busy at once", err, time.Since(at))
	}
}

// The index keeps a page's first MaxLinks links, in the order written, the
// frontmatter's first: a page of 870,000 would hold the notebook's index
// lock for seconds (M6 closeout A-I1).
func TestThePageFactsKeepTheFirstMaxLinks(t *testing.T) {
	content := "---\nup: \"[[Up]]\"\n---\n" + strings.Repeat("[[a]] ", domain.MaxLinks) + "[[last]]\n"
	f := factsOf(t, content)
	if len(f.Links) != domain.MaxLinks {
		t.Fatalf("%d links kept, want %d", len(f.Links), domain.MaxLinks)
	}
	if first, last := f.Links[0], f.Links[len(f.Links)-1]; first.Target != "Up" || first.Property != "up" || last.Target != "a" {
		t.Errorf("first %+v, last %+v; want the property's link first, the body's [[a]] last", first, last)
	}
}

// A property link finds its string among those of its path: many of them,
// each a link, take a time as long as they are (M6 closeout A-M3: some
// 10,000 squared before), at paths of their own and all at one path, as
// keys holding a '.' write it (Codex review R3). Four times as many take
// less than eight times as long, the best of three.
func TestThePropertyLinksFindTheirStringsInTimeAsLongAsThey(t *testing.T) {
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	for _, shape := range []struct {
		name  string
		small int // and four times as many; at one path, as many as the YAML's values may be
		write func(b *strings.Builder, n int)
		path  func(i, n int) string // the path of the i-th of n
	}{
		{"a list's", 2_000, func(b *strings.Builder, n int) {
			b.WriteString("related:\n")
			for i := range n {
				fmt.Fprintf(b, "  - \"[[a%d]]\"\n", i)
			}
		}, func(i, _ int) string { return fmt.Sprintf("related.%d", i) }},
		{"one path's", 1 << 9, func(b *strings.Builder, n int) {
			onePath(b, "", bits.Len(uint(n)))
		}, func(_, n int) string { return strings.Repeat("a.", bits.Len(uint(n))-1) + "a" }},
	} {
		took := func(n int) time.Duration {
			var b strings.Builder
			b.WriteString("---\n")
			shape.write(&b, n)
			b.WriteString("---\n")
			facts := md.Parse([]byte(b.String())).Facts()
			best := time.Duration(math.MaxInt64)
			for range 3 {
				at := time.Now()
				f, err := markdownadapter.PageFacts(facts)
				best = min(best, time.Since(at))
				if err != nil || len(f.Links) != n {
					t.Fatalf("%s %d property links: %d facts, %v", shape.name, n, len(f.Links), err)
				}
				for i, l := range f.Links {
					if l.Quote == 0 || l.Property != shape.path(i, n) {
						t.Fatalf("%s %d property links: the %d-th is %+v, want it with its string's quote, at %s",
							shape.name, n, i, l, shape.path(i, n))
					}
				}
			}
			return best
		}
		small, large := took(shape.small), took(4*shape.small)
		if large > 8*small {
			t.Errorf("%s %d property links took %s, %d %s: more than 8 times as long",
				shape.name, 4*shape.small, large, shape.small, small)
		}
	}
}

// onePath writes, at indent, the 2^(n-1) strings at the path of n keys
// "a" (a.a.a…): one for each way keys holding a '.' split it, each a link.
func onePath(b *strings.Builder, indent string, n int) {
	for k := 1; k <= n; k++ {
		key := strings.Repeat("a.", k-1) + "a"
		if k == n {
			fmt.Fprintf(b, "%s%s: '[[x]]'\n", indent, key)
			continue
		}
		fmt.Fprintf(b, "%s%s:\n", indent, key)
		onePath(b, indent+"  ", n-k)
	}
}

// A parse now that panics holds none of the budget (M6 closeout A-M4):
// what it took goes back, and the whole budget is free after.
func TestAParseNowThatPanicsHoldsNoneOfTheBudget(t *testing.T) {
	panicking := markdown.Extension{Name: "panicking", Extract: func(markdown.Tree) any { panic("an extension's bug") }}
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{}), panicking})
	if err != nil {
		t.Fatal(err)
	}
	const size = 64 << 10
	budget := markdown.NewBudget(size, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	parser := markdownadapter.NewParser(md, budget)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the parse did not panic")
			}
		}()
		_, _ = parser.ParseNow(context.Background(), strings.Repeat("[[A]] ", 4000))
	}()
	all, err := budget.TakeNow(context.Background(), size)
	if err != nil {
		t.Fatalf("the whole budget after the panic: %v", err)
	}
	all.Release()
}

// A parse now whose facts are in error holds none of the budget (M6
// closeout FA-N1): without the dialect's extraction its facts have no
// links, PageFacts says so, and the whole budget is free after.
func TestAParseNowOfFactsInErrorHoldsNoneOfTheBudget(t *testing.T) {
	md, err := markdown.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	const size = 64 << 10
	budget := markdown.NewBudget(size, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := markdownadapter.NewParser(md, budget).ParseNow(context.Background(), strings.Repeat("[[A]] ", 4000)); err == nil {
		t.Fatal("facts without the dialect's extraction parsed")
	}
	all, err := budget.TakeNow(context.Background(), size)
	if err != nil {
		t.Fatalf("the whole budget after the error: %v", err)
	}
	all.Release()
}
