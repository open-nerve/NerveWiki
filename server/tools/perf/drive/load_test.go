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
	log := client.NewLog(nil)
	e := Editors{Clients: f.clients(3, log), Notebook: "nb-ed", Bytes: 10 << 10, Links: 20, HeavyLinks: 500, Seed: 1,
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
	started := time.Now()
	before, after, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(started); took > e.For+5*time.Second {
		t.Errorf("the run took %v for %v", took, e.For)
	}
	if past := sentPast(log.Records(), firstSent(log.Records(), "session-open")+int64(e.For)); len(past) > 0 {
		t.Errorf("sent past the end: %v", past)
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
	if f.count("put-same ") != 0 {
		t.Errorf("a save of the content the page had: %v", events)
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
// events whose tree changed, merged within the interval, and not on
// another notebook's or on one without its tree, past the interval too;
// the agent's pages.
func TestTabsReadTheTreeOnItsEvents(t *testing.T) {
	api := &pageAPI{}
	var opened atomic.Int64 // the first event stream's, past the run's start
	f := newFake(t, func(f *fake, w http.ResponseWriter, r *http.Request, body []byte) {
		switch r.URL.Path {
		case "/api/v0/events":
			opened.CompareAndSwap(0, time.Now().UnixNano())
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
			// Past the interval, each alone: another notebook's tree, then
			// this one's pages without its tree; neither is read (review B's
			// M6).
			for _, e := range []string{
				`{"workspace_id":"w","notebook_id":"nb-other","tree":true,"pages":[]}`,
				`{"workspace_id":"w","notebook_id":"nb-tree","tree":false,"pages":[]}`,
			} {
				select {
				case <-time.After(350 * time.Millisecond):
				case <-r.Context().Done():
					return
				}
				_, _ = io.WriteString(w, "event: pages\ndata: "+e+"\n\n")
				w.(http.Flusher).Flush()
			}
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
	tb := Tabs{Clients: f.clients(3, log), Notebook: "nb-tree", For: 1000 * time.Millisecond, Every: 60 * time.Millisecond,
		Interval: 150 * time.Millisecond}
	started := time.Now()
	if err := tb.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(started); took > tb.For+5*time.Second {
		t.Errorf("the run took %v for %v", took, tb.For)
	}
	if past := sentPast(log.Records(), opened.Load()+int64(tb.For)); len(past) > 0 {
		t.Errorf("sent past the end: %v", past)
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

// A tree read the throttle schedules inside the run but whose time comes
// after For is not sent: the run waits for it and drops it (review B's N4;
// the round's x27).
func TestATabsReadDueAfterTheRunIsNotSent(t *testing.T) {
	f := newFake(t, func(f *fake, w http.ResponseWriter, r *http.Request, _ []byte) {
		switch r.URL.Path {
		case "/api/v0/events":
			w.Header().Set("Content-Type", "text/event-stream")
			e := "event: pages\ndata: {\"workspace_id\":\"w\",\"notebook_id\":\"nb-tree\",\"tree\":true,\"pages\":[]}\n\n"
			_, _ = io.WriteString(w, e)
			w.(http.Flusher).Flush()
			// Inside the run and inside the interval: the read it asks for
			// is due at 600 ms, past For's 300.
			select {
			case <-time.After(100 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
			_, _ = io.WriteString(w, e)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		case "/api/v0/notebooks/nb-tree/nodes":
			f.note("tree read")
			answer(w, http.StatusOK, map[string]any{"data": []any{}})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	tb := Tabs{Clients: f.clients(2, client.NewLog(nil)), Notebook: "nb-tree", For: 300 * time.Millisecond,
		Every: time.Hour, Interval: 600 * time.Millisecond}
	start := time.Now()
	if err := tb.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := f.count("tree read"); n != 1 {
		t.Errorf("the tree read %d times, want once: the read due after the run is not sent", n)
	}
	if took := time.Since(start); took < 550*time.Millisecond {
		t.Errorf("the run took %v: it did not wait for the read it had scheduled", took)
	}
}

// A tree read sent at once and still in flight at the end is waited for:
// the run returns with its record (review B's N8).
func TestATabsReadInFlightAtTheEndIsWaitedFor(t *testing.T) {
	f := newFake(t, func(f *fake, w http.ResponseWriter, r *http.Request, _ []byte) {
		switch r.URL.Path {
		case "/api/v0/events":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "event: pages\ndata: {\"workspace_id\":\"w\",\"notebook_id\":\"nb-tree\",\"tree\":true,\"pages\":[]}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		case "/api/v0/notebooks/nb-tree/nodes":
			select {
			case <-time.After(300 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
			answer(w, http.StatusOK, map[string]any{"data": []any{}})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	log := client.NewLog(nil)
	tb := Tabs{Clients: f.clients(2, log), Notebook: "nb-tree", For: 100 * time.Millisecond, Every: time.Hour,
		Interval: time.Second}
	if err := tb.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(log.Records(), func(r client.Record) bool {
		return r.Op == "tree" && r.Status == http.StatusOK && !r.Cut && r.Took() >= 300*time.Millisecond
	}) {
		t.Errorf("the read in flight at the end not waited for: %+v", log.Records())
	}
}

// Requests slower than their loop's period, so that a tick is ready when
// the end comes: past the end an editor sends its session's close and
// nothing else, the tree writer, the monitor and the tabs' agent nothing,
// and a run ends soon after it (review B's M7, M8). The end is at most the
// first request the loops send plus For. An editor beats its session
// before each save: the rounds take turns at a slow heartbeat, so that the
// end comes in it and the save after it must not go; at a slow save, so
// that the end comes in it and the tick and the end are ready at once; and
// at a renewal every save with a slow close, so that the end comes in the
// close and no session is opened after it (review B's N10, review C's
// N10). A select takes either at random, so nine short rounds.
func TestTheLoopsSendNothingPastTheirEnd(t *testing.T) {
	api := &pageAPI{}
	var opened, beat, save, closing atomic.Int64
	f := newFake(t, func(f *fake, w http.ResponseWriter, r *http.Request, body []byte) {
		slow := time.Duration(0)
		switch {
		case r.URL.Path == "/api/v0/events":
			opened.CompareAndSwap(0, time.Now().UnixNano())
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		case strings.HasSuffix(r.URL.Path, "/heartbeat"):
			slow = time.Duration(beat.Load())
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v0/edit-sessions/"):
			slow = time.Duration(closing.Load())
		case r.Method == http.MethodPut:
			slow = time.Duration(save.Load())
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/pages"),
			strings.HasPrefix(r.URL.Path, "/api/v0/nodes/"):
			slow = 50 * time.Millisecond // the tree writer's and the agent's
		}
		select {
		case <-time.After(slow):
		case <-r.Context().Done():
			return
		}
		if !api.handle(f, w, r, body) {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	var mu sync.Mutex
	var monitored []int64 // when the monitor's reads began
	psql := fakePSQL(f, func(sql string) string {
		switch {
		case strings.HasPrefix(sql, "SELECT pg_total_relation_size"):
			return "1|1\n"
		case strings.Contains(sql, "pg_stat_slru"):
			mu.Lock()
			monitored = append(monitored, time.Now().UnixNano())
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
		}
		return ""
	})
	for round := range 9 {
		renew := time.Minute
		switch round % 3 {
		case 0:
			beat.Store(int64(40 * time.Millisecond))
			save.Store(int64(5 * time.Millisecond))
			closing.Store(0)
		case 1:
			beat.Store(int64(5 * time.Millisecond))
			save.Store(int64(40 * time.Millisecond))
			closing.Store(0)
		case 2:
			beat.Store(0)
			save.Store(int64(5 * time.Millisecond))
			closing.Store(int64(40 * time.Millisecond))
			renew = time.Nanosecond
		}
		log := client.NewLog(nil)
		mu.Lock()
		monitored = nil
		mu.Unlock()
		e := Editors{Clients: f.clients(4, log), Notebook: "nb", Bytes: 1000, Links: 2, Seed: 1, For: 100 * time.Millisecond,
			Save: 10 * time.Millisecond, Heartbeat: time.Nanosecond, Renew: renew, Tree: 10 * time.Millisecond,
			Monitor: 10 * time.Millisecond, Monitored: io.Discard, PSQL: psql}
		started := time.Now()
		if _, _, err := e.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if took := time.Since(started); took > e.For+5*time.Second {
			t.Errorf("round %d: the editors took %v for %v", round, took, e.For)
		}
		end := firstSent(log.Records(), "session-open") + int64(e.For)
		if past := sentPast(log.Records(), end); len(past) > 0 {
			t.Errorf("round %d: the editors sent past the end: %v", round, past)
		}
		mu.Lock()
		if len(monitored) == 0 || slices.ContainsFunc(monitored, func(at int64) bool { return at > end+int64(stampSlack) }) {
			t.Errorf("round %d: the monitor read past the end, or never: %v, the end at most %d", round, monitored, end)
		}
		mu.Unlock()
		if closed := slices.IndexFunc(log.Records(), func(r client.Record) bool { return r.Op == "session-close" }); closed < 0 {
			t.Errorf("round %d: no session closed", round)
		}
		opened.Store(0)
		log = client.NewLog(nil)
		tb := Tabs{Clients: f.clients(2, log), Notebook: "nb", For: 100 * time.Millisecond, Every: 10 * time.Millisecond,
			Interval: time.Second}
		started = time.Now()
		if err := tb.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if took := time.Since(started); took > tb.For+5*time.Second {
			t.Errorf("round %d: the tabs took %v for %v", round, took, tb.For)
		}
		if past := sentPast(log.Records(), opened.Load()+int64(tb.For)); len(past) > 0 || firstSent(log.Records(), "agent-create") == 0 {
			t.Errorf("round %d: the agent sent past the end: %v, or nothing", round, past)
		}
	}
}

// stampSlack is how long past the end a request asked for before it may
// be stamped sent: a loop asks for the end before it builds a request, the
// client stamps the request as it sends it, and in between the goroutine
// can be put off. Without it, 4 of 200 runs of the package had an editor's
// save stamped past the bound these tests take for the end. The requests
// past the end the tests are for come a whole request later: 40 or 50 ms
// in TestTheLoopsSendNothingPastTheirEnd.
const stampSlack = 5 * time.Millisecond

// sentPast is how many requests of each op the records sent past at, by
// more than stampSlack, and how far past the latest went; an editor's
// session-close aside: the one request a loop sends past its end.
func sentPast(records []client.Record, at int64) map[string]string {
	past := map[string]string{}
	n, latest := map[string]int{}, map[string]time.Duration{}
	for _, r := range records {
		if r.Sent > at+int64(stampSlack) && r.Op != "session-close" {
			n[r.Op]++
			latest[r.Op] = max(latest[r.Op], time.Duration(r.Sent-at))
		}
	}
	for op, k := range n {
		past[op] = fmt.Sprintf("%d, the latest %v past", k, latest[op])
	}
	return past
}

// firstSent is when the records first sent op, 0 when they never did.
func firstSent(records []client.Record, op string) int64 {
	first := int64(0)
	for _, r := range records {
		if r.Op == op && (first == 0 || r.Sent < first) {
			first = r.Sent
		}
	}
	return first
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
	// Before each timed operation: the background restore's job succeeded,
	// then the queue read empty. The vault moved back to the root and the
	// names given back after them are not timed.
	untimed := []string{"move id-vault under <nil>", "rename id-vault vault", "rename id-hub p7 hub page"}
	succeededAt, quietAt := -1, -1
	operations := 0
	for i, ev := range events {
		switch {
		case strings.HasSuffix(ev, "succeeded"):
			succeededAt = i
		case strings.HasPrefix(ev, "psql SELECT (SELECT => 0"):
			quietAt = i
		case slices.Contains(untimed, ev):
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
	// Each timed move goes under another page than the one before, the
	// first away from the root; each timed rename to a new name; then the
	// vault back at the root, and the names back, once each.
	var moves, renames []string
	for _, ev := range events {
		switch {
		case strings.HasPrefix(ev, "move id-vault under "):
			moves = append(moves, strings.TrimPrefix(ev, "move id-vault under "))
		case strings.HasPrefix(ev, "rename id-vault "):
			renames = append(renames, strings.TrimPrefix(ev, "rename id-vault "))
		}
	}
	if !slices.Equal(moves, []string{"page-1", "page-2", "page-1", "<nil>"}) || !slices.Equal(renames, []string{"vault r1", "vault r2",
		"vault r3", "vault"}) || f.count("rename id-hub p7 hub page r") != 3 || f.count("rename id-hub p7 hub page") != 4 {
		t.Errorf("moves %v, renames %v: %v", moves, renames, events)
	}
	// One rep moves and renames too.
	one := o
	one.Reps, one.Out = 1, io.Discard
	before := len(f.all())
	if err := one.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var once []string
	for _, ev := range f.all()[before:] {
		if strings.HasPrefix(ev, "move id-vault") || strings.HasPrefix(ev, "rename id-vault") {
			once = append(once, ev)
		}
	}
	if !slices.Equal(once, []string{"move id-vault under page-3", "move id-vault under <nil>", "rename id-vault vault r1",
		"rename id-vault vault"}) {
		t.Errorf("one rep: %v", once)
	}
	if !strings.Contains(out.String(), `most-linked id=id-hub name="p7 hub page" linking_pages=31`) ||
		strings.Count(out.String(), "delete-vault status=204") != 3 {
		t.Errorf("out %q", out.String())
	}
}

// B4's operations wait for the whole background's quiet by default, for
// the notebook's alone with QuietNotebook, as -quiet notebook asks under
// the editors' load (M12/P4/S7 design 6.3 item 5).
func TestOpsWaitForTheQuietTheyAreToldOf(t *testing.T) {
	for _, notebook := range []bool{false, true} {
		var reads []string
		o := Ops{Notebook: "nb-v", Idle: time.Millisecond, Poll: time.Millisecond, QuietNotebook: notebook,
			PSQL: client.PSQL{Container: "c0ffee", Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
				reads = append(reads, args[len(args)-1])
				if len(reads) < 2 {
					return []byte("1\n"), nil
				}
				return []byte("0\n"), nil
			}}}
		if err := o.quiet(context.Background()); err != nil {
			t.Fatal(err)
		}
		want := "SELECT (SELECT count(*) FROM page_search_queue) + (SELECT count(*) FROM river_job WHERE state IN ('running', 'available'))"
		if notebook {
			want = "SELECT (SELECT count(*) FROM page_search_queue WHERE notebook_id = 'nb-v') + " +
				"(SELECT count(*) FROM river_job WHERE state IN ('running', 'available') AND kind <> 'page.index_search')"
		}
		if len(reads) != 2 || reads[0] != want || reads[1] != want {
			t.Errorf("QuietNotebook %t read %q; want %q until quiet", notebook, reads, want)
		}
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
			var download, finished any
			if state == "succeeded" {
				finished = "2026-11-17T10:00:03.25Z"
			}
			if state == "succeeded" && strings.HasSuffix(r.URL.Path, "/job-e") {
				download = map[string]any{"url": "/api/v0/transfer-jobs/job-e/download?e=1795000000&s=c2ln", "expires_at": "x"}
			}
			answer(w, http.StatusOK, map[string]any{"state": state, "download": download, "created_at": "2026-11-17T10:00:00Z",
				"started_at": "2026-11-17T10:00:01Z", "finished_at": finished})
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
		!strings.Contains(lines[2], `"transfer":"export"`) || !strings.Contains(lines[0], `"job_server_ms":3250`) ||
		!strings.Contains(lines[2], `"job_running_ms":2250`) {
		t.Errorf("out:\n%s", out.String())
	}
}
