package postgresadapter

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Keep each own-gen statement and its generated parameter order explicit.
// The caller supplies the actual transaction's lintBodyQuerier execution mode.
func ingestOldBodyReferencesFirst(ctx context.Context, q postgres.Querier, arg gen.IngestOldBodyReferencesFirstParams) (pgx.Rows, error) {
	return q.Query(ctx, gen.IngestOldBodyReferencesFirst, arg.NotebookID, arg.FirstSource, arg.LastSource, arg.Take, arg.WikiPageIds)
}

func ingestOldBodyReferencesAfter(ctx context.Context, q postgres.Querier, arg gen.IngestOldBodyReferencesAfterParams) (pgx.Rows, error) {
	return q.Query(ctx, gen.IngestOldBodyReferencesAfter, arg.NotebookID, arg.FirstSource, arg.LastSource, arg.AfterSource, arg.AfterStart, arg.Take, arg.WikiPageIds)
}

// A single generated row supplies four stable Scan destinations. Consume every
// column and native terminal error before returning values to the app. NULL is
// a logical projection error, so a later Scan/Err still takes precedence, as it
// did when sqlc first collected the complete result. No Rows escape the adapter.
func collectIngestOldBodyReferences(rows pgx.Rows, take int) ([]app.BodyReference, error) {
	defer rows.Close()
	var row gen.IngestOldBodyReferencesFirstRow
	dest := [...]any{&row.SourceID, &row.RangeStart, &row.ResolvedID, &row.ResolvedAsset}
	var out []app.BodyReference
	badTarget, tooMany := false, false
	count := 0
	for rows.Next() {
		if err := rows.Scan(dest[:]...); err != nil {
			return nil, err
		}
		count++
		if count > take {
			tooMany = true
		}
		if row.ResolvedID == nil {
			badTarget = true
			continue
		}
		if badTarget || tooMany {
			continue
		}
		if out == nil {
			out = make([]app.BodyReference, 0, take)
		}
		out = append(out, app.BodyReference{EdgePos: app.EdgePos{SourceID: row.SourceID, Start: row.RangeStart}, ResolvedID: *row.ResolvedID, ResolvedAsset: row.ResolvedAsset})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if badTarget {
		return nil, errors.New("lint reads: body reference has no target ID")
	}
	if tooMany {
		return nil, errors.New("ingest old bodies: query exceeds requested batch")
	}
	return out, nil
}
