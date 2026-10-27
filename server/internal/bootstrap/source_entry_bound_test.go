package bootstrap

import (
	"context"
	"maps"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The real HTTP writer and database reader enforce the source-entry bound
// before reading hashes or calling the attachment port. The tracer observes
// returned rows, not physical index access: adapter plan tests prove that
// separate obligation. All production guards and participants remain installed.
func TestSourceEntryStopsBeforeReadingTheWholeSubtree(t *testing.T) {
	trace := &sourceEntryTrace{}
	assets := &sourceEntryAttachments{}
	tm := newAcmeTeamWiring(t, "member", "", wiring{page: func(d *page.Deps) {
		cfg := d.Pool.Config()
		trace.previous = cfg.ConnConfig.Tracer
		cfg.ConnConfig.Tracer = trace
		pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		d.Pool, d.Tx = pool, postgres.NewTxManager(pool, time.Second)
		assets.AttachmentInfos, d.Attachments = d.Attachments, assets
	}})
	wiki := tm.llmWiki(t, "Source entry bound")
	root := tm.createPage(t, "alice", wiki.id, "", "Large")
	tm.addSourceEntryPages(t, root, 9999)
	tm.upload(t, "alice", wiki.id, root, "empty.bin", "")
	if got := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 OR parent_id = $1", root); got != 10001 {
		t.Fatalf("source-entry fixture has %d nodes, want root, 9999 empty pages and one empty asset", got)
	}
	checkPages(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
	before := sourceEntryRows(t, tm.pool)
	trace.start()
	assets.active.Store(true)
	c := nodeMove("alice", root, wiki.raw)
	status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens[c.by], c.body)
	queries := trace.stop()
	assets.active.Store(false)
	var problem struct{ Code string }
	decodeAnswer(t, body, &problem)
	if status != http.StatusUnprocessableEntity || problem.Code != "page.source_too_large" {
		t.Errorf("source-entry overflow = %d %s, want 422 page.source_too_large", status, body)
	}
	// Full persistent rows ensure this rejected move did not change its root.
	if after := sourceEntryRows(t, tm.pool); !maps.Equal(after, before) {
		t.Error("source-entry overflow changed persisted tree, bytes, hashes, history, sessions or notebook")
	}
	if got := queries["SourceChildrenWithin"]; got.calls != 1 || got.rows != 10000 {
		t.Errorf("finite children read = %+v, want one query returning 10000 plus its root", got)
	}
	for _, name := range []string{"ChildrenOfAll", "SourceHeads", "SetSourceHashes", "PageContent", "Contents", "PageStates"} {
		if got := queries[name]; got.calls != 0 {
			t.Errorf("overflow did forbidden work %s: %+v", name, got)
		}
	}
	if got := assets.calls.Load(); got != 0 {
		t.Errorf("overflow called the attachment metadata port %d times", got)
	}
	if t.Failed() {
		return
	}
	// A real delete leaves exactly 10000 live nodes. Empty pages and the empty
	// attachment both count, even though only the latter has a source hash.
	leaf := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE parent_id = $1 AND kind = 'page' ORDER BY id LIMIT 1", root)[0]
	tm.send(t, nodeDeletion("alice", leaf), http.StatusNoContent)
	trace.start()
	assets.calls.Store(0)
	assets.active.Store(true)
	tm.send(t, nodeMove("alice", root, wiki.raw), http.StatusOK)
	queries = trace.stop()
	assets.active.Store(false)
	if got := queries["SourceChildrenWithin"]; got.calls != 2 || got.rows != 9999 {
		t.Errorf("complete 10000-node read = %+v, want its children and final empty leaf probe", got)
	}
	if queries["SourceHeads"].calls == 0 || assets.calls.Load() == 0 {
		t.Error("successful source entry did not exercise the observed hash and attachment readers")
	}
	if got := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND source_hash IS NOT NULL", wiki.id); got != 1 {
		t.Errorf("complete entry has %d source hashes, want the zero-byte asset only", got)
	}
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	// A source tree may grow beyond the entry bound after arriving. Moving
	// it within sources, its source root, or an ancestor preserves membership
	// and must keep the original large-move behavior at the real HTTP entry.
	tm.createPage(t, "alice", wiki.id, root, "One more")
	bucket := tm.createPage(t, "alice", wiki.id, wiki.raw, "Bucket")
	outer := tm.createPage(t, "alice", wiki.id, "", "Outer")
	destination := tm.createPage(t, "alice", wiki.id, "", "Destination")
	for _, move := range []struct{ name, id, parent string }{
		{"within sources", root, bucket},
		{"source root", wiki.raw, outer},
		{"source ancestor", outer, destination},
	} {
		if !t.Run(move.name, func(t *testing.T) {
			if got := count(t, tm.pool, `WITH RECURSIVE carried AS (
				SELECT id FROM nodes WHERE id = $1 AND deleted_at IS NULL
				UNION ALL SELECT n.id FROM nodes n JOIN carried c ON n.parent_id = c.id
				WHERE n.deleted_at IS NULL
			) SELECT count(*) FROM carried`, move.id); got <= 10000 {
				t.Fatalf("membership-preserving move carries %d nodes, want over 10000", got)
			}
			trace.start()
			assets.calls.Store(0)
			assets.active.Store(true)
			tm.send(t, nodeMove("alice", move.id, move.parent), http.StatusOK)
			queries := trace.stop()
			assets.active.Store(false)
			for _, name := range []string{"SourceChildrenWithin", "SourceHeads", "SetSourceHashes", "PageContent", "Contents", "PageStates"} {
				if got := queries[name]; got.calls != 0 {
					t.Errorf("membership-preserving move did source-entry work %s: %+v", name, got)
				}
			}
			if got := assets.calls.Load(); got != 0 {
				t.Errorf("membership-preserving move called the attachment metadata port %d times", got)
			}
			if got := queryStrings(t, tm.pool, "SELECT parent_id::text FROM nodes WHERE id = $1", move.id); len(got) != 1 || got[0] != move.parent {
				t.Errorf("successful move parent = %v, want %s", got, move.parent)
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
		}) {
			return
		}
	}
	// Moving out clears the derived values without reading bodies or assets.
	trace.start()
	assets.calls.Store(0)
	assets.active.Store(true)
	tm.send(t, nodeMove("alice", root, ""), http.StatusOK)
	queries = trace.stop()
	assets.active.Store(false)
	if queries["SourceChildrenWithin"].calls != 0 || queries["SourceHeads"].calls != 0 || assets.calls.Load() != 0 {
		t.Errorf("source-out used source-entry readers: queries=%v, assets=%d", queries, assets.calls.Load())
	}
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	// Ordinary mode retains the original single-move behavior at 10001. The
	// downgrade is real, and the source tree's added page retains that size.
	if status, body := tm.downgrade(t, "alice", wiki.id); status != http.StatusOK {
		t.Fatalf("downgrade = %d %s", status, body)
	}
	if got := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE (id = $1 OR parent_id = $1) AND deleted_at IS NULL", root); got != 10001 {
		t.Fatalf("ordinary positive has %d live nodes, want 10001", got)
	}
	trace.start()
	tm.send(t, nodeMove("alice", root, wiki.raw), http.StatusOK)
	queries = trace.stop()
	if queries["SourceChildrenWithin"].calls != 0 || queries["SourceHeads"].calls != 0 {
		t.Errorf("ordinary 10001-node move used source-entry readers: %v", queries)
	}
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	checkPages(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}

type sourceEntryQuery struct{ calls, rows int64 }
type sourceEntryQueryKey struct{}

type sourceEntryTrace struct {
	previous pgx.QueryTracer
	mu       sync.Mutex
	active   bool
	queries  map[string]sourceEntryQuery
}

func (s *sourceEntryTrace) start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queries, s.active = map[string]sourceEntryQuery{}, true
}

func (s *sourceEntryTrace) stop() map[string]sourceEntryQuery {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	return maps.Clone(s.queries)
}

func (s *sourceEntryTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if s.previous != nil {
		ctx = s.previous.TraceQueryStart(ctx, conn, data)
	}
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	parts := strings.Fields(data.SQL)
	if active && len(parts) > 2 && parts[0] == "--" && parts[1] == "name:" {
		return context.WithValue(ctx, sourceEntryQueryKey{}, parts[2])
	}
	return ctx
}

func (s *sourceEntryTrace) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if s.previous != nil {
		s.previous.TraceQueryEnd(ctx, conn, data)
	}
	if name, ok := ctx.Value(sourceEntryQueryKey{}).(string); ok {
		s.mu.Lock()
		defer s.mu.Unlock()
		q := s.queries[name]
		q.calls++
		q.rows += data.CommandTag.RowsAffected()
		s.queries[name] = q
	}
}

type sourceEntryAttachments struct {
	page.AttachmentInfos
	active atomic.Bool
	calls  atomic.Int64
}

func (s *sourceEntryAttachments) Of(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]page.AttachmentInfo, error) {
	if s.active.Load() {
		s.calls.Add(1)
	}
	return s.AttachmentInfos.Of(ctx, ids)
}

// Seed valid empty pages in one owned setup transaction, with one ordinary
// creation changeset, item and initial version per page. This avoids thousands
// of HTTP setup calls and does not manufacture an oversized write unit. No
// source hash is repaired or changed after the operation under test.
func (tm acmeTeam) addSourceEntryPages(t *testing.T, parent string, n int) {
	t.Helper()
	tx, err := tm.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(t.Context(), `CREATE TEMP TABLE source_entry_seed ON COMMIT DROP AS
		SELECT gen_random_uuid() AS node_id, gen_random_uuid() AS changeset_id,
		coalesce((SELECT max(sort_order)::integer FROM nodes WHERE parent_id = $1), 0) + g AS ord
		FROM generate_series(1, $2::integer) g`, parent, n); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order,
			created_by_id, updated_by_id, created_at, updated_at)
		SELECT s.node_id, p.notebook_id, p.id, 'page', 'Child' || lpad(s.ord::text, 8, '0'),
			'child' || lpad(s.ord::text, 8, '0'), s.ord, p.created_by_id, p.created_by_id, now(), now()
		FROM source_entry_seed s JOIN nodes p ON p.id = $1`,
		`INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
		SELECT s.changeset_id, p.notebook_id, 'edit', 'web', p.created_by_id, now(), now()
		FROM source_entry_seed s JOIN nodes p ON p.id = $1`,
		`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT s.node_id, '', 1, sha256(''::bytea), 0, p.created_by_id, now()
		FROM source_entry_seed s JOIN nodes p ON p.id = $1`,
		`INSERT INTO page_revisions (id, changeset_id, node_id, revision, content, content_hash, byte_size, created_at, updated_at)
		SELECT gen_random_uuid(), s.changeset_id, s.node_id, 1, '', sha256(''::bytea), 0, now(), now()
		FROM source_entry_seed s JOIN nodes p ON p.id = $1`,
		`INSERT INTO changeset_items (id, changeset_id, node_id, after_parent_id, after_name,
			after_sort_order, revision, created_at, updated_at)
		SELECT gen_random_uuid(), s.changeset_id, s.node_id, p.id, n.name, n.sort_order, 1, now(), now()
		FROM source_entry_seed s JOIN nodes p ON p.id = $1 JOIN nodes n ON n.id = s.node_id`,
		`INSERT INTO page_search_queue (node_id, notebook_id, bulk)
		SELECT s.node_id, p.notebook_id, false FROM source_entry_seed s JOIN nodes p ON p.id = $1
		ORDER BY s.node_id`,
	} {
		if _, err := tx.Exec(t.Context(), sql, parent); err != nil {
			t.Fatal(err)
		}
	}
	// Empty content has a valid, empty link extraction at revision one.
	if _, err := tx.Exec(t.Context(), `INSERT INTO indexed_pages (node_id, notebook_id, revision, extractor, frontmatter_valid)
		SELECT s.node_id, p.notebook_id, 1, $2, true FROM source_entry_seed s JOIN nodes p ON p.id = $1`, parent, linking.Extractor); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func sourceEntryRows(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range []string{"nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "asset_blobs", "notebooks", "edit_sessions", "page_search_queue", "indexed_pages"} {
		var digest string
		if err := pool.QueryRow(t.Context(), "SELECT md5(coalesce(string_agg(to_jsonb(x)::text, E'\\n' ORDER BY to_jsonb(x)::text), '')) FROM "+table+" x").Scan(&digest); err != nil {
			t.Fatal(err)
		}
		out[table] = digest
	}
	return out
}
