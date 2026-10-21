package mcpadapter

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// searchFunc is a Search of a function.
type searchFunc func(notebookID uuid.UUID, r app.SearchRequest) (app.SearchPage, error)

func (f searchFunc) InNotebook(_ context.Context, notebookID uuid.UUID, r app.SearchRequest) (app.SearchPage, error) {
	return f(notebookID, r)
}

// search's answer (M9/P4 design 3.10.2): each result's path from its
// ancestors' names, kind page or attachment, the excerpt's text with "…"
// at each end it is cut at and null without one, the time in UTC; the
// count; next_cursor the use case's, null without one; building. The call
// passes its notebook, q and its arguments.
func TestTheSearchAnswersItsShape(t *testing.T) {
	at := time.Date(2026, 10, 21, 10, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	root, guide, png := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	p := app.SearchPage{Hits: []app.SearchHit{
		{Node: app.SearchNode{ID: png, Kind: domain.KindAsset, Name: "flow.png"}, Ancestors: []domain.Ancestor{{ID: root, Name: "Root"}},
			ChangedAt: at},
		{Node: app.SearchNode{ID: guide, Kind: domain.KindPage, Name: "Guide"}, ChangedAt: at, Tier: domain.SearchTierBody,
			Excerpt: &domain.SearchExcerpt{Text: "the flow here", CutStart: true, CutEnd: true}},
		{Node: app.SearchNode{ID: uuid.NewV7(), Kind: domain.KindPage, Name: "Flow"}, Ancestors: []domain.Ancestor{{ID: root, Name: "Root"}},
			ChangedAt: at, Tier: domain.SearchTierBody, Excerpt: &domain.SearchExcerpt{Text: "flow"}},
	}, NextCursor: "next", Count: &domain.SearchCount{Value: 42, Kind: domain.CountAbout}, Building: true}
	var passed []any
	tools := tools{uc: UseCases{Search: searchFunc(func(id uuid.UUID, r app.SearchRequest) (app.SearchPage, error) {
		passed = []any{id, r}
		return p, nil
	})}}
	notebook, limit, cursor := uuid.NewV7(), 3, "c0"
	res, err := tools.search(context.Background(), mcpserver.Call{Notebook: notebook}, searchIn{Q: "flow", Limit: &limit, Cursor: &cursor})
	if err != nil {
		t.Fatal(err)
	}
	if r, ok := passed[1].(app.SearchRequest); passed[0] != notebook || !ok || r.Q == nil || *r.Q != "flow" || r.Limit != &limit ||
		r.Cursor != &cursor || r.NotebookID != nil {
		t.Errorf("search passed %v", passed)
	}
	want := `{"results":[{"path":"Root/flow.png","kind":"attachment","excerpt":null,"changed_at":"2026-10-21T08:30:00Z"},` +
		`{"path":"Guide.md","kind":"page","excerpt":"…the flow here…","changed_at":"2026-10-21T08:30:00Z"},` +
		`{"path":"Root/Flow.md","kind":"page","excerpt":"flow","changed_at":"2026-10-21T08:30:00Z"}],` +
		`"count":{"value":42,"kind":"about"},"next_cursor":"next","building":true}`
	if raw := rawOf(t, res); raw != want {
		t.Errorf("the answer is\n%s\nwant\n%s", raw, want)
	}
	p = app.SearchPage{Hits: []app.SearchHit{}}
	res, err = tools.search(context.Background(), mcpserver.Call{}, searchIn{Q: "flow"})
	if raw := rawOf(t, res); err != nil || raw != `{"results":[],"count":null,"next_cursor":null,"building":false}` {
		t.Errorf("no results on a later page = %s, %v", raw, err)
	}
}

// An answer holds as many results as it can (M9/P4 design 3.10.2): 50
// results of paths of ten pages and a page, each title 255 bytes of "&",
// are cut at the bound, next_cursor the last kept's Cursor, of its tier;
// every one kept, the use case's next_cursor stands.
func TestTheSearchFitsItsAnswer(t *testing.T) {
	title := strings.Repeat("&", 255)
	ancestors := make([]domain.Ancestor, 10)
	for i := range ancestors {
		ancestors[i] = domain.Ancestor{ID: uuid.NewV7(), Name: title}
	}
	at := time.Date(2026, 10, 21, 10, 30, 0, 0, time.UTC)
	hits := make([]app.SearchHit, domain.MaxSearchLimit)
	for i := range hits {
		hits[i] = app.SearchHit{Node: app.SearchNode{ID: uuid.NewV7(), Kind: domain.KindPage, Name: title}, Ancestors: ancestors,
			ChangedAt: at.Add(-time.Duration(i) * time.Minute), Tier: domain.SearchTierBody,
			Excerpt: &domain.SearchExcerpt{Text: strings.Repeat("&", 160), CutStart: true, CutEnd: true}}
	}
	tools := tools{uc: UseCases{Search: searchFunc(func(uuid.UUID, app.SearchRequest) (app.SearchPage, error) {
		return app.SearchPage{Hits: hits, NextCursor: "after", Count: &domain.SearchCount{Value: 1000, Kind: domain.CountAtLeast}}, nil
	})}}
	res, err := tools.search(context.Background(), mcpserver.Call{}, searchIn{Q: "x"})
	if err != nil {
		t.Fatal(err)
	}
	var a searchAnswer
	raw := rawOf(t, res)
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatal(err)
	}
	k := len(a.Results)
	if n := mcpserver.TextBytes(raw); k == 0 || k >= len(hits) || n > mcpserver.MaxAnswerBytes {
		t.Fatalf("%d results in %d bytes, want some, cut at %d", k, n, mcpserver.MaxAnswerBytes)
	}
	want, err := hits[k-1].Cursor()
	if err != nil {
		t.Fatal(err)
	}
	var c domain.SearchCursor
	if a.NextCursor == nil || *a.NextCursor != want || a.Count == nil || a.Count.Value != 1000 {
		t.Errorf("cut at %d results: next %v, count %v; want the %dth's cursor %s, the count", k, a.NextCursor, a.Count, k, want)
	} else if err := shared.DecodeCursor(want, &c); err != nil || c.Tier != domain.SearchTierBody || c.ID != hits[k-1].Node.ID {
		t.Errorf("the %dth's cursor is %+v, %v; want of the body tier, at it", k, c, err)
	}
}
