package domain_test

import (
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// PagesLocked carries its locks, matches its code under errors.Is, and
// leaves the shared error without them.
func TestPagesLockedCarriesItsLocks(t *testing.T) {
	locks := []shared.LockHolder{{PageID: uuid.New(), UserID: uuid.New(), DisplayName: "Ada"}}
	var e *shared.Error
	if err := domain.PagesLocked(locks); !errors.As(err, &e) || !errors.Is(err, domain.ErrPagesLocked) {
		t.Fatalf("PagesLocked() = %v, want linking.pages_locked", err)
	}
	if !slices.Equal(e.Locks, locks) || domain.ErrPagesLocked.Locks != nil {
		t.Errorf("locks %+v, the shared error's %+v", e.Locks, domain.ErrPagesLocked.Locks)
	}
}
