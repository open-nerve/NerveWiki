package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// CreatePage creates a page: POST /api/v0/notebooks/{notebook_id}/pages
// (M4/P1 design 3.7).
type CreatePage struct {
	writer *Writer
	nodes  Nodes
	parser *ContentParser
	logger *slog.Logger
}

// NewCreatePage returns the use case.
func NewCreatePage(writer *Writer, nodes Nodes, parser *ContentParser, logger *slog.Logger) *CreatePage {
	return &CreatePage{writer: writer, nodes: nodes, parser: parser, logger: logger}
}

// Execute creates the page d in the notebook id, from client, in a unit
// that changes the tree; it answers the page as the unit leaves it. Its
// content is checked first: 422 on content. Then a notebook that does not
// exist, is deleted, or that the caller has no role in is
// notebook.not_found; a reader gets forbidden: decided before the content
// is parsed, and again in the unit; the other values are checked after
// both.
func (c *CreatePage) Execute(ctx context.Context, id uuid.UUID, d PageDraft, client domain.Client) (PageView, error) {
	if err := domain.CheckContent("content", d.Content); err != nil {
		return PageView{}, err
	}
	spec := UnitSpec{NotebookID: id, Action: domain.ActionCreate, Tree: true, Client: client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotebookNotFound}
	parse, release, err := c.parser.Parse(ctx, spec, d.Content)
	if err != nil {
		return PageView{}, err
	}
	defer release()
	d.Parsed = parse
	var out PageView
	outcome, err := c.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		n, err := u.CreatePage(ctx, d)
		if err != nil {
			return err
		}
		// A participant may have changed the page since.
		out, err = readPage(ctx, c.nodes, n.ID)
		return err
	})
	if err != nil {
		return PageView{}, err
	}
	c.logger.InfoContext(ctx, "page created", logged(outcome, out.Node, client)...)
	return out, nil
}

// logged are a write's log attributes: where, which node, which changeset,
// who and from where. A title never is: it may say what the page holds.
func logged(o Outcome, n domain.Node, client domain.Client) []any {
	return []any{
		slog.String("workspace_id", o.WorkspaceID.String()), slog.String("notebook_id", n.NotebookID.String()),
		slog.String("node_id", n.ID.String()), slog.String("changeset_id", o.ChangesetID.String()),
		slog.String("user_id", o.By.String()), slog.String("client", string(client)),
	}
}
