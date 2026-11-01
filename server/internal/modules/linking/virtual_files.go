package linking

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

type FileIndexHead struct {
	ID        uuid.UUID
	Typed     bool
	TypeStart string
	TypeKey   string
}

type FileIndexFacts struct {
	Type, Summary *string
	Tags          []string
}

// IndexFileReads exposes existing bounded index projections without a public
// index request's pagination, authorization or catalogue scan.
type IndexFileReads interface {
	Heads(context.Context, uuid.UUID) ([]FileIndexHead, error)
	Items(context.Context, []uuid.UUID) (map[uuid.UUID]FileIndexFacts, error)
	Inbound(context.Context, []uuid.UUID) (map[uuid.UUID]int, error)
}

type indexFileReads struct{ store *postgresadapter.Store }

func NewIndexFileReads(pool *pgxpool.Pool) IndexFileReads {
	return indexFileReads{postgresadapter.New(pool)}
}

func (r indexFileReads) Heads(ctx context.Context, notebook uuid.UUID) ([]FileIndexHead, error) {
	rows, err := r.store.IndexPages(ctx, notebook, nil)
	if err != nil {
		return nil, err
	}
	out := make([]FileIndexHead, len(rows))
	keyer := domain.NewIndexKeyer()
	for i, row := range rows {
		out[i] = FileIndexHead{ID: row.ID, Typed: row.Typed, TypeStart: row.TypeStart, TypeKey: keyer.Key(row.TypeStart)}
	}
	return out, nil
}

func (r indexFileReads) Items(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]FileIndexFacts, error) {
	rows, err := r.store.IndexItems(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]FileIndexFacts, len(rows))
	for id, row := range rows {
		out[id] = FileIndexFacts{Type: row.Type, Summary: row.Summary, Tags: row.Tags}
	}
	return out, nil
}

func (r indexFileReads) Inbound(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	return r.store.InboundCounts(ctx, ids, 1000)
}
