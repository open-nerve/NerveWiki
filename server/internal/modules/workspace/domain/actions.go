package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The module's actions: each has a row in the access module's rule table
// (bootstrap's actions test).
const (
	ActionRead shared.Action = "workspace.read"
)

// Actions lists the module's actions.
func Actions() []shared.Action {
	return []shared.Action{ActionRead}
}
