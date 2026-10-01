package domain

import (
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Member is an account's membership of a notebook.
type Member struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	UserID     uuid.UUID
	Role       shared.NotebookRole
	CreatedAt  time.Time // when the account first joined
}
