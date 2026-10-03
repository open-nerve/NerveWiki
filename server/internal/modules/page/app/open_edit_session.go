package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// OpenEditSession opens an edit session of a page: POST
// /api/v0/pages/{page_id}/edit-sessions (M4/P4 design 3.5).
type OpenEditSession struct {
	writer *Writer
	nodes  Nodes
	logger *slog.Logger
}

// NewOpenEditSession returns the use case.
func NewOpenEditSession(writer *Writer, nodes Nodes, logger *slog.Logger) *OpenEditSession {
	return &OpenEditSession{writer: writer, nodes: nodes, logger: logger}
}

// Execute opens the caller's edit session of the page id, from client, in
// a unit that keeps the tree; with takeOver, it ends the caller's own
// alive sessions of the page first (M5 design 4.2). The page is found
// unlocked first, for its notebook; the unit locks its gate. A page that
// does not exist, is deleted, or whose notebook the caller has no role in
// is page.not_found; a reader gets forbidden; then the vetoers.
func (o *OpenEditSession) Execute(ctx context.Context, id uuid.UUID, client domain.Client, takeOver bool) (EditSession, error) {
	n, err := o.nodes.FindNode(ctx, id)
	switch {
	case err != nil:
		return EditSession{}, found(err, domain.ErrNotFound)
	case n.Kind != domain.KindPage:
		return EditSession{}, domain.ErrNotFound
	}
	var s EditSession
	spec := UnitSpec{NotebookID: n.NotebookID, Action: domain.ActionEdit, Client: client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotFound}
	outcome, err := o.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		s, err = u.OpenSession(ctx, id, takeOver)
		return err
	})
	if err != nil {
		return EditSession{}, err
	}
	o.logger.InfoContext(ctx, "edit session opened", append(sessionLogged(outcome.WorkspaceID, s), slog.Bool("take_over", takeOver))...)
	return s, nil
}

// sessionLogged are an edit session's log attributes: its ids, where and
// whose, and its client.
func sessionLogged(workspaceID uuid.UUID, s EditSession) []any {
	attrs := []any{slog.String("edit_session_id", s.ID.String())}
	if workspaceID != (uuid.UUID{}) {
		attrs = append(attrs, slog.String("workspace_id", workspaceID.String()))
	}
	return append(attrs, slog.String("notebook_id", s.NotebookID.String()), slog.String("node_id", s.NodeID.String()),
		slog.String("user_id", s.UserID.String()), slog.String("client", string(s.Client)))
}
