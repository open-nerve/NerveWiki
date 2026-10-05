package markdownadapter_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
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
// "aliases" but for ASCII case, its string or its list's; whether a
// wikilink is in a table's cell. A body's link has no quote.
func TestALinkTellsWhatARewriteReads(t *testing.T) {
	content := "---\naliases: [\"[[A]]\", '[[B]]']\nAliases: \"[[C]]\"\nr: '[[D]]'\nn:\n  aliases: \"[[E]]\"\n---\n" +
		"| h |\n| --- |\n| [[F]] |\n\n[[G]] [h](H.md)\n"
	type read struct {
		Quote            byte
		Aliases, InTable bool
	}
	var got []read
	for _, l := range factsOf(t, content).Links {
		got = append(got, read{l.Quote, l.Aliases, l.InTable})
	}
	want := []read{{'"', true, false}, {'\'', true, false}, {'"', false, false}, {'\'', false, false}, {'"', false, false}, {0, false, true}, {0, false, false}, {0, false, false}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("read %+v\nwant %+v", got, want)
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
