package markdownadapter_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"uuid"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

// views answers what it is given, and records what it was asked.
type views struct {
	page  app.Page
	links []app.Link
	rs    map[int]domain.Resolution
	err   error
}

func (v *views) Resolve(_ context.Context, p app.Page, links []app.Link) (map[int]domain.Resolution, error) {
	v.page, v.links = p, links
	return v.rs, v.err
}

// The extension's Resolve asks the views of the page and its links, each
// target written as the index writes it, and leads each link to the page
// it resolves to, none to none.
func TestResolveAsksTheViewsAsTheIndexWritesTargets(t *testing.T) {
	a := uuid.NewV7()
	v := &views{rs: map[int]domain.Resolution{2: {ID: a, Ambiguous: true}, 12: {}}}
	page := markdown.Page{NotebookID: uuid.NewV7(), PageID: uuid.NewV7(), Revision: 4}
	links := []obsidian.Link{
		{Kind: obsidian.KindWikilink, Target: "A", Anchor: "h", Range: markdown.Span{Start: 2, Stop: 3}},
		{Kind: obsidian.KindLink, Target: "b\x00c.md", Range: markdown.Span{Start: 12, Stop: 18}},
	}
	got, err := markdownadapter.Resolve(v)(context.Background(), page, links)
	if err != nil || !reflect.DeepEqual(got, map[int]uuid.UUID{2: a}) {
		t.Errorf("Resolve = %v, %v", got, err)
	}
	wantLinks := []app.Link{
		{SourceID: page.PageID, Start: 2, Target: "A"},
		{SourceID: page.PageID, Start: 12, Target: "b\xef\xbf\xbdc.md"},
	}
	if want := (app.Page{ID: page.PageID, NotebookID: page.NotebookID, Revision: 4}); v.page != want || !reflect.DeepEqual(v.links, wantLinks) {
		t.Errorf("asked of %+v for %+v", v.page, v.links)
	}
	v.err = errors.New("down")
	if _, err := markdownadapter.Resolve(v)(context.Background(), page, links); !errors.Is(err, v.err) {
		t.Errorf("Resolve: %v, want %v", err, v.err)
	}
}
