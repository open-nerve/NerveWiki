package markdownadapter

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// retryAfter is the Retry-After of the 503 when the budget does not free
// up in time.
const retryAfter = time.Second

// Budget implements app.ParseBudget (M4/P4 review P2): the bytes of
// content parsed and rendered at once, page.parse_budget_bytes, which a
// request waits for at most page.parse_max_wait, as the password hashes'
// slots (auth.password.max_concurrent_hashes) do. The waiters are served in
// the order they came.
type Budget struct {
	bytes   *semaphore.Weighted
	size    int
	wait    time.Duration
	logger  *slog.Logger
	waiting atomic.Int64
}

// NewBudget returns a budget of size bytes; a request waits at most wait.
func NewBudget(size int, wait time.Duration, logger *slog.Logger) *Budget {
	return &Budget{bytes: semaphore.NewWeighted(int64(size)), size: size, wait: wait, logger: logger}
}

// Take implements app.ParseBudget. A content larger than the budget takes
// all of it; nothing is taken for an empty one.
func (b *Budget) Take(ctx context.Context, n int) (func(), error) {
	n = min(n, b.size)
	if n <= 0 {
		return func() {}, nil
	}
	if b.bytes.TryAcquire(int64(n)) {
		return b.releaser(n), nil
	}
	queued := b.waiting.Add(1)
	defer b.waiting.Add(-1)
	waited, cancel := context.WithTimeout(ctx, b.wait)
	defer cancel()
	if err := b.bytes.Acquire(waited, int64(n)); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			b.logger.LogAttrs(ctx, slog.LevelInfo, "content parsing is saturated", slog.Int64("queued", queued))
			return nil, shared.ServerBusy(retryAfter)
		}
		return nil, err
	}
	return b.releaser(n), nil
}

// releaser gives n bytes back once, however often it is called.
func (b *Budget) releaser(n int) func() {
	var once atomic.Bool
	return func() {
		if once.CompareAndSwap(false, true) {
			b.bytes.Release(int64(n))
		}
	}
}
