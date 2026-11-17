package drive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/tools/perf/client"
)

// fakePSQL answers the drivers' SQL by its text, each call noted.
func fakePSQL(f *fake, answer func(sql string) string) client.PSQL {
	return client.PSQL{Container: "c0ffee", User: "nervewiki", Database: "nervewiki",
		Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			if name != "docker" || args[0] != "exec" || args[1] != "c0ffee" {
				return nil, fmt.Errorf("ran %s %v", name, args)
			}
			sql := args[len(args)-1]
			out := answer(sql)
			f.note("psql %s => %s", strings.Fields(sql)[0]+" "+strings.Fields(sql)[1], strings.TrimSpace(out))
			return []byte(out), nil
		}}
}

// The editors' load: each editor saves over each revision in its session,
// beats it, closes it and opens another; the first's page has its links;
// the tree writer creates, renames and moves; page_revisions is rewritten
// and read before and after, and monitored (design 3.4.5).
func TestEditorsSaveRenewAndWriteTheTree(t *testing.T) {
	api := &pageAPI{}
	f := newFake(t, func(f *fake, w http.ResponseWriter, r *http.Request, body []byte) {
		if !api.handle(f, w, r, body) {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	var monitored bytes.Buffer
	sizes := 0
	e := Editors{Clients: f.clients(3, client.NewLog(nil)), Notebook: "nb-ed", Bytes: 10 << 10, Links: 20, HeavyLinks: 500, Seed: 1,
		For: 400 * time.Millisecond, Save: 10 * time.Millisecond, Heartbeat: 40 * time.Millisecond, Renew: 120 * time.Millisecond,
		Tree: 15 * time.Millisecond, Monitor: 50 * time.Millisecond, Monitored: &monitored,
		PSQL: fakePSQL(f, func(sql string) string {
			switch {
			case strings.HasPrefix(sql, "VACUUM FULL page_revisions"):
				return ""
			case strings.HasPrefix(sql, "SELECT pg_total_relation_size('page_revisions')"):
				sizes++
				return fmt.Sprintf("%d|%d\n", 8192*sizes, 10*sizes)
			case strings.Contains(sql, "pg_stat_slru"):
				return "MultiXactOffset|1|2|3|4|5|6|7\nMultiXactMember|1|2|3|4|5|6|7\n"
			case strings.Contains(sql, "pg_stat_activity"):
				return "LWLock|MultiXactGen|2\n"
			}
			t.Errorf("sql %q", sql)
			return ""
		})}
	before, after, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if before != (Versions{8192, 10}) || after != (Versions{16384, 20}) {
		t.Errorf("versions %+v then %+v", before, after)
	}
	events := f.all()
	vacuums := []int{}
	for i, ev := range events {
		if strings.HasPrefix(ev, "psql VACUUM FULL") {
			vacuums = append(vacuums, i)
		}
	}
	firstSave := slices.IndexFunc(events, func(ev string) bool { return strings.Contains(ev, "session session-") && strings.HasPrefix(ev, "put") })
	if len(vacuums) != 2 || firstSave < vacuums[0] || firstSave > vacuums[1] {
		t.Errorf("VACUUM FULL at %v, the first save at %d", vacuums, firstSave)
	}
	if f.count("put page-1 base 1 session <nil> bytes") != 1 || !slices.ContainsFunc(events, func(ev string) bool {
		return strings.HasPrefix(ev, "put page-1 base 1 session <nil>") && strings.HasSuffix(ev, "links 500")
	}) || !slices.ContainsFunc(events, func(ev string) bool {
		return strings.HasPrefix(ev, "put page-2 base 1 session <nil> bytes 10240") && strings.HasSuffix(ev, "links 20")
	}) {
		t.Errorf("the pages' contents: %v", events)
	}
	// Each editor's sessions renewed, each save in one of them, over each
	// revision.
	for _, page := range []string{"page-1", "page-2"} {
		if f.count("session-open "+page) < 3 {
			t.Errorf("%s's sessions opened %d times, want renewed", page, f.count("session-open "+page))
		}
		for base := 2; base <= 6; base++ {
			if f.count(fmt.Sprintf("put %s base %d session session-", page, base)) != 1 {
				t.Errorf("%s's save over %d: %v", page, base, events)
			}
		}
	}
	if f.count("session-close session-") < 4 || f.count("heartbeat session-") == 0 {
		t.Errorf("%d closed, %d beats", f.count("session-close session-"), f.count("heartbeat session-"))
	}
	if f.count("create page-") < 5 || f.count("rename page-") == 0 || f.count("move page-") == 0 || f.count("move page-4") != 0 {
		t.Errorf("the tree writes: %v", events)
	}
	if !strings.Contains(monitored.String(), `"MultiXactGen"`) {
		t.Errorf("monitored %q", monitored.String())
	}
}

// The tabs' reads go as the refresher's: the first at once, the requests
// within the interval merged into one at its end, a request after it at
// once again (design 3.4.5, 7.2's 37).
func TestATabsReadsAreThrottledAsTheRefreshers(t *testing.T) {
	now := time.Unix(1000, 0)
	var scheduled []func()
	var at []time.Duration
	th := &Throttle{Interval: 500 * time.Millisecond, Now: func() time.Time { return now },
		After: func(d time.Duration, f func()) { scheduled, at = append(scheduled, f), append(at, d) }}
	var reads atomic.Int32
	var wg sync.WaitGroup
	read := func() {
		defer wg.Done()
		reads.Add(1)
	}
	step := func(d time.Duration) { now = now.Add(d) }
	wg.Add(1)
	th.Request(read) // 0 ms: at once
	wg.Wait()
	for range 3 { // 100, 200, 300 ms: one at 500 ms
		step(100 * time.Millisecond)
		th.Request(read)
	}
	if reads.Load() != 1 || len(scheduled) != 1 || at[0] != 400*time.Millisecond {
		t.Fatalf("after four requests within the interval: %d reads, %d scheduled in %v", reads.Load(), len(scheduled), at)
	}
	step(200 * time.Millisecond) // 500 ms
	wg.Add(1)
	scheduled[0]()
	step(100 * time.Millisecond) // 600 ms: at 1,000 ms
	th.Request(read)
	if reads.Load() != 2 || len(scheduled) != 2 || at[1] != 400*time.Millisecond {
		t.Fatalf("at 600 ms: %d reads, %d scheduled in %v", reads.Load(), len(scheduled), at)
	}
	step(400 * time.Millisecond) // 1,000 ms
	wg.Add(1)
	scheduled[1]()
	step(700 * time.Millisecond) // 1,700 ms: at once
	wg.Add(1)
	th.Request(read)
	wg.Wait()
	if reads.Load() != 4 || len(scheduled) != 2 {
		t.Errorf("at 1,700 ms: %d reads, %d scheduled", reads.Load(), len(scheduled))
	}
}

// The tabs' run: a stream each, the tree read on its notebook's pages
// events whose tree changed, merged within the interval; the agent's
// pages.
func TestTabsReadTheTreeOnItsEvents(t *testing.T) {
	api := &pageAPI{}
	f := newFake(t, func(f *fake, w http.ResponseWriter, r *http.Request, body []byte) {
		switch r.URL.Path {
		case "/api/v0/events":
			if r.Header.Get("Accept") != "text/event-stream" {
				t.Errorf("events accept %q", r.Header.Get("Accept"))
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "event: hello\ndata: {\"heartbeat_seconds\":20}\n\n")
			for _, e := range []string{
				`{"workspace_id":"w","notebook_id":"nb-tree","tree":true,"pages":[]}`,
				`{"workspace_id":"w","notebook_id":"nb-other","tree":true,"pages":[]}`,
				`{"workspace_id":"w","notebook_id":"nb-tree","tree":false,"pages":null}`,
				`{"workspace_id":"w","notebook_id":"nb-tree","tree":true,"pages":null}`,
				`{"workspace_id":"w","notebook_id":"nb-tree","tree":true,"pages":[]}`,
			} {
				_, _ = io.WriteString(w, "event: pages\ndata: "+e+"\n\n")
			}
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		case "/api/v0/notebooks/nb-tree/nodes":
			f.note("tree read by %s", r.Header.Get("Authorization"))
			answer(w, http.StatusOK, map[string]any{"data": []any{}})
		default:
			if !api.handle(f, w, r, body) {
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			}
		}
	})
	log := client.NewLog(nil)
	tb := Tabs{Clients: f.clients(3, log), Notebook: "nb-tree", For: 400 * time.Millisecond, Every: 60 * time.Millisecond,
		Interval: 150 * time.Millisecond}
	if err := tb.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, tab := range []string{"Bearer pat-0", "Bearer pat-1"} {
		if n := f.count("tree read by " + tab); n != 2 {
			t.Errorf("%s read the tree %d times, want at once and once merged", tab, n)
		}
	}
	if f.count("tree read by Bearer pat-2") != 0 || f.count("create page-") < 3 {
		t.Errorf("the agent: %v", f.all())
	}
	trees := 0
	for _, r := range log.Records() {
		if r.Op == "tree" && r.Bytes > 0 {
			trees++
		}
	}
	if trees != 4 {
		t.Errorf("%d tree reads recorded with their bytes", trees)
	}
}

// B4's operations wait for the background restore to succeed and for the
// background to be quiet before each (design 3.4.5, 7.2's 38).
func TestOpsWaitForTheRestoreAndTheQuiet(t *testing.T) {
	api := &pageAPI{}
	var mu sync.Mutex
	polls := map[string]int{}
	jobs := 0
	f := newFake(t, func(f *fake, w http.ResponseWriter, r *http.Request, body []byte) {
		switch {
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v0/nodes/"):
			f.note("delete %s", strings.TrimPrefix(r.URL.Path, "/api/v0/nodes/"))
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v0/notebooks/nb-v/trash-restores":
			mu.Lock()
			jobs++
			id := fmt.Sprintf("job-%d", jobs)
			mu.Unlock()
			f.note("restore %s as %s", body, id)
			answer(w, http.StatusAccepted, map[string]any{"id": id, "state": "queued"})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v0/transfer-jobs/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v0/transfer-jobs/")
			mu.Lock()
			polls[id]++
			state := "running"
			if polls[id] > 2 {
				state = "succeeded"
			}
			mu.Unlock()
			f.note("job %s %s", id, state)
			answer(w, http.StatusOK, map[string]any{"id": id, "state": state})
		default:
			if !api.handle(f, w, r, body) {
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			}
		}
	})
	quietCalls := 0
	var out bytes.Buffer
	o := Ops{Client: f.clients(1, client.NewLog(nil))[0], Notebook: "nb-v", Root: "vault", Reps: 3, Idle: 5 * time.Millisecond,
		Poll: 2 * time.Millisecond, Out: &out,
		PSQL: fakePSQL(f, func(sql string) string {
			switch {
			case strings.Contains(sql, "page_search_queue"):
				quietCalls++
				if quietCalls%3 != 0 {
					return "4\n"
				}
				return "0\n"
			case strings.Contains(sql, "parent_id IS NULL AND name = 'vault'"):
				return "id-vault\n"
			case strings.Contains(sql, "parent_id = 'id-vault' AND name = 'f1'"):
				return "id-f1\n"
			case strings.Contains(sql, "page_links"):
				return "id-hub|p7 hub page|31\n"
			}
			t.Errorf("sql %q", sql)
			return ""
		})}
	if err := o.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	events := f.all()
	// Before each operation: the background restore's job succeeded, then
	// the queue read empty.
	succeededAt, quietAt := -1, -1
	operations := 0
	for i, ev := range events {
		switch {
		case strings.HasSuffix(ev, "succeeded"):
			succeededAt = i
		case strings.HasPrefix(ev, "psql SELECT (SELECT => 0"):
			quietAt = i
		case strings.HasPrefix(ev, "delete "), strings.HasPrefix(ev, "move id-vault"), strings.HasPrefix(ev, "rename id-"):
			operations++
			if quietAt < 0 || quietAt < succeededAt {
				t.Errorf("%s at %d: the quiet last read at %d, the last restore succeeded at %d", ev, i, quietAt, succeededAt)
			}
			quietAt = -1
		}
	}
	if operations != 15 || f.count("delete id-vault") != 3 || f.count("delete id-f1") != 3 || f.count("job job-") != 18 {
		t.Errorf("%d operations: %v", operations, events)
	}
	// Moves under one home, the other, then the root; renames back to the
	// names.
	for _, want := range []string{"move id-vault under page-1", "move id-vault under page-2", "move id-vault under <nil>",
		"rename id-vault vault r1", "rename id-vault vault r2", "rename id-vault vault", "rename id-hub p7 hub page r1",
		"rename id-hub p7 hub page"} {
		if !slices.Contains(events, want) {
			t.Errorf("no %q: %v", want, events)
		}
	}
	if !strings.Contains(out.String(), `most-linked id=id-hub name="p7 hub page" linking_pages=31`) ||
		strings.Count(out.String(), "delete-vault status=204") != 3 {
		t.Errorf("out %q", out.String())
	}
}

// B8's import uploads the zip under the root page it makes, waits for the
// job and the quiet; the export waits for its job and downloads; each
// writes the WAL and the sizes around it (design 3.4.5).
func TestTransferImportsAndExports(t *testing.T) {
	dir := t.TempDir()
	zip := filepath.Join(dir, "vault.zip")
	if err := os.WriteFile(zip, []byte("PK zip bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	storage := filepath.Join(dir, "data")
	if err := os.MkdirAll(filepath.Join(storage, "imports"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storage, "imports", "x"), make([]byte, 100), 0o600); err != nil {
		t.Fatal(err)
	}
	api := &pageAPI{}
	var mu sync.Mutex
	polls := 0
	f := newFake(t, func(f *fake, w http.ResponseWriter, r *http.Request, body []byte) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v0/notebooks/nb-v/nodes":
			nodes := []any{}
			if f.count("create ") > 0 {
				nodes = append(nodes, map[string]any{"id": "page-1", "parent_id": nil, "name": "vault", "kind": "page"})
			}
			answer(w, http.StatusOK, map[string]any{"data": nodes})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v0/notebooks/nb-v/imports":
			_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil {
				t.Fatal(err)
			}
			form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(1 << 20)
			if err != nil {
				t.Fatal(err)
			}
			file, err := form.File["file"][0].Open()
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(file)
			f.note("import under %v of %q", form.Value["parent_id"], b)
			answer(w, http.StatusAccepted, map[string]any{"id": "job-i", "state": "queued"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v0/notebooks/nb-v/exports":
			f.note("export %s", body)
			answer(w, http.StatusAccepted, map[string]any{"id": "job-e", "state": "queued"})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v0/transfer-jobs/") && !strings.HasSuffix(r.URL.Path, "/download"):
			mu.Lock()
			polls++
			state := "running"
			if polls%2 == 0 {
				state = "succeeded"
			}
			mu.Unlock()
			f.note("job %s", state)
			var download any
			if state == "succeeded" && strings.HasSuffix(r.URL.Path, "/job-e") {
				download = map[string]any{"url": "/api/v0/transfer-jobs/job-e/download?e=1795000000&s=c2ln", "expires_at": "x"}
			}
			answer(w, http.StatusOK, map[string]any{"state": state, "download": download})
		case r.URL.Path == "/api/v0/transfer-jobs/job-e/download":
			if r.URL.RawQuery != "e=1795000000&s=c2ln" || r.Header.Get("Authorization") != "" {
				t.Errorf("download %q with %q", r.URL.RawQuery, r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(make([]byte, 4096))
		default:
			if !api.handle(f, w, r, body) {
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			}
		}
	})
	var out bytes.Buffer
	quiet := 0
	tr := Transfer{Client: f.clients(1, client.NewLog(nil))[0], Notebook: "nb-v", Root: "vault", Poll: 2 * time.Millisecond,
		Storage: storage, Out: &out,
		PSQL: fakePSQL(f, func(sql string) string {
			switch {
			case strings.HasPrefix(sql, "SELECT pg_current_wal_lsn()"):
				return "0/16B3748|123456789\n"
			case strings.HasPrefix(sql, "SELECT pg_wal_lsn_diff("):
				return "40960\n"
			case strings.Contains(sql, "page_search_queue"):
				if quiet++; quiet < 3 {
					return "7\n"
				}
				return "0\n"
			}
			t.Errorf("sql %q", sql)
			return ""
		})}
	if err := tr.Import(context.Background(), zip); err != nil {
		t.Fatal(err)
	}
	if err := tr.Export(context.Background(), filepath.Join(dir, "export.zip")); err != nil {
		t.Fatal(err)
	}
	events := f.all()
	if !slices.Contains(events, `import under [page-1] of "PK zip bytes"`) || !slices.Contains(events, `export {"root_id":"page-1"}`) ||
		f.count("create page-1 in nb-v under <nil> title vault") != 1 || quiet != 3 {
		t.Errorf("events %v, quiet read %d times", events, quiet)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "export.zip")); err != nil || len(got) != 4096 {
		t.Errorf("downloaded %d bytes, %v", len(got), err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], `"transfer":"import"`) || !strings.Contains(lines[0], `"wal_bytes":40960`) ||
		!strings.Contains(lines[0], `"Imports":100`) || !strings.Contains(lines[0], `"quiet_ms":`) || lines[1] != "download bytes=4096" ||
		!strings.Contains(lines[2], `"transfer":"export"`) {
		t.Errorf("out:\n%s", out.String())
	}
}
