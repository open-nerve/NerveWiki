//go:build !race

package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/access"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/ratelimit"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// What a restore costs, by the size of its item (M8/P2 design 3.11): the
// measurement MaxUnitNodes is taken from, run by hand,
//
//	go test -run '^$' -bench 'BenchmarkTrash' -benchtime 3x ./internal/bootstrap/
//
// and a test that holds a restore at the bound to its time. The page
// module is wired as serve wires it, its observers and participants, its
// budget and the production's settings that bound a request, but for the
// bound of a unit's nodes, which the benchmark raises; its routes are
// served in process, alice's by a token any.

// trashBench is a notebook of alice's on a database of its own whose
// tables keep no statistics until analyzed, and the page module on it.
type trashBench struct {
	pool   *pgxpool.Pool
	router http.Handler
	md     *markdown.Markdown
	alice  uuid.UUID
	nb     uuid.UUID
}

// benchAuth authenticates every token as the user's session.
type benchAuth struct{ user uuid.UUID }

func (a benchAuth) Authenticate(ctx context.Context, _ string) (context.Context, string, error) {
	actor := shared.Actor{UserID: a.user, SessionID: uuid.NewV7()}
	return shared.WithActor(ctx, actor), "session:" + actor.SessionID.String(), nil
}

// newTrashBench is the bench, its unit bounded at maxNodes, 0 the
// domain's.
func newTrashBench(tb testing.TB, maxNodes int) trashBench {
	tb.Helper()
	ctx := context.Background()
	url := pgtest.NewDatabase(tb)
	cfg := testConfig(tb, url, false)
	// The production's bounds of a request (server.*), and its pool.
	cfg.Server.ReadTimeout, cfg.Server.WriteTimeout, cfg.Server.RequestTimeout = 30*time.Second, 60*time.Second, 15*time.Second
	cfg.Database.MaxConns = 10
	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(pool.Close)
	b := trashBench{pool: pool, alice: uuid.NewV7(), nb: uuid.NewV7()}
	acme, now := uuid.NewV7(), time.Now()
	for _, x := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@example.com', 'x', 'Alice', $2, $2)`,
			[]any{b.alice, now}},
		{`INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, $3, $3)`,
			[]any{acme, b.alice, now}},
		{`INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2, 'admin', $2, $2, $3, $3)`, []any{acme, b.alice, now}},
		{`INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Eng', $3, $3, $4, $4)`,
			[]any{b.nb, acme, b.alice, now}},
		{`INSERT INTO notebook_members (id, notebook_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2, 'admin', $2, $2, $3, $3)`, []any{b.nb, b.alice, now}},
	} {
		if _, err := pool.Exec(ctx, x.sql, x.args...); err != nil {
			tb.Fatalf("%s: %v", x.sql, err)
		}
	}
	for _, table := range []string{"nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "asset_blobs",
		"indexed_pages", "page_links", "page_aliases", "page_tags", "page_properties"} {
		if _, err := pool.Exec(ctx, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)"); err != nil {
			tb.Fatal(err)
		}
	}
	logger := slog.New(slog.DiscardHandler)
	authorizer := access.New(access.Deps{Memberships: workspace.NewMemberships(pool), Notebooks: notebookFacts{notebook.NewFacts(pool)}})
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	md, budget, err := parsing(cfg, logger, pool, assetEmbeds{asset.NewEmbeds(pool, key, clock.System{})}.assets)
	if err != nil {
		tb.Fatal(err)
	}
	deps := pageDeps(cfg, pool, logger, authorizer, md, budget)
	deps.MaxUnitNodes = maxNodes
	pg := page.New(deps)
	router := httpserver.NewRouter(logger)
	api, err := httpserver.NewAPI(apiConfig(cfg, logger, ratelimit.New(time.Now), benchAuth{b.alice}, nil, nil, pg.BodyLimits()))
	if err != nil {
		tb.Fatal(err)
	}
	pg.Register(router, api)
	b.router, b.md = router, md
	return b
}

// serve sends alice's request and answers its status and body.
func (b trashBench) serve(method, path, body string) (int, string) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer any")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	b.router.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

// load is the contents of an item's pages: each page's size and links.
type load struct {
	name  string
	bytes int
	links int
}

// The loads (M8/P2 design 3.11): light, a page of 8 KiB and 20 links; full,
// the unit's bounds shared among 100 pages, 8 MiB and 20,000 links.
func benchLoads() []load {
	return []load{{"light", 8 << 10, 20}, {"full", domain.MaxUnitBytes / domain.MaxUnitPages, domain.MaxUnitLinks / domain.MaxUnitPages}}
}

// content is a page's content of the load: its links to the item's pages
// by name, in turn, then text to its size.
func (l load) content(i int) string {
	var s strings.Builder
	for k := range l.links {
		fmt.Fprintf(&s, "[[Page %d]]\n", (i+k)%(domain.MaxUnitPages-1))
	}
	line := "Lorem ipsum dolor sit amet, consectetur adipiscing elit.\n"
	for s.Len()+len(line) <= l.bytes {
		s.WriteString(line)
	}
	s.WriteString(strings.Repeat("x", l.bytes-s.Len()))
	return s.String()
}

// item creates an item of 100 pages, a root and 99 under it, of the load,
// through the module, and attachments under the pages, through SQL with
// their rows and items; answers the root and the pages' contents.
func (b trashBench) item(tb testing.TB, l load, attachments int) (string, []string) {
	tb.Helper()
	contents := []string{l.content(domain.MaxUnitPages - 1)}
	body, _ := json.Marshal(map[string]any{"parent_id": nil, "title": "Root", "content": contents[0]})
	status, answer := b.serve(http.MethodPost, "/api/v0/notebooks/"+b.nb.String()+"/pages", string(body))
	if status != http.StatusCreated {
		tb.Fatalf("create Root = %d %s", status, answer)
	}
	var root struct{ ID string }
	if err := json.Unmarshal([]byte(answer), &root); err != nil {
		tb.Fatal(err)
	}
	for i := range domain.MaxUnitPages - 1 {
		contents = append(contents, l.content(i))
		body, _ := json.Marshal(map[string]any{"parent_id": root.ID, "title": fmt.Sprintf("Page %d", i), "content": contents[i+1]})
		if status, answer := b.serve(http.MethodPost, "/api/v0/notebooks/"+b.nb.String()+"/pages", string(body)); status != http.StatusCreated {
			tb.Fatalf("create Page %d = %d %s", i, status, answer)
		}
	}
	ctx := context.Background()
	changeset := uuid.NewV7()
	for _, x := range []struct {
		sql  string
		args []any
	}{
		{`WITH p AS (SELECT id, row_number() OVER (ORDER BY id) - 1 AS k FROM nodes WHERE notebook_id = $1 AND kind = 'page')
		INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
		SELECT gen_random_uuid(), $1, p.id, 'asset', 'a' || g || '.png', 'a' || g || '.png', g, $2, $2, now(), now()
		FROM generate_series(0, $3::integer - 1) g JOIN p ON p.k = g % 100`, []any{b.nb, b.alice, attachments}},
		{`INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at)
		SELECT gen_random_uuid(), id, notebook_id, 'image/png', 3, sha256('abc'), created_by_id, created_at FROM nodes
		WHERE notebook_id = $1 AND kind = 'asset'`, []any{b.nb}},
		{`INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
		VALUES ($1, $2, 'edit', 'web', $3, now(), now())`, []any{changeset, b.nb, b.alice}},
		{`INSERT INTO changeset_items (id, changeset_id, node_id, after_parent_id, after_name, after_sort_order, created_at, updated_at)
		SELECT gen_random_uuid(), $1, id, parent_id, name, sort_order, created_at, created_at FROM nodes
		WHERE notebook_id = $2 AND kind = 'asset'`, []any{changeset, b.nb}},
	} {
		if _, err := b.pool.Exec(ctx, x.sql, x.args...); err != nil {
			tb.Fatalf("%s: %v", x.sql, err)
		}
	}
	return root.ID, contents
}

// timed runs a request of alice's that must answer want, and answers how
// long it took.
func (b trashBench) timed(tb testing.TB, method, path string, want int) time.Duration {
	tb.Helper()
	at := time.Now()
	status, answer := b.serve(method, path, "")
	took := time.Since(at)
	if status != want {
		tb.Fatalf("%s %s = %d %s, want %d", method, path, status, answer, want)
	}
	return took
}

// parsed is how long the contents take to parse, one after the other, as
// a restore parses them.
func (b trashBench) parsed(contents []string) time.Duration {
	at := time.Now()
	for _, c := range contents {
		b.md.Parse([]byte(c)).Facts()
	}
	return time.Since(at)
}

// vacuumed clears the dead versions the requests before left of the
// item's rows, as a server's autovacuum does, and adds no statistics: each
// restore and deletion leaves one of every row it writes, and the
// iterations' would pile up, read through again by every request after
// (by the fourth deletion analyzed, ten versions of each item read for
// one, a deletion of 10,000 nodes 6 seconds instead of 1).
func (b trashBench) vacuumed(tb testing.TB) {
	tb.Helper()
	if _, err := b.pool.Exec(context.Background(), "VACUUM"); err != nil {
		tb.Fatal(err)
	}
}

// BenchmarkTrashRestore restores an item of 100 pages of each load and of
// 0 to 10,000 attachments, without statistics, then analyzed; each
// iteration a restore, then the item's deletion, each after a vacuum,
// untimed. It reports the request, the parse of the contents in it, the
// unit, the rest, and the deletion, each a mean in milliseconds.
func BenchmarkTrashRestore(b *testing.B) {
	for _, l := range benchLoads() {
		for _, attachments := range []int{0, 1000, 2000, 5000, 10000} {
			bench := newTrashBench(b, 1<<20)
			root, contents := bench.item(b, l, attachments)
			deletion := bench.timed(b, http.MethodDelete, "/api/v0/nodes/"+root, http.StatusNoContent)
			parse := bench.parsed(contents)
			for _, stats := range []string{"without statistics", "analyzed"} {
				if stats == "analyzed" {
					if _, err := bench.pool.Exec(context.Background(), "ANALYZE"); err != nil {
						b.Fatal(err)
					}
				}
				b.Run(fmt.Sprintf("%s/%d attachments/%s", l.name, attachments, stats), func(b *testing.B) {
					var request, deleted time.Duration
					for range b.N {
						b.StopTimer()
						bench.vacuumed(b)
						b.StartTimer()
						request += bench.timed(b, http.MethodPost, "/api/v0/nodes/"+root+"/restore", http.StatusOK)
						b.StopTimer()
						bench.vacuumed(b)
						deleted += bench.timed(b, http.MethodDelete, "/api/v0/nodes/"+root, http.StatusNoContent)
						b.StartTimer()
					}
					n := float64(b.N)
					ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
					b.ReportMetric(ms(request)/n, "request-ms")
					b.ReportMetric(ms(parse), "parse-ms")
					b.ReportMetric(ms(request)/n-ms(parse), "unit-ms")
					b.ReportMetric(ms(deleted)/n, "delete-ms")
					b.ReportMetric(ms(deletion), "first-delete-ms")
				})
			}
		}
	}
}

// BenchmarkTrashDelete deletes an item of as many nodes as an import
// writes at most, transfer.import_max_entries' default: 100 pages of the
// light load, the rest attachments; without statistics, then analyzed
// while the item is not deleted, as a server's statistics are. Each
// iteration deletes an item of its own, made untimed: a restore of so many
// nodes is past the unit's bound. A deletion past the request's 15
// seconds goes into M12's handoff (M8/P2 design 3.11).
func BenchmarkTrashDelete(b *testing.B) {
	entries := testConfig(b, "", false).Transfer.ImportMaxEntries
	for _, stats := range []string{"without statistics", "analyzed"} {
		b.Run(fmt.Sprintf("%d nodes/%s", entries, stats), func(b *testing.B) {
			var deleted time.Duration
			for range b.N {
				b.StopTimer()
				bench := newTrashBench(b, 0)
				root, _ := bench.item(b, benchLoads()[0], entries-domain.MaxUnitPages)
				if stats == "analyzed" {
					if _, err := bench.pool.Exec(context.Background(), "ANALYZE"); err != nil {
						b.Fatal(err)
					}
				}
				b.StartTimer()
				deleted += bench.timed(b, http.MethodDelete, "/api/v0/nodes/"+root, http.StatusNoContent)
			}
			b.ReportMetric(float64(deleted)/float64(time.Millisecond)/float64(b.N), "delete-ms")
		})
	}
}

// A restore at the unit's bound, of the full load, takes its time: in 10
// seconds, a CI's slower than the machine the bound was measured on (M8/P2
// design 3.11).
func TestARestoreAtTheUnitsBoundTakesItsTime(t *testing.T) {
	bench := newTrashBench(t, 0)
	root, _ := bench.item(t, benchLoads()[1], domain.MaxUnitNodes-domain.MaxUnitPages)
	bench.timed(t, http.MethodDelete, "/api/v0/nodes/"+root, http.StatusNoContent)
	if took := bench.timed(t, http.MethodPost, "/api/v0/nodes/"+root+"/restore", http.StatusOK); took > 10*time.Second {
		t.Errorf("a restore of %d nodes took %s, want 10 s at most", domain.MaxUnitNodes, took)
	}
	if n := count(t, bench.pool, "SELECT count(*) FROM nodes WHERE deleted_at IS NULL"); n != domain.MaxUnitNodes {
		t.Errorf("%d nodes back, want %d", n, domain.MaxUnitNodes)
	}
	checkPages(t, bench.pool)
}
