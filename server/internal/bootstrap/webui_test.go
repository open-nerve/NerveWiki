package bootstrap

import (
	"net/http"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The frontend is mounted on "/" below the platform's routes: every page path
// answers index.html with the pages' CSP and the platform's security
// headers, while /api/ and the probes keep their own answers.
func TestTheFrontendServesEveryOtherPath(t *testing.T) {
	base := startApp(t, testConfig(t, pgtest.NewDatabase(t), false), migrations.FS())
	tests := []struct {
		path, status, contentType string
		page                      bool
	}{
		{"/", "200", "text/html; charset=utf-8", true},
		{"/acme/notebooks/1/pages", "200", "text/html; charset=utf-8", true},
		{"/assets/index-a1.js", "200", "text/javascript; charset=utf-8", false},
		{"/assets/index-old.js", "404", "text/plain; charset=utf-8", false},
		{"/api/nope", "404", httpserver.ContentTypeProblem, false},
		{"/api", "404", httpserver.ContentTypeProblem, false}, // ServeMux redirects it to /api/
		{"/healthz", "200", "application/json", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			_, res, body := send(t, http.MethodGet, base+tt.path)

			if got := res.Status[:3]; got != tt.status || res.Header.Get("Content-Type") != tt.contentType {
				t.Errorf("GET %s = %s %s, want %s %s", tt.path, res.Status, res.Header.Get("Content-Type"), tt.status, tt.contentType)
			}
			csp := res.Header.Get("Content-Security-Policy")
			if tt.page != (csp != "") || tt.page != strings.Contains(string(body), "<title>Nerve Wiki</title>") {
				t.Errorf("GET %s: CSP %q, body %q; want the page with its CSP: %v", tt.path, csp, body, tt.page)
			}
			if res.Header.Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("GET %s: no X-Content-Type-Options: the platform middleware did not run", tt.path)
			}
		})
	}
}
