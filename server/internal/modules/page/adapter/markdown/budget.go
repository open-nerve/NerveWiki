package markdownadapter

import (
	"context"
	"errors"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
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

// NewBudget returns the adapter of b, which must be there: a module
// without the server's budget is a fault of its wiring (M6/P2 review L2).
func NewBudget(b *markdown.Budget) *Budget {
	if b == nil {
		panic("markdownadapter: no parse budget")
	}
	return &Budget{budget: b}
}

// Take implements app.ParseBudget: a budget that does not free up in time
// is 503 server_busy, to retry after a second.
func (b *Budget) Take(ctx context.Context, n int) (app.BudgetHold, error) {
	hold, err := b.budget.Take(ctx, n)
	switch {
	case errors.Is(err, markdown.ErrBusy):
		return nil, shared.ServerBusy(retryAfter)
	case err != nil:
		return nil, err
	}
	return hold, nil
}
