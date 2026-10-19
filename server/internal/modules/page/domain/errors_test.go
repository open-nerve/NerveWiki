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

// LockedAll names every holder, the first as lock too, in the order given,
// matches page.locked, and leaves the shared error without them.
func TestLockedAllNamesEveryHolder(t *testing.T) {
	holders := []shared.LockHolder{
		{PageID: uuid.NewV7(), UserID: uuid.NewV7(), DisplayName: "Ada"},
		{PageID: uuid.NewV7(), UserID: uuid.NewV7(), DisplayName: "Grace"},
		{PageID: uuid.NewV7(), UserID: uuid.NewV7(), DisplayName: "Ada"},
	}
	var e *shared.Error
	if err := domain.LockedAll(holders); !errors.As(err, &e) || !errors.Is(err, domain.ErrLocked) {
		t.Fatalf("LockedAll() = %v, want page.locked", err)
	}
	if p, u, name, ok := e.ProblemLock(); !ok || p != holders[0].PageID || u != holders[0].UserID || name != "Ada" {
		t.Errorf("LockedAll's lock = %s %s %q %v, want the first holder", p, u, name, ok)
	}
	var got []shared.LockHolder
	e.ProblemLocks(func(p, u uuid.UUID, name string) {
		got = append(got, shared.LockHolder{PageID: p, UserID: u, DisplayName: name})
	})
	if len(got) != 3 || got[0] != holders[0] || got[1] != holders[1] || got[2] != holders[2] {
		t.Errorf("LockedAll's locks = %+v, want %+v", got, holders)
	}
	if domain.ErrLocked.Lock != nil || domain.ErrLocked.Locks != nil {
		t.Error("LockedAll set its members on the shared error")
	}
}

// The errors that carry what an MCP answer says are their codes' problems
// under errors.Is and errors.As (M9/P2 design 3.8.5): page.title_taken,
// page.revision_mismatch, the lock's refusal they wrap, and the 422 on
// recursive, which counts the nodes in words and names no path.
func TestTheWritesErrorsAreTheirCodes(t *testing.T) {
	var e *shared.Error
	for _, tt := range []struct {
		err  error
		want *shared.Error
	}{
		{&domain.PathTaken{Path: "A.md"}, domain.ErrTitleTaken},
		{&domain.VersionMismatch{Path: "A.md", Why: domain.Changed}, domain.ErrRevisionMismatch},
		{&domain.LockPaths{Err: domain.Locked(uuid.NewV7(), uuid.NewV7(), "Ada")}, domain.ErrLocked},
	} {
		if !errors.Is(tt.err, tt.want) || !errors.As(tt.err, &e) || e.Code != tt.want.Code || tt.err.Error() != tt.want.Error() {
			t.Errorf("%T is %v, want %s", tt.err, tt.err, tt.want.Code)
		}
	}
	for _, tt := range []struct {
		pages, assets int
		want          string
	}{
		{3, 2, "3 pages and 2 attachments"},
		{1, 1, "1 page and 1 attachment"},
		{2, 0, "2 pages"},
		{0, 1, "1 attachment"},
	} {
		var err error = &domain.Recursive{Field: "recursive", Pages: tt.pages, Assets: tt.assets}
		if !errors.As(err, &e) || e.Code != shared.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Field != "recursive" ||
			e.Fields[0].Code != shared.FieldRequired || e.Fields[0].Message != "must be true to delete a page with children: "+tt.want ||
			err.Error() != e.Error() {
			t.Errorf("Recursive{%d, %d} = %+v", tt.pages, tt.assets, e)
		}
		if got := domain.PagesAndAttachments(tt.pages, tt.assets); got != tt.want {
			t.Errorf("PagesAndAttachments(%d, %d) = %q, want %q", tt.pages, tt.assets, got, tt.want)
		}
	}
	if err := error(&domain.AttachmentOnPath{Segment: 2}); errors.As(err, &e) {
		t.Errorf("AttachmentOnPath is the problem %s: it has none of its own", e.Code)
	}
}

// A batch's errors (M9/P3 design 3.9.2): page.batch_too_large saying which
// bound, on a copy, never the shared error; an op's failure, which is the
// error it wraps under errors.Is and errors.As and names the op and its
// tool; and page.revision_mismatch at a tree op the notebook kept
// changing under.
func TestTheBatchErrorsAreTheirCodes(t *testing.T) {
	var e *shared.Error
	for _, tt := range []struct {
		err  error
		want string
	}{
		{domain.BatchTooLarge("ops", 101, 100, "split the batch"), "101 ops, at most 100: split the batch."},
		{domain.BatchTooLarge("bytes of strings", 7, 6, ""), "7 bytes of strings, at most 6."},
	} {
		if !errors.Is(tt.err, domain.ErrBatchTooLarge) || !errors.As(tt.err, &e) || e.Code != "page.batch_too_large" ||
			e.Kind != shared.KindInvalid || e.Detail != tt.want || len(e.Fields) != 0 {
			t.Errorf("BatchTooLarge = %+v, want page.batch_too_large %q", tt.err, tt.want)
		}
	}
	if domain.ErrBatchTooLarge.Detail != "The batch is too large for one change." {
		t.Errorf("BatchTooLarge set its detail on the shared error: %q", domain.ErrBatchTooLarge.Detail)
	}
	inner := &domain.PathTaken{Path: "A.md"}
	var err error = &domain.OpFailed{Op: 3, Tool: domain.OpKindMove, Err: inner}
	var taken *domain.PathTaken
	if !errors.Is(err, domain.ErrTitleTaken) || !errors.As(err, &taken) || taken != inner ||
		err.Error() != "ops[3] (move): "+domain.ErrTitleTaken.Error() {
		t.Errorf("OpFailed = %v, want ops[3] (move) wrapping page.title_taken", err)
	}
	changed := domain.BatchChanged()
	if !errors.Is(changed, domain.ErrRevisionMismatch) || !errors.As(changed, &e) || e.Kind != shared.KindConflict ||
		e.Detail != "the notebook changed while the batch was planned, twice: read it again and send the batch again." {
		t.Errorf("BatchChanged = %+v, want page.revision_mismatch saying the notebook changed twice", changed)
	}
	if domain.ErrRevisionMismatch.Detail != "The page was changed since it was read." {
		t.Errorf("BatchChanged set its detail on the shared error: %q", domain.ErrRevisionMismatch.Detail)
	}
}
