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

// CreateChangeset implements app.ChangesetWriter.
func (s *Store) CreateChangeset(ctx context.Context, c app.Changeset) error {
	var reverts *uuid.UUID
	if c.RevertsID != (uuid.UUID{}) {
		reverts = &c.RevertsID
	}
	if err := s.queries(ctx).CreateChangeset(ctx, gen.CreateChangesetParams{
		ID: c.ID, NotebookID: c.NotebookID, Kind: string(c.Kind), RevertsID: reverts, Client: string(c.Client), Message: c.Message,
		By: c.By, Now: c.At,
	}); err != nil {
		return fmt.Errorf("create changeset: %w", err)
	}
	return nil
}

// LockChangeset implements app.ChangesetWriter.
func (s *Store) LockChangeset(ctx context.Context, id uuid.UUID) (app.Changeset, error) {
	r, err := s.queries(ctx).LockChangeset(ctx, id)
	if err != nil {
		return app.Changeset{}, notFound("lock changeset", err)
	}
	c := app.Changeset{ID: r.ID, NotebookID: r.NotebookID, Kind: domain.ChangesetKind(r.Kind), Client: domain.Client(r.Client),
		By: r.CreatedByID, Reverted: r.Reverted}
	if r.RevertsID != nil {
		c.RevertsID = *r.RevertsID
	}
	return c, nil
}

// TouchChangeset implements app.ChangesetWriter.
func (s *Store) TouchChangeset(ctx context.Context, id uuid.UUID, at time.Time) error {
	if err := s.queries(ctx).TouchChangeset(ctx, gen.TouchChangesetParams{ID: id, Now: at}); err != nil {
		return fmt.Errorf("touch changeset: %w", err)
	}
	return nil
}

// RecordItems implements app.ChangesetWriter: the items' columns as
// arrays, a state there is none of and a parent at the root marked so. A
// restore's or a creation's item holds the revision the page's content is
// at; a deletion's, the revision the statement reads of the content its
// unit deleted.
func (s *Store) RecordItems(ctx context.Context, items []app.Item) error {
	if len(items) == 0 {
		return nil
	}
	n := len(items)
	p := gen.RecordItemsParams{
		ChangesetID: items[0].ChangesetID, Now: items[0].At, Ids: make([]uuid.UUID, n), NodeIds: make([]uuid.UUID, n),
		HasBefores: make([]bool, n), BeforeParentIds: make([]uuid.UUID, n), BeforeNames: make([]string, n),
		BeforeSortOrders: make([]float64, n), HasAfters: make([]bool, n), AfterParentIds: make([]uuid.UUID, n),
		AfterNames: make([]string, n), AfterSortOrders: make([]float64, n), Restored: make([]bool, n), Revisions: make([]int32, n),
	}
	for i, it := range items {
		if it.ChangesetID != p.ChangesetID || !it.At.Equal(p.Now) {
			return fmt.Errorf("record changeset items: item %d is of another changeset or time than the first's", i)
		}
		c := it.Change
		p.Ids[i], p.NodeIds[i], p.Restored[i] = it.ID, c.NodeID, c.Restored != nil
		if b := c.Before; b != nil {
			p.HasBefores[i], p.BeforeParentIds[i], p.BeforeNames[i], p.BeforeSortOrders[i] = true, orZero(b.ParentID), b.Name, b.SortOrder
		}
		if a := c.After; a != nil {
			p.HasAfters[i], p.AfterParentIds[i], p.AfterNames[i], p.AfterSortOrders[i] = true, orZero(a.ParentID), a.Name, a.SortOrder
			if c.Before == nil {
				p.Revisions[i] = int32(c.Revision)
			}
		}
	}
	if err := s.queries(ctx).RecordItems(ctx, p); err != nil {
		return fmt.Errorf("record changeset items: %w", err)
	}
	return nil
}

// orZero is the id, or the zero id for none: a parent at the root.
func orZero(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.UUID{}
	}
	return *id
}

// RecordRevision implements app.ChangesetWriter.
func (s *Store) RecordRevision(ctx context.Context, r app.Revision) error {
	var base *int32
	if r.Base != nil {
		b := int32(*r.Base)
		base = &b
	}
	n, err := s.queries(ctx).RecordRevision(ctx, gen.RecordRevisionParams{
		ID: r.ID, ChangesetID: r.ChangesetID, NodeID: r.NodeID, BaseRevision: base, Revision: int32(r.Revision),
		Content: r.Content, ContentHash: r.Hash, ByteSize: int32(r.ByteSize), Now: r.At,
	})
	switch {
	case err != nil:
		return fmt.Errorf("record page revision: %w", err)
	case n == 0:
		based := "a base"
		if r.Base != nil {
			based = fmt.Sprintf("revision %d", *r.Base)
		}
		return fmt.Errorf("record page revision: changeset %s's version of page %s is not at %s: another's write came between",
			r.ChangesetID, r.NodeID, based)
	}
	return nil
}

// DropUnchanged implements app.ChangesetWriter: two statements, the items
// then the changeset.
func (s *Store) DropUnchanged(ctx context.Context, id uuid.UUID) (bool, error) {
	q := s.queries(ctx)
	if err := q.DropUnchangedItems(ctx, id); err != nil {
		return false, fmt.Errorf("drop the unchanged items: %w", err)
	}
	n, err := q.DropEmptyChangeset(ctx, id)
	if err != nil {
		return false, fmt.Errorf("drop the empty changeset: %w", err)
	}
	return n > 0, nil
}
