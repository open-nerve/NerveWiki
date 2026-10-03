package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// ReleaseEditLock is a notebook admin's forced unlock of a page: DELETE
// /api/v0/pages/{page_id}/edit-lock (M5 design 4.2).
type ReleaseEditLock struct {
	writer *Writer
	nodes  Nodes
	logger *slog.Logger
}

// NewReleaseEditLock returns the use case.
func NewReleaseEditLock(writer *Writer, nodes Nodes, logger *slog.Logger) *ReleaseEditLock {
	return &ReleaseEditLock{writer: writer, nodes: nodes, logger: logger}
}

// Execute ends whichever sessions hold the lock of the page id, from
// client, in a unit that keeps the tree. The page is found unlocked first,
// for its notebook; the unit locks its gate. A page that does not exist,
// is deleted, or whose notebook the caller has no role in is
// page.not_found; anyone but the notebook's admins gets forbidden. A page
// no session holds is released already: no error.
func (r *ReleaseEditLock) Execute(ctx context.Context, id uuid.UUID, client domain.Client) error {
	n, err := r.nodes.FindNode(ctx, id)
	switch {
	case err != nil:
		return found(err, domain.ErrNotFound)
	case n.Kind != domain.KindPage:
		return domain.ErrNotFound
	}
	var ended int
	spec := UnitSpec{NotebookID: n.NotebookID, Action: domain.ActionReleaseEditLock, Client: client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotFound}
	outcome, err := r.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		ended, err = u.Unlock(ctx, id)
		return err
	})
	if err != nil {
		return err
	}
	r.logger.InfoContext(ctx, "edit lock released",
		slog.String("workspace_id", outcome.WorkspaceID.String()), slog.String("notebook_id", n.NotebookID.String()),
		slog.String("node_id", n.ID.String()), slog.String("user_id", outcome.By.String()), slog.String("client", string(client)),
		slog.Int("sessions", ended))
	return nil
}
