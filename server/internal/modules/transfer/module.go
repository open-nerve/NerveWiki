// Package transfer is the module of the imports and exports of notebooks
// (M7 design 4.9–4.12; M7/P5 design): each a background job, its row in
// transfer_jobs, its archive a file of the store. Its root is what
// bootstrap sees: New for the HTTP side and the jobs, which read the
// notebooks' trees through the page module's ExportNodes, the links
// through the linking module's LinkedPages and the files through the asset
// module's Blobs; the parts in the other modules' events (lifecycle.go);
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

// QueueExport is the exports' queue, which bootstrap gives
// jobs.export_workers.
const QueueExport = riveradapter.QueueExport

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
	// Clock tells the time.
	Clock = app.Clock
)

// ErrFileMissing is a file Blobs.Open does not find in the store.
var ErrFileMissing = app.ErrFileMissing

// Deps are what bootstrap gives the module.
type Deps struct {
	Pool         *pgxpool.Pool
	Tx           shared.TxManager
	Snapshots    shared.Snapshots
	Store        storage.Store
	Inserter     *jobs.Inserter
	Clock        Clock
	Logger       *slog.Logger
	Authorizer   shared.Authorizer
	Workspaces   Workspaces
	Notebooks    Notebooks
	Names        Names
	Nodes        Nodes
	Linked       Linked
	Blobs        Blobs
	Contributors []ExportContributor
	// DownloadKey signs the archives' addresses: the signing keys'
	// derivation for DownloadKeyInfo.
	DownloadKey []byte
	// ExportTTL is transfer.export_ttl, JobTimeout transfer.job_timeout,
	// HeartbeatTimeout transfer.heartbeat_timeout, MaxQueued
	// transfer.max_queued; MinFreeBytes storage.min_free_bytes; MinRate
	// asset.upload_min_rate, the downloads' slowest.
	ExportTTL        time.Duration
	JobTimeout       time.Duration
	HeartbeatTimeout time.Duration
	MaxQueued        int
	MinFreeBytes     int64
	MinRate          int64
}

// beat is how often a running job writes its heartbeat and progress.
const beat = time.Second

// Module is the wired transfer module.
type Module struct {
	uc      httpadapter.UseCases
	minRate int64
	jobs    []jobs.Job
}

// New wires the module: its rows on the pool, its archives in the store,
// its exports enqueued with the insert-only client.
func New(d Deps) *Module {
	rows, archives, signer := postgresadapter.New(d.Pool), archiveadapter.New(d.Store, d.Logger), macadapter.New(d.DownloadKey)
	export := app.NewExport(app.ExportDeps{Tx: d.Tx, Snapshots: d.Snapshots, Authorizer: d.Authorizer, Notebooks: d.Notebooks, Nodes: d.Nodes,
		Linked: d.Linked, Blobs: d.Blobs, Archives: archives, Rows: rows, Contributors: d.Contributors, Clock: d.Clock, Logger: d.Logger, Beat: beat})
	return &Module{
		uc: httpadapter.UseCases{
			Start: app.NewStartExport(app.StartDeps{Tx: d.Tx, Authorizer: d.Authorizer, Workspaces: d.Workspaces, Notebooks: d.Notebooks, Nodes: d.Nodes,
				Rows: rows, Archives: archives, Queue: riveradapter.NewQueue(d.Inserter), Names: d.Names, Signer: signer, Clock: d.Clock, Logger: d.Logger,
				MaxQueued: d.MaxQueued, MinFree: d.MinFreeBytes}),
			Reads: app.NewReads(app.ReadsDeps{Authorizer: d.Authorizer, Notebooks: d.Notebooks, Names: d.Names, Signer: signer, Clock: d.Clock,
				Rows: rows}),
			Cancel: app.NewCancel(app.CancelDeps{Tx: d.Tx, Rows: rows, Authorizer: d.Authorizer, Notebooks: d.Notebooks, Names: d.Names,
				Signer: signer, Clock: d.Clock}),
			Download: app.NewDownload(rows, archives, signer, d.Clock, d.Logger),
		},
		minRate: d.MinRate,
		jobs: []jobs.Job{
			riveradapter.ExportJob(export, d.JobTimeout),
			riveradapter.RescueJob(app.NewRescue(rows, d.Clock, d.Logger, d.HeartbeatTimeout)),
			riveradapter.ExpireJob(app.NewExpire(rows, archives, d.Clock, d.Logger, d.ExportTTL)),
			riveradapter.SweepJob(app.NewSweep(rows, archives, d.Clock, d.Logger)),
		},
	}
}

// Register mounts the module's API on router, the root router from
// httpserver.NewRouter, behind api's per-route middlewares.
func (m *Module) Register(router *httpserver.Router, api *httpserver.API) {
	httpadapter.Register(router, api, m.uc, m.minRate)
}

// Jobs are the module's background jobs, for the server's jobs runner:
// the exports, the rescue of the interrupted ones, their expiry and the
// sweep of the archives no job keeps.
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
