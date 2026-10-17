// Package mcpadapter is the page module's MCP adapter (M9 design 4.1,
// 4.9): the tools list and read, their arguments, their answers within
// mcpserver's bound, and the words of their errors. It imports mcpserver,
// never the SDK.
package mcpadapter

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// ListByPatternUseCase lists a notebook's nodes by a pattern.
type ListByPatternUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID, pattern string, limit *int, cursor *string) (app.Listing, error)
}

// ReadPathUseCase reads the node at a path.
type ReadPathUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID, path string, offset, column, limit *int, m app.TextMeasure) (app.Read, error)
}

// UseCases are the use cases the tools call.
type UseCases struct {
	List ListByPatternUseCase
	Read ReadPathUseCase
}

// Tools are the module's MCP tools, each in both entries, a notebook's:
// list and read, whose attachments attachments reads.
func Tools(uc UseCases, attachments app.AttachmentReader) []mcpserver.Tool {
	t := tools{uc: uc, attachments: attachments}
	return []mcpserver.Tool{
		mcpserver.NewTool(mcpserver.Spec{Name: "list", Title: "List nodes", Description: listDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.BothEntries, Notebook: true}, t.list),
		mcpserver.NewTool(mcpserver.Spec{Name: "read", Title: "Read a page or an attachment", Description: readDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.BothEntries, Notebook: true}, t.read),
	}
}

// tools are the handlers of the module's tools.
type tools struct {
	uc          UseCases
	attachments app.AttachmentReader
}

// The tools' descriptions, as api/mcp/tools.json has them (M9/P1 design
// 3.18).
const (
	listDescription = `List the notebook's pages and attachments. With pattern empty or "/", the root's children; ` +
		`with "A/B/", the children of the page A/B.md; both in the tree's order. Any other pattern is a glob over the paths, ` +
		`in path order: * matches within a segment, ? one character, ** any number of segments as a segment of its own ` +
		`("**/*.md" is every page, "A/**" everything under A). Matching ignores case. Each item has its path, its kind ` +
		`(page or attachment) and whether it has children; a page also its version, size and last change, an attachment its ` +
		`type and size. An answer holds at most limit items and about 48 KiB, and a costly glob over a large notebook stops ` +
		`after a bounded amount of matching, maybe with no items: whenever next_cursor is set, pass it back as cursor to go ` +
		`on. A path without wildcards that names nothing may come with a hint, as when it names a page without its .md.`
	readDescription = `Read a page or an attachment by its path from the notebook's root; a page's path ends with .md. ` +
		`A page comes as two text blocks: a line of JSON metadata (path, kind, version, offset, column, lines, total_lines, ` +
		`byte_size, next_offset, next_column), then its lines exactly as stored, frontmatter, byte order mark and CRLF ` +
		`included. offset is the first line, from 1; limit the number of lines, 2000 by default. An answer holds about 45 ` +
		`KiB of text at most: go on from next_offset, and from next_column when a line was too long for one answer; the ` +
		`parts read in turn are the whole content. An attachment comes as its metadata (type, size, SHA-256, width and ` +
		`height, how to link it, a download address valid for an hour or two), then the image itself for a PNG, JPEG, GIF ` +
		`or WebP of at most 3 MiB and 8000 pixels a side, or else a link to download it. Without the server's public ` +
		`address set (server.public_url), an attachment comes as its metadata only, with a note saying so.`
)
