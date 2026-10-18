package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// listFunc is a ListByPattern of a function.
type listFunc func(notebookID uuid.UUID, pattern string, limit *int, cursor *string) (app.Listing, error)

func (f listFunc) Execute(_ context.Context, notebookID uuid.UUID, pattern string, limit *int, cursor *string) (app.Listing, error) {
	return f(notebookID, pattern, limit, cursor)
}

// readFunc is a ReadPath of a function.
type readFunc func(notebookID uuid.UUID, path string, offset, column, limit *int, m app.TextMeasure) (app.Read, error)

func (f readFunc) Execute(_ context.Context, notebookID uuid.UUID, path string, offset, column, limit *int, m app.TextMeasure) (app.Read, error) {
	return f(notebookID, path, offset, column, limit, m)
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

// answerOf decodes a structured result's JSON.
func answerOf(t *testing.T, res mcpserver.Result) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(rawOf(t, res)), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// blockJSON decodes a block's text, a line of JSON.
func blockJSON(t *testing.T, b mcpserver.Block) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(b.Text), &out); err != nil {
		t.Fatalf("the block %q: %v", b.Text, err)
	}
	return out
}

// The module's tools are list and read, each in both entries, and
// open_notebook, in the account entry alone, each a notebook's, read-only
// (M9/P1 design 3.10.5, 3.11.3, 3.17.3); the composition root's
// TestTheToolListsAreTheContract holds their schemas to the file.
func TestTheToolsAreListReadAndOpenNotebook(t *testing.T) {
	tools := Tools(UseCases{}, nil)
	want := []mcpserver.Spec{
		{Name: "list", Title: "List nodes", Description: listDescription, Annotations: mcpserver.Annotations{ReadOnly: true},
			Entry: mcpserver.BothEntries, Notebook: true},
		{Name: "read", Title: "Read a page or an attachment", Description: readDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.BothEntries, Notebook: true},
		{Name: "open_notebook", Title: "Open a notebook", Description: openNotebookDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.AccountEntry, Notebook: true},
	}
	if len(tools) != len(want) {
		t.Fatalf("%d tools, want %d", len(tools), len(want))
	}
	for i, tool := range tools {
		if tool.Spec() != want[i] {
			t.Errorf("tool %d = %+v, want %+v", i, tool.Spec(), want[i])
		}
	}
}

// A path that names nothing is worded with the path as it was written and
// its hint (M9/P1 design 3.10.5); page.not_found stays its code; any other
// error is the use case's.
func TestAPathThatNamesNothingIsWorded(t *testing.T) {
	for _, tt := range []struct {
		missing *domain.PathNotFound
		want    string
	}{
		{&domain.PathNotFound{Path: "A/B/"}, `page.not_found: Nothing is at "A/B/".`},
		{&domain.PathNotFound{Path: "Guide", Hint: domain.PageHint("Guide.md")},
			`page.not_found: Nothing is at "Guide". A page's path ends with .md: "Guide.md".`},
	} {
		var te *mcpserver.ToolError
		err := toolErr(tt.missing)
		if !errors.As(err, &te) || te.Text != tt.want || !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("toolErr(%+v) = %#v, want the text %q", tt.missing, err, tt.want)
		}
	}
	for _, err := range []error{errors.New("boom"), domain.ErrNotFound} {
		var te *mcpserver.ToolError
		if got := toolErr(err); errors.As(got, &te) || !errors.Is(got, err) {
			t.Errorf("toolErr(%v) = %#v, want it as it is", err, got)
		}
	}
}

// Both tools pass their call's notebook and arguments to their use case,
// and answer its path's not-found in the module's words.
func TestTheToolsPassTheirArguments(t *testing.T) {
	notebook := uuid.NewV7()
	limit, cursor, offset, column := 3, "abc", 2, 5
	var listed, read []any
	tools := tools{uc: UseCases{
		List: listFunc(func(id uuid.UUID, pattern string, l *int, c *string) (app.Listing, error) {
			listed = []any{id, pattern, l, c}
			return app.Listing{}, &domain.PathNotFound{Path: pattern}
		}),
		Read: readFunc(func(id uuid.UUID, path string, o, c, l *int, _ app.TextMeasure) (app.Read, error) {
			read = []any{id, path, o, c, l}
			return app.Read{}, &domain.PathNotFound{Path: path}
		}),
	}}
	call := mcpserver.Call{Notebook: notebook}
	_, err := tools.list(context.Background(), call, listIn{Pattern: new("A/"), Limit: &limit, Cursor: &cursor})
	var te *mcpserver.ToolError
	if !errors.As(err, &te) || te.Text != `page.not_found: Nothing is at "A/".` ||
		listed[0] != notebook || listed[1] != "A/" || listed[2] != &limit || listed[3] != &cursor {
		t.Errorf("list passed %v and answered %v", listed, err)
	}
	if _, err := tools.list(context.Background(), call, listIn{}); err == nil || listed[1] != "" || listed[2] != (*int)(nil) {
		t.Errorf("list without arguments passed %v", listed)
	}
	_, err = tools.read(context.Background(), call, readIn{Path: "A.md", Offset: &offset, Column: &column, Limit: &limit})
	if !errors.As(err, &te) || te.Text != `page.not_found: Nothing is at "A.md".` ||
		read[0] != notebook || read[1] != "A.md" || read[2] != &offset || read[3] != &column || read[4] != &limit {
		t.Errorf("read passed %v and answered %v", read, err)
	}
}
