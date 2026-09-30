package httpadapter_test

import (
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
)

type fixedSource domain.Build

func (s fixedSource) Build() domain.Build { return domain.Build(s) }

// get serves the module with uc and answers a GET of path, checked against
// the contract.
func get(t *testing.T, uc httpadapter.UseCases, path string) (*http.Response, string) {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	router := httpserver.NewRouter(logger)
	api, err := httpserver.NewAPI(httpserver.APIConfig{Logger: logger, MaxBodyBytes: 1 << 20, RequestTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
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
	getInfo := app.NewGetInfo(fixedSource{Version: "1.2.3", Commit: "4f2a9c1"})

	res, body := get(t, httpadapter.UseCases{GetInfo: getInfo}, "/api/v0/instance")

	want := `{"api_version":"v0","commit":"4f2a9c1","product":"Nerve Wiki","version":"1.2.3"}` + "\n"
	if res.StatusCode != http.StatusOK || body != want {
		t.Errorf("GET /api/v0/instance = %d %s, want 200 %s", res.StatusCode, body, want)
	}
}
