package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"uuid"
)

// MaxPayload is the longest payload, in bytes: NOTIFY's, 8000 with the
// terminating zero (M5 design 4.10).
const MaxPayload = 7999

// ErrTooLong is a payload longer than MaxPayload.
var ErrTooLong = errors.New("events: payload too long")

// payload is an Event as NOTIFY carries it.
type payload struct {
	Type        Type            `json:"type"`
	WorkspaceID uuid.UUID       `json:"workspace_id"`
	NotebookID  *uuid.UUID      `json:"notebook_id,omitempty"`
	Data        json.RawMessage `json:"data"`
}

// NewEvent returns the event of type t of the workspace, of the notebook
// when it is not zero, with data, which encodes to a JSON object.
func NewEvent(t Type, workspaceID, notebookID uuid.UUID, data any) (Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("events: %s data: %w", t, err)
	}
	return Event{Type: t, WorkspaceID: workspaceID, NotebookID: notebookID, Data: raw}, nil
}

// Encode returns e's payload: ErrNoType for a type no event can have,
// ErrTooLong when it is longer than MaxPayload, which the publisher sheds
// first.
func Encode(e Event) (string, error) {
	if err := CheckType(e.Type); err != nil {
		return "", err
	}
	p := payload{Type: e.Type, WorkspaceID: e.WorkspaceID, Data: e.Data}
	if e.NotebookID != uuid.Nil() {
		p.NotebookID = &e.NotebookID
	}
	out, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("events: encode %s: %w", e.Type, err)
	}
	if len(out) > MaxPayload {
		return "", fmt.Errorf("%w: %s of %d bytes", ErrTooLong, e.Type, len(out))
	}
	return string(out), nil
}

// Decode reads a payload Encode wrote. A payload it cannot read is an
// error: the listener logs it and drops it.
func Decode(s string) (Event, error) {
	var p payload
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return Event{}, fmt.Errorf("events: decode: %w", err)
	}
	if p.Type == "" || p.WorkspaceID == uuid.Nil() || len(p.Data) == 0 || p.Data[0] != '{' {
		return Event{}, fmt.Errorf("events: decode: not an event: %.80q", s)
	}
	e := Event{Type: p.Type, WorkspaceID: p.WorkspaceID, Data: p.Data}
	if p.NotebookID != nil {
		e.NotebookID = *p.NotebookID
	}
	return e, nil
}
