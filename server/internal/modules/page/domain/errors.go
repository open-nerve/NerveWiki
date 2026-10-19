package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	// ErrWorkspaceNotFound: the workspace module's workspace.not_found, for
	// the operations that name a workspace, a search: no workspace has the
	// slug, it is deleted, or the caller is not an active member of it.
	ErrWorkspaceNotFound = shared.NewError(shared.KindNotFound, "workspace.not_found", "No such workspace.")
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
	// ErrVersionNotFound: no version of the page holds the revision (M8
	// design 4.3).
	ErrVersionNotFound = shared.NewError(shared.KindNotFound, "page.version_not_found", "No such version of the page.")
	// ErrNotInTrash: the node a trash read or a restore names is alive (M8
	// design 4.6).
	ErrNotInTrash = shared.NewError(shared.KindConflict, "page.not_in_trash", "The node is not in the trash.")
	// ErrNotTrashRoot: the node is in the trash with what it was deleted
	// with, the trash item of another root. NotTrashRoot returns it with the
	// root_id member.
	ErrNotTrashRoot = shared.NewError(shared.KindConflict, "page.not_trash_root",
		"The node is in the trash as part of another item: its root is restored with it.")
	// ErrRestoreExpired: the trash item is past the time it can be restored
	// in, its deletion's time and the retention less TrashMargin (M8 design
	// 4.11).
	ErrRestoreExpired = shared.NewError(shared.KindConflict, "page.restore_expired",
		"The trash item can no longer be restored.")
	// ErrRestoreTooLarge: the trash item holds more than one unit restores
	// (M8 design 4.9).
	ErrRestoreTooLarge = shared.NewError(shared.KindInvalid, "page.restore_too_large",
		"The trash item is too large to restore in one request.")
	// ErrTrashChanged: the trash item changed since the restore read it,
	// restored meanwhile or purged in part (M8/P2 design 3.5).
	ErrTrashChanged = shared.NewError(shared.KindConflict, "page.trash_changed",
		"The trash item changed since it was read.")
	// ErrChangesetNotFound: no changeset has the id, its notebook is
	// deleted, or the caller has no role in it (M8 design 4.7).
	ErrChangesetNotFound = shared.NewError(shared.KindNotFound, "page.changeset_not_found", "No such changeset.")
	// ErrChangesetTrimmed: the purge took part of what the changeset
	// changed, the history of nodes deleted past the retention: it can no
	// longer be reverted whole (M8 design 4.2, 4.8).
	ErrChangesetTrimmed = shared.NewError(shared.KindConflict, "page.changeset_trimmed",
		"The purge took part of what the changeset changed: it can no longer be reverted whole.")
	// ErrRevertTooLarge: the changeset changed more than one unit reverts
	// (M8 design 4.9).
	ErrRevertTooLarge = shared.NewError(shared.KindInvalid, "page.revert_too_large",
		"The changeset is too large to revert in one request.")
	// ErrRevertConflict: nodes the changeset changed changed since, or
	// cannot go back as they were (M8 design 4.8). RevertConflicts returns
	// it with the conflicts members.
	ErrRevertConflict = shared.NewError(shared.KindConflict, "page.revert_conflict",
		"Nodes the changeset changed changed since, or cannot go back as they were.")
)

// ErrJobBatchTooLarge is a background job's batch of more than one step past
// what a unit writes besides its nodes (M8/P4 design 3.3): the rows of
// history it restores, counted before its contents are parsed, or the
// links they hold, once they are. No problem of the API: the job halves
// the batch and tries again; a lone step never is.
var ErrJobBatchTooLarge = errors.New("page: a background job's batch is past the unit's history or links")

// RevertConflicts is ErrRevertConflict naming the conflicts, the first
// MaxConflicts of count.
func RevertConflicts(conflicts []RevertConflict, count int) error {
	e := *ErrRevertConflict
	e.Conflicts = make([]shared.Conflict, len(conflicts))
	for i, c := range conflicts {
		e.Conflicts[i] = shared.Conflict{NodeID: c.NodeID, Name: c.Name, Reason: string(c.Reason), HolderID: c.HolderID}
	}
	e.ConflictCount = &count
	return &e
}

// NotTrashRoot is ErrNotTrashRoot naming the root of the trash item the
// node is of.
func NotTrashRoot(rootID uuid.UUID) error {
	e := *ErrNotTrashRoot
	e.RootID = &rootID
	return &e
}

// Locked is ErrLocked naming the lock: the page locked and its holder
// (M5 design 4.5).
func Locked(pageID, userID uuid.UUID, displayName string) error {
	e := *ErrLocked
	e.Lock = &shared.LockHolder{PageID: pageID, UserID: userID, DisplayName: displayName}
	return &e
}

// LockedAll is ErrLocked naming each of holders, one at least, the first
// as Lock too: a delete over MCP that edit sessions in its subtree hold
// (M9/P2 design 3.6), in the order of what the delete changes.
func LockedAll(holders []shared.LockHolder) error {
	e := *ErrLocked
	first := holders[0]
	e.Lock, e.Locks = &first, holders
	return &e
}

// PathTaken is page.title_taken at a path: a node holds the name the path
// asks for, a page's or an attachment's (Asset).
type PathTaken struct {
	Path  string
	Asset bool
}

func (e *PathTaken) Error() string { return ErrTitleTaken.Error() }
func (e *PathTaken) Unwrap() error { return ErrTitleTaken }

// Mismatch is why a version token is not the page's at a path.
type Mismatch int

// The reasons.
const (
	OtherNode    Mismatch = iota + 1 // the path names another page than the token's
	Changed                          // the page's content was written since the token's revision
	KeptChanging                     // it was written again while an edit was made the second time
)

// VersionMismatch is page.revision_mismatch at a path: why, and the
// page's version there now.
type VersionMismatch struct {
	Path    string
	Current VersionToken
	Why     Mismatch
}

func (e *VersionMismatch) Error() string { return ErrRevisionMismatch.Error() }
func (e *VersionMismatch) Unwrap() error { return ErrRevisionMismatch }

// LockPaths is a lock's refusal, page.locked or linking.pages_locked, with
// the paths of the first MaxNamedLocks pages it names, by page id: a page
// gone since has none, and none has one when they could not be read.
type LockPaths struct {
	Err   error
	Paths map[uuid.UUID]string
}

func (e *LockPaths) Error() string { return e.Err.Error() }
func (e *LockPaths) Unwrap() error { return e.Err }

// MaxNamedLocks is how many of a refusal's locks have their paths read:
// mcpserver.MaxErrorLocks, which page's adapter/mcp holds it to.
const MaxNamedLocks = 20

// AttachmentOnPath is a path's segment, from 1, that names an attachment
// where the path goes on under it: no problem of its own, since what it
// is depends on the field the path is (M9/P2 design 3.7).
type AttachmentOnPath struct{ Segment int }

func (e *AttachmentOnPath) Error() string {
	return fmt.Sprintf("page: segment %d of the path is an attachment", e.Segment)
}

// Recursive is 422 on recursive: a page with Pages pages and Assets
// assets under it is deleted only with recursive.
type Recursive struct{ Pages, Assets int }

func (e *Recursive) Error() string { return e.Unwrap().Error() }

// Unwrap is the 422 itself. Its message counts the nodes and names no
// path (M9/P2 design 3.10.3).
func (e *Recursive) Unwrap() error {
	return shared.Invalid(shared.FieldError{Field: "recursive", Code: shared.FieldRequired,
		Message: "must be true to delete a page with children: " + PagesAndAttachments(e.Pages, e.Assets)})
}

// PagesAndAttachments is pages and assets counted in words: "3 pages and
// 2 attachments", "1 page", "2 attachments"; a count of none is left out.
func PagesAndAttachments(pages, assets int) string {
	var parts []string
	if pages > 0 {
		parts = append(parts, counted(pages, "page"))
	}
	if assets > 0 {
		parts = append(parts, counted(assets, "attachment"))
	}
	return strings.Join(parts, " and ")
}

// counted is n of what, plural but for one.
func counted(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return strconv.Itoa(n) + " " + what + "s"
}

// Unlocked is ErrEditSessionUnlocked naming who released the lock.
func Unlocked(userID uuid.UUID, displayName string) error {
	e := *ErrEditSessionUnlocked
	e.EndedBy = &shared.Person{UserID: userID, DisplayName: displayName}
	return &e
}

// NotATask is 422 on offset: no task item of the content has its character
// there (M5/P6 design 3.3).
func NotATask() error {
	return shared.Invalid(shared.FieldError{Field: "offset", Code: shared.FieldOutOfRange,
		Message: "is not a task item's position in the content at base_revision"})
}

// TaskWouldGo is 422 on offset: ticking or clearing the task item there
// would leave none, as ticking "- [ ]: /u" makes a link reference
// definition (M5/P6 design 3.3).
func TaskWouldGo() error {
	return shared.Invalid(shared.FieldError{Field: "offset", Code: shared.FieldNotAllowed,
		Message: "ticking or clearing the task item there would leave none"})
}

// NotAllowed is 422 on field: a parent or a sibling the write names is no
// node of the notebook, or not where the write says.
func NotAllowed(field, message string) error {
	return shared.Invalid(shared.FieldError{Field: field, Code: shared.FieldNotAllowed, Message: message})
}
