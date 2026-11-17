package drive

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/open-nerve/NerveWiki/server/tools/perf/client"
)

// BusyWait is how long the parse budget and the large bodies' slots wait
// before 503 server_busy (platform/markdown/budget.go, bootstrap/mcp.go;
// design 2.3's P4-21).
const BusyWait = 2 * time.Second

// EarlyBusy reports whether r is a 503 whose head came within BusyWait of
// its sending, at the client: a busy answer that did not wait (B2's
// criterion).
func EarlyBusy(r client.Record) bool {
	return r.Status == http.StatusServiceUnavailable && r.ToHeader() < BusyWait
}

// Bodies is B2's run (design 3.6): K credentials, ceil(K/2) reading a
// page's reading view each, floor(K/2) writing a page's content each in an
// edit session, each in a loop, over Warmup and then For, the requests in
// flight then waited for, Drain at most (DefaultDrain when 0); a page of
// Size bytes of Kind (normal, pathological). Readers' pages are in the
// notebook Read; writer w's in Writers[w], or all in Writers[0] when
// there is one.
type Bodies struct {
	Clients   []*client.Client
	Kind      string
	Size      int
	Seed      uint64
	Read      string
	Writers   []string
	Warmup    time.Duration
	For       time.Duration
	Drain     time.Duration
	Heartbeat time.Duration // the edit session's, 20 s as the web's
}

// Content is the run's page content.
func (b Bodies) Content() (string, error) {
	switch b.Kind {
	case "normal":
		return NormalBody(b.Seed, b.Size), nil
	case "pathological":
		return PathologicalBody(b.Size), nil
	}
	return "", fmt.Errorf("a body of kind %q, want normal or pathological", b.Kind)
}

// Run makes the pages, runs the loops, and answers when they measured
// from: the end of the warmup.
func (b Bodies) Run(ctx context.Context) (time.Time, error) {
	content, err := b.Content()
	if err != nil {
		return time.Time{}, err
	}
	readers := (len(b.Clients) + 1) / 2
	type writer struct {
		page    page
		session string
	}
	pages := make([]string, readers)
	writers := make([]writer, len(b.Clients)-readers)
	run := time.Now().UnixMilli()
	for i := range pages {
		p, err := createPage(ctx, b.Clients[i], "setup-page", b.Read, "", fmt.Sprintf("read %s %d %d", b.Kind, run, i), content)
		if err != nil {
			return time.Time{}, err
		}
		pages[i] = p.ID
	}
	for w := range writers {
		c := b.Clients[readers+w]
		notebook := b.Writers[0]
		if len(b.Writers) > 1 {
			notebook = b.Writers[w]
		}
		p, err := createPage(ctx, c, "setup-page", notebook, "", fmt.Sprintf("write %s %d %d", b.Kind, run, w), content)
		if err != nil {
			return time.Time{}, err
		}
		s, err := openSession(ctx, c, "setup-session", p.ID)
		if err != nil {
			return time.Time{}, err
		}
		writers[w] = writer{page: p, session: s}
	}
	start := time.Now()
	from, end := start.Add(b.Warmup), start.Add(b.Warmup+b.For)
	stop, send, cancel := ending(ctx, end, b.Drain)
	defer cancel()
	var wg sync.WaitGroup
	for i, id := range pages {
		wg.Go(func() {
			for stop.Err() == nil {
				_, _, _ = b.Clients[i].Send(send, client.Request{Op: "view", Method: http.MethodGet,
					Path: "/api/v0/pages/" + url.PathEscape(id) + "/view"}, io.Discard)
			}
		})
	}
	for w, wr := range writers {
		c := b.Clients[readers+w]
		wg.Go(func() {
			base, beat := wr.page.Revision, time.Now()
			for k := 0; stop.Err() == nil; k++ {
				if time.Since(beat) >= b.Heartbeat {
					_, _ = c.JSON(send, "heartbeat", http.MethodPost, "/api/v0/edit-sessions/"+url.PathEscape(wr.session)+"/heartbeat",
						nil, nil, http.StatusOK)
					beat = time.Now()
				}
				if p, err := putContent(send, c, "write", wr.page.ID, changed(content, k), base, wr.session); err == nil {
					base = p.Revision
				}
			}
		})
	}
	wg.Wait()
	return from, nil
}

// BodiesSummary is a run's busy answers, sent from from: all, and those
// early.
func BodiesSummary(records []client.Record, from time.Time) (busy, early int) {
	for _, r := range records {
		if r.Sent < from.UnixNano() || (r.Op != "view" && r.Op != "write") {
			continue
		}
		if r.Status == http.StatusServiceUnavailable {
			busy++
		}
		if EarlyBusy(r) {
			early++
		}
	}
	return busy, early
}
