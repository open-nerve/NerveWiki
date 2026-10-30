//go:build !race

package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

const wikiCostBody = "---\ntype: note\nsummary: cost\n---\nbody\n"

type wikiCostFixture struct {
	*lintRuntimeFixture
	book                        guardedNotebook
	prototype, emitter          string
	source, pending             string
	sourceCreated, assetCreated time.Time
	bulkPages, bulkAssets       int
	trace                       *wikiCostTrace
}

func wikiCostID(kind, n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-700%d-8000-%012x", kind, n+1))
}

// The prototype is written through HTTP/parser/indexing. Bulk rows use the
// same complete creation shape as seedPagesSQL, including current history;
// copied properties are actual prototype facts, never rule answers.
func newWikiCostFixture(t *testing.T) *wikiCostFixture {
	t.Helper()
	trace := &wikiCostTrace{}
	runtimeFixture := newLintRuntime(t, func(d *wiki.Deps) {
		d.Sources = wikiCostSources{d.Sources, trace}
		d.Nodes = wikiCostNodes{d.Nodes, trace}
		d.Rules = wikiCostRules{d.Rules, trace}
		d.Paths = wikiCostPaths{d.Paths, trace}
	})
	trace.previous = runtimeFixture.trace.previous
	runtimeFixture.trace.previous = trace
	f := &wikiCostFixture{lintRuntimeFixture: runtimeFixture, trace: trace}
	started := time.Now()
	f.book = f.tm.llmWiki(t, "Hundred thousand pages")
	f.prototype = f.tm.createPageWith(t, "alice", f.book.id, "", "Prototype", wikiCostBody)
	assetPrototype := f.tm.upload(t, "alice", f.book.id, f.prototype, "needle.js", "cost bytes")
	f.source = f.tm.createPageWith(t, "alice", f.book.id, f.book.raw, "Source", "source payload")
	f.pending = f.tm.upload(t, "alice", f.book.id, f.book.raw, "pending.txt", "pending bytes").ID
	// A deliberately missing ancestor leaves all three links unresolved.
	// The .md spelling chooses a page; the plain .js spelling chooses an
	// asset and must not be a wanted page even with thousands of candidates.
	f.emitter = f.tm.createPageWith(t, "alice", f.book.id, "", "Emitter", wikiCostBody+
		"[[missing/needle.js]]\n[[missing/needle.js.md]]\n[[truly-wanted]]\n")
	f.bulkPages = 100000 - 5 // raw, AGENTS, prototype, Source, Emitter.
	for first := 0; first < f.bulkPages; first += 10000 {
		f.seedPages(t, first, min(10000, f.bulkPages-first))
	}
	f.bulkAssets = f.bulkPages / 10
	f.seedAssets(t, assetPrototype.ID)
	for _, query := range []string{
		"ANALYZE nodes,page_contents,page_revisions,changesets,changeset_items,asset_blobs",
		"ANALYZE indexed_pages,page_properties,page_links,page_tags,page_aliases",
	} {
		lintCostExec(t, f.tm.pool, query)
	}
	f.checkSeed(t)
	if t.Failed() {
		t.FailNow()
	}
	if err := f.tm.pool.QueryRow(t.Context(), "SELECT created_at FROM nodes WHERE id=$1", f.source).Scan(&f.sourceCreated); err != nil {
		t.Fatal(err)
	}
	if err := f.tm.pool.QueryRow(t.Context(), "SELECT created_at FROM nodes WHERE id=$1", f.pending).Scan(&f.assetCreated); err != nil {
		t.Fatal(err)
	}
	if !f.sourceCreated.Before(f.assetCreated) {
		t.Fatal("independent pending order requires source page created before its attachment sibling")
	}
	sourceIndexSettle(t, f.tm.pool)
	lintCostLog(t, "wiki-seed", struct {
		ElapsedNS                                                       int64
		Pages, BulkPages, BulkAssets, LiveAssets, PageDepth, AssetDepth int
		Statistics                                                      string
	}{int64(time.Since(started)), 100000, f.bulkPages, f.bulkAssets, f.bulkAssets + 2, 10, 11, "ANALYZE; ordinary production DescribeExec; warm fixture validation excluded from Execute"})
	return f
}

func (f *wikiCostFixture) seedPages(t *testing.T, first, n int) {
	t.Helper()
	ids, parents, names := make([]uuid.UUID, n), make([]string, n), make([]string, n)
	for i := range n {
		at := first + i
		ids[i] = wikiCostID(0, at)
		if at%10 == 0 {
			names[i] = fmt.Sprintf("branch%05d", at/10)
		} else {
			parents[i] = wikiCostID(0, at-1).String()
			names[i] = fmt.Sprintf("level%d", at%10)
			if at%10 == 9 {
				names[i] = "needle.js"
			}
		}
	}
	tx, err := f.tm.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	changeset := uuid.NewV7()
	for _, query := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at)
 SELECT $1,id,'edit','web',created_by_id,now(),now() FROM notebooks WHERE id=$2`, []any{changeset, f.book.id}},
		{`INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at)
 SELECT u.id,b.id,nullif(u.parent,'')::uuid,'page',u.name,u.name,1000+u.n,b.created_by_id,b.created_by_id,now(),now()
 FROM unnest($1::uuid[],$2::text[],$3::text[]) WITH ORDINALITY u(id,parent,name,n),notebooks b WHERE b.id=$4`, []any{ids, parents, names, f.book.id}},
		{`INSERT INTO page_contents(node_id,content,revision,content_hash,byte_size,updated_by_id,updated_at)
 SELECT n.id,p.content,1,p.content_hash,p.byte_size,n.created_by_id,now() FROM nodes n,page_contents p
 WHERE n.id=ANY($1::uuid[]) AND p.node_id=$2`, []any{ids, f.prototype}},
		{`INSERT INTO page_revisions(id,changeset_id,node_id,revision,content,content_hash,byte_size,created_at,updated_at)
 SELECT gen_random_uuid(),$1,node_id,1,content,content_hash,byte_size,now(),now() FROM page_contents WHERE node_id=ANY($2::uuid[])`, []any{changeset, ids}},
		{`INSERT INTO changeset_items(id,changeset_id,node_id,after_parent_id,after_name,after_sort_order,revision,created_at,updated_at)
 SELECT gen_random_uuid(),$1,id,parent_id,name,sort_order,1,now(),now() FROM nodes WHERE id=ANY($2::uuid[])`, []any{changeset, ids}},
		{`INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT n.id,n.notebook_id,1,p.extractor,p.frontmatter_valid FROM nodes n,indexed_pages p
 WHERE n.id=ANY($1::uuid[]) AND p.node_id=$2`, []any{ids, f.prototype}},
		{`INSERT INTO page_properties(source_id,position,notebook_id,key,value)
 SELECT n.id,p.position,n.notebook_id,p.key,p.value FROM nodes n,page_properties p
 WHERE n.id=ANY($1::uuid[]) AND p.source_id=$2`, []any{ids, f.prototype}},
		{`INSERT INTO page_search_queue(node_id,notebook_id,bulk) SELECT id,notebook_id,false FROM nodes WHERE id=ANY($1::uuid[])`, []any{ids}},
	} {
		if _, err := tx.Exec(t.Context(), query.sql, query.args...); err != nil {
			t.Fatalf("complete wiki seed: %v: %s", err, query.sql)
		}
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func (f *wikiCostFixture) seedAssets(t *testing.T, prototype string) {
	t.Helper()
	ids, blobs, parents := make([]uuid.UUID, f.bulkAssets), make([]uuid.UUID, f.bulkAssets), make([]uuid.UUID, f.bulkAssets)
	for i := range f.bulkAssets {
		ids[i], blobs[i], parents[i] = wikiCostID(1, i), wikiCostID(2, i), wikiCostID(0, i*10+9)
		if err := os.WriteFile(filepath.Join(f.tm.storage, "blobs", blobs[i].String()), []byte("cost bytes"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	changeset := uuid.NewV7()
	for _, query := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at)
 SELECT $1,id,'edit','web',created_by_id,now(),now() FROM notebooks WHERE id=$2`, []any{changeset, f.book.id}},
		{`INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at)
 SELECT u.id,b.id,u.parent,'asset','needle.js','needle.js',1,b.created_by_id,b.created_by_id,now(),now()
 FROM unnest($1::uuid[],$2::uuid[]) u(id,parent),notebooks b WHERE b.id=$3`, []any{ids, parents, f.book.id}},
		{`INSERT INTO asset_blobs(id,node_id,notebook_id,mime,byte_size,sha256,width,height,created_by_id,created_at)
 SELECT u.blob,u.node,p.notebook_id,p.mime,p.byte_size,p.sha256,p.width,p.height,p.created_by_id,now()
 FROM unnest($1::uuid[],$2::uuid[]) u(blob,node),asset_blobs p WHERE p.node_id=$3`, []any{blobs, ids, prototype}},
		{`INSERT INTO changeset_items(id,changeset_id,node_id,after_parent_id,after_name,after_sort_order,created_at,updated_at)
 SELECT gen_random_uuid(),$1,id,parent_id,name,sort_order,now(),now() FROM nodes WHERE id=ANY($2::uuid[])`, []any{changeset, ids}},
		{`INSERT INTO page_search_queue(node_id,notebook_id,bulk) SELECT id,notebook_id,false FROM nodes WHERE id=ANY($1::uuid[])`, []any{ids}},
	} {
		lintCostExec(t, f.tm.pool, query.sql, query.args...)
	}
}

func (f *wikiCostFixture) checkSeed(t *testing.T) {
	t.Helper()
	checks := []struct {
		name, query string
		count       int
		args        []any
	}{
		{"live current pages", `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id JOIN indexed_pages i ON i.node_id=n.id
 WHERE n.notebook_id=$1 AND n.kind='page' AND n.deleted_at IS NULL AND c.deleted_at IS NULL AND i.notebook_id=n.notebook_id AND i.revision=c.revision AND i.extractor=2`, 100000, []any{f.book.id}},
		{"current history", `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id JOIN page_revisions r ON r.node_id=n.id AND r.revision=c.revision
 WHERE n.notebook_id=$1 AND r.deleted_at IS NULL AND (r.content,r.content_hash,r.byte_size)=(c.content,c.content_hash,c.byte_size)
 AND c.content_hash=sha256(convert_to(c.content,'UTF8')) AND c.byte_size=octet_length(c.content)`, 100000, []any{f.book.id}},
		{"page creation items", `SELECT count(DISTINCT n.id) FROM nodes n JOIN changeset_items i ON i.node_id=n.id AND i.revision=1 JOIN changesets s ON s.id=i.changeset_id
 WHERE n.notebook_id=$1 AND n.kind='page' AND i.after_name=n.name AND i.after_parent_id IS NOT DISTINCT FROM n.parent_id AND s.notebook_id=n.notebook_id`, 100000, []any{f.book.id}},
		{"prototype properties", `SELECT count(*) FROM page_properties WHERE source_id=$1 AND ((position=0 AND key='type' AND value='"note"'::jsonb) OR (position=1 AND key='summary' AND value='"cost"'::jsonb))`, 2, []any{f.prototype}},
		{"bulk prototype body", `SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id WHERE n.notebook_id=$1 AND n.id<'00000000-0000-7001-8000-000000000000'::uuid AND c.content=$2`, f.bulkPages, []any{f.book.id, wikiCostBody}},
		{"bulk prototype properties", `SELECT count(*) FROM page_properties p JOIN page_properties template ON template.source_id=$2 AND template.position=p.position
 WHERE p.notebook_id=$1 AND p.source_id<'00000000-0000-7001-8000-000000000000'::uuid AND (p.key,p.value)=(template.key,template.value)`, f.bulkPages * 2, []any{f.book.id, f.prototype}},
		{"deep same-key pages", `SELECT count(*) FROM nodes WHERE notebook_id=$1 AND name_key='needle.js' AND kind='page'`, f.bulkAssets, []any{f.book.id}},
		{"deep same-key assets", `SELECT count(*) FROM nodes WHERE notebook_id=$1 AND name_key='needle.js' AND kind='asset'`, f.bulkAssets + 1, []any{f.book.id}},
		{"unresolved real emitter", `SELECT count(*) FROM page_links WHERE source_id=$1 AND resolved_id IS NULL AND property_key IS NULL AND kind='wikilink'`, 3, []any{f.emitter}},
	}
	for _, check := range checks {
		if got := count(t, f.tm.pool, check.query, check.args...); got != check.count {
			t.Fatalf("wiki cost seed %s=%d, want %d", check.name, got, check.count)
		}
	}
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
	checkAssets(t, f.tm.pool, f.tm.storage)
	assertSourceHashesMatchRebuild(t, t.Context(), f.tm.pool)
}

type wikiCostSQL struct {
	Name, Stage                           string
	PID                                   uint32
	Snapshot                              uint64
	ElapsedNS, Rows, LogicalArgumentBytes int64
	ArrayElements                         int
	Failed                                bool
	PathIDs                               []uuid.UUID `json:",omitempty"`
}
type wikiCostSQLStart struct {
	at  time.Time
	row wikiCostSQL
}
type wikiCostSQLKey struct{}
type wikiCostStageKey struct{}
type wikiCostStage struct {
	Name      string
	ElapsedNS int64
	Rows      int
	Failed    bool
}
type wikiCostTrace struct {
	previous pgx.QueryTracer
	mu       sync.Mutex
	enabled  bool
	queries  []wikiCostSQL
	stages   []wikiCostStage
	timings  *wiki.PreparationTimings
	pathIDs  []uuid.UUID
}

func (p *wikiCostTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	if p.previous != nil {
		ctx = p.previous.TraceQueryStart(ctx, conn, q)
	}
	p.mu.Lock()
	enabled := p.enabled
	p.mu.Unlock()
	if !enabled {
		return ctx
	}
	row := wikiCostSQL{PID: conn.PgConn().PID()}
	row.Stage, _ = ctx.Value(wikiCostStageKey{}).(string)
	row.Snapshot, _ = ctx.Value(lintSnapshotKey{}).(uint64)
	fields := strings.Fields(q.SQL)
	if len(fields) > 2 && fields[0] == "--" && fields[1] == "name:" {
		row.Name = fields[2]
	}
	for _, arg := range q.Args {
		switch value := arg.(type) {
		case []uuid.UUID:
			row.ArrayElements += len(value)
			row.LogicalArgumentBytes += int64(16 * len(value))
			if row.Name == "LinkTargetsByIDs" {
				row.PathIDs = slices.Clone(value)
			}
		case []int32:
			row.ArrayElements += len(value)
			row.LogicalArgumentBytes += int64(4 * len(value))
		case []string:
			row.ArrayElements += len(value)
			for _, s := range value {
				row.LogicalArgumentBytes += int64(len(s))
			}
		case string:
			row.LogicalArgumentBytes += int64(len(value))
		case uuid.UUID:
			row.LogicalArgumentBytes += 16
		}
	}
	return context.WithValue(ctx, wikiCostSQLKey{}, wikiCostSQLStart{time.Now(), row})
}

func (p *wikiCostTrace) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, q pgx.TraceQueryEndData) {
	if p.previous != nil {
		p.previous.TraceQueryEnd(ctx, conn, q)
	}
	start, ok := ctx.Value(wikiCostSQLKey{}).(wikiCostSQLStart)
	if !ok {
		return
	}
	start.row.ElapsedNS = int64(time.Since(start.at))
	start.row.Rows = q.CommandTag.RowsAffected()
	start.row.Failed = q.Err != nil
	p.mu.Lock()
	p.queries = append(p.queries, start.row)
	p.mu.Unlock()
}

func (p *wikiCostTrace) stage(name string, start time.Time, rows int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.enabled {
		p.stages = append(p.stages, wikiCostStage{name, int64(time.Since(start)), rows, err != nil})
	}
}
func (p *wikiCostTrace) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = true
	p.queries = nil
	p.stages = nil
	p.timings = nil
	p.pathIDs = nil
}
func (p *wikiCostTrace) stop() ([]wikiCostSQL, []wikiCostStage, *wiki.PreparationTimings, []uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = false
	return slices.Clone(p.queries), slices.Clone(p.stages), p.timings, slices.Clone(p.pathIDs)
}

type wikiCostSources struct {
	wiki.Sources
	trace *wikiCostTrace
}

func (p wikiCostSources) Nodes(ctx context.Context, notebook, root uuid.UUID) ([]wiki.SourceNode, error) {
	start := time.Now()
	rows, err := p.Sources.Nodes(context.WithValue(ctx, wikiCostStageKey{}, "sources"), notebook, root)
	p.trace.stage("sources", start, len(rows), err)
	return rows, err
}

type wikiCostNodes struct {
	nodes wiki.Nodes
	trace *wikiCostTrace
}

func (p wikiCostNodes) Nodes(ctx context.Context, notebook uuid.UUID) ([]wiki.CompactNode, error) {
	start := time.Now()
	rows, err := p.nodes.Nodes(context.WithValue(ctx, wikiCostStageKey{}, "nodes"), notebook)
	p.trace.stage("nodes", start, len(rows), err)
	return rows, err
}

type wikiCostRules struct {
	wiki.Rules
	trace *wikiCostTrace
}

func (p wikiCostRules) PrepareRules(ctx context.Context, input wiki.ReadInput, query wiki.RuleQuery) (wiki.PreparedRules, error) {
	start := time.Now()
	prepared, err := p.Rules.PrepareRules(context.WithValue(ctx, wikiCostStageKey{}, "rules"), input, query)
	p.trace.stage("rules", start, len(input.Nodes), err)
	if err == nil && prepared != nil {
		v := prepared.Timings()
		p.trace.mu.Lock()
		p.trace.timings = &v
		p.trace.mu.Unlock()
	}
	return prepared, err
}

type wikiCostPaths struct {
	paths wiki.Paths
	trace *wikiCostTrace
}

func (p wikiCostPaths) Paths(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	start := time.Now()
	paths, err := p.paths.Paths(context.WithValue(ctx, wikiCostStageKey{}, "paths"), notebook, ids)
	p.trace.stage("paths", start, len(paths), err)
	p.trace.mu.Lock()
	p.trace.pathIDs = slices.Clone(ids)
	p.trace.mu.Unlock()
	return paths, err
}
