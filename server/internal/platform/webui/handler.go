package webui

import (
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const (
	indexFile = "index.html"
	// assetsDir holds Vite's content-hashed files: a changed file gets a new name.
	assetsDir = "assets"

	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheRevalidate = "no-cache"

	notBuiltMessage = "The web frontend is not built into this binary: run `make build`, " +
		"or use the Vite dev server (`make web-dev`) during development."
)

// Handler serves the single-page app in files (GET and HEAD only):
//
//   - a file: served as is; files under assets/ are cached for good, others
//     (index.html, files copied from public/) are revalidated on every use,
//     by an ETag of their content;
//   - a missing path under assets/: 404, so a chunk of an older build fails
//     as a missing file instead of turning into HTML;
//   - any other path: index.html, and the client-side router renders the
//     page, its own 404 included.
//
// An HTML file goes out with the pages' Content-Security-Policy; other files
// and errors have none. The security headers every response carries are the
// platform middleware's.
//
// Hidden files (any name part starting with "."), such as dist/.gitkeep, are
// never served. Without index.html every request answers 404 with a hint.
// Mount it on the pattern "/" so that /api/ and the probes keep their routes.
func Handler(files fs.FS) http.Handler {
	if _, err := fs.Stat(files, indexFile); err != nil {
		return &handler{files: files}
	}
	return &handler{files: files, built: true, etags: contentETags(files)}
}

type handler struct {
	files fs.FS
	built bool
	etags map[string]string // name → ETag, for the files outside assets/
}

// contentETags makes an ETag from the content of each file outside assets/:
// the embedded files have no modification time, so without one a browser
// downloads index.html and theme-init.js again on every load. A file that
// cannot be read gets none; serving it reports the error.
func contentETags(files fs.FS) map[string]string {
	etags := map[string]string{}
	_ = fs.WalkDir(files, ".", func(name string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return nil
		case d.IsDir() && name == assetsDir:
			return fs.SkipDir
		case d.Type().IsRegular():
			if data, err := fs.ReadFile(files, name); err == nil {
				sum := sha256.Sum256(data)
				etags[name] = `"` + base64.RawURLEncoding.EncodeToString(sum[:16]) + `"`
			}
		}
		return nil
	})
	return etags
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !h.built {
		w.Header().Set("Cache-Control", cacheRevalidate)
		http.Error(w, notBuiltMessage, http.StatusNotFound)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = indexFile
	}
	switch {
	case h.isFile(name):
		h.serveFile(w, r, name)
	case name == assetsDir || strings.HasPrefix(name, assetsDir+"/"):
		http.NotFound(w, r)
	default:
		h.serveFile(w, r, indexFile)
	}
}

// isFile reports whether name is a regular, visible file in h.files.
func (h *handler) isFile(name string) bool {
	if strings.HasPrefix(name, ".") || strings.Contains(name, "/.") {
		return false
	}
	info, err := fs.Stat(h.files, name)
	return err == nil && info.Mode().IsRegular()
}

func (h *handler) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	cache := cacheRevalidate
	if strings.HasPrefix(name, assetsDir+"/") {
		cache = cacheImmutable
	}
	w.Header().Set("Cache-Control", cache)
	if etag, ok := h.etags[name]; ok {
		w.Header().Set("ETag", etag) // ServeFileFS answers If-None-Match with 304
	}
	if path.Ext(name) == ".html" {
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	}
	// ServeFileFS sets Content-Type from the extension (sniffing the content
	// otherwise), answers HEAD and Range, and redirects /index.html to ./.
	http.ServeFileFS(w, r, h.files, name)
}
