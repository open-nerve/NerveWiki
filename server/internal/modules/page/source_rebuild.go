package page

import (
	"context"
	"errors"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// SourceRebuild rebuilds a notebook's source hashes as an upgrade's
// approval makes it an LLM Wiki (M11/P3 design 3.4), built from the pool
// alone (v0.1 design 13.1 item 11): the wiki module's approval calls it
// through its port, in its own transaction.
type SourceRebuild struct {
	store       *postgresadapter.Store
	attachments AttachmentInfos
}

// NewSourceRebuild returns the rebuild over pool; attachments are the
// attachments' files' metadata, the asset module's, as the units'
// (Deps.Attachments). None is a fault of the wiring, as New's.
func NewSourceRebuild(pool *pgxpool.Pool, attachments AttachmentInfos) SourceRebuild {
	if attachments == nil {
		panic("page: no attachment infos")
	}
	return SourceRebuild{store: postgresadapter.New(pool), attachments: attachments}
}

// Rebuild is RebuildSources in the transaction ctx carries, which holds
// the notebook's row: notebook's live nodes' source hashes cleared, then
// the strict live descendants' of root set; how many it set. None is an
// error: the hashes and the mode it goes with must be one transaction's.
func (s SourceRebuild) Rebuild(ctx context.Context, notebook, root uuid.UUID) (int, error) {
	if !postgres.InTx(ctx) {
		return 0, errors.New("rebuild the source hashes: not in a transaction")
	}
	return app.RebuildSources(ctx, s.store, s.attachments, notebook, root)
}
