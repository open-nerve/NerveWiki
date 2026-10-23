// Package mcpadapter is the linking module's MCP adapter (M9 design 4.1,
// 4.9): the tools backlinks and index, their arguments and their answers
// within mcpserver's bound. The use cases answer errors in their words, a
// path's in page's tools' own (M9/P4 design 3.9.1): the tools answer them
// as they are. It imports mcpserver, never the SDK.
package mcpadapter

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// BacklinksUseCase lists the pages that link to the page at a path.
type BacklinksUseCase interface {
	AtPath(ctx context.Context, notebookID uuid.UUID, path string, limit *int, cursor *string) (app.BacklinksAt, error)
}

// IndexUseCase lists the pages of a notebook's index.
type IndexUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID, q app.IndexQuery) (app.IndexPage, error)
}

// UseCases are the use cases the tools call.
type UseCases struct {
	Backlinks BacklinksUseCase
	Index     IndexUseCase
}

// Tools are the module's MCP tools, backlinks and index, each in both
// entries, a notebook's, read-only (M9/P4 design 3.10.3, 3.10.4).
func Tools(uc UseCases) []mcpserver.Tool {
	t := tools{uc: uc}
	read := mcpserver.Annotations{ReadOnly: true}
	return []mcpserver.Tool{
		mcpserver.NewTool(mcpserver.Spec{Name: "backlinks", Title: "List the pages linking to a page", Description: backlinksDescription,
			Annotations: read, Entry: mcpserver.BothEntries, Notebook: true}, t.backlinks),
		mcpserver.NewTool(mcpserver.Spec{Name: "index", Title: "List the notebook's pages by type, tag or parent",
			Description: indexDescription, Annotations: read, Entry: mcpserver.BothEntries, Notebook: true}, t.index),
	}
}

// tools are the handlers of the module's tools.
type tools struct {
	uc UseCases
}

// The tools' descriptions, as api/mcp/tools.json has them (M9/P4 design
// 3.11).
const (
	backlinksDescription = `List the pages that link to a page: each with its path, how many of its links lead there (1000 means 1000 or ` +
		`more) and the lines around the first ten of them, each at most 240 bytes. Only a page has backlinks: an ` +
		`attachment's path is not found. A page's links to itself are left out; a page written since the link index ` +
		`read it has no lines yet. Go on with next_cursor while it is set.`
	indexDescription = `List the notebook's pages as its link index holds them: each with its path; its frontmatter's type and ` +
		`summary, the first key of that name whatever its case, when its value is text; its tags, 20 at most; when ` +
		`its content last changed; how many other pages link to it (1000 means 1000 or more); and its parent page's ` +
		`path. type, summary and each tag are cut to 256 bytes, cut naming what was cut. type keeps the pages of ` +
		`exactly that type; tag those with that tag or one under it ("plan" takes "plan/q4"; a leading # is dropped); ` +
		`parent the child pages of that page, "/" those at the root. Omit a filter rather than give it empty: "" is ` +
		`refused. The pages with a type come first, by type, then the others. A page the index has not read yet, as ` +
		`one written before nervewiki reindex after an upgrade, is not listed. Go on with next_cursor while it is ` +
		`set.`
)

// nextCursor is the next_cursor of an answer that holds k of n items,
// each with the cursor after it: the use case's next when it holds all of
// them, the cursor after the last it holds when it leaves some out, null
// when it holds none of some.
func nextCursor(k, n int, cursors []string, next string) *string {
	switch {
	case k == 0 && n > 0:
		return nil
	case k < n:
		next = cursors[k-1]
	}
	return cursorOf(next)
}

// cursorOf is a cursor as an answer writes it: null for none.
func cursorOf(cursor string) *string {
	if cursor == "" {
		return nil
	}
	return &cursor
}
