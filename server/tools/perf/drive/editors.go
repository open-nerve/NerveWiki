package drive

import (
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

// Editors is the editors' load (design 3.4.5; M4-performance 1 and 6,
// batch G): N editors, a page each of Bytes and Links links (the first
// of HeavyLinks when it is positive), each in an edit session, changing a
// byte and writing every Save as the web's autosave, the session's
// heartbeat every Heartbeat, and the session closed and another opened
// every Renew; a tree writer creating, renaming and moving in turn every
// Tree; psql's monitor every Monitor; for For, the requests in flight then
// waited for, Drain at most (DefaultDrain when 0); before and after,
// page_revisions rewritten by VACUUM FULL and its size and rows read.
type Editors struct {
	Clients    []*client.Client // the editors' and, last, the tree writer's
	Notebook   string
	Bytes      int
	Links      int
	HeavyLinks int
	Seed       uint64
	For        time.Duration
	Drain      time.Duration
	Save       time.Duration
	Heartbeat  time.Duration
	Renew      time.Duration
	Tree       time.Duration
	Monitor    time.Duration
	PSQL       client.PSQL
	Monitored  io.Writer // psql's monitor lines
}

// Versions is page_revisions' size, table, TOAST and indexes, and rows.
type Versions struct{ Bytes, Rows int64 }

// versions rewrites page_revisions and reads it.
func (e Editors) versions(ctx context.Context) (Versions, error) {
	if _, err := e.PSQL.Query(ctx, "VACUUM FULL page_revisions"); err != nil {
		return Versions{}, err
	}
	rows, err := e.PSQL.Query(ctx, "SELECT pg_total_relation_size('page_revisions'), (SELECT count(*) FROM page_revisions)")
	if err != nil {
		return Versions{}, err
	}
	if len(rows) != 1 || len(rows[0]) != 2 {
		return Versions{}, fmt.Errorf("page_revisions read as %v", rows)
	}
	var v Versions
	if _, err := fmt.Sscan(rows[0][0]+" "+rows[0][1], &v.Bytes, &v.Rows); err != nil {
		return Versions{}, err
	}
	return v, nil
}

// Run runs the load: page_revisions before and after.
func (e Editors) Run(ctx context.Context) (before, after Versions, err error) {
	n := len(e.Clients) - 1
	if n < 1 {
		return before, after, fmt.Errorf("%d clients: an editor and the tree writer at least", len(e.Clients))
	}
	run := time.Now().UnixMilli()
	pages := make([]page, n)
	contents := make([]string, n)
	for i := range pages {
		links := e.Links
		if i == 0 && e.HeavyLinks > 0 {
			links = e.HeavyLinks
		}
		contents[i] = LinkedBody(e.Seed+uint64(i), e.Bytes, links)
		if pages[i], err = createPage(ctx, e.Clients[i], "setup-page", e.Notebook, "", fmt.Sprintf("editor %d %d", run, i), contents[i]); err != nil {
			return before, after, err
		}
	}
	tree := e.Clients[n]
	var homes [2]page
	for i := range homes {
		if homes[i], err = createPage(ctx, tree, "setup-page", e.Notebook, "", fmt.Sprintf("tree %d %c", run, 'a'+i), ""); err != nil {
			return before, after, err
		}
	}
	if before, err = e.versions(ctx); err != nil {
		return before, after, err
	}
	stop, send, cancel := ending(ctx, time.Now().Add(e.For), e.Drain)
	defer cancel()
	var wg sync.WaitGroup
	for i := range pages {
		wg.Go(func() { e.edit(stop, send, e.Clients[i], pages[i], contents[i]) })
	}
	wg.Go(func() { e.treeWrites(stop, send, tree, homes, run) })
	if e.Monitored != nil {
		wg.Go(func() { e.monitor(stop, send) })
	}
	wg.Wait()
	after, err = e.versions(ctx)
	return before, after, err
}

// edit is an editor's loop until stop ends, its requests sent in send;
// its session closed at its end.
func (e Editors) edit(stop, send context.Context, c *client.Client, p page, content string) {
	session, err := openSession(send, c, "session-open", p.ID)
	if err != nil {
		return
	}
	base, beat, opened := p.Revision, time.Now(), time.Now()
	tick := time.NewTicker(e.Save)
	defer tick.Stop()
	for k := 0; ; k++ {
		select {
		case <-stop.Done():
			e.closeSession(c, session)
			return
		case <-tick.C:
		}
		if time.Since(opened) >= e.Renew {
			e.closeSession(c, session)
			if session, err = openSession(send, c, "session-open", p.ID); err != nil {
				return
			}
			beat, opened = time.Now(), time.Now()
		}
		if time.Since(beat) >= e.Heartbeat {
			_, _ = c.JSON(send, "heartbeat", http.MethodPost, "/api/v0/edit-sessions/"+url.PathEscape(session)+"/heartbeat", nil, nil,
				http.StatusOK)
			beat = time.Now()
		}
		if w, err := putContent(send, c, "save", p.ID, changed(content, k), base, session); err == nil {
			base = w.Revision
		}
	}
}

// closeSession ends a session, past the run's end.
func (e Editors) closeSession(c *client.Client, session string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = c.JSON(ctx, "session-close", http.MethodDelete, "/api/v0/edit-sessions/"+url.PathEscape(session), nil, nil,
		http.StatusNoContent)
}

// treeWrites creates a page under the first home, renames it, moves it to
// the other home, in turn, one every e.Tree, until stop ends.
func (e Editors) treeWrites(stop, send context.Context, c *client.Client, homes [2]page, run int64) {
	tick := time.NewTicker(e.Tree)
	defer tick.Stop()
	var last string
	for k := 0; ; k++ {
		select {
		case <-stop.Done():
			return
		case <-tick.C:
		}
		switch {
		case k%3 == 0 || last == "":
			if p, err := createPage(send, c, "tree-create", e.Notebook, homes[0].ID, fmt.Sprintf("t %d %d", run, k), ""); err == nil {
				last = p.ID
			}
		case k%3 == 1:
			_, _ = c.JSON(send, "tree-rename", http.MethodPatch, "/api/v0/nodes/"+url.PathEscape(last),
				map[string]string{"name": fmt.Sprintf("t %d %d renamed", run, k)}, nil, http.StatusOK)
		default:
			_, _ = c.JSON(send, "tree-move", http.MethodPost, "/api/v0/nodes/"+url.PathEscape(last)+"/move",
				map[string]any{"parent_id": homes[1].ID}, nil, http.StatusOK)
		}
	}
}

// monitor writes MultiXact's SLRU and the client backends' waits every
// e.Monitor until stop ends, a JSON line each.
func (e Editors) monitor(stop, send context.Context) {
	tick := time.NewTicker(e.Monitor)
	defer tick.Stop()
	for {
		select {
		case <-stop.Done():
			return
		case <-tick.C:
		}
		slru, err1 := e.PSQL.Query(send, "SELECT name, blks_zeroed, blks_hit, blks_read, blks_written, blks_exists, flushes, truncates "+
			"FROM pg_stat_slru WHERE lower(name) LIKE 'multixact%'")
		waits, err2 := e.PSQL.Query(send, "SELECT coalesce(wait_event_type, ''), coalesce(wait_event, ''), count(*) FROM pg_stat_activity "+
			"WHERE backend_type = 'client backend' AND wait_event IS NOT NULL GROUP BY 1, 2 ORDER BY 3 DESC")
		line := map[string]any{"unix_ns": time.Now().UnixNano(), "slru": slru, "waits": waits}
		if err1 != nil || err2 != nil {
			line["error"] = strings.TrimSpace(fmt.Sprint(err1, " ", err2))
		}
		b, err := json.Marshal(line)
		if err != nil {
			continue
		}
		_, _ = e.Monitored.Write(append(b, '\n'))
	}
}
