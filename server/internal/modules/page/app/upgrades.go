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

// UpgradeRoots are what gates 1 and 2 of an upgrade's check read (M11
// design 4.7): the page at the notebook's root keyed agents, nil for none
// (an attachment is none), whether its body past its frontmatter and a
// byte order mark, trimmed, is not empty; whether the proposed source root
// is a live page of the notebook.
type UpgradeRoots struct {
	AgentsID   *uuid.UUID
	AgentsBody bool
	RootPage   bool
}

// BaselineNode is a baseline row as an upgrade's check reads it (M11 design
// 4.7 gate 6, the report's counts): the node's kind and parent as the
// upgrade started; a page's distinct wanted targets then, 0 for an
// attachment.
type BaselineNode struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
	Kind     domain.Kind
	Wanted   int
}

// Preserved is gate 7 of an upgrade's check (M11 design 4.7): the baseline
// rows' nodes and pages, those alive and those whose content is the
// ledger's, and the first five not kept, by id.
type Preserved struct {
	Nodes, NodesKept, Pages, PagesKept int
	NotKept                            []uuid.UUID
}
