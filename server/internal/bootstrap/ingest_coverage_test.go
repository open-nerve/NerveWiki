package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

type ingestQueryTrace struct {
	mu     sync.Mutex
	active bool
	calls  map[string]int
	rows   map[string]int64
}
type ingestQueryName struct{}

func (q *ingestQueryTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.active || !strings.HasPrefix(d.SQL, "-- name: ") {
		return ctx
	}
	name := strings.Fields(d.SQL)[2]
	q.calls[name]++
	return context.WithValue(ctx, ingestQueryName{}, name)
}
func (q *ingestQueryTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	name, ok := ctx.Value(ingestQueryName{}).(string)
	if !ok {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rows[name] += d.CommandTag.RowsAffected()
}
func (q *ingestQueryTrace) start() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.calls = map[string]int{}
	q.rows = map[string]int64{}
	q.active = true
}
func (q *ingestQueryTrace) stop() (map[string]int, map[string]int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.active = false
	return q.calls, q.rows
}

func TestIngestCoverage(t *testing.T) {
	t.Run("zero_raw_eof", testIngestZeroRawEOF)
	t.Run("budget_between", testIngestBudgetBetween)
	for _, state := range []string{"missing_unrelated", "stale_unrelated", "head_revision", "healthy"} {
		t.Run(state, func(t *testing.T) {
			f := newIngestFixture(t, nil)
			q := f.tm.createPageWith(t, "alice", f.nb, "", "Q", "[[raw/S]]")
			switch state {
			case "missing_unrelated":
				ingestExec(t, f.tm, "DELETE FROM indexed_pages WHERE node_id=$1", q)
			case "stale_unrelated":
				ingestExec(t, f.tm, "UPDATE indexed_pages SET extractor=1 WHERE node_id=$1", q)
			case "head_revision":
				ingestExec(t, f.tm, "UPDATE indexed_pages SET revision=9 WHERE node_id=$1", q)
			}
			cs := f.write(t, f.p, "[[raw/S]] [[raw/T]]")
			if state == "healthy" {
				assertIngestMarks(t, f.tm, cs, f.t)
			} else {
				assertIngestMarks(t, f.tm, cs)
			}
			f.tm.reindex(t, f.nb)
			checkPages(t, f.tm.pool)
			checkLinks(t, f.tm.pool)
		})
	}
	t.Run("raw_eof", func(t *testing.T) {
		trace := &ingestQueryTrace{}
		f := newIngestFixture(t, &wiring{queryTracer: trace})
		pages := make([]sqlPage, 1024)
		for i := range pages {
			pages[i] = sqlPage{f.raw, fmt.Sprintf("Source empty %04d", i), ""}
		}
		seedPagesSQL(t, f.tm.pool, f.nb, pages)
		q := f.tm.createPageWith(t, "alice", f.nb, "", "Tail Q", "[[raw/S]]")
		// Keep non-live rows in the raw relation: only complete metadata EOF
		// certifies the unrelated final wiki, not the first eligible count.
		ingestExec(t, f.tm, `UPDATE nodes SET deleted_at=now() WHERE notebook_id=$1 AND name LIKE 'Source empty %' AND name<'Source empty 0256'`, f.nb)
		ingestExec(t, f.tm, `UPDATE page_contents c SET deleted_at=n.deleted_at FROM nodes n WHERE n.id=c.node_id AND n.notebook_id=$1 AND n.deleted_at IS NOT NULL`, f.nb)
		trace.start()
		cs := f.write(t, f.p, "[[raw/S]] [[raw/T]]")
		calls, rows := trace.stop()
		assertIngestMarks(t, f.tm, cs, f.t)
		raw := count(t, f.tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1", f.nb)
		if raw < 1025 || rows["IngestHeadsFirst"]+rows["IngestHeadsAfter"] != int64(raw) || calls["IngestHeadsAfter"] < 2 {
			t.Fatalf("raw EOF counts calls=%v rows=%v raw=%d", calls, rows, raw)
		}
		if n := count(t, f.tm.pool, "SELECT count(*) FROM page_links WHERE source_id=$1 AND resolved_id=$2", q, f.s); n != 1 {
			t.Fatal("tail oracle lacks its real blocking edge")
		}
	})
	t.Run("page_content_missing", func(t *testing.T) {
		f := newIngestFixture(t, nil)
		q := f.tm.createPage(t, "alice", f.nb, "", "Broken Q")
		ingestExec(t, f.tm, "DELETE FROM page_contents WHERE node_id=$1", q)
		n := count(t, f.tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", f.nb)
		f.tm.send(t, contentWrite("alice", f.p, "[[raw/T]]", 1, ""), http.StatusInternalServerError)
		p := f.tm.content(t, "alice", f.p)
		if p.Revision != 1 || p.Content != "" || count(t, f.tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", f.nb) != n || count(t, f.tm.pool, "SELECT revision FROM indexed_pages WHERE node_id=$1", f.p) != 1 {
			t.Fatal("coverage metadata error did not roll back the exact write/index/history")
		}
	})
	t.Run("no_candidates", func(t *testing.T) {
		trace := &ingestQueryTrace{}
		f := newIngestFixture(t, &wiring{queryTracer: trace})
		trace.start()
		cs := f.write(t, f.p, "a real changed body without links")
		calls, _ := trace.stop()
		assertIngestMarks(t, f.tm, cs)
		for _, name := range []string{"IngestHeadsFirst", "IngestHeadsAfter", "IngestNovelSources", "LintSourceNodes"} {
			if calls[name] != 0 {
				t.Fatalf("noncandidate save performed %s: %v", name, calls)
			}
		}
		if count(t, f.tm.pool, "SELECT revision FROM indexed_pages WHERE node_id=$1", f.p) != 2 {
			t.Fatal("noncandidate positive write did not update its real index")
		}
	})
}
