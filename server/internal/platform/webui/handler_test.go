package webui

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

const indexHTML = "<!doctype html><title>Nerve Wiki</title>"

// built mirrors the layout of web/apps/web/dist once copied into dist/.
func built() fstest.MapFS {
	return fstest.MapFS{
		".gitkeep":            {},
		"index.html":          {Data: []byte(indexHTML)},
		"theme-init.js":       {Data: []byte("void 0;")},
		"icons/favicon.svg":   {Data: []byte("<svg/>")},
		"assets/index-a1.js":  {Data: []byte("export {};")},
		"assets/index-b2.css": {Data: []byte("body{}")},
		"assets/.hidden":      {Data: []byte("secret")},
	}
}

func serve(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

// wantResponse checks status, Content-Type, Cache-Control and body.
func wantResponse(t *testing.T, rec *httptest.ResponseRecorder, target string, status int, contentType, cacheControl, body string) {
	t.Helper()
	if rec.Code != status {
		t.Errorf("%s: status = %d, want %d", target, rec.Code, status)
	}
	sent := rec.Result().Header // as the status went out, not as set after it
	if got := sent.Get("Content-Type"); got != contentType {
		t.Errorf("%s: Content-Type = %q, want %q", target, got, contentType)
	}
	if got := sent.Get("Cache-Control"); got != cacheControl {
		t.Errorf("%s: Cache-Control = %q, want %q", target, got, cacheControl)
	}
	if got := rec.Body.String(); got != body {
		t.Errorf("%s: body = %q, want %q", target, got, body)
	}
}

func TestServesIndexAtRoot(t *testing.T) {
	rec := serve(Handler(built()), http.MethodGet, "/")

	wantResponse(t, rec, "/", http.StatusOK, "text/html; charset=utf-8", "no-cache", indexHTML)
}

func TestPagePathsFallBackToIndex(t *testing.T) {
	h := Handler(built())
	for _, target := range []string{
		"/nope",
		"/acme/notebooks/0199f1c2-7a1b-7c3d-8e4f-5a6b7c8d9e0f/pages",
		"/acme/settings/",
		"/icons",         // a directory, not a file
		"/.gitkeep",      // hidden files are never served
		"/favicon.ico",   // no such file outside assets/
		"/acme?tab=home", // the query does not matter
	} {
		wantResponse(t, serve(h, http.MethodGet, target), target, http.StatusOK, "text/html; charset=utf-8", "no-cache", indexHTML)
	}
}

func TestServesFilesWithCachePolicy(t *testing.T) {
	h := Handler(built())
	tests := []struct {
		target, contentType, cacheControl, body string
	}{
		// Vite content-hashes everything under assets/: a changed file gets a new name.
		{"/assets/index-a1.js", "text/javascript; charset=utf-8", "public, max-age=31536000, immutable", "export {};"},
		{"/assets/index-b2.css", "text/css; charset=utf-8", "public, max-age=31536000, immutable", "body{}"},
		// Files copied from public/ keep their names, so they are revalidated.
		{"/theme-init.js", "text/javascript; charset=utf-8", "no-cache", "void 0;"},
		{"/icons/favicon.svg", "image/svg+xml", "no-cache", "<svg/>"},
	}
	for _, tt := range tests {
		wantResponse(t, serve(h, http.MethodGet, tt.target), tt.target, http.StatusOK, tt.contentType, tt.cacheControl, tt.body)
	}
}

// A missing hashed file (for example a chunk of an older build) must fail as
// a missing file: answering index.html would hand HTML to a script loader.
func TestMissingAssetsAre404(t *testing.T) {
	h := Handler(built())
	for _, target := range []string{"/assets/index-old.js", "/assets/", "/assets", "/assets/.hidden"} {
		rec := serve(h, http.MethodGet, target)
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), indexHTML) {
			t.Errorf("GET %s = %d %q, want 404 without index.html", target, rec.Code, rec.Body)
		}
	}
}

func TestIndexHTMLRedirectsToRoot(t *testing.T) {
	rec := serve(Handler(built()), http.MethodGet, "/index.html")

	if location := rec.Result().Header.Get("Location"); rec.Code != http.StatusMovedPermanently || location != "./" {
		t.Errorf("GET /index.html = %d Location %q, want 301 to ./", rec.Code, location)
	}
}

func TestHeadHasHeadersButNoBody(t *testing.T) {
	h := Handler(built())
	for _, target := range []string{"/", "/acme/notebooks", "/assets/index-a1.js"} {
		rec := serve(h, http.MethodHead, target)
		length := rec.Result().Header.Get("Content-Length")
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 || length == "" {
			t.Errorf("HEAD %s = %d, body %d bytes, Content-Length %q; want 200, no body, a length",
				target, rec.Code, rec.Body.Len(), length)
		}
	}
}

func TestOnlyGetAndHeadAreAllowed(t *testing.T) {
	for _, files := range []fs.FS{built(), fstest.MapFS{}} {
		h := Handler(files)
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions} {
			rec := serve(h, method, "/assets/index-a1.js")
			if allow := rec.Result().Header.Get("Allow"); rec.Code != http.StatusMethodNotAllowed || allow != "GET, HEAD" {
				t.Errorf("%s = %d Allow %q, want 405 Allow \"GET, HEAD\"", method, rec.Code, allow)
			}
		}
	}
}

// Without index.html (dist/ holds only .gitkeep) every page explains how to
// get the frontend.
func TestUnbuiltFrontendExplainsItself(t *testing.T) {
	h := Handler(fstest.MapFS{".gitkeep": {}})
	for _, target := range []string{"/", "/acme/notebooks", "/assets/index-a1.js"} {
		wantResponse(t, serve(h, http.MethodGet, target), target, http.StatusNotFound, "text/plain; charset=utf-8", "no-cache", notBuiltMessage+"\n")
	}
}

// The files a browser revalidates carry an ETag of their content, so that
// revalidation costs a 304; the hashed assets need none.
func TestRevalidatedFilesAnswer304ToTheirETag(t *testing.T) {
	h := Handler(built())
	for _, target := range []string{"/", "/nope", "/theme-init.js"} {
		etag := serve(h, http.MethodGet, target).Result().Header.Get("ETag")
		if etag == "" {
			t.Fatalf("GET %s has no ETag", target)
		}
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("If-None-Match", etag)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
			t.Errorf("GET %s with If-None-Match %s = %d with %d bytes, want 304 without a body", target, etag, rec.Code, rec.Body.Len())
		}
	}
	if etag := serve(h, http.MethodGet, "/assets/index-a1.js").Result().Header.Get("ETag"); etag != "" {
		t.Errorf("an asset has the ETag %s, want none: its name changes with its content", etag)
	}
	if a, b := serve(h, http.MethodGet, "/").Result().Header.Get("ETag"), serve(h, http.MethodGet, "/theme-init.js").Result().Header.Get("ETag"); a == b {
		t.Errorf("index.html and theme-init.js share the ETag %s", a)
	}
}

// A range of a file is 206; a Range of several ranges is passed by, the
// whole file 200, as the downloads answer it.
func TestServesARangeAndPassesSeveralBy(t *testing.T) {
	h := Handler(built())
	for _, tt := range []struct {
		value, body string
		status      int
	}{
		{"bytes=0-5", "export", http.StatusPartialContent},
		{"bytes=0-0,1-1", "export {};", http.StatusOK},
		{"bytes=" + strings.Repeat("0-0,", 1000) + "0-0", "export {};", http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodGet, "/assets/index-a1.js", nil)
		req.Header.Set("Range", tt.value)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != tt.status || rec.Body.String() != tt.body {
			t.Errorf("Range %.40s = %d %q, want %d %q", tt.value, rec.Code, rec.Body.String(), tt.status, tt.body)
		}
	}
}
