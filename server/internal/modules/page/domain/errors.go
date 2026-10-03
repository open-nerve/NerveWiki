package domain

import (
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The module's problems (M4 design 5; M4/P1 design 3.5).
var (
	// ErrNotFound: no node has the id, it is deleted, or the caller has no
	// role in its notebook.
	ErrNotFound = shared.NewError(shared.KindNotFound, "page.not_found", "No such page.")
	// ErrNotebookNotFound: the notebook module's notebook.not_found, for
	// the operations that name a notebook: no notebook has the id, it is
	// deleted, or the caller has no role in it.
	ErrNotebookNotFound = shared.NewError(shared.KindNotFound, "notebook.not_found", "No such notebook.")
	// ErrTitleTaken: a node under the same parent has a title of the same
	// key.
	ErrTitleTaken = shared.NewError(shared.KindConflict, "page.title_taken",
		"A page or attachment under the same parent has this title.")
	// ErrCycle: the page would move under itself or one of its descendants.
	ErrCycle = shared.NewError(shared.KindConflict, "page.cycle", "A page cannot move under itself or one of its subpages.")
	// ErrTooDeep: the page would be deeper than MaxDepth.
	ErrTooDeep = shared.NewError(shared.KindConflict, "page.too_deep", "Pages nest at most 10 levels deep.")
	// ErrRevisionMismatch: the write's base_revision is not the content's
	// revision: someone wrote it since the writer read it.
	ErrRevisionMismatch = shared.NewError(shared.KindConflict, "page.revision_mismatch",
		"The page was changed since it was read.")
	// ErrEditSessionEnded: the edit session a content write names is not
	// the writer's alive session of the page: there is none, it expired, or
	// it is someone else's, another page's, or another client's (the web or
	// a token) of the writer.
	ErrEditSessionEnded = shared.NewError(shared.KindConflict, "page.edit_session_ended", "The edit session has ended.")
	// ErrEditSessionNotFound: no alive edit session of the caller has the
	// id, or the caller may see its notebook no more.
	ErrEditSessionNotFound = shared.NewError(shared.KindNotFound, "page.edit_session_not_found", "No such edit session.")
	// ErrLocked: another edit session holds the page's lock (M5 design
	// 4.4). Locked returns it with the lock member.
	ErrLocked = shared.NewError(shared.KindConflict, "page.locked", "Another edit session holds the page's lock.")
	// ErrEditSessionTakenOver: the caller's edit session was taken over by
	// their own session elsewhere (M5 design 4.3).
	ErrEditSessionTakenOver = shared.NewError(shared.KindConflict, "page.edit_session_taken_over",
		"The edit session was taken over elsewhere.")
	// ErrEditSessionUnlocked: an admin of the notebook released the page's
	// lock, ending the caller's edit session. Unlocked returns it with the
	// ended_by member.
	ErrEditSessionUnlocked = shared.NewError(shared.KindConflict, "page.edit_session_unlocked",
		"An admin of the notebook released the page's lock.")
)

// Locked is ErrLocked naming the lock: the page locked and its holder
// (M5 design 4.5).
func Locked(pageID, userID uuid.UUID, displayName string) error {
	e := *ErrLocked
	e.Lock = &shared.LockHolder{PageID: pageID, UserID: userID, DisplayName: displayName}
	return &e
}

// Unlocked is ErrEditSessionUnlocked naming who released the lock.
func Unlocked(userID uuid.UUID, displayName string) error {
	e := *ErrEditSessionUnlocked
	e.EndedBy = &shared.Person{UserID: userID, DisplayName: displayName}
	return &e
}

// NotAllowed is 422 on field: a parent or a sibling the write names is no
// node of the notebook, or not where the write says.
func NotAllowed(field, message string) error {
	return shared.Invalid(shared.FieldError{Field: field, Code: shared.FieldNotAllowed, Message: message})
}
