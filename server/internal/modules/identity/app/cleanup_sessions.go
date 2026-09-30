package app

import (
	"context"
	"log/slog"
)

// cleanupBatch is the most sessions one statement deletes: each batch is a
// short statement of its own, however many sessions have expired.
const cleanupBatch = 1000

// CleanupSessions deletes the expired sessions: the periodic job
// identity.cleanup_expired_sessions (M1/P4 design 3.5). An expired session
// authenticates nothing and refreshes nothing; only its row is left.
type CleanupSessions struct {
	sessions ExpiredSessionDeleter
	clock    Clock
	logger   *slog.Logger
}

// NewCleanupSessions returns the use case.
func NewCleanupSessions(sessions ExpiredSessionDeleter, clock Clock, logger *slog.Logger) *CleanupSessions {
	return &CleanupSessions{sessions: sessions, clock: clock, logger: logger}
}

// Execute deletes, batch after batch, the sessions that expired before the
// clock's now, until a batch comes back short. A session another
// transaction holds is skipped and left to the next run. It returns how
// many it deleted, those of the batches before a failure too.
func (u *CleanupSessions) Execute(ctx context.Context) (int, error) {
	now := u.clock.Now()
	deleted := 0
	for {
		n, err := u.sessions.DeleteExpiredSessions(ctx, now, cleanupBatch)
		deleted += n
		if err != nil {
			return deleted, err
		}
		if n < cleanupBatch {
			break
		}
	}
	if deleted > 0 {
		u.logger.InfoContext(ctx, "expired sessions deleted", slog.Int("deleted", deleted))
	}
	return deleted, nil
}
