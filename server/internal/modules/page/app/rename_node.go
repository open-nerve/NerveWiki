package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// RenameNode renames a page or an attachment: PATCH /api/v0/nodes/{node_id}
// (M4/P1 design 3.7).
type RenameNode struct {
	writer *Writer
	nodes  Nodes
	logger *slog.Logger
}

// NewRenameNode returns the use case.
func NewRenameNode(writer *Writer, nodes Nodes, logger *slog.Logger) *RenameNode {
	return &RenameNode{writer: writer, nodes: nodes, logger: logger}
}

// Execute renames the node id to name, from client, in a unit that changes
// the tree; it answers the node as the unit leaves it. The node is found
// unlocked first, for its notebook; the unit finds it again under the
// notebook's lock. A node that does not exist, is deleted, or whose
// notebook the caller has no role in is page.not_found; a reader gets
// forbidden; the name is checked after both. A name the node has already
// writes and logs nothing.
func (r *RenameNode) Execute(ctx context.Context, id uuid.UUID, name string, client domain.Client) (domain.Node, error) {
	n, err := r.nodes.FindNode(ctx, id)
	if err != nil {
		return domain.Node{}, found(err, domain.ErrNotFound)
	}
	var out domain.Node
	spec := UnitSpec{NotebookID: n.NotebookID, Action: domain.ActionRename, Tree: true, Client: client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotFound}
	outcome, err := r.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		if _, err := u.Rename(ctx, id, name); err != nil {
			return err
		}
		// A participant may have changed the node since.
		out, err = r.nodes.FindNodeIn(ctx, n.NotebookID, id)
		return found(err, domain.ErrNotFound)
	})
	if err != nil {
		return domain.Node{}, err
	}
	if outcome.ChangesetID != (uuid.UUID{}) {
		r.logger.InfoContext(ctx, "node renamed", logged(outcome, out, client)...)
	}
	return out, nil
}
