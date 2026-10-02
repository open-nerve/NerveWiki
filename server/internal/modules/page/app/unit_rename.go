package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Renaming a node in a unit (M4/P1 design 3.6).

// Rename renames the node id: 404 for one that is not in the notebook,
// 422 for a name that breaks the rules, 409 for a name a sibling holds. A
// name the node has already writes nothing; one that differs from it in
// case alone is written.
func (u *Unit) Rename(ctx context.Context, id uuid.UUID, name string) (domain.Node, error) {
	return u.rename(ctx, id, name, true)
}

func (u *Unit) rename(ctx context.Context, id uuid.UUID, name string, participate bool) (domain.Node, error) {
	n, err := u.w.d.Nodes.FindNodeIn(ctx, u.write.NotebookID, id)
	if err != nil {
		return domain.Node{}, found(err, domain.ErrNotFound)
	}
	title, err := domain.CheckTitle("name", name)
	switch {
	case err != nil:
		return domain.Node{}, err
	case title.Name == n.Name:
		return n, nil
	}
	if title.Key != n.NameKey {
		siblings, err := u.w.d.Nodes.Children(ctx, u.write.NotebookID, n.ParentID)
		if err != nil {
			return domain.Node{}, err
		}
		if err := titleFree(siblings, title, n.ID); err != nil {
			return domain.Node{}, err
		}
	}
	renamed := n
	renamed.Name, renamed.NameKey, renamed.UpdatedBy, renamed.UpdatedAt = title.Name, title.Key, u.write.By, u.write.At
	before, after := n.State(), renamed.State()
	step := u.step(domain.OpRename, domain.Change{NodeID: n.ID, Before: &before, After: &after})
	err = u.apply(ctx, step, participate, func(ctx context.Context) error {
		return u.w.d.NodeWriter.RenameNode(ctx, renamed)
	})
	return renamed, err
}
