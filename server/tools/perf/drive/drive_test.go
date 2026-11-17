package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	sessions  int
}

func (p *pageAPI) handle(f *fake, w http.ResponseWriter, r *http.Request, body []byte) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.revisions == nil {
		p.revisions = map[string]int64{}
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
	if c, _ := (Bodies{Kind: "pathological", Size: 3 << 20}).Content(); c != strings.Repeat("a*_", 1<<20) {
		t.Error("the pathological body is not a*_ repeated")
	}
	if n := NormalBody(2, MaxContentBytes); len(n) != MaxContentBytes || !strings.Contains(n, "## ") || !strings.Contains(n, "\n- ") ||
		!strings.Contains(n, "[[Page ") || n != NormalBody(2, MaxContentBytes) {
		t.Error("the normal body is not 5 MiB of headings, lists and links, its seed's")
	}
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

// The summary times each operation's answers from its start, a request
// the run's end cut counted apart.
func TestTheSummaryCountsCutRequestsApart(t *testing.T) {
	at := time.Unix(100, 0)
	ns := func(d time.Duration) int64 { return at.Add(d).UnixNano() }
	records := []client.Record{
		{Op: "view", Sent: ns(-time.Second), Done: ns(0), Status: 200},
		{Op: "view", Sent: ns(0), Done: ns(30 * time.Millisecond), Status: 200},
		{Op: "view", Sent: ns(time.Second), Done: ns(time.Second + 10*time.Millisecond), Status: 503},
		{Op: "view", Sent: ns(2 * time.Second), Done: ns(3 * time.Second), Status: 0, Error: "context deadline exceeded"},
		{Op: "write", Sent: ns(time.Second), Done: ns(2 * time.Second), Status: 0, Error: "context deadline exceeded"},
	}
	var out bytes.Buffer
	if err := Summarize(&out, records, at); err != nil {
		t.Fatal(err)
	}
	want := "view n=2 p50=10.0ms p99=30.0ms max=30.0ms statuses={\"200\":1,\"503\":1} cut=1\n" +
		"write n=0 p50=0.0ms p99=0.0ms max=0.0ms statuses={} cut=1\n"
	if out.String() != want {
		t.Errorf("summary\n%s\nwant\n%s", out.String(), want)
	}
}
