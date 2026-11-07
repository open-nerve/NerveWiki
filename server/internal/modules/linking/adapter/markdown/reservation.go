package markdownadapter

import (
	"context"
	"errors"
	"sync"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

var _ app.RewriteReservations = Parser{}

// TryReserve is optional admission before any extra task starts. Unlike
// ParseNow, its false answer is not a public busy or a failed parse.
func (p Parser) TryReserve(upperBytes int) (app.RewriteReservation, bool) {
	if upperBytes < 0 {
		return nil, false
	}
	parent, ok := p.budget.TryReserve(p.budget.Takes(upperBytes))
	if !ok {
		return nil, false
	}
	return &parserReservation{parser: p, parent: parent, upper: upperBytes}, true
}

type parserReservation struct {
	parser Parser
	parent *markdown.Reservation
	mu     sync.Mutex
	upper  int
}

func (r *parserReservation) TryGrow(upperBytes int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if upperBytes < 0 || !r.parent.TryGrow(r.parser.budget.Takes(upperBytes)) {
		return false
	}
	r.upper = max(r.upper, upperBytes)
	return true
}

func (r *parserReservation) ParseNow(_ context.Context, content string) (app.Parsed, error) {
	r.mu.Lock()
	if len(content) > r.upper {
		r.mu.Unlock()
		return app.Parsed{}, errors.New("rewrite parse reservation: content exceeds its admitted bound")
	}
	hold, ok := r.parent.Take(len(content))
	r.mu.Unlock()
	if !ok {
		return app.Parsed{}, errors.New("rewrite parse reservation: closed or occupied grant")
	}
	return r.parser.parseHeld(content, hold)
}

func (r *parserReservation) Close() { r.parent.Close() }
