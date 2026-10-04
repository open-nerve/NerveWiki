package markdown

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
)

// ErrBusy is a Take's when the budget does not free up in time: the
// caller's adapter answers it 503 server_busy.
var ErrBusy = errors.New("markdown: the parse budget did not free up in time")

// FactsRatio is the most the Facts of a content hold, in times its size,
// beyond what its frontmatter's values hold (Facts.Limit;
// markdowntest.CheckCosts; M6/P2 review M1: some 20 at worst, a page of
// nothing but wikilinks), where its parse holds some 300 (parseRatio, M4/P3
// design 3.10).
const (
	FactsRatio = 30
	parseRatio = 300
)

// Budget bounds the bytes of content parsed at once (M4/P4 review P2; M6
// design 4.7), and the facts kept of them: a parse holds some 300 times its
// content at worst, its facts what Facts.Limit says, about a tenth of
// that. The composition root makes
// one and hands it to every module that parses; a take waits for its bytes
// at most the budget's wait. The waiters are served in the order they
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

// minTake is the least a content takes (M6/P2 fix check 3 L1): however
// short, a frontmatter's aliases may expand to the YAML's limit of values,
// whose facts keep up to some 480 KB, the parse not much more; 4 KiB counts
// 1.2 MB. Its strings' paths take at most maxYAMLPathsRatio times the
// YAML's size, well within the parseRatio times it counts for.
const minTake = 4 << 10

// Take holds the bytes of a content of n bytes, for its parse, waiting for
// them at most the budget's wait: then it answers ErrBusy, or the
// context's error if the request ran out first. A content takes at least
// minTake, one larger than the budget all of it; nothing is taken for an
// empty one.
func (b *Budget) Take(ctx context.Context, n int) (*Hold, error) {
	h := &Hold{budget: b, content: max(n, 0)}
	if n <= 0 {
		return h, nil
	}
	h.n = min(max(n, minTake), b.size)
	if b.bytes.TryAcquire(int64(h.n)) {
		return h, nil
	}
	queued := b.waiting.Add(1)
	defer b.waiting.Add(-1)
	waited, cancel := context.WithTimeout(ctx, b.wait)
	defer cancel()
	if err := b.bytes.Acquire(waited, int64(h.n)); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			b.logger.LogAttrs(ctx, slog.LevelInfo, "content parsing is saturated", slog.Int64("queued", queued))
			return nil, ErrBusy
		}
		return nil, err
	}
	return h, nil
}

// Hold is the bytes of the budget a content holds.
type Hold struct {
	budget  *Budget
	content int // the content's size
	mu      sync.Mutex
	n       int // the bytes held
}

// KeepFacts gives back what the content's parse held beyond what f, its
// facts, hold: f.Limit counted as the parse is, a parseRatio-th of it. The
// tree is gone, the facts are kept until Release (M6/P2 review M1). It
// keeps no more than the take held, at least minTake, which what a short
// frontmatter's aliases keep is within.
func (h *Hold) KeepFacts(f Facts) {
	h.keep((f.Limit(h.content) + parseRatio - 1) / parseRatio)
}

// Release gives back what is held; again, nothing.
func (h *Hold) Release() { h.keep(0) }

// keep gives back all but n of the bytes held, holding no more than it did.
func (h *Hold) keep(n int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if n < h.n {
		h.budget.bytes.Release(int64(h.n - n))
		h.n = n
	}
}
