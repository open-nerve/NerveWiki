//go:build !race

package bootstrap

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

func lintRangeDown(t *testing.T, m *postgres.Migrator, pool *pgxpool.Pool) {
	t.Helper()
	r, err := m.Down(t.Context())
	if err != nil || r == nil || r.Version != 52 || r.Source != "00052_linking_lint_ranges.sql" {
		t.Fatalf("Down = %+v, %v; want only range migration 52", r, err)
	}
	lintRangeDefinition(t, pool, false)
	lintSourcesDefinition(t, pool)
	lintMigrationState(t, pool, true, false)
}

func lintRangeApplied(t *testing.T, rows []postgres.Migration, err error) {
	t.Helper()
	if err != nil || !slices.Equal(rows, []postgres.Migration{{Version: 52, Source: "00052_linking_lint_ranges.sql"}}) {
		t.Fatalf("full production Up = %+v, %v; want only migration 52", rows, err)
	}
}

func lintRangeMigrationContent(dense bool) string {
	var content strings.Builder
	content.WriteString("---\n")
	for i := range 20 {
		key := fmt.Sprintf("unrelated%02d", i)
		if i == 0 {
			key = `""`
		}
		if dense {
			variant := []byte("sources")
			for j := range variant {
				if i&(1<<j) != 0 {
					variant[j] -= 'a' - 'A'
				}
			}
			key = string(variant)
		}
		value := "x"
		if i < 10 {
			value = "'[[Target]]'"
			if i%2 != 0 {
				value = "'[[Missing]]'"
			}
		}
		fmt.Fprintf(&content, "%s: %s\n", key, value)
	}
	content.WriteString("---\n\n[[Target]]\n[[Missing]]\n[Target](Target)\n[Missing](Missing)\n![[Target]]\n![[Missing]]\n![image](missing.png)\n![image](other.png)\n[[Target]]\n[Target](Target)\n")
	return content.String()
}

type lintRangeMigrationBook struct {
	Notebook, Target, Prototype string
}

type lintRangeMigrationFixture struct {
	Pool    *pgxpool.Pool
	Books   []lintRangeMigrationBook
	Content string
	PerBook int
	Dense   bool
}

// Two real notebooks have their own identically named targets. Their two
// prototypes are saved through the real writer and resolver. Bulk copies
// reuse every stored property/link fact, changing only source/notebook.
// Content/history are real page rows; their current index matches revision1.
func lintRangeMigrationSeed(t *testing.T, dense bool, perBook int) lintRangeMigrationFixture {
	t.Helper()
	database, app, first := lintCostSeedApp(t)
	f := lintRangeMigrationFixture{Content: lintRangeMigrationContent(dense), PerBook: perBook, Dense: dense}
	for _, notebook := range []string{first, app.team.openNotebook(t, "alice", "Eng B")} {
		book := lintRangeMigrationBook{Notebook: notebook}
		book.Target = app.team.createPage(t, "alice", notebook, "", "Target")
		book.Prototype = app.team.createPage(t, "alice", notebook, "", "Prototype")
		if a := app.team.sender(t, contentWrite("alice", book.Prototype, f.Content, 1, ""))(); a.err != nil || a.status != http.StatusOK {
			t.Fatalf("real prototype save = %+v", a)
		}
		f.Books = append(f.Books, book)
	}
	sourceIndexSettle(t, app.team.pool)
	app.stop()
	f.Pool = connect(t, database)
	lintMigrationState(t, f.Pool, true, true)
	m := lintCostMigrator(t, f.Pool, 52)
	lintRangeDown(t, m, f.Pool)
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	for _, book := range f.Books {
		for start := 0; start < perBook; start += 10000 {
			pages := make([]sqlPage, min(10000, perBook-start))
			for i := range pages {
				pages[i] = sqlPage{name: fmt.Sprintf("rangecost%06d", start+i), content: f.Content}
			}
			seedPagesSQL(t, f.Pool, book.Notebook, pages)
		}
		lintCostExec(t, f.Pool, `INSERT INTO page_properties(source_id,position,notebook_id,key,value)
 SELECT n.id,p.position,n.notebook_id,p.key,p.value FROM nodes n CROSS JOIN page_properties p
 WHERE n.notebook_id=$1 AND n.name LIKE 'rangecost%' AND p.source_id=$2`, book.Notebook, book.Prototype)
		lintCostExec(t, f.Pool, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,property_key,target,anchor,display,target_key,target_alt_key,resolved_id,ambiguous,aliases,resolved_asset)
 SELECT n.id,l.range_start,l.range_end,n.notebook_id,l.kind,l.property_key,l.target,l.anchor,l.display,l.target_key,l.target_alt_key,l.resolved_id,l.ambiguous,l.aliases,l.resolved_asset
 FROM nodes n CROSS JOIN page_links l WHERE n.notebook_id=$1 AND n.name LIKE 'rangecost%' AND l.source_id=$2`, book.Notebook, book.Prototype)
	}
	lintRangeMigrationFacts(t, f)
	return f
}

func lintRangeMigrationFacts(t *testing.T, f lintRangeMigrationFixture) {
	t.Helper()
	for _, book := range f.Books {
		linked := f.PerBook + 1 // bulk and the real saved prototype; Target has no links.
		check := func(label, sql string, want int, args ...any) {
			t.Helper()
			if n := count(t, f.Pool, sql, args...); n != want {
				t.Fatalf("%s notebook=%s got=%d want=%d", label, book.Notebook, n, want)
			}
		}
		check("live current pages", `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id JOIN indexed_pages i ON i.node_id=n.id
 WHERE n.notebook_id=$1 AND n.kind='page' AND n.deleted_at IS NULL AND c.deleted_at IS NULL
 AND i.notebook_id=n.notebook_id AND i.revision=c.revision AND i.extractor=2 AND i.frontmatter_valid`, f.PerBook+2, book.Notebook)
		check("exact bulk/prototype contents", `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id
 WHERE n.notebook_id=$1 AND n.id<>$2 AND c.content=$3 AND c.content_hash=sha256(convert_to(c.content,'UTF8'))
 AND c.byte_size=octet_length(c.content) AND n.source_hash IS NULL
 AND c.revision=CASE WHEN n.id=$4::uuid THEN 2 ELSE 1 END`, linked, book.Notebook, book.Target, f.Content, book.Prototype)
		check("history rows", `SELECT count(*) FROM page_revisions r JOIN nodes n ON n.id=r.node_id WHERE n.notebook_id=$1`, f.PerBook+3, book.Notebook)
		lintCostLog(t, "range-prototype-history", queryStrings(t, f.Pool, `SELECT jsonb_build_object(
 'name',n.name,'revision',c.revision,'history_revision',r.revision,'item_revision',item.revision,
 'same_content',r.content=c.content,'same_hash',r.content_hash=c.content_hash,'same_size',r.byte_size=c.byte_size,
 'history_live',r.deleted_at IS NULL)::text FROM nodes n JOIN page_contents c ON c.node_id=n.id
 JOIN page_revisions r ON r.node_id=n.id AND r.revision=c.revision
 LEFT JOIN changeset_items item ON item.changeset_id=r.changeset_id AND item.node_id=n.id
 WHERE n.id IN ($1,$2) ORDER BY n.name`, book.Target, book.Prototype))
		check("current history matches content", `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id
 JOIN page_revisions r ON r.node_id=n.id AND r.revision=c.revision
 JOIN changesets s ON s.id=r.changeset_id AND s.notebook_id=n.notebook_id
 WHERE n.notebook_id=$1 AND r.content=c.content AND r.content_hash=c.content_hash AND r.byte_size=c.byte_size AND r.deleted_at IS NULL
 AND s.deleted_at IS NULL`, f.PerBook+2, book.Notebook)
		// A pure content edit records history without a node-state item.
		// The bulk pages and Target are creations, whose items hold revision1.
		check("current creation items", `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id
 JOIN page_revisions r ON r.node_id=n.id AND r.revision=c.revision
 JOIN changeset_items item ON item.changeset_id=r.changeset_id AND item.node_id=n.id
 WHERE n.notebook_id=$1 AND c.revision=1 AND item.revision=1 AND item.after_name=n.name`, f.PerBook+1, book.Notebook)
		check("twenty properties per source", `SELECT count(*) FROM (SELECT source_id FROM page_properties WHERE notebook_id=$1 GROUP BY source_id HAVING count(*)=20) s`, linked, book.Notebook)
		check("twenty links per source", `SELECT count(*) FROM (SELECT source_id FROM page_links WHERE notebook_id=$1 GROUP BY source_id HAVING count(*)=20) s`, linked, book.Notebook)
		check("properties total", "SELECT count(*) FROM page_properties WHERE notebook_id=$1", linked*20, book.Notebook)
		check("links total", "SELECT count(*) FROM page_links WHERE notebook_id=$1", linked*20, book.Notebook)
		check("body NULL origin", "SELECT count(*) FROM page_links WHERE notebook_id=$1 AND property_key IS NULL", linked*10, book.Notebook)
		check("frontmatter non-NULL origin", "SELECT count(*) FROM page_links WHERE notebook_id=$1 AND property_key IS NOT NULL", linked*10, book.Notebook)
		sources, empty := 0, linked
		if f.Dense {
			sources, empty = linked*20, 0
		}
		check("sources density", "SELECT count(*) FROM page_properties WHERE notebook_id=$1 AND translate(key,'SOURCE','source')='sources'", sources, book.Notebook)
		check("empty-path non-NULL origin", "SELECT count(*) FROM page_links WHERE notebook_id=$1 AND property_key=''", empty, book.Notebook)
		check("resolved to own Target", "SELECT count(*) FROM page_links WHERE notebook_id=$1 AND resolved_id=$2 AND NOT ambiguous AND NOT aliases AND NOT resolved_asset", linked*10, book.Notebook, book.Target)
		check("unresolved links", "SELECT count(*) FROM page_links WHERE notebook_id=$1 AND resolved_id IS NULL AND NOT ambiguous", linked*10, book.Notebook)
		check("cross-notebook resolution", `SELECT count(*) FROM page_links l JOIN nodes target ON target.id=l.resolved_id WHERE l.notebook_id=$1 AND target.notebook_id<>l.notebook_id`, 0, book.Notebook)
		for kind, n := range map[string]int{"wikilink": 13, "link": 3, "embed": 2, "image": 2} {
			check("kind "+kind, "SELECT count(*) FROM page_links WHERE notebook_id=$1 AND kind=$2", linked*n, book.Notebook, kind)
		}
		check("unchanged prototype property facts", `SELECT count(*) FROM page_properties q LEFT JOIN page_properties p ON p.source_id=$2 AND p.position=q.position
 WHERE q.notebook_id=$1 AND (p.source_id IS NULL OR (q.key,q.value) IS DISTINCT FROM (p.key,p.value))`, 0, book.Notebook, book.Prototype)
		check("unchanged prototype link facts", `SELECT count(*) FROM page_links q LEFT JOIN page_links p ON p.source_id=$2 AND p.range_start=q.range_start
 WHERE q.notebook_id=$1 AND (p.source_id IS NULL OR (q.range_end,q.kind,q.property_key,q.target,q.anchor,q.display,q.target_key,q.target_alt_key,q.resolved_id,q.ambiguous,q.aliases,q.resolved_asset)
 IS DISTINCT FROM (p.range_end,p.kind,p.property_key,p.target,p.anchor,p.display,p.target_key,p.target_alt_key,p.resolved_id,p.ambiguous,p.aliases,p.resolved_asset))`, 0, book.Notebook, book.Prototype)
		lintCostLog(t, "range-migration-facts", struct {
			Notebook, Target, Prototype string
			Dense                       bool
			Pages, Properties, Links    int
			Body, EmptyPath, Resolved   int
		}{book.Notebook, book.Target, book.Prototype, f.Dense, f.PerBook + 2, linked * 20, linked * 20, linked * 10, empty, linked * 10})
	}
}

// The small fixture exercises the same production parser/resolver facts
// and real Down52 -> Down51 -> Up51 -> Up52 boundaries before large costs.
func TestLintMigrationFixturesKeepVersionsAndLinkFacts(t *testing.T) {
	if testing.Short() {
		t.Skip("production migration and real linking fixture, without race")
	}
	for _, dense := range []bool{false, true} {
		t.Run(fmt.Sprintf("dense-%t", dense), func(t *testing.T) {
			f := lintRangeMigrationSeed(t, dense, 2)
			sources := lintSourcesMigrator(t, f.Pool)
			lintSourcesDown(t, sources, f.Pool)
			lintMigrationState(t, f.Pool, false, false)
			ran, err := sources.Up(t.Context())
			lintSourcesApplied(t, ran, err)
			lintMigrationState(t, f.Pool, true, false)
			lintRangeDefinition(t, f.Pool, false)
			if err := sources.Close(); err != nil {
				t.Fatal(err)
			}
			ranges := lintCostMigrator(t, f.Pool, 52)
			ran, err = ranges.Up(t.Context())
			lintRangeApplied(t, ran, err)
			lintMigrationState(t, f.Pool, true, true)
			lintRangeDefinition(t, f.Pool, true)
			lintSourcesDefinition(t, f.Pool)
			lintRangeMigrationFacts(t, f)
		})
	}
}

func TestLintRangeMigrationCostsEmptySources(t *testing.T) { lintRangeMigrationCost(t, false) }

func TestLintRangeMigrationCostsDenseSources(t *testing.T) { lintRangeMigrationCost(t, true) }

// Each process owns 100004 live pages, 2000040 properties and 2000040
// links. Seed and invariant reads warm the input; natural full Up and the
// separately injected commit barrier are not cold/production guarantees.
func lintRangeMigrationCost(t *testing.T, dense bool) {
	t.Helper()
	if testing.Short() {
		t.Skip("full production Up52 uses 100k pages/2M properties/2M links, without race")
	}
	f := lintRangeMigrationSeed(t, dense, 50000)
	pool := f.Pool
	trace := &lintMigrationTrace{Index: lintRangeIndexName}
	cfg := pool.Config()
	cfg.MaxConns = 1
	cfg.ConnConfig.Tracer = trace
	measured, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer measured.Close()
	m := lintCostMigrator(t, measured, 52)
	defer func() { _ = m.Close() }()
	before := lintRangeFootprintOf(t, pool)
	progress := lintObserveMigration(t, pool, "page_links")
	started := time.Now()
	ran, upErr := m.Up(t.Context())
	upElapsed := time.Since(started)
	observed := progress()
	after := lintRangeFootprintOf(t, pool)
	lintRangeApplied(t, ran, upErr)
	lintMigrationState(t, pool, true, true)
	lintRangeDefinition(t, pool, true)
	lintSourcesDefinition(t, pool)
	if observed.Samples == 0 || observed.ShareSamples == 0 || observed.MaxBlocksTotal == 0 || observed.MaxBlocksDone == 0 {
		t.Fatalf("no real range build/SHARE scan observation: %+v", observed)
	}
	var blockSize int64
	if err := pool.QueryRow(t.Context(), "SELECT current_setting('block_size')::bigint").Scan(&blockSize); err != nil {
		t.Fatal(err)
	}
	if observed.MaxBlocksTotal*blockSize != before.HeapBytes {
		t.Fatalf("range scan covers %d blocks of %d bytes, heap=%d", observed.MaxBlocksTotal, blockSize, before.HeapBytes)
	}
	if before.HeapBytes != after.HeapBytes || before.RangeBytes != 0 || after.RangeBytes <= 0 || before.SourcesIndexBytes != after.SourcesIndexBytes {
		t.Fatal("range migration rewrote heap/sources index or did not build a range index")
	}
	trace.mu.Lock()
	build := trace.BuildNS
	trace.mu.Unlock()
	if build <= 0 || build > int64(upElapsed) {
		t.Fatalf("traced production range build=%dns full Up=%s", build, upElapsed)
	}
	lintCostLog(t, "full-production-range-up", struct {
		Dense, WarmInput         bool
		Pages, Properties, Links int
		UpNS, BuildNS, WALBytes  int64
		Before, After            lintRangeFootprint
		Observed                 lintMigrationObservation
	}{dense, true, 100004, 2000040, 2000040, int64(upElapsed), build, sourceIndexWAL(t, pool, before.WAL, after.WAL), before, after, observed})
	lintRangeMigrationFacts(t, f)
	lintRangeDown(t, m, pool)
	lintMigrationLockInterleaving(t, m, pool, 52)
	lintMigrationState(t, pool, true, true)
	lintRangeDefinition(t, pool, true)
	lintRangeMigrationFacts(t, f)
}
