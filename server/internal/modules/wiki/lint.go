package wiki

import (
	"context"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/postgres"
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

// An upgrade's start and read (M11/P1 design 3.14): their ports, which
// bootstrap fills from the other modules, and the view they answer.
type (
	UpgradeWorkspaces = app.UpgradeWorkspaces
	UpgradeStates     = app.UpgradeStates
	UpgradeNotebook   = app.UpgradeNotebook
	NotebookUpgrade   = app.NotebookUpgrade
	ModeChange        = app.ModeChange
	UpgradePages      = app.UpgradePages
	UpgradeBaseline   = app.UpgradeBaseline
	SessionHolder     = app.SessionHolder
	RootNode          = app.RootNode
	WritingJobs       = app.WritingJobs
	Names             = app.Names
	UpgradeView       = app.UpgradeView
	UpgradeState      = domain.UpgradeState
)

// The upgrade's states under way, for bootstrap's conversions.
const (
	UpgradeUpgrading   = domain.UpgradeUpgrading
	UpgradeReviewing   = domain.UpgradeReviewing
	UpgradeRollingBack = domain.UpgradeRollingBack
)

// Deps supplies the runtime's actual read ports and its process-wide bounds.
// RunObserver is nil in production; tests observe transitions without controls.
// Pool is the module's own table's; Tx through StartTimeout an upgrade's
// start's and read's (M11/P1 design 3.15): UpgradeAction is the notebook
// module's notebook.change_mode, StartTimeout bounds the start's
// transaction, Template is the one bootstrap built for the creation.
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

	Pool          *pgxpool.Pool
	Tx            shared.TxManager
	Workspaces    UpgradeWorkspaces
	UpgradeStates UpgradeStates
	Upgrades      UpgradePages
	Writing       WritingJobs
	Names         Names
	UpgradeAction shared.Action
	StartTimeout  time.Duration
	Template      *Template
}

// Module owns one runtime shared by every internal caller. Construction does
// not launch a worker; a flight starts only after Execute authorizes its caller.
type Module struct {
	lint         *app.Lint
	runs         *app.Runs
	playbooks    *app.Playbooks
	startUpgrade *app.StartUpgrade
	getUpgrade   *app.GetUpgrade
}

func New(d Deps) *Module {
	runs := app.NewRuns(d.MaxLintRuns, d.LintTimeout, d.Logger, d.RunObserver)
	lint := app.NewLint(app.LintDeps{
		Notebooks: d.Notebooks, Sources: d.Sources, Nodes: d.Nodes,
		Rules: d.Rules, Cursors: d.Cursors, Paths: d.Paths,
		Snapshots: d.Snapshots, Authorizer: d.Authorizer, Clock: d.Clock, Runs: runs,
	})
	upgrades := app.UpgradeDeps{
		Tx: d.Tx, Notebooks: d.Notebooks, Workspaces: d.Workspaces, States: d.UpgradeStates, Pages: d.Upgrades,
		Rows: postgresadapter.New(d.Pool), Writing: d.Writing, Names: d.Names, Authorizer: d.Authorizer,
		Action: d.UpgradeAction, Template: d.Template, Clock: d.Clock, Logger: d.Logger, Timeout: d.StartTimeout,
	}
	return &Module{
		lint:         lint,
		runs:         runs,
		playbooks:    app.NewPlaybooks(d.Notebooks, d.Authorizer),
		startUpgrade: app.NewStartUpgrade(upgrades, lint, runs),
		getUpgrade:   app.NewGetUpgrade(upgrades),
	}
}

func (m *Module) Execute(ctx context.Context, scope LintScope, query LintQuery) (LintResult, error) {
	return m.lint.Execute(ctx, scope, query)
}

// StartUpgrade starts upgrading the standard notebook id to an LLM Wiki
// from its template in locale, "" for en (M11 design 4.4).
func (m *Module) StartUpgrade(ctx context.Context, id uuid.UUID, locale string) (UpgradeView, error) {
	return m.startUpgrade.Execute(ctx, id, locale)
}

// GetUpgrade reads notebook id's upgrade under way, for its admins.
func (m *Module) GetUpgrade(ctx context.Context, id uuid.UUID) (UpgradeView, error) {
	return m.getUpgrade.Execute(ctx, id)
}

// Stop closes the same owner used by Execute, including expired workers still
// cleaning up. Repeated calls join that owner's drain.
func (m *Module) Stop(ctx context.Context) error { return m.runs.Stop(ctx) }
