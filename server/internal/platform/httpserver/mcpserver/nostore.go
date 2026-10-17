package mcpserver

import "net/http"

// noStore keeps Cache-Control: no-store on every answer of the route: the
// SDK sets no-cache, no-transform on the answers that carry a result
// (sdk/mcp/streamable.go:1686), and a tool's result is the notebook's.
// wrote tells ServeHTTP whether anything answered.
type noStore struct {
	http.ResponseWriter
	wrote bool
}

// WriteHeader sets no-store over what the SDK set, then writes the header.
func (w *noStore) WriteHeader(code int) {
	w.wrote = true
	w.Header().Set("Cache-Control", "no-store")
	w.ResponseWriter.WriteHeader(code)
}

// Write writes the header 200 first, as net/http would, but no-store.
func (w *noStore) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Flush flushes the underlying writer, the header written first: the SDK
// flushes an event stream's header through http.ResponseController.
func (w *noStore) Flush() {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap is for http.ResponseController and the platform's responseStarted.
func (w *noStore) Unwrap() http.ResponseWriter { return w.ResponseWriter }
