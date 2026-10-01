package postgresadapter

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres/gen"
)

// DeleteAuditEventsOf implements app.NotebooksDeleter.
func (s *Store) DeleteAuditEventsOf(ctx context.Context, workspaceID, by uuid.UUID, at time.Time) error {
	if err := s.queries(ctx).DeleteAuditEventsOf(ctx, gen.DeleteAuditEventsOfParams{WorkspaceID: workspaceID, By: by, Now: at}); err != nil {
		return fmt.Errorf("delete notebook audit events of workspace: %w", err)
	}
	return nil
}
