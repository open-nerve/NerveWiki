//go:build !race

package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// treeLinkReads counts, between start and stop, the statements of the reads
// of a reach's links a request runs, by the name sqlc gives each, the
// whole name: LinksReached is the start of the other two.
type treeLinkReads struct {
	mu     sync.Mutex
	active bool
	counts map[string]int
}

// treeLinkReadNames are the statements counted: the setting that chooses the
// wide read, and the three reads (M12/P4/S7 design 3.1).
func treeLinkReadNames() []string {
	return []string{"PlanCacheMode", "LinksReached", "LinksReachedWide", "LinksReachedWideGeneric"}
}

func (r *treeLinkReads) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	fields := strings.Fields(strings.SplitN(q.SQL, "\n", 2)[0])
	if len(fields) != 4 || fields[0] != "--" || fields[1] != "name:" {
		return ctx
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, name := range treeLinkReadNames() {
		if r.active && fields[2] == name {
			r.counts[name]++
		}
	}
	return ctx
}

func (*treeLinkReads) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (r *treeLinkReads) start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active, r.counts = true, map[string]int{}
}

// stop answers the counts since start, each of treeLinkReadNames, 0 for none.
func (r *treeLinkReads) stop() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = false
	out := map[string]int{}
	for _, name := range treeLinkReadNames() {
		out[name] = r.counts[name]
	}
	return out
}

// The local means of two runs each (M12/P4/S7 design 4.4; 8 cores, arm64,
// through heavy.sh, ms): a move without statistics 444.3, 439.3; the
// fastest of three renames 403.0, 399.8, of three moves 434.2, 418.3; the
// deletion 580.8, 572.8. The bound of each operation is the larger of 3.6
// times its mean (three times, and 2.4 × 1.25 × 1.2, the larger) and its
// mean and 2 s, its time sub-second and a deletion or a move without
// statistics a single sample: here the latter, each.
const (
	treeMoveWithoutStatistics = 442 * time.Millisecond
	treeRename                = 401 * time.Millisecond
	treeMove                  = 426 * time.Millisecond
	treeDelete                = 577 * time.Millisecond
)

func treeBound(mean time.Duration) time.Duration {
	return max(mean*36/10, mean+2*time.Second)
}

// A top of about 10,000 nodes that no link leads to, moved, renamed and
// deleted through REST, as B4's vault (M12/P4/S7 design 4.4; decision 660):
// a move and a rename read the links they reach once, wide (A and B), and
// write no page; a deletion reads them once, wide; each within its bound,
// which guards the request's deadline, not B4's room (6.3 measures that).
// A move without statistics first, then, analyzed, the fastest of three
// renames and of three moves, and the deletion, after which the index has
// none of the top's rows.
func TestTreeWritesOfALargeTopReadTheLinksOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("a top of 10,000 nodes moved, renamed and deleted through serve")
	}
	reads := &treeLinkReads{}
	bench := newTrashBenchTraced(t, reads, func(*config.Config) {}, func(*page.Deps) {})
	var fsync string
	if err := bench.pool.QueryRow(context.Background(), "SHOW fsync").Scan(&fsync); err != nil || fsync != "off" {
		t.Fatalf("fsync is %q (%v): the bounds are of a database without it", fsync, err)
	}
	bench.importOf(t, shapeDeep, 10_000)
	top, q := bench.rootPage(t, "top"), bench.rootPage(t, "q")
	p0 := md5ID(bench.nb, "0")
	bench.exec(t, "UPDATE nodes SET parent_id = $2 WHERE id = $1", p0, top)
	bench.exec(t, "UPDATE changeset_items i SET after_parent_id = $2 FROM changesets c WHERE c.id = i.changeset_id AND c.kind = 'import' AND i.node_id = $1", p0, top)
	if n := count(t, bench.pool, `WITH RECURSIVE sub AS (SELECT id FROM nodes WHERE id = $1
		UNION ALL SELECT n.id FROM nodes n JOIN sub s ON n.parent_id = s.id) SELECT count(*) FROM sub`, top); n != 10_001 {
		t.Fatalf("the top's subtree holds %d nodes, want 10,001", n)
	}
	if n := count(t, bench.pool, "SELECT count(*) FROM page_links WHERE resolved_id = $1 OR target_key IN ('top', 'top2')", top); n != 0 {
		t.Fatalf("%d links lead to the top: its participant would write them", n)
	}
	versions := count(t, bench.pool, "SELECT count(*) FROM page_revisions")
	request := func(what, method, path, body string, want int, statements map[string]int) time.Duration {
		t.Helper()
		reads.start()
		began := time.Now()
		status, answer := bench.serve(method, path, body)
		took := time.Since(began)
		got := reads.stop()
		if status != want {
			t.Fatalf("%s = %d %s", what, status, answer)
		}
		// Errors, not fatal: a run that reads otherwise still times every
		// operation (the design's reverse controls, 6.2).
		if !maps.Equal(got, statements) {
			t.Errorf("%s read %v; want %v", what, got, statements)
		}
		if method != http.MethodDelete {
			if n := count(t, bench.pool, "SELECT count(*) FROM page_revisions"); n != versions {
				t.Errorf("%s wrote %d versions: it wrote pages again", what, n-versions)
				versions = n
			}
		}
		return took
	}
	once := map[string]int{"PlanCacheMode": 1, "LinksReached": 0, "LinksReachedWide": 1, "LinksReachedWideGeneric": 0}
	move := func(to uuid.UUID) time.Duration {
		parent := "null"
		if to != uuid.Nil() {
			parent = `"` + to.String() + `"`
		}
		return request("move the top", http.MethodPost, "/api/v0/nodes/"+top.String()+"/move", `{"parent_id":`+parent+`}`, http.StatusOK, once)
	}
	names := []string{"top2", "top"}
	rename := func() time.Duration {
		name := names[0]
		names[0], names[1] = names[1], names[0]
		return request("rename the top", http.MethodPatch, "/api/v0/nodes/"+top.String(), `{"name":"`+name+`"}`, http.StatusOK, once)
	}
	type timing struct {
		what       string
		took, mean time.Duration
	}
	timed := []timing{{"a move without statistics", move(q), treeMoveWithoutStatistics}}
	move(uuid.Nil())
	bench.analyze(t)
	timed = append(timed, timing{"a rename", fastestOf(t, "a rename of the top", rename), treeRename})
	at, i := []uuid.UUID{q, uuid.Nil()}, 0 // to q, back, to q again
	timed = append(timed, timing{"a move", fastestOf(t, "a move of the top", func() time.Duration { i++; return move(at[(i+1)%2]) }), treeMove})
	timed = append(timed, timing{"the deletion",
		request("delete the top", http.MethodDelete, "/api/v0/nodes/"+top.String(), "", http.StatusNoContent, once), treeDelete})
	if n := count(t, bench.pool, `SELECT count(*) FROM page_links l JOIN nodes n ON n.id = l.source_id WHERE n.deleted_at IS NOT NULL`); n != 0 {
		t.Errorf("the index holds %d links of the pages deleted", n)
	}
	checkPages(t, bench.pool)
	for _, x := range timed {
		t.Logf("%s took %s", x.what, x.took)
		if bound := treeBound(x.mean); x.took > bound {
			t.Errorf("%s took %s, past %s (the local mean %s)", x.what, x.took, bound, x.mean)
		}
	}
}

// rootPage creates a page of title at the notebook's root through REST,
// empty, and answers its id.
func (b trashBench) rootPage(tb testing.TB, title string) uuid.UUID {
	tb.Helper()
	status, answer := b.serve(http.MethodPost, "/api/v0/notebooks/"+b.nb.String()+"/pages", fmt.Sprintf(`{"parent_id":null,"title":%q}`, title))
	if status != http.StatusCreated {
		tb.Fatalf("create %s = %d %s", title, status, answer)
	}
	var created struct{ ID uuid.UUID }
	if err := json.Unmarshal([]byte(answer), &created); err != nil {
		tb.Fatal(err)
	}
	return created.ID
}
