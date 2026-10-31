package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

// RunKey identifies computation, never authorization. Implicit requests
// clear Contract/HasRoot/RootID; a root uses value equality, not its address.
type RunKey struct {
	NotebookID      uuid.UUID
	Explicit        bool
	Contract        domain.Mode
	HasRoot         bool
	RootID          uuid.UUID
	Kind            domain.LintQueryKind
	Rule            domain.LintRule
	CursorCanonical string
	Limit           int
}

type RuleRunStat struct {
	Rule     domain.LintRule
	Duration time.Duration
	Count    domain.LintCount
}

// RunStats contains only observations made by the worker. Nil is unknown,
// not a measured zero. PrepareElapsed is wall time, not a per-rule SQL cost.
type RunStats struct {
	Mode           domain.Mode
	StalePages     *int64
	PrepareElapsed *time.Duration
	SharedSources  *time.Duration
	Rules          []RuleRunStat
}

// RunOutput gives the owner statistics even on a returned error; callers
// receive only a successful independent Result, never a partial response.
type RunOutput struct {
	Result domain.LintResult
	Stats  RunStats
}

type Logger interface {
	InfoContext(context.Context, string, ...any)
}

type RunEventKind uint8

const (
	RunCreated RunEventKind = iota + 1
	RunJoined
	RunAcquired
	RunExpired
	RunClosed
	RunFinished
)

// RunEvent exposes a state transition without caller input or controls.
type RunEvent struct {
	Kind       RunEventKind
	FlightID   uint64
	NotebookID uuid.UUID
}

// RunObserver is optional and called outside the owner's mutex. Observers
// record events; production execution never waits for an observer signal.
type RunObserver func(RunEvent)
