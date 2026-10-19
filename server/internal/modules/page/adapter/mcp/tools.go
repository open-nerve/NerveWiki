// Package mcpadapter is the page module's MCP adapter (M9 design 4.1,
// 4.6, 4.9, 4.10): the tools list, read and open_notebook, write, edit,
// move and delete (M9/P2 design 3.11), and versions, log and changeset,
// the history's reads (3.16), their arguments, their answers within
// mcpserver's bound, and the words of their errors; the built-in
// instructions. It imports mcpserver, never the SDK.
package mcpadapter

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
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

// WritePathUseCase writes a page's whole content by its path.
type WritePathUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID, p app.PathWrite, client domain.Client) (app.Written, error)
}

// EditPageUseCase replaces a string in a page's content.
type EditPageUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID, p app.PageEdit, client domain.Client) (app.Edited, error)
}

// RelocatePathUseCase moves or renames a node to its full new path.
type RelocatePathUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID, p app.PathMove, client domain.Client) (app.Relocated, error)
}

// DeletePathUseCase puts a node in the trash by its path.
type DeletePathUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID, p app.PathDelete, client domain.Client) (app.Deleted, error)
}

// BatchUseCase applies a batch of writes as one changeset (M9 design
// 4.11).
type BatchUseCase interface {
	Execute(ctx context.Context, notebookID uuid.UUID, p app.PathBatch, client domain.Client) (app.Batched, error)
}

// VersionsUseCase lists the versions of the page at a path.
type VersionsUseCase interface {
	AtPath(ctx context.Context, notebookID uuid.UUID, path string, limit *int, cursor *string) (app.PathVersionList, error)
}

// VersionUseCase reads a version of the page at a path.
type VersionUseCase interface {
	AtPath(ctx context.Context, notebookID uuid.UUID, path string, revision int, offset, column, limit *int, m app.TextMeasure) (
		app.PathVersion, error)
}

// LogUseCase lists a notebook's changesets, a node by its path.
type LogUseCase interface {
	AtPath(ctx context.Context, notebookID uuid.UUID, q app.PathLogQuery) (app.PathLog, error)
}

// ChangesetUseCase reads a notebook's changeset and a page of its changes.
type ChangesetUseCase interface {
	InNotebook(ctx context.Context, notebookID, id uuid.UUID, limit *int, cursor *string) (app.ChangesetPage, error)
}

// UseCases are the use cases the tools call.
type UseCases struct {
	List      ListByPatternUseCase
	Read      ReadPathUseCase
	Open      OpenNotebookUseCase
	Write     WritePathUseCase
	Edit      EditPageUseCase
	Move      RelocatePathUseCase
	Delete    DeletePathUseCase
	Batch     BatchUseCase
	Versions  VersionsUseCase
	Version   VersionUseCase
	Log       LogUseCase
	Changeset ChangesetUseCase
}

// Tools are the module's MCP tools, each a notebook's: list and read, in
// both entries, read's attachments read by attachments; open_notebook, in
// the account entry alone, where an agent picks its notebook; write, edit,
// move and delete, in both entries, each destructive, all but edit
// idempotent: called again, write writes nothing, move and delete find
// nothing at their path (M9/P2 design 3.11); versions, log and changeset,
// in both entries, read-only: the history is read here, restored and
// reverted in the web (3.16).
func Tools(uc UseCases, attachments app.AttachmentReader) []mcpserver.Tool {
	t := tools{uc: uc, attachments: attachments}
	return []mcpserver.Tool{
		mcpserver.NewTool(mcpserver.Spec{Name: "list", Title: "List nodes", Description: listDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.BothEntries, Notebook: true}, t.list),
		mcpserver.NewTool(mcpserver.Spec{Name: "read", Title: "Read a page or an attachment", Description: readDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.BothEntries, Notebook: true}, t.read),
		mcpserver.NewTool(mcpserver.Spec{Name: "open_notebook", Title: "Open a notebook", Description: openNotebookDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.AccountEntry, Notebook: true}, t.openNotebook),
		mcpserver.NewTool(mcpserver.Spec{Name: "write", Title: "Write a page", Description: writeDescription,
			Annotations: mcpserver.Annotations{Destructive: true, Idempotent: true}, Entry: mcpserver.BothEntries, Notebook: true}, t.write),
		mcpserver.NewTool(mcpserver.Spec{Name: "edit", Title: "Edit a page", Description: editDescription,
			Annotations: mcpserver.Annotations{Destructive: true}, Entry: mcpserver.BothEntries, Notebook: true}, t.edit),
		mcpserver.NewTool(mcpserver.Spec{Name: "move", Title: "Move or rename a page or an attachment", Description: moveDescription,
			Annotations: mcpserver.Annotations{Destructive: true, Idempotent: true}, Entry: mcpserver.BothEntries, Notebook: true}, t.move),
		mcpserver.NewTool(mcpserver.Spec{Name: "delete", Title: "Delete a page or an attachment", Description: deleteDescription,
			Annotations: mcpserver.Annotations{Destructive: true, Idempotent: true}, Entry: mcpserver.BothEntries, Notebook: true}, t.delete),
		mcpserver.NewTool(mcpserver.Spec{Name: "versions", Title: "List or read a page's versions", Description: versionsDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.BothEntries, Notebook: true}, t.versions),
		mcpserver.NewTool(mcpserver.Spec{Name: "log", Title: "List the notebook's changes", Description: logDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.BothEntries, Notebook: true}, t.log),
		mcpserver.NewTool(mcpserver.Spec{Name: "changeset", Title: "Read a changeset", Description: changesetDescription,
			Annotations: mcpserver.Annotations{ReadOnly: true}, Entry: mcpserver.BothEntries, Notebook: true}, t.changeset),
	}
}

// changesetOf is a write's changeset and time as an answer gives them:
// both null when nothing changed, the time in UTC.
func changesetOf(o app.Outcome) (*string, *time.Time) {
	if o.ChangesetID == (uuid.UUID{}) {
		return nil, nil
	}
	return new(o.ChangesetID.String()), new(o.At.UTC())
}

// tools are the handlers of the module's tools.
type tools struct {
	uc          UseCases
	attachments app.AttachmentReader
}

// The tools' descriptions, as api/mcp/tools.json has them (M9/P1 design
// 3.18, M9/P2 design 3.18).
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
		`A page comes as two text blocks: a line of JSON metadata (path, kind, version, offset, column, lines, ` +
		`total_lines, byte_size, next_offset, next_column), then its lines exactly as stored, frontmatter, byte order ` +
		`mark and CRLF included. offset is the first line, from 1; limit the number of lines, 2000 by default. An answer ` +
		`holds about 48 KiB at most, its metadata line included: go on from next_offset, and from next_column when a line ` +
		`was too long for one answer; the parts read in turn are the whole content. An attachment comes as its metadata ` +
		`(type, size, SHA-256, width and height, how to link it, a download address valid for an hour or two), then the ` +
		`image itself for a PNG, JPEG, GIF or WebP of at most 3 MiB and 8000 pixels a side, or else a link to download ` +
		`it. Without the server's public address set (server.public_url) there is no download address: an image still ` +
		`comes itself, any other attachment as its metadata only, with a note saying so.`
	writeDescription = `Write a page's whole content by its path from the notebook's root; the path ends with .md. Without ` +
		`base_version, it creates the page, and the parent pages it lacks, empty, in one change; a path that is a ` +
		`page already is refused. With base_version, the version read, list or write gave, it replaces the ` +
		`content of the page at the path: refused when the page changed since, or when the path names another ` +
		`page now; read it again. When the page holds that content already, nothing is written and changed is ` +
		`false, with an older base_version of that page too, and while the page is being edited. The content is ` +
		`stored byte for byte: at most 5 MiB of UTF-8, without NUL. A page being edited in the web editor, by you ` +
		`too, refuses the write, saying by whom; the lock cannot be taken over. The answer has the page's path, ` +
		`node_id and version, changed (false when nothing was written), the nodes it created, and the changeset's ` +
		`id and time.`
	editDescription = `Replace old_string with new_string in a page's content on the server, byte for byte, as a find and ` +
		`replace: old_string must occur exactly once (occurrences that overlap count), unless replace_all is set, ` +
		`which replaces each, left to right. Copy old_string from what read gave, with enough of the text around ` +
		`it to be unique: quotes and spaces are matched as they are. On a page whose line breaks are all CRLF and an ` +
		`old_string without CR, old_string's bare LFs are matched as CRLF and new_string's are written as CRLF (a CR ` +
		`in new_string stays); with a CR in old_string, both are taken as given. Without base_version, the edit ` +
		`applies to the content as it is now, made once more if the page changed while it was made; with ` +
		`base_version, it is refused when the page changed since. A page being edited in the web editor, by you ` +
		`too, refuses the edit, saying by whom; the lock cannot be taken over. The answer has the page's version, ` +
		`how many times old_string was replaced, changed (false when the content stayed as it was), and the ` +
		`changeset's id and time.`
	moveDescription = `Move, rename, or both at once, a page or an attachment: from is its path, to the full path it will have, ` +
		`both from the notebook's root ("Archive/Plan 2026.md"), never a page to move it into. A page's path ends ` +
		`with .md, and an attachment's keeps an extension, which may change. A page moves with its children. The ` +
		`page above to must exist: move creates no pages. The links to what moves, and to its pages, are ` +
		`rewritten to follow it, unless update_links is false, which leaves every page linking to it as it is: ` +
		`those links may then lead elsewhere, or nowhere. Rewriting the links of a page being edited in the web ` +
		`editor, by you too, is refused, saying by whom; the lock cannot be taken over. The answer has the new ` +
		`path, node_id, changed, and the changeset's id and time (null when nothing moved).`
	deleteDescription = `Put a page or an attachment in the notebook's trash, by its path from the notebook's root: people ` +
		`restore it from the trash in the web. A page with pages or attachments under it needs recursive set to ` +
		`true, which puts them in the trash with it. Any of its pages being edited in the web editor, by you too, ` +
		`refuses the deletion, saying by whom; the locks cannot be taken over. The answer has how many pages and ` +
		`attachments went to the trash, itself counted, and the changeset's id and time.`
	versionsDescription = `Without revision, list a page's versions, newest first: each with its revision, size, ` +
		`times, and the changeset that wrote it (its kind, client and author); the answer also has the page's ` +
		`current version. Go on with next_cursor while it is set. With revision, read that version's content as ` +
		`read reads a page: two text blocks, a line of JSON metadata (path, kind, revision, current_version, ` +
		`offset, column, lines, total_lines, byte_size, next_offset, next_column), then its lines exactly as ` +
		`stored; go on from next_offset, and next_column. The version answered has its own revision, which may be ` +
		`above the one asked for: an edit session's saves share one version. Revisions are for reading: write and ` +
		`edit take the page's version, from read. A page in the trash has no path: its versions are read in the ` +
		`web.`
	logDescription = `List the notebook's changesets, newest first: what people and agents changed, each with its ` +
		`kind, client, author (id and display name), times, message, how many nodes and contents it changed, what ` +
		`it reverts and what reverted it, and its first three changes; a changeset too large for the answer comes ` +
		`alone, its changes cut to fit and changes_cut true. path keeps the changesets that changed that page or ` +
		`attachment; actor those of an account, "me" for yours; client those of a client, "mcp" for every agent. ` +
		`A page or an attachment in the trash has no path: find its changesets in the whole log. Go on with ` +
		`next_cursor while it is set. changeset reads one in full. History is read-only here: people restore ` +
		`versions and revert changesets in the web.`
	changesetDescription = `Read one of the notebook's changesets by the id log gives: its kind, client, author, ` +
		`times, message, counts, what it reverts and what reverted it, and whether it renames or moves nodes; then ` +
		`what it did to each node it changed, a page at a time by node id: where the node was before and after in ` +
		`the tree, the revision of the content it wrote, and the node's current path (null when it is in the ` +
		`trash). The changeset itself comes on the first page, without cursor; go on with next_cursor while it is ` +
		`set. A changeset of another notebook is not found here. History is read-only here: people revert ` +
		`changesets in the web.`
)
