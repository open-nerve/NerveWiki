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
	// Scope is an upgrade's check's and items' upgrade id, a submission's
	// id (M11 design 4.7): checks of one upgrade join, two upgrades' never,
	// a submission none (it would keep a report of a snapshot older than
	// its freeze). Zero for lint's.
	Scope uuid.UUID
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
// Check is an upgrade's check's or submission's, nil for lint's.
type RunOutput struct {
	Result domain.LintResult
	Check  *domain.UpgradeCheck
	Stats  RunStats
}

// RunResult is a flight's one result, a copy for each caller: lint's
// result, and an upgrade's check, nil for lint's (M11/P2 design 3.8).
type RunResult struct {
	Lint  domain.LintResult
	Check *domain.UpgradeCheck
}

// Logger is the module's log: lint's runs write info lines, an upgrade's
// start info and warn lines (M11/P1 design 3.23).
type Logger interface {
	InfoContext(context.Context, string, ...any)
	WarnContext(context.Context, string, ...any)
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
