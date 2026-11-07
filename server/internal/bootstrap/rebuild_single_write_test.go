package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The public, default NewAdmin composition must build final link rows once.
// The fixture and its expected values do not depend on a new builder or SQLC
// symbol. On the old implementation all value/event checks pass, then the
// actual Set UPDATE (five resolved rows) is the first failing work assertion.
func TestRebuildInsertsFinalLinksOnce(t *testing.T) {
	for _, mode := range []string{"standard", "llm_wiki"} {
		t.Run(mode, func(t *testing.T) {
			tm := newAcmeTeam(t, "", "")
			nb := tm.createWith(t, tm.tokens["alice"], fmt.Sprintf(`{"name":"Single write","mode":%q}`, mode))
			const content = "---\n\"\": '[[Target]]'\nstatus: ready\ntags: [Alpha]\n---\n" +
				"[[Target#Part|shown]] [[Late]] [[Twin]] [](asset.txt) [[Missing]]\n#Tag #Tag\n"
			// PageIDs is ordered by id. The source is created before both alias
			// providers: a page-at-a-time resolve before collecting later aliases
			// would lose Late, and could miss the tied candidate for Twin.
			source := tm.createPageWith(t, "alice", nb.ID, "", "Source", content)
			target := tm.createPageWith(t, "alice", nb.ID, "", "Target", "Target\n")
			aliasA := tm.createPageWith(t, "alice", nb.ID, "", "AliasA", "---\naliases: [Late, Twin]\n---\n")
			aliasB := tm.createPageWith(t, "alice", nb.ID, "", "AliasB", "---\naliases: [Twin]\n---\n")
			tm.createPageWith(t, "alice", nb.ID, "", "Empty", "")
			asset := tm.upload(t, "alice", nb.ID, "", "asset.txt", "asset bytes\n")
			if count(t, tm.pool, `SELECT count(*) FROM nodes WHERE id=$1 AND id<$2::uuid AND $2::uuid<$3::uuid`, source, aliasA, aliasB) != 1 {
				t.Fatal("fixture does not put the source before the two ordered alias providers")
			}
			before := tm.indexOf(t.Context(), t, nb.ID)
			var workspace string
			if err := tm.pool.QueryRow(t.Context(), "SELECT workspace_id::text FROM notebooks WHERE id=$1", nb.ID).Scan(&workspace); err != nil {
				t.Fatal(err)
			}

			// Keep the original pool setup and use the real reindexAdmin. Only
			// this dedicated pool's tracer changes; it issues no SQL itself.
			pc := tm.pool.Config()
			trace := &rebuildSingleWriteTrace{previous: pc.ConnConfig.Tracer}
			pc.ConnConfig.Tracer = trace
			pool, err := pgxpool.NewWithConfig(t.Context(), pc)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			admin, err := reindexAdmin(pool, testConfig(t, tm.url, false), slog.New(slog.DiscardHandler))
			if err != nil {
				t.Fatal(err)
			}
			trace.start()
			rebuilt, err := admin.Rebuild(t.Context(), uuid.MustParse(nb.ID))
			work := trace.stop()
			if err != nil {
				t.Fatal(err)
			}
			wantPages := 5
			if mode == "llm_wiki" {
				wantPages += 2 // the unchanged, link-free raw and AGENTS templates
			}
			if rebuilt.Pages != wantPages || rebuilt.Links != 6 || rebuilt.Unresolved != 1 || len(rebuilt.Clashes) != 0 {
				t.Fatalf("rebuild counts = %+v; want pages=%d links=6 unresolved=1 no clashes", rebuilt, wantPages)
			}
			// These rows specify every persisted link field, including nullable
			// origin, keys, decorations and all three resolution fields. Byte
			// positions are literals in the document above, not parser output.
			gotLinks := queryStrings(t, tm.pool, `SELECT format('%s:%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s',
				range_start,range_end,kind,CASE WHEN property_key IS NULL THEN 'body' ELSE 'property:'||property_key END,
				target,coalesce(anchor,'-'),coalesce(display,'-'),coalesce(target_key,'-'),coalesce(target_alt_key,'-'),
				coalesce(resolved_id::text,'-'),ambiguous::text,resolved_asset::text,aliases::text)
				FROM page_links WHERE source_id=$1 ORDER BY range_start`, source)
			wantLinks := []string{
				"11:17|wikilink|property:|Target|-|-|target|-|" + target + "|false|false|false",
				"55:61|wikilink|body|Target|Part|shown|target|-|" + target + "|false|false|false",
				"77:81|wikilink|body|Late|-|-|late|-|" + aliasA + "|false|false|false",
				"86:90|wikilink|body|Twin|-|-|twin|-|" + aliasA + "|true|false|false",
				"96:105|link|body|asset.txt|-|-|asset.txt|-|" + asset.ID + "|false|true|false",
				"109:116|wikilink|body|Missing|-|-|missing|-|-|false|false|false",
			}
			if !slices.Equal(gotLinks, wantLinks) {
				t.Fatalf("literal rebuilt links differ:\n%v\nwant\n%v", gotLinks, wantLinks)
			}
			// Keep SQL NULL identity distinct from the printable "-" above.
			gotNulls := queryStrings(t, tm.pool, `SELECT format('%s|%s|%s|%s|%s|%s|%s',
				range_start,(property_key IS NULL)::text,(anchor IS NULL)::text,(display IS NULL)::text,
				(target_key IS NULL)::text,(target_alt_key IS NULL)::text,(resolved_id IS NULL)::text)
				FROM page_links WHERE source_id=$1 ORDER BY range_start`, source)
			wantNulls := []string{
				"11|false|true|true|false|true|false",
				"55|true|false|false|false|true|false",
				"77|true|true|true|false|true|false",
				"86|true|true|true|false|true|false",
				"96|true|true|true|false|true|false",
				"109|true|true|true|false|true|true",
			}
			if !slices.Equal(gotNulls, wantNulls) {
				t.Fatalf("literal rebuilt SQL NULLs differ:\n%v\nwant\n%v", gotNulls, wantNulls)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM page_links WHERE notebook_id=$1", nb.ID); n != 6 {
				t.Fatalf("complete notebook links=%d want6", n)
			}
			headRows := queryStrings(t, tm.pool, `SELECT format('%s|%s|%s|%s',n.name,i.revision,i.extractor,i.frontmatter_valid::text)
				FROM indexed_pages i JOIN nodes n ON n.id=i.node_id WHERE i.notebook_id=$1 ORDER BY n.name COLLATE "C"`, nb.ID)
			wantHeads := []string{"AliasA|1|2|true", "AliasB|1|2|true", "Empty|1|2|true", "Source|1|2|true", "Target|1|2|true"}
			if mode == "llm_wiki" {
				wantHeads = append(wantHeads, "AGENTS|1|2|true", "raw|1|2|true")
				slices.Sort(wantHeads)
			}
			if !slices.Equal(headRows, wantHeads) {
				t.Fatalf("all indexed heads=%v want%v", headRows, wantHeads)
			}
			sideRows := queryStrings(t, tm.pool, `SELECT line FROM (
				SELECT format('tag|%s|%s|%s|%s',n.name,x.tag_key,x.tag,x.count) AS line
				FROM page_tags x JOIN nodes n ON n.id=x.source_id WHERE x.notebook_id=$1
				UNION ALL SELECT format('property|%s|%s|%s|%s',n.name,x.position,x.key,x.value::text)
				FROM page_properties x JOIN nodes n ON n.id=x.source_id WHERE x.notebook_id=$1
				UNION ALL SELECT format('alias|%s|%s|%s',n.name,x.alias_key,x.alias)
				FROM page_aliases x JOIN nodes n ON n.id=x.source_id WHERE x.notebook_id=$1
			) facts ORDER BY line COLLATE "C"`, nb.ID)
			wantSides := []string{
				"alias|AliasA|late|Late", "alias|AliasA|twin|Twin", "alias|AliasB|twin|Twin",
				`property|AliasA|0|aliases|["Late", "Twin"]`, `property|AliasB|0|aliases|["Twin"]`,
				`property|Source|0||"[[Target]]"`, `property|Source|1|status|"ready"`, `property|Source|2|tags|["Alpha"]`,
				"tag|Source|alpha|Alpha|1", "tag|Source|tag|Tag|2",
			}
			slices.Sort(wantSides)
			if !slices.Equal(sideRows, wantSides) {
				t.Fatalf("complete side facts=%v want%v", sideRows, wantSides)
			}
			if after := tm.indexOf(t.Context(), t, nb.ID); !slices.Equal(before, after) {
				t.Fatalf("full index after rebuild differs from the normally written fixture:\n%s", diff(before, after))
			}
			checkPages(t, tm.pool)
			checkAssets(t, tm.pool, tm.storage)
			if len(work.failures) != 0 || len(work.notifications) != 1 {
				t.Fatalf("rebuild native failures=%v notifications=%d want0/1", work.failures, len(work.notifications))
			}
			// Decode the actual pg_notify argument, not a replacement publisher.
			var event struct {
				Type        string          `json:"type"`
				WorkspaceID string          `json:"workspace_id"`
				NotebookID  string          `json:"notebook_id"`
				Data        json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal([]byte(work.notifications[0]), &event); err != nil {
				t.Fatal(err)
			}
			if event.Type != "links" || event.WorkspaceID != workspace || event.NotebookID != nb.ID ||
				string(event.Data) != `{"pages":null,"targets":null,"aliases":false}` {
				t.Fatalf("rebuilt event=%+v data=%s", event, event.Data)
			}
			t.Logf("REBUILD_SINGLE_WRITE work=%+v counts=%+v", work.queries, rebuilt)
			// This order makes the old127 RED about the actual extra UPDATE,
			// after all semantic checks above, not about a new name or type.
			if work.updates.Calls != 0 || work.updates.Rows != 0 || work.upserts != 0 {
				t.Fatalf("REBUILD_SINGLE_WRITE: extra page_links UPDATE calls=%d rows=%d upserts=%d; want zero", work.updates.Calls, work.updates.Rows, work.upserts)
			}
			if work.reads.Calls != 0 || work.reads.Rows != 0 {
				t.Fatalf("REBUILD_SINGLE_WRITE: page_links readback calls=%d rows=%d; want zero", work.reads.Calls, work.reads.Rows)
			}
			if work.inserts.Rows != 6 {
				t.Fatalf("REBUILD_SINGLE_WRITE: actual inserted rows=%d want each of six final links once", work.inserts.Rows)
			}
		})
	}
}

// A read-only observer of the real SQL and CommandTag. Classifying statements
// by their table operation also sees a future SQL name without importing any
// as-yet nonexistent generated constant. The query-name inventory is evidence.
var (
	rebuildSingleInsert = regexp.MustCompile(`(?i)\bINSERT\s+INTO\s+page_links\b`)
	rebuildSingleUpdate = regexp.MustCompile(`(?i)\bUPDATE\s+page_links\b`)
	rebuildSingleRead   = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+page_links\b`)
	rebuildSingleDelete = regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+page_links\b`)
)

type rebuildSingleWriteCount struct{ Calls, Rows int64 }
type rebuildSingleWriteWork struct {
	queries                 map[string]rebuildSingleWriteCount
	inserts, updates, reads rebuildSingleWriteCount
	upserts                 int
	notifications, failures []string
}
type rebuildSingleWriteTrace struct {
	previous pgx.QueryTracer
	mu       sync.Mutex
	active   bool
	work     rebuildSingleWriteWork
}
type rebuildSingleWriteKey struct{}
type rebuildSingleWriteStart struct {
	name, notification           string
	insert, update, read, upsert bool
}

func (r *rebuildSingleWriteTrace) start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.work = rebuildSingleWriteWork{queries: make(map[string]rebuildSingleWriteCount)}
	r.active = true
}
func (r *rebuildSingleWriteTrace) stop() rebuildSingleWriteWork {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = false
	return r.work
}
func (r *rebuildSingleWriteTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if r.previous != nil {
		ctx = r.previous.TraceQueryStart(ctx, conn, d)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.active {
		return ctx
	}
	at := rebuildSingleWriteStart{}
	if strings.HasPrefix(d.SQL, "-- name: ") {
		at.name = strings.Fields(d.SQL)[2]
	}
	// SQLC's whole-line comments are not SQL operations. Strip them before
	// matching so an explanatory comment cannot create a phantom UPDATE.
	var statement strings.Builder
	for _, line := range strings.Split(d.SQL, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			statement.WriteString(line)
			statement.WriteByte('\n')
		}
	}
	sql := statement.String()
	at.insert = rebuildSingleInsert.MatchString(sql)
	at.update = rebuildSingleUpdate.MatchString(sql)
	at.read = rebuildSingleRead.MatchString(sql) && !at.update && !rebuildSingleDelete.MatchString(sql)
	at.upsert = at.insert && strings.Contains(strings.ToUpper(sql), "ON CONFLICT")
	if d.SQL == "SELECT pg_notify($1, $2)" && len(d.Args) == 2 && d.Args[0] == "nwiki_events" {
		at.notification, _ = d.Args[1].(string)
	}
	return context.WithValue(ctx, rebuildSingleWriteKey{}, at)
}
func (r *rebuildSingleWriteTrace) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryEndData) {
	if r.previous != nil {
		r.previous.TraceQueryEnd(ctx, conn, d)
	}
	at, ok := ctx.Value(rebuildSingleWriteKey{}).(rebuildSingleWriteStart)
	if !ok {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.active {
		return
	}
	add := func(c rebuildSingleWriteCount) rebuildSingleWriteCount {
		c.Calls++
		c.Rows += d.CommandTag.RowsAffected()
		return c
	}
	if at.name != "" {
		r.work.queries[at.name] = add(r.work.queries[at.name])
	}
	if at.insert {
		r.work.inserts = add(r.work.inserts)
	}
	if at.update {
		r.work.updates = add(r.work.updates)
	}
	if at.read {
		r.work.reads = add(r.work.reads)
	}
	if at.upsert {
		r.work.upserts++
	}
	if at.notification != "" {
		r.work.notifications = append(r.work.notifications, at.notification)
	}
	if d.Err != nil {
		r.work.failures = append(r.work.failures, fmt.Sprintf("%s: %v", at.name, d.Err))
	}
}
