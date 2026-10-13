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

// NotTrashRoot carries its root_id, matches its code under errors.Is, and
// leaves the shared error without it.
func TestNotTrashRootCarriesItsRoot(t *testing.T) {
	root := uuid.New()
	var e *shared.Error
	if err := domain.NotTrashRoot(root); !errors.As(err, &e) || !errors.Is(err, domain.ErrNotTrashRoot) {
		t.Fatalf("NotTrashRoot() = %v, want page.not_trash_root", err)
	}
	if id, ok := e.ProblemRootID(); !ok || id != root {
		t.Errorf("NotTrashRoot's root_id = %s %v, want %s", id, ok, root)
	}
	if domain.ErrNotTrashRoot.RootID != nil {
		t.Error("NotTrashRoot set its member on the shared error")
	}
}

// RevertConflicts carries its conflicts and their count, matches its code
// under errors.Is, and leaves the shared error without them.
func TestRevertConflictsCarryTheirMembers(t *testing.T) {
	a, b, holder := uuid.New(), uuid.New(), uuid.New()
	conflicts := []domain.RevertConflict{
		{NodeID: a, Name: "A", Reason: domain.ConflictChanged},
		{NodeID: b, Name: "B", Reason: domain.ConflictTitleTaken, HolderID: &holder},
	}
	var e *shared.Error
	if err := domain.RevertConflicts(conflicts, 101); !errors.As(err, &e) || !errors.Is(err, domain.ErrRevertConflict) {
		t.Fatalf("RevertConflicts() = %v, want page.revert_conflict", err)
	}
	var got []shared.Conflict
	n, ok := e.ProblemConflicts(func(id uuid.UUID, name, reason string, h *uuid.UUID) {
		got = append(got, shared.Conflict{NodeID: id, Name: name, Reason: reason, HolderID: h})
	})
	want := []shared.Conflict{{NodeID: a, Name: "A", Reason: "changed"}, {NodeID: b, Name: "B", Reason: "title_taken", HolderID: &holder}}
	if !ok || n != 101 || len(got) != 2 || got[0] != want[0] || got[1].NodeID != b || got[1].Reason != "title_taken" ||
		got[1].HolderID == nil || *got[1].HolderID != holder {
		t.Errorf("RevertConflicts' members = %+v, %d %v; want %+v of 101", got, n, ok, want)
	}
	if domain.ErrRevertConflict.Conflicts != nil || domain.ErrRevertConflict.ConflictCount != nil {
		t.Error("RevertConflicts set its members on the shared error")
	}
}
