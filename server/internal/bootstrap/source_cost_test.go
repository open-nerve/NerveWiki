//go:build !race

package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// M10/P2 source requests: three samples in each of no_statistics, analyzed,
// and force_generic_setting (the source ports still use Planned). The worst
// state's fastest create is150.124ms; maximum successful large operations are
// move465.672ms/restore2531.534ms/revert2513.225ms/Forget1013.238ms.
// New source limits are three times those baselines, rounded up to100ms.
// Existing request4s/commit2s deadlines and all older guards are unchanged.
const (
	sourceCreateTime  = 500 * time.Millisecond
	sourceMoveTime    = 1400 * time.Millisecond
	sourceRestoreTime = 7600 * time.Millisecond
	sourceRevertTime  = 7600 * time.Millisecond
	sourceForgetTime  = 3100 * time.Millisecond
)

func sourceCostWithin(t *testing.T, label string, took, bound time.Duration) {
	t.Helper()
	t.Logf("source-cost %s elapsed=%s bound=%s", label, took, bound)
	if took > bound {
		t.Errorf("source-cost %s took %s, bound %s", label, took, bound)
	}
}

// Complete initial creation history for every seed precedes measurement.
// The payload is exactly64 UTF-8 bytes, with zero links, unique per node.
func sourceCostPages(t *testing.T, tm acmeTeam, parent string, n int, inside bool) {
	t.Helper()
	for start := 0; start < n; start += 10000 {
		tm.addSourceEntryPages(t, parent, min(10000, n-start))
	}
	for _, sql := range []string{
		`UPDATE page_contents c SET content=rpad('source-cost:'||c.node_id::text,64,'.'), byte_size=64,
   content_hash=sha256(convert_to(rpad('source-cost:'||c.node_id::text,64,'.'),'UTF8')) FROM nodes n WHERE n.id=c.node_id AND n.parent_id=$1`,
		`UPDATE page_revisions r SET content=c.content,byte_size=c.byte_size,content_hash=c.content_hash
   FROM page_contents c JOIN nodes n ON n.id=c.node_id WHERE r.node_id=c.node_id AND r.revision=c.revision AND n.parent_id=$1`,
	} {
		if _, err := tm.pool.Exec(t.Context(), sql, parent); err != nil {
			t.Fatal(err)
		}
	}
	if inside {
		if _, err := tm.pool.Exec(t.Context(), "UPDATE nodes n SET source_hash=c.content_hash FROM page_contents c WHERE n.id=c.node_id AND n.parent_id=$1", parent); err != nil {
			t.Fatal(err)
		}
	}
}

func sourceCostHealthy(t *testing.T, tm acmeTeam) {
	t.Helper()
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
}

func sourceCostAnalyze(t *testing.T, tm acmeTeam) {
	t.Helper()
	if _, err := tm.pool.Exec(t.Context(), "ANALYZE nodes,page_contents,page_revisions,changesets,changeset_items,asset_blobs"); err != nil {
		t.Fatal(err)
	}
	t.Log("source-cost setup: 100000 existing source pages, payload64 bytes, zero links; ANALYZE complete, setup and invariant scans excluded")
}

func sourceCostTimed(t *testing.T, tm acmeTeam, label string, c step, want int) (string, time.Duration) {
	t.Helper()
	at := time.Now()
	status, body := askTyped(t, tm.contract, c.method, tm.base+c.path, tm.tokens[c.by], c.contentType, c.body)
	took := time.Since(at)
	t.Logf("source-cost operation=%s elapsed_ns=%d status=%d", label, took.Nanoseconds(), status)
	if status != want {
		t.Fatalf("source-cost %s = %d %s want%d", label, status, body, want)
	}
	return body, took
}

func sourceCostCreate(t *testing.T, tm acmeTeam, nb, parent, title, body string) step {
	t.Helper()
	value, err := json.Marshal(map[string]any{"parent_id": parent, "title": title, "content": body})
	if err != nil {
		t.Fatal(err)
	}
	return request("alice", http.MethodPost, "/api/v0/notebooks/"+nb+"/pages", string(value))
}

func TestSourceCreateAmongHundredThousandTakesItsTime(t *testing.T) {
	sourceHTTPCost(t, "create")
}

// One empty root +99 nonempty pages +9900 distinct attachments. Every file
// is actually materialized, and its complete initial row/item is committed.
func sourceCostItem(t *testing.T, tm acmeTeam, wiki guardedNotebook) string {
	t.Helper()
	root := tm.createPage(t, "alice", wiki.id, "", "Mixed group")
	sourceCostPages(t, tm, root, 99, false)
	tx, err := tm.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	for _, sql := range []string{
		`CREATE TEMP TABLE source_cost_assets ON COMMIT DROP AS SELECT gen_random_uuid() id,gen_random_uuid() blob,gen_random_uuid() changeset,g FROM generate_series(1,9900) g`,
		`INSERT INTO nodes (id,notebook_id,parent_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at)
   SELECT s.id,p.notebook_id,p.id,'asset','cost'||s.g||'.bin','cost'||s.g||'.bin',100+s.g,p.created_by_id,p.created_by_id,now(),now() FROM source_cost_assets s JOIN nodes p ON p.id=$1`,
		`INSERT INTO changesets (id,notebook_id,kind,client,created_by_id,created_at,updated_at)
   SELECT s.changeset,p.notebook_id,'edit','web',p.created_by_id,now(),now() FROM source_cost_assets s JOIN nodes p ON p.id=$1`,
		`INSERT INTO changeset_items (id,changeset_id,node_id,after_parent_id,after_name,after_sort_order,created_at,updated_at)
   SELECT gen_random_uuid(),s.changeset,n.id,n.parent_id,n.name,n.sort_order,now(),now() FROM source_cost_assets s JOIN nodes n ON n.id=s.id`,
		`INSERT INTO asset_blobs (id,node_id,notebook_id,mime,byte_size,sha256,created_by_id,created_at)
   SELECT s.blob,n.id,n.notebook_id,'application/octet-stream',64,sha256(convert_to(rpad('source-cost-file:'||s.blob::text,64,'.'),'UTF8')),n.created_by_id,now() FROM source_cost_assets s JOIN nodes n ON n.id=s.id`,
		`INSERT INTO page_search_queue (node_id,notebook_id,bulk) SELECT n.id,n.notebook_id,false FROM source_cost_assets s JOIN nodes n ON n.id=s.id ORDER BY n.id`,
	} {
		var args []any
		if strings.Contains(sql, "$1") {
			args = []any{root}
		}
		if _, err := tx.Exec(t.Context(), sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := tx.Query(t.Context(), "SELECT blob::text,rpad('source-cost-file:'||blob::text,64,'.') FROM source_cost_assets ORDER BY blob")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tm.storage, "blobs"), 0700); err != nil {
		t.Fatal(err)
	}
	written := 0
	for rows.Next() {
		var id, payload string
		if err := rows.Scan(&id, &payload); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tm.storage, "blobs", id), []byte(payload), 0600); err != nil {
			t.Fatal(err)
		}
		written++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if written != 9900 {
		t.Fatalf("files=%d", written)
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id=$1 OR parent_id=$1", root); got != 10000 {
		t.Fatalf("mixed item=%d want10000", got)
	}
	sourceCostHealthy(t, tm)
	return root
}

func TestSourceMoveAtTenThousandTakesItsTime(t *testing.T) {
	sourceHTTPCost(t, "move")
}

func TestSourceRestoreAtTenThousandTakesItsTime(t *testing.T) {
	sourceHTTPCost(t, "restore")
}

func TestSourceRevertAtTenThousandTakesItsTime(t *testing.T) {
	sourceHTTPCost(t, "revert")
}

func TestSourceForgetHundredThousandTakesItsTime(t *testing.T) {
	sourceHTTPCost(t, "forget")
}
