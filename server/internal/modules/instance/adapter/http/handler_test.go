package httpadapter_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/instance/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
)

type fixedSource domain.Build

// noTokens accepts no token: the module's operations are public.
type noTokens struct{}

func (noTokens) Authenticate(context.Context, string) (context.Context, string, error) {
	return nil, "", errors.New("no token is valid here")
}

func (s fixedSource) Build() domain.Build { return domain.Build(s) }

// get serves the module with uc and answers a GET of path, checked against
// the contract.
func get(t *testing.T, uc httpadapter.UseCases, path string) (*http.Response, string) {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	router := httpserver.NewRouter(logger)
	api := httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: noTokens{}, PublicOperations: httpadapter.PublicOperations()})
	httpadapter.Register(router, api, uc)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	res := rec.Result()

	apitest.Load(t).CheckResponse(t, req, res)
	body, _ := io.ReadAll(res.Body)
	return res, string(body)
}

func TestGetInstanceMatchesTheContract(t *testing.T) {
	// The TTL in whole seconds, a part of one dropped.
	for _, c := range []struct {
		settings app.Settings
		ttl      int64
	}{
		{app.Settings{SignupEnabled: true, AssetMaxBytes: 1 << 10, ImportMaxBytes: 1 << 20, ExportTTL: 10*time.Minute + 999*time.Millisecond}, 600},
		{app.Settings{WorkspaceCreationEnabled: true, AssetMaxBytes: 50 << 20, ImportMaxBytes: 512 << 20, ExportTTL: 24 * time.Hour}, 86400},
	} {
		settings := c.settings
		getInfo := app.NewGetInfo(fixedSource{Version: "1.2.3", Commit: "4f2a9c1"}, settings)

		res, body := get(t, httpadapter.UseCases{GetInfo: getInfo}, "/api/v0/instance")

		want := fmt.Sprintf(`{"api_version":"v0","asset_max_bytes":%d,"commit":"4f2a9c1","export_ttl_seconds":%d,"import_max_bytes":%d,`+
			`"product":"Nerve Wiki","signup_enabled":%t,"version":"1.2.3","workspace_creation_enabled":%t}`, settings.AssetMaxBytes, c.ttl,
			settings.ImportMaxBytes, settings.SignupEnabled, settings.WorkspaceCreationEnabled) + "\n"
		if res.StatusCode != http.StatusOK || body != want {
			t.Errorf("GET /api/v0/instance = %d %s, want 200 %s", res.StatusCode, body, want)
		}
	}
}
