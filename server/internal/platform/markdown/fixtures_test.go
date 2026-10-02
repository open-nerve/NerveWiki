package markdown_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

// Every fixture's frontmatter is what its JSON says: none, not valid, or
// valid with these properties (rule 1).
func TestTheFixturesFrontmatterIsTheirs(t *testing.T) {
	m, err := markdown.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range markdowntest.Fixtures(t) {
		t.Run(f.Name, func(t *testing.T) {
			var want struct {
				Frontmatter *struct {
					Valid      bool           `json:"valid"`
					Properties map[string]any `json:"properties"`
				} `json:"frontmatter"`
			}
			if err := json.Unmarshal(f.JSON, &want); err != nil {
				t.Fatal(err)
			}
			fm := m.Parse(f.Content).Frontmatter()
			switch w := want.Frontmatter; {
			case w == nil:
				if fm.Present {
					t.Errorf("a frontmatter, where the fixture has none: %+v", fm)
				}
			case !w.Valid:
				if !fm.Present || fm.Valid {
					t.Errorf("frontmatter %+v, want present and not valid", fm)
				}
			default:
				if !fm.Present || !fm.Valid {
					t.Fatalf("frontmatter %+v, want valid", fm)
				}
				var got map[string]any
				raw, _ := json.Marshal(markdown.JSONOf(fm.Properties))
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatal(err)
				}
				if w.Properties == nil {
					w.Properties = map[string]any{}
				}
				if !reflect.DeepEqual(got, w.Properties) {
					t.Errorf("properties %v, want %v", got, w.Properties)
				}
			}
		})
	}
}
