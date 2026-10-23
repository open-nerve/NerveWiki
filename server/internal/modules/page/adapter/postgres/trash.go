package postgresadapter

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// ListTrash implements app.Trash.
func (s *Store) ListTrash(ctx context.Context, notebookID uuid.UUID, since time.Time, below domain.TrashCursor, limit int) (
	[]domain.TrashRoot, error,
) {
	rows, err := s.queries(ctx).ListTrash(ctx, gen.ListTrashParams{
		NotebookID: notebookID, Since: since, BelowAt: below.At, BelowID: below.ID, MaxRows: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list trash: %w", err)
	}
	out := make([]domain.TrashRoot, len(rows))
	for i, r := range rows {
		if out[i], err = trashRootOf(gen.FindTrashRootRow(r)); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// FindTrashRoot implements app.Trash.
func (s *Store) FindTrashRoot(ctx context.Context, id uuid.UUID) (domain.TrashRoot, error) {
	r, err := s.queries(ctx).FindTrashRoot(ctx, id)
	if err != nil {
		return domain.TrashRoot{}, notFound("find trash root", err)
	}
	return trashRootOf(r)
}

// trashRootOf is a row of the queries that read a trash item's root.
func trashRootOf(r gen.FindTrashRootRow) (domain.TrashRoot, error) {
	if r.DeletedAt == nil || r.TrashPages == nil || r.TrashAssets == nil || r.TrashHeight == nil {
		return domain.TrashRoot{}, fmt.Errorf("trash root %s: not deleted, or without its counts", r.ID)
	}
	root := domain.TrashRoot{
		Node: nodeOf(gen.FindNodeRow{
			ID: r.ID, NotebookID: r.NotebookID, ParentID: r.ParentID, Kind: r.Kind, Name: r.Name, NameKey: r.NameKey,
			SortOrder: r.SortOrder, CreatedByID: r.CreatedByID, UpdatedByID: r.UpdatedByID, CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		}),
		DeletedAt: *r.DeletedAt, Pages: int(*r.TrashPages), Assets: int(*r.TrashAssets), Height: int(*r.TrashHeight),
	}
	if r.Revision != nil {
		root.Revision = int(*r.Revision)
	}
	return root, nil
}

// TrashRootOf implements app.Trash.
func (s *Store) TrashRootOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	root, err := s.queries(ctx).TrashRootOf(ctx, id)
	if err != nil {
		return uuid.UUID{}, notFound("trash root of", err)
	}
	return root, nil
}

// TrashLines implements app.Trash. A line that does not reach a root is an
// error: only a defect makes one.
func (s *Store) TrashLines(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.TrashAncestor, error) {
	rows, err := s.queries(ctx).TrashLines(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("trash lines: %w", err)
	}
	out := make(map[uuid.UUID][]domain.TrashAncestor, len(ids))
	for _, r := range rows {
		line := out[r.RootID]
		if len(line) == 0 && r.ParentID != nil {
			return nil, fmt.Errorf("trash line of node %s: the chain does not reach a root", r.RootID)
		}
		out[r.RootID] = append(line, domain.TrashAncestor{ID: r.ID, Name: r.Name, DeletedAt: r.DeletedAt})
	}
	return out, nil
}

// TrashGroup implements app.Trash: the root, then its descendants of its
// time level by level, each level read in one statement by its parents,
// planned with them, as Subtree reads.
func (s *Store) TrashGroup(ctx context.Context, notebookID, id uuid.UUID, at time.Time) ([]domain.TrashMember, error) {
	q, levels := s.queries(ctx), s.planned(ctx)
	root, err := q.TrashGroupRoot(ctx, gen.TrashGroupRootParams{ID: id, NotebookID: notebookID, DeletedAt: at})
	if err != nil {
		return nil, notFound("trash group", err)
	}
	out := []domain.TrashMember{memberOf(gen.TrashChildrenOfAllRow(root), 1)}
	parents := []uuid.UUID{root.ID}
	for level := 2; level <= subtreeLevels && len(parents) > 0; level++ {
		rows, err := levels.TrashChildrenOfAll(ctx, gen.TrashChildrenOfAllParams{NotebookID: notebookID, Parents: parents, DeletedAt: at})
		if err != nil {
			return nil, fmt.Errorf("trash group: %w", err)
		}
		parents = make([]uuid.UUID, len(rows))
		for i, r := range rows {
			out = append(out, memberOf(r, level))
			parents[i] = r.ID
		}
	}
	return out, nil
}

// memberOf is a row of the queries that read a trash item's group, at
// level.
func memberOf(r gen.TrashChildrenOfAllRow, level int) domain.TrashMember {
	m := domain.TrashMember{Node: nodeOf(gen.FindNodeRow{
		ID: r.ID, NotebookID: r.NotebookID, ParentID: r.ParentID, Kind: r.Kind, Name: r.Name, NameKey: r.NameKey,
		SortOrder: r.SortOrder, CreatedByID: r.CreatedByID, UpdatedByID: r.UpdatedByID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}), Level: level}
	if r.Revision != nil && r.ByteSize != nil {
		m.Revision, m.Bytes = int(*r.Revision), int(*r.ByteSize)
	}
	return m
}

// TrashContents implements app.Trash. Its ids are an item's pages, at most
// the unit's, and so are its rows of the time: a plan for any arguments
// that compares each with the ids costs as much as the one with them.
func (s *Store) TrashContents(ctx context.Context, ids []uuid.UUID, at time.Time) (map[uuid.UUID]string, error) {
	rows, err := s.queries(ctx).TrashContents(ctx, gen.TrashContentsParams{Ids: ids, DeletedAt: at})
	if err != nil {
		return nil, fmt.Errorf("trash contents: %w", err)
	}
	out := make(map[uuid.UUID]string, len(rows))
	for _, r := range rows {
		out[r.NodeID] = r.Content
	}
	return out, nil
}

// TrashContentsWithin implements app.Trash, as TrashContents.
func (s *Store) TrashContentsWithin(ctx context.Context, ids []uuid.UUID, at time.Time, max int64) (app.ContentsWithin, error) {
	rows, err := s.queries(ctx).TrashContentsWithin(ctx, gen.TrashContentsWithinParams{Ids: ids, DeletedAt: at, MaxBytes: max})
	if err != nil {
		return app.ContentsWithin{}, fmt.Errorf("trash contents within: %w", err)
	}
	out := app.ContentsWithin{Contents: make(map[uuid.UUID]string, len(rows))}
	for _, r := range rows {
		if !r.Within {
			out.Past, out.PastBytes = r.NodeID, int(r.ByteSize)
			continue
		}
		out.Contents[r.NodeID] = r.Content
	}
	return out, nil
}

// TrashHistory implements app.Trash, planned with ids: from a cached
// statement's sixth run, statistics taken while the rows were alive make
// its plan for any arguments read the rows of the time and compare each
// with the ids one by one, 5 seconds at the unit's bounds where the plan
// with them took 50 ms (M8/P2A fix check 1, I-1).
func (s *Store) TrashHistory(ctx context.Context, ids []uuid.UUID, at time.Time, limit int) (int, error) {
	n, err := s.planned(ctx).TrashHistory(ctx, gen.TrashHistoryParams{Ids: ids, DeletedAt: at, Lim: int32(limit)})
	if err != nil {
		return 0, fmt.Errorf("trash history: %w", err)
	}
	return int(n), nil
}

// timesOf are at's nodes and their times, as the statements by key take
// them.
func timesOf(at map[uuid.UUID]time.Time) ([]uuid.UUID, []time.Time) {
	ids, times := make([]uuid.UUID, 0, len(at)), make([]time.Time, 0, len(at))
	for id, t := range at {
		ids, times = append(ids, id), append(times, t)
	}
	return ids, times
}

// TrashNodesAt implements app.Trash, planned with its arguments, as each
// of the jobs' statements of many keys is (M8/P4 design 3.11).
func (s *Store) TrashNodesAt(ctx context.Context, notebookID uuid.UUID, at map[uuid.UUID]time.Time) (map[uuid.UUID]app.TrashNode, error) {
	ids, times := timesOf(at)
	rows, err := s.planned(ctx).TrashNodesAt(ctx, gen.TrashNodesAtParams{Ids: ids, DeletedAts: times, NotebookID: notebookID})
	if err != nil {
		return nil, fmt.Errorf("trash nodes at: %w", err)
	}
	out := make(map[uuid.UUID]app.TrashNode, len(rows))
	for _, r := range rows {
		n := app.TrashNode{TrashMember: memberOf(levelRow(r), 0), Root: r.TrashRoot}
		if r.TrashRoot {
			if r.TrashPages == nil || r.TrashAssets == nil || r.TrashHeight == nil {
				return nil, fmt.Errorf("trash root %s: without its counts", r.ID)
			}
			n.Pages, n.Assets, n.Height = int(*r.TrashPages), int(*r.TrashAssets), int(*r.TrashHeight)
		}
		out[r.ID] = n
	}
	return out, nil
}

// levelRow is a node TrashNodesAt read as a level of a group reads it.
func levelRow(r gen.TrashNodesAtRow) gen.TrashChildrenOfAllRow {
	return gen.TrashChildrenOfAllRow{
		ID: r.ID, NotebookID: r.NotebookID, ParentID: r.ParentID, Kind: r.Kind, Name: r.Name, NameKey: r.NameKey,
		SortOrder: r.SortOrder, CreatedByID: r.CreatedByID, UpdatedByID: r.UpdatedByID, CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt, Revision: r.Revision, ByteSize: r.ByteSize,
	}
}

// TrashGroups implements app.Trash: the roots in one statement, then each
// level of all the groups in one, by its parents and their times, planned
// with them; each group's levels in the order TrashGroup reads them.
func (s *Store) TrashGroups(ctx context.Context, notebookID uuid.UUID, roots map[uuid.UUID]time.Time) (
	map[uuid.UUID][]domain.TrashMember, error,
) {
	q := s.planned(ctx)
	ids, times := timesOf(roots)
	top, err := q.TrashNodesAt(ctx, gen.TrashNodesAtParams{Ids: ids, DeletedAts: times, NotebookID: notebookID})
	if err != nil {
		return nil, fmt.Errorf("trash groups: %w", err)
	}
	out := make(map[uuid.UUID][]domain.TrashMember, len(top))
	// groupOf is each node read's group, by its root.
	groupOf := make(map[uuid.UUID]uuid.UUID, len(top))
	parents, ats := make([]uuid.UUID, len(top)), make([]time.Time, len(top))
	for i, r := range top {
		out[r.ID] = []domain.TrashMember{memberOf(levelRow(r), 1)}
		groupOf[r.ID], parents[i], ats[i] = r.ID, r.ID, roots[r.ID]
	}
	for level := 2; level <= subtreeLevels && len(parents) > 0; level++ {
		rows, err := q.TrashChildrenAt(ctx, gen.TrashChildrenAtParams{Parents: parents, DeletedAts: ats, NotebookID: notebookID})
		if err != nil {
			return nil, fmt.Errorf("trash groups: %w", err)
		}
		parents, ats = make([]uuid.UUID, len(rows)), make([]time.Time, len(rows))
		for i, r := range rows {
			group := groupOf[*r.ParentID]
			out[group] = append(out[group], memberOf(gen.TrashChildrenOfAllRow(r), level))
			groupOf[r.ID], parents[i], ats[i] = group, r.ID, roots[group]
		}
	}
	return out, nil
}

// TrashShape implements app.Trash: a level a statement, planned with its
// parents, as TrashGroup reads.
func (s *Store) TrashShape(ctx context.Context, notebookID, id uuid.UUID, at time.Time) ([]domain.ShapeNode, error) {
	q := s.planned(ctx)
	var out []domain.ShapeNode
	parents := []uuid.UUID{id}
	for level := 2; level <= subtreeLevels && len(parents) > 0; level++ {
		rows, err := q.TrashShapeOfAll(ctx, gen.TrashShapeOfAllParams{NotebookID: notebookID, Parents: parents, DeletedAt: at})
		if err != nil {
			return nil, fmt.Errorf("trash shape: %w", err)
		}
		parents = make([]uuid.UUID, len(rows))
		for i, r := range rows {
			n := domain.ShapeNode{ID: r.ID, ParentID: *r.ParentID, Asset: domain.Kind(r.Kind) == domain.KindAsset}
			if r.ByteSize != nil {
				n.Bytes = int(*r.ByteSize)
			}
			out, parents[i] = append(out, n), r.ID
		}
	}
	return out, nil
}
