// Package domain holds the transfer module's rules (M7 design 4.9–4.12;
// M7/P5 design 3.5–3.10): the jobs and their states, the actions on them,
// the reports, and where an export puts each node of a notebook in its
// archive.
package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The module's actions, decided on a notebook: an export is a read, so
// every role exports, reads its jobs and cancels them; the use cases keep
// another's jobs to the notebook's admins.
const (
	ActionExport shared.Action = "transfer.export"
	ActionRead   shared.Action = "transfer.read"
	ActionCancel shared.Action = "transfer.cancel"
)

// Actions are the module's actions, which the access module's rule table
// lists (bootstrap's actions test).
func Actions() []shared.Action {
	return []shared.Action{ActionExport, ActionRead, ActionCancel}
}
