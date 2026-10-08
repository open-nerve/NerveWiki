package httpadapter_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
)

// A parameter of a read that does not bind is 400, invalid_format on it.
func TestReadsBindTheirParameters(t *testing.T) {
	h := newHarness(t)
	list := "/api/v0/notebooks/" + notebookID().String() + "/assets"
	for _, tt := range []struct{ path, field string }{
		{"/api/v0/assets/nope", "node_id"},
		{"/api/v0/notebooks/nope/assets", "notebook_id"},
		{list + "?limit=x", "limit"},
		{list + "?parent_id=x", "parent_id"},
	} {
		res, body := h.get(t, http.MethodGet, tt.path, "session")
		var p problem
		_ = json.Unmarshal(body, &p)
		if res.StatusCode != http.StatusBadRequest || p.Code != "bad_request" || len(p.Errors) != 1 || p.Errors[0].Field != tt.field ||
			p.Errors[0].Code != "invalid_format" {
			t.Errorf("%s = %d %s, want 400, invalid_format on %s", tt.path, res.StatusCode, body, tt.field)
		}
	}
}

// A read past the request's deadline is 500, logged as a request that ran
// out of time, a warning, not as a fault.
func TestReadPastItsDeadlineIsLoggedAsAWarning(t *testing.T) {
	h := newHarnessWith(t, httpservertest.APIOptions{RequestTimeout: 100 * time.Millisecond})
	n, _ := h.attach("a.png", "image/png", "abc")
	h.nodes.gate, h.nodes.asked = make(chan struct{}), make(chan struct{}, 1)
	defer close(h.nodes.gate)
	res, body := h.get(t, http.MethodGet, "/api/v0/assets/"+n.ID.String(), "session")
	if res.StatusCode != http.StatusInternalServerError || code(body) != "internal_error" {
		t.Errorf("getAsset = %d %s, want 500 internal_error", res.StatusCode, body)
	}
	if logs := h.logs.String(); !strings.Contains(logs, `level=WARN msg="API request deadline exceeded"`) ||
		strings.Contains(logs, "level=ERROR") {
		t.Errorf("logs %q, want the deadline as a warning, no error", logs)
	}
}

// The reads and the upload take the platform's buckets, the failures'
// gate by the client's IP and the authenticated bucket by the credential,
// and not the downloads' bucket.
func TestRoutesTakeThePlatformsBuckets(t *testing.T) {
	gate, authenticated, anonymous := &bucket{left: 1000}, &bucket{left: 1000}, &bucket{left: 1000}
	h := newHarnessWith(t, httpservertest.APIOptions{AuthFailure: gate, Authenticated: authenticated, Anonymous: anonymous})
	n, _ := h.attach("a.png", "image/png", "abc")
	ct, upload := form(t, file("b.png", "x"))
	for _, tt := range []struct {
		name string
		send func() (*http.Response, []byte)
		want int
	}{
		{"getAsset", func() (*http.Response, []byte) {
			return h.get(t, http.MethodGet, "/api/v0/assets/"+n.ID.String(), "session")
		}, http.StatusOK},
		{"listAssets", func() (*http.Response, []byte) {
			return h.get(t, http.MethodGet, "/api/v0/notebooks/"+notebookID().String()+"/assets", "session")
		}, http.StatusOK},
		{"uploadAsset", func() (*http.Response, []byte) {
			return h.post(t, uploadPath(), "session", ct, bytes.NewReader(upload))
		}, http.StatusCreated},
	} {
		gate.keys, authenticated.keys, anonymous.keys, h.downloads.keys = nil, nil, nil, nil
		if res, body := tt.send(); res.StatusCode != tt.want {
			t.Fatalf("%s = %d %s, want %d", tt.name, res.StatusCode, body, tt.want)
		}
		if !slices.Equal(gate.keys, []string{"127.0.0.1"}) || !slices.Equal(authenticated.keys, []string{"session"}) ||
			len(anonymous.keys) != 0 || len(h.downloads.keys) != 0 {
			t.Errorf("%s: gate %q, authenticated %q, anonymous %q, downloads %q; want 127.0.0.1, session, none, none", tt.name,
				gate.keys, authenticated.keys, anonymous.keys, h.downloads.keys)
		}
	}
}

// The stream routes' policies: the upload's body is the largest file and
// the Envelope, at the lowest rate, under the platform's buckets; the
// download leaves at the lowest rate, under its own bucket.
func TestStreamRoutesPolicies(t *testing.T) {
	limits := httpadapter.Limits{MaxBytes: 5 << 20, MinRate: 32 << 10, ContentBucket: &bucket{}}
	if got, want := httpadapter.UploadPolicy(limits), (httpserver.StreamPolicy{MaxBytes: 5<<20 + 64<<10, MinRate: 32 << 10}); got != want {
		t.Errorf("the upload's policy = %+v, want %+v", got, want)
	}
	want := httpserver.StreamPolicy{MinRate: 32 << 10, Bucket: limits.ContentBucket, BucketName: "asset_content"}
	if got := httpadapter.DownloadPolicy(limits); got != want {
		t.Errorf("the download's policy = %+v, want %+v", got, want)
	}
}
