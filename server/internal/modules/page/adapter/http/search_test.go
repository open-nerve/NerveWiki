package httpadapter_test

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type fakeSearch struct{ *fakes }

// searchHits are the hits the fake answers, newest change first: a page
// under Parent / Child, its name and its content hit, the content going
// on after the excerpt; an attachment at the root; a page its content
// alone hit, twice, the content going on before the excerpt; a page its
// name alone hit, its content holding none.
func searchHits() []app.SearchHit {
	return []app.SearchHit{
		{Node: app.SearchNode{ID: id(12), NotebookID: id(10), Kind: domain.KindPage, Name: "Go语言笔记"},
			Ancestors: []domain.Ancestor{{ID: id(11), Name: "Parent"}, {ID: id(14), Name: "Child"}},
			TitleHits: []domain.TextRange{{Start: 0, End: 2}},
			Excerpt:   &domain.SearchExcerpt{Text: "Learn Go in a week", Hits: []domain.TextRange{{Start: 6, End: 8}}, CutEnd: true},
			ChangedAt: at()},
		{Node: app.SearchNode{ID: id(43), NotebookID: id(15), Kind: domain.KindAsset, Name: "go.png"},
			TitleHits: []domain.TextRange{{Start: 0, End: 2}}, ChangedAt: at().Add(-time.Minute),
			Attachment: &app.AttachmentInfo{MIME: "image/png", Bytes: 70}},
		{Node: app.SearchNode{ID: id(16), NotebookID: id(10), Kind: domain.KindPage, Name: "Notes"},
			Ancestors: []domain.Ancestor{{ID: id(11), Name: "Parent"}},
			Excerpt: &domain.SearchExcerpt{Text: "uses go and Go", Hits: []domain.TextRange{{Start: 5, End: 7}, {Start: 12, End: 14}},
				CutStart: true},
			ChangedAt: at().Add(-2 * time.Minute)},
		{Node: app.SearchNode{ID: id(17), NotebookID: id(10), Kind: domain.KindPage, Name: "Go"}, Ancestors: []domain.Ancestor{},
			TitleHits: []domain.TextRange{{Start: 0, End: 2}}, ChangedAt: at().Add(-3 * time.Minute)},
	}
}

const (
	searchHitsJSON = `[{"ancestors":[{"id":"0199a2b4-0000-7000-8000-000000000011","name":"Parent"},` +
		`{"id":"0199a2b4-0000-7000-8000-000000000014","name":"Child"}],"attachment":null,"changed_at":"2026-10-02T10:00:00Z",` +
		`"excerpt":{"cut_end":true,"cut_start":false,"hits":[{"end":8,"start":6}],"text":"Learn Go in a week"},` +
		`"node":{"id":"0199a2b4-0000-7000-8000-000000000012","kind":"page","name":"Go语言笔记",` +
		`"notebook_id":"0199a2b4-0000-7000-8000-000000000010"},"title_hits":[{"end":2,"start":0}]},` +
		`{"ancestors":[],"attachment":{"byte_size":70,"mime":"image/png"},"changed_at":"2026-10-02T09:59:00Z","excerpt":null,` +
		`"node":{"id":"0199a2b4-0000-7000-8000-000000000043","kind":"asset","name":"go.png",` +
		`"notebook_id":"0199a2b4-0000-7000-8000-000000000015"},"title_hits":[{"end":2,"start":0}]},` +
		`{"ancestors":[{"id":"0199a2b4-0000-7000-8000-000000000011","name":"Parent"}],"attachment":null,` +
		`"changed_at":"2026-10-02T09:58:00Z","excerpt":{"cut_end":false,"cut_start":true,"hits":[{"end":7,"start":5},` +
		`{"end":14,"start":12}],"text":"uses go and Go"},"node":{"id":"0199a2b4-0000-7000-8000-000000000016","kind":"page",` +
		`"name":"Notes","notebook_id":"0199a2b4-0000-7000-8000-000000000010"},"title_hits":[]},` +
		`{"ancestors":[],"attachment":null,"changed_at":"2026-10-02T09:57:00Z","excerpt":null,` +
		`"node":{"id":"0199a2b4-0000-7000-8000-000000000017","kind":"page","name":"Go",` +
		`"notebook_id":"0199a2b4-0000-7000-8000-000000000010"},"title_hits":[{"end":2,"start":0}]}]`
	// searchPageJSON is a first page of the hits, counted exact.
	searchPageJSON = `{"building":false,"count":{"kind":"exact","value":4},"data":` + searchHitsJSON + `,"next_cursor":"next"}`
	// laterPageJSON is a later page of them, uncounted.
	laterPageJSON = `{"building":false,"count":null,"data":` + searchHitsJSON + `,"next_cursor":"next"}`
)

// searchCount is the first page's count of the workspace slug: about 37
// for "about", at least 960 for "floor", at least 1,000 for "capped", 4
// exact for any other.
func searchCount(slug string) *domain.SearchCount {
	switch slug {
	case "about":
		return &domain.SearchCount{Value: 37, Kind: domain.CountAbout}
	case "floor":
		return &domain.SearchCount{Value: 960, Kind: domain.CountAtLeast}
	case "capped":
		return &domain.SearchCount{Value: domain.MaxSearchCount, Kind: domain.CountAtLeast}
	}
	return &domain.SearchCount{Value: 4, Kind: domain.CountExact}
}

// Execute answers searchHits and the next cursor "next", the first page
// counted by searchCount, a later one not, the workspace "building"
// building; for the cursor "end", nothing more; for the workspace "empty",
// no hit, counted exact 0; for "pending", no hit yet, uncounted, and a
// next cursor. Nothing more and pending answer nil slices.
func (f fakeSearch) Execute(_ context.Context, slug string, r app.SearchRequest) (app.SearchPage, error) {
	f.got = []any{slug, r}
	switch {
	case slug == "empty":
		return app.SearchPage{Hits: []app.SearchHit{}, Count: &domain.SearchCount{Kind: domain.CountExact}}, f.err
	case slug == "pending":
		return app.SearchPage{NextCursor: "next"}, f.err
	case r.Cursor != nil && *r.Cursor == "end":
		return app.SearchPage{}, f.err
	}
	p := app.SearchPage{Hits: searchHits(), NextCursor: "next", Building: slug == "building"}
	if r.Cursor == nil {
		p.Count = searchCount(slug)
	}
	return p, f.err
}

const searchPath = "/api/v0/workspaces/acme/search"

// The answer is the use case's page as the contract shapes it, and the
// parameters reach the use case as sent: q absent is nil, as empty is "";
// a limit outside 1–50 is the use case's to refuse.
func TestTheSearchAnswersTheUseCase(t *testing.T) {
	goQ, empty, both, cursor, end := "go", "", "go 语言", "abc", "end"
	seven, zero, many := 7, 0, 51
	notebook := id(15)
	for _, tt := range []struct {
		name, path, want string
		got              app.SearchRequest
	}{
		{"no q", searchPath, searchPageJSON, app.SearchRequest{}},
		{"q", searchPath + "?q=go", searchPageJSON, app.SearchRequest{Q: &goQ}},
		{"an empty q", searchPath + "?q=", searchPageJSON, app.SearchRequest{Q: &empty}},
		{"a notebook's later page", searchPath + "?q=go+%E8%AF%AD%E8%A8%80&notebook_id=0199a2b4-0000-7000-8000-000000000015" +
			"&cursor=abc&limit=7", laterPageJSON, app.SearchRequest{Q: &both, NotebookID: &notebook, Cursor: &cursor, Limit: &seven}},
		{"a limit of 0", searchPath + "?q=go&limit=0", searchPageJSON, app.SearchRequest{Q: &goQ, Limit: &zero}},
		{"a limit of 51", searchPath + "?q=go&limit=51", searchPageJSON, app.SearchRequest{Q: &goQ, Limit: &many}},
		{"none left", searchPath + "?q=go&cursor=end", `{"building":false,"count":null,"data":[],"next_cursor":null}`,
			app.SearchRequest{Q: &goQ, Cursor: &end}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakes{}
			status, body := call(t, f.serve(t), "session", http.MethodGet, tt.path, "")
			if status != http.StatusOK || body != tt.want {
				t.Errorf("GET %s = %d %s\nwant 200 %s", tt.path, status, body, tt.want)
			}
			if want := []any{"acme", tt.got}; !reflect.DeepEqual(f.got, want) {
				t.Errorf("the use case got %v, want %v", f.got, want)
			}
		})
	}
}

// A first page's count reaches the answer as counted, from exact 0 to at
// least 1,000, the contract's bounds; none yet is null; building is the
// use case's.
func TestTheSearchCountsAsTheUseCase(t *testing.T) {
	for _, tt := range []struct{ slug, want string }{
		{"acme", searchPageJSON},
		{"about", strings.Replace(searchPageJSON, `{"kind":"exact","value":4}`, `{"kind":"about","value":37}`, 1)},
		{"floor", strings.Replace(searchPageJSON, `{"kind":"exact","value":4}`, `{"kind":"at_least","value":960}`, 1)},
		{"capped", strings.Replace(searchPageJSON, `{"kind":"exact","value":4}`, `{"kind":"at_least","value":1000}`, 1)},
		{"building", strings.Replace(searchPageJSON, `"building":false`, `"building":true`, 1)},
		{"empty", `{"building":false,"count":{"kind":"exact","value":0},"data":[],"next_cursor":null}`},
		{"pending", `{"building":false,"count":null,"data":[],"next_cursor":"next"}`},
	} {
		path := "/api/v0/workspaces/" + tt.slug + "/search?q=go"
		f := &fakes{}
		status, body := call(t, f.serve(t), "session", http.MethodGet, path, "")
		goQ := "go"
		if status != http.StatusOK || body != tt.want || !reflect.DeepEqual(f.got, []any{tt.slug, app.SearchRequest{Q: &goQ}}) {
			t.Errorf("GET %s = %d %s, use case got %v\nwant 200 %s", path, status, body, f.got, tt.want)
		}
	}
}

// Every code the search declares, as the use case answers it: the
// cursor's, q's and limit's as the domain words them, each field's code
// one the contract knows.
func TestTheSearchAnswersEachProblem(t *testing.T) {
	invalid := func(q *string) error {
		_, err := domain.ParseSearchQuery(q)
		return err
	}
	_, limit := domain.SearchLimit(new(51))
	long := strings.Repeat("a", domain.MaxSearchQueryBytes+1)
	for _, tt := range []struct {
		name   string
		err    error
		status int
		code   string
		field  string
	}{
		{"a cursor", shared.InvalidCursor(), http.StatusBadRequest, "bad_request", `"field":"cursor","code":"invalid_format"`},
		{"no q", invalid(nil), http.StatusUnprocessableEntity, "validation_failed", `"field":"q","code":"required"`},
		{"a q not UTF-8", invalid(new("a\xffb")), http.StatusUnprocessableEntity, "validation_failed",
			`"field":"q","code":"invalid_format"`},
		{"a long q", invalid(&long), http.StatusUnprocessableEntity, "validation_failed", `"field":"q","code":"too_long"`},
		{"a q of signs", invalid(new("!!")), http.StatusUnprocessableEntity, "validation_failed", `"field":"q","code":"not_allowed"`},
		{"a limit", limit, http.StatusUnprocessableEntity, "validation_failed", `"field":"limit","code":"out_of_range"`},
		{"a workspace", domain.ErrWorkspaceNotFound, http.StatusNotFound, "workspace.not_found", ""},
		{"a notebook", domain.ErrNotebookNotFound, http.StatusNotFound, "notebook.not_found", ""},
	} {
		if tt.err == nil {
			t.Fatalf("%s: the domain answered no error", tt.name)
		}
		status, body := call(t, (&fakes{err: tt.err}).serve(t), "session", http.MethodGet, searchPath+"?q=go", "")
		if status != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) || !strings.Contains(body, tt.field) ||
			strings.Contains(body, `"field"`) != (tt.field != "") {
			t.Errorf("%s: %d %s, want %d %s %s", tt.name, status, body, tt.status, tt.code, tt.field)
		}
	}
}

// notebook_id is a uuid and limit an integer: anything else is the
// binding's 400, before the use case.
func TestASearchRequestTheBoundaryRefuses(t *testing.T) {
	for _, query := range []string{
		"?q=go&notebook_id=notes",
		"?q=go&notebook_id=0199a2b4-0000-7000-8000",
		"?q=go&limit=x",
		"?q=go&limit=1.5",
		"?q=go&limit=99999999999999999999",
	} {
		f := &fakes{}
		status, body := call(t, f.serve(t), "session", http.MethodGet, searchPath+query, "")
		if status != http.StatusBadRequest || !strings.Contains(body, `"code":"bad_request"`) || f.got != nil {
			t.Errorf("GET %s = %d %s, use case got %v; want 400 bad_request, the use case not called", query, status, body, f.got)
		}
	}
}
