package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Renaming a node in a unit (M4/P1 design 3.6).

// Rename renames the node id: 404 for one that is not in the notebook,
// 422 for a name that breaks the rules, a page's title's or an
// attachment's (domain.CheckAssetRename), 409 for a name a sibling holds.
// A name the node has already writes nothing; one that differs from it in
// case alone is written.
func (u *Unit) Rename(ctx context.Context, id uuid.UUID, name string) (domain.Node, error) {
	return u.rename(ctx, id, name, true)
}

func (u *Unit) rename(ctx context.Context, id uuid.UUID, name string, participate bool) (domain.Node, error) {
	n, err := u.w.d.Nodes.FindNodeIn(ctx, u.write.NotebookID, id)
	if err != nil {
		return domain.Node{}, found(err, domain.ErrNotFound)
	}
	title, err := nameOf(n, name, "name")
	if err != nil {
		return domain.Node{}, err
	}
	return u.renameTo(ctx, n, title, participate, titleFree, u.w.d.NodeWriter.RenameNode)
}

// renameTo renames n to title under its parent, title checked: nothing
// written when n is so named; a key that changes checked by free among
// the siblings; one step of OpRename, its write write's.
func (u *Unit) renameTo(ctx context.Context, n domain.Node, title domain.Title, participate bool,
	free func(siblings []domain.Node, title domain.Title, self uuid.UUID) error, write func(context.Context, domain.Node) error,
) (domain.Node, error) {
	if title.Name == n.Name {
		return n, nil
	}
	if title.Key != n.NameKey {
		siblings, err := u.w.d.Nodes.Children(ctx, u.write.NotebookID, n.ParentID)
		if err != nil {
			return domain.Node{}, err
		}
		if err := free(siblings, title, n.ID); err != nil {
			return domain.Node{}, err
		}
	}
	renamed := n
	renamed.Name, renamed.NameKey, renamed.UpdatedBy, renamed.UpdatedAt = title.Name, title.Key, u.write.By, u.write.At
	before, after := n.State(), renamed.State()
	step := u.step(domain.OpRename, domain.Change{NodeID: n.ID, Before: &before, After: &after})
	err := u.apply(ctx, step, participate, func(ctx context.Context) error { return write(ctx, renamed) })
	return renamed, err
}

// nameOf checks name as n's new name, by n's kind, as field.
func nameOf(n domain.Node, name, field string) (domain.Title, error) {
	if n.Kind == domain.KindAsset {
		return domain.CheckAssetRename(field, n.Name, name)
	}
	return domain.CheckTitle(field, name)
}
