package obsidian_test

import (
	"context"
	"encoding/binary"
	"maps"
	"regexp"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

// resolveByStart resolves every link to a page whose id is where the link's
// target starts: the HTML then tells which link each of its links is.
func resolveByStart(_ context.Context, _ markdown.Page, links []obsidian.Link) (map[int]obsidian.Target, error) {
	to := map[int]obsidian.Target{}
	for _, l := range links {
		var id uuid.UUID
		binary.BigEndian.PutUint64(id[8:], uint64(l.Range.Start)+1)
		to[l.Range.Start] = obsidian.Target{Node: id}
	}
	return to, nil
}

// resolveAssetsByStart resolves every link to an attachment whose id is
// where the link's target starts.
func resolveAssetsByStart(ctx context.Context, p markdown.Page, links []obsidian.Link) (map[int]obsidian.Target, error) {
	to, err := resolveByStart(ctx, p, links)
	for start, t := range to {
		t.Asset = true
		to[start] = t
	}
	return to, err
}

// imagesNamed answers each attachment as an image at an address that
// names it.
func imagesNamed(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]obsidian.Asset, error) {
	out := map[uuid.UUID]obsidian.Asset{}
	for _, id := range ids {
		out[id] = obsidian.Asset{MIME: "image/png", Bytes: 1, URL: "/api/v0/assets/" + id.String() + "/content"}
	}
	return out, nil
}

// nodeID is a link's page in the HTML, assetID its attachment.
var (
	nodeID  = regexp.MustCompile(`data-nw-node="([0-9a-f-]{36})"`)
	assetID = regexp.MustCompile(`"/api/v0/assets/([0-9a-f-]{36})/content"`)
)

// unrendered are the links of the fixtures that are extracted and not
// rendered, by where their targets start: those in a comment, which the
// HTML leaves out (rule 6), and a wikilink in a link's text, which the
// link holds as its text (fixtures 030, 040).
//
//nolint:gochecknoglobals // read only
var unrendered = map[string][]int{
	"021-comments.md":                {11, 37},
	"030-link-with-wikilink-text.md": {3},
	"040-wikilink-in-link-text.md":   {7},
	"043-comment-edges.md":           {4},
	"044-comment-unclosed-block.md":  {19, 31},
	"073-comment-across-blocks.md":   {12},
}

// The links in a fixture's HTML are the links its extraction takes (v0.1
// design 4.3; M6 design 4.1; M6 closeout C-M5): each link rendered is one
// extracted, and each extracted is rendered but those unrendered lists;
// to pages, and to attachments, images, at their addresses (M7/P3 review
// C4).
func TestTheFixturesRenderedLinksAreTheirExtractedLinks(t *testing.T) {
	for _, mode := range []struct {
		name    string
		options obsidian.Options
		ids     *regexp.Regexp
	}{
		{"to pages", obsidian.Options{Resolve: resolveByStart}, nodeID},
		{"to attachments", obsidian.Options{Resolve: resolveAssetsByStart, Assets: imagesNamed}, assetID},
	} {
		t.Run(mode.name, func(t *testing.T) { checkRenderedLinks(t, newMarkdownWith(t, mode.options), mode.ids) })
	}
}

// checkRenderedLinks checks that the links m renders of each fixture, by
// the ids ids finds, are its extracted links, but those unrendered lists.
func checkRenderedLinks(t *testing.T, m *markdown.Markdown, ids *regexp.Regexp) {
	t.Helper()
	for _, f := range markdowntest.Fixtures(t) {
		t.Run(f.Name, func(t *testing.T) {
			d := m.Parse(f.Content)
			page := markdown.Page{NotebookID: uuid.New(), PageID: uuid.New(), Revision: 1}
			view, err := m.Render(context.Background(), d, page)
			if err != nil {
				t.Fatal(err)
			}
			html := view.HTML
			rendered := map[int]bool{}
			for _, match := range ids.FindAllStringSubmatch(html, -1) {
				id := uuid.MustParse(match[1])
				rendered[int(binary.BigEndian.Uint64(id[8:]))-1] = true
			}
			got, _ := d.Extracted(obsidian.Name).(obsidian.Extracted)
			extracted := map[int]bool{}
			var missing []int
			for _, l := range got.Links {
				extracted[l.Range.Start] = true
				if !rendered[l.Range.Start] {
					missing = append(missing, l.Range.Start)
				}
			}
			for _, start := range slices.Sorted(maps.Keys(rendered)) {
				if !extracted[start] {
					t.Errorf("the link rendered at %d is not extracted", start)
				}
			}
			if want := unrendered[f.Name]; !slices.Equal(missing, want) {
				t.Errorf("extracted, not rendered: the links at %v, want %v", missing, want)
			}
		})
	}
}
