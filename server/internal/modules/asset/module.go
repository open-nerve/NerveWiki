// Package asset is the module of the attachments (M7 design 4.2–4.5; M7/P2
// design): an attachment is a node of a notebook's tree, which the page
// module keeps, and a file, which this module keeps in the store with its
// row. Its root is what bootstrap sees: New for the HTTP side, which
// creates the attachments' nodes through the page module's TreeWrites and
// reads them through its AssetNodes, and Jobs, the orphan sweep; the
// parts in the other modules' events (lifecycle.go); Purgers for the purge;
// ContentKeyInfo, the derivation of the key that signs the contents'
// addresses; Actions for the composition's checks.
package asset

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	filesadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/files"
	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/http"
	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/mac"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres"
	riveradapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/river"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/sniff"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ContentKeyInfo derives the key that signs the addresses of the
// attachments' contents from identity's signing key (SigningKeys.Derive):
// changing it changes every address signed.
const ContentKeyInfo = "nervewiki asset-content mac v1"

// What the module reads and writes of the other modules: bootstrap adapts
// the page module's values, field by field.
type (
	// Tree creates the attachments' nodes: bootstrap adapts page's
	// TreeWrites to it.
	Tree = app.Tree
	// Nodes reads the notebooks' trees: bootstrap adapts page's AssetNodes
	// to it.
	Nodes = app.Nodes
	// Cursor is where a list of attachments goes on.
	Cursor = app.Cursor
	// Notebooks reads the notebooks: bootstrap hands it the notebook
	// module's.
	Notebooks = app.Notebooks
	// NewNode is an attachment's node to create.
	NewNode = app.NewNode
	// FileMeta is what the page module's guards see of its file.
	FileMeta = app.FileMeta
	// Node is a node of a notebook's tree.
	Node = app.Node
	// Clock tells the time.
	Clock = app.Clock
)

// Deps are what bootstrap gives the module.
type Deps struct {
	Pool       *pgxpool.Pool
	Store      storage.Store
	Clock      Clock
	Logger     *slog.Logger
	Authorizer shared.Authorizer
	Notebooks  Notebooks
	Tree       Tree
	Nodes      Nodes
	// ContentKey signs the contents' addresses: the signing keys'
	// derivation for ContentKeyInfo.
	ContentKey []byte
	// MaxBytes is asset.max_bytes, MinRate asset.upload_min_rate,
	// MinFreeBytes storage.min_free_bytes.
	MaxBytes     int64
	MinRate      int64
	MinFreeBytes int64
	// ContentBucket is ratelimit.asset_content, the downloads' bucket.
	ContentBucket httpserver.Limiter
}

// Module is the wired asset module.
type Module struct {
	uc     httpadapter.UseCases
	limits httpadapter.Limits
	logger *slog.Logger
	jobs   []jobs.Job
}

// New wires the module: its files in the store, its rows on the pool.
func New(d Deps) *Module {
	files, rows := filesadapter.New(d.Store), postgresadapter.New(d.Pool)
	blobs := app.NewBlobs(files, rows, sniff.Sniffer{}, d.Logger)
	signer := macadapter.New(d.ContentKey)
	return &Module{
		uc: httpadapter.UseCases{
			Upload: app.NewUpload(app.UploadDeps{Tree: d.Tree, Blobs: blobs, Files: files, Signer: signer, Logger: d.Logger,
				MaxBytes: d.MaxBytes, MinFree: d.MinFreeBytes}),
			Reads: app.NewReads(app.ReadsDeps{Authorizer: d.Authorizer, Notebooks: d.Notebooks, Nodes: d.Nodes, Rows: rows, Signer: signer,
				Clock: d.Clock, Logger: d.Logger}),
			Content: app.NewContent(d.Nodes, blobs, signer, d.Clock, d.Logger),
		},
		limits: httpadapter.Limits{MaxBytes: d.MaxBytes, MinRate: d.MinRate, ContentBucket: d.ContentBucket},
		logger: d.Logger,
		jobs:   []jobs.Job{riveradapter.SweepJob(app.NewSweep(files, rows, d.Clock, d.Logger))},
	}
}

// Register mounts the module's API on router, the root router from
// httpserver.NewRouter, behind api's per-route middlewares.
func (m *Module) Register(router *httpserver.Router, api *httpserver.API) {
	httpadapter.Register(router, api, m.uc, m.limits, m.logger)
}

// Jobs are the module's background jobs, for the server's jobs runner: the
// sweep of the files no row holds.
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
