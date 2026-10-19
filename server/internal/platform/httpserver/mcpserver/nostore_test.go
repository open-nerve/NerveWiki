package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The writer keeps no-store however the header goes out: WriteHeader, a
// first Write, a Flush, and over what the handler set; it unwraps to the
// writer below, and tells whether anything answered.
func TestNoStoreKeepsItsHeader(t *testing.T) {
	tests := []struct {
		name  string
		write func(w http.ResponseWriter)
	}{
		{"WriteHeader", func(w http.ResponseWriter) { w.WriteHeader(http.StatusAccepted) }},
		{"a Write alone", func(w http.ResponseWriter) { _, _ = w.Write([]byte("{}")) }},
		{"the SDK's header first", func(w http.ResponseWriter) {
			w.Header().Set("Cache-Control", "no-cache, no-transform")
			w.WriteHeader(http.StatusOK)
		}},
		{"a Flush", func(w http.ResponseWriter) { _ = http.NewResponseController(w).Flush() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ns := &noStore{ResponseWriter: rec}
			tt.write(ns)
			if got := rec.Result().Header.Get("Cache-Control"); got != "no-store" || !ns.wrote {
				t.Errorf("Cache-Control = %q, wrote %v; want no-store, written", got, ns.wrote)
			}
		})
	}
	rec := httptest.NewRecorder()
	ns := &noStore{ResponseWriter: rec}
	ns.Flush()
	if !rec.Flushed || ns.Unwrap() != rec {
		t.Errorf("flushed %v, unwraps to %v; want the recorder flushed, and it", rec.Flushed, ns.Unwrap())
	}
	if (&noStore{ResponseWriter: httptest.NewRecorder()}).wrote {
		t.Error("a writer nothing went through says it wrote")
	}
}

// Every answer of the route is no-store: the SDK's results, its 202 and
// its own refusals, mcpserver's 403 and 404, the edge's 401, 413 and 400,
// and the 404s of the paths below it and of /.well-known/. The SDK's
// handler alone rewrites a result's to no-cache, no-transform: the writer
// is what keeps it.
func TestEveryAnswerIsNotStored(t *testing.T) {
	s := serveMCP(t, setup{})
	nb := Pattern + "?notebook=" + visible.String()
	tests := []struct {
		name, method, path, token, body string
		header                          http.Header
		status                          int
	}{
		{"a tool's result", http.MethodPost, nb, patToken, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo"}}`, v2025(), http.StatusOK},
		{"a handshake", http.MethodPost, Pattern, patToken, initialize, nil, http.StatusOK},
		{"a notification", http.MethodPost, Pattern, patToken, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, v2025(), http.StatusAccepted},
		{"a protocol the SDK refuses", http.MethodPost, Pattern, patToken, initialize, http.Header{"Mcp-Protocol-Version": {"1999-01-01"}}, http.StatusBadRequest},
		{"a GET", http.MethodGet, Pattern, patToken, "", nil, http.StatusMethodNotAllowed},
		{"another media type", http.MethodPost, Pattern, patToken, initialize, http.Header{"Content-Type": {"text/plain"}}, http.StatusUnsupportedMediaType},
		{"no token", http.MethodPost, Pattern, "", initialize, nil, http.StatusUnauthorized},
		{"a session's access token", http.MethodPost, Pattern, sessionToken, initialize, nil, http.StatusForbidden},
		{"a notebook alice cannot see", http.MethodPost, Pattern + "?notebook=nope", patToken, initialize, nil, http.StatusNotFound},
		{"a batch", http.MethodPost, Pattern, patToken, `[` + initialize + `]`, nil, http.StatusBadRequest},
		{"a ping over its bound", http.MethodPost, Pattern, patToken, pingOf(MaxOtherBytes + 1), v2025(), http.StatusRequestEntityTooLarge},
		{"a path below the route", http.MethodGet, Pattern + "/x", "", "", nil, http.StatusNotFound},
		{"OAuth's metadata", http.MethodGet, "/.well-known/oauth-protected-resource", "", "", nil, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := s.send(t, context.Background(), tt.method, tt.path, tt.token, tt.body, tt.header)
			if a.status != tt.status || a.header.Get("Cache-Control") != "no-store" {
				t.Errorf("%s %s = %d Cache-Control %q, want %d no-store", tt.method, tt.path, a.status, a.header.Get("Cache-Control"), tt.status)
			}
		})
	}
	// The same SDK handler, without the writer.
	bare := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := &request{ctx: r.Context(), read: time.Now(), cred: Credential{UserID: alice, TokenID: aliceTok}, entry: AccountEntry}
		s.srv.handler.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestKey{}, req)))
	}))
	defer bare.Close()
	direct := &served{base: bare.URL}
	a := direct.post(t, context.Background(), "/", "", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"notebook":"x"}}}`, v2025())
	if a.status != http.StatusOK || a.header.Get("Cache-Control") != "no-cache, no-transform" {
		t.Errorf("the SDK's handler alone = %d Cache-Control %q, want 200 no-cache, no-transform", a.status, a.header.Get("Cache-Control"))
	}
}
