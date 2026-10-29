package markdownadapter_test

import (
	"reflect"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Origin is present even when the legal YAML key has no bytes. The
// document and byte ranges below are independent of InFrontmatter.
func TestLinkOriginFacts(t *testing.T) {
	for _, kind := range []string{"wiki", "markdown"} {
		t.Run("quoted-empty/"+kind, func(t *testing.T) {
			for _, quote := range []struct {
				name string
				mark byte
			}{{"single", '\''}, {"double", '"'}} {
				t.Run(quote.name, func(t *testing.T) {
					value, target, linkKind, start, end, namedAt, bodyAt := "[[Empty]]", "Empty", "wikilink", 11, 16, 30, 45
					if kind == "markdown" {
						value, target, linkKind, start, end, namedAt, bodyAt = "[empty](Empty.md)", "Empty.md", "link", 17, 25, 38, 53
					}
					content := "---\n\"\": " + string(quote.mark) + value + string(quote.mark) + "\nnamed: '[[Named]]'\n---\n[[Body]]\n"
					got := factsOf(t, content)
					want := []domain.Link{
						{Kind: linkKind, Property: "", Quote: quote.mark, Target: target, Start: start, End: end},
						{Kind: "wikilink", Property: "named", Quote: '\'', Target: "Named", Start: namedAt, End: namedAt + 5},
						{Kind: "wikilink", Property: "", Target: "Body", Start: bodyAt, End: bodyAt + 4},
					}
					if !got.FrontmatterValid || !reflect.DeepEqual(got.Links, want) {
						t.Fatalf("facts of %q = valid %t, %+v; want %+v", content, got.FrontmatterValid, got.Links, want)
					}
					for i, frontmatter := range []bool{true, true, false} {
						if got.Links[i].InFrontmatter() != frontmatter {
							t.Errorf("link %d origin = %t, want %t", i, got.Links[i].InFrontmatter(), frontmatter)
						}
					}
				})
			}
		})
	}
}
