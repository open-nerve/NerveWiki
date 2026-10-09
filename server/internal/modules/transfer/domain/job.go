package domain

import (
	"time"
	"uuid"
)

// Kind is what a job does.
type Kind string

// The two kinds of job.
const (
	KindImport Kind = "import"
	KindExport Kind = "export"
)

// State is where a job is (M7/P5 design 3.5): queued, then running, then
// one of the four ends; an export that succeeded expires. The statements
// that move a job name the state they move it from.
type State string

// The states of a job.
const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
	StateExpired   State = "expired"
)

// Ended reports whether a job in s has ended: it has its report.
func (s State) Ended() bool {
	return s != StateQueued && s != StateRunning
}

// Client is where a job was started from: a sign-in session's token is the
// web's, a personal access token the API's. A job's writes are its (P6).
type Client string

// The clients a job is started from.
const (
	ClientWeb Client = "web"
	ClientAPI Client = "api"
)

// Progress is how many of a job's nodes are done, of all.
type Progress struct {
	Done, Total int64
}

// Job is a job's row.
type Job struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	// RootID is the page exported with its subtree; nil for the whole
	// notebook.
	RootID *uuid.UUID
	Kind   Kind
	State  State
	// Name is what is exported, by name: the notebook's or the page's.
	Name      string
	CreatedBy uuid.UUID
	Client    Client
	Progress  Progress
	// CancelRequested is when a cancel of the running job was asked.
	CancelRequested *time.Time
	Heartbeat       *time.Time
	Started         *time.Time
	Finished        *time.Time
	// Report is the job's, once it has ended.
	Report *Report
	// ResultBytes is an export's archive's size, once it succeeded.
	ResultBytes *int64
	CreatedAt   time.Time
}

// Archive is the key of a job's archive in the store: exports/<id>.zip
// (M7 design 4.1).
func Archive(kind Kind, id uuid.UUID) string {
	return string(kind) + "s/" + id.String() + ".zip"
}
