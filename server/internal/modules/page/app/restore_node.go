package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// NodeRestored is what a restore did: the item's root as it left it,
// whether it renamed it, where it put it and why, and its changeset.
type NodeRestored struct {
	Node        domain.Node
	Renamed     bool
	Placement   domain.Placement
	ChangesetID uuid.UUID
}

// RestoreNode brings a trash item back: POST
// /api/v0/nodes/{node_id}/restore (M8/P2 design 3.5).
type RestoreNode struct {
	reads  TrashReads
	writer *Writer
	parser *ContentParser
	// maxNodes bounds the nodes of an item one unit restores: domain's
	// MaxUnitNodes, but where a measurement raises it.
	maxNodes int
	logger   *slog.Logger
}

// NewRestoreNode returns the use case; maxNodes 0 is domain.MaxUnitNodes.
func NewRestoreNode(reads TrashReads, writer *Writer, parser *ContentParser, maxNodes int, logger *slog.Logger) *RestoreNode {
	if maxNodes == 0 {
		maxNodes = domain.MaxUnitNodes
	}
	return &RestoreNode{reads: reads, writer: writer, parser: parser, maxNodes: maxNodes, logger: logger}
}

// Execute brings the trash item of the root id back, from client, in a
// unit of its own changeset, a trash_restore. The codes come in M8 design
// 4.14's order, all but the last unlocked: a node that does not exist, or
// whose notebook is deleted or the caller has no role in, is
// page.not_found; a reader forbidden; then as GetTrashItem answers,
// page.not_in_trash, page.not_trash_root and page.restore_expired; then
// page.restore_too_large, by the item's counts, past the unit's nodes, and
// by its group, past its pages or their content's bytes (a lone page
// never is); page.trash_changed for a page whose content the purge took,
// not parsed; then the contents parsed, largest first, server_busy when their waits
// run out, and page.restore_too_large past the unit's links, known only
// once they are; then the unit, which reads the item again under the lock
// (Unit.Restore): page.restore_expired at its time, page.trash_changed
// for an item changed meanwhile, and from the observers, an attachment's
// file the purge took.
func (r *RestoreNode) Execute(ctx context.Context, id uuid.UUID, client domain.Client) (NodeRestored, error) {
	n, err := r.reads.Nodes.FindAnyNode(ctx, id)
	if err != nil {
		return NodeRestored{}, found(err, domain.ErrNotFound)
	}
	spec := UnitSpec{NotebookID: n.NotebookID, Action: domain.ActionRestore, Tree: true, Client: client, NotFound: domain.ErrNotFound,
		Kind: domain.ChangesetTrashRestore}
	if err := r.writer.Allowed(ctx, spec); err != nil {
		return NodeRestored{}, err
	}
	root, err := r.reads.trashRoot(ctx, n, r.reads.Clock.Now())
	if err != nil {
		return NodeRestored{}, err
	}
	if root.Pages+root.Assets > r.maxNodes {
		return NodeRestored{}, domain.ErrRestoreTooLarge
	}
	group, err := r.reads.Trash.TrashGroup(ctx, root.Node.NotebookID, root.Node.ID, root.DeletedAt)
	if err != nil {
		return NodeRestored{}, found(err, domain.ErrTrashChanged)
	}
	var pages []uuid.UUID
	bytes, taken := 0, false
	for _, m := range group {
		if m.Node.Kind == domain.KindPage {
			pages, bytes, taken = append(pages, m.Node.ID), bytes+m.Bytes, taken || m.Revision == 0
		}
	}
	switch {
	case domain.UnitTooLarge(len(pages), bytes, 0):
		return NodeRestored{}, domain.ErrRestoreTooLarge
	case taken:
		return NodeRestored{}, domain.ErrTrashChanged
	}
	contents, err := r.reads.Trash.TrashContents(ctx, pages, root.DeletedAt)
	switch {
	case err != nil:
		return NodeRestored{}, err
	case len(contents) != len(pages):
		return NodeRestored{}, domain.ErrTrashChanged
	}
	facts, release, err := r.parser.DecidedAll(ctx, contents, domain.ErrRestoreTooLarge)
	if err != nil {
		return NodeRestored{}, err
	}
	defer release()
	var restored RestoredItem
	outcome, err := r.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		var err error
		restored, err = u.Restore(ctx, TrashRestore{Root: root, Group: group, Facts: facts, Retention: r.reads.Retention})
		return err
	})
	if err != nil {
		return NodeRestored{}, err
	}
	r.logger.InfoContext(ctx, "node restored", append(logged(outcome, restored.Node, client), slog.Int("nodes", len(group)),
		slog.Int("pages", len(pages)), slog.Int("assets", len(group)-len(pages)), slog.String("placed", string(restored.Placement.Reason)),
		slog.Bool("renamed", restored.Renamed), slog.Int("others_renamed", restored.Others))...)
	return NodeRestored{Node: restored.Node, Renamed: restored.Renamed, Placement: restored.Placement, ChangesetID: outcome.ChangesetID}, nil
}
