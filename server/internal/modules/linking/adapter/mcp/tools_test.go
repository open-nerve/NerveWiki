package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// backlinksFunc is a BacklinksUseCase of a function.
type backlinksFunc func(notebookID uuid.UUID, path string, limit *int, cursor *string) (app.BacklinksAt, error)

func (f backlinksFunc) AtPath(_ context.Context, notebookID uuid.UUID, path string, limit *int, cursor *string) (app.BacklinksAt, error) {
	return f(notebookID, path, limit, cursor)
}

// indexFunc is an IndexUseCase of a function.
type indexFunc func(notebookID uuid.UUID, q app.IndexQuery) (app.IndexPage, error)

func (f indexFunc) Execute(_ context.Context, notebookID uuid.UUID, q app.IndexQuery) (app.IndexPage, error) {
	return f(notebookID, q)
}

// rawOf is a structured result's JSON, as Fit encoded it.
func rawOf(t *testing.T, res mcpserver.Result) string {
	t.Helper()
	raw, ok := res.Structured.(json.RawMessage)
	if !ok {
		t.Fatalf("the result is %#v, not the encoded JSON", res)
	}
	return string(raw)
}

// longestPath is the longest path of a page n segments deep (M9/P4 design
// 3.10.4, review B-M8): a page's of ten segments of 255 bytes, 2,562 bytes,
// its parent's of nine, 2,306, each segment all "&", which an answer
// writes in six bytes.
func longestPath(n int) string {
	return strings.TrimSuffix(strings.Repeat(strings.Repeat("&", 255)+"/", n), "/") + ".md"
}

// controls is a string of n C0 controls, which an answer writes in seven
// bytes each.
func controls(n int) string { return strings.Repeat("\x01", n) }

// The module's tools are backlinks and index, each in both entries, a
// notebook's, read-only (M9/P4 design 3.10.5). The composition root's
// TestTheToolListsAreTheContract holds their schemas to the file.
func TestTheToolsAreBacklinksAndIndex(t *testing.T) {
	read := mcpserver.Annotations{ReadOnly: true}
	want := []mcpserver.Spec{
		{Name: "backlinks", Title: "List the pages linking to a page", Description: backlinksDescription, Annotations: read,
			Entry: mcpserver.BothEntries, Notebook: true},
		{Name: "index", Title: "List the notebook's pages by type, tag or parent", Description: indexDescription, Annotations: read,
			Entry: mcpserver.BothEntries, Notebook: true},
	}
	tools := Tools(UseCases{})
	if len(tools) != len(want) {
		t.Fatalf("%d tools, want %d", len(tools), len(want))
	}
	for i, tool := range tools {
		if tool.Spec() != want[i] {
			t.Errorf("tool %d = %+v, want %+v", i, tool.Spec(), want[i])
		}
	}
}

// backlinks' answer (M9/P4 design 3.10.3): each page's path, links and
// contexts, next_cursor the use case's, null without one. The call passes
// its notebook and its arguments, and answers the use case's error as it
// is: page's words for a path's.
func TestTheBacklinksAnswerTheirShape(t *testing.T) {
	b := app.BacklinksAt{Pages: []app.BacklinkingAt{
		{Backlinking: app.Backlinking{PageID: uuid.NewV7(), Links: 1000, Contexts: []string{"see [[Plan]]", "and <[[Plan]]>"}},
			Path: "Notes/A & B.md", Cursor: "c1"},
		{Backlinking: app.Backlinking{PageID: uuid.NewV7(), Links: 1, Contexts: []string{}}, Path: "Old.md", Cursor: "c2"},
	}, NextCursor: "c2"}
	var passed []any
	failed := &mcpserver.ToolError{Err: errors.New("x"), Text: `page.not_found: Nothing is at "Plan.md".`}
	var answer error
	tools := tools{uc: UseCases{Backlinks: backlinksFunc(func(id uuid.UUID, path string, limit *int, cursor *string) (app.BacklinksAt, error) {
		passed = []any{id, path, limit, cursor}
		return b, answer
	})}}
	notebook, limit, cursor := uuid.NewV7(), 2, "c0"
	res, err := tools.backlinks(context.Background(), mcpserver.Call{Notebook: notebook}, backlinksIn{Path: "Plan.md", Limit: &limit,
		Cursor: &cursor})
	if err != nil {
		t.Fatal(err)
	}
	if passed[0] != notebook || passed[1] != "Plan.md" || passed[2] != &limit || passed[3] != &cursor {
		t.Errorf("backlinks passed %v", passed)
	}
	want := `{"pages":[{"path":"Notes/A & B.md","links":1000,"contexts":["see [[Plan]]","and <[[Plan]]>"]},` +
		`{"path":"Old.md","links":1,"contexts":[]}],"next_cursor":"c2"}`
	if raw := rawOf(t, res); raw != want {
		t.Errorf("the answer is\n%s\nwant\n%s", raw, want)
	}
	b = app.BacklinksAt{Pages: []app.BacklinkingAt{}}
	res, err = tools.backlinks(context.Background(), mcpserver.Call{}, backlinksIn{Path: "Plan.md"})
	if raw := rawOf(t, res); err != nil || raw != `{"pages":[],"next_cursor":null}` {
		t.Errorf("no backlinks = %s, %v", raw, err)
	}
	answer = failed
	if _, err := tools.backlinks(context.Background(), mcpserver.Call{}, backlinksIn{Path: "Plan.md"}); err != failed { //nolint:errorlint // the very error, as it is
		t.Errorf("the use case's error answered as %v", err)
	}
}

// A backlink at its largest, the longest path and ten contexts of 240 C0
// controls, about 33 KB, is answered whole; more of them are cut at the
// bound, next_cursor the last kept's cursor (M9/P4 design 3.9.2, 3.10.3).
func TestTheBacklinksFitTheirAnswer(t *testing.T) {
	contexts := make([]string, domain.MaxContexts)
	for i := range contexts {
		contexts[i] = controls(domain.MaxContext)
	}
	pages := make([]app.BacklinkingAt, 5)
	for i := range pages {
		pages[i] = app.BacklinkingAt{Backlinking: app.Backlinking{PageID: uuid.NewV7(), Links: domain.MaxCount, Contexts: contexts},
			Path: longestPath(10), Cursor: "after-" + string(rune('a'+i))}
	}
	var b app.BacklinksAt
	tools := tools{uc: UseCases{Backlinks: backlinksFunc(func(uuid.UUID, string, *int, *string) (app.BacklinksAt, error) { return b, nil })}}
	for _, tt := range []struct {
		name  string
		pages []app.BacklinkingAt
		next  string
		want  int
		after *string
	}{
		{"the largest alone", pages[:1], "", 1, nil},
		{"the largest alone, more after it", pages[:1], "later", 1, new("later")},
		{"five of the largest", pages, "later", 1, new("after-a")},
	} {
		b = app.BacklinksAt{Pages: tt.pages, NextCursor: tt.next}
		res, err := tools.backlinks(context.Background(), mcpserver.Call{}, backlinksIn{Path: "Plan.md"})
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		raw := rawOf(t, res)
		var a backlinksAnswer
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			t.Fatal(err)
		}
		if n := mcpserver.TextBytes(raw); len(a.Pages) != tt.want || n > mcpserver.MaxAnswerBytes || n < 30_000 ||
			(a.NextCursor == nil) != (tt.after == nil) || a.NextCursor != nil && *a.NextCursor != *tt.after {
			t.Errorf("%s: %d pages in %d bytes, next %v; want %d within %d, next %v", tt.name, len(a.Pages), n, a.NextCursor, tt.want,
				mcpserver.MaxAnswerBytes, tt.after)
		}
	}
	b = app.BacklinksAt{Pages: []app.BacklinkingAt{{Path: strings.Repeat("&", 9_000) + ".md", Backlinking: app.Backlinking{Contexts: []string{}}}}}
	if _, err := tools.backlinks(context.Background(), mcpserver.Call{}, backlinksIn{Path: "Plan.md"}); !errors.Is(err, errNoBacklinkFits) {
		t.Errorf("a first page over the bound answered %v", err)
	}
}

// index's answer (M9/P4 design 3.10.4): each page's path, type and summary
// (null for none), tags, changed_at in UTC, inbound links, parent (null at
// the root) and cut; next_cursor the use case's, null without one. The
// call passes its notebook and its arguments, and answers the use case's
// error as it is.
func TestTheIndexAnswersItsShape(t *testing.T) {
	at := time.Date(2026, 10, 22, 10, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	p := app.IndexPage{Items: []app.IndexItem{
		{PageID: uuid.NewV7(), Path: "Projects/Roadmap.md", Type: new("project"), Summary: new("Q4 <plan>"), Tags: []string{"plan/q4", "x"},
			ChangedAt: at, Inbound: 1000, ParentPath: new("Projects.md"), Cut: []string{"summary", "tags[1]"}, Cursor: "c1"},
		{PageID: uuid.NewV7(), Path: "Inbox.md", Tags: []string{}, ChangedAt: at, Cut: []string{}, Cursor: "c2"},
	}, Next: "c2"}
	var passed []any
	var answer error
	tools := tools{uc: UseCases{Index: indexFunc(func(id uuid.UUID, q app.IndexQuery) (app.IndexPage, error) {
		passed = []any{id, q}
		return p, answer
	})}}
	notebook, limit := uuid.NewV7(), 2
	in := indexIn{Type: new("project"), Tag: new("#plan"), Parent: new("/"), Limit: &limit, Cursor: new("c0")}
	res, err := tools.index(context.Background(), mcpserver.Call{Notebook: notebook}, in)
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := passed[1].(app.IndexQuery); passed[0] != notebook || !ok || q.Type != in.Type || q.Tag != in.Tag || q.Parent != in.Parent ||
		q.Limit != in.Limit || q.Cursor != in.Cursor {
		t.Errorf("index passed %v", passed)
	}
	want := `{"pages":[{"path":"Projects/Roadmap.md","type":"project","summary":"Q4 <plan>","tags":["plan/q4","x"],` +
		`"changed_at":"2026-10-22T08:30:00Z","inbound_links":1000,"parent":"Projects.md","cut":["summary","tags[1]"]},` +
		`{"path":"Inbox.md","type":null,"summary":null,"tags":[],"changed_at":"2026-10-22T08:30:00Z","inbound_links":0,` +
		`"parent":null,"cut":[]}],"next_cursor":"c2"}`
	if raw := rawOf(t, res); raw != want {
		t.Errorf("the answer is\n%s\nwant\n%s", raw, want)
	}
	p = app.IndexPage{Items: []app.IndexItem{}}
	res, err = tools.index(context.Background(), mcpserver.Call{}, indexIn{})
	if raw := rawOf(t, res); err != nil || raw != `{"pages":[],"next_cursor":null}` {
		t.Errorf("no pages = %s, %v", raw, err)
	}
	failed := errors.New("boom")
	answer = failed
	if _, err := tools.index(context.Background(), mcpserver.Call{}, indexIn{}); err != failed { //nolint:errorlint // the very error, as it is
		t.Errorf("the use case's error answered as %v", err)
	}
}

// largestItem is an index's item at its largest but for its tags (M9/P4
// design 3.10.4, review B-M8): the longest path and parent, type and
// summary of 256 C0 controls; with tags, twenty of 256 C0 controls, cut
// naming the first's and that there were more.
func largestItem(cursor string, tagged bool) app.IndexItem {
	it := app.IndexItem{PageID: uuid.NewV7(), Path: longestPath(10), Type: new(controls(domain.MaxIndexString)),
		Summary: new(controls(domain.MaxIndexString)), Tags: []string{}, ChangedAt: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC),
		Inbound: domain.MaxInbound, ParentPath: new(longestPath(9)), Cut: []string{"type", "summary"}, Cursor: cursor}
	if tagged {
		for range domain.MaxIndexTags {
			it.Tags = append(it.Tags, controls(domain.MaxIndexString))
		}
		it.Cut = append(it.Cut, "tags[0]", "tags")
	}
	return it
}

// An answer holds as many pages as it can (M9/P4 design 3.10.4): pages at
// their largest but for their tags, about 33 KB, one alone; next_cursor
// the last kept's cursor, or the use case's when it holds them all. A
// first page the answer cannot hold, its twenty tags of 256 C0 controls,
// about 69 KB, comes alone without its tags, "tags" in its cut in place
// of its tags' entries, next_cursor after it: each call goes on by a page
// (mutant 182: without that, an internal error).
func TestTheIndexFitsItsAnswer(t *testing.T) {
	var p app.IndexPage
	tools := tools{uc: UseCases{Index: indexFunc(func(uuid.UUID, app.IndexQuery) (app.IndexPage, error) { return p, nil })}}
	whole, _ := json.Marshal(indexAnswer{Pages: []indexPage{indexPageOf(largestItem("c", true))}})
	if n := mcpserver.TextBytes(string(whole)); n <= mcpserver.MaxAnswerBytes {
		t.Fatalf("a page with its tags is %d bytes, within %d: the rows tell nothing", n, mcpserver.MaxAnswerBytes)
	}
	for _, tt := range []struct {
		name   string
		items  []app.IndexItem
		next   string
		after  *string
		tagged bool // the first page comes with its tags
	}{
		{"the largest alone", []app.IndexItem{largestItem("c1", false)}, "", nil, true},
		{"three of the largest", []app.IndexItem{largestItem("c1", false), largestItem("c2", false), largestItem("c3", false)}, "later",
			new("c1"), true},
		{"one too large", []app.IndexItem{largestItem("c1", true)}, "", nil, false},
		{"one too large, more after it", []app.IndexItem{largestItem("c1", true)}, "c1", new("c1"), false},
		{"one too large and another", []app.IndexItem{largestItem("c1", true), largestItem("c2", false)}, "", new("c1"), false},
	} {
		p = app.IndexPage{Items: tt.items, Next: tt.next}
		res, err := tools.index(context.Background(), mcpserver.Call{}, indexIn{})
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		raw := rawOf(t, res)
		var a indexAnswer
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			t.Fatal(err)
		}
		if n := mcpserver.TextBytes(raw); len(a.Pages) != 1 || n > mcpserver.MaxAnswerBytes || n < 30_000 ||
			(a.NextCursor == nil) != (tt.after == nil) || a.NextCursor != nil && *a.NextCursor != *tt.after {
			t.Errorf("%s: %d pages in %d bytes, next %v; want one within %d, next %v", tt.name, len(a.Pages), n, a.NextCursor,
				mcpserver.MaxAnswerBytes, tt.after)
			continue
		}
		first, want := a.Pages[0], tt.items[0]
		wantCut := []string{"type", "summary"}
		if !tt.tagged {
			wantCut = append(wantCut, "tags")
		}
		if first.Path != want.Path || first.Summary == nil || *first.Summary != *want.Summary || !slices.Equal(first.Cut, wantCut) ||
			len(first.Tags) != 0 {
			t.Errorf("%s: the first page cut %v with %d tags; want cut %v, no tags", tt.name, first.Cut, len(first.Tags), wantCut)
		}
	}
	p = app.IndexPage{Items: []app.IndexItem{{Path: strings.Repeat("&", 9_000) + ".md", Tags: []string{}, Cut: []string{}}}}
	if _, err := tools.index(context.Background(), mcpserver.Call{}, indexIn{}); !errors.Is(err, errNoIndexPageFits) {
		t.Errorf("a first page over the bound without its tags answered %v", err)
	}
}
