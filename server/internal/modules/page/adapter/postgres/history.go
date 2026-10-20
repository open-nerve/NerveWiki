package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// FindChangeset implements app.History.
func (s *Store) FindChangeset(ctx context.Context, id uuid.UUID) (domain.Changeset, error) {
	r, err := s.queries(ctx).FindChangeset(ctx, id)
	if err != nil {
		return domain.Changeset{}, notFound("find changeset", err)
	}
	return changesetOf(gen.ListChangesetsRow(r)), nil
}

// ListChangesets implements app.History: the log's statement, one
// actor's, or one node's, by the filter, each read through its own index
// whatever plan is kept for it.
func (s *Store) ListChangesets(ctx context.Context, notebookID uuid.UUID, f domain.LogFilter, below domain.ChangesetCursor, limit int) (
	[]domain.Changeset, error,
) {
	q := s.queries(ctx)
	var client *string
	if f.Client != "" {
		client = new(string(f.Client))
	}
	var rows []gen.ListChangesetsRow
	var err error
	switch {
	case f.PageID != nil:
		var page []gen.ListChangesetsOfPageRow
		page, err = q.ListChangesetsOfPage(ctx, gen.ListChangesetsOfPageParams{NotebookID: notebookID, BelowAt: below.At,
			BelowID: below.ID, Actor: f.Actor, Client: client, AllMcp: f.AllMCP, MaxRows: int32(limit), PageID: *f.PageID})
		for _, r := range page {
			rows = append(rows, gen.ListChangesetsRow(r))
		}
	case f.Actor != nil:
		var by []gen.ListChangesetsByRow
		by, err = q.ListChangesetsBy(ctx, gen.ListChangesetsByParams{NotebookID: notebookID, Actor: *f.Actor, BelowAt: below.At,
			BelowID: below.ID, Client: client, AllMcp: f.AllMCP, MaxRows: int32(limit)})
		for _, r := range by {
			rows = append(rows, gen.ListChangesetsRow(r))
		}
	default:
		rows, err = q.ListChangesets(ctx, gen.ListChangesetsParams{NotebookID: notebookID, BelowAt: below.At, BelowID: below.ID,
			Client: client, AllMcp: f.AllMCP, MaxRows: int32(limit)})
	}
	if err != nil {
		return nil, fmt.Errorf("list changesets: %w", err)
	}
	out := make([]domain.Changeset, len(rows))
	for i, r := range rows {
		out[i] = changesetOf(r)
	}
	return out, nil
}

// changesetOf is a row of the queries that read a changeset for the log.
func changesetOf(r gen.ListChangesetsRow) domain.Changeset {
	c := domain.Changeset{
		ID: r.ID, NotebookID: r.NotebookID, Kind: domain.ChangesetKind(r.Kind), Client: domain.Client(r.Client), Message: r.Message,
		By: r.CreatedByID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Trimmed: r.Trimmed,
	}
	if r.RevertsID != nil {
		c.RevertsID = *r.RevertsID
		if r.RevertsKind != nil && r.RevertsBy != nil && r.RevertsAt != nil {
			c.Reverts = &domain.ChangesetBrief{ID: *r.RevertsID, Kind: domain.ChangesetKind(*r.RevertsKind), By: *r.RevertsBy,
				CreatedAt: *r.RevertsAt}
		}
	}
	return c
}

// ChangesetFacts implements app.History.
func (s *Store) ChangesetFacts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.ChangesetFacts, error) {
	rows, err := s.queries(ctx).ChangesetFacts(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("changeset facts: %w", err)
	}
	out := make(map[uuid.UUID]domain.ChangesetFacts, len(rows))
	for _, r := range rows {
		out[r.ID] = domain.ChangesetFacts{Nodes: int(r.NodeCount), Contents: int(r.ContentCount), RevertedBy: r.RevertedBy}
	}
	return out, nil
}

// ChangesetMoves implements app.History: a changeset gone since it was
// read, which the purge took, neither.
func (s *Store) ChangesetMoves(ctx context.Context, id uuid.UUID) (domain.ChangesetMoves, error) {
	r, err := s.queries(ctx).ChangesetMoves(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.ChangesetMoves{}, nil
	case err != nil:
		return domain.ChangesetMoves{}, fmt.Errorf("changeset moves: %w", err)
	}
	return domain.ChangesetMoves{Renames: r.Renames, Moves: r.Moves}, nil
}

// ListChanges implements app.History.
func (s *Store) ListChanges(ctx context.Context, id, after uuid.UUID, limit int) ([]domain.ChangeRow, error) {
	rows, err := s.queries(ctx).ListChanges(ctx, gen.ListChangesParams{ID: id, After: after, MaxRows: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("list changes: %w", err)
	}
	out := make([]domain.ChangeRow, len(rows))
	for i, r := range rows {
		out[i] = changeOf(r)
	}
	return out, nil
}

// FirstChanges implements app.History.
func (s *Store) FirstChanges(ctx context.Context, ids []uuid.UUID, n int) (map[uuid.UUID][]domain.ChangeRow, error) {
	rows, err := s.queries(ctx).FirstChanges(ctx, gen.FirstChangesParams{Ids: ids, N: int32(n)})
	if err != nil {
		return nil, fmt.Errorf("first changes: %w", err)
	}
	out := make(map[uuid.UUID][]domain.ChangeRow, len(ids))
	for _, r := range rows {
		out[r.ChangesetID] = append(out[r.ChangesetID], changeOf(gen.ListChangesRow{
			NodeID: r.NodeID, Kind: r.Kind, Name: r.Name, DeletedAt: r.DeletedAt, HasItem: r.HasItem, BeforeParentID: r.BeforeParentID,
			BeforeParentName: r.BeforeParentName, BeforeName: r.BeforeName, BeforeSortOrder: r.BeforeSortOrder,
			AfterParentID: r.AfterParentID, AfterParentName: r.AfterParentName, AfterName: r.AfterName,
			AfterSortOrder: r.AfterSortOrder, Restored: r.Restored, BaseRevision: r.BaseRevision, Revision: r.Revision,
		}))
	}
	return out, nil
}

// changeOf is a row of the queries that read a changeset's changes: an
// item's state where its name is, a version where its revision is.
func changeOf(r gen.ListChangesRow) domain.ChangeRow {
	c := domain.ChangeRow{NodeID: r.NodeID, NodeKind: domain.Kind(r.Kind), Name: r.Name, Alive: r.DeletedAt == nil, Restored: r.Restored}
	place := func(parent *uuid.UUID, parentName, name *string, order *float64) *domain.ChangePlace {
		if !r.HasItem || name == nil || order == nil {
			return nil
		}
		return &domain.ChangePlace{ParentID: parent, ParentName: parentName, Name: *name, SortOrder: *order}
	}
	c.Before = place(r.BeforeParentID, r.BeforeParentName, r.BeforeName, r.BeforeSortOrder)
	c.After = place(r.AfterParentID, r.AfterParentName, r.AfterName, r.AfterSortOrder)
	if r.Revision != nil {
		c.Content = &domain.ChangeVersion{Base: intOf(r.BaseRevision), Revision: int(*r.Revision)}
	}
	return c
}

// intOf is v as an int, nil for none.
func intOf(v *int32) *int {
	if v == nil {
		return nil
	}
	return new(int(*v))
}

// CountItems implements app.History.
func (s *Store) CountItems(ctx context.Context, id uuid.UUID, limit int) (int, error) {
	n, err := s.queries(ctx).CountItems(ctx, gen.CountItemsParams{ID: id, Lim: int32(limit)})
	if err != nil {
		return 0, fmt.Errorf("count changeset items: %w", err)
	}
	return int(n), nil
}

// ChangesetItems implements app.History.
func (s *Store) ChangesetItems(ctx context.Context, id uuid.UUID) ([]domain.ChangesetItem, error) {
	rows, err := s.queries(ctx).ChangesetItems(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("changeset items: %w", err)
	}
	out := make([]domain.ChangesetItem, len(rows))
	for i, r := range rows {
		out[i] = itemOf(gen.ChangesetItemsOfRow(r))
	}
	return out, nil
}

// ChangesetItemsOf implements app.History, planned with its arguments.
func (s *Store) ChangesetItemsOf(ctx context.Context, id uuid.UUID, ids []uuid.UUID) ([]domain.ChangesetItem, error) {
	rows, err := s.planned(ctx).ChangesetItemsOf(ctx, gen.ChangesetItemsOfParams{ID: id, NodeIds: ids})
	if err != nil {
		return nil, fmt.Errorf("changeset items of nodes: %w", err)
	}
	out := make([]domain.ChangesetItem, len(rows))
	for i, r := range rows {
		out[i] = itemOf(r)
	}
	return out, nil
}

// itemOf is a row of the statements that read a changeset's items.
func itemOf(r gen.ChangesetItemsOfRow) domain.ChangesetItem {
	state := func(parent *uuid.UUID, name *string, order *float64) *domain.TreeState {
		if name == nil || order == nil {
			return nil
		}
		return &domain.TreeState{ParentID: parent, Name: *name, SortOrder: *order}
	}
	out := domain.ChangesetItem{NodeID: r.NodeID, Before: state(r.BeforeParentID, r.BeforeName, r.BeforeSortOrder),
		After: state(r.AfterParentID, r.AfterName, r.AfterSortOrder), Restored: r.Restored}
	if r.Revision != nil {
		out.Revision = int(*r.Revision)
	}
	return out
}

// ChangesetVersions implements app.History. A version whose base the purge
// took has none of its size: zero.
func (s *Store) ChangesetVersions(ctx context.Context, id uuid.UUID) ([]domain.ChangesetVersion, error) {
	rows, err := s.queries(ctx).ChangesetVersions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("changeset versions: %w", err)
	}
	out := make([]domain.ChangesetVersion, len(rows))
	for i, r := range rows {
		out[i] = changedVersionOf(gen.ChangesetVersionsOfRow(r))
	}
	return out, nil
}

// ChangesetVersionsOf implements app.History, planned with its arguments.
func (s *Store) ChangesetVersionsOf(ctx context.Context, id uuid.UUID, ids []uuid.UUID) ([]domain.ChangesetVersion, error) {
	rows, err := s.planned(ctx).ChangesetVersionsOf(ctx, gen.ChangesetVersionsOfParams{ID: id, NodeIds: ids})
	if err != nil {
		return nil, fmt.Errorf("changeset versions of pages: %w", err)
	}
	out := make([]domain.ChangesetVersion, len(rows))
	for i, r := range rows {
		out[i] = changedVersionOf(r)
	}
	return out, nil
}

// changedVersionOf is a row of the statements that read a changeset's versions.
func changedVersionOf(r gen.ChangesetVersionsOfRow) domain.ChangesetVersion {
	out := domain.ChangesetVersion{NodeID: r.NodeID, Base: intOf(r.BaseRevision), Revision: int(r.Revision), Hash: r.ContentHash}
	if r.BaseBytes != nil {
		out.BaseBytes = int(*r.BaseBytes)
	}
	return out
}

// NodesNow implements app.History, a cached statement: with the ids of a
// background revert's changeset its plan for any reads as fast as one with
// them, 252 against 270 ms at 50,000 ids (BenchmarkBackgroundSteps/arrays;
// M8/P4 design 3.11), unlike the trash's arrays' statements, which run
// planned.
func (s *Store) NodesNow(ctx context.Context, ids []uuid.UUID, reverted uuid.UUID) (map[uuid.UUID]domain.NodeNow, error) {
	rows, err := s.queries(ctx).NodesNow(ctx, gen.NodesNowParams{Ids: ids, Reverted: reverted})
	if err != nil {
		return nil, fmt.Errorf("nodes now: %w", err)
	}
	out := make(map[uuid.UUID]domain.NodeNow, len(rows))
	for _, r := range rows {
		n := domain.NodeNow{Node: nodeOf(gen.FindNodeRow{
			ID: r.ID, NotebookID: r.NotebookID, ParentID: r.ParentID, Kind: r.Kind, Name: r.Name, NameKey: r.NameKey,
			SortOrder: r.SortOrder, CreatedByID: r.CreatedByID, UpdatedByID: r.UpdatedByID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}), DeletedAt: r.DeletedAt, TrashRoot: r.TrashRoot, DeletedByRevert: r.DeletedByRevert}
		if r.TrashPages != nil && r.TrashAssets != nil {
			n.TrashPages, n.TrashAssets = int(*r.TrashPages), int(*r.TrashAssets)
		}
		out[r.ID] = n
	}
	return out, nil
}

// ContentsNow implements app.History.
func (s *Store) ContentsNow(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.ContentNow, error) {
	rows, err := s.queries(ctx).ContentsNow(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("contents now: %w", err)
	}
	out := make(map[uuid.UUID]domain.ContentNow, len(rows))
	for _, r := range rows {
		out[r.NodeID] = domain.ContentNow{Revision: int(r.Revision), Hash: r.ContentHash}
	}
	return out, nil
}

// refArrays are refs as the version statements take them.
func refArrays(refs []domain.VersionRef) ([]uuid.UUID, []int32) {
	ids, revisions := make([]uuid.UUID, len(refs)), make([]int32, len(refs))
	for i, r := range refs {
		ids[i], revisions[i] = r.NodeID, int32(r.Revision)
	}
	return ids, revisions
}

// VersionHashes implements app.History.
func (s *Store) VersionHashes(ctx context.Context, refs []domain.VersionRef) (map[domain.VersionRef][]byte, error) {
	ids, revisions := refArrays(refs)
	rows, err := s.queries(ctx).VersionHashes(ctx, gen.VersionHashesParams{NodeIds: ids, Revisions: revisions})
	if err != nil {
		return nil, fmt.Errorf("version hashes: %w", err)
	}
	out := make(map[domain.VersionRef][]byte, len(rows))
	for _, r := range rows {
		out[domain.VersionRef{NodeID: r.NodeID, Revision: int(r.Revision)}] = r.ContentHash
	}
	return out, nil
}

// VersionContents implements app.History.
func (s *Store) VersionContents(ctx context.Context, refs []domain.VersionRef) (map[domain.VersionRef]app.PageContent, error) {
	ids, revisions := refArrays(refs)
	rows, err := s.queries(ctx).VersionContents(ctx, gen.VersionContentsParams{NodeIds: ids, Revisions: revisions})
	if err != nil {
		return nil, fmt.Errorf("version contents: %w", err)
	}
	out := make(map[domain.VersionRef]app.PageContent, len(rows))
	for _, r := range rows {
		out[domain.VersionRef{NodeID: r.NodeID, Revision: int(r.Revision)}] = app.PageContent{Content: r.Content,
			Revision: int(r.Revision), Hash: r.ContentHash}
	}
	return out, nil
}
