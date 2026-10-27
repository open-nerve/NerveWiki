package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// notebookSourceHashes gives downgrade the page module's pool-only
// cleanup port. Forget uses the transaction and notebook lock in ctx.
type notebookSourceHashes struct {
	area page.SourceArea
}

func (s notebookSourceHashes) Forget(ctx context.Context, notebookID uuid.UUID) error {
	return s.area.Forget(ctx, notebookID)
}
