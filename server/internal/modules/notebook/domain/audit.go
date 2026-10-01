package domain

import (
	"encoding/json"
	"errors"
	"time"
	"uuid"
)

// AuditAction is what was done with an ownerless notebook (M3 design 4).
type AuditAction string

// The actions.
const (
	AuditTakenOver AuditAction = "taken_over" // a workspace admin took it over
	AuditDeleted   AuditAction = "deleted"    // a workspace admin deleted it
	AuditReturned  AuditAction = "returned"   // its former owner came back to the workspace
)

// AuditEvent is an audit record of a workspace's ownerless notebook. It
// names the notebook as it was then: the record outlives it.
type AuditEvent struct {
	ID            uuid.UUID
	WorkspaceID   uuid.UUID
	NotebookID    uuid.UUID
	NotebookName  string
	Action        AuditAction
	FormerOwnerID uuid.UUID
	ActorID       uuid.UUID // who did it: the former owner for AuditReturned
	At            time.Time
}

// errCursorShape is an audit list cursor that is not [created_at, id].
var errCursorShape = errors.New("the cursor is not [created_at, id]")

// AuditCursor is the payload of the audit list's cursor (M3/P3 design
// 3.4): the last row's created_at and id, the list's order, newest first.
type AuditCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// MarshalJSON writes the cursor as [created_at, id], created_at in UTC, so
// each position has exactly one spelling.
func (c AuditCursor) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]string{c.CreatedAt.UTC().Format(time.RFC3339Nano), c.ID.String()})
}

// UnmarshalJSON reads an array of exactly two strings, an RFC 3339 time and
// a uuid. It also reads spellings of them that MarshalJSON never writes,
// such as the time at an offset instead of in UTC, or upper-case hex;
// shared.DecodeCursor refuses those.
func (c *AuditCursor) UnmarshalJSON(b []byte) error {
	var parts []string
	if err := json.Unmarshal(b, &parts); err != nil {
		return err
	}
	if len(parts) != 2 {
		return errCursorShape
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return err
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return err
	}
	*c = AuditCursor{CreatedAt: at, ID: id}
	return nil
}
