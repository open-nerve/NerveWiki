package markdownadapter_test

import (
	"reflect"
	"strings"
	"testing"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

func factsOf(t *testing.T, content string) domain.Facts {
	t.Helper()
	md, err := markdown.New([]markdown.Extension{obsidian.Extension()})
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
	want := domain.Facts{
		FrontmatterValid: true,
		Links: []domain.Link{
			link("wikilink", "src", "Other", "P", "o", "Other"),
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

// The frontmatter's tags and aliases are read as Obsidian reads them: the
// first key that is "tags" or "aliases" but for ASCII case, a string one
// (commas and all), a list's strings, trimmed as JavaScript trims; an empty
// one is none, and a tag must be one by rule 9, after a '#' it may start
// with. "alias" and "tag" are other keys, as is a key that is one but for
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
		{"aliases: \"\u00a0\u3000Wide\u2028\ufeff\"\ntags: \"\\Nx\"", nil, []string{"Wide"}},
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

// A frontmatter that is not valid has nothing; none is valid; facts that
// are not the Markdown's are an error.
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
}
