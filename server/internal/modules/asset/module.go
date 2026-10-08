// Package asset is the module of the attachments (M7 design 4.2–4.5; M7/P2
// design): an attachment is a node of a notebook's tree, which the page
// module keeps, and a file, which this module keeps in the store with its
// row. Its root is what bootstrap sees: New for the HTTP side, which
// creates the attachments' nodes through the page module's TreeWrites;
// Purgers for the purge; ContentKeyInfo, the derivation of the key that
// signs the contents' addresses; Actions for the composition's checks.
package asset

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	filesadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/files"
	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/http"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/sniff"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
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
	Pool   *pgxpool.Pool
	Store  storage.Store
	Clock  Clock
	Logger *slog.Logger
	Tree   Tree
	// ContentKey signs the contents' addresses: the signing keys'
	// derivation for ContentKeyInfo.
	ContentKey []byte
	// MaxBytes is asset.max_bytes, MinRate asset.upload_min_rate,
	// MinFreeBytes storage.min_free_bytes.
	MaxBytes     int64
	MinRate      int64
	MinFreeBytes int64
}

// Module is the wired asset module.
type Module struct {
	uc     httpadapter.UseCases
	limits httpadapter.Limits
	logger *slog.Logger
}

// New wires the module: its files in the store, its rows on the pool.
func New(d Deps) *Module {
	files := filesadapter.New(d.Store)
	blobs := app.NewBlobs(files, postgresadapter.New(d.Pool), sniff.Sniffer{})
	signer := app.NewSigner(d.ContentKey, d.Clock)
	return &Module{
		uc: httpadapter.UseCases{
			Upload: app.NewUpload(app.UploadDeps{Tree: d.Tree, Blobs: blobs, Files: files, Signer: signer, Logger: d.Logger,
				MaxBytes: d.MaxBytes, MinFree: d.MinFreeBytes}),
		},
		limits: httpadapter.Limits{MaxBytes: d.MaxBytes, MinRate: d.MinRate},
		logger: d.Logger,
	}
}

// Register mounts the module's API on router, the root router from
// httpserver.NewRouter, behind api's per-route middlewares.
func (m *Module) Register(router *httpserver.Router, api *httpserver.API) {
	httpadapter.Register(router, api, m.uc, m.limits, m.logger)
}

// Actions are the module's actions: bootstrap checks they are the access
// module's rule table.
func Actions() []shared.Action {
	return domain.Actions()
}
