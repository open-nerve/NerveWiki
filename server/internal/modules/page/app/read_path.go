package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ReadPath reads a page by its path for MCP's read (M9 design 4.9), or
// finds the attachment there, which the adapter reads through asset.
type ReadPath struct {
	notebooks Notebooks
	paths     NodePaths
	nodes     Nodes
	auth      shared.Authorizer
}

// NewReadPath returns the use case.
func NewReadPath(notebooks Notebooks, paths PathStore, nodes Nodes, auth shared.Authorizer) *ReadPath {
	return &ReadPath{notebooks: notebooks, paths: NewNodePaths(paths), nodes: nodes, auth: auth}
}

// TextMeasure is how read's answer counts its text, which page's
// adapter/mcp gives from mcpserver: page counts no bytes of its own. Bytes
// is a text block's measure of s, each character on its own; Room the
// measure the answer leaves the slice of the page at path, its metadata
// line out.
type TextMeasure struct {
	Bytes func(s string) int
	Room  func(path string) int
}

// Read is a read's node, at Path, as its names are: a page's slice, or an
// attachment to read, Page nil.
type Read struct {
	Path string
	Node domain.Node
	Page *PageRead
}

// PageRead is what read gives of a page: its version, the window asked
// for, the slice of its content in it, and the content's bytes.
type PageRead struct {
	Version domain.VersionToken
	Window  domain.Window
	Slice   domain.Slice
	Bytes   int
}

// Execute reads the node at path in the notebook notebookID, unlocked,
// without a transaction (M9/P1 design 3.11.1). Its order: the actor; the
// path, 422, a trailing "/" refused; the window's offset, column and
// limit, 422; the notebook, notebook.not_found, page.read, which every
// role may (no 403: a node not seen is not found); the node,
// page.not_found; a page's content, page.not_found when it went since the
// node was read; the column on its line, 422; the slice, within m's room.
// An attachment is answered as its node: its reads are asset's.
func (r *ReadPath) Execute(ctx context.Context, notebookID uuid.UUID, path string, offset, column, limit *int, m TextMeasure) (Read, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Read{}, err
	}
	p, err := domain.ParsePath("path", path, false)
	if err != nil {
		return Read{}, err
	}
	w, err := domain.CheckWindow(offset, column, limit)
	if err != nil {
		return Read{}, err
	}
	if err := decideNotebook(ctx, r.notebooks, r.auth, actor, domain.ActionRead, notebookID); err != nil {
		return Read{}, err
	}
	at, err := r.paths.Find(ctx, notebookID, p)
	if err != nil {
		return Read{}, err
	}
	out := Read{Path: domain.PathOf(at.Ancestors, at.Node), Node: at.Node}
	if at.Node.Kind != domain.KindPage {
		return out, nil
	}
	c, err := r.nodes.PageContent(ctx, at.Node.ID)
	if err != nil {
		return Read{}, found(err, &domain.PathNotFound{Path: p.Written})
	}
	slice, err := domain.Cut(c.Content, w, m.Room(out.Path), m.Bytes)
	if err != nil {
		return Read{}, err
	}
	out.Page = &PageRead{Version: domain.VersionToken{Node: at.Node.ID, Revision: c.Revision}, Window: w, Slice: slice,
		Bytes: len(c.Content)}
	return out, nil
}
