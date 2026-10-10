package bootstrap

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/buildinfo"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// send makes a request of method to url and returns it with its answer and
// the answer's body, which also stays readable in res.Body.
func send(t *testing.T, method, url string) (*http.Request, *http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	res.Body = io.NopCloser(bytes.NewReader(body))
	return req, res, body
}

// The instance module answers through the whole chain: the platform
// middleware (X-Request-Id), then the module's per-route middlewares and
// handler; the limits it reports are the configuration's.
func TestServesTheInstanceAPI(t *testing.T) {
	contract := apitest.Load(t)
	cfg := testConfig(t, pgtest.NewDatabase(t), false)
	cfg.Asset.MaxBytes, cfg.Transfer.ImportMaxBytes, cfg.Transfer.ExportTTL = 3<<20, 5<<20, 90*time.Minute
	base := startApp(t, cfg, migrations.FS())

	req, res, body := send(t, http.MethodGet, base+"/api/v0/instance")

	contract.CheckResponse(t, req, res)
	var got struct {
		Product        string `json:"product"`
		Version        string `json:"version"`
		Commit         string `json:"commit"`
		APIVersion     string `json:"api_version"`
		AssetMaxBytes  int64  `json:"asset_max_bytes"`
		ImportMaxBytes int64  `json:"import_max_bytes"`
		ExportTTL      int64  `json:"export_ttl_seconds"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	info := buildinfo.Get()
	if res.StatusCode != http.StatusOK || got.Product != "Nerve Wiki" || got.Version != info.Version || got.Commit != info.Commit || got.APIVersion != "v0" {
		t.Errorf("GET /api/v0/instance = %d %s, want 200 Nerve Wiki %s %s v0", res.StatusCode, body, info.Version, info.Commit)
	}
	if got.AssetMaxBytes != 3<<20 || got.ImportMaxBytes != 5<<20 || got.ExportTTL != 5400 {
		t.Errorf("GET /api/v0/instance = %s, want asset.max_bytes 3 MiB, transfer.import_max_bytes 5 MiB, transfer.export_ttl 5400 s", body)
	}
	if res.Header.Get(httpserver.HeaderRequestID) == "" {
		t.Error("response has no X-Request-Id: the platform middleware did not run")
	}
}

// Mounting a module keeps the platform's /api/ fallback: an unknown path and
// a known path with the wrong method both answer 404 problem+json.
func TestUnknownAPIRequestsStillAnswerProblem404(t *testing.T) {
	contract := apitest.Load(t)
	base := startApp(t, testConfig(t, pgtest.NewDatabase(t), false), migrations.FS())
	for _, tt := range []struct{ method, path string }{
		{http.MethodGet, "/api/v0/nope"},
		{http.MethodPost, "/api/v0/instance"},
	} {
		_, res, body := send(t, tt.method, base+tt.path)

		if res.StatusCode != http.StatusNotFound || res.Header.Get("Content-Type") != httpserver.ContentTypeProblem {
			t.Errorf("%s %s = %d %s, want 404 problem+json", tt.method, tt.path, res.StatusCode, res.Header.Get("Content-Type"))
		}
		contract.CheckSchema(t, "Problem", body)
		var p httpserver.Problem
		if err := json.Unmarshal(body, &p); err != nil || p.Detail != "no API endpoint for "+tt.method+" "+tt.path {
			t.Errorf("%s %s detail = %q (%v), want no API endpoint for %s %s", tt.method, tt.path, p.Detail, err, tt.method, tt.path)
		}
	}
}
