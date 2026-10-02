package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// PutPageContent writes a page's content: PUT
// /api/v0/pages/{page_id}/content (M4/P4 design 3.6).
type PutPageContent struct {
	writer   *Writer
	nodes    Nodes
	markdown Markdown
	logger   *slog.Logger
}

// NewPutPageContent returns the use case.
func NewPutPageContent(writer *Writer, nodes Nodes, markdown Markdown, logger *slog.Logger) *PutPageContent {
	return &PutPageContent{writer: writer, nodes: nodes, markdown: markdown, logger: logger}
}

// ContentPut is a write of a page's whole content: the content, the
// revision it was read at, and the edit session it is made in (zero: none).
type ContentPut struct {
	Content     string
	Base        int
	EditSession uuid.UUID
}

// Execute writes p as the content of the page id, from client, in a unit
// that keeps the tree; it answers the page as the unit leaves it. The
// content is checked and parsed first: 422 on content. The page is found
// unlocked, for its notebook; the unit finds it again under the locks. A
// page that does not exist, is deleted, or whose notebook the caller has
// no role in is page.not_found; a reader gets forbidden; then the edit
// session's 409 and the base's (Unit.WriteContent). A content the page
// holds already writes and logs nothing.
func (w *PutPageContent) Execute(ctx context.Context, id uuid.UUID, p ContentPut, client domain.Client) (PageView, error) {
	parse, err := parsed(w.markdown, "content", p.Content)
	if err != nil {
		return PageView{}, err
	}
	n, err := w.nodes.FindNode(ctx, id)
	switch {
	case err != nil:
		return PageView{}, found(err, domain.ErrNotFound)
	case n.Kind != domain.KindPage:
		return PageView{}, domain.ErrNotFound
	}
	var out PageView
	spec := UnitSpec{NotebookID: n.NotebookID, Action: domain.ActionWrite, Client: client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotFound}
	outcome, err := w.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		cw := ContentWrite{NodeID: id, Content: p.Content, Parsed: parse, Base: p.Base, EditSession: p.EditSession}
		if _, err := u.WriteContent(ctx, cw); err != nil {
			return err
		}
		out, err = readPage(ctx, w.nodes, id)
		return err
	})
	if err != nil {
		return PageView{}, err
	}
	if outcome.ChangesetID != (uuid.UUID{}) {
		attrs := append(logged(outcome, out.Node, client), slog.Int("revision", out.Content.Revision))
		if p.EditSession != (uuid.UUID{}) {
			attrs = append(attrs, slog.String("edit_session_id", p.EditSession.String()))
		}
		w.logger.InfoContext(ctx, "page content written", attrs...)
	}
	return out, nil
}
