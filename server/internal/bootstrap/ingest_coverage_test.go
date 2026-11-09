package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
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

// stop hands back copies: a query that started while the trace was active
// (River's fetch shares the pool) may end after stop and still add its rows
// under the lock, while the test reads what stop returned without it.
func (q *ingestQueryTrace) stop() (map[string]int, map[string]int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.active = false
	return maps.Clone(q.calls), maps.Clone(q.rows)
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
		assertNoIngestScan(t, calls)
		if count(t, f.tm.pool, "SELECT revision FROM indexed_pages WHERE node_id=$1", f.p) != 2 {
			t.Fatal("noncandidate positive write did not update its real index")
		}
	})
	t.Run("already_referenced", testIngestAlreadyReferenced)
	t.Run("diagnostics", testIngestDiagnostics)
}

// One debug line, with IDs, a reason and counts, only when a candidate meets
// too little evidence. Each leg reads the log from where it started.
func testIngestDiagnostics(t *testing.T) {
	type line struct {
		Msg, Reason, NotebookID, ChangesetID string
		Raw, Heads, Candidates, Suppressed   int
		Added                                int
		text                                 string
	}
	setup := func(t *testing.T) (ingestFixture, *syncBuffer) {
		t.Helper()
		logs := &syncBuffer{}
		tm := newAcmeTeamOn(t, "", "", nil, nil, slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
		return ingestFixtureOn(t, tm), logs
	}
	since := func(t *testing.T, logs *syncBuffer, start int) []line {
		t.Helper()
		var out []line
		for text := range strings.SplitSeq(logs.String()[start:], "\n") {
			if !strings.Contains(text, `"msg":"ingest `) {
				continue
			}
			var raw struct {
				Msg         string `json:"msg"`
				Reason      string `json:"reason"`
				NotebookID  string `json:"notebook_id"`
				ChangesetID string `json:"changeset_id"`
				Raw         int    `json:"raw_nodes"`
				Heads       int    `json:"eligible_heads"`
				Candidates  int    `json:"candidates"`
				Suppressed  int    `json:"suppressed"`
				Added       int    `json:"added"`
			}
			if err := json.Unmarshal([]byte(text), &raw); err != nil {
				t.Fatalf("%s: %v", text, err)
			}
			out = append(out, line{raw.Msg, raw.Reason, raw.NotebookID, raw.ChangesetID, raw.Raw, raw.Heads, raw.Candidates, raw.Suppressed, raw.Added, text})
		}
		return out
	}
	one := func(t *testing.T, f ingestFixture, logs *syncBuffer, start int, cs, msg, reason string, candidates, suppressed int) line {
		t.Helper()
		got := since(t, logs, start)
		if len(got) != 1 {
			t.Fatalf("diagnostics=%+v, want exactly one", got)
		}
		l := got[0]
		if l.Msg != msg || l.Reason != reason || l.NotebookID != f.nb || l.ChangesetID != cs || l.Candidates != candidates || l.Suppressed != suppressed {
			t.Fatalf("diagnostic=%+v, want %s/%s for %s with %d candidates, %d suppressed", l, msg, reason, cs, candidates, suppressed)
		}
		for _, private := range []string{"raw/", "[[", "Source"} {
			if strings.Contains(l.text, private) {
				t.Fatalf("diagnostic carries %q: %s", private, l.text)
			}
		}
		return l
	}
	t.Run("none_without_candidates", func(t *testing.T) {
		f, logs := setup(t)
		start := len(logs.String())
		f.write(t, f.p, "changed without links")
		if got := since(t, logs, start); len(got) != 0 {
			t.Fatalf("ordinary save logged %+v", got)
		}
	})
	t.Run("coverage_gap", func(t *testing.T) {
		f, logs := setup(t)
		q := f.tm.createPage(t, "alice", f.nb, "", "Q")
		ingestExec(t, f.tm, "UPDATE indexed_pages SET extractor=1 WHERE node_id=$1", q)
		start := len(logs.String())
		cs := f.write(t, f.p, "[[raw/T]]")
		assertIngestMarks(t, f.tm, cs)
		if l := one(t, f, logs, start, cs, "ingest coverage incomplete", "stale_extractor", 1, 1); l.Raw == 0 || l.Heads == 0 {
			t.Fatalf("coverage diagnostic lost its counts: %+v", l)
		}
	})
	t.Run("unknown_old_page", func(t *testing.T) {
		f, logs := setup(t)
		ingestExec(t, f.tm, "UPDATE indexed_pages SET extractor=1 WHERE node_id=$1", f.p)
		start := len(logs.String())
		cs := f.write(t, f.p, "[[raw/S]]")
		assertIngestMarks(t, f.tm, cs)
		if l := one(t, f, logs, start, cs, "ingest incomplete old evidence", "old_direct_unknown", 1, 1); l.Raw != 0 {
			t.Fatalf("unknown old page read the raw tree: %+v", l)
		}
	})
	t.Run("suppressed_asset", func(t *testing.T) {
		f, logs := setup(t)
		f.tm.upload(t, "alice", f.nb, f.raw, "a.txt", "followed asset")
		parent := f.tm.createPageWith(t, "alice", f.nb, f.raw, "Parent", "Source body [A](a.txt).")
		q := f.tm.createPageWith(t, "alice", f.nb, "", "Q", "[[raw/Parent]]")
		ingestExec(t, f.tm, "UPDATE indexed_pages SET extractor=1 WHERE node_id=$1", parent)
		start := len(logs.String())
		cs := f.batch(t, opWriteOn("Q.md", "changed [[raw/Parent]]", versionOf(q, 1)), opWriteOn("P.md", "[A](raw/a.txt) [[raw/T]]", versionOf(f.p, 1)))
		assertIngestMarks(t, f.tm, cs, f.t)
		if l := one(t, f, logs, start, cs, "ingest incomplete old evidence", "old_followed_unknown", 2, 1); l.Added != 1 || l.Raw == 0 {
			t.Fatalf("old evidence diagnostic lost its counts: %+v", l)
		}
	})
}

func assertNoIngestScan(t *testing.T, calls map[string]int) {
	t.Helper()
	for _, name := range []string{"IngestHeadsFirst", "IngestHeadsAfter", "IngestNovelSources", "LintSourceNodes"} {
		if calls[name] != 0 {
			t.Fatalf("save without a remaining candidate performed %s: %v", name, calls)
		}
	}
}

// A page saved again with the sources it already cites has no candidate left:
// what the page cited before takes them out before the whole-notebook read.
func testIngestAlreadyReferenced(t *testing.T) {
	trace := &ingestQueryTrace{}
	f := newIngestFixture(t, &wiring{queryTracer: trace})
	a := f.tm.upload(t, "alice", f.nb, f.raw, "a.txt", "asset")
	body := "[[raw/S]] [asset](raw/a.txt)"
	assertIngestMarks(t, f.tm, f.write(t, f.p, body), f.s, a.ID)

	trace.start()
	cs := f.write(t, f.p, "typo fixed "+body)
	calls, _ := trace.stop()
	assertIngestMarks(t, f.tm, cs)
	assertNoIngestScan(t, calls)

	trace.start()
	cs = f.write(t, f.p, body+" [[raw/T]]")
	calls, _ = trace.stop()
	assertIngestMarks(t, f.tm, cs, f.t)
	if calls["IngestHeadsFirst"] != 1 || calls["IngestNovelSources"] != 1 {
		t.Fatalf("a new reference beside old ones skipped the proof: %v", calls)
	}

	u := f.tm.createPageWith(t, "alice", f.nb, f.raw, "U", "Source U evidence.")
	ingestExec(t, f.tm, "UPDATE indexed_pages SET extractor=1 WHERE node_id=$1", f.p)
	trace.start()
	cs = f.write(t, f.p, body+" [[raw/U]]")
	calls, _ = trace.stop()
	assertIngestMarks(t, f.tm, cs)
	assertNoIngestScan(t, calls)
	if got := f.tm.resolvedFrom(t, f.p); !slices.Contains(got, u) {
		t.Fatalf("unknown old page lost its real new reference: %v", got)
	}
}
