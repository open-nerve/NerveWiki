package bootstrap

import (
	"errors"
	"fmt"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// The existing sentinels, particularly the edit lock, take precedence
// over the mark. An import can pass the same refusal through treeError
// twice: it must keep the same pointer, not merely the same fields.
func TestTreeErrorKeepsTheLockFirst(t *testing.T) {
	node := uuid.NewV7()
	locked := treeError(domain.Refuse(domain.ErrLocked, node))
	var refusal *transfer.Refused
	if !errors.Is(locked, transfer.ErrLocked) || !errors.Is(locked, domain.ErrLocked) || errors.As(locked, &refusal) {
		t.Errorf("treeError(marked lock) = %v, want the lock sentinel, not a refusal", locked)
	}
	marked := domain.Refuse(domain.ErrProtected, node)
	got := treeError(marked)
	if !errors.As(got, &refusal) || refusal.Code != "page.protected" || refusal.NodeID != node ||
		!errors.Is(got, marked) || got.Error() != "page.protected: "+marked.Error() {
		t.Fatalf("treeError(marked protected) = %v, want a transfer refusal at %s, wrapping the problem", got, node)
	}
	if again := treeError(got); again != got { //nolint:errorlint // idempotence preserves the exact pointer, not another wrapper
		t.Errorf("treeError(refusal) = %p, want original %p", again, got)
	}
	wrapped := fmt.Errorf("import: %w", got)
	if again := treeError(wrapped); again != wrapped { //nolint:errorlint // preserve the incoming wrapper itself
		t.Errorf("treeError(wrapped refusal) = %v, want the original wrapper", again)
	}
	if got := treeError(domain.ErrProtected); got != domain.ErrProtected { //nolint:errorlint // unmarked errors pass through unchanged
		t.Errorf("treeError(unmarked protected) = %v, want the original problem", got)
	}
}
