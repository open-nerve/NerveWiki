// Package instance is the pilot module and the template for every module
// (M0/P4 design 3.5): GET /api/v0/instance tells API clients what this Nerve
// Wiki instance runs.
package instance

import (
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/adapter/buildinfo"
	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/instance/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// Module is the wired instance module.
type Module struct {
	uc httpadapter.UseCases
}

// Deps are what bootstrap gives the module: the parts of the
// configuration GET /api/v0/instance reports.
type Deps struct {
	SignupEnabled            bool  // auth.signup_enabled
	WorkspaceCreationEnabled bool  // workspace.creation_enabled
	AssetMaxBytes            int64 // asset.max_bytes
}

// New wires the module: GetInfo reads the build of the running binary.
func New(d Deps) *Module {
	return &Module{uc: httpadapter.UseCases{
		GetInfo: app.NewGetInfo(buildinfo.Source{}, app.Settings{
			SignupEnabled: d.SignupEnabled, WorkspaceCreationEnabled: d.WorkspaceCreationEnabled, AssetMaxBytes: d.AssetMaxBytes,
		}),
	}}
}

// PublicOperations are the module's routes that need no token.
func (m *Module) PublicOperations() []string {
	return httpadapter.PublicOperations()
}

// Register mounts the module's API on router, the root router from
// httpserver.NewRouter, behind api's per-route middlewares.
func (m *Module) Register(router *httpserver.Router, api *httpserver.API) {
	httpadapter.Register(router, api, m.uc)
}
