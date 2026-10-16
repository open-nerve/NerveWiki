//go:build !race

package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// searchOfSomeSize is eng of 5,000 synthetic pages of about 4 KB, their
// rows built by the job, and 500 of them written again since, newest,
// queued exact: a backlog the job has yet to build; and ops of 5,000 pages
// an import queued in bulk (M8/P5 design 3.9's guard). No table has
// statistics: the job analyzes page_search from its 10,000th row. Its
// search is the use case over a store on one connection, which caches
// the statements a server's do, and plans the search's own with their
// arguments.
type searchOfSomeSize struct {
	f      fixture
	pool   *pgxpool.Pool // the search's one connection
	search *app.Search
	ctx    context.Context
	built  time.Duration // the least of the job's three runs that built eng's rows
}

func newSearchOfSomeSize(t *testing.T) searchOfSomeSize {
	t.Helper()
	ctx := context.Background()
	f := newFixture(t)
	for _, table := range []string{"nodes", "page_contents", "page_search", "page_search_queue"} {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)")
	}
	corpus := newSearchCorpus(20261013)
	eng := corpus.pages(t, f, f.eng, 5000, 4000)
	// The job's build of eng's rows, three times from none, the queue and
	// the rows emptied in between: a run is as fast as it can be in the
	// least of them (M8 closeout A-1). TRUNCATE leaves no dead row and no
	// statistics, so the third leaves the tables as the first did.
	var builds []time.Duration
	for i := range 3 {
		if i > 0 {
			f.exec(t, "TRUNCATE page_search, page_search_queue")
		}
		f.queueSearch(t, f.eng, false, eng...)
		started := time.Now()
		f.drain(t)
		builds = append(builds, time.Since(started))
	}
	t.Logf("the job built 5,000 pages in %v", builds)
	backlog := eng[:500]
	contents, hashes := make([]string, len(backlog)), make([][]byte, len(backlog))
	for i := range backlog {
		contents[i] = corpus.content(4000)
		sum := sha256.Sum256([]byte(contents[i]))
		hashes[i] = sum[:]
	}
	f.exec(t, `UPDATE page_contents SET content = x.content, revision = 2, content_hash = x.hash, byte_size = octet_length(x.content),
			updated_at = $4::timestamptz + x.k * interval '1 second'
		FROM unnest($1::uuid[], $2::text[], $3::bytea[]) WITH ORDINALITY AS x(id, content, hash, k) WHERE node_id = x.id`,
		backlog, contents, hashes, now().Add(24*time.Hour))
	f.queueSearch(t, f.eng, false, backlog...)
	f.queueSearch(t, f.ops, true, corpus.pages(t, f, f.ops, 5000, 1000)...)
	if stats := f.count(t, `SELECT count(*) FROM pg_stats WHERE tablename IN ('nodes', 'page_contents', 'page_search', 'page_search_queue')`); stats != 0 {
		t.Fatalf("the tables have statistics of %d columns", stats)
	}
	if rows := f.count(t, `SELECT count(*) FROM page_search`); rows != 5000 {
		t.Fatalf("page_search holds %d rows, want eng's 5,000", rows)
	}
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := postgresadapter.New(pool)
	search := app.NewSearch(app.SearchDeps{Snapshots: postgres.NewTxManager(pool, 5*time.Second), Workspaces: searchWorkspace{uuid.NewV7()},
		Visible: searchVisible{f.eng, f.ops}, Auth: searchAllowed{}, Reads: s, Trash: s, Attachments: noAttachments{}})
	return searchOfSomeSize{f: f, pool: pool, search: search, built: slices.Min(builds),
		ctx: shared.WithActor(ctx, shared.Actor{UserID: f.alice, SessionID: uuid.NewV7()})}
}

// The search reads a page in time whatever the statistics (M8/P5 design
// 3.9's guard): each class of query's first page, with its count, and its
// third; the walk that checks its bound of pages and finds none; eight
// terms; and a search while ops is being built (searchReads). Each eight
// times on one connection: without statistics, analyzed, and planned for
// any arguments, as a server set so would; its median within searchLimit
// of this machine's, a CI's runner slower, well under the request's 15
// seconds. The 300 ms target is the benchmark's on 300,000 pages: this
// guard catches a plan gone wrong, seconds for milliseconds. So do the
// job's build of 5,000 pages and a unit's queueing of 10,000 nodes.
func TestTheSearchReadsInTimeAtAnyStatistics(t *testing.T) {
	g := newSearchOfSomeSize(t)
	reads := searchReads(&g.f.eng)
	cursors := map[string]*string{} // the third pages' cursors, by query and notebook
	key := func(r searchRead) string {
		if r.notebook == nil {
			return r.q
		}
		return r.q + " in " + r.notebook.String()
	}
	for _, r := range reads {
		if r.page != 3 {
			continue
		}
		var cursor *string
		for range 2 {
			page, err := g.search.Execute(g.ctx, "acme", app.SearchRequest{Q: &r.q, NotebookID: r.notebook, Cursor: cursor})
			if err != nil || page.NextCursor == "" {
				t.Fatalf("%q: a page before the third = %d hits, %v; want a next one", r.q, len(page.Hits), err)
			}
			cursor = &page.NextCursor
		}
		cursors[key(r)] = cursor
	}
	for k, stage := range []string{"without statistics", "analyzed", "planned for any arguments"} {
		switch stage {
		case "analyzed":
			g.f.exec(t, "ANALYZE")
		case "planned for any arguments":
			if _, err := g.pool.Exec(context.Background(), "SET plan_cache_mode = force_generic_plan"); err != nil {
				t.Fatal(err)
			}
		}
		for _, r := range reads {
			req := app.SearchRequest{Q: &r.q, NotebookID: r.notebook}
			if r.page == 3 {
				req.Cursor = cursors[key(r)]
			}
			took := make([]time.Duration, 8)
			for i := range took {
				started := time.Now()
				page, err := g.search.Execute(g.ctx, "acme", req)
				took[i] = time.Since(started)
				if err != nil || page.Building != r.building {
					t.Fatalf("%q page %d %s = building %v, %v; want building %v", r.q, r.page, stage, page.Building, err, r.building)
				}
			}
			slices.Sort(took)
			median := (took[3] + took[4]) / 2
			t.Logf("%q page %d %s: median %s, longest %s", r.q, r.page, stage, median.Round(time.Microsecond),
				took[7].Round(time.Microsecond))
			if limit := searchLimit(r.medians[k]); median > limit {
				t.Errorf("%q page %d %s took %s, its median; want within %s", r.q, r.page, stage, median, limit)
			}
		}
	}
	t.Logf("the job built 5,000 pages in %s, the least of three runs", g.built.Round(time.Millisecond))
	// 8.57 s here, the least of three runs of the guard (2.0 s before the
	// indexes kept no pending list: every key of a row goes into them as it
	// is written; M8 Codex review R1; measured again in its fix round 2):
	// three times it, 25.72 s, to the second above.
	if limit := 26 * time.Second; g.built > limit {
		t.Errorf("the job built 5,000 pages in %s, the least of three runs; want within %s", g.built, limit)
	}
	// A unit's queueing of 10,000 nodes, an import's batch or a deletion's
	// at its bound, in its transaction, rolled back: the least of eight
	// runs, as the build's (M8 closeout A-1).
	ids := make([]uuid.UUID, 10_000)
	for i := range ids {
		ids[i] = uuid.NewV7()
	}
	tx := postgres.NewTxManager(g.f.pool, 5*time.Second)
	took := make([]time.Duration, 8)
	for i := range took {
		_ = tx.WithinTx(context.Background(), func(ctx context.Context) error {
			started := time.Now()
			if err := g.f.s.QueueSearch(ctx, g.f.eng, ids, false); err != nil {
				t.Fatal(err)
			}
			took[i] = time.Since(started)
			return errCaught
		})
	}
	slices.Sort(took)
	t.Logf("QueueSearch of 10,000 nodes: least %s, median %s, longest %s", took[0].Round(time.Microsecond),
		((took[3] + took[4]) / 2).Round(time.Microsecond), took[7].Round(time.Microsecond))
	// 19 ms here, the median.
	if limit := 57 * time.Millisecond; took[0] > limit {
		t.Errorf("QueueSearch of 10,000 nodes took %s, the least of eight runs; want within %s", took[0], limit)
	}
}
