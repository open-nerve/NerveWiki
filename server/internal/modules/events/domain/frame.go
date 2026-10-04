package domain

import (
	"encoding/json"
	"fmt"
	"time"
	"uuid"
)

// ResetReason is why a stream ends with a reset frame. The client
// reconnects at once and refreshes all it shows (M5 design 4.10).
type ResetReason string

// The reasons of a reset.
const (
	// ResetAccess: the account may see other notebooks now.
	ResetAccess ResetReason = "access"
	// ResetNotebooksDeleted: a notebook the stream sees is deleted.
	ResetNotebooksDeleted ResetReason = "notebooks_deleted"
	// ResetExpired: the stream's credential expired.
	ResetExpired ResetReason = "expired"
	// ResetUnauthenticated: the credential failed at a heartbeat: revoked,
	// signed out, or its account deactivated.
	ResetUnauthenticated ResetReason = "unauthenticated"
	// ResetReconnected: the server's listener lost its connection and has
	// a new one; what was sent meanwhile is lost.
	ResetReconnected ResetReason = "reconnected"
	// ResetOverflow: the stream's client fell behind.
	ResetOverflow ResetReason = "overflow"
)

// HelloFrame is a stream's first frame: how often the server sends a
// heartbeat, in whole seconds, from which the client derives its waits.
func HelloFrame(heartbeat time.Duration) []byte {
	return frame("hello", fmt.Appendf(nil, `{"heartbeat_seconds":%d}`, int(heartbeat/time.Second)))
}

// EventFrame is e as a frame: its event field is e's type, its data e's
// data with the workspace's id, and the notebook's when e is of one.
func EventFrame(e Event) ([]byte, error) {
	if err := CheckType(e.Type); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(e.Data, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("events: %s data is no JSON object: %w", e.Type, err)
	}
	fields["workspace_id"] = idJSON(e.WorkspaceID)
	if e.NotebookID != uuid.Nil() {
		fields["notebook_id"] = idJSON(e.NotebookID)
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("events: %s frame: %w", e.Type, err)
	}
	return frame(string(e.Type), data), nil
}

// ResetFrame is the frame a stream ends with, saying why.
func ResetFrame(reason ResetReason) []byte {
	return frame("reset", fmt.Appendf(nil, `{"reason":%q}`, reason))
}

// HeartbeatFrame is a comment line: it keeps proxies and the client from
// taking a quiet stream for a dead one.
func HeartbeatFrame() []byte {
	return []byte(": heartbeat\n\n")
}

// frame is one server-sent event; data holds no line break.
func frame(event string, data []byte) []byte {
	out := make([]byte, 0, len(event)+len(data)+16)
	out = append(out, "event: "...)
	out = append(out, event...)
	out = append(out, "\ndata: "...)
	out = append(out, data...)
	return append(out, "\n\n"...)
}

func idJSON(id uuid.UUID) json.RawMessage {
	return json.RawMessage(`"` + id.String() + `"`)
}
