package postgresadapter

import (
	"bytes"
	"context"
	"errors"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

var _ app.IngestOldBodyReads = (*IngestReads)(nil)

// OldBodyReferences reads the complete old-F body relation in the actual write
// transaction. The private app capture has already proved these emitters current
// under the notebook index lock, before any Replace/Delete. InTx alone does not
// establish that proof. There is no fallback after a query or decoding error.
func (r *IngestReads) OldBodyReferences(ctx context.Context, notebook uuid.UUID, emitters []uuid.UUID, after *app.EdgePos, take int) ([]app.BodyReference, error) {
	if take < 1 || take > app.IngestOldBodyReadRows {
		return nil, errors.New("ingest old bodies: take must be between 1 and 16384")
	}
	if after != nil && after.Start < 0 {
		return nil, errors.New("lint reads: negative cursor position")
	}
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		return nil, errors.New("ingest old bodies: outside a transaction")
	}
	if len(emitters) == 0 {
		return nil, nil
	}
	// UUID byte ordering matches PostgreSQL, including the literal zero ID.
	// Preserve the input relation: callers may provide unordered or duplicate IDs.
	first, last := emitters[0], emitters[0]
	for _, id := range emitters[1:] {
		if bytes.Compare(id[:], first[:]) < 0 {
			first = id
		}
		if bytes.Compare(id[:], last[:]) > 0 {
			last = id
		}
	}
	q := lintBodyQuerier{Querier: tx}
	var rows pgx.Rows
	var err error
	if after == nil {
		rows, err = ingestOldBodyReferencesFirst(ctx, q, gen.IngestOldBodyReferencesFirstParams{NotebookID: notebook, WikiPageIds: emitters, FirstSource: first, LastSource: last, Take: int32(take)})
	} else {
		rows, err = ingestOldBodyReferencesAfter(ctx, q, gen.IngestOldBodyReferencesAfterParams{NotebookID: notebook, WikiPageIds: emitters, FirstSource: first, LastSource: last, Take: int32(take), AfterSource: after.SourceID, AfterStart: after.Start})
	}
	if err != nil {
		return nil, err
	}
	return collectIngestOldBodyReferences(rows, take)
}
