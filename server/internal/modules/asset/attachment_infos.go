package asset

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
)

// AttachmentInfo is an attachment's type and size.
type AttachmentInfo = app.AttachmentInfo

// AttachmentInfos reads the attachments' type and size by their nodes,
// deleted or not (M8 design 4.1): the page module's trash shows them of an
// attachment in the trash, which no other read of the module gives. Built
// from the pool alone, as the composition root builds the page module
// before this one (v0.1 design 13.1, item 11).
type AttachmentInfos struct {
	store *postgresadapter.Store
}

// NewAttachmentInfos returns the reads over pool.
func NewAttachmentInfos(pool *pgxpool.Pool) AttachmentInfos {
	return AttachmentInfos{store: postgresadapter.New(pool)}
}

// Of are the infos of the attachments of nodeIDs, by node, in the caller's
// read: a node without a row of its file is not among them.
func (a AttachmentInfos) Of(ctx context.Context, nodeIDs []uuid.UUID) (map[uuid.UUID]AttachmentInfo, error) {
	return a.store.AttachmentInfos(ctx, nodeIDs)
}
