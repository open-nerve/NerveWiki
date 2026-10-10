package httpadapter_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
)

// contentPolicy is every content's Content-Security-Policy.
const contentPolicy = "sandbox; default-src 'none'; img-src 'self' data:; media-src 'self'; style-src 'unsafe-inline'"

// attach puts an attachment named name at notebookID()'s root, its file
// of type mime holding content, as an upload leaves it.
func (h *harness) attach(name, mime, content string) (app.Node, domain.Blob) {
	n := app.Node{ID: uuid.NewV7(), NotebookID: notebookID(), Asset: true, Name: name, NameKey: strings.ToLower(name), CreatedBy: alice(),
		CreatedAt: now()}
	sum := sha256.Sum256([]byte(content))
	b := domain.Blob{ID: uuid.NewV7(), NodeID: n.ID, NotebookID: n.NotebookID, MIME: mime, Bytes: int64(len(content)), SHA256: sum[:],
		CreatedBy: alice(), CreatedAt: now()}
	h.nodes.add(n)
	h.rows.rows[n.ID] = b
	h.files.files[domain.Key(b.ID)] = []byte(content)
	return n, b
}

// address is the signed address of b's content, shown or downloaded.
func (h *harness) address(n app.Node, b domain.Blob, download bool) string {
	s := h.signer.Sign(h.signedAt, n.ID, b.ID)
	u := "/api/v0/assets/" + n.ID.String() + "/content?b=" + b.ID.String() + "&e=" + strconv.FormatInt(s.Expires.Unix(), 10)
	if download {
		return u + "&s=" + s.Download + "&d=1"
	}
	return u + "&s=" + s.Inline
}

// get sends method path with token, none when empty, and the headers,
// checks the answer against the contract, and answers it with its body. A
// HEAD is not checked: the router answers it on every GET route, which
// the contract describes as GET alone.
func (h *harness) get(t *testing.T, method, path, token string, headers ...string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, h.base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	res.Body = io.NopCloser(bytes.NewReader(body))
	if method != http.MethodHead {
		h.contract.CheckResponse(t, req, res)
	}
	return res, body
}

// Each type is served as the table says, without a token: shown or
// downloaded, under its name, sandboxed, cached privately until the
// address expires (90 minutes at 10:30), its SHA-256 its ETag.
func TestContentServesEachTypeWithItsHeaders(t *testing.T) {
	for _, tt := range []struct {
		name, mime  string
		download    bool
		disposition string
	}{
		{"a.png", "image/png", false, "inline"},
		{"a.svg", "image/svg+xml", false, "inline"},
		{"a.pdf", "application/pdf", false, "inline"},
		{"a.mp4", "video/mp4", false, "inline"},
		{"a.zip", "application/octet-stream", false, "attachment"},
		{"a.png", "image/png", true, "attachment"},
	} {
		t.Run(tt.name+" downloaded "+strconv.FormatBool(tt.download), func(t *testing.T) {
			h := newHarness(t)
			n, b := h.attach(tt.name, tt.mime, "the bytes")
			res, body := h.get(t, http.MethodGet, h.address(n, b, tt.download), "")
			want := map[string]string{
				"Content-Type":                 tt.mime,
				"Content-Disposition":          tt.disposition + `; filename="` + tt.name + `"; filename*=UTF-8''` + tt.name,
				"Content-Security-Policy":      contentPolicy,
				"Cache-Control":                "private, max-age=5400, immutable",
				"ETag":                         `"` + hex.EncodeToString(b.SHA256) + `"`,
				"Cross-Origin-Resource-Policy": "same-origin",
				"X-Content-Type-Options":       "nosniff",
			}
			if res.StatusCode != http.StatusOK || string(body) != "the bytes" {
				t.Errorf("download = %d %q, want 200 the bytes", res.StatusCode, body)
			}
			for k, v := range want {
				if got := res.Header.Get(k); got != v {
					t.Errorf("%s = %q, want %q", k, got, v)
				}
			}
		})
	}
}

// A name of other characters than ASCII's is written twice: in ASCII, each
// other character, a quote or a backslash an underscore; and in UTF-8,
// escaped.
func TestContentNamesTheFileForEveryBrowser(t *testing.T) {
	h := newHarness(t)
	n, b := h.attach(`报告 "v1"\x.pdf`, "application/pdf", "%PDF")
	res, _ := h.get(t, http.MethodGet, h.address(n, b, true), "")
	want := `attachment; filename="__ _v1__x.pdf"; filename*=UTF-8''%E6%8A%A5%E5%91%8A%20%22v1%22%5Cx.pdf`
	if got := res.Header.Get("Content-Disposition"); got != want {
		t.Errorf("Content-Disposition = %s, want %s", got, want)
	}
}

// Any query but the one signed, as the server writes it, is not_found,
// no-store, alike, sandboxed as any answer; so is a path whose id is
// spelled otherwise, and an address of another node or one expired. A node
// id that is no id is 400, as any operation's parameter.
func TestContentReadsItsAddressStrictly(t *testing.T) {
	h := newHarness(t)
	n, b := h.attach("a.png", "image/png", "abc")
	good := h.address(n, b, false)
	path, query, _ := strings.Cut(good, "?")
	q := map[string]string{}
	for part := range strings.SplitSeq(query, "&") {
		k, v, _ := strings.Cut(part, "=")
		q[k] = v
	}
	other, _ := h.attach("b.png", "image/png", "abc")
	escaped := "%" + fmt.Sprintf("%02X", q["b"][0]) + q["b"][1:]
	if u, err := url.QueryUnescape(escaped); err != nil || u != q["b"] {
		t.Fatalf("b escaped = %q, which unescapes to %q, %v; want b", escaped, u, err)
	}
	download := h.address(n, b, true)
	_, downloadQuery, _ := strings.Cut(download, "?")
	ds := strings.TrimSuffix(downloadQuery[strings.Index(downloadQuery, "&s=")+3:], "&d=1")
	for _, tt := range []struct{ name, path, query string }{
		{"no b", "", "e=" + q["e"] + "&s=" + q["s"]},
		{"no e", "", "b=" + q["b"] + "&s=" + q["s"]},
		{"no s", "", "b=" + q["b"] + "&e=" + q["e"]},
		{"nothing", "", ""},
		{"b in upper case", "", "b=" + strings.ToUpper(q["b"]) + "&e=" + q["e"] + "&s=" + q["s"]},
		{"b without hyphens", "", "b=" + strings.ReplaceAll(q["b"], "-", "") + "&e=" + q["e"] + "&s=" + q["s"]},
		{"b escaped", "", "b=" + escaped + "&e=" + q["e"] + "&s=" + q["s"]},
		{"e with a leading zero", "", "b=" + q["b"] + "&e=0" + q["e"] + "&s=" + q["s"]},
		{"e with a sign", "", "b=" + q["b"] + "&e=%2B" + q["e"] + "&s=" + q["s"]},
		{"s cut short", "", "b=" + q["b"] + "&e=" + q["e"] + "&s=" + q["s"][1:]},
		{"s one longer", "", "b=" + q["b"] + "&e=" + q["e"] + "&s=" + q["s"] + "A"},
		{"s padded", "", "b=" + q["b"] + "&e=" + q["e"] + "&s=" + q["s"][:21] + "="},
		{"s of another address", "", "b=" + q["b"] + "&e=" + q["e"] + "&s=" + strings.Repeat("A", 22)},
		{"d=0", "", query + "&d=0"},
		{"d=0 on the download's signature", "", "b=" + q["b"] + "&e=" + q["e"] + "&s=" + ds + "&d=0"},
		{"d empty on the download's signature", "", "b=" + q["b"] + "&e=" + q["e"] + "&s=" + ds + "&d="},
		{"d=1 on the shown signature", "", query + "&d=1"},
		{"b twice", "", query + "&b=" + q["b"]},
		{"another member", "", query + "&x=1"},
		{"an empty member", "", query + "&"},
		{"a member without a value", "", query + "&d"},
		{"a later e", "", "b=" + q["b"] + "&e=" + q["e"] + "0&s=" + q["s"]},
		{"in another order", "", "e=" + q["e"] + "&b=" + q["b"] + "&s=" + q["s"]},
		{"other keys, the values in place", "", "x=" + q["b"] + "&y=" + q["e"] + "&z=" + q["s"]},
		{"the keys swapped, the values in place", "", "e=" + q["b"] + "&b=" + q["e"] + "&s=" + q["s"]},
		{"d twice", "", downloadQuery + "&d=1"},
		{"a member after d", "", downloadQuery + "&x=1"},
		{"d twice on the shown signature", "", query + "&d=1&d=1"},
		{"a member after d on the shown signature", "", query + "&d=1&x=1"},
		{"longer keys, the values in place", "", "bb=" + q["b"] + "&ee=" + q["e"] + "&ss=" + q["s"]},
		{"another key for d", "", "b=" + q["b"] + "&e=" + q["e"] + "&s=" + ds + "&x=1"},
		{"d before s", "", "b=" + q["b"] + "&e=" + q["e"] + "&d=1&s=" + ds},
		{"the path's id in upper case", strings.ToUpper(n.ID.String()), query},
		{"the path's id without hyphens", strings.ReplaceAll(n.ID.String(), "-", ""), query},
		{"the path's id escaped", "%" + fmt.Sprintf("%02X", n.ID.String()[0]) + n.ID.String()[1:], query},
		{"another node", other.ID.String(), query},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := path
			if tt.path != "" {
				p = "/api/v0/assets/" + tt.path + "/content"
			}
			res, body := h.get(t, http.MethodGet, p+"?"+tt.query, "")
			if res.StatusCode != http.StatusNotFound || code(body) != "not_found" || res.Header.Get("Cache-Control") != "no-store" ||
				res.Header.Get("Content-Disposition") != "" || !isSandboxed(res) {
				t.Errorf("download = %d %s, Cache-Control %q, CSP %q; want 404 not_found, no-store, sandboxed", res.StatusCode, body,
					res.Header.Get("Cache-Control"), res.Header.Get("Content-Security-Policy"))
			}
		})
	}
	for _, u := range []string{good, download} {
		if res, _ := h.get(t, http.MethodGet, u, ""); res.StatusCode != http.StatusOK {
			t.Errorf("the address signed %s = %d, want 200", u, res.StatusCode)
		}
	}
	res, body := h.get(t, http.MethodGet, "/api/v0/assets/nope/content?"+query, "")
	var p problem
	_ = json.Unmarshal(body, &p)
	if res.StatusCode != http.StatusBadRequest || len(p.Errors) != 1 || p.Errors[0].Field != "node_id" ||
		p.Errors[0].Code != "invalid_format" || !isSandboxed(res) {
		t.Errorf("a node id that is no id = %d %s, CSP %q; want 400, invalid_format on node_id, sandboxed", res.StatusCode, body,
			res.Header.Get("Content-Security-Policy"))
	}
}

// isSandboxed reports whether res carries the contents' policy and their
// resource policy.
func isSandboxed(res *http.Response) bool {
	return res.Header.Get("Content-Security-Policy") == contentPolicy && res.Header.Get("Cross-Origin-Resource-Policy") == "same-origin"
}

// A signature is spelled with 22 base64url characters, whatever they are,
// and nothing else.
func TestSignatureIsSpelledInBase64URL(t *testing.T) {
	for _, tt := range []struct {
		s    string
		want bool
	}{
		{"AZaz09-_AZaz09-_AZaz09", true},
		{"ZZZZZZZZZZZZZZZZZZZZZZ", true},
		{"zzzzzzzzzzzzzzzzzzzzzz", true},
		{"9999999999999999999999", true},
		{"----------------------", true},
		{"______________________", true},
		{"AZaz09-_AZaz09-_AZaz0", false},
		{"AZaz09-_AZaz09-_AZaz09A", false},
		{"AZaz09+/AZaz09+/AZaz09", false},
		{"AZaz09-_AZaz09-_AZaz0=", false},
		{"AZaz09-_AZaz09-_AZaz0.", false},
		{"AZaz09-_AZaz09-_AZazé", false}, // 22 bytes
	} {
		if got := httpadapter.Signature(tt.s); got != tt.want {
			t.Errorf("Signature(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
	for _, c := range "@[`{/:" { // beside the ranges
		if s := "AZaz09-_AZaz09-_AZaz0" + string(c); httpadapter.Signature(s) {
			t.Errorf("Signature(%q) = true, want false", s)
		}
	}
}

// code is a problem's code.
func code(body []byte) string {
	var p problem
	_ = json.Unmarshal(body, &p)
	return p.Code
}

// An address expired is not_found.
func TestContentOfAnAddressExpiredIsNotFound(t *testing.T) {
	h := newHarness(t)
	n, b := h.attach("a.png", "image/png", "abc")
	h.signedAt = now().Add(-2 * time.Hour) // what is signed then expires by now
	res, body := h.get(t, http.MethodGet, h.address(n, b, false), "")
	if res.StatusCode != http.StatusNotFound || code(body) != "not_found" {
		t.Errorf("download = %d %s, want 404 not_found", res.StatusCode, body)
	}
}

// A range is 206; several ranges are passed by, the file served whole
// (M7 closeout A-N1); a copy the
// ETag names, or one as new as the file's Last-Modified, 304; a range
// outside the file 416, sandboxed still; a condition on a change is passed
// by, the file served; HEAD answers the headers alone. Each closes the
// file it opened.
func TestContentAnswersRangesAndConditionalRequests(t *testing.T) {
	h := newHarness(t)
	n, b := h.attach("a.png", "image/png", "abcdef")
	u := h.address(n, b, false)
	etag := `"` + hex.EncodeToString(b.SHA256) + `"`
	if res, body := h.get(t, http.MethodGet, u, "", "Range", "bytes=1-2"); res.StatusCode != http.StatusPartialContent ||
		string(body) != "bc" || res.Header.Get("Content-Range") != "bytes 1-2/6" {
		t.Errorf("a range = %d %q %q, want 206 bc bytes 1-2/6", res.StatusCode, body, res.Header.Get("Content-Range"))
	}
	if res, body := h.get(t, http.MethodGet, u, "", "Range", "bytes=0-1,3-4"); res.StatusCode != http.StatusOK || string(body) != "abcdef" ||
		res.Header.Get("Content-Type") != "image/png" {
		t.Errorf("two ranges = %d %q %q, want 200 the file, image/png", res.StatusCode, body, res.Header.Get("Content-Type"))
	}
	if res, body := h.get(t, http.MethodGet, u, "", "If-None-Match", etag); res.StatusCode != http.StatusNotModified || len(body) != 0 {
		t.Errorf("a copy the ETag names = %d %q, want 304", res.StatusCode, body)
	}
	res, _ := h.get(t, http.MethodGet, u, "")
	if modified := res.Header.Get("Last-Modified"); modified != now().Format(http.TimeFormat) {
		t.Errorf("Last-Modified = %q, want the file's time", modified)
	} else if res, body := h.get(t, http.MethodGet, u, "", "If-Modified-Since", modified); res.StatusCode != http.StatusNotModified ||
		len(body) != 0 {
		t.Errorf("a copy as new as the file = %d %q, want 304", res.StatusCode, body)
	}
	if res, _ := h.get(t, http.MethodGet, u, "", "Range", "bytes=10-20"); res.StatusCode != http.StatusRequestedRangeNotSatisfiable ||
		res.Header.Get("Content-Range") != "bytes */6" || res.Header.Get("Content-Security-Policy") != contentPolicy {
		t.Errorf("a range outside = %d %q, CSP %q; want 416 bytes */6, sandboxed", res.StatusCode, res.Header.Get("Content-Range"),
			res.Header.Get("Content-Security-Policy"))
	}
	for _, c := range [][2]string{{"If-Match", `"other"`}, {"If-Unmodified-Since", "Mon, 01 Jan 2001 00:00:00 GMT"}} {
		if res, body := h.get(t, http.MethodGet, u, "", c[0], c[1]); res.StatusCode != http.StatusOK || string(body) != "abcdef" {
			t.Errorf("%s %s = %d %q, want 200 the file", c[0], c[1], res.StatusCode, body)
		}
	}
	if res, body := h.get(t, http.MethodHead, u, ""); res.StatusCode != http.StatusOK || len(body) != 0 || res.Header.Get("Content-Length") != "6" {
		t.Errorf("HEAD = %d %q, length %q; want 200, no body, 6", res.StatusCode, body, res.Header.Get("Content-Length"))
	}
	waitFor(t, func() bool { return h.files.unclosed() == 0 })
}

// An address that expires after 2038, past a 32-bit time, is read as any.
func TestContentReadsAnAddressPast2038(t *testing.T) {
	h := newHarness(t)
	n, b := h.attach("a.png", "image/png", "abc")
	h.signedAt = time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	if res, body := h.get(t, http.MethodGet, h.address(n, b, false), ""); res.StatusCode != http.StatusOK || string(body) != "abc" {
		t.Errorf("download = %d %q, want 200 the file", res.StatusCode, body)
	}
}

// The downloads take from their own bucket, by client IP: past it,
// rate_limited.
func TestContentHasABucketOfItsOwn(t *testing.T) {
	h := newHarness(t)
	h.downloads.left = 1
	n, b := h.attach("a.png", "image/png", "abc")
	if res, _ := h.get(t, http.MethodGet, h.address(n, b, false), ""); res.StatusCode != http.StatusOK {
		t.Fatalf("the first download = %d, want 200", res.StatusCode)
	}
	res, body := h.get(t, http.MethodGet, h.address(n, b, false), "")
	if res.StatusCode != http.StatusTooManyRequests || code(body) != "rate_limited" || h.downloads.keys[0] != "127.0.0.1" ||
		!isSandboxed(res) {
		t.Errorf("the second = %d %s by %q, want 429 rate_limited by the client's IP", res.StatusCode, body, h.downloads.keys)
	}
}

// A download whose content opens once the server began shutting down is
// server_busy, no-store, without the file's headers: the server does not
// start sending it, nor lets the refusal be cached. The stream learns of
// the shutdown a moment after the listener closes, on goroutines of its
// own: the content opens a while after.
func TestContentAsTheServerShutsDownIsServerBusy(t *testing.T) {
	h := newHarness(t)
	n, b := h.attach("a.png", "image/png", "abc")
	h.nodes.gate, h.nodes.asked = make(chan struct{}), make(chan struct{}, 1)
	stop, done := h.serve(t)
	answered := make(chan *http.Response, 1)
	go func() {
		res, _ := h.get(t, http.MethodGet, h.address(n, b, false), "")
		answered <- res
	}()
	<-h.nodes.asked
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
	close(h.nodes.gate)
	select {
	case res := <-answered:
		if res.StatusCode != http.StatusServiceUnavailable || res.Header.Get("Retry-After") != "5" || !isSandboxed(res) ||
			res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("ETag") != "" || res.Header.Get("Content-Disposition") != "" {
			t.Errorf("download = %d, headers %v; want 503 after 5 s, sandboxed, no-store, none of the file's", res.StatusCode, res.Header)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the download did not answer")
	}
	<-done
}

// A step past the request's deadline is answered 500 and logged as a
// request that ran out of time, a warning, not as a fault.
func TestContentPastItsDeadlineIsLoggedAsAWarning(t *testing.T) {
	h := newHarnessWith(t, httpservertest.APIOptions{RequestTimeout: 100 * time.Millisecond})
	n, b := h.attach("a.png", "image/png", "abc")
	h.nodes.gate, h.nodes.asked = make(chan struct{}), make(chan struct{}, 1)
	defer close(h.nodes.gate)
	res, body := h.get(t, http.MethodGet, h.address(n, b, false), "")
	if res.StatusCode != http.StatusInternalServerError || code(body) != "internal_error" {
		t.Errorf("download = %d %s, want 500 internal_error", res.StatusCode, body)
	}
	if logs := h.logs.String(); !strings.Contains(logs, `level=WARN msg="API request deadline exceeded"`) ||
		strings.Contains(logs, "level=ERROR") {
		t.Errorf("logs %q, want the deadline as a warning, no error", logs)
	}
}

// An upload's address downloads the file as it was sent.
func TestAnUploadDownloadsAsItWasSent(t *testing.T) {
	h := newHarness(t)
	ct, body := form(t, file("a.png", "\x89PNG\r\n\x1a\nthe rest"))
	_, answer := h.post(t, uploadPath(), "session", ct, bytes.NewReader(body))
	var a struct {
		ContentURL string `json:"content_url"`
	}
	if err := json.Unmarshal(answer, &a); err != nil {
		t.Fatal(err)
	}
	if res, got := h.get(t, http.MethodGet, a.ContentURL, ""); res.StatusCode != http.StatusOK || string(got) != "\x89PNG\r\n\x1a\nthe rest" {
		t.Errorf("download = %d %q, want the file", res.StatusCode, got)
	}
}
