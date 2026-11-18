package drive

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/open-nerve/NerveWiki/server/tools/perf/client"
)

// TreeInterval is how often a tab reads a notebook's tree at most
// (web/apps/web/src/events/handlers.ts's TREE_INTERVAL_MS).
const TreeInterval = 500 * time.Millisecond

// Throttle is the web's refresher for one key (web/apps/web/src/events/
// refresher.ts): a request reads at once when the interval since the last
// read has passed, else once at its end, the requests before it merged
// into that one; a read is not waited for, so that one may start while the
// last goes on, as the refresher's does. Track, when given, counts each
// read from its request to its end, so that a run waits for them.
type Throttle struct {
	Interval time.Duration
	Now      func() time.Time
	After    func(d time.Duration, f func()) // time.AfterFunc's
	Track    *sync.WaitGroup
	mu       sync.Mutex
	last     time.Time
	read     bool // whether there was a read
	pending  bool // whether a read is scheduled
}

// NewThrottle is a throttle of interval on the wall clock.
func NewThrottle(interval time.Duration) *Throttle {
	return &Throttle{Interval: interval, Now: time.Now, After: func(d time.Duration, f func()) { time.AfterFunc(d, f) }}
}

// Request asks for read: now, at the end of the interval, or not at all
// when a read is scheduled already, which keeps its time.
func (t *Throttle) Request(read func()) {
	t.mu.Lock()
	if t.pending {
		t.mu.Unlock()
		return
	}
	now := t.Now()
	wait := t.last.Add(t.Interval).Sub(now)
	if !t.read || wait <= 0 {
		t.last, t.read = now, true
		t.mu.Unlock()
		if t.Track != nil {
			t.Track.Go(read)
		} else {
			go read()
		}
		return
	}
	t.pending = true
	t.mu.Unlock()
	if t.Track != nil {
		t.Track.Add(1)
	}
	t.After(wait, func() {
		if t.Track != nil {
			defer t.Track.Done()
		}
		t.mu.Lock()
		t.pending, t.last = false, t.Now()
		t.mu.Unlock()
		read()
	})
}

// Event is a server-sent event: its type and data.
type Event struct {
	Type string
	Data string
}

// ReadEvents reads a text/event-stream, each event to on, until it ends.
func ReadEvents(r io.Reader, on func(Event)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	var e Event
	var data []string
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if e.Type != "" || len(data) > 0 {
				e.Data = strings.Join(data, "\n")
				on(e)
			}
			e, data = Event{}, nil
		case strings.HasPrefix(line, "event:"):
			e.Type = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	return sc.Err()
}

// Tabs is the tree's reread (design 3.4.5, 3.7's last row): K tabs, an
// event stream each, each reading the notebook's whole tree, as the
// refresher does, on a pages event of the notebook whose tree changed;
// an agent creating a page every Every; for For, the reads in flight then
// waited for, Drain at most (DefaultDrain when 0), and a read due after
// For not sent.
type Tabs struct {
	Clients  []*client.Client // the tabs', and last the agent's
	Notebook string
	For      time.Duration
	Drain    time.Duration
	Every    time.Duration
	Interval time.Duration
}

// Run runs it; the tree reads are op tree, their bytes in the records.
func (tb Tabs) Run(ctx context.Context) error {
	k := len(tb.Clients) - 1
	if k < 1 {
		return fmt.Errorf("%d clients: a tab and the agent at least", len(tb.Clients))
	}
	stop, send, cancel := ending(ctx, time.Now().Add(tb.For), tb.Drain)
	defer cancel()
	var wg, reads sync.WaitGroup
	streams := make(chan error, k)
	for i := range k {
		c := tb.Clients[i]
		throttle := NewThrottle(tb.Interval)
		throttle.Track = &reads
		wg.Go(func() {
			err := tb.stream(stop, c, func() {
				if past(stop) {
					return
				}
				_, _, _ = c.Send(send, client.Request{Op: "tree", Method: http.MethodGet,
					Path: "/api/v0/notebooks/" + url.PathEscape(tb.Notebook) + "/nodes"}, io.Discard)
			}, throttle)
			if stop.Err() == nil {
				streams <- err
			}
		})
	}
	wg.Go(func() {
		agent := tb.Clients[k]
		tick := time.NewTicker(tb.Every)
		defer tick.Stop()
		run := time.Now().UnixMilli()
		for n := 0; ; n++ {
			select {
			case <-stop.Done():
				return
			case <-tick.C:
			}
			if past(stop) {
				return
			}
			_, _ = createPage(send, agent, "agent-create", tb.Notebook, "", fmt.Sprintf("tab agent %d %d", run, n), "")
		}
	})
	wg.Wait()
	reads.Wait()
	close(streams)
	if err, ended := <-streams; ended {
		return fmt.Errorf("an event stream ended before the run: %w", err)
	}
	return nil
}

// stream reads c's event stream, asking throttle to read on each pages
// event of the notebook whose tree changed.
func (tb Tabs) stream(ctx context.Context, c *client.Client, read func(), throttle *Throttle) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+"/api/v0/events", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("the event stream answered %d", res.StatusCode)
	}
	return ReadEvents(res.Body, func(e Event) {
		if e.Type != "pages" {
			return
		}
		var p struct {
			NotebookID string `json:"notebook_id"`
			Tree       bool   `json:"tree"`
		}
		if json.Unmarshal([]byte(e.Data), &p) == nil && p.Tree && p.NotebookID == tb.Notebook {
			throttle.Request(read)
		}
	})
}
