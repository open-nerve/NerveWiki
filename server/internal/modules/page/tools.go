package page

import (
	mcpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/mcp"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// AttachmentReader reads an attachment for read: bootstrap adapts asset's
// AttachmentReads to it, asset.not_found to ErrNotFound and its ErrNoFile
// to ErrNoAttachmentFile (M9/P1 design 3.11.2).
type AttachmentReader = app.AttachmentReader

// Attachment is an attachment as read tells it.
type Attachment = app.Attachment

// ErrNoAttachmentFile is an attachment whose file is not in the store.
var ErrNoAttachmentFile = app.ErrNoAttachmentFile

// ErrNotFound is page.not_found.
var ErrNotFound = domain.ErrNotFound

// ToolDeps are what page's tools need beyond the module's use cases: asset
// is wired after page (v0.1 design 13.1, item 11).
type ToolDeps struct{ Attachments AttachmentReader }

// Tools are page's MCP tools, list, read (M9 design 4.9) and
// open_notebook (4.6), write, edit, move and delete (4.10), and versions,
// log and changeset (4.9); no Attachments panics.
func (m *Module) Tools(d ToolDeps) []mcpserver.Tool {
	if d.Attachments == nil {
		panic("page: no attachment reader for the MCP tools")
	}
	return mcpadapter.Tools(mcpadapter.UseCases{List: m.list, Read: m.read, Open: m.open, Write: m.write, Edit: m.edit,
		Move: m.move, Delete: m.remove, Batch: m.batch, Versions: m.versions, Version: m.version, Log: m.log,
		Changeset: m.changeset}, d.Attachments)
}
