package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// A real parser/indexed prototype supplies the complete body, hash and facts.
// Bulk pages retain real creation history, current revisions and extraction;
// none of the rows contain an answer from the virtual file generator.
type virtualExportLoad struct {
	tm           acmeTeam
	book         guardedNotebook
	prototype    string
	trace        *lintRuntimeTrace
	read, export *lintRuntimeSnapshots
	lintEvents   atomic.Int32
	percentRun   int
}

func newVirtualExportLoad(t *testing.T, name string) *virtualExportLoad {
	return newVirtualExportLoadConfigured(t, name, nil, nil, nil)
}

func newVirtualExportLoadConfigured(t *testing.T, name string, inspect func(*config.Config), observer pgx.QueryTracer, observeTransfer func(*transfer.Deps)) *virtualExportLoad {
	t.Helper()
	f := &virtualExportLoad{trace: &lintRuntimeTrace{}}
	var traced *pgxpool.Pool
	w := wiring{
		queryTracer: observer,
		page: func(d *page.Deps) {
			cfg := d.Pool.Config()
			f.trace.previous = cfg.ConnConfig.Tracer
			cfg.ConnConfig.Tracer = f.trace
			var err error
			traced, err = pgxpool.NewWithConfig(t.Context(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(traced.Close)
			f.read = &lintRuntimeSnapshots{Snapshots: d.Snapshots, pool: traced}
			d.Snapshots = f.read
		},
		transfer: func(d *transfer.Deps) {
			f.export = &lintRuntimeSnapshots{Snapshots: d.Snapshots, pool: traced}
			d.Snapshots = f.export
			if observeTransfer != nil {
				observeTransfer(d)
			}
		},
		wiki: func(d *wiki.Deps) { d.RunObserver = func(wiki.RunEvent) { f.lintEvents.Add(1) } },
	}
	f.tm = newAcmeTeamOn(t, "member", "", inspect, &w, slog.New(slog.DiscardHandler))
	f.book = f.tm.llmWiki(t, name)
	f.prototype = f.tm.createPageWith(t, "alice", f.book.id, "", "Prototype", "---\ntype: note\nsummary: bounded fixture\n---\nbody")
	return f
}

func (f *virtualExportLoad) seed(t *testing.T, n int, longNames bool) []uuid.UUID {
	return f.seedAfter(t, n, longNames, 0, 3)
}

func (f *virtualExportLoad) seedAfter(t *testing.T, n int, longNames bool, offset, existingPages int) []uuid.UUID {
	t.Helper()
	var all []uuid.UUID
	for first := 0; first < n; first += 1000 {
		count := min(1000, n-first)
		ids, changes, names := make([]uuid.UUID, count), make([]uuid.UUID, count), make([]string, count)
		var change uuid.UUID
		for i := range count {
			ids[i] = uuid.NewV7()
			if i%10 == 0 {
				change = uuid.NewV7()
			}
			changes[i] = change
			names[i] = fmt.Sprintf("page%06d", offset+first+i)
			if longNames {
				percent := f.percentRun
				if percent == 0 {
					percent = 240
				}
				names[i] = fmt.Sprintf("%05d-", first+i) + strings.Repeat("%", percent)
			}
		}
		tx, err := f.tm.pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at)
 SELECT u.id,b.id,'edit','web',b.created_by_id,now(),now() FROM (SELECT DISTINCT unnest($1::uuid[]) id) u,notebooks b WHERE b.id=$2`, []any{changes, f.book.id}},
			{`INSERT INTO nodes(id,notebook_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at)
 SELECT u.id,b.id,'page',u.name,u.name,1000+u.n,b.created_by_id,b.created_by_id,now(),now()
 FROM unnest($1::uuid[],$2::text[]) WITH ORDINALITY u(id,name,n),notebooks b WHERE b.id=$3`, []any{ids, names, f.book.id}},
			{`INSERT INTO page_contents(node_id,content,revision,content_hash,byte_size,updated_by_id,updated_at)
 SELECT n.id,p.content,1,p.content_hash,p.byte_size,n.created_by_id,now() FROM nodes n,page_contents p WHERE n.id=ANY($1::uuid[]) AND p.node_id=$2`, []any{ids, f.prototype}},
			{`INSERT INTO page_revisions(id,changeset_id,node_id,revision,content,content_hash,byte_size,created_at,updated_at)
 SELECT gen_random_uuid(),u.cs,c.node_id,1,c.content,c.content_hash,c.byte_size,now(),now()
 FROM unnest($1::uuid[],$2::uuid[]) u(id,cs) JOIN page_contents c ON c.node_id=u.id`, []any{ids, changes}},
			{`INSERT INTO changeset_items(id,changeset_id,node_id,after_parent_id,after_name,after_sort_order,revision,created_at,updated_at)
 SELECT gen_random_uuid(),u.cs,n.id,n.parent_id,n.name,n.sort_order,1,now(),now()
 FROM unnest($1::uuid[],$2::uuid[]) u(id,cs) JOIN nodes n ON n.id=u.id`, []any{ids, changes}},
			{`INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT n.id,n.notebook_id,1,p.extractor,p.frontmatter_valid FROM nodes n,indexed_pages p WHERE n.id=ANY($1::uuid[]) AND p.node_id=$2`, []any{ids, f.prototype}},
			{`INSERT INTO page_properties(source_id,position,notebook_id,key,value)
 SELECT n.id,p.position,n.notebook_id,p.key,p.value FROM nodes n,page_properties p WHERE n.id=ANY($1::uuid[]) AND p.source_id=$2`, []any{ids, f.prototype}},
			{`INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,property_key,target,anchor,display,target_key,target_alt_key,resolved_id,ambiguous,aliases,resolved_asset)
 SELECT n.id,p.range_start,p.range_end,n.notebook_id,p.kind,p.property_key,p.target,p.anchor,p.display,p.target_key,p.target_alt_key,p.resolved_id,p.ambiguous,p.aliases,p.resolved_asset
 FROM nodes n,page_links p WHERE n.id=ANY($1::uuid[]) AND p.source_id=$2`, []any{ids, f.prototype}},
			{`INSERT INTO page_search_queue(node_id,notebook_id,bulk) SELECT id,notebook_id,false FROM nodes WHERE id=ANY($1::uuid[])`, []any{ids}},
		} {
			if _, err := tx.Exec(t.Context(), q.sql, q.args...); err != nil {
				_ = tx.Rollback(context.Background())
				t.Fatalf("complete virtual-file fixture: %v", err)
			}
		}
		if err := tx.Commit(t.Context()); err != nil {
			t.Fatal(err)
		}
		all = append(all, ids...)
	}
	if got := count(t, f.tm.pool, `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id
 JOIN indexed_pages i ON i.node_id=n.id JOIN page_revisions r ON r.node_id=n.id AND r.revision=c.revision
 WHERE n.notebook_id=$1 AND n.deleted_at IS NULL AND c.deleted_at IS NULL AND i.revision=c.revision AND i.extractor=2
 AND (r.content,r.content_hash,r.byte_size)=(c.content,c.content_hash,c.byte_size)
	AND c.content_hash=sha256(convert_to(c.content,'UTF8')) AND c.byte_size=octet_length(c.content)`, f.book.id); got != n+existingPages {
		t.Fatalf("complete current parser/history fixture = %d, want %d", got, n+existingPages)
	}
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
	return all
}

func (f *virtualExportLoad) resetTrace() {
	f.trace.mu.Lock()
	f.trace.queries = nil
	f.trace.mu.Unlock()
}

func virtualExportNoInbound(t *testing.T) {
	f := newVirtualExportLoad(t, "BoundedWiki")
	f.seed(t, 10002, false)
	f.resetTrace()
	entries := virtualExportEntries(t, f.tm, f.book.id, "")
	index, log := entries["BoundedWiki/index.md"].data, entries["BoundedWiki/log.md"].data
	if len(index) > 2<<20 || !strings.Contains(index, "Showing 10000 of 10005 pages.") || len(log) > 1<<20 || !strings.Contains(log, "Showing 1000 of the most recent 1000 changesets.") {
		t.Fatalf("real export ignored its item/history/byte limits: index=%d log=%d", len(index), len(log))
	}
	queries, _ := f.trace.copy()
	heads, history := 0, 0
	for _, q := range queries {
		switch q.Name {
		case "InboundCounts":
			t.Fatal("export executed the read-only inbound profile's SQL")
		case "IndexPages":
			heads++
		case "ListChangesets":
			history++
		}
	}
	if heads != 1 || history != 1 || f.export.next.Load() != 1 || f.lintEvents.Load() != 0 {
		t.Fatalf("export did not use its one observed snapshot without lint: heads=%d history=%d snapshots=%d lint=%d", heads, history, f.export.next.Load(), f.lintEvents.Load())
	}
}

func virtualExportPercentBudget(t *testing.T) { virtualExportPercentBudgetOf(t, 240) }

func virtualExportPercentBudgetOf(t *testing.T, percent int) {
	f := newVirtualExportLoad(t, "EncodedWiki")
	f.percentRun = percent
	f.seed(t, 5500, true)
	entries := virtualExportEntries(t, f.tm, f.book.id, "")
	for _, c := range []struct {
		name string
		cap  int
		end  *regexp.Regexp
	}{{"index.md", 2 << 20, regexp.MustCompile(`\nShowing (\d+) of (\d+) pages\.`)},
		{"log.md", 1 << 20, regexp.MustCompile(`\nShowing (\d+) of the most recent (\d+) changesets\.`)}} {
		body := entries["EncodedWiki/"+c.name].data
		footer := c.end.FindStringSubmatch(body)
		if len(body) > c.cap || footer == nil {
			t.Fatalf("encoded %s crossed its real ZIP byte budget or lost its footer: %d", c.name, len(body))
		}
		shown, _ := strconv.Atoi(footer[1])
		total, _ := strconv.Atoi(footer[2])
		if shown < 1 || shown >= total || strings.Count(body, "\n- ") != shown {
			t.Fatalf("encoded %s failed to omit complete items with an honest footer: %d/%d", c.name, shown, total)
		}
		for _, link := range regexp.MustCompile(`\]\(<([^>]*)>\)`).FindAllStringSubmatch(body, -1) {
			path := strings.ReplaceAll(link[1], "%25", "%")
			if _, exists := entries["EncodedWiki/"+path]; !exists {
				t.Fatalf("encoded %s truncated a destination that has no real ZIP entry: %q", c.name, link[1])
			}
		}
	}
}
