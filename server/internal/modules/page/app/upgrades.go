package app

import (
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// Baseline is what an upgrade's start recorded (M11 design 4.4 step 11):
// its nodes and, of them, its pages.
type Baseline struct {
	Nodes int
	Pages int
}

// SessionHolder is an edit session alive in a notebook: its page and its
// editor (M11 design 4.4 step 7).
type SessionHolder struct {
	PageID uuid.UUID
	UserID uuid.UUID
}

// KeyedRootNode is a node at a notebook's root by its title key (M11 design
// 4.4 step 8).
type KeyedRootNode struct {
	ID   uuid.UUID
	Kind domain.Kind
	Key  string
}
