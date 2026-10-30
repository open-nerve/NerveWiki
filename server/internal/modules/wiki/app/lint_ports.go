package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// LintScope is implicit when Contract is nil, in which case AreaRootID is
// nil too. Explicit standard has no root; explicit LLM requires a root,
// including a present zero UUID. The stored notebook can still be standard.
type LintScope struct {
	NotebookID uuid.UUID
	Contract   *domain.Mode
	AreaRootID *uuid.UUID
}

type LintQuery struct {
	Rule   *string
	Limit  *int
	Cursor *string
}

type NotebookState struct {
	Mode         domain.Mode
	SourceRootID uuid.UUID
}

type Notebooks interface {
	WorkspaceOf(context.Context, uuid.UUID) (uuid.UUID, bool, error)
	StateOf(context.Context, uuid.UUID) (NotebookState, bool, error)
}

type SourceNode struct {
	ID        uuid.UUID
	Kind      domain.NodeKind
	CreatedAt time.Time
	PageBytes int64
}

type CompactNode struct {
	ID        uuid.UUID
	ParentID  *uuid.UUID
	Kind      domain.NodeKind
	NameKey   string
	Linkable  bool
	CreatedAt time.Time
	PageBytes int64
}

type Sources interface {
	Nodes(context.Context, uuid.UUID, uuid.UUID) ([]SourceNode, error)
}

type Nodes interface {
	Nodes(context.Context, uuid.UUID) ([]CompactNode, error)
}

type Paths interface {
	Paths(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]string, error)
}

type ReadInput struct {
	NotebookID    uuid.UUID
	EffectiveMode domain.Mode
	Explicit      bool
	AreaRootID    *uuid.UUID
	Nodes         []CompactNode
	SourceIDs     []uuid.UUID
}

type RuleQuery struct {
	Rule  *domain.LintRule
	After *domain.LintPosition
	Limit int
}

type RuleTiming struct {
	Rule     domain.LintRule
	Duration time.Duration
}

// PreparationTimings separates the single shared sources phase from each
// rule's exclusive work. Nil SharedSources means that phase did not run.
type PreparationTimings struct {
	SharedSources *time.Duration
	Rules         []RuleTiming
}

type PreparedRules interface {
	StalePages() int64
	RuleCount(context.Context, domain.LintRule) (domain.LintCount, error)
	RuleItems(context.Context, domain.LintRule) ([]domain.LintItem, bool, error)
	Timings() PreparationTimings
}

type Rules interface {
	PrepareRules(context.Context, ReadInput, RuleQuery) (PreparedRules, error)
}

// DecodedCursor leaves the pure codec and binding with its module owner.
type DecodedCursor interface {
	PositionFor(LintScope, domain.LintRule) (domain.LintPosition, error)
}

type Cursors interface {
	Decode(string) (DecodedCursor, error)
	Encode(LintScope, domain.LintRule, domain.LintPosition) (string, error)
}

type Clock interface{ Now() time.Time }

type Runner interface {
	Do(context.Context, RunKey, func(context.Context) (RunOutput, error)) (domain.LintResult, error)
}

type LintDeps struct {
	Notebooks  Notebooks
	Sources    Sources
	Nodes      Nodes
	Rules      Rules
	Cursors    Cursors
	Paths      Paths
	Snapshots  shared.Snapshots
	Authorizer shared.Authorizer
	Clock      Clock
	Runs       Runner
}
