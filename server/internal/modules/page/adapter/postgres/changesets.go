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
	if err := s.queries(ctx).CreateChangeset(ctx, gen.CreateChangesetParams{
		ID: c.ID, NotebookID: c.NotebookID, Kind: string(c.Kind), Client: string(c.Client), Message: c.Message, By: c.By, Now: c.At,
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
	return app.Changeset{ID: r.ID, NotebookID: r.NotebookID, Kind: domain.ChangesetKind(r.Kind), Client: domain.Client(r.Client),
		By: r.CreatedByID}, nil
}

// TouchChangeset implements app.ChangesetWriter.
func (s *Store) TouchChangeset(ctx context.Context, id uuid.UUID, at time.Time) error {
	if err := s.queries(ctx).TouchChangeset(ctx, gen.TouchChangesetParams{ID: id, Now: at}); err != nil {
		return fmt.Errorf("touch changeset: %w", err)
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
	} else {
		p.DeletedAt = &it.At
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
