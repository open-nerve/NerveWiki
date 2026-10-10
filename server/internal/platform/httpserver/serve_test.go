package httpserver_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// A range is 206; a Range of several ranges, or a condition on a change,
// is passed by: the whole file, 200.
func TestServeFixed(t *testing.T) {
	modified := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name, header, value string
		status              int
		body, contentRange  string
	}{
		{"a range", "Range", "bytes=1-2", http.StatusPartialContent, "bc", "bytes 1-2/6"},
		{"a range from the end", "Range", "bytes=-2", http.StatusPartialContent, "ef", "bytes 4-5/6"},
		{"two ranges", "Range", "bytes=0-1,3-4", http.StatusOK, "abcdef", ""},
		{"many ranges", "Range", "bytes=" + strings.Repeat("0-0,", 1000) + "0-0", http.StatusOK, "abcdef", ""},
		{"a range outside", "Range", "bytes=10-20", http.StatusRequestedRangeNotSatisfiable, "", "bytes */6"},
		{"If-Match", "If-Match", `"other"`, http.StatusOK, "abcdef", ""},
		{"If-Unmodified-Since", "If-Unmodified-Since", modified.Add(-time.Hour).Format(http.TimeFormat), http.StatusOK, "abcdef", ""},
		{"If-Modified-Since", "If-Modified-Since", modified.Format(http.TimeFormat), http.StatusNotModified, "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/f", nil)
			r.Header.Set(tt.header, tt.value)
			w := httptest.NewRecorder()
			httpserver.ServeFixed(w, r, modified, strings.NewReader("abcdef"))
			body, _ := io.ReadAll(w.Result().Body)
			if w.Code != tt.status || tt.status != http.StatusRequestedRangeNotSatisfiable && string(body) != tt.body ||
				w.Header().Get("Content-Range") != tt.contentRange {
				t.Errorf("= %d %q, Content-Range %q; want %d %q, %q", w.Code, body, w.Header().Get("Content-Range"), tt.status, tt.body,
					tt.contentRange)
			}
		})
	}
}
