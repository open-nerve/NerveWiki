package markdown

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
)

// ErrBusy is a Take's when the budget does not free up in time: the
// caller's adapter answers it 503 server_busy.
var ErrBusy = errors.New("markdown: the parse budget did not free up in time")

// Budget bounds the bytes of content parsed and rendered at once (M4/P4
// review P2; M6 design 4.7): a parse holds some 300 times its content at
// worst. The composition root makes one, of page.parse_budget_bytes, which
// a request waits for at most page.parse_max_wait, as the password hashes'
// slots (auth.password.max_concurrent_hashes) are waited for, and hands it
// to every module that parses. The waiters are served in the order they
// came.
type Budget struct {
	bytes   *semaphore.Weighted
	size    int
	wait    time.Duration
	logger  *slog.Logger
	waiting atomic.Int64
}

// NewBudget returns a budget of size bytes; a request waits at most wait.
// Both must be positive, as the configuration's validation has them: a
// budget of nothing would bound nothing, silently.
func NewBudget(size int, wait time.Duration, logger *slog.Logger) *Budget {
	if size < 1 || wait <= 0 {
		panic(fmt.Sprintf("markdown: a parse budget of %d bytes and a wait of %s", size, wait))
	}
	return &Budget{bytes: semaphore.NewWeighted(int64(size)), size: size, wait: wait, logger: logger}
}

// Take holds n bytes of the budget until release, waiting for them at most
// the budget's wait: then it answers ErrBusy, or the context's error if
// the request ran out first. A content larger than the budget takes all of
// it; nothing is taken for an empty one.
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
			return nil, ErrBusy
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
