package app

import (
	"context"
	"log/slog"
)

// cleanupBatch is the most sessions one statement deletes: each batch is a
// short statement of its own, however many sessions have expired.
const cleanupBatch = 1000

// CleanupEditSessions deletes the edit sessions expired and not ended: the
// periodic job page.cleanup_expired_edit_sessions (M4/P4 design 3.5). An
// expired session is no event (M4 design 4): no subscriber follows it.
type CleanupEditSessions struct {
	sessions ExpiredSessions
	clock    Clock
	logger   *slog.Logger
}

// NewCleanupEditSessions returns the use case.
func NewCleanupEditSessions(sessions ExpiredSessions, clock Clock, logger *slog.Logger) *CleanupEditSessions {
	return &CleanupEditSessions{sessions: sessions, clock: clock, logger: logger}
}

// Execute deletes, batch after batch, the sessions expired at the clock's
// now, until a batch comes back short. A session another transaction holds
// is skipped and left to the next run. It returns how many it deleted,
// those of the batches before a failure too.
func (c *CleanupEditSessions) Execute(ctx context.Context) (int, error) {
	now := c.clock.Now()
	deleted := 0
	for {
		n, err := c.sessions.DeleteExpiredSessions(ctx, now, cleanupBatch)
		deleted += n
		if err != nil {
			return deleted, err
		}
		if n < cleanupBatch {
			break
		}
	}
	if deleted > 0 {
		c.logger.InfoContext(ctx, "expired edit sessions deleted", slog.Int("deleted", deleted))
	}
	return deleted, nil
}
