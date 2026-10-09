// Package transfer is the module of the imports and exports of notebooks
// (M7 design 4.9–4.12; M7/P5, P6 design): each a background job, its row
// in transfer_jobs, its archive a file of the store. Its root is what
// bootstrap sees: New for the HTTP side and the jobs, which read the
// notebooks' trees through the page module's ExportNodes, the links
// through the linking module's LinkedPages and the files through the asset
// module's Blobs, and write an import's nodes through the page module's
// TreeWrites, its files through the asset module's Blobs, and refresh the
// statistics of the tables it fills; the parts in the other modules'
// events (lifecycle.go);
// Purgers for the purge; the export contributors' extension point
// (contributors.go); DownloadKeyInfo, the derivation of the key that signs
// the archives' addresses; Actions for the composition's checks.
package transfer

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	archiveadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/archive"
	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/http"
	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/mac"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/postgres"
	riveradapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/river"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// DownloadKeyInfo derives the key that signs the addresses of the
// exports' archives from identity's signing key (SigningKeys.Derive):
// changing it changes every address signed.
const DownloadKeyInfo = "nervewiki export-download mac v1"

// The exports' and the imports' queues, which bootstrap gives
// jobs.export_workers and jobs.import_workers.
const (
	QueueExport = riveradapter.QueueExport
	QueueImport = riveradapter.QueueImport
)

// What the module reads of the other modules: bootstrap adapts their
// values.
type (
	// Workspaces locks a notebook's workspace: the workspace module's.
	Workspaces = app.Workspaces
	// Notebooks reads and locks the notebooks: the notebook module's.
	Notebooks = app.Notebooks
	// Names reads the accounts' display names: identity's directory.
	Names = app.Names
	// Nodes reads an export's scope: bootstrap adapts the page module's
	// ExportNodes to it.
	Nodes = app.Nodes
	// Node is a node of an export's scope.
	Node = domain.Node
	// Linked tells which pages links lead to: the linking module's.
	Linked = app.Linked
	// Blobs reads the attachments' files: bootstrap adapts the asset
	// module's Blobs to it.
	Blobs = app.Blobs
	// Blob is an attachment's file, and when it was written.
	Blob = app.Blob
	// Clock tells the time.
	Clock = app.Clock
	// Tree writes an import's nodes: bootstrap adapts the page module's
	// TreeWrites to it, and Parsed, ImportUnit and its operands, and
	// CreatedNode to the page module's.
	Tree          = app.Tree
	Parsed        = app.Parsed
	ImportSpec    = app.ImportSpec
	ImportUnit    = app.ImportUnit
	ImportedPage  = app.ImportedPage
	ImportedAsset = app.ImportedAsset
	CreatedNode   = app.CreatedNode
	// Attachments writes an import's attachments' files: bootstrap adapts
	// the asset module's Blobs to it, and File and Owner to its.
	Attachments = app.Attachments
	File        = app.File
	Owner       = app.Owner
	// Analyzer refreshes the statistics of a module's tables an import
	// fills: the page, linking and asset modules' Statistics.
	Analyzer = app.Analyzer
)

// The errors the other modules' adapters answer: a file Blobs.Open does
// not find in the store; an import's page too deep, its parent gone; an
// attachment's file larger than its largest, or the store out of room.
var (
	ErrFileMissing = app.ErrFileMissing
	ErrTooDeep     = app.ErrTooDeep
	ErrNoParent    = app.ErrNoParent
	ErrTooLarge    = app.ErrTooLarge
	ErrStorageFull = domain.ErrStorageFull
)

// Deps are what bootstrap gives the module.
type Deps struct {
	Pool        *pgxpool.Pool
	Tx          shared.TxManager
	Snapshots   shared.Snapshots
	Store       storage.Store
	Inserter    *jobs.Inserter
	Clock       Clock
	Logger      *slog.Logger
	Authorizer  shared.Authorizer
	Workspaces  Workspaces
	Notebooks   Notebooks
	Names       Names
	Nodes       Nodes
	Linked      Linked
	Blobs       Blobs
	Tree        Tree
	Attachments Attachments
	// Statistics are the page, linking and asset modules', in this order.
	Statistics   []Analyzer
	Contributors []ExportContributor
	// DownloadKey signs the archives' addresses: the signing keys'
	// derivation for DownloadKeyInfo.
	DownloadKey []byte
	// ExportTTL is transfer.export_ttl, JobTimeout transfer.job_timeout,
	// HeartbeatTimeout transfer.heartbeat_timeout, MaxQueued
	// transfer.max_queued; the Import* are transfer.import_max_bytes,
	// import_max_entries and import_max_unpacked_bytes; MinFreeBytes
	// storage.min_free_bytes; MinRate asset.upload_min_rate, the uploads'
	// and downloads' slowest; AssetMaxBytes asset.max_bytes; MaxContentBytes
	// the largest page's content, the page module's.
	ExportTTL              time.Duration
	JobTimeout             time.Duration
	HeartbeatTimeout       time.Duration
	MaxQueued              int
	ImportMaxBytes         int64
	ImportMaxEntries       int
	ImportMaxUnpackedBytes int64
	MinFreeBytes           int64
	MinRate                int64
	AssetMaxBytes          int64
	MaxContentBytes        int64
}

// beat is how often a running job writes its heartbeat and progress;
// backoff the first wait of an import the server is too busy for.
const (
	beat    = time.Second
	backoff = time.Second
)

// Module is the wired transfer module.
type Module struct {
	uc     httpadapter.UseCases
	limits httpadapter.Limits
	logger *slog.Logger
	jobs   []jobs.Job
}

// New wires the module: its rows on the pool, its archives in the store,
// its jobs enqueued with the insert-only client.
func New(d Deps) *Module {
	rows, archives, signer := postgresadapter.New(d.Pool), archiveadapter.New(d.Store, d.Logger), macadapter.New(d.DownloadKey)
	queue := riveradapter.NewQueue(d.Inserter)
	export := app.NewExport(app.ExportDeps{Tx: d.Tx, Snapshots: d.Snapshots, Authorizer: d.Authorizer, Notebooks: d.Notebooks, Nodes: d.Nodes,
		Linked: d.Linked, Blobs: d.Blobs, Archives: archives, Rows: rows, Contributors: d.Contributors, Clock: d.Clock, Logger: d.Logger, Beat: beat})
	imp := app.NewImport(app.ImportDeps{Authorizer: d.Authorizer, Notebooks: d.Notebooks, Nodes: d.Nodes, Tree: d.Tree, Attachments: d.Attachments,
		Statistics: d.Statistics, Archives: archives, Rows: rows, Clock: d.Clock, Logger: d.Logger, Beat: beat, MaxEntries: d.ImportMaxEntries,
		MaxUnpacked: d.ImportMaxUnpackedBytes, MaxContent: d.MaxContentBytes, MaxAsset: d.AssetMaxBytes, Backoff: backoff})
	start := app.StartDeps{Tx: d.Tx, Authorizer: d.Authorizer, Workspaces: d.Workspaces, Notebooks: d.Notebooks, Nodes: d.Nodes,
		Rows: rows, Archives: archives, Queue: queue, Names: d.Names, Signer: signer, Clock: d.Clock, Logger: d.Logger, Uploads: app.NewUploads(),
		MaxQueued: d.MaxQueued, MinFree: d.MinFreeBytes, ImportMaxBytes: d.ImportMaxBytes}
	return &Module{
		uc: httpadapter.UseCases{
			Start:  app.NewStartExport(start),
			Import: app.NewStartImport(start),
			Reads: app.NewReads(app.ReadsDeps{Authorizer: d.Authorizer, Notebooks: d.Notebooks, Names: d.Names, Signer: signer, Clock: d.Clock,
				Rows: rows, ExportTTL: d.ExportTTL}),
			Cancel: app.NewCancel(app.CancelDeps{Tx: d.Tx, Rows: rows, Authorizer: d.Authorizer, Notebooks: d.Notebooks, Names: d.Names,
				Signer: signer, Clock: d.Clock, Logger: d.Logger, ExportTTL: d.ExportTTL}),
			Download: app.NewDownload(rows, archives, signer, d.Clock, d.Logger, d.ExportTTL),
		},
		limits: httpadapter.Limits{ImportMaxBytes: d.ImportMaxBytes, MinRate: d.MinRate},
		logger: d.Logger,
		jobs: []jobs.Job{
			riveradapter.ExportJob(export, d.JobTimeout),
			riveradapter.ImportJob(imp, d.JobTimeout),
			riveradapter.RescueJob(app.NewRescue(rows, queue, d.Clock, d.Logger, d.HeartbeatTimeout)),
			riveradapter.ExpireJob(app.NewExpire(rows, archives, d.Clock, d.Logger, d.ExportTTL)),
			riveradapter.SweepJob(app.NewSweep(rows, archives, d.Clock, d.Logger)),
		},
	}
}

// Register mounts the module's API on router, the root router from
// httpserver.NewRouter, behind api's per-route middlewares.
func (m *Module) Register(router *httpserver.Router, api *httpserver.API) {
	httpadapter.Register(router, api, m.uc, m.limits, m.logger)
}

// Jobs are the module's background jobs, for the server's jobs runner:
// the exports and the imports, the rescue of the interrupted ones, the
// exports' expiry and the sweep of the archives no job keeps.
func (m *Module) Jobs() []jobs.Job {
	return m.jobs
}

// PublicOperations are the module's routes that need no token: the
// download.
func (m *Module) PublicOperations() []string {
	return httpadapter.PublicOperations()
}

// Actions are the module's actions: bootstrap checks they are the access
// module's rule table.
func Actions() []shared.Action {
	return domain.Actions()
}
