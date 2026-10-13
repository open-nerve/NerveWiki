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

// FindAnyNode implements app.Nodes.
func (s *Store) FindAnyNode(ctx context.Context, id uuid.UUID) (app.AnyNode, error) {
	r, err := s.queries(ctx).FindAnyNode(ctx, id)
	if err != nil {
		return app.AnyNode{}, notFound("find any node", err)
	}
	return app.AnyNode{Node: nodeOf(gen.FindNodeRow{
		ID: r.ID, NotebookID: r.NotebookID, ParentID: r.ParentID, Kind: r.Kind, Name: r.Name, NameKey: r.NameKey,
		SortOrder: r.SortOrder, CreatedByID: r.CreatedByID, UpdatedByID: r.UpdatedByID, CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}), DeletedAt: r.DeletedAt, TrashRoot: r.TrashRoot}, nil
}

// ListVersions implements app.Versions.
func (s *Store) ListVersions(ctx context.Context, pageID uuid.UUID, below int, limit int) ([]domain.Version, error) {
	rows, err := s.queries(ctx).ListVersions(ctx, gen.ListVersionsParams{NodeID: pageID, Below: int64(below), Max: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	out := make([]domain.Version, len(rows))
	for i, r := range rows {
		out[i] = versionOf(r)
	}
	return out, nil
}

// FindVersion implements app.Versions.
func (s *Store) FindVersion(ctx context.Context, pageID uuid.UUID, revision int) (domain.Version, string, error) {
	r, err := s.queries(ctx).FindVersion(ctx, gen.FindVersionParams{NodeID: pageID, Revision: int32(revision)})
	if err != nil {
		return domain.Version{}, "", notFound("find version", err)
	}
	return versionOf(gen.ListVersionsRow{
		ID: r.ID, BaseRevision: r.BaseRevision, Revision: r.Revision, ByteSize: r.ByteSize, CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt, ChangesetID: r.ChangesetID, Kind: r.Kind, Client: r.Client, CreatedByID: r.CreatedByID,
		ChangesetCreatedAt: r.ChangesetCreatedAt,
	}), r.Content, nil
}

// AliveChangesetOf implements app.Versions.
func (s *Store) AliveChangesetOf(ctx context.Context, pageID uuid.UUID, now time.Time) (uuid.UUID, error) {
	ids, err := s.queries(ctx).AliveChangesetOf(ctx, gen.AliveChangesetOfParams{NodeID: pageID, Now: now})
	switch {
	case err != nil:
		return uuid.UUID{}, fmt.Errorf("alive changeset of page: %w", err)
	case len(ids) == 0 || ids[0] == nil:
		return uuid.UUID{}, nil
	}
	return *ids[0], nil
}

// versionOf is a row of the version queries, which read the same columns but for FindVersion's content.
func versionOf(r gen.ListVersionsRow) domain.Version {
	v := domain.Version{
		ID: r.ID, Revision: int(r.Revision), ByteSize: int(r.ByteSize), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		Changeset: domain.VersionChangeset{
			ID: r.ChangesetID, Kind: domain.ChangesetKind(r.Kind), Client: domain.Client(r.Client), By: r.CreatedByID,
			CreatedAt: r.ChangesetCreatedAt,
		},
	}
	if r.BaseRevision != nil {
		base := int(*r.BaseRevision)
		v.Base = &base
	}
	return v
}
