// Package page is the module of a notebook's pages: the tree of nodes,
// their content, the changesets and versions of their writes, the edit
// sessions and their lock (v0.1 design 3.5, 3.6, 3.8, 3.9; M4 design; M5
// design). Its root is what bootstrap sees: New for the HTTP side and the
// jobs, and TreeWrites, the attachments' nodes the asset module creates
// and the imports the transfer module writes; NewNotebookDeletion and
// NewNotebookActivity, its parts in the notebook module's deletion and
// activity; NewEditLock, its registrant of its own extension points;
// NewAssetNodes, the asset module's reads of the tree; NewExportNodes, the
// transfer module's; NewStatistics, the statistics an import updates;
// Purgers for the purge; Actions for the composition's checks; MCPEntry,
// the MCP server's notebook entry, AccountInstructions, its account
// entry's instructions, and Tools, the module's MCP tools.
package page

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/http"
	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/markdown"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	riveradapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/river"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Clock tells the time.
type Clock = app.Clock

// Tx runs the write units' transactions: bootstrap hands
// postgres.NewTxManager to it.
type Tx = app.Tx

// Workspaces is what the module reads of workspaces: bootstrap hands
// workspace.NewWorkspaces to it.
type Workspaces = app.Workspaces

// Notebooks is what the module reads of notebooks: bootstrap hands
// notebook.NewNotebooks to it.
type Notebooks = app.Notebooks

// Names reads the accounts' display names: bootstrap hands identity's
// directory to it.
type Names = app.Names

// VisibleNotebooks lists the notebooks an account sees, which a search
// reads: bootstrap hands notebook.NewVisibleNotebooks to it.
type VisibleNotebooks = app.VisibleNotebooks

// AttachmentInfos reads the attachments' type and size, deleted or not:
// bootstrap hands the asset module's to it (M8 design 4.1).
type (
	AttachmentInfos = app.AttachmentInfos
	AttachmentInfo  = app.AttachmentInfo
)

// The module's extension points (M4 design 8): bootstrap composes their
// registrants in registrants.go.
type (
	Write        = app.Write
	Options      = app.Options
	Step         = app.Step
	Change       = domain.Change
	TreeState    = domain.TreeState
	Restored     = domain.Restored
	WriteGuard   = app.WriteGuard
	Participant  = app.Participant
	Appender     = app.Appender
	ContentWrite = app.ContentWrite
	Event        = app.Event
	PageObserver = app.PageObserver
	// The edit sessions' (M4/P4 design 3.7).
	SessionOpening        = app.SessionOpening
	EditSessionVetoer     = app.EditSessionVetoer
	SessionOpened         = app.SessionOpened
	SessionEnded          = app.SessionEnded
	EditSessionSubscriber = app.EditSessionSubscriber
)

// A batch's links (M9/P3 design 3.7): what its plan asks linking of the
// links on its simulated tree, and what its deferred rewrite follows
// besides its changes; bootstrap converts them to linking's, field by
// field.
type (
	// BatchLinks resolves and rewrites links for a batch's plan: bootstrap
	// adapts linking.NewBatchLinks to it.
	BatchLinks = app.BatchLinks
	// LinksOn is a content of a page of a batch's plan to resolve.
	LinksOn = app.LinksOn
	// LinksMoved is a content of a page of a batch's plan to rewrite.
	LinksMoved = app.LinksMoved
	// TreeReads is what the links read of one tree of a batch's plan.
	TreeReads = app.TreeReads
	// PlanTree is the tree of a batch's plan after one of its ops.
	PlanTree = domain.PlanTree
	// NodePath is a node of a PlanTree with its path.
	NodePath = domain.NodePath
	// PathStep is a node on a path from the root.
	PathStep = domain.PathStep
	// LinksBefore is where a page's links led at a moment of a batch.
	LinksBefore = domain.LinksBefore
	// Lead is where a link led at a moment of a batch.
	Lead = domain.Lead
	// BatchRewrite is what a batch's deferred participants follow besides
	// its changes (Step.Batch).
	BatchRewrite = app.BatchRewrite
	// WrittenPage is a page a batch wrote, for its deferred rewrite.
	WrittenPage = app.WrittenPage
)

// BatchTooLarge is page.batch_too_large saying which bound: what, n of
// it, at most max, and what to do. Bootstrap answers linking's
// TooManyRewrites with it.
func BatchTooLarge(what string, n, max int, hint string) error {
	return domain.BatchTooLarge(what, n, max, hint)
}

// ErrTrashChanged is a trash item's restore that found the item changed
// since it read it: an observer answers it for a row of the item the purge
// took (M8/P2 design 3.6).
var ErrTrashChanged = domain.ErrTrashChanged

// ErrChangesetReverted is an import's unit that would merge into the
// changeset of its first, which a revert was made of as it ran (M8/P3
// design 3.7): nothing of the unit was written.
var ErrChangesetReverted = app.ErrChangesetReverted

// SearchQueue is the River queue of the search's job, of one worker: the
// jobs runner's configuration lists it (M8/P5 design 3.4).
const SearchQueue = riveradapter.SearchIndexQueue

// TrashMargin is how long before the purge's retention a trash item can no
// longer be restored: bootstrap tells the instance's retention less it.
const TrashMargin = domain.TrashMargin

// Deps are what bootstrap gives the module.
type Deps struct {
	Pool       *pgxpool.Pool
	Tx         Tx
	Clock      Clock
	Logger     *slog.Logger
	Authorizer shared.Authorizer
	Workspaces Workspaces
	Notebooks  Notebooks
	// Names names a lock's holder and who released one, and who made a
	// version's changeset.
	Names Names
	// Markdown is the one parse and rendering of Markdown, with the
	// registered extensions (M4 design 8).
	Markdown *markdown.Markdown
	// The registrants of the extension points.
	Guards                 []WriteGuard
	Participants           []Participant
	Observers              []PageObserver
	EditSessionVetoers     []EditSessionVetoer
	EditSessionSubscribers []EditSessionSubscriber
	// EditSessionCleanupInterval is how often the expired edit sessions
	// are deleted (page.edit_session_cleanup_interval).
	EditSessionCleanupInterval time.Duration
	// Budget bounds the content parsed and rendered at once: the server's
	// one, which every module that parses shares (M6 design 4.7).
	Budget *markdown.Budget
	// Retention is the purge's, jobs.purge_retention: a trash item can be
	// restored until it, less TrashMargin, after its deletion (M8 design
	// 4.11).
	Retention time.Duration
	// Attachments shows an attachment in the trash: it must be there.
	Attachments AttachmentInfos
	// MaxUnitNodes and MaxUnitHistory bound the nodes of a trash item a
	// request restores or reverts, or of the changeset it reverts, and the
	// rows of their history; MaxRevertSteps a revert's places, deletions
	// and restores: 0 is the domain's, domain.MaxUnitNodes, MaxUnitHistory
	// and MaxRevertSteps, which only a measurement raises (M8/P2 design
	// 3.11, M8/P3 design 3.11); production leaves them 0.
	MaxUnitNodes, MaxUnitHistory, MaxRevertSteps int
	// MaxJobItems bounds the items of a changeset a background revert
	// reverts: 0 is the domain's, domain.MaxJobItems, measured (M8/P4
	// design 3.11); production leaves it 0.
	MaxJobItems int
	// MaxBatchRewrites bounds the pages a batch's deferred rewrite writes:
	// 0 is the domain's, domain.MaxBatchRewrites, which only a measurement
	// raises (M9/P3 design 3.8.2); production leaves it 0.
	MaxBatchRewrites int
	// Snapshots runs a search's reads in one read-only transaction:
	// bootstrap hands it the Tx's manager. VisibleNotebooks are the
	// notebooks a search searches. Both must be there.
	Snapshots        shared.Snapshots
	VisibleNotebooks VisibleNotebooks
	// SearchInterval is how often the search's job runs
	// (page.search_index_interval): 0 registers no job, only in the tests,
	// which run it themselves (NewSearchIndex's Drain).
	SearchInterval time.Duration
	// BatchLinks are linking's links of a batch's plan (M9/P3 design
	// 3.7.4), which MCP's batch resolves and rewrites links with.
	BatchLinks BatchLinks
}

// Module is the wired page module.
type Module struct {
	uc           httpadapter.UseCases
	gate         *app.NotebookGate   // MCP's notebook entry (MCPEntry)
	instructions *app.Instructions   // its handshake's and open_notebook's, with their one cache
	list         *app.ListByPattern  // MCP's list (Tools)
	read         *app.ReadPath       // MCP's read (Tools)
	open         *app.OpenNotebook   // MCP's open_notebook (Tools)
	write        *app.WritePath      // MCP's write (Tools)
	edit         *app.EditPage       // MCP's edit (Tools)
	move         *app.RelocatePath   // MCP's move (Tools)
	remove       *app.DeletePath     // MCP's delete (Tools)
	batch        *app.ApplyBatch     // MCP's batch (Tools)
	versions     *app.ListVersions   // REST's and MCP's versions (Tools)
	version      *app.GetVersion     // REST's and MCP's versions with a revision (Tools)
	log          *app.ListChangesets // REST's and MCP's log (Tools)
	changeset    *app.GetChangeset   // REST's and MCP's changeset (Tools)
	jobs         []jobs.Job
	assets       *app.AssetWrites
	imports      *app.ImportWrites
	jobWrites    *app.JobWrites
}

// New wires the module: every write runs in the one writer's units. A
// module without the attachments' infos, the snapshots or the visible
// notebooks is a fault of its wiring.
func New(d Deps) *Module {
	switch {
	case d.Attachments == nil:
		panic("page: no attachment infos")
	case d.Snapshots == nil:
		panic("page: no snapshots")
	case d.VisibleNotebooks == nil:
		panic("page: no visible notebooks")
	}
	store := postgresadapter.New(d.Pool)
	md := markdownadapter.New(d.Markdown)
	budget := markdownadapter.NewBudget(d.Budget)
	writer := app.NewWriter(app.WriterDeps{
		Tx: d.Tx, Clock: d.Clock, Auth: d.Authorizer, Workspaces: d.Workspaces, Notebooks: d.Notebooks,
		Nodes: store, NodeWriter: store, Trash: store, History: store, Changesets: store, SessionWriter: store, Search: store,
		Names:  d.Names,
		Guards: d.Guards, Participants: d.Participants, Observers: d.Observers,
		SessionVetoers: d.EditSessionVetoers, SessionSubscribers: d.EditSessionSubscribers,
	})
	parser := app.NewContentParser(writer, md, budget, d.Budget.Wait())
	versions := app.VersionReads{Notebooks: d.Notebooks, Nodes: store, Versions: store, Names: d.Names, Auth: d.Authorizer, Clock: d.Clock,
		Paths: store, States: store}
	trash := app.TrashReads{Notebooks: d.Notebooks, Nodes: store, Trash: store, Names: d.Names, Attachments: d.Attachments,
		Auth: d.Authorizer, Clock: d.Clock, Retention: d.Retention}
	history := app.HistoryReads{Notebooks: d.Notebooks, History: store, Names: d.Names, Auth: d.Authorizer, Paths: store}
	locks := app.NewLockHolders(store, d.Names) // edit's precheck (M9/P2 design 3.8.2)
	// The history's reads, REST's and MCP's the same use cases (M9/P2
	// design 3.15).
	listVersions, getVersion := app.NewListVersions(versions), app.NewGetVersion(versions)
	listChangesets, getChangeset := app.NewListChangesets(history), app.NewGetChangeset(history)
	// MCP's batch (M9/P3 design 3.6), its links' on the plan linking's.
	batch := app.NewApplyBatch(writer, parser, store, store, store, d.BatchLinks, locks, d.Notebooks, d.Authorizer, d.MaxBatchRewrites,
		d.Clock, d.Logger)
	m := &Module{uc: httpadapter.UseCases{
		ListNodes:       app.NewListNodes(d.Notebooks, store, d.Authorizer),
		CreatePage:      app.NewCreatePage(writer, store, parser, d.Logger),
		GetPage:         app.NewGetPage(d.Notebooks, store, d.Authorizer),
		GetPageContent:  app.NewGetPageContent(d.Notebooks, store, d.Authorizer),
		PutPageContent:  app.NewPutPageContent(writer, store, parser, d.Logger),
		GetPageView:     app.NewGetPageView(d.Notebooks, store, d.Authorizer, md, budget),
		RenameNode:      app.NewRenameNode(writer, store, d.Logger),
		MoveNode:        app.NewMoveNode(writer, store, d.Logger),
		DeleteNode:      app.NewDeleteNode(writer, store, d.Logger),
		OpenSession:     app.NewOpenEditSession(writer, store, d.Logger),
		Heartbeat:       app.NewHeartbeatEditSession(store, d.Notebooks, d.Authorizer, d.Names, d.Clock),
		EndSession:      app.NewEndEditSession(d.Tx, store, d.Notebooks, d.Clock, d.EditSessionSubscribers, d.Logger),
		GetEditLock:     app.NewGetEditLock(d.Notebooks, store, store, d.Names, d.Authorizer, d.Clock),
		ReleaseEditLock: app.NewReleaseEditLock(writer, store, d.Logger),
		ToggleTask:      app.NewToggleTask(writer, store, parser, md, d.Logger),
		ListVersions:    listVersions,
		GetVersion:      getVersion,
		GetVersionView:  app.NewGetVersionView(versions, md, budget),
		RestoreVersion:  app.NewRestoreVersion(versions, writer, parser, d.Logger),
		ListTrash:       app.NewListTrash(trash),
		GetTrashItem:    app.NewGetTrashItem(trash),
		RestoreNode:     app.NewRestoreNode(trash, writer, parser, app.RestoreBounds{Nodes: d.MaxUnitNodes, History: d.MaxUnitHistory}, d.Logger),
		ListChangesets:  listChangesets,
		GetChangeset:    getChangeset,
		ListChanges:     app.NewListChanges(history),
		RevertChangeset: app.NewRevertChangeset(trash, store, writer, parser,
			app.RevertBounds{Nodes: d.MaxUnitNodes, History: d.MaxUnitHistory, Steps: d.MaxRevertSteps}, d.Logger),
		Search: app.NewSearch(app.SearchDeps{Snapshots: d.Snapshots, Workspaces: d.Workspaces, Visible: d.VisibleNotebooks,
			Auth: d.Authorizer, Reads: store, Trash: store, Attachments: d.Attachments}),
	}, gate: app.NewNotebookGate(d.Notebooks, d.Authorizer), jobs: []jobs.Job{
		riveradapter.CleanupJob(app.NewCleanupEditSessions(store, d.Clock, d.Logger), d.EditSessionCleanupInterval),
	}, assets: app.NewAssetWrites(writer, store), imports: app.NewImportWrites(writer, parser, md),
		jobWrites:    app.NewJobWrites(trash, store, writer, parser, store, d.MaxJobItems, d.MaxUnitHistory),
		list:         app.NewListByPattern(d.Notebooks, store, store, d.Attachments, d.Authorizer),
		read:         app.NewReadPath(d.Notebooks, store, store, d.Authorizer),
		write:        app.NewWritePath(writer, parser, store, store, store, d.Notebooks, d.Authorizer, d.Logger),
		edit:         app.NewEditPage(writer, parser, store, store, store, locks, d.Notebooks, d.Authorizer, d.Clock, d.Logger),
		move:         app.NewRelocatePath(writer, store, d.Notebooks, d.Authorizer, d.Logger),
		remove:       app.NewDeletePath(writer, store, d.Notebooks, d.Authorizer, d.Logger),
		batch:        batch,
		versions:     listVersions,
		version:      getVersion,
		log:          listChangesets,
		changeset:    getChangeset,
		instructions: app.NewInstructions(store, store, md, mcpserver.TextBytes)}
	m.open = app.NewOpenNotebook(m.gate, store, m.instructions)
	if d.SearchInterval > 0 {
		m.jobs = append(m.jobs, riveradapter.SearchIndexJob(
			app.NewIndexSearch(d.Tx, store, d.Clock, d.Logger, domain.MinSearchRebuild), d.SearchInterval))
	}
	return m
}

// Jobs are the module's background jobs, for the server's jobs runner: the
// cleanup of the expired edit sessions, and the search's job when its
// interval is set.
func (m *Module) Jobs() []jobs.Job {
	return m.jobs
}

// Register mounts the module's API on router, the root router from
// httpserver.NewRouter, behind api's per-route middlewares.
func (m *Module) Register(router *httpserver.Router, api *httpserver.API) {
	httpadapter.Register(router, api, m.uc)
}

// BodyLimits are the module's routes whose body may be larger than
// server.max_body_bytes, with their limit: a page's content (M4/P4 design
// 3.8).
func (m *Module) BodyLimits() map[string]int64 {
	return httpadapter.BodyLimits()
}

// Actions are the module's actions: bootstrap checks they are the access
// module's rule table.
func Actions() []shared.Action {
	return domain.Actions()
}
