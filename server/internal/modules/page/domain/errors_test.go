package domain_test

import (
	"errors"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Locked and Unlocked carry their problem's member (M5 design 4.5), match
// their code under errors.Is, and leave the shared errors without it.
func TestLockProblemsCarryTheirMembers(t *testing.T) {
	page, holder, admin := uuid.New(), uuid.New(), uuid.New()

	var locked *shared.Error
	if err := domain.Locked(page, holder, "Ada"); !errors.As(err, &locked) || !errors.Is(err, domain.ErrLocked) {
		t.Fatalf("Locked() = %v, want page.locked", err)
	}
	if p, u, name, ok := locked.ProblemLock(); !ok || p != page || u != holder || name != "Ada" {
		t.Errorf("Locked's lock = %s %s %q %v, want %s %s Ada", p, u, name, ok, page, holder)
	}

	var unlocked *shared.Error
	if err := domain.Unlocked(admin, "Grace"); !errors.As(err, &unlocked) || !errors.Is(err, domain.ErrEditSessionUnlocked) {
		t.Fatalf("Unlocked() = %v, want page.edit_session_unlocked", err)
	}
	if u, name, ok := unlocked.ProblemEndedBy(); !ok || u != admin || name != "Grace" {
		t.Errorf("Unlocked's ended_by = %s %q %v, want %s Grace", u, name, ok, admin)
	}

	if domain.ErrLocked.Lock != nil || domain.ErrEditSessionUnlocked.EndedBy != nil {
		t.Error("a constructor set its member on the shared error")
	}
}
