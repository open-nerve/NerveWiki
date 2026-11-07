package postgresadapter

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// linksReached reads the original generated statement through the planned
// querier supplied by Store.Links, retaining only final app.Link values.
func linksReached(ctx context.Context, q postgres.Querier, notebookID uuid.UUID, r domain.Reach) ([]app.Link, error) {
	rows, err := q.Query(ctx, gen.LinksReached, notebookID, r.Keys, r.Targets, r.Sources)
	if err != nil {
		return nil, err
	}
	var links linksReachedBuffer
	return links.read(rows)
}

const linksReachedBlockSize = 512

// linksReachedBuffer belongs to one query. Full blocks never grow; only the
// first block starts small, so a tiny result does not reserve 512 links.
type linksReachedBuffer struct {
	full  [][]app.Link
	tail  []app.Link
	count int
}

func (b *linksReachedBuffer) add(link app.Link) {
	if len(b.tail) == linksReachedBlockSize {
		b.full = append(b.full, b.tail)
		b.tail = nil
	}
	if len(b.tail) == cap(b.tail) {
		size := linksReachedBlockSize
		if len(b.full) == 0 {
			size = min(linksReachedBlockSize, max(8, cap(b.tail)*2))
		}
		next := make([]app.Link, len(b.tail), size)
		copy(next, b.tail)
		b.tail = next
	}
	b.tail = append(b.tail, link)
	b.count++
}

func (b *linksReachedBuffer) finish() []app.Link {
	// The exact final allocation also preserves a non-nil empty result.
	out := make([]app.Link, b.count)
	at := 0
	for _, block := range b.full {
		at += copy(out[at:], block)
	}
	copy(out[at:], b.tail)
	return out
}

func (b *linksReachedBuffer) read(rows pgx.Rows) ([]app.Link, error) {
	defer rows.Close()
	var row gen.LinksReachedRow
	dest := [...]any{&row.SourceID, &row.RangeStart, &row.Target, &row.ResolvedID,
		&row.Ambiguous, &row.ResolvedAsset, &row.Aliases}
	for rows.Next() {
		if err := rows.Scan(dest[:]...); err != nil {
			return nil, err
		}
		link := app.Link{SourceID: row.SourceID, Start: int(row.RangeStart), Target: row.Target, Aliases: row.Aliases}
		if row.ResolvedID != nil {
			link.Resolution = domain.Resolution{ID: *row.ResolvedID, Ambiguous: row.Ambiguous, Asset: row.ResolvedAsset}
		}
		b.add(link)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return b.finish(), nil
}
