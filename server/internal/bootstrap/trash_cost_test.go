//go:build !race

package bootstrap

import (
	"cmp"
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
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
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
// tables keep no statistics until analyzed, and the page module on it, its
// writes of the background jobs among them; its configuration, its API and
// its module, which a bench may mount more on (batch_cost_test.go); and the
// parse budget its modules share.
type trashBench struct {
	pool   *pgxpool.Pool
	router http.Handler
	md     *markdown.Markdown
	budget *markdown.Budget
	jobs   page.TreeWrites
	alice  uuid.UUID
	nb     uuid.UUID
	cfg    config.Config
	mux    *httpserver.Router
	api    *httpserver.API
	pg     *page.Module
}

// benchAuth authenticates every token as the user's session, but "pat",
// one of the user's personal access tokens, which MCP takes
// (batch_cost_test.go).
type benchAuth struct{ user uuid.UUID }

func (a benchAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	if token == "pat" {
		actor := shared.Actor{UserID: a.user, APITokenID: uuid.NewV7()}
		return shared.WithActor(ctx, actor), "pat:" + actor.APITokenID.String(), nil
	}
	actor := shared.Actor{UserID: a.user, SessionID: uuid.NewV7()}
	return shared.WithActor(ctx, actor), "session:" + actor.SessionID.String(), nil
}

// newTrashBench is the bench, its unit bounded at maxNodes and maxHistory;
// 0 keeps the composition root's bound, the domain's unless pageDeps sets
// one, so a test at the bounds answers as the server would.
func newTrashBench(tb testing.TB, maxNodes, maxHistory int) trashBench {
	tb.Helper()
	return newTrashBenchOfSteps(tb, maxNodes, maxHistory, 0)
}

// newTrashBenchOfSteps is the bench, a revert's round also bounded at
// maxSteps, 0 the composition root's.
func newTrashBenchOfSteps(tb testing.TB, maxNodes, maxHistory, maxSteps int) trashBench {
	tb.Helper()
	return newTrashBenchWith(tb, func(d *page.Deps) {
		d.MaxUnitNodes, d.MaxUnitHistory = cmp.Or(maxNodes, d.MaxUnitNodes), cmp.Or(maxHistory, d.MaxUnitHistory)
		d.MaxRevertSteps = cmp.Or(maxSteps, d.MaxRevertSteps)
	})
}

// newTrashBenchWith is the bench, the page module's dependencies as set
// leaves those the composition root gives it.
func newTrashBenchWith(tb testing.TB, set func(d *page.Deps)) trashBench {
	tb.Helper()
	return newTrashBenchOf(tb, func(*config.Config) {}, set)
}

// newTrashBenchOf is the bench, its configuration as configure leaves the
// test's, the production's bounds of a request and its pool, and the page
// module's dependencies as set leaves those the composition root gives it.
func newTrashBenchOf(tb testing.TB, configure func(c *config.Config), set func(d *page.Deps)) trashBench {
	tb.Helper()
	ctx := context.Background()
	url := pgtest.NewDatabase(tb)
	cfg := testConfig(tb, url, false)
	// The production's bounds of a request (server.*), and its pool.
	cfg.Server.ReadTimeout, cfg.Server.WriteTimeout, cfg.Server.RequestTimeout = 30*time.Second, 60*time.Second, 15*time.Second
	cfg.Database.MaxConns = 10
	configure(&cfg)
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
	set(&deps)
	pg := page.New(deps)
	router := httpserver.NewRouter(logger)
	api, err := httpserver.NewAPI(apiConfig(cfg, logger, ratelimit.New(time.Now), benchAuth{b.alice}, nil, nil, pg.BodyLimits()))
	if err != nil {
		tb.Fatal(err)
	}
	pg.Register(router, api)
	b.router, b.md, b.budget, b.jobs = router, md, budget, pg.TreeWrites()
	b.cfg, b.mux, b.api, b.pg = cfg, router, api, pg
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
// long it took; a wrong answer says it too, a 500 past the request's
// deadline among them.
func (b trashBench) timed(tb testing.TB, method, path string, want int) time.Duration {
	tb.Helper()
	at := time.Now()
	status, answer := b.serve(method, path, "")
	took := time.Since(at)
	if status != want {
		tb.Fatalf("%s %s = %d %s after %s, want %d", method, path, status, answer, took, want)
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
			bench := newTrashBench(b, 1<<20, 1<<30)
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
				bench := newTrashBench(b, 0, 0)
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

// versions writes the item's pages versions from after to up to, each
// step a changeset of the notebook's and an item of each page in it, the
// pages' contents on the last, which holds them, as a page's latest
// version does (a revert compares the content it restores with it, M8/P3
// design 3.5): the history of pages edited that often.
// A version is of 1 KiB that does not compress, about half the widest a
// row keeps in its table, about 2 KB, past which TOAST moves the content
// out of it: at 1.8 KiB the restore at the bounds took 8% longer (M8/P2A
// fix check 1, N-2).
func (b trashBench) versions(tb testing.TB, after, upTo int) {
	tb.Helper()
	ctx := context.Background()
	pages := `SELECT id, parent_id, name, sort_order FROM nodes WHERE notebook_id = $1::uuid AND kind = 'page' AND deleted_at IS NULL`
	for _, x := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
		SELECT md5($1::uuid::text || k)::uuid, $1::uuid, 'edit', 'web', $2, now(), now() FROM generate_series($3::integer + 1, $4::integer) k`,
			[]any{b.nb, b.alice, after, upTo}},
		{`WITH pool AS (SELECT i, (SELECT string_agg(md5(i::text || '.' || g), '') FROM generate_series(1, 32) g) AS content
			FROM generate_series(0, 999) i)
		INSERT INTO page_revisions (id, changeset_id, node_id, base_revision, revision, content, content_hash, byte_size,
			created_at, updated_at)
		SELECT gen_random_uuid(), md5($1::uuid::text || k)::uuid, p.id, k - 1, k, pool.content, sha256(convert_to(pool.content, 'UTF8')),
			octet_length(pool.content), now(), now()
		FROM (` + pages + `) p, generate_series($2::integer + 1, $3::integer) k JOIN pool ON pool.i = k % 1000`,
			[]any{b.nb, after, upTo}},
		{`INSERT INTO changeset_items (id, changeset_id, node_id, after_parent_id, after_name, after_sort_order, created_at, updated_at)
		SELECT gen_random_uuid(), md5($1::uuid::text || k)::uuid, p.id, p.parent_id, p.name, p.sort_order, now(), now()
		FROM (` + pages + `) p, generate_series($2::integer + 1, $3::integer) k`, []any{b.nb, after, upTo}},
		{`UPDATE page_contents SET revision = $2 WHERE node_id IN (SELECT id FROM nodes WHERE notebook_id = $1 AND kind = 'page')`,
			[]any{b.nb, upTo}},
		{`UPDATE page_revisions r SET content = c.content, content_hash = c.content_hash, byte_size = c.byte_size
		FROM page_contents c JOIN nodes n ON n.id = c.node_id
		WHERE n.notebook_id = $1 AND n.kind = 'page' AND r.node_id = c.node_id AND r.revision = $2`, []any{b.nb, upTo}},
	} {
		if _, err := b.pool.Exec(ctx, x.sql, x.args...); err != nil {
			tb.Fatalf("%s: %v", x.sql, err)
		}
	}
}

// history is the rows of the trash item root's history: the versions and
// items of its group, deleted at its time; the group read first, then its
// rows by their ids, as TrashHistory reads them (one statement joining the
// two took minutes, analyzed).
func (b trashBench) history(tb testing.TB, root string) int {
	tb.Helper()
	ctx := context.Background()
	var ids []uuid.UUID
	var at time.Time
	if err := b.pool.QueryRow(ctx, `WITH RECURSIVE g AS (SELECT id, deleted_at FROM nodes WHERE id = $1 AND trash_root
			UNION ALL SELECT n.id, n.deleted_at FROM nodes n JOIN g ON n.parent_id = g.id AND n.deleted_at = g.deleted_at)
		SELECT array_agg(id), max(deleted_at) FROM g`, root).Scan(&ids, &at); err != nil {
		tb.Fatal(err)
	}
	var n int
	if err := b.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM page_revisions WHERE node_id = ANY($1) AND deleted_at = $2)
		+ (SELECT count(*) FROM changeset_items WHERE node_id = ANY($1) AND deleted_at = $2)`, ids, at).Scan(&n); err != nil {
		tb.Fatal(err)
	}
	return n
}

// BenchmarkTrashRestoreHistory restores an item of the full load at the
// unit's bound of nodes, 100 pages and 9,900 attachments, whose pages have
// 250 to 2,000 versions, each with its item (M8/P2A review, A-M4): the
// measurement MaxUnitHistory is taken from, as MaxUnitNodes is. Without
// statistics, then on a database of its own analyzed after each tier's
// versions, while the item is not deleted, as a server's statistics are.
// Each iteration a deletion, then a restore, each after a vacuum,
// untimed; the history grows by the two's items. It reports the request,
// the parse, the unit, the deletion, each a mean in milliseconds, and the
// rows of the history a restore wrote, a mean.
func BenchmarkTrashRestoreHistory(b *testing.B) {
	for _, stats := range []string{"without statistics", "analyzed"} {
		bench := newTrashBench(b, 1<<20, 1<<30)
		root, contents := bench.item(b, benchLoads()[1], domain.MaxUnitNodes-domain.MaxUnitPages)
		parse := bench.parsed(contents)
		after := 1
		for _, upTo := range []int{250, 500, 1000, 1500, 2000} {
			bench.versions(b, after, upTo)
			after = upTo
			if stats == "analyzed" {
				if _, err := bench.pool.Exec(context.Background(), "ANALYZE"); err != nil {
					b.Fatal(err)
				}
			}
			b.Run(fmt.Sprintf("%d versions a page/%s", upTo, stats), func(b *testing.B) {
				var request, deleted time.Duration
				rows := 0
				for range b.N {
					b.StopTimer()
					bench.vacuumed(b)
					b.StartTimer()
					deleted += bench.timed(b, http.MethodDelete, "/api/v0/nodes/"+root, http.StatusNoContent)
					b.StopTimer()
					rows += bench.history(b, root)
					bench.vacuumed(b)
					b.StartTimer()
					request += bench.timed(b, http.MethodPost, "/api/v0/nodes/"+root+"/restore", http.StatusOK)
				}
				n := float64(b.N)
				ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
				b.ReportMetric(ms(request)/n, "request-ms")
				b.ReportMetric(ms(parse), "parse-ms")
				b.ReportMetric(ms(request)/n-ms(parse), "unit-ms")
				b.ReportMetric(ms(deleted)/n, "delete-ms")
				b.ReportMetric(float64(rows)/n, "rows")
			})
		}
	}
}

// toppedUp gives the trash item root's attachments n more items, of new
// changesets, deleted at its time: its history to a row.
func (b trashBench) toppedUp(tb testing.TB, root string, n int) {
	tb.Helper()
	for n > 0 {
		changeset := uuid.NewV7()
		tag, err := b.pool.Exec(context.Background(), `WITH cs AS (
				INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
				VALUES ($2, $3, 'edit', 'web', $4, now(), now()) RETURNING id
			), a AS (SELECT n.id, n.parent_id, n.name, n.sort_order, n.deleted_at FROM nodes n JOIN nodes r ON r.id = $1
				WHERE n.notebook_id = r.notebook_id AND n.kind = 'asset' AND n.deleted_at = r.deleted_at ORDER BY n.id LIMIT $5)
			INSERT INTO changeset_items (id, changeset_id, node_id, after_parent_id, after_name, after_sort_order, created_at,
				updated_at, deleted_at)
			SELECT gen_random_uuid(), cs.id, a.id, a.parent_id, a.name, a.sort_order, now(), now(), a.deleted_at FROM a, cs`,
			root, changeset, b.nb, b.alice, n)
		if err != nil || tag.RowsAffected() == 0 {
			tb.Fatalf("top the history up by %d: %d, %v", n, tag.RowsAffected(), err)
		}
		n -= int(tag.RowsAffected())
	}
}

// A restore at the unit's bounds, of the full load, its nodes' and its
// history's, takes its time: in 12 seconds, a CI's slower than the
// machine the bounds were measured on (M8/P2 design 3.11; M8/P2A review,
// A-M4), three times its 3.8–4.0 s there, under the bench's request
// deadline, 15 s; the time logged for CI's runs (make test-go runs it
// with -v).
// Deleted again, its history 420,000 rows with the items of the restore
// and of the deletion, 20,000 past the bound, it is
// page.restore_too_large and writes nothing. The bench keeps the
// composition root's bounds: one far from the domain's fails here, and
// TestPageDepsLeaveTheUnitsBoundsTheDomains pins them exactly.
func TestARestoreAtTheUnitsBoundTakesItsTime(t *testing.T) {
	bench := newTrashBench(t, 0, 0)
	root, _ := bench.item(t, benchLoads()[1], domain.MaxUnitNodes-domain.MaxUnitPages)
	// The item's rows, its deletion's items among them, leave the versions
	// and their items the rest of the bound, a step of them each page's.
	bench.versions(t, 1, (domain.MaxUnitHistory-2*domain.MaxUnitNodes-2*domain.MaxUnitPages)/(2*domain.MaxUnitPages))
	bench.timed(t, http.MethodDelete, "/api/v0/nodes/"+root, http.StatusNoContent)
	rows := bench.history(t, root)
	if rows > domain.MaxUnitHistory {
		t.Fatalf("the item's history is %d rows, past the bound %d before it is topped up", rows, domain.MaxUnitHistory)
	}
	bench.toppedUp(t, root, domain.MaxUnitHistory-rows)
	if rows := bench.history(t, root); rows != domain.MaxUnitHistory {
		t.Fatalf("the item's history is %d rows, want the bound %d", rows, domain.MaxUnitHistory)
	}
	took := bench.timed(t, http.MethodPost, "/api/v0/nodes/"+root+"/restore", http.StatusOK)
	t.Logf("a restore of %d nodes, %d rows of history, took %s", domain.MaxUnitNodes, domain.MaxUnitHistory, took)
	if took > 12*time.Second {
		t.Errorf("a restore of %d nodes, %d rows of history, took %s, want 12 s at most", domain.MaxUnitNodes, domain.MaxUnitHistory, took)
	}
	if n := count(t, bench.pool, "SELECT count(*) FROM nodes WHERE deleted_at IS NULL"); n != domain.MaxUnitNodes {
		t.Errorf("%d nodes back, want %d", n, domain.MaxUnitNodes)
	}
	checkPages(t, bench.pool)

	bench.timed(t, http.MethodDelete, "/api/v0/nodes/"+root, http.StatusNoContent)
	if status, answer := bench.serve(http.MethodPost, "/api/v0/nodes/"+root+"/restore", ""); status != http.StatusUnprocessableEntity ||
		!strings.Contains(answer, `"code":"page.restore_too_large"`) {
		t.Errorf("a restore past the history's bound = %d %s, want 422 page.restore_too_large", status, answer)
	}
	if n := count(t, bench.pool, "SELECT count(*) FROM changesets WHERE kind = 'trash_restore'"); n != 1 {
		t.Errorf("%d restores' changesets, want the first's alone", n)
	}
	checkPages(t, bench.pool)
}
