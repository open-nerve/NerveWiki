package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The trash's reads (M8 design 4.5; M8/P2 design 3.4, 3.5): the list and
// an item, which the restore's codes share.

// TrashReads are what the trash's reads need: Retention is the purge's,
// jobs.purge_retention, which bounds when an item can be restored.
type TrashReads struct {
	Notebooks   Notebooks
	Nodes       Nodes
	Trash       Trash
	Names       Names
	Attachments AttachmentInfos
	Auth        shared.Authorizer
	Clock       Clock
	Retention   time.Duration
}

// TrashItem is a trash item as the trash lists it: its root, with its
// counts and a page's revision; where it was, its root's ancestors' names;
// who deleted it, by name; until when it can be restored; where a restore
// puts it; and an attachment's type and size, nil for a page, or an
// attachment whose file the purge took.
type TrashItem struct {
	Root            domain.TrashRoot
	Ancestors       []domain.Ancestor
	DeletedByName   string
	RestorableUntil time.Time
	Placement       domain.Placement
	Asset           *AttachmentInfo
}

// trashItems are the items of roots: their lines, their deleters' names
// and their attachments' infos, each read once.
func (r TrashReads) trashItems(ctx context.Context, roots []domain.TrashRoot) ([]TrashItem, error) {
	if len(roots) == 0 {
		return nil, nil
	}
	ids, deleters, assets := make([]uuid.UUID, len(roots)), []uuid.UUID{}, []uuid.UUID{}
	seen := map[uuid.UUID]bool{}
	for i, root := range roots {
		ids[i] = root.Node.ID
		if by := root.Node.UpdatedBy; !seen[by] {
			seen[by] = true
			deleters = append(deleters, by)
		}
		if root.Node.Kind == domain.KindAsset {
			assets = append(assets, root.Node.ID)
		}
	}
	lines, err := r.Trash.TrashLines(ctx, ids)
	if err != nil {
		return nil, err
	}
	names, err := r.Names.DisplayNames(ctx, deleters)
	if err != nil {
		return nil, err
	}
	infos := map[uuid.UUID]AttachmentInfo{}
	if len(assets) > 0 {
		if infos, err = r.Attachments.Of(ctx, assets); err != nil {
			return nil, err
		}
	}
	out := make([]TrashItem, len(roots))
	for i, root := range roots {
		line := lines[root.Node.ID]
		ancestors := make([]domain.Ancestor, len(line))
		for j, a := range line {
			ancestors[j] = domain.Ancestor{ID: a.ID, Name: a.Name}
		}
		out[i] = TrashItem{
			Root: root, Ancestors: ancestors, DeletedByName: names[root.Node.UpdatedBy],
			RestorableUntil: domain.RestorableUntil(root.DeletedAt, r.Retention), Placement: domain.PlacementOf(line, root.Height),
		}
		if info, ok := infos[root.Node.ID]; ok {
			out[i].Asset = &info
		}
	}
	return out, nil
}

// trashRoot is the trash item n is, n a node in the trash, as of now: 409
// page.not_in_trash for a node alive; page.not_trash_root, with its root,
// for one in an item of another root; page.restore_expired for one past
// the time it could be restored in (M8 design 4.14, the order of getTrashItem
// and restoreNode). A node restored since the caller read it is not in the
// trash.
func (r TrashReads) trashRoot(ctx context.Context, n AnyNode, now time.Time) (domain.TrashRoot, error) {
	switch {
	case n.DeletedAt == nil:
		return domain.TrashRoot{}, domain.ErrNotInTrash
	case !n.TrashRoot:
		root, err := r.Trash.TrashRootOf(ctx, n.ID)
		if err != nil {
			return domain.TrashRoot{}, found(err, domain.ErrNotInTrash)
		}
		return domain.TrashRoot{}, domain.NotTrashRoot(root)
	case !domain.Restorable(*n.DeletedAt, now, r.Retention):
		return domain.TrashRoot{}, domain.ErrRestoreExpired
	}
	root, err := r.Trash.FindTrashRoot(ctx, n.ID)
	if err != nil {
		return domain.TrashRoot{}, found(err, domain.ErrNotInTrash)
	}
	return root, nil
}
