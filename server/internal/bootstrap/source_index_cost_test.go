//go:build !race

package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

const sourceComparedIndex = "nodes_notebook_id_kind_source_hash_idx"

// runApp has an end-of-test cleanup, but a clone template must have no
// connections before the next case starts. This owns only its own app and
// inspection pool; stop is also safe when the normal test cleanup calls it.
func sourceIndexApp(t *testing.T, database string) (acmeTeam, func()) {
	t.Helper()
	cfg := testConfig(t, database, false)
	if cfg.Server.RequestTimeout != 4*time.Second || cfg.Database.CommitTimeout != 2*time.Second {
		t.Fatal("index comparison must keep the test app's normal request/commit deadlines")
	}
	a, err := newApp(context.Background(), cfg, slog.New(slog.DiscardHandler), migrations.FS(), testWebUI())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done, stopped := make(chan error, 1), make(chan error, 1)
	var once sync.Once
	var pool *pgxpool.Pool
	stop := func() {
		once.Do(func() {
			cancel()
			if err := <-stopped; err != nil {
				t.Errorf("source index app stop: %v", err)
			}
			a.close()
			if pool != nil {
				pool.Close()
			}
		})
	}
	t.Cleanup(stop)
	go func() {
		err := a.run(ctx)
		done <- err
		stopped <- err
	}()
	base := waitHealthy(t, cfg.Server.AddrFile, done)
	pool, err = postgres.NewPool(t.Context(), cfg.Database)
	if err != nil {
		t.Fatal(err)
	}
	return acmeTeam{url: database, base: base, storage: cfg.Storage.Dir, pool: pool,
		contract: apitest.Load(t), tokens: map[string]string{}, members: map[string]uuid.UUID{}}, stop
}

func sourceIndexDisconnected(t *testing.T, database string) {
	t.Helper()
	u, err := url.Parse(database)
	if err != nil {
		t.Fatal(err)
	}
	name := strings.TrimPrefix(u.Path, "/")
	u.Path = "/postgres"
	conn, err := pgx.Connect(t.Context(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	// CREATE DATABASE owns the database lock and stops autovacuum workers
	// before copying. They can revisit a closed seed independently of our
	// pools; every other backend must still be absent here.
	var connected int
	if err := conn.QueryRow(t.Context(), `SELECT count(*) FROM pg_stat_activity
		WHERE datname=$1 AND backend_type IS DISTINCT FROM 'autovacuum worker'`, name).Scan(&connected); err != nil {
		t.Fatal(err)
	}
	if connected != 0 {
		t.Fatalf("clone template has %d remaining connections", connected)
	}
}

func sourceIndexFingerprint(t *testing.T, tm acmeTeam, notebook string) string {
	t.Helper()
	rows, err := tm.pool.Query(t.Context(), `SELECT n.id::text,n.parent_id::text,n.kind,n.source_hash,
	 c.content_hash,c.byte_size,c.content FROM nodes n JOIN page_contents c ON c.node_id=n.id
	 WHERE n.notebook_id=$1 AND n.source_hash IS NOT NULL AND n.deleted_at IS NULL ORDER BY n.id`, notebook)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	hash, count := sha256.New(), 0
	for rows.Next() {
		var id, parent, kind, body string
		var sourceHash, contentHash []byte
		var size int64
		if err := rows.Scan(&id, &parent, &kind, &sourceHash, &contentHash, &size, &body); err != nil {
			t.Fatal(err)
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%s\x00%x\x00%x\x00%d\x00%s\n", id, parent, kind, sourceHash, contentHash, size, body)
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 100000 {
		t.Fatalf("comparison starts with %d source pages, want100000", count)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type sourceIndexFootprint struct {
	HeapBytes, AllIndexBytes, TotalBytes, SourceIndexBytes int64
	WAL                                                    string
}

func sourceIndexFootprintOf(t *testing.T, pool *pgxpool.Pool) sourceIndexFootprint {
	t.Helper()
	var s sourceIndexFootprint
	err := pool.QueryRow(t.Context(), `SELECT pg_relation_size('nodes'),pg_indexes_size('nodes'),
	 pg_total_relation_size('nodes'),COALESCE(pg_relation_size(to_regclass($1)),0),pg_current_wal_insert_lsn()::text`, sourceComparedIndex).
		Scan(&s.HeapBytes, &s.AllIndexBytes, &s.TotalBytes, &s.SourceIndexBytes, &s.WAL)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func sourceIndexWAL(t *testing.T, pool *pgxpool.Pool, before, after string) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(t.Context(), "SELECT pg_wal_lsn_diff($1::pg_lsn,$2::pg_lsn)::bigint", after, before).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func sourceIndexDefinition(t *testing.T, pool *pgxpool.Pool, columns int) string {
	t.Helper()
	var keys []string
	var definition, predicate string
	var keyCount, attrs int
	var unique, valid bool
	err := pool.QueryRow(t.Context(), `SELECT i.indnkeyatts,i.indnatts,i.indisunique,i.indisvalid,
	 pg_get_indexdef(i.indexrelid),pg_get_expr(i.indpred,i.indrelid),
	 ARRAY(SELECT pg_get_indexdef(i.indexrelid,k,true) FROM generate_series(1,i.indnkeyatts::int) k)
	 FROM pg_index i WHERE i.indexrelid=to_regclass($1)`, sourceComparedIndex).
		Scan(&keyCount, &attrs, &unique, &valid, &definition, &predicate, &keys)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"notebook_id", "kind", "source_hash"}
	if columns == 4 {
		want = append(want, "id")
	}
	if keyCount != columns || attrs != columns || unique || !valid || !slices.Equal(keys, want) ||
		!strings.Contains(predicate, "source_hash IS NOT NULL") || !strings.Contains(predicate, "deleted_at IS NULL") {
		t.Fatalf("wrong comparison index: %s; keys%v valid%v unique%v", definition, keys, valid, unique)
	}
	if n := count(t, pool, "SELECT count(*) FROM pg_indexes WHERE tablename='nodes' AND indexdef LIKE '%source_hash%'"); n != 1 {
		t.Fatalf("source write would maintain %d source indexes, want1", n)
	}
	return definition
}

// Normal startup jobs are allowed to finish before each write window. The
// runner and normal request/commit deadlines remain unchanged. Cluster WAL
// deltas still include any background activity during that window.
func sourceIndexSettle(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	const ready = "SELECT count(*) FROM river_job WHERE state IN ('available','running','retryable') AND scheduled_at<=now()"
	quiet := 0
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		if count(t, pool, ready) == 0 {
			quiet++
			if quiet == 3 {
				return
			}
		} else {
			quiet = 0
		}
	}
	t.Fatal("startup jobs did not settle before source index write sample")
}

type sourceIndexCostSample struct {
	Pair, Columns                   int
	Fingerprint, Definition         string
	BuildElapsed, WriteElapsed      time.Duration
	BuildWALBytes, WriteWALBytes    int64
	BuildBefore, BuildAfter         sourceIndexFootprint
	WriteBefore, WriteAfter         sourceIndexFootprint
	RequestDeadline, CommitDeadline time.Duration
	CreatedNode, Content            string
}

func sourceIndexCostLog(t *testing.T, value any) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("source index comparison %s", b)
}

// Three paired samples use six independent copies of exactly the same
// initial 100k source rows and full history. Build samples are populated,
// warm-table index builds, not the separate 6M cold migration/backfill.
// Write samples are complete, successful HTTP source creations including
// hash maintenance and history. Their timing/WAL difference is the normal
// application write difference, not isolated index-only work or WAL.
func TestSourceIndexFourthKeyCostsItsSpaceAndNormalWrites(t *testing.T) {
	prepared := pgtest.NewDatabase(t)
	seed, stop := sourceIndexApp(t, prepared)
	seed.tokens["alice"] = registerAccount(t, seed.contract, seed.base, "alice@example.com").AccessToken
	if status, body := ask(t, seed.contract, http.MethodPost, seed.base+"/api/v0/workspaces", seed.tokens["alice"], `{"name":"Acme","slug":"acme"}`); status != http.StatusCreated {
		t.Fatalf("seed workspace=%d %s", status, body)
	}
	var member uuid.UUID
	if err := seed.pool.QueryRow(t.Context(), "SELECT id FROM workspace_members").Scan(&member); err != nil {
		t.Fatal(err)
	}
	seed.members["alice"] = member
	wiki := seed.llmWiki(t, "Index comparison")
	for _, table := range []string{"nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "asset_blobs"} {
		if _, err := seed.pool.Exec(t.Context(), "ALTER TABLE "+table+" SET (autovacuum_enabled=false)"); err != nil {
			t.Fatal(err)
		}
	}
	sourceCostPages(t, seed, wiki.raw, 100000, true)
	sourceCostHealthy(t, seed)
	sourceCostAnalyze(t, seed)
	fingerprint := sourceIndexFingerprint(t, seed, wiki.id)
	stop()
	sourceIndexDisconnected(t, prepared)
	for pair := 1; pair <= 3; pair++ {
		order := []int{3, 4}
		if pair == 2 {
			order = []int{4, 3}
		}
		results := map[int]sourceIndexCostSample{}
		for _, columns := range order {
			t.Run(fmt.Sprintf("pair%d/columns%d", pair, columns), func(t *testing.T) {
				database := pgtest.NewDatabaseFrom(t, prepared)
				pool := connect(t, database)
				if _, err := pool.Exec(t.Context(), "DROP INDEX "+sourceComparedIndex); err != nil {
					t.Fatal(err)
				}
				// With the compared index absent, the same aggregate warms
				// all source-hash table values before either CREATE INDEX.
				var bytes int64
				if err := pool.QueryRow(t.Context(), "SELECT sum(octet_length(source_hash)) FROM nodes").Scan(&bytes); err != nil || bytes != 3200000 {
					t.Fatalf("warm source hash bytes=%d want3200000: %v", bytes, err)
				}
				keys := "notebook_id,kind,source_hash"
				if columns == 4 {
					keys += ",id"
				}
				e := sourceIndexCostSample{Pair: pair, Columns: columns, Fingerprint: fingerprint,
					RequestDeadline: 4 * time.Second, CommitDeadline: 2 * time.Second}
				e.BuildBefore = sourceIndexFootprintOf(t, pool)
				started := time.Now()
				_, err := pool.Exec(t.Context(), "CREATE INDEX "+sourceComparedIndex+" ON nodes ("+keys+") WHERE source_hash IS NOT NULL AND deleted_at IS NULL")
				e.BuildElapsed = time.Since(started)
				if err != nil {
					t.Fatal(err)
				}
				e.BuildAfter = sourceIndexFootprintOf(t, pool)
				e.BuildWALBytes = sourceIndexWAL(t, pool, e.BuildBefore.WAL, e.BuildAfter.WAL)
				e.Definition = sourceIndexDefinition(t, pool, columns)
				if e.BuildBefore.SourceIndexBytes != 0 || e.BuildAfter.SourceIndexBytes <= 0 || e.BuildWALBytes <= 0 {
					t.Fatalf("index build was not materialized: %+v", e)
				}
				pool.Close()
				tm, closeApp := sourceIndexApp(t, database)
				defer closeApp()
				// Each app has its own ephemeral signing key. Authenticate
				// against this clone; no token from seed crosses app lifetimes.
				status, body := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/auth/login", "", `{"email":"alice@example.com","password":"Tr0ub4dor&3"}`)
				var tokens authTokens
				if status != http.StatusOK || json.Unmarshal([]byte(body), &tokens) != nil || tokens.AccessToken == "" {
					t.Fatalf("clone login=%d %s", status, body)
				}
				tm.tokens["alice"], tm.members["alice"] = tokens.AccessToken, member
				if actual := sourceIndexFingerprint(t, tm, wiki.id); actual != fingerprint {
					t.Fatalf("clone source rows changed: fingerprint%s want%s", actual, fingerprint)
				}
				sourceCostHealthy(t, tm)
				sourceCostAnalyze(t, tm)
				sourceIndexSettle(t, tm.pool)
				e.Content = fmt.Sprintf("paired-normal-source-write-%d", pair)
				c := sourceCostCreate(t, tm, wiki.id, wiki.raw, fmt.Sprintf("Paired%d", pair), e.Content)
				e.WriteBefore = sourceIndexFootprintOf(t, tm.pool)
				started = time.Now()
				status, body = askTyped(t, tm.contract, c.method, tm.base+c.path, tm.tokens[c.by], c.contentType, c.body)
				e.WriteElapsed = time.Since(started)
				e.WriteAfter = sourceIndexFootprintOf(t, tm.pool)
				e.WriteWALBytes = sourceIndexWAL(t, tm.pool, e.WriteBefore.WAL, e.WriteAfter.WAL)
				if status != http.StatusCreated {
					sourceIndexCostLog(t, e)
					t.Fatalf("normal source create=%d %s", status, body)
				}
				e.CreatedNode = idOf(t, answer{body: body})
				if n := count(t, tm.pool, "SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id WHERE n.id=$1 AND n.source_hash=sha256(convert_to($2,'UTF8')) AND c.content=$2", e.CreatedNode, e.Content); n != 1 {
					t.Fatal("normal source write did not commit its exact body/hash")
				}
				if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND source_hash IS NOT NULL AND deleted_at IS NULL", wiki.id); n != 100001 {
					t.Fatalf("source rows after normal write=%d want100001", n)
				}
				if e.WriteWALBytes <= 0 || sourceIndexDefinition(t, tm.pool, columns) != e.Definition {
					t.Fatal("normal write produced no WAL or changed the compared index")
				}
				sourceCostHealthy(t, tm)
				sourceIndexCostLog(t, e)
				results[columns] = e
			})
		}
		if a, ok := results[3]; ok {
			if b, ok := results[4]; ok {
				sourceIndexCostLog(t, struct {
					Pair                                                  int
					FourMinusThreeBuild, FourMinusThreeWrite              time.Duration
					FourMinusThreeIndexBytes, FourMinusThreeBuildWALBytes int64
					FourMinusThreeWriteWALBytes                           int64
				}{pair, b.BuildElapsed - a.BuildElapsed, b.WriteElapsed - a.WriteElapsed,
					b.BuildAfter.SourceIndexBytes - a.BuildAfter.SourceIndexBytes,
					b.BuildWALBytes - a.BuildWALBytes, b.WriteWALBytes - a.WriteWALBytes})
			}
		}
	}
}
