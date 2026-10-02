package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// MoveNode moves a page or an attachment, under another parent or among
// its siblings: POST /api/v0/nodes/{node_id}/move (M4/P2 design 3.5).
type MoveNode struct {
	writer *Writer
	nodes  Nodes
	logger *slog.Logger
}

// NewMoveNode returns the use case.
func NewMoveNode(writer *Writer, nodes Nodes, logger *slog.Logger) *MoveNode {
	return &MoveNode{writer: writer, nodes: nodes, logger: logger}
}

// Destination is where a move puts a node: under ParentID (nil: the
// notebook's root), at Position among its siblings there.
type Destination struct {
	ParentID *uuid.UUID
	Position Position
}

// Execute moves the node id to to, from client, in a unit that changes the
// tree; it answers the node as the unit leaves it. The node is found
// unlocked first, for its notebook; the unit finds it again under the
// notebook's lock. A node that does not exist, is deleted, or whose
// notebook the caller has no role in is page.not_found; a reader gets
// forbidden; the destination is checked after both. A move to where the
// node is writes and logs nothing.
func (m *MoveNode) Execute(ctx context.Context, id uuid.UUID, to Destination, client domain.Client) (domain.Node, error) {
	n, err := m.nodes.FindNode(ctx, id)
	if err != nil {
		return domain.Node{}, found(err, domain.ErrNotFound)
	}
	var out domain.Node
	spec := UnitSpec{NotebookID: n.NotebookID, Action: domain.ActionMove, Tree: true, Client: client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotFound}
	outcome, err := m.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		if _, err := u.Move(ctx, id, to.ParentID, to.Position); err != nil {
			return err
		}
		// A participant may have changed the node since.
		out, err = m.nodes.FindNodeIn(ctx, n.NotebookID, id)
		return found(err, domain.ErrNotFound)
	})
	if err != nil {
		return domain.Node{}, err
	}
	if outcome.ChangesetID != (uuid.UUID{}) {
		m.logger.InfoContext(ctx, "node moved", logged(outcome, out, client)...)
	}
	return out, nil
}
