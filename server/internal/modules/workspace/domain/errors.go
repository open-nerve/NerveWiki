package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The module's problems (M2 design 5).
var (
	// ErrNotFound: no workspace has the slug, it is deleted, or the caller
	// is not an active member of it.
	ErrNotFound = shared.NewError(shared.KindNotFound, "workspace.not_found", "No such workspace.")
	// ErrCreationDisabled: workspace.creation_enabled is false; the
	// server's administrator creates them (nervewiki workspaces create).
	ErrCreationDisabled = shared.NewError(shared.KindForbidden, "workspace.creation_disabled",
		"Creating workspaces is disabled on this instance.")
	// ErrSlugTaken: a workspace not deleted has the slug.
	ErrSlugTaken = shared.NewError(shared.KindConflict, "workspace.slug_taken", "The slug is taken.")
)
