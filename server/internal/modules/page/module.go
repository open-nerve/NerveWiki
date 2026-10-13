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
// Purgers for the purge; Actions for the composition's checks.
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

// ErrTrashChanged is a trash item's restore that found the item changed
// since it read it: an observer answers it for a row of the item the purge
// took (M8/P2 design 3.6).
var ErrTrashChanged = domain.ErrTrashChanged

// ErrChangesetReverted is an import's unit that would merge into the
// changeset of its first, which a revert was made of as it ran (M8/P3
// design 3.7): nothing of the unit was written.
var ErrChangesetReverted = app.ErrChangesetReverted

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
	// request restores and the rows of their history: 0 is the domain's,
	// domain.MaxUnitNodes and MaxUnitHistory, which only a measurement
	// raises (M8/P2 design 3.11); production leaves them 0.
	MaxUnitNodes, MaxUnitHistory int
}

// Module is the wired page module.
type Module struct {
	uc      httpadapter.UseCases
	jobs    []jobs.Job
	assets  *app.AssetWrites
	imports *app.ImportWrites
}

// New wires the module: every write runs in the one writer's units. A
// module without the attachments' infos is a fault of its wiring.
func New(d Deps) *Module {
	if d.Attachments == nil {
		panic("page: no attachment infos")
	}
	store := postgresadapter.New(d.Pool)
	md := markdownadapter.New(d.Markdown)
	budget := markdownadapter.NewBudget(d.Budget)
	writer := app.NewWriter(app.WriterDeps{
		Tx: d.Tx, Clock: d.Clock, Auth: d.Authorizer, Workspaces: d.Workspaces, Notebooks: d.Notebooks,
		Nodes: store, NodeWriter: store, Trash: store, History: store, Changesets: store, SessionWriter: store, Names: d.Names,
		Guards: d.Guards, Participants: d.Participants, Observers: d.Observers,
		SessionVetoers: d.EditSessionVetoers, SessionSubscribers: d.EditSessionSubscribers,
	})
	parser := app.NewContentParser(writer, md, budget, d.Budget.Wait())
	versions := app.VersionReads{Notebooks: d.Notebooks, Nodes: store, Versions: store, Names: d.Names, Auth: d.Authorizer, Clock: d.Clock}
	trash := app.TrashReads{Notebooks: d.Notebooks, Nodes: store, Trash: store, Names: d.Names, Attachments: d.Attachments,
		Auth: d.Authorizer, Clock: d.Clock, Retention: d.Retention}
	return &Module{uc: httpadapter.UseCases{
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
		ListVersions:    app.NewListVersions(versions),
		GetVersion:      app.NewGetVersion(versions),
		GetVersionView:  app.NewGetVersionView(versions, md, budget),
		RestoreVersion:  app.NewRestoreVersion(versions, writer, parser, d.Logger),
		ListTrash:       app.NewListTrash(trash),
		GetTrashItem:    app.NewGetTrashItem(trash),
		RestoreNode:     app.NewRestoreNode(trash, writer, parser, app.RestoreBounds{Nodes: d.MaxUnitNodes, History: d.MaxUnitHistory}, d.Logger),
	}, jobs: []jobs.Job{
		riveradapter.CleanupJob(app.NewCleanupEditSessions(store, d.Clock, d.Logger), d.EditSessionCleanupInterval),
	}, assets: app.NewAssetWrites(writer, store), imports: app.NewImportWrites(writer, parser, md)}
}

// Jobs are the module's background jobs, for the server's jobs runner: the
// cleanup of the expired edit sessions.
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
