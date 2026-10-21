//go:build !race

package postgresadapter_test

import (
	"context"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The bounds of index over 100,000 pages: three times the slowest of
// their medians here, each test run alone three times (M9/P4 design 6.1,
// S2's report).
const (
	// indexPagesBound is IndexPages' median at any statistics: 192–283 ms.
	indexPagesBound = 850 * time.Millisecond
	// inboundBound is InboundCounts' median of a page of 100 items, each
	// linked from 1,001 pages: 120–132 ms.
	inboundBound = 400 * time.Millisecond
	// indexCallBound is ListIndex's call: IndexPages, the order's keys of
	// every type, IndexItems and InboundCounts of a page of 100: 1.06–1.21 s.
	indexCallBound = 3650 * time.Millisecond
)

// worstType is a type of the costliest shape for its key (M9/P4 design
// 3.9.3, review B-M3): a letter and 127 marks of four bytes, of two
// classes out of their order, which NFC sorts, 509 bytes.
func worstType() string {
	marks := strings.Repeat("\U0001E000\U0001D167", 64)
	return "A" + marks[:len(marks)-4]
}

// guardAccess is a notebook the caller reads, faked: the guard times the
// index's reads, not the decision.
type guardAccess struct{ workspace uuid.UUID }

func (g guardAccess) WorkspaceOf(context.Context, uuid.UUID) (uuid.UUID, bool, error) {
	return g.workspace, true, nil
}
func (guardAccess) Authorize(context.Context, shared.Actor, shared.Action, shared.Target) (shared.Grant, error) {
	return shared.Grant{NotebookRole: shared.NotebookReader}, nil
}

// guardPaths writes each page's path from its id, at once: the page
// module's are timed by its own guards. The rest of NodePaths is not
// called.
type guardPaths struct{ app.NodePaths }

func (guardPaths) Pages(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]app.PagePath, error) {
	out := make(map[uuid.UUID]app.PagePath, len(ids))
	for _, id := range ids {
		out[id] = app.PagePath{Path: id.String() + ".md"}
	}
	return out, nil
}

// index over 100,000 pages takes its time at any statistics (M9/P4 design
// 6.1; review B-M3, B-I2): each page of five properties, its type of the
// costliest shape for its key, and 2,000 pages of 1,000 properties without
// a type; the first 100 pages by id each linked from 1,001 pages.
// IndexPages, without statistics, analyzed and planned for any arguments,
// reads every page and the start of its type, one probe a page by 00047's
// index: without it the pages without a type read every property, some
// two seconds more. InboundCounts of the first 100 stops at 1,000 each.
// ListIndex's call, which orders them by their keys, takes its time too.
// Each is a median of three, against three times its time here; the live
// memory of the pages read and their keys, and the keys' time, are logged.
func TestIndexOfAHundredThousandPagesTakesItsTime(t *testing.T) {
	if testing.Short() {
		t.Skip("a bound in time, measured alone by make test-go-costs")
	}
	ctx := context.Background()
	url := pgtest.NewDatabase(t)
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: url, MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"indexed_pages", "page_properties", "page_links"} {
		if _, err := pool.Exec(ctx, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)"); err != nil {
			t.Fatal(err)
		}
	}
	notebook := uuid.NewV7()
	start := time.Now()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO indexed_pages (node_id, notebook_id, revision, extractor, frontmatter_valid)
			SELECT uuidv7(), $1, 1, 1, true FROM generate_series(1, 100000)`, []any{notebook}},
		{`INSERT INTO page_properties (source_id, position, notebook_id, key, value)
			SELECT p.node_id, k, $1, CASE k WHEN 1 THEN 'type' WHEN 3 THEN 'summary' ELSE 'p' || k END,
				CASE k WHEN 1 THEN to_jsonb($2::text) ELSE to_jsonb('v' || k) END
			FROM indexed_pages p CROSS JOIN generate_series(0, 4) k WHERE p.revision = 1`, []any{notebook, worstType()}},
		{`INSERT INTO indexed_pages (node_id, notebook_id, revision, extractor, frontmatter_valid)
			SELECT uuidv7(), $1, 2, 1, true FROM generate_series(1, 2000)`, []any{notebook}},
		{`INSERT INTO page_properties (source_id, position, notebook_id, key, value)
			SELECT p.node_id, k, $1, 'property' || k, to_jsonb('a value of ' || k) FROM indexed_pages p
				CROSS JOIN generate_series(0, 999) k
			WHERE p.revision = 2`, []any{notebook}},
		{`INSERT INTO page_links (source_id, range_start, range_end, notebook_id, kind, target, resolved_id, ambiguous)
			SELECT uuidv7(), 0, 1, $1, 'wikilink', 'x', t.node_id, false
			FROM (SELECT node_id FROM indexed_pages ORDER BY node_id LIMIT 100) t CROSS JOIN generate_series(1, 1001)`,
			[]any{notebook}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.sql, err)
		}
	}
	t.Logf("the seed: %v", time.Since(start))
	rows, err := pool.Query(ctx, `SELECT node_id FROM indexed_pages ORDER BY node_id LIMIT 100`)
	if err != nil {
		t.Fatal(err)
	}
	first, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		t.Fatal(err)
	}
	generic := func() *pgxpool.Pool {
		cfg, err := pgxpool.ParseConfig(url)
		if err != nil {
			t.Fatal(err)
		}
		cfg.ConnConfig.RuntimeParams["plan_cache_mode"] = "force_generic_plan"
		p, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(p.Close)
		return p
	}
	median := func(name string, bound time.Duration, read func() error) {
		t.Helper()
		var times []time.Duration
		for range 3 {
			start := time.Now()
			err := read()
			times = append(times, time.Since(start))
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
		slices.Sort(times)
		t.Logf("%s: %v", name, times)
		if times[1] > bound {
			t.Errorf("%s: a median of %v, over %v", name, times[1], bound)
		}
	}
	for _, stage := range []string{"without statistics", "analyzed", "generic"} {
		p := pool
		switch stage {
		case "analyzed":
			if _, err := pool.Exec(ctx, "ANALYZE indexed_pages, page_properties, page_links"); err != nil {
				t.Fatal(err)
			}
		case "generic":
			p = generic()
		}
		store := postgresadapter.New(p)
		median("IndexPages, "+stage, indexPagesBound, func() error {
			pages, err := store.IndexPages(ctx, notebook, nil)
			typed := 0
			for _, p := range pages {
				if p.Typed && p.TypeStart == worstType() {
					typed++
				}
			}
			if err == nil && (len(pages) != 102_000 || typed != 100_000) {
				t.Fatalf("%d pages read, %d of the type", len(pages), typed)
			}
			return err
		})
		median("InboundCounts of 100, "+stage, inboundBound, func() error {
			counts, err := store.InboundCounts(ctx, first, domain.MaxInbound)
			if err == nil && (len(counts) != 100 || counts[first[0]] != domain.MaxInbound) {
				t.Fatalf("inbound counts %d", len(counts))
			}
			return err
		})
	}

	store := postgresadapter.New(pool)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	pages, err := store.IndexPages(ctx, notebook, nil)
	if err != nil {
		t.Fatal(err)
	}
	keyer, keys := domain.NewIndexKeyer(), make([]string, 0, len(pages))
	start = time.Now()
	for _, p := range pages {
		if p.Typed {
			keys = append(keys, keyer.Key(p.TypeStart))
		}
	}
	took := time.Since(start)
	runtime.GC()
	runtime.ReadMemStats(&after)
	t.Logf("the keys of %d types: %v; the pages read and their keys hold %.1f MB live, %.1f MB allocated", len(keys), took,
		float64(after.HeapAlloc-before.HeapAlloc)/1e6, float64(after.TotalAlloc-before.TotalAlloc)/1e6)
	runtime.KeepAlive(pages)

	uc := app.ListIndex{Access: app.Access{Notebooks: guardAccess{uuid.NewV7()}, Auth: guardAccess{}}, Reads: store,
		Paths: guardPaths{}}
	caller := shared.WithActor(ctx, shared.Actor{UserID: uuid.NewV7(), SessionID: uuid.NewV7()})
	limit := 100
	median("ListIndex of a page of 100", indexCallBound, func() error {
		got, err := uc.Execute(caller, notebook, app.IndexQuery{Limit: &limit})
		if err == nil && (len(got.Items) != 100 || got.Items[0].PageID != first[0] || got.Items[0].Inbound != domain.MaxInbound) {
			t.Fatalf("the call's first item %+v of %d", got.Items[0], len(got.Items))
		}
		return err
	})
}
