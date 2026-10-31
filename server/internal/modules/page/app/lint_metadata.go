package app

import (
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// LintNode is compact live metadata, not a display name or an ancestor path.
// PageBytes is meaningful only for a page; attachment bytes belong to asset.
type LintNode struct {
	ID        uuid.UUID
	ParentID  *uuid.UUID
	Kind      domain.Kind
	NameKey   string
	Linkable  bool
	CreatedAt time.Time
	PageBytes int64
}

// SourceNode is one strict live descendant of a proposed source-area root.
// A source page may be empty and need not have a source hash.
type SourceNode struct {
	ID        uuid.UUID
	Kind      domain.Kind
	CreatedAt time.Time
	PageBytes int64
}

// SourceData contains one read's complete live metadata and strict source area.
// It belongs to this call; it is never cached across snapshots or notebooks.
type SourceData struct {
	Sources []SourceNode
	Nodes   []LintNode
}
