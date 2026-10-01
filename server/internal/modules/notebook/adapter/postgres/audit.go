package postgresadapter

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
)

// DeleteAuditEventsOf implements app.NotebooksDeleter.
func (s *Store) DeleteAuditEventsOf(ctx context.Context, workspaceID, by uuid.UUID, at time.Time) error {
	if err := s.queries(ctx).DeleteAuditEventsOf(ctx, gen.DeleteAuditEventsOfParams{WorkspaceID: workspaceID, By: by, Now: at}); err != nil {
		return fmt.Errorf("delete notebook audit events of workspace: %w", err)
	}
	return nil
}

// AddAuditEvent implements app.AuditRecorder.
func (s *Store) AddAuditEvent(ctx context.Context, e domain.AuditEvent) error {
	err := s.queries(ctx).AddAuditEvent(ctx, gen.AddAuditEventParams{
		ID: e.ID, WorkspaceID: e.WorkspaceID, NotebookID: e.NotebookID, NotebookName: e.NotebookName, Action: string(e.Action),
		FormerOwnerID: e.FormerOwnerID, ActorID: e.ActorID, At: e.At,
	})
	if err != nil {
		return fmt.Errorf("add notebook audit event: %w", err)
	}
	return nil
}
