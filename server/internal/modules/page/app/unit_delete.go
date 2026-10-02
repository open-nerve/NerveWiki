package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Deleting a subtree in a unit (M4/P2 design 3.4).

// Delete deletes the node id and its descendants, at the unit's time: 404
// for a node that is not in the notebook. The siblings keep their order.
// The step tells every node of the subtree, with no after; each change is
// an item, which goes to the trash with its node. The subtree's edit
// sessions are deleted after it, and the subscribers follow the end of the
// alive ones. It returns the subtree as it was.
func (u *Unit) Delete(ctx context.Context, id uuid.UUID) (domain.Subtree, error) {
	sub, err := u.w.d.Nodes.Subtree(ctx, u.write.NotebookID, id)
	if err != nil {
		return nil, found(err, domain.ErrNotFound)
	}
	ids := make([]uuid.UUID, len(sub))
	changes := make([]domain.Change, len(sub))
	for i, s := range sub {
		state := s.Node.State()
		ids[i], changes[i] = s.Node.ID, domain.Change{NodeID: s.Node.ID, Before: &state}
	}
	err = u.apply(ctx, u.step(domain.OpDelete, changes...), true, func(ctx context.Context) error {
		if err := u.w.d.NodeWriter.DeleteNodes(ctx, ids, u.write.By, u.write.At); err != nil {
			return err
		}
		sessions, err := u.w.d.SessionWriter.DeleteNodeSessions(ctx, ids)
		if err != nil {
			return err
		}
		return tellEnded(ctx, u.w.d.SessionSubscribers, u.write.WorkspaceID, sessions, domain.EndedWithPage, u.write.By, u.write.At)
	})
	return sub, err
}
