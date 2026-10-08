// Package domain holds the asset module's rules (M7 design 4.2–4.5; M7/P2
// design 3.4–3.6): an attachment's file, the type the server serves it
// as, and the actions on attachments.
package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The module's actions, decided on a notebook: writers upload, every role
// reads.
const (
	ActionUpload shared.Action = "asset.upload"
	ActionRead   shared.Action = "asset.read"
)

// Actions are the module's actions, which the access module's rule table
// lists (bootstrap's actions test).
func Actions() []shared.Action {
	return []shared.Action{ActionUpload, ActionRead}
}
