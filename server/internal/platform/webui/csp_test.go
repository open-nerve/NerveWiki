package webui

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

// Scripts only from nervewiki: no inline script, no eval, no other origin.
func TestPolicyAllowsOnlyNervewikisScripts(t *testing.T) {
	var scripts string
	for directive := range strings.SplitSeq(contentSecurityPolicy, "; ") {
		if rest, ok := strings.CutPrefix(directive, "script-src "); ok {
			scripts = rest
		}
	}
	if scripts != "'self'" {
		t.Errorf("script-src = %q, want 'self' alone; policy %s", scripts, contentSecurityPolicy)
	}
	for _, want := range []string{"default-src 'self'", "object-src 'none'", "base-uri 'none'", "frame-ancestors 'none'"} {
		if !strings.Contains(contentSecurityPolicy, want) {
			t.Errorf("policy lacks %q: %s", want, contentSecurityPolicy)
		}
	}
}

// A worker loads and sends nothing.
func TestWorkersLoadNothing(t *testing.T) {
	if workerPolicy != "default-src 'none'" {
		t.Errorf("workerPolicy = %q, want default-src 'none' alone", workerPolicy)
	}
}

// The pages' policy goes on the pages, the workers' on the scripts under
// assets/, and none on anything else: other files, missing assets and the
// answers of an unbuilt frontend.
func TestPagesAndWorkersCarryTheirPolicies(t *testing.T) {
	h := Handler(built())
	for _, tt := range []struct{ method, target, want string }{
		{http.MethodGet, "/", contentSecurityPolicy},
		{http.MethodGet, "/nope?tab=a", contentSecurityPolicy},
		{http.MethodHead, "/acme", contentSecurityPolicy},
		{http.MethodGet, "/assets/index-a1.js", workerPolicy},
		{http.MethodHead, "/assets/index-a1.js", workerPolicy},
		{http.MethodGet, "/assets/index-b2.css", ""},
		{http.MethodGet, "/theme-init.js", ""},
		{http.MethodGet, "/assets/index-old.js", ""},
	} {
		rec := serve(h, tt.method, tt.target)
		if got := rec.Result().Header.Get("Content-Security-Policy"); got != tt.want {
			t.Errorf("%s %s (%d): Content-Security-Policy = %q, want %q", tt.method, tt.target, rec.Code, got, tt.want)
		}
	}

	unbuilt := serve(Handler(fstest.MapFS{".gitkeep": {}}), http.MethodGet, "/")
	if got := unbuilt.Result().Header.Get("Content-Security-Policy"); got != "" {
		t.Errorf("unbuilt GET / (%d): Content-Security-Policy = %q, want none", unbuilt.Code, got)
	}
}
