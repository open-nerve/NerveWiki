package domain_test

import (
	"errors"
	"fmt"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// A refusal is marked with its node once (M10/P1 design 3.22): Refuse of a
// marked error keeps its node, and errors.Is and errors.As see the problem
// through the mark, wrapped or not.
func TestARefusalIsMarkedOnce(t *testing.T) {
	first, second := uuid.NewV7(), uuid.NewV7()
	err := domain.Refuse(domain.ErrProtected, first)
	var r *domain.Refused
	if !errors.As(err, &r) || r.NodeID != first || !errors.Is(err, domain.ErrProtected) || err.Error() != domain.ErrProtected.Error() {
		t.Fatalf("Refuse = %v (%+v), want page.protected of the first node", err, r)
	}
	wrapped := fmt.Errorf("the guard: %w", err)
	if again := domain.Refuse(wrapped, second); again != wrapped || !errors.As(again, &r) || r.NodeID != first { //nolint:errorlint // the very error
		t.Errorf("Refuse of a marked error = %v (%+v), want it as it was, of the first node", again, r)
	}
	if !errors.Is(wrapped, domain.ErrProtected) {
		t.Errorf("a wrapped refusal is not page.protected: %v", wrapped)
	}
}
