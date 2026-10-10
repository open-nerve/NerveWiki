package httpadapter_test

import (
	"fmt"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
)

// archivePolicy is every download's Content-Security-Policy.
const archivePolicy = "sandbox; default-src 'none'"

// fetch sends method path without a body, as send does.
func (h *harness) fetch(t *testing.T, method, path, token string, headers ...string) (*http.Response, []byte) {
	t.Helper()
	return h.send(t, method, path, token, "", headers...)
}

// address is the signed address of job's archive, as the reads write it:
// expiring with it, the TTL after it ended.
func (h *harness) address(j domain.Job) string {
	until := h.signedAt.Add(exportTTL)
	if j.Finished != nil {
		until = j.Finished.Add(exportTTL)
	}
	return httpadapter.DownloadURL(j.ID, h.signer.Sign(h.signedAt, j.ID, until))
}

// Every address the reads give downloads, in an export's last hour too:
// the address expires with the export, its signature signing the expiry it
// carries.
func TestTheReadsAddressesDownload(t *testing.T) {
	h := newHarness(t)
	fresh := h.job(alice(), domain.StateSucceeded, now().Add(-time.Hour), "fresh")
	ending := h.job(alice(), domain.StateSucceeded, now().Add(-exportTTL+29*time.Minute), "ending")
	for name, j := range map[string]domain.Job{"fresh": fresh, "ending": ending} {
		_, body := h.send(t, http.MethodGet, "/api/v0/transfer-jobs/"+j.ID.String(), "session", "")
		var a jobAnswer
		decode(t, body, &a)
		if a.Download == nil {
			t.Errorf("%s: getTransferJob = %s, want its address", name, body)
			continue
		}
		res, got := h.fetch(t, http.MethodGet, a.Download.URL, "")
		if res.StatusCode != http.StatusOK || string(got) != name {
			t.Errorf("%s: download = %d %q, want 200 its archive", name, res.StatusCode, got)
		}
	}
	waitFor(t, func() bool { return h.archives.unclosed() == 0 })
}

// An export's archive downloads without a token as a zip, an attachment
// named after the export, sandboxed, not stored, its time the file's.
func TestDownloadServesTheArchive(t *testing.T) {
	h := newHarness(t)
	j := h.job(alice(), domain.StateSucceeded, now(), "PK-the-archive")
	h.rows.jobs[j.ID].Name = "Café notes"
	res, body := h.fetch(t, http.MethodGet, h.address(j), "")
	if res.StatusCode != http.StatusOK || string(body) != "PK-the-archive" {
		t.Fatalf("download = %d %q, want 200 the archive", res.StatusCode, body)
	}
	for header, want := range map[string]string{
		"Content-Type":            "application/zip",
		"Content-Disposition":     `attachment; filename="Caf_ notes.zip"; filename*=UTF-8''Caf%C3%A9%20notes.zip`,
		"Content-Security-Policy": archivePolicy,
		"Cache-Control":           "no-store",
		"Last-Modified":           now().Add(-time.Hour).Format(http.TimeFormat),
		"Accept-Ranges":           "bytes",
	} {
		if got := res.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	waitFor(t, func() bool { return h.archives.unclosed() == 0 })
}

// The address is read as the server writes it: anything else is
// not_found, sandboxed, not stored, without the archive's headers, and no
// file is opened.
func TestDownloadReadsItsAddressStrictly(t *testing.T) {
	h := newHarness(t)
	j := h.job(alice(), domain.StateSucceeded, now(), "zip")
	other := h.job(bob(), domain.StateSucceeded, now(), "zip")
	good := h.address(j)
	path, query, _ := strings.Cut(good, "?")
	e, s, _ := strings.Cut(strings.TrimPrefix(query, "e="), "&s=")
	id := j.ID.String()
	for _, tt := range []struct{ name, path, query string }{
		{"nothing", "", ""},
		{"no e", "", "s=" + s},
		{"no s", "", "e=" + e},
		{"s before e", "", "s=" + s + "&e=" + e},
		{"e twice", "", query + "&e=" + e},
		{"s twice", "", query + "&s=" + s},
		{"another member", "", query + "&x=1"},
		{"another member first", "", "x=1&" + query},
		{"an empty member", "", query + "&"},
		{"other keys, the values in place", "", "x=" + e + "&y=" + s},
		{"e with a leading zero", "", "e=0" + e + "&s=" + s},
		{"e with a sign", "", "e=%2B" + e + "&s=" + s},
		{"e of zero", "", "e=0&s=" + s},
		{"e not a number", "", "e=x&s=" + s},
		{"a later e", "", "e=" + e + "0&s=" + s},
		{"an earlier e", "", "e=" + strconv.Itoa(mustAtoi(t, e)-3600) + "&s=" + s},
		{"s cut short", "", "e=" + e + "&s=" + s[1:]},
		{"s one longer", "", "e=" + e + "&s=" + s + "A"},
		{"s padded", "", "e=" + e + "&s=" + s[:21] + "="},
		{"s escaped", "", "e=" + e + "&s=" + "%" + fmt.Sprintf("%02X", s[0]) + s[1:]},
		{"s of another address", "", "e=" + e + "&s=" + strings.Repeat("A", 22)},
		{"another job's address", other.ID.String(), query},
		{"the path's id in upper case", strings.ToUpper(id), query},
		{"the path's id without hyphens", strings.ReplaceAll(id, "-", ""), query},
		{"the path's id escaped", "%" + fmt.Sprintf("%02X", id[0]) + id[1:], query},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := path
			if tt.path != "" {
				p = "/api/v0/transfer-jobs/" + tt.path + "/download"
			}
			res, body := h.fetch(t, http.MethodGet, p+"?"+tt.query, "")
			if res.StatusCode != http.StatusNotFound || code(body) != "not_found" || res.Header.Get("Cache-Control") != "no-store" ||
				res.Header.Get("Content-Disposition") != "" || res.Header.Get("Content-Security-Policy") != archivePolicy {
				t.Errorf("download = %d %s, headers %v; want 404 not_found, no-store, sandboxed", res.StatusCode, body, res.Header)
			}
		})
	}
	if h.archives.unclosed() != 0 {
		t.Errorf("%d archives left open", h.archives.unclosed())
	}
	if res, body := h.fetch(t, http.MethodGet, good, ""); res.StatusCode != http.StatusOK || string(body) != "zip" {
		t.Errorf("the address signed = %d %q, want 200 the archive", res.StatusCode, body)
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// An address signed in one hour is the same all that hour, and lasts until
// the end of the next: then it is not_found.
func TestDownloadAddressLastsUntilTheNextHourEnds(t *testing.T) {
	h := newHarness(t)
	j := h.job(alice(), domain.StateSucceeded, now(), "zip")
	h.signedAt = time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	early := h.address(j)
	h.signedAt = time.Date(2026, 10, 9, 10, 59, 59, 0, time.UTC)
	if late := h.address(j); late != early {
		t.Errorf("signed at 10:00 %s, at 10:59:59 %s; want the same", early, late)
	}
	if res, _ := h.fetch(t, http.MethodGet, early, ""); res.StatusCode != http.StatusOK {
		t.Errorf("the hour's address at 10:30 = %d, want 200", res.StatusCode)
	}
	h.signedAt = now().Add(-2 * time.Hour) // what is signed then expires by now
	if res, body := h.fetch(t, http.MethodGet, h.address(j), ""); res.StatusCode != http.StatusNotFound || code(body) != "not_found" {
		t.Errorf("an address expired = %d %s, want 404 not_found", res.StatusCode, body)
	}
}

// Only an export that succeeded, not deleted, its file in the store,
// downloads; anything else at a signed address is not_found, alike.
func TestDownloadOfAJobWithoutAnArchiveIsNotFound(t *testing.T) {
	h := newHarness(t)
	missing := h.job(alice(), domain.StateSucceeded, now(), "")
	for _, tt := range []struct {
		name string
		job  domain.Job
	}{
		{"queued", h.job(alice(), domain.StateQueued, now(), "")},
		{"running", h.job(bob(), domain.StateRunning, now(), "")},
		{"failed", h.job(alice(), domain.StateFailed, now(), "")},
		{"expired", h.job(alice(), domain.StateExpired, now(), "zip")},
		{"its file missing", missing},
		{"an import", func() domain.Job {
			j := h.job(alice(), domain.StateSucceeded, now(), "zip")
			h.rows.jobs[j.ID].Kind = domain.KindImport
			return j
		}()},
		{"deleted", func() domain.Job {
			j := h.job(alice(), domain.StateSucceeded, now(), "zip")
			delete(h.rows.jobs, j.ID)
			return j
		}()},
	} {
		if res, body := h.fetch(t, http.MethodGet, h.address(tt.job), ""); res.StatusCode != http.StatusNotFound || code(body) != "not_found" {
			t.Errorf("%s: download = %d %s, want 404 not_found", tt.name, res.StatusCode, body)
		}
	}
	if logs := h.logs.String(); !strings.Contains(logs, `msg="export archive missing" job_id=`+missing.ID.String()) {
		t.Errorf("logs %q, want the missing archive", logs)
	}
}

// A range is 206; several ranges are passed by, the archive served whole
// (M7 closeout A-N1); a range outside the archive 416, sandboxed still; a
// condition on a change is passed by, the archive served; a copy as new as
// the file 304; HEAD answers the headers alone.
func TestDownloadAnswersRanges(t *testing.T) {
	h := newHarness(t)
	j := h.job(alice(), domain.StateSucceeded, now(), "abcdef")
	u := h.address(j)
	if res, body := h.fetch(t, http.MethodGet, u, "", "Range", "bytes=1-2"); res.StatusCode != http.StatusPartialContent ||
		string(body) != "bc" || res.Header.Get("Content-Range") != "bytes 1-2/6" {
		t.Errorf("a range = %d %q %q, want 206 bc bytes 1-2/6", res.StatusCode, body, res.Header.Get("Content-Range"))
	}
	if res, body := h.fetch(t, http.MethodGet, u, "", "Range", "bytes=0-1,3-4"); res.StatusCode != http.StatusOK || string(body) != "abcdef" {
		t.Errorf("two ranges = %d %q, want 200 the archive", res.StatusCode, body)
	}
	if res, _ := h.fetch(t, http.MethodGet, u, "", "Range", "bytes=10-20"); res.StatusCode != http.StatusRequestedRangeNotSatisfiable ||
		res.Header.Get("Content-Range") != "bytes */6" || res.Header.Get("Content-Security-Policy") != archivePolicy {
		t.Errorf("a range outside = %d %q, CSP %q; want 416 bytes */6, sandboxed", res.StatusCode, res.Header.Get("Content-Range"),
			res.Header.Get("Content-Security-Policy"))
	}
	for _, c := range [][2]string{{"If-Match", `"other"`}, {"If-Unmodified-Since", "Mon, 01 Jan 2001 00:00:00 GMT"}} {
		if res, body := h.fetch(t, http.MethodGet, u, "", c[0], c[1]); res.StatusCode != http.StatusOK || string(body) != "abcdef" {
			t.Errorf("%s %s = %d %q, want 200 the archive", c[0], c[1], res.StatusCode, body)
		}
	}
	modified := now().Add(-time.Hour).Format(http.TimeFormat)
	if res, body := h.fetch(t, http.MethodGet, u, "", "If-Modified-Since", modified); res.StatusCode != http.StatusNotModified || len(body) != 0 {
		t.Errorf("a copy as new as the file = %d %q, want 304", res.StatusCode, body)
	}
	// No ETag: If-None-Match matches as * alone, and sent, it overrides
	// If-Modified-Since; If-Range is a time.
	for _, tt := range []struct {
		name    string
		headers []string
		status  int
		body    string
	}{
		{"If-None-Match *", []string{"If-None-Match", "*"}, http.StatusNotModified, ""},
		{"If-None-Match a tag", []string{"If-None-Match", `"x"`, "If-Modified-Since", modified}, http.StatusOK, "abcdef"},
		{"If-Range its time", []string{"If-Range", modified, "Range", "bytes=1-2"}, http.StatusPartialContent, "bc"},
		{"If-Range a tag", []string{"If-Range", `"x"`, "Range", "bytes=1-2"}, http.StatusOK, "abcdef"},
	} {
		if res, body := h.fetch(t, http.MethodGet, u, "", tt.headers...); res.StatusCode != tt.status || string(body) != tt.body {
			t.Errorf("%s = %d %q, want %d %q", tt.name, res.StatusCode, body, tt.status, tt.body)
		}
	}
	if res, body := h.fetch(t, http.MethodHead, u, ""); res.StatusCode != http.StatusOK || len(body) != 0 || res.Header.Get("Content-Length") != "6" {
		t.Errorf("HEAD = %d %q, length %q; want 200, no body, 6", res.StatusCode, body, res.Header.Get("Content-Length"))
	}
	waitFor(t, func() bool { return h.archives.unclosed() == 0 })
}

// The download is public: it takes the platform's anonymous bucket by the
// client's IP, a token or none, and past it is rate_limited, sandboxed.
func TestDownloadTakesTheAnonymousBucket(t *testing.T) {
	anonymous, authenticated := &bucket{left: 1}, &bucket{left: 1000}
	h := newHarnessWith(t, httpservertest.APIOptions{Anonymous: anonymous, Authenticated: authenticated})
	j := h.job(alice(), domain.StateSucceeded, now(), "zip")
	if res, _ := h.fetch(t, http.MethodGet, h.address(j), "session"); res.StatusCode != http.StatusOK {
		t.Fatalf("the first download = %d, want 200", res.StatusCode)
	}
	res, body := h.fetch(t, http.MethodGet, h.address(j), "")
	if res.StatusCode != http.StatusTooManyRequests || code(body) != "rate_limited" || !slices.Equal(anonymous.keys, []string{"127.0.0.1", "127.0.0.1"}) ||
		len(authenticated.keys) != 0 || res.Header.Get("Content-Security-Policy") != archivePolicy {
		t.Errorf("the second = %d %s, anonymous %q, authenticated %q; want 429 rate_limited by the client's IP alone", res.StatusCode, body,
			anonymous.keys, authenticated.keys)
	}
}

// A download whose archive opens once the server began shutting down is
// server_busy after 5 s, no-store, sandboxed, without the archive's
// headers. The stream learns of the shutdown a moment after the listener
// closes, on goroutines of its own: the job's row is read a while after.
func TestDownloadAsTheServerShutsDownIsServerBusy(t *testing.T) {
	h := newHarness(t)
	j := h.job(alice(), domain.StateSucceeded, now(), "zip")
	h.rows.gate, h.rows.asked = make(chan struct{}), make(chan struct{}, 1)
	stop, done := h.serve(t)
	answered := make(chan *http.Response, 1)
	go func() {
		res, _ := h.fetch(t, http.MethodGet, h.address(j), "")
		answered <- res
	}()
	<-h.rows.asked
	stop()
	addr := strings.TrimPrefix(h.base, "http://")
	waitFor(t, func() bool {
		c, err := net.Dial("tcp", addr)
		if err == nil {
			_ = c.Close()
		}
		return err != nil
	})
	time.Sleep(200 * time.Millisecond)
	close(h.rows.gate)
	select {
	case res := <-answered:
		if res.StatusCode != http.StatusServiceUnavailable || res.Header.Get("Retry-After") != "5" ||
			res.Header.Get("Content-Security-Policy") != archivePolicy || res.Header.Get("Cache-Control") != "no-store" ||
			res.Header.Get("Content-Disposition") != "" {
			t.Errorf("download = %d, headers %v; want 503 after 5 s, sandboxed, no-store, none of the archive's", res.StatusCode, res.Header)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the download did not answer")
	}
	<-done
	waitFor(t, func() bool { return h.archives.unclosed() == 0 })
}

// A step past the request's deadline is answered 500 and logged as a
// request that ran out of time, a warning, not as a fault.
func TestDownloadPastItsDeadlineIsLoggedAsAWarning(t *testing.T) {
	h := newHarnessWith(t, httpservertest.APIOptions{RequestTimeout: 100 * time.Millisecond})
	j := h.job(alice(), domain.StateSucceeded, now(), "zip")
	h.rows.gate, h.rows.asked = make(chan struct{}), make(chan struct{}, 1)
	defer close(h.rows.gate)
	res, body := h.fetch(t, http.MethodGet, h.address(j), "")
	if res.StatusCode != http.StatusInternalServerError || code(body) != "internal_error" {
		t.Errorf("download = %d %s, want 500 internal_error", res.StatusCode, body)
	}
	if logs := h.logs.String(); !strings.Contains(logs, `level=WARN msg="API request deadline exceeded"`) ||
		strings.Contains(logs, "level=ERROR") {
		t.Errorf("logs %q, want the deadline as a warning, no error", logs)
	}
}
