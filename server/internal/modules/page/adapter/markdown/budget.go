package markdownadapter

import (
	"context"
	"errors"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// retryAfter is the Retry-After of the 503 when the budget does not free
// up in time.
const retryAfter = time.Second

// Budget implements app.ParseBudget with the platform's budget, the one
// the composition root hands to every module that parses (M6 design 4.7).
type Budget struct {
	budget *markdown.Budget
}

// NewBudget returns the adapter of b.
func NewBudget(b *markdown.Budget) *Budget {
	return &Budget{budget: b}
}

// Take implements app.ParseBudget: a budget that does not free up in time
// is 503 server_busy, to retry after a second.
func (b *Budget) Take(ctx context.Context, n int) (func(), error) {
	release, err := b.budget.Take(ctx, n)
	if errors.Is(err, markdown.ErrBusy) {
		return nil, shared.ServerBusy(retryAfter)
	}
	return release, err
}
