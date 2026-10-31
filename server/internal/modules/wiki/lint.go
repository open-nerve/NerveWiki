package wiki

import (
	"context"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The module root exposes only wiki-owned values and ports. Bootstrap converts
// other modules' projections without importing their app or domain packages.
type (
	LintRule           = domain.LintRule
	LintCount          = domain.LintCount
	LintQueryKind      = domain.LintQueryKind
	LintPosition       = domain.LintPosition
	LintItem           = domain.LintItem
	LintRuleResult     = domain.LintRuleResult
	LintResult         = domain.LintResult
	NodeKind           = domain.NodeKind
	LintScope          = app.LintScope
	LintQuery          = app.LintQuery
	NotebookState      = app.NotebookState
	Notebooks          = app.Notebooks
	SourceNode         = app.SourceNode
	SourceData         = app.SourceData
	Sources            = app.Sources
	CompactNode        = app.CompactNode
	Nodes              = app.Nodes
	Paths              = app.Paths
	ReadInput          = app.ReadInput
	RuleQuery          = app.RuleQuery
	RuleTiming         = app.RuleTiming
	PreparationTimings = app.PreparationTimings
	PreparedRules      = app.PreparedRules
	Rules              = app.Rules
	DecodedCursor      = app.DecodedCursor
	Cursors            = app.Cursors
	Clock              = app.Clock
	Logger             = app.Logger
	RunObserver        = app.RunObserver
	RunEvent           = app.RunEvent
	RunEventKind       = app.RunEventKind
)

const (
	RuleWanted             = domain.RuleWanted
	RuleOrphans            = domain.RuleOrphans
	RuleAmbiguous          = domain.RuleAmbiguous
	RuleInvalidFrontmatter = domain.RuleInvalidFrontmatter
	RuleMissingType        = domain.RuleMissingType
	RuleMissingSummary     = domain.RuleMissingSummary
	RuleUndigestedSources  = domain.RuleUndigestedSources
	RuleInvalidSources     = domain.RuleInvalidSources
	NodePage               = domain.NodePage
	NodeAsset              = domain.NodeAsset
	LintSummary            = domain.LintSummary
	LintItems              = domain.LintItems
	ActionLintRead         = domain.ActionLintRead
	RunCreated             = app.RunCreated
	RunJoined              = app.RunJoined
	RunAcquired            = app.RunAcquired
	RunExpired             = app.RunExpired
	RunClosed              = app.RunClosed
	RunFinished            = app.RunFinished
)

// Deps supplies the runtime's actual read ports and its process-wide bounds.
// RunObserver is nil in production; tests observe transitions without controls.
type Deps struct {
	Notebooks   Notebooks
	Sources     Sources
	Nodes       Nodes
	Rules       Rules
	Cursors     Cursors
	Paths       Paths
	Snapshots   shared.Snapshots
	Authorizer  shared.Authorizer
	Clock       Clock
	Logger      Logger
	MaxLintRuns int
	LintTimeout time.Duration
	RunObserver RunObserver
}

// Module owns one runtime shared by every internal caller. Construction does
// not launch a worker; a flight starts only after Execute authorizes its caller.
type Module struct {
	lint *app.Lint
	runs *app.Runs
}

func New(d Deps) *Module {
	runs := app.NewRuns(d.MaxLintRuns, d.LintTimeout, d.Logger, d.RunObserver)
	return &Module{
		lint: app.NewLint(app.LintDeps{
			Notebooks: d.Notebooks, Sources: d.Sources, Nodes: d.Nodes,
			Rules: d.Rules, Cursors: d.Cursors, Paths: d.Paths,
			Snapshots: d.Snapshots, Authorizer: d.Authorizer, Clock: d.Clock, Runs: runs,
		}),
		runs: runs,
	}
}

func (m *Module) Execute(ctx context.Context, scope LintScope, query LintQuery) (LintResult, error) {
	return m.lint.Execute(ctx, scope, query)
}

// Stop closes the same owner used by Execute, including expired workers still
// cleaning up. Repeated calls join that owner's drain.
func (m *Module) Stop(ctx context.Context) error { return m.runs.Stop(ctx) }
