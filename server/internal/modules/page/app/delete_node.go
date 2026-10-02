package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// DeleteNode deletes a page or an attachment with everything under it:
// DELETE /api/v0/nodes/{node_id} (M4/P2 design 3.5).
type DeleteNode struct {
	writer *Writer
	nodes  Nodes
	logger *slog.Logger
}

// NewDeleteNode returns the use case.
func NewDeleteNode(writer *Writer, nodes Nodes, logger *slog.Logger) *DeleteNode {
	return &DeleteNode{writer: writer, nodes: nodes, logger: logger}
}

// Execute deletes the node id and its subtree, from client, in a unit that
// changes the tree. The node is found unlocked first, for its notebook;
// the unit finds it again under the notebook's lock. A node that does not
// exist, is deleted, or whose notebook the caller has no role in is
// page.not_found; a reader gets forbidden.
func (d *DeleteNode) Execute(ctx context.Context, id uuid.UUID, client domain.Client) error {
	n, err := d.nodes.FindNode(ctx, id)
	if err != nil {
		return found(err, domain.ErrNotFound)
	}
	var deleted int
	spec := UnitSpec{NotebookID: n.NotebookID, Action: domain.ActionDelete, Tree: true, Client: client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotFound}
	outcome, err := d.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		sub, err := u.Delete(ctx, id)
		deleted = len(sub)
		return err
	})
	if err != nil {
		return err
	}
	d.logger.InfoContext(ctx, "node deleted", append(logged(outcome, n, client), slog.Int("nodes", deleted))...)
	return nil
}
