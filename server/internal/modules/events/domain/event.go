// Package domain holds the event stream's events, their payloads and the
// frames a stream writes (M5 design 4.10).
package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"uuid"
)

// Type is an event's type. A stream passes on a type it does not know:
// another M adds its own (M5 design 8).
type Type string

// ErrNoType is a type no event can have: empty, with a colon or a line
// break, which a frame's event field cannot hold, or one of the stream's
// own frames, hello and reset.
var ErrNoType = errors.New("events: no event type")

// CheckType returns ErrNoType for a type no event can have.
func CheckType(t Type) error {
	if t == "" || t == "hello" || t == "reset" || strings.ContainsAny(string(t), "\r\n:") {
		return fmt.Errorf("%w: %q", ErrNoType, t)
	}
	return nil
}

// The types of M5.
const (
	TypePages            Type = "pages"
	TypeLock             Type = "lock"
	TypeAccess           Type = "access"
	TypeNotebooksDeleted Type = "notebooks_deleted"
)

// Event is what a write publishes: its type, the workspace it is of, the
// notebook when it is of one (zero: of the whole workspace), and its data,
// a JSON object of ids. A stream gets an event of a notebook when it sees
// the notebook, and one of a workspace when it sees the workspace.
type Event struct {
	Type        Type
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	Data        json.RawMessage
}

// MaxPages is how many pages a pages event lists at most: more, and it
// lists none, and the client takes every page as written.
const MaxPages = 20

// Pages is the data of a pages event, one a write unit: whether it changed
// the tree (a page created, renamed, moved or deleted), and the pages
// whose content it wrote with their new revisions. Pages is empty when it
// wrote no content, and nil, null in JSON, when it wrote more than
// MaxPages.
type Pages struct {
	Tree  bool           `json:"tree"`
	Pages []PageRevision `json:"pages"`
}

// PageRevision is a page whose content was written, and its revision.
type PageRevision struct {
	ID       uuid.UUID `json:"id"`
	Revision int       `json:"revision"`
}

// Lock is the data of a lock event: an edit session of the page opened or
// ended. Its id keeps the end and the opening of a take-over, in one
// transaction, from being folded into one notification.
type Lock struct {
	PageID    uuid.UUID `json:"page_id"`
	SessionID uuid.UUID `json:"session_id"`
}

// Access is the data of an access event: who may see other notebooks of
// the workspace now, UserIDs, and whether every member might, Reached.
// UserIDs is nil, null in JSON, when they are too many for a payload:
// every member might, then.
type Access struct {
	UserIDs []uuid.UUID `json:"user_ids"`
	Reached bool        `json:"reached"`
}

// Everyone reports whether the change may reach every member of the
// workspace.
func (a Access) Everyone() bool { return a.Reached || a.UserIDs == nil }

// NotebooksDeleted is the data of a notebooks_deleted event: the notebooks
// of the workspace deleted. NotebookIDs is nil, null in JSON, when they
// are too many for a payload: any of the workspace's, then.
type NotebooksDeleted struct {
	NotebookIDs []uuid.UUID `json:"notebook_ids"`
}
