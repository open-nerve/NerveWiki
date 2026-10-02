package postgresadapter

import (
	"context"
	"fmt"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
)

// CreateChangeset implements app.ChangesetWriter.
func (s *Store) CreateChangeset(ctx context.Context, c app.Changeset) error {
	if err := s.queries(ctx).CreateChangeset(ctx, gen.CreateChangesetParams{
		ID: c.ID, NotebookID: c.NotebookID, Kind: c.Kind, Client: string(c.Client), Message: c.Message, By: c.By, Now: c.At,
	}); err != nil {
		return fmt.Errorf("create changeset: %w", err)
	}
	return nil
}

// RecordItem implements app.ChangesetWriter.
func (s *Store) RecordItem(ctx context.Context, it app.Item) error {
	p := gen.RecordItemParams{ID: it.ID, ChangesetID: it.ChangesetID, NodeID: it.Change.NodeID, Now: it.At}
	if b := it.Change.Before; b != nil {
		p.BeforeParentID, p.BeforeName, p.BeforeSortOrder = b.ParentID, &b.Name, &b.SortOrder
	}
	if a := it.Change.After; a != nil {
		p.AfterParentID, p.AfterName, p.AfterSortOrder = a.ParentID, &a.Name, &a.SortOrder
	}
	if err := s.queries(ctx).RecordItem(ctx, p); err != nil {
		return fmt.Errorf("record changeset item: %w", err)
	}
	return nil
}

// RecordRevision implements app.ChangesetWriter.
func (s *Store) RecordRevision(ctx context.Context, r app.Revision) error {
	var base *int32
	if r.Base != nil {
		b := int32(*r.Base)
		base = &b
	}
	if err := s.queries(ctx).RecordRevision(ctx, gen.RecordRevisionParams{
		ID: r.ID, ChangesetID: r.ChangesetID, NodeID: r.NodeID, BaseRevision: base, Revision: int32(r.Revision),
		Content: r.Content, ContentHash: r.Hash, ByteSize: int32(r.ByteSize), Now: r.At,
	}); err != nil {
		return fmt.Errorf("record page revision: %w", err)
	}
	return nil
}
