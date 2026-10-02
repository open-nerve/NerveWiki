package postgresadapter

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
)

// NotebookActivities implements app.Activities.
func (s *Store) NotebookActivities(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]app.NotebookActivity, error) {
	rows, err := s.queries(ctx).NotebookActivities(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("notebook activities: %w", err)
	}
	out := make(map[uuid.UUID]app.NotebookActivity, len(rows))
	for _, r := range rows {
		out[r.NotebookID] = app.NotebookActivity{Bytes: r.Bytes, LastWriteAt: r.LastWriteAt}
	}
	return out, nil
}
