package domain

import (
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Member is an account's membership of a workspace.
type Member struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	UserID      uuid.UUID
	Role        shared.WorkspaceRole
}
