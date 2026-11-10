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
	Rollbacks         = app.Rollbacks
	Names             = app.Names
	UpgradeView       = app.UpgradeView
	UpgradeState      = domain.UpgradeState
)

// An upgrade's check (M11/P2 design 3.9, 3.11): the page module's reads,
// which bootstrap converts, and what the check answers.
type (
	UpgradeGatePages = app.UpgradeGatePages
	UpgradeRoots     = domain.UpgradeRoots
	BaselineNode     = domain.BaselineNode
	Preserved        = domain.Preserved
	UpgradeChecked   = app.UpgradeChecked
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
// start's, read's, abandonment's and rollback's end's (M11/P1 design
// 3.15, 3.20): UpgradeAction is the notebook module's
// notebook.change_mode, Rollbacks the transfer module's, StartTimeout
// bounds the start's transaction, Template is the one bootstrap built for
// the creation. Gates are the page module's reads of an upgrade's check,
// ReservedKeys its root's reserved title keys (M11/P2 design 3.11).
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
	RollbackPages RollbackPages
	Writing       WritingJobs
	Rollbacks     Rollbacks
	Names         Names
	UpgradeAction shared.Action
	StartTimeout  time.Duration
	Template      *Template
	Gates         UpgradeGatePages
	ReservedKeys  []string
}

// Module owns one runtime shared by every internal caller. Construction does
// not launch a worker; a flight starts only after Execute authorizes its caller.
type Module struct {
	lint         *app.Lint
	runs         *app.Runs
	playbooks    *app.Playbooks
	startUpgrade *app.StartUpgrade
	checkUpgrade *app.CheckUpgrade
	getUpgrade   *app.GetUpgrade
	abandon      *app.AbandonUpgrade
	endRollback  *app.EndUpgradeRollback
}

func New(d Deps) *Module {
	runs := app.NewRuns(d.MaxLintRuns, d.LintTimeout, d.Logger, d.RunObserver)
	lint := app.NewLint(app.LintDeps{
		Notebooks: d.Notebooks, Sources: d.Sources, Nodes: d.Nodes,
		Rules: d.Rules, Cursors: d.Cursors, Paths: d.Paths,
		Snapshots: d.Snapshots, Authorizer: d.Authorizer, Clock: d.Clock, Runs: runs,
	})
	upgrades := app.UpgradeDeps{
		Tx: d.Tx, Snapshots: d.Snapshots, Notebooks: d.Notebooks, Workspaces: d.Workspaces, States: d.UpgradeStates, Pages: d.Upgrades,
		Rollback: d.RollbackPages, Rows: postgresadapter.New(d.Pool), Writing: d.Writing, Rollbacks: d.Rollbacks, Names: d.Names,
		Authorizer: d.Authorizer, Action: d.UpgradeAction, Template: d.Template, Clock: d.Clock, Logger: d.Logger,
		Timeout: d.StartTimeout, Gates: d.Gates, Cursors: d.Cursors, Paths: d.Paths, ReservedKeys: d.ReservedKeys,
	}
	return &Module{
		lint:         lint,
		runs:         runs,
		playbooks:    app.NewPlaybooks(d.Notebooks, d.Authorizer),
		startUpgrade: app.NewStartUpgrade(upgrades, lint, runs),
		checkUpgrade: app.NewCheckUpgrade(upgrades, lint, runs),
		getUpgrade:   app.NewGetUpgrade(upgrades),
		abandon:      app.NewAbandonUpgrade(upgrades),
		endRollback:  app.NewEndUpgradeRollback(upgrades),
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

// CheckUpgrade checks notebook id's upgrade under way, or reads one rule's
// items in its scope (M11 design 4.7).
func (m *Module) CheckUpgrade(ctx context.Context, id uuid.UUID, query LintQuery) (UpgradeChecked, error) {
	return m.checkUpgrade.Execute(ctx, id, query)
}

// GetUpgrade reads notebook id's upgrade under way, for its admins.
func (m *Module) GetUpgrade(ctx context.Context, id uuid.UUID) (UpgradeView, error) {
	return m.getUpgrade.Execute(ctx, id)
}

// AbandonUpgrade abandons notebook id's upgrade, or continues its stopped
// rollback (M11 design 4.10).
func (m *Module) AbandonUpgrade(ctx context.Context, id uuid.UUID) (UpgradeView, error) {
	return m.abandon.Execute(ctx, id)
}

// EndUpgradeRollback ends the stopped rollback of notebook id's upgrade,
// keeping the notebook as it is (M11 design 4.10).
func (m *Module) EndUpgradeRollback(ctx context.Context, id uuid.UUID) (UpgradeView, error) {
	return m.endRollback.Execute(ctx, id)
}

// Stop closes the same owner used by Execute, including expired workers still
// cleaning up. Repeated calls join that owner's drain.
func (m *Module) Stop(ctx context.Context) error { return m.runs.Stop(ctx) }
