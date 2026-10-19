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

// wordsOf is a tool's error in the module's words, as mcpserver reads
// it: its text and the members it adds, all of its items when it has a
// fit; "" for an error that is as it is.
func wordsOf(err error) (string, map[string]any) {
	var te *mcpserver.ToolError
	switch {
	case !errors.As(err, &te):
		return "", nil
	case te.Fit != nil:
		return te.Fit.Build(te.Fit.N)
	}
	return te.Text, te.Extra
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
// (M9/P1 design 3.10.5, 3.11.3, 3.17.3); write, edit, move and delete,
// in both entries, destructive, all but edit idempotent (M9/P2 design
// 3.11); and versions, log and changeset, in both entries, read-only
// (3.16). The composition root's TestTheToolListsAreTheContract holds
// their schemas to the file.
func TestTheToolsAreTheReadsAndTheWrites(t *testing.T) {
	tools := Tools(UseCases{}, nil)
	write, read := mcpserver.Annotations{Destructive: true, Idempotent: true}, mcpserver.Annotations{ReadOnly: true}
	want := []mcpserver.Spec{
		{Name: "list", Title: "List nodes", Description: listDescription, Annotations: mcpserver.Annotations{ReadOnly: true},
			Entry: mcpserver.BothEntries, Notebook: true},
		{Name: "read", Title: "Read a page or an attachment", Description: readDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.BothEntries, Notebook: true},
		{Name: "open_notebook", Title: "Open a notebook", Description: openNotebookDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.AccountEntry, Notebook: true},
		{Name: "write", Title: "Write a page", Description: writeDescription, Annotations: write, Entry: mcpserver.BothEntries,
			Notebook: true},
		{Name: "edit", Title: "Edit a page", Description: editDescription, Annotations: mcpserver.Annotations{Destructive: true},
			Entry: mcpserver.BothEntries, Notebook: true},
		{Name: "move", Title: "Move or rename a page or an attachment", Description: moveDescription, Annotations: write,
			Entry: mcpserver.BothEntries, Notebook: true},
		{Name: "delete", Title: "Delete a page or an attachment", Description: deleteDescription, Annotations: write,
			Entry: mcpserver.BothEntries, Notebook: true},
		{Name: "versions", Title: "List or read a page's versions", Description: versionsDescription, Annotations: read,
			Entry: mcpserver.BothEntries, Notebook: true},
		{Name: "log", Title: "List the notebook's changes", Description: logDescription, Annotations: read,
			Entry: mcpserver.BothEntries, Notebook: true},
		{Name: "changeset", Title: "Read a changeset", Description: changesetDescription, Annotations: read,
			Entry: mcpserver.BothEntries, Notebook: true},
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
	if text, _ := wordsOf(err); text != `page.not_found: Nothing is at "A/".` ||
		listed[0] != notebook || listed[1] != "A/" || listed[2] != &limit || listed[3] != &cursor {
		t.Errorf("list passed %v and answered %v", listed, err)
	}
	if _, err := tools.list(context.Background(), call, listIn{}); err == nil || listed[1] != "" || listed[2] != (*int)(nil) {
		t.Errorf("list without arguments passed %v", listed)
	}
	_, err = tools.read(context.Background(), call, readIn{Path: "A.md", Offset: &offset, Column: &column, Limit: &limit})
	if text, _ := wordsOf(err); text != `page.not_found: Nothing is at "A.md".` ||
		read[0] != notebook || read[1] != "A.md" || read[2] != &offset || read[3] != &column || read[4] != &limit {
		t.Errorf("read passed %v and answered %v", read, err)
	}
}

// page's domain holds what mcpserver names, which it cannot import: the
// name of a client that told none (M9/P2 design 3.3.1), and how many of a
// refusal's locks have their paths read, the locks an answer holds (3.8.5).
func TestTheDomainsNamesAreMCPServers(t *testing.T) {
	if domain.UnknownClientName != mcpserver.UnknownClient {
		t.Errorf("domain.UnknownClientName = %q, mcpserver.UnknownClient %q", domain.UnknownClientName, mcpserver.UnknownClient)
	}
	if domain.MaxNamedLocks != mcpserver.MaxErrorLocks {
		t.Errorf("domain.MaxNamedLocks = %d, mcpserver.MaxErrorLocks %d", domain.MaxNamedLocks, mcpserver.MaxErrorLocks)
	}
}
