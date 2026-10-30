package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// CompactNode is the page module's live metadata converted at the composition
// boundary. It contains no display name, body, or candidate ancestor path.
type CompactNode struct {
	ID        uuid.UUID
	ParentID  *uuid.UUID
	Kind      domain.NodeKind
	NameKey   string
	Linkable  bool
	CreatedAt time.Time
	PageBytes int64
}

// ReadInput belongs to one caller-owned snapshot. SourceIDs are the complete
// strict live descendants returned by SourceArea, even when there are >10k.
// Their metadata is in Nodes. An explicit LLM contract may select an area root
// even when the stored notebook mode is standard.
type ReadInput struct {
	NotebookID    uuid.UUID
	EffectiveMode domain.LintMode
	Explicit      bool
	AreaRootID    *uuid.UUID
	Nodes         []CompactNode
	SourceIDs     []uuid.UUID
}

// RuleQuery binds a preparation to a summary or one rule's requested page.
// Summary has nil Rule/After and zero Limit. A rule's zero Limit means 50;
// explicit limits are 1..100. A validated Begin position means the first item.
type RuleQuery struct {
	Rule  *domain.Rule
	After *domain.Position
	Limit int
}

// PageRuleInput contains complete sorted, unique eligibility relations. Only
// orphans consumes ExcludedEmitters. The adapter rechecks notebook/current.
type PageRuleInput struct {
	NotebookID       uuid.UUID
	CandidateIDs     []uuid.UUID
	ExcludedEmitters []uuid.UUID
}

// PendingAssetInput gives F every nonempty source page, including stale or
// unindexed pages. Its real SQL current join establishes the parent relation;
// the app's pending-page set independently requires a current indexed head.
type PendingAssetInput struct {
	NotebookID       uuid.UUID
	ParentCandidates []uuid.UUID
}

// LintRuleStore has named static queries rather than rule-driven SQL. Counts
// apply qualification before their 10000 cap. Items use a nil first position,
// allow the zero UUID as a real position, and return at most take IDs in order.
// No Rows escape. Every statement uses the caller's snapshot and Planned.
type LintRuleStore interface {
	LintOrphanCount(context.Context, PageRuleInput) (domain.Count, error)
	LintOrphanItems(context.Context, PageRuleInput, *uuid.UUID, int) ([]uuid.UUID, error)
	LintFrontmatterCount(context.Context, PageRuleInput) (domain.Count, error)
	LintFrontmatterItems(context.Context, PageRuleInput, *uuid.UUID, int) ([]uuid.UUID, error)
	LintMissingTypeCount(context.Context, PageRuleInput) (domain.Count, error)
	LintMissingTypeItems(context.Context, PageRuleInput, *uuid.UUID, int) ([]uuid.UUID, error)
	LintMissingSummaryCount(context.Context, PageRuleInput) (domain.Count, error)
	LintMissingSummaryItems(context.Context, PageRuleInput, *uuid.UUID, int) ([]uuid.UUID, error)
	// LintPendingAssets returns the complete subset of the <=512 sorted unique
	// candidate asset IDs that have no qualifying body parent in F. The caller
	// has already excluded directly digested assets; all parent candidates are
	// passed again in each batch, and that work is part of the query's cost.
	LintPendingAssets(context.Context, PendingAssetInput, []uuid.UUID) ([]uuid.UUID, error)
}

// AssetSizes is adapted from the asset module's Infos at the composition root.
// Prepare calls it once with the unique final returned asset IDs, or not at
// all for an empty set. Missing entries and negative byte sizes are errors.
type AssetSizes interface {
	BytesOf(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]int64, error)
}
