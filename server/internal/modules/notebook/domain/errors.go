package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The module's problems (M3 design 5).
var (
	// ErrNotFound: no notebook has the id, it is deleted, or the caller has
	// no role in it.
	ErrNotFound = shared.NewError(shared.KindNotFound, "notebook.not_found", "No such notebook.")
	// ErrWorkspaceNotFound: the workspace module's workspace.not_found, for
	// the operations that name a workspace: no workspace has the slug, it
	// is deleted, or the caller is not an active member of it.
	ErrWorkspaceNotFound = shared.NewError(shared.KindNotFound, "workspace.not_found", "No such workspace.")
)
