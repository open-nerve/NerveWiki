package postgresadapter_test

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"uuid"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

func originFacts(t *testing.T, content string) domain.Facts {
	t.Helper()
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	facts, err := markdownadapter.PageFacts(md.Parse([]byte(content)).Facts())
	if err != nil {
		t.Fatal(err)
	}
	return facts
}

// The actual writer keeps SQL NULL distinct from a legal empty property
// path, and keeps every parallel input aligned through Add and Replace.
func TestLintOriginRows(t *testing.T) {
	t.Run("current-format", func(t *testing.T) {
		f := newFixture(t)
		p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
		for _, operation := range []string{"AddPage", "ReplacePage"} {
			t.Run(operation, func(t *testing.T) {
				content := "---\n\"\": '[[Empty#P|empty]]'\nnamed:\n  path: \"[named](Named.md#H)\"\n---\n[[Body#B|body]] ![[Pic.png]] [doc](Other.md) ![alt](Image.png)\n"
				if operation == "ReplacePage" {
					content = strings.ReplaceAll(content, "Empty", "Again")
					p.Revision = 2
				}
				err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
					if err := f.s.Lock(ctx, p.NotebookID); err != nil {
						return err
					}
					if operation == "AddPage" {
						return f.s.AddPage(ctx, p, originFacts(t, content))
					}
					_, err := f.s.ReplacePage(ctx, p, originFacts(t, content))
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
				if got := f.rows(t, `SELECT revision::text, extractor::text, frontmatter_valid::text FROM indexed_pages WHERE node_id=$1`, p.ID); !reflect.DeepEqual(got, [][]string{{strconv.Itoa(p.Revision), "2", "true"}}) {
					t.Errorf("literal current format: %v; want revision %d, extractor 2, valid", got, p.Revision)
				}
				// The tokens identify literal document ranges; neither expected
				// origin nor expected keys are taken from parsed/writer facts.
				empty := "Empty"
				if operation == "ReplacePage" {
					empty = "Again"
				}
				want := [][]string{}
				for _, v := range [][]string{
					{"wikilink", "", empty, "P", "empty", strings.ToLower(empty), "NULL", "false"},
					{"link", "named.path", "Named.md", "H", "NULL", "named", "named.md", "false"},
					{"wikilink", "NULL", "Body", "B", "body", "body", "NULL", "false"},
					{"embed", "NULL", "Pic.png", "NULL", "NULL", "pic.png", "NULL", "false"},
					{"link", "NULL", "Other.md", "NULL", "NULL", "other", "other.md", "false"},
					{"image", "NULL", "Image.png", "NULL", "NULL", "image.png", "NULL", "false"},
				} {
					at := strings.Index(content, v[2])
					want = append(want, append([]string{strconv.Itoa(at), strconv.Itoa(at + len(v[2]))}, v...))
				}
				got := f.rows(t, `SELECT range_start::text, range_end::text, kind, property_key, target, anchor, display, target_key, target_alt_key, aliases::text FROM page_links WHERE source_id=$1 ORDER BY page_links.range_start`, p.ID)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("stored origins and aligned rows = %v\nwant %v", got, want)
				}
			})
		}
	})
}

func TestLintOriginProperties(t *testing.T) {
	t.Run("empty-key", func(t *testing.T) {
		f := newFixture(t)
		for _, kind := range []string{"page", "unresolved", "asset"} {
			t.Run(kind, func(t *testing.T) {
				p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
				content := "---\n\"\": '[[Empty]]'\nnamed: '[[Named]]'\n---\n[[Body]]\n"
				f.replace(t, p, originFacts(t, content))
				resolution := domain.Resolution{}
				if kind != "unresolved" {
					resolution.ID = uuid.NewV7()
					resolution.Asset = kind == "asset"
				}
				f.resolve(t, p.ID, resolution, 11)
				named := uuid.NewV7()
				f.resolve(t, p.ID, domain.Resolution{ID: named}, 30)
				f.resolve(t, p.ID, domain.Resolution{ID: uuid.NewV7()}, 45)
				got, ok, err := f.s.Properties(t.Context(), p.ID)
				want := []app.PropertyLink{{Key: "", NodeID: resolution.ID, Asset: resolution.Asset}, {Key: "named", NodeID: named}}
				if err != nil || !ok || !got.Valid || !reflect.DeepEqual(got.Links, want) {
					t.Errorf("property links: %+v, %t, %v; want %+v", got, ok, err, want)
				}
			})
		}
	})
}
