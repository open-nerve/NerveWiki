//go:build !race

package bootstrap

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ingestMigrationAt() time.Time { return time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC) }

type ingestMigrationBook struct {
	id, workspace, root, agents uuid.UUID
	actors                      []uuid.UUID
}

type ingestMigrationFixture struct {
	pool  *pgxpool.Pool
	books []ingestMigrationBook
	want  map[string]int64
}

// Each book receives fresh identities in production tables with all original
// constraints/indexes. The only temporary tables map seed ordinals to IDs;
// they disappear at each book's commit and are not a substitute schema.
func ingestMigrationSeed(t *testing.T, pool *pgxpool.Pool, books int) ingestMigrationFixture {
	t.Helper()
	f := ingestMigrationFixture{pool: pool}
	for _, table := range []string{"nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "indexed_pages", "page_search_queue"} {
		lintCostExec(t, pool, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	for number := range books {
		b := ingestMigrationBook{id: uuid.NewV7(), workspace: uuid.NewV7(), root: uuid.NewV7(), agents: uuid.NewV7(),
			actors: []uuid.UUID{uuid.NewV7(), uuid.NewV7(), uuid.NewV7()}}
		ingestMigrationSeedBook(t, pool, b, number)
		f.books = append(f.books, b)
	}
	ingestMigrationSentinels(t, pool, f.books[0])
	// Bodies are empty or the plain text source. Both have exact empty facts,
	// current extractor2 and valid frontmatter; the actual search remains queued.
	lintCostExec(t, pool, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT n.id,n.notebook_id,c.revision,2,true FROM nodes n JOIN page_contents c ON c.node_id=n.id WHERE n.deleted_at IS NULL`)
	lintCostExec(t, pool, `INSERT INTO page_search_queue(node_id,notebook_id,bulk)
 SELECT id,notebook_id,true FROM nodes WHERE deleted_at IS NULL ORDER BY id`)
	n := int64(books)
	// Agents is one explicit template page/revision/item per main book, using
	// main changeset2. Three independent small history sentinels add three rows.
	f.want = map[string]int64{
		"users": 3 * n, "workspaces": n, "workspace_members": 3 * n, "notebooks": n + 1, "notebook_members": 3*n + 1,
		"nodes": 50002*n + 3, "page_contents": 50002*n + 3, "page_revisions": 70001*n + 3,
		"changesets": 100000*n + 3, "changeset_items": 130000*n + 3, "asset_blobs": 0,
		"indexed_pages": 50002*n + 2, "page_links": 0, "page_tags": 0, "page_properties": 0, "page_aliases": 0,
		"page_search": 0, "page_search_queue": 50002*n + 2, "page_search_tags": 0,
	}
	f.check52(t)
	return f
}

func ingestMigrationSeedBook(t *testing.T, pool *pgxpool.Pool, b ingestMigrationBook, number int) {
	t.Helper()
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	at := ingestMigrationAt()
	for i, actor := range b.actors {
		exec(`INSERT INTO users(id,email,password,display_name,created_at,updated_at) VALUES($1,$2,'fixture hash','Actor',$3,$3)`,
			actor, fmt.Sprintf("migration-%d-%d@example.com", number, i), at)
	}
	exec(`INSERT INTO workspaces(id,slug,name,created_by_id,updated_by_id,created_at,updated_at)
 VALUES($1,$2,'History',$3,$3,$4,$4)`, b.workspace, fmt.Sprintf("migration-%d", number), b.actors[0], at)
	exec(`INSERT INTO notebooks(id,workspace_id,name,mode,source_root_id,created_by_id,updated_by_id,created_at,updated_at)
 VALUES($1,$2,'History','llm_wiki',$3,$4,$4,$5,$5)`, b.id, b.workspace, b.root, b.actors[0], at)
	for i, actor := range b.actors {
		role := "member"
		if i == 0 {
			role = "admin"
		}
		exec(`INSERT INTO workspace_members(id,workspace_id,user_id,role,created_by_id,updated_by_id,created_at,updated_at)
 VALUES(uuidv7(),$1,$2,$3,$4,$4,$5,$5)`, b.workspace, actor, role, b.actors[0], at)
		role = "editor"
		if i == 0 {
			role = "admin"
		}
		exec(`INSERT INTO notebook_members(id,notebook_id,user_id,role,created_by_id,updated_by_id,created_at,updated_at)
 VALUES(uuidv7(),$1,$2,$3,$4,$4,$5,$5)`, b.id, actor, role, b.actors[0], at)
	}
	exec(`CREATE TEMP TABLE ingest_migration_cs(k integer PRIMARY KEY,id uuid NOT NULL,at timestamptz NOT NULL) ON COMMIT DROP`)
	exec(`INSERT INTO ingest_migration_cs SELECT k,uuidv7(),$1::timestamptz+k*interval '1 second' FROM generate_series(1,100000) k`, at)
	exec(`INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at)
 SELECT id,$1,'edit',CASE WHEN k%1000=0 THEN 'cli' ELSE (ARRAY['web','api','mcp:a','mcp:b'])[1+k%4] END,
 ($2::uuid[])[1+k%3],at,at FROM ingest_migration_cs`, b.id, b.actors)
	exec(`CREATE TEMP TABLE ingest_migration_pages(k integer PRIMARY KEY,id uuid NOT NULL) ON COMMIT DROP`)
	exec(`INSERT INTO ingest_migration_pages SELECT k,uuidv7() FROM generate_series(1,50000) k`)
	exec(`CREATE TEMP TABLE ingest_migration_events ON COMMIT DROP AS
 SELECT k,n,row_number() OVER(PARTITION BY n ORDER BY k)::integer AS step FROM (
 SELECT k,1+k%50000 AS n FROM ingest_migration_cs WHERE k%5 IN(0,3,4) AND k<>99950
 UNION ALL SELECT 99950,k FROM ingest_migration_pages) e`)
	exec(`CREATE INDEX ON ingest_migration_events(n,step)`)
	// Long's first tree event is its creation in changeset1, with revision1.
	// Agents uses changeset2, so no original live edit becomes an empty log row.
	exec(`INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at)
 VALUES($1,$2,NULL,'page','Long','long',19999,$3,$4,$5,$6),
 ($7,$2,NULL,'page','AGENTS','agents',1,$8,$8,$9,$9)`,
		b.root, b.id, b.actors[1], b.actors[99997%3], at.Add(time.Second), at.Add(99997*time.Second), b.agents, b.actors[2], at.Add(2*time.Second))
	exec(`INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at)
 SELECT p.id,$1,$2,'page','N'||lpad(p.k::text,5,'0'),'n'||lpad(p.k::text,5,'0'),e.steps-1,
 ($3::uuid[])[1+e.first_k%3],($3::uuid[])[1+e.last_k%3],first.at,last.at
 FROM ingest_migration_pages p JOIN (SELECT n,min(k) first_k,max(k) last_k,max(step) steps FROM ingest_migration_events GROUP BY n) e ON e.n=p.k
 JOIN ingest_migration_cs first ON first.k=e.first_k JOIN ingest_migration_cs last ON last.k=e.last_k`, b.id, b.root, b.actors)
	exec(`INSERT INTO page_contents(node_id,content,revision,content_hash,byte_size,updated_by_id,updated_at)
 SELECT p.id,CASE WHEN p.k<=10000 THEN 'source' ELSE '' END,1,
 sha256(convert_to(CASE WHEN p.k<=10000 THEN 'source' ELSE '' END,'UTF8')),
 CASE WHEN p.k<=10000 THEN 6 ELSE 0 END,($1::uuid[])[1+e.k%3],c.at FROM ingest_migration_pages p
 JOIN ingest_migration_events e ON e.n=p.k AND e.step=1 JOIN ingest_migration_cs c ON c.k=e.k`, b.actors)
	exec(`INSERT INTO page_contents(node_id,content,revision,content_hash,byte_size,updated_by_id,updated_at)
 VALUES($1,'',20000,sha256(''),0,$2,$3),($4,'',1,sha256(''),0,$5,$6)`,
		b.root, b.actors[99996%3], at.Add(99996*time.Second), b.agents, b.actors[2], at.Add(2*time.Second))
	exec(`INSERT INTO page_revisions(id,changeset_id,node_id,base_revision,revision,content,content_hash,byte_size,created_at,updated_at)
 SELECT uuidv7(),id,$1,NULLIF((k-1)/5,0),(k-1)/5+1,'',sha256(''),0,at,at FROM ingest_migration_cs WHERE k%5=1`, b.root)
	// All 50k ordinary pages now have their first stored version, attached to
	// their actual creation item. The old read-cost fixture supplied only10k.
	exec(`INSERT INTO page_revisions(id,changeset_id,node_id,revision,content,content_hash,byte_size,created_at,updated_at)
 SELECT uuidv7(),c.id,p.id,1,CASE WHEN p.k<=10000 THEN 'source' ELSE '' END,
 sha256(convert_to(CASE WHEN p.k<=10000 THEN 'source' ELSE '' END,'UTF8')),
 CASE WHEN p.k<=10000 THEN 6 ELSE 0 END,c.at,c.at FROM ingest_migration_pages p
 JOIN ingest_migration_events e ON e.n=p.k AND e.step=1 JOIN ingest_migration_cs c ON c.k=e.k`)
	exec(`INSERT INTO changeset_items(id,changeset_id,node_id,before_parent_id,before_name,before_sort_order,
 after_parent_id,after_name,after_sort_order,created_at,updated_at,revision)
 SELECT uuidv7(),c.id,p.id,CASE WHEN e.step>1 THEN $1::uuid END,
 CASE WHEN e.step>1 THEN 'N'||lpad(p.k::text,5,'0') END,CASE WHEN e.step>1 THEN e.step-2 END,
 $1,'N'||lpad(p.k::text,5,'0'),e.step-1,c.at,c.at,CASE WHEN e.step=1 THEN 1 END
 FROM ingest_migration_events e JOIN ingest_migration_pages p ON p.k=e.n JOIN ingest_migration_cs c ON c.k=e.k`, b.root)
	exec(`INSERT INTO changeset_items(id,changeset_id,node_id,before_name,before_sort_order,after_name,after_sort_order,created_at,updated_at,revision)
 SELECT uuidv7(),c.id,$1,CASE WHEN e.step>1 THEN 'Long' END,CASE WHEN e.step>1 THEN e.step-2 END,
 'Long',e.step-1,c.at,c.at,CASE WHEN e.step=1 THEN 1 END
 FROM (SELECT CASE WHEN k=2 THEN 1 ELSE k END AS k,(k-2)/5+1 AS step FROM ingest_migration_cs WHERE k%5=2) e
 JOIN ingest_migration_cs c ON c.k=e.k`, b.root)
	exec(`INSERT INTO page_revisions(id,changeset_id,node_id,revision,content,content_hash,byte_size,created_at,updated_at)
 SELECT uuidv7(),id,$1,1,'',sha256(''),0,at,at FROM ingest_migration_cs WHERE k=2`, b.agents)
	exec(`INSERT INTO changeset_items(id,changeset_id,node_id,after_name,after_sort_order,created_at,updated_at,revision)
 SELECT uuidv7(),id,$1,'AGENTS',1,at,at,1 FROM ingest_migration_cs WHERE k=2`, b.agents)
	// These ten thousand live descendants are current eligible sources too.
	exec(`UPDATE nodes n SET source_hash=c.content_hash FROM page_contents c,ingest_migration_pages p
 WHERE n.id=p.id AND c.node_id=n.id AND p.k<=10000`)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func ingestMigrationSentinels(t *testing.T, pool *pgxpool.Pool, b ingestMigrationBook) {
	t.Helper()
	at := ingestMigrationAt().Add(100001 * time.Second)
	deletedAt := at.Add(time.Hour)
	deletedBook := uuid.NewV7()
	lintCostExec(t, pool, `INSERT INTO notebooks(id,workspace_id,name,created_by_id,updated_by_id,created_at,updated_at,deleted_at)
 VALUES($1,$2,'Deleted history',$3,$3,$4,$5,$5)`, deletedBook, b.workspace, b.actors[0], at, deletedAt)
	lintCostExec(t, pool, `INSERT INTO notebook_members(id,notebook_id,user_id,role,created_by_id,updated_by_id,created_at,updated_at,deleted_at)
 VALUES(uuidv7(),$1,$2,'admin',$2,$2,$3,$4,$4)`, deletedBook, b.actors[0], at, deletedAt)
	for _, name := range []string{"Trimmed", "Imported", "Deleted"} {
		book, kind := b.id, "edit"
		var deleted any
		if name == "Imported" {
			kind = "import"
		}
		if name == "Deleted" {
			book = deletedBook
			deleted = deletedAt
		}
		page, cs := uuid.NewV7(), uuid.NewV7()
		nodeUpdated := at
		if name == "Deleted" {
			nodeUpdated = deletedAt
		}
		lintCostExec(t, pool, `INSERT INTO changesets(id,notebook_id,kind,client,created_by_id,created_at,updated_at,deleted_at,trimmed,message)
 VALUES($1,$2,$3,'api',$4,$5,$5,$6,$7,$8)`, cs, book, kind, b.actors[0], at, deleted, name == "Trimmed", name+" sentinel")
		lintCostExec(t, pool, `INSERT INTO nodes(id,notebook_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at,deleted_at)
 VALUES($1,$2,'page',$3,$4,0,$5,$5,$6,$7,$8)`, page, book, name, strings.ToLower(name), b.actors[0], at, nodeUpdated, deleted)
		lintCostExec(t, pool, `INSERT INTO page_contents(node_id,content,revision,content_hash,byte_size,updated_by_id,updated_at,deleted_at)
 VALUES($1,'',1,sha256(''),0,$2,$3,$4)`, page, b.actors[0], at, deleted)
		lintCostExec(t, pool, `INSERT INTO page_revisions(id,changeset_id,node_id,revision,content,content_hash,byte_size,created_at,updated_at,deleted_at)
 VALUES(uuidv7(),$1,$2,1,'',sha256(''),0,$3,$3,$4)`, cs, page, at, deleted)
		lintCostExec(t, pool, `INSERT INTO changeset_items(id,changeset_id,node_id,after_name,after_sort_order,revision,created_at,updated_at,deleted_at)
 VALUES(uuidv7(),$1,$2,$3,0,1,$4,$4,$5)`, cs, page, name, at, deleted)
	}
}

func (f ingestMigrationFixture) check52(t *testing.T) {
	t.Helper()
	for table, want := range f.want {
		if got := int64(count(t, f.pool, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize())); got != want {
			t.Fatalf("INGEST_MIGRATION_NONEMPTY: %s rows=%d want=%d", table, got, want)
		}
	}
	for _, b := range f.books {
		for _, check := range []struct {
			sql  string
			want int
		}{
			{`SELECT count(*) FROM changesets WHERE notebook_id=$1 AND created_at<=$2`, 100000},
			{`SELECT count(*) FROM nodes WHERE notebook_id=$1 AND (id=$2 OR parent_id=$2)`, 50001},
			{`SELECT count(*) FROM page_revisions r JOIN nodes n ON n.id=r.node_id WHERE n.notebook_id=$1 AND (n.id=$2 OR n.parent_id=$2)`, 70000},
			{`SELECT count(*) FROM changeset_items i JOIN nodes n ON n.id=i.node_id WHERE n.notebook_id=$1 AND (n.id=$2 OR n.parent_id=$2)`, 129999},
		} {
			// The predicates use exactly the arguments they mention.
			args := []any{b.id, b.root}
			if strings.Contains(check.sql, "created_at") {
				args = []any{b.id, ingestMigrationAt().Add(100000 * time.Second)}
			}
			if got := count(t, f.pool, check.sql, args...); got != check.want {
				t.Fatalf("main history count=%d want=%d", got, check.want)
			}
		}
		if n := count(t, f.pool, `SELECT count(*) FROM changeset_items i JOIN changesets c ON c.id=i.changeset_id WHERE c.notebook_id=$1 AND c.created_at=$2`, b.id, ingestMigrationAt().Add(99950*time.Second)); n != 50000 {
			t.Fatalf("large history items=%d", n)
		}
	}
	for _, query := range []string{
		`SELECT count(*) FROM page_contents WHERE content_hash<>sha256(convert_to(content,'UTF8')) OR byte_size<>octet_length(content)`,
		`SELECT count(*) FROM page_revisions WHERE content_hash<>sha256(convert_to(content,'UTF8')) OR byte_size<>octet_length(content)`,
		`SELECT count(*) FROM page_contents c WHERE c.revision IS DISTINCT FROM (SELECT max(r.revision) FROM page_revisions r WHERE r.node_id=c.node_id)`,
		`SELECT count(*) FROM page_revisions r JOIN changesets c ON c.id=r.changeset_id JOIN nodes n ON n.id=r.node_id WHERE n.notebook_id<>c.notebook_id`,
		`SELECT count(*) FROM changeset_items i JOIN changesets c ON c.id=i.changeset_id JOIN nodes n ON n.id=i.node_id WHERE n.notebook_id<>c.notebook_id`,
		`SELECT count(*) FROM (SELECT i.*,row_number() OVER(PARTITION BY i.node_id ORDER BY c.created_at,c.id) k,
 lag(i.after_name) OVER(PARTITION BY i.node_id ORDER BY c.created_at,c.id) prior_name,
 lag(i.after_sort_order) OVER(PARTITION BY i.node_id ORDER BY c.created_at,c.id) prior_sort,
 lag(i.after_parent_id) OVER(PARTITION BY i.node_id ORDER BY c.created_at,c.id) prior_parent
 FROM changeset_items i JOIN changesets c ON c.id=i.changeset_id) s
 WHERE (k=1 AND before_name IS NOT NULL) OR (k>1 AND (before_name,before_sort_order,before_parent_id) IS DISTINCT FROM (prior_name,prior_sort,prior_parent))`,
		`SELECT count(*) FROM nodes n JOIN (
 SELECT DISTINCT ON (i.node_id) i.node_id,i.after_name,i.after_sort_order,i.after_parent_id
 FROM changeset_items i JOIN changesets c ON c.id=i.changeset_id
 ORDER BY i.node_id,c.created_at DESC,c.id DESC) i ON i.node_id=n.id
 WHERE (n.name,n.sort_order,n.parent_id) IS DISTINCT FROM (i.after_name,i.after_sort_order,i.after_parent_id)`,
	} {
		if n := count(t, f.pool, query); n != 0 {
			t.Fatalf("INGEST_MIGRATION_VALID_HISTORY: %d violations of %s", n, query)
		}
	}
	if n := count(t, f.pool, sourceHashViolationSQL); n != 0 {
		t.Fatalf("source hash violations=%d", n)
	}
	checkLinks(t, f.pool)
	checkSearch(t, f.pool)
}

type ingestMigrationTable struct {
	Name         string
	Columns, Key []string
}
type ingestMigrationDigest struct {
	Rows, Bytes int64
	SHA256      string
}
type ingestMigrationHash struct {
	hash.Hash
	bytes int64
}

func (h *ingestMigrationHash) Write(p []byte) (int, error) {
	n, err := h.Hash.Write(p)
	h.bytes += int64(n)
	return n, err
}

// Freeze the original52 column list once. Later snapshots never subtract a
// guessed set of new fields or re-read a possibly altered projection.
func (f ingestMigrationFixture) oldTables(t *testing.T) []ingestMigrationTable {
	t.Helper()
	var names []string
	for name := range f.want {
		names = append(names, name)
	}
	slices.Sort(names)
	var out []ingestMigrationTable
	for _, name := range names {
		table := ingestMigrationTable{Name: name}
		err := f.pool.QueryRow(t.Context(), `SELECT
 ARRAY(SELECT a.attname::text FROM pg_attribute a WHERE a.attrelid=$1::regclass AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum),
 ARRAY(SELECT a.attname::text FROM pg_index i CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY k(attnum,ord)
 JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.attnum WHERE i.indrelid=$1::regclass AND i.indisprimary AND k.ord<=i.indnkeyatts ORDER BY k.ord)`, name).Scan(&table.Columns, &table.Key)
		if err != nil || len(table.Columns) == 0 || len(table.Key) == 0 {
			t.Fatalf("old52 columns/real PK %s: %+v, %v", name, table, err)
		}
		if name == "changesets" && slices.Contains(table.Columns, "ingest_count") {
			t.Fatal("target was already53 before its first Up")
		}
		out = append(out, table)
	}
	return out
}

func (f ingestMigrationFixture) image(t *testing.T, tables []ingestMigrationTable) map[string]ingestMigrationDigest {
	t.Helper()
	conn, err := f.pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	out := map[string]ingestMigrationDigest{}
	for _, table := range tables {
		quote := func(names []string) string {
			parts := make([]string, len(names))
			for i, name := range names {
				parts[i] = pgx.Identifier{name}.Sanitize()
			}
			return strings.Join(parts, ",")
		}
		sql := "COPY (SELECT " + quote(table.Columns) + " FROM " + pgx.Identifier{table.Name}.Sanitize() + " ORDER BY " + quote(table.Key) + ") TO STDOUT (FORMAT binary)"
		h := &ingestMigrationHash{Hash: sha256.New()}
		tag, err := conn.Conn().PgConn().CopyTo(t.Context(), h, sql)
		if err != nil {
			t.Fatal(err)
		}
		var counted int64
		if err := conn.QueryRow(t.Context(), "SELECT count(*) FROM "+pgx.Identifier{table.Name}.Sanitize()).Scan(&counted); err != nil {
			t.Fatal(err)
		}
		if counted != f.want[table.Name] || tag.RowsAffected() != counted || h.bytes <= 0 {
			t.Fatalf("INGEST_MIGRATION_COPY: %s tag=%d count=%d bytes=%d", table.Name, tag.RowsAffected(), counted, h.bytes)
		}
		out[table.Name] = ingestMigrationDigest{Rows: counted, Bytes: h.bytes, SHA256: fmt.Sprintf("%x", h.Sum(nil))}
	}
	return out
}

func (f ingestMigrationFixture) sameImage(t *testing.T, tables []ingestMigrationTable, before map[string]ingestMigrationDigest, stage string) {
	t.Helper()
	after := f.image(t, tables)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("INGEST_MIGRATION_OLD_IMAGE %s: before=%+v after=%+v", stage, before, after)
	}
	lintCostLog(t, "ingest-old-image-"+stage, after)
}
