package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Deleting a subtree in a unit (M4/P2 design 3.4).

// Delete deletes the node id and its descendants, at the unit's time: 404
// for a node that is not in the notebook. It is Subtree, then
// DeleteSubtree, and returns the subtree as it was.
func (u *Unit) Delete(ctx context.Context, id uuid.UUID) (domain.Subtree, error) {
	sub, err := u.Subtree(ctx, id)
	if err != nil {
		return nil, err
	}
	return sub, u.DeleteSubtree(ctx, sub)
}

// Subtree is the node id and every node under it, in the unit's notebook
// and transaction, the root first and then level by level: page.not_found
// for a node that is not in it.
func (u *Unit) Subtree(ctx context.Context, id uuid.UUID) (domain.Subtree, error) {
	sub, err := u.w.d.Nodes.Subtree(ctx, u.write.NotebookID, id)
	if err != nil {
		return nil, found(err, domain.ErrNotFound)
	}
	return sub, nil
}

// DeleteSubtree deletes sub, a subtree Subtree read in this unit, into the
// trash (M8/P2 design 3.3): its root becomes a trash item, which holds its
// counts and height; the siblings keep their order. The step tells every
// node of the subtree, with no after; each change is an item, which goes
// to the trash with its node. The subtree's edit sessions' rows are
// deleted after it, and the subscribers follow the end of the alive ones.
// The unit's lock of the notebook's row keeps sub the tree's; one read
// elsewhere is its caller's defect.
func (u *Unit) DeleteSubtree(ctx context.Context, sub domain.Subtree) error {
	ids := make([]uuid.UUID, len(sub))
	changes := make([]domain.Change, len(sub))
	kinds := make(map[uuid.UUID]domain.Kind, len(sub))
	for i, s := range sub {
		state := s.Node.State()
		kinds[s.Node.ID] = s.Node.Kind
		ids[i], changes[i] = s.Node.ID, domain.Change{NodeID: s.Node.ID, Before: &state}
	}
	step := u.step(domain.OpDelete, changes...)
	step.Kinds = kinds
	return u.apply(ctx, step, true, func(ctx context.Context) error {
		pages, assets := sub.Counts()
		if err := u.w.d.NodeWriter.DeleteNodes(ctx, Deletion{
			Root: sub[0].Node.ID, IDs: ids, Pages: pages, Assets: assets, Height: sub.Height(), By: u.write.By, At: u.write.At,
		}); err != nil {
			return err
		}
		sessions, err := u.w.d.SessionWriter.DeleteNodeSessions(ctx, ids)
		if err != nil {
			return err
		}
		return tellEnded(ctx, u.w.d.SessionSubscribers, u.write.WorkspaceID, sessions, domain.EndedWithPage, u.write.By, u.write.At)
	})
}
