package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/tools/perf/client"
)

// fake is a serve's stand-in: handle answers each request, its body read;
// what happened is kept in order, the fake psql's answers among it.
type fake struct {
	mu     sync.Mutex
	events []string
	srv    *httptest.Server
}

func newFake(t *testing.T, handle func(f *fake, w http.ResponseWriter, r *http.Request, body []byte)) *fake {
	t.Helper()
	f := &fake{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("%s %s: %v", r.Method, r.URL.Path, err)
		}
		if r.Header.Get("Authorization") == "" && !strings.HasSuffix(r.URL.Path, "/download") {
			t.Errorf("%s %s without a token", r.Method, r.URL.Path)
		}
		handle(f, w, r, body)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// note keeps an event.
func (f *fake) note(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, fmt.Sprintf(format, args...))
}

// all are the events so far.
func (f *fake) all() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.events...)
}

// count is how many events begin with prefix.
func (f *fake) count(prefix string) int {
	n := 0
	for _, e := range f.all() {
		if strings.HasPrefix(e, prefix) {
			n++
		}
	}
	return n
}

// clients are n clients of the fake, their records to log.
func (f *fake) clients(n int, log *client.Log) []*client.Client {
	out := make([]*client.Client, n)
	for i := range out {
		out[i] = client.New(f.srv.URL, fmt.Sprintf("pat-%d", i), i, log)
	}
	return out
}

// answer writes v as JSON with status.
func answer(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// pageAPI answers the page calls the drivers make: a page made, its
// content written over its revision (409 over another), a session opened,
// beaten, closed, its view read, a node renamed or moved.
type pageAPI struct {
	mu        sync.Mutex
	pages     int
	revisions map[string]int64
	contents  map[string]string // each page's last content written
	sessions  int
}

func (p *pageAPI) handle(f *fake, w http.ResponseWriter, r *http.Request, body []byte) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.revisions == nil {
		p.revisions, p.contents = map[string]int64{}, map[string]string{}
	}
	var in map[string]any
	_ = json.Unmarshal(body, &in)
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/") // api v0 ...
	switch {
	case r.Method == http.MethodPost && len(parts) == 5 && parts[2] == "notebooks" && parts[4] == "pages":
		p.pages++
		id := fmt.Sprintf("page-%d", p.pages)
		p.revisions[id] = 1
		f.note("create %s in %s under %v title %v", id, parts[3], in["parent_id"], in["title"])
		answer(w, http.StatusCreated, map[string]any{"id": id, "revision": 1})
	case r.Method == http.MethodPut && len(parts) == 5 && parts[2] == "pages" && parts[4] == "content":
		id := parts[3]
		base, _ := in["base_revision"].(float64)
		content, _ := in["content"].(string)
		f.note("put %s base %d session %v bytes %d links %d", id, int64(base), in["edit_session_id"], len(content),
			strings.Count(content, "[["))
		if int64(base) != p.revisions[id] {
			answer(w, http.StatusConflict, map[string]any{"status": 409, "code": "conflict", "detail": "x"})
			return true
		}
		// A write of the content the page has writes no version on a serve
		// (page/app/unit_content.go): the drivers' writes each change it.
		if last, ok := p.contents[id]; ok && last == content {
			f.note("put-same %s base %d", id, int64(base))
		}
		p.contents[id] = content
		p.revisions[id]++
		answer(w, http.StatusOK, map[string]any{"id": id, "revision": p.revisions[id]})
	case r.Method == http.MethodPost && len(parts) == 5 && parts[2] == "pages" && parts[4] == "edit-sessions":
		p.sessions++
		f.note("session-open %s session-%d", parts[3], p.sessions)
		answer(w, http.StatusCreated, map[string]any{"id": fmt.Sprintf("session-%d", p.sessions)})
	case r.Method == http.MethodPost && len(parts) == 5 && parts[2] == "edit-sessions" && parts[4] == "heartbeat":
		f.note("heartbeat %s", parts[3])
		answer(w, http.StatusOK, map[string]any{"id": parts[3]})
	case r.Method == http.MethodDelete && len(parts) == 4 && parts[2] == "edit-sessions":
		f.note("session-close %s", parts[3])
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && len(parts) == 5 && parts[2] == "pages" && parts[4] == "view":
		f.note("view %s", parts[3])
		answer(w, http.StatusOK, map[string]any{"html": "<p>x</p>", "revision": p.revisions[parts[3]]})
	case r.Method == http.MethodPatch && len(parts) == 4 && parts[2] == "nodes":
		f.note("rename %s %v", parts[3], in["name"])
		answer(w, http.StatusOK, map[string]any{"id": parts[3]})
	case r.Method == http.MethodPost && len(parts) == 5 && parts[2] == "nodes" && parts[4] == "move":
		f.note("move %s under %v", parts[3], in["parent_id"])
		answer(w, http.StatusOK, map[string]any{"id": parts[3]})
	default:
		return false
	}
	return true
}

// B2's early busy answer is one whose head came within 2.0 s at the
// client: a 503 at once counts, one after 2.1 s does not (design 3.4.5,
// 7.2's 36).
func TestAnEarlyBusyIsTheClientsTime(t *testing.T) {
	f := newFake(t, func(_ *fake, w http.ResponseWriter, r *http.Request, _ []byte) {
		if r.URL.Path == "/late" {
			time.Sleep(2100 * time.Millisecond)
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"status":503,"code":"server_busy","detail":"later"}`)
	})
	log := client.NewLog(nil)
	c := f.clients(1, log)[0]
	start := time.Now()
	for _, path := range []string{"/early", "/late"} {
		if rec, _, err := c.Send(context.Background(), client.Request{Op: "view", Method: http.MethodGet, Path: path}, nil); err != nil ||
			rec.Code != "server_busy" {
			t.Fatalf("%s: %+v, %v", path, rec, err)
		}
	}
	if busy, early := BodiesSummary(log.Records(), start); busy != 2 || early != 1 {
		t.Errorf("%d busy, %d early; want 2 and the one at once", busy, early)
	}
}

// B2's run: readers read their pages' views, writers write theirs in a
// session over each revision, with a heartbeat, in the notebooks named
// (design 3.6's B2).
func TestBodiesReadAndWrite(t *testing.T) {
	api := &pageAPI{}
	f := newFake(t, func(f *fake, w http.ResponseWriter, r *http.Request, body []byte) {
		if !api.handle(f, w, r, body) {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	log := client.NewLog(nil)
	b := Bodies{Clients: f.clients(5, log), Kind: "pathological", Size: 3000, Read: "nb-read", Writers: []string{"nb-w0", "nb-w1"},
		Warmup: 30 * time.Millisecond, For: 200 * time.Millisecond, Heartbeat: 50 * time.Millisecond}
	if _, err := b.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.count("create page-1 in nb-read") != 1 || f.count("create page-3 in nb-read") != 1 || f.count("create page-4 in nb-w0") != 1 ||
		f.count("create page-5 in nb-w1") != 1 {
		t.Errorf("pages %v", f.all())
	}
	if f.count("view page-1") == 0 || f.count("view page-3") == 0 || f.count("view page-4") != 0 {
		t.Errorf("views: %d, %d, %d", f.count("view page-1"), f.count("view page-3"), f.count("view page-4"))
	}
	// Each writer's writes go over each revision, in its session.
	for _, w := range []struct{ page, session string }{{"page-4", "session-1"}, {"page-5", "session-2"}} {
		for base := 2; base <= 4; base++ {
			if f.count(fmt.Sprintf("put %s base %d session %s bytes 3000", w.page, base, w.session)) != 1 {
				t.Errorf("%s's write over %d in %s: %v", w.page, base, w.session, f.all())
			}
		}
	}
	if f.count("heartbeat session-") == 0 || f.count("put page-4 base 1 session <nil> bytes 3000") != 1 {
		t.Errorf("no heartbeat, or the first content not written: %v", f.all())
	}
	if f.count("put-same ") != 0 {
		t.Errorf("a write of the content the page had: %v", f.all())
	}
	// changed changes the middle byte each time, the first time away from
	// what the content has there, p or not.
	for _, c := range []string{"aaaaa", "aapaa", "aaqaa"} {
		last := c
		for k := range 4 {
			got := changed(c, k)
			if got == last || len(got) != len(c) || got[:2] != c[:2] || got[3:] != c[3:] {
				t.Errorf("changed(%q, %d) = %q after %q", c, k, got, last)
			}
			last = got
		}
	}
	if c, _ := (Bodies{Kind: "pathological", Size: 3 << 20}).Content(); c != strings.Repeat("a*_", 1<<20) {
		t.Error("the pathological body is not a*_ repeated")
	}
	if n := NormalBody(2, MaxContentBytes); len(n) != MaxContentBytes || !strings.Contains(n, "## ") || !strings.Contains(n, "\n- ") ||
		!strings.Contains(n, "[[Page ") || n != NormalBody(2, MaxContentBytes) {
		t.Error("the normal body is not 5 MiB of headings, lists and links, its seed's")
	}
}

// metaStarts is how each credential's _meta starts in a shape: 2a many
// members, 2b nested empty arrays, 2c a long flat array (valueMembers), 3
// one of each; 1 and L the long string alone (M12/P4/S3's check B, M2:
// a 2b that sent 2a's members passed).
func metaStarts(shape string) [3]string {
	starts := map[byte]string{'a': `"m0":1`, 'b': `"a":[[]`, 'c': `"a":[1`}
	switch shape {
	case "2a", "2b", "2c":
		s := starts[shape[1]]
		return [3]string{s, s, s}
	case "3":
		return [3]string{starts['a'], starts['b'], starts['c']}
	}
	return [3]string{`"pad":"`, `"pad":"`, `"pad":"`}
}

// B3's shapes: at the bound, MaxBodyValues values counted as bodyshape
// counts them, a body past 64 KiB; a value more past it; 5 MiB of
// content; and their run, three at once (design 3.6's B3, 7.2's 39).
func TestMCPShapesAtTheValuesBound(t *testing.T) {
	for _, c := range []struct {
		shape  string
		values [3]int
	}{
		{"2a", [3]int{MaxBodyValues, MaxBodyValues, MaxBodyValues}},
		{"2b", [3]int{MaxBodyValues, MaxBodyValues, MaxBodyValues}},
		{"2c", [3]int{MaxBodyValues, MaxBodyValues, MaxBodyValues}},
		{"3", [3]int{MaxBodyValues + 1, MaxBodyValues + 1, MaxBodyValues + 1}},
	} {
		want := metaStarts(c.shape)
		sends, err := MCPShape(c.shape, "b3/0", 3, 1024)
		if err != nil {
			t.Fatal(err)
		}
		bodies := map[string]bool{}
		for i, s := range sends {
			n, err := client.CountValues(s.Body)
			if err != nil || n != c.values[i] || len(s.Body) <= MaxOtherBytes || s.Content != "" {
				t.Errorf("%s, credential %d: %d values of %d bytes, %v; want %d past %d bytes", c.shape, i, n, len(s.Body), err,
					c.values[i], MaxOtherBytes)
			}
			at := bytes.Index(s.Body, []byte(`"_meta":{`)) + len(`"_meta":{`)
			bodies[string(s.Body[at:at+6])] = true
			if !bytes.HasPrefix(s.Body[at:], []byte(want[i])) {
				t.Errorf("%s, credential %d: _meta starts %q, want %q", c.shape, i, s.Body[at:at+8], want[i])
			}
		}
		if c.shape == "3" && len(bodies) != 3 {
			t.Errorf("shape 3's credentials send %d shapes, want three", len(bodies))
		}
	}
	sends, err := MCPShape("1", "b3/0", 3, MaxContentBytes)
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range sends {
		var call struct {
			Params struct {
				Name      string            `json:"name"`
				Arguments map[string]string `json:"arguments"`
			} `json:"params"`
		}
		if err := json.Unmarshal(s.Body, &call); err != nil || call.Params.Name != "write" || len(call.Params.Arguments["content"]) != MaxContentBytes ||
			call.Params.Arguments["path"] != fmt.Sprintf("b3/0/1-%d.md", i) {
			t.Errorf("shape 1, credential %d: %s %q, %v", i, call.Params.Name, call.Params.Arguments["path"], err)
		}
	}
	four, err := MCPShape("4", "b3/0", 3, 2048)
	if err != nil || four[0].Body == nil || four[1].Body == nil || four[2].Body != nil || len(four[2].Content) != 2048 {
		t.Errorf("shape 4: %v", err)
	}
	if _, err := MCPShape("5", "b3/0", 3, 2048); err == nil {
		t.Error("a shape 5")
	}

	api := &pageAPI{}
	var mu sync.Mutex
	inFlight, most := 0, 0
	f := newFake(t, func(f *fake, w http.ResponseWriter, r *http.Request, body []byte) {
		if r.URL.Path == "/mcp" {
			mu.Lock()
			inFlight++
			most = max(most, inFlight)
			mu.Unlock()
			time.Sleep(30 * time.Millisecond)
			mu.Lock()
			inFlight--
			mu.Unlock()
			n, _ := client.CountValues(body)
			f.note("mcp %s %d %s", r.URL.Query().Get("notebook"), n, r.Header.Get("Mcp-Protocol-Version"))
			answer(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 7, "result": map[string]any{}})
			return
		}
		if !api.handle(f, w, r, body) {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	cs := f.clients(3, client.NewLog(nil))
	run := MCPRun{Clients: [3]*client.Client{cs[0], cs[1], cs[2]}, Notebook: "nb-mcp", Shape: "2b", Seed: 3, Size: 1024, Reps: 2,
		Gap: 10 * time.Millisecond}
	if err := run.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.count(fmt.Sprintf("mcp nb-mcp %d %s", MaxBodyValues, client.MCPVersion)) != 6 || most != 3 {
		t.Errorf("2b twice: %v, %d at once", f.all(), most)
	}
	run.Shape = "4"
	if err := run.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.count("put page-1 base 2") != 1 || f.count("put page-1 base 3") != 1 {
		t.Errorf("shape 4's REST writes: %v", f.all())
	}
}

// B3 at /mcp's bound, as M9/P2 measured it (M12/P4/S3's review B, I2):
// each MCP body exactly the bound, README's 37,814,272 bytes; the content
// 5 MiB of '<', six bytes each; 2's values at the values' bound and 3's a
// value more, each credential's _meta in its shape; 4's third a REST write
// within the content's body bound; L the long string alone; and the run
// sends them three at once. The bodies are checked by their bytes, the
// values counted once a shape (each credential in 3): under -race a large
// body's decoding is the test's time (CI's costs-3, check 1 of S3).
func TestMCPShapesAtTheBodyBound(t *testing.T) {
	if MaxMCPBodyBytes != 37_814_272 {
		t.Fatalf("the bound is %d bytes, want 37,814,272", MaxMCPBodyBytes)
	}
	base, err := client.CountValues(client.Call("write", map[string]string{"path": "p", "content": "x"}, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	escaped := []byte(`"content":"` + strings.Repeat(`\u003c`, MaxContentBytes) + `"`)
	for _, c := range []struct {
		shape   string
		values  int
		content []byte
	}{
		{"1", base + 1, escaped},
		{"2a", MaxBodyValues, escaped},
		{"2b", MaxBodyValues, escaped},
		{"2c", MaxBodyValues, escaped},
		{"3", MaxBodyValues + 1, escaped},
		{"L", base + 1, []byte(`"content":"x"`)},
	} {
		want := metaStarts(c.shape)
		sends, err := MCPShapeAtBound(c.shape, "b3/0")
		if err != nil {
			t.Fatal(err)
		}
		var first []byte
		for i, s := range sends {
			n := c.values
			if i == 0 || c.shape == "3" {
				if n, err = client.CountValues(s.Body); err != nil {
					t.Fatal(err)
				}
			}
			// json.Marshal writes the arguments' members in order: content, path.
			args := []byte(`"arguments":{`)
			at := bytes.Index(s.Body, args) + len(args)
			path := fmt.Sprintf(`,"path":"b3/0/%s-%d.md"}`, c.shape, i)
			if len(s.Body) != MaxMCPBodyBytes || n != c.values || !bytes.HasPrefix(s.Body[at:], c.content) ||
				!bytes.HasPrefix(s.Body[at+len(c.content):], []byte(path)) || s.Content != "" {
				t.Errorf("%s, credential %d: %d bytes, %d values, its content or %s not as it should be; want %d bytes, %d values",
					c.shape, i, len(s.Body), n, path, MaxMCPBodyBytes, c.values)
			}
			meta := bytes.Index(s.Body, []byte(`"_meta":{`)) + len(`"_meta":{`)
			if !bytes.HasPrefix(s.Body[meta:], []byte(want[i])) {
				t.Errorf("%s, credential %d: _meta starts %q, want %q", c.shape, i, s.Body[meta:meta+8], want[i])
			}
			// The values are counted on credential 0's alone (but in 3): the
			// others' _meta before the long string is credential 0's, byte for
			// byte (check 2 of S3, B3-N1).
			members := s.Body[meta : meta+bytes.Index(s.Body[meta:], []byte(`"pad":"`))]
			if i == 0 {
				first = members
			} else if c.shape != "3" && !bytes.Equal(members, first) {
				t.Errorf("%s, credential %d: _meta before the long string is %d bytes, not credential 0's %d", c.shape, i,
					len(members), len(first))
			}
		}
	}
	four, err := MCPShapeAtBound("4", "b3/0")
	if err != nil || len(four[0].Body) != MaxMCPBodyBytes || len(four[1].Body) != MaxMCPBodyBytes || four[2].Body != nil ||
		four[2].Content != EscapedContent() {
		t.Fatalf("shape 4: %v", err)
	}
	// The REST write as the run sends it, within the content's body bound
	// (page/adapter/http/limits.go's contentBodyLimit).
	in, err := json.Marshal(map[string]any{"content": changed(four[2].Content, 1), "base_revision": 1 << 40})
	if err != nil || len(in) > 6*MaxContentBytes+64<<10 || len(in) < 6*(MaxContentBytes-1) {
		t.Errorf("shape 4's REST write: %d bytes, %v", len(in), err)
	}
	if _, err := MCPShapeAtBound("5", "b3/0"); err == nil {
		t.Error("a shape 5 at the bound")
	}
	if _, err := MCPShape("L", "b3/0", 3, 2048); err == nil {
		t.Error("a shape L below the bound")
	}

	// The run: each request counted when it arrives, before its body is
	// read, and answered once all three arrived (or after 10 s), so that how
	// long the bodies take to upload does not matter; sent one after
	// another, they are never in flight together (check 1 of S3: a fixed
	// overlap saw 2 at once under -race in CI).
	var mu sync.Mutex
	arrived, inFlight, most := 0, 0, 0
	var sizes []int64
	all := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		arrived++
		inFlight++
		most = max(most, inFlight)
		if arrived == 3 {
			close(all)
		}
		mu.Unlock()
		n, _ := io.Copy(io.Discard, r.Body)
		select {
		case <-all:
		case <-time.After(10 * time.Second):
		}
		mu.Lock()
		inFlight--
		sizes = append(sizes, n)
		mu.Unlock()
		answer(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 7, "result": map[string]any{}})
	}))
	t.Cleanup(srv.Close)
	var cs [3]*client.Client
	for i := range cs {
		cs[i] = client.New(srv.URL, fmt.Sprintf("pat-%d", i), i, client.NewLog(nil))
	}
	run := MCPRun{Clients: cs, Notebook: "nb-mcp", Shape: "L", Bound: true, Reps: 1}
	if err := run.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sizes, []int64{MaxMCPBodyBytes, MaxMCPBodyBytes, MaxMCPBodyBytes}) || most != 3 {
		t.Errorf("L at the bound: %v bytes, %d at once", sizes, most)
	}
}

// The summary times each operation's answers from its start; a request
// without its whole answer is not timed, cut when its context ended, with
// the longest of them (not the last: review B's N8), failed when the
// transport did, and a failure is an error after the summary (review B's
// M2, review C's note).
func TestTheSummaryCountsCutRequestsApart(t *testing.T) {
	at := time.Unix(100, 0)
	ns := func(d time.Duration) int64 { return at.Add(d).UnixNano() }
	records := []client.Record{
		{Op: "view", Sent: ns(-time.Second), Done: ns(0), Status: 200},
		{Op: "view", Sent: ns(0), Done: ns(30 * time.Millisecond), Status: 200},
		{Op: "view", Sent: ns(time.Second), Done: ns(time.Second + 10*time.Millisecond), Status: 503},
		{Op: "view", Sent: ns(2 * time.Second), Header: ns(2 * time.Second), Done: ns(4500 * time.Millisecond), Status: 200,
			Error: "context deadline exceeded", Cut: true},
		{Op: "view", Sent: ns(2 * time.Second), Done: ns(3 * time.Second), Error: "context deadline exceeded", Cut: true},
		{Op: "write", Sent: ns(time.Second), Done: ns(2 * time.Second), Error: "context deadline exceeded", Cut: true},
	}
	var out bytes.Buffer
	if err := Summarize(&out, records, at); err != nil {
		t.Fatal(err)
	}
	want := "view n=2 p50=10.0ms p99=30.0ms max=30.0ms statuses={\"200\":1,\"503\":1} cut=2 errors=0 cut_longest=2500.0ms\n" +
		"write n=0 p50=0.0ms p99=0.0ms max=0.0ms statuses={} cut=1 errors=0 cut_longest=1000.0ms\n"
	if out.String() != want {
		t.Errorf("summary\n%s\nwant\n%s", out.String(), want)
	}
	out.Reset()
	records = append(records, client.Record{Op: "write", Sent: ns(time.Second), Done: ns(time.Second), Error: "read: connection reset by peer"})
	err := Summarize(&out, records, at)
	if err == nil || !strings.Contains(out.String(), "write n=0 p50=0.0ms p99=0.0ms max=0.0ms statuses={} cut=1 errors=1 ") {
		t.Errorf("a failed request: %v\n%s", err, out.String())
	}
}

// past is a run's stop come: its context ended, or its deadline gone by
// while the timer that ends the context has not run yet (review B's M7).
func TestPastIsTheStopsEndOrItsDeadline(t *testing.T) {
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	for _, c := range []struct {
		name string
		stop context.Context
		want bool
	}{
		{"running", deadlineOnly{context.Background(), time.Now().Add(time.Hour)}, false},
		{"its deadline gone by, its timer not run", deadlineOnly{context.Background(), time.Now().Add(-time.Millisecond)}, true},
		{"ended", ended, true},
		{"no deadline", context.Background(), false},
	} {
		if got := past(c.stop); got != c.want {
			t.Errorf("%s: past %v, want %v", c.name, got, c.want)
		}
	}
}

// deadlineOnly has a deadline and never ends: a context whose timer has
// not run.
type deadlineOnly struct {
	context.Context
	at time.Time
}

func (d deadlineOnly) Deadline() (time.Time, bool) { return d.at, true }

// A run stops sending at its end and waits for what is in flight: a
// reader's and a writer's request longer than the run's rest are answered
// and timed, not cut, and nothing is sent past the end, nor after a
// writer's heartbeat that the end came in; one longer than the drain is
// cut, and recorded as cut, before its answer's head or in its body
// (review B's M2, M8, N8).
func TestARunWaitsForItsRequestsInFlight(t *testing.T) {
	api := &pageAPI{}
	var view, write, beat atomic.Int64
	var bodyLate atomic.Bool
	wait := func(r *http.Request, d *atomic.Int64) bool {
		select {
		case <-time.After(time.Duration(d.Load())):
			return true
		case <-r.Context().Done():
			return false
		}
	}
	f := newFake(t, func(f *fake, w http.ResponseWriter, r *http.Request, body []byte) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/view") && bodyLate.Load():
			// The head at once, the body past the drain.
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			if wait(r, &view) {
				_, _ = io.WriteString(w, "<p>late</p>")
			}
			return
		case strings.HasSuffix(r.URL.Path, "/view") && !wait(r, &view),
			r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/content") && !wait(r, &write),
			strings.HasSuffix(r.URL.Path, "/heartbeat") && !wait(r, &beat):
			return
		}
		if !api.handle(f, w, r, body) {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	// A reader and a writer, each request 150 ms in a run of 50.
	view.Store(int64(150 * time.Millisecond))
	write.Store(int64(150 * time.Millisecond))
	log := client.NewLog(nil)
	b := Bodies{Clients: f.clients(2, log), Kind: "normal", Size: 1000, Read: "nb-read", Writers: []string{"nb-w"},
		For: 50 * time.Millisecond, Heartbeat: time.Minute}
	started := time.Now()
	from, err := b.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	took := time.Since(started)
	end := from.Add(b.For).UnixNano()
	answered := map[string]int{}
	for _, r := range log.Records() {
		switch {
		case r.Sent > end:
			t.Errorf("%s sent %v past the end", r.Op, time.Duration(r.Sent-end))
		case r.Cut:
			t.Errorf("%s cut", r.Op)
		case r.Status == http.StatusOK && r.Took() >= 150*time.Millisecond:
			answered[r.Op]++
		}
	}
	if answered["view"] != 1 || answered["write"] != 1 || took < 150*time.Millisecond || took > b.For+5*time.Second {
		t.Errorf("a view and a write of 150 ms in a run of 50 ms: %v answered, the run %v", answered, took)
	}
	// A writer's heartbeat that the end comes in: no write after it.
	write.Store(0)
	beat.Store(int64(150 * time.Millisecond))
	log = client.NewLog(nil)
	b.Clients, b.Heartbeat = f.clients(2, log), time.Nanosecond
	if from, err = b.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	end = from.Add(b.For).UnixNano()
	beats := 0
	for _, r := range log.Records() {
		if r.Op == "write" && r.Sent > end {
			t.Errorf("a write sent %v past the end, after the heartbeat", time.Duration(r.Sent-end))
		}
		if r.Op == "heartbeat" && r.Status == http.StatusOK {
			beats++
		}
	}
	if beats != 1 {
		t.Errorf("%d heartbeats answered, want the one the end came in", beats)
	}
	// A reader alone, past the drain: cut before its answer's head, then
	// in its body; neither is an error.
	view.Store(int64(10 * time.Second))
	for _, late := range []bool{false, true} {
		bodyLate.Store(late)
		log = client.NewLog(nil)
		b.Clients, b.Drain = f.clients(1, log), 100*time.Millisecond
		if from, err = b.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		var summary bytes.Buffer
		if err := Summarize(&summary, log.Records(), from); err != nil || !strings.Contains(summary.String(), "view n=0 ") ||
			!strings.Contains(summary.String(), " cut=1 errors=0 cut_longest=") {
			t.Errorf("a view past the drain, its body late %v: %v\n%s", late, err, summary.String())
		}
		if late && !slices.ContainsFunc(log.Records(), func(r client.Record) bool { return r.Op == "view" && r.Status == 200 && r.Cut }) {
			t.Errorf("the view cut in its body: %+v", log.Records())
		}
	}
}
