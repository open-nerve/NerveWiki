package bootstrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
)

const sourceHashGap = "nodes whose source hash differs from their live source membership and metadata"

// Both directions matter: an eligible NULL is as wrong as a stale non-NULL.
// Metadata missing from an eligible source is a violation even if its hash is
// NULL. This observer reads the complete database, not the indexed subset.
const sourceHashViolationSQL = `WITH RECURSIVE area(notebook_id, root_id, id) AS (
	SELECT b.id, r.id, r.id FROM notebooks b JOIN nodes r ON r.id = b.source_root_id
		AND r.notebook_id = b.id AND r.kind = 'page' AND r.deleted_at IS NULL
	WHERE b.mode = 'llm_wiki' AND b.deleted_at IS NULL
	UNION
	SELECT a.notebook_id, a.root_id, n.id FROM area a JOIN nodes n ON n.parent_id = a.id
		AND n.notebook_id = a.notebook_id AND n.deleted_at IS NULL
), expected AS (
	SELECT n.id, n.source_hash, a.id IS NOT NULL AS inside, n.kind,
		c.node_id AS page_id, c.content_hash, c.byte_size, b.node_id AS asset_id, b.sha256,
		CASE WHEN a.id IS NOT NULL AND n.kind = 'page' AND c.byte_size > 0 THEN c.content_hash
			WHEN a.id IS NOT NULL AND n.kind = 'asset' THEN b.sha256 ELSE NULL::bytea END AS hash
	FROM nodes n LEFT JOIN area a ON a.id = n.id AND a.id <> a.root_id
	LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL
	LEFT JOIN asset_blobs b ON b.node_id = n.id AND b.notebook_id = n.notebook_id AND b.deleted_at IS NULL
)
SELECT count(*) FROM expected WHERE source_hash IS DISTINCT FROM hash OR
	(inside AND ((kind = 'page' AND (page_id IS NULL OR octet_length(content_hash) <> 32))
		OR (kind = 'asset' AND (asset_id IS NULL OR octet_length(sha256) <> 32))))`

// Rebuild from raw parents and actual body bytes, independently of SourceFacts,
// source metadata queries, stored content_hash, and the recursive SQL observer.
func sourceHashRebuildMismatches(t *testing.T, ctx context.Context, pool *pgxpool.Pool) []string {
	t.Helper()
	type notebook struct {
		root *uuid.UUID
		live bool
	}
	books := map[uuid.UUID]notebook{}
	rows, err := pool.Query(ctx, "SELECT id, source_root_id, mode = 'llm_wiki' AND deleted_at IS NULL FROM notebooks")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id uuid.UUID
		var b notebook
		if err := rows.Scan(&id, &b.root, &b.live); err != nil {
			t.Fatal(err)
		}
		books[id] = b
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	type node struct {
		id, nb uuid.UUID
		parent *uuid.UUID
		kind   string
		live   bool
		hash   []byte
		body   *string
		asset  []byte
	}
	nodes := map[uuid.UUID]node{}
	rows, err = pool.Query(ctx, `SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.deleted_at IS NULL,
		n.source_hash, c.content, a.sha256 FROM nodes n
		LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL
		LEFT JOIN asset_blobs a ON a.node_id = n.id AND a.notebook_id = n.notebook_id AND a.deleted_at IS NULL`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var n node
		if err := rows.Scan(&n.id, &n.nb, &n.parent, &n.kind, &n.live, &n.hash, &n.body, &n.asset); err != nil {
			t.Fatal(err)
		}
		nodes[n.id] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	var found []string
	for _, n := range nodes {
		b := books[n.nb]
		inside := false
		if n.live && b.live && b.root != nil && n.id != *b.root {
			seen := map[uuid.UUID]bool{n.id: true}
			for p := n.parent; p != nil && !seen[*p]; {
				seen[*p] = true
				ancestor, ok := nodes[*p]
				if !ok || !ancestor.live || ancestor.nb != n.nb || ancestor.kind != "page" {
					break
				}
				if ancestor.id == *b.root {
					inside = true
					break
				}
				p = ancestor.parent
			}
		}
		var want []byte
		if inside {
			switch n.kind {
			case "page":
				if n.body == nil {
					found = append(found, fmt.Sprintf("source page %s lacks live body", n.id))
					continue
				}
				if len(*n.body) > 0 {
					h := sha256.Sum256([]byte(*n.body))
					want = h[:]
				}
			case "asset":
				if len(n.asset) != sha256.Size {
					found = append(found, fmt.Sprintf("source asset %s lacks valid SHA", n.id))
					continue
				}
				want = n.asset
			}
		}
		if !bytes.Equal(n.hash, want) {
			found = append(found, fmt.Sprintf("node %s source hash %x, rebuilt %x", n.id, n.hash, want))
		}
	}
	return found
}

func assertSourceHashesMatchRebuild(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, mismatch := range sourceHashRebuildMismatches(t, ctx, pool) {
		t.Error(mismatch)
	}
}

func TestSourceHashObserversCatchBothDirections(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	wiki := tm.llmWiki(t, "Source observer")
	source := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Body", "原文\r\n")
	empty := tm.createPage(t, "alice", wiki.id, wiki.raw, "Empty")
	outside := tm.createPageWith(t, "alice", wiki.id, "", "Outside", "elsewhere")
	asset := tm.upload(t, "alice", wiki.id, wiki.raw, "source.txt", "file bytes")
	zero := tm.upload(t, "alice", wiki.id, wiki.raw, "empty.bin", "")
	trash := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Trash", "trash")
	tm.send(t, nodeDeletion("alice", trash), http.StatusNoContent)
	other := tm.llmWiki(t, "Other source observer")
	tm.createPageWith(t, "alice", other.id, other.raw, "Other body", "原文\r\n")
	standard := tm.createNotebook(t, "alice", "Standard observer")
	plain := tm.createPageWith(t, "alice", standard, "", "Plain", "ordinary")

	// The healthy baseline is entirely written through the real application.
	// Positive counts and hashes prevent agreement between two empty sets.
	exec := func(t *testing.T, sql string, args ...any) {
		t.Helper()
		if _, err := tm.pool.Exec(t.Context(), sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	checkHealthy := func(t *testing.T) {
		t.Helper()
		if got := count(t, tm.pool, sourceHashViolationSQL); got != 0 {
			t.Fatalf("healthy fixture has %d source hash violations", got)
		}
		assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
		checkPages(t, tm.pool)
		checkAssets(t, tm.pool, tm.storage)
	}
	var pages, assets, zeroAssets int
	if err := tm.pool.QueryRow(t.Context(), `SELECT
		count(*) FILTER (WHERE n.kind = 'page'), count(*) FILTER (WHERE n.kind = 'asset'),
		count(*) FILTER (WHERE n.kind = 'asset' AND b.byte_size = 0)
		FROM nodes n LEFT JOIN asset_blobs b ON b.node_id = n.id
		WHERE n.source_hash IS NOT NULL AND n.deleted_at IS NULL`).Scan(&pages, &assets, &zeroAssets); err != nil {
		t.Fatal(err)
	}
	if pages != 2 || assets != 2 || zeroAssets != 1 {
		t.Fatalf("source fixtures have %d pages, %d assets, %d zero-byte assets; want 2, 2, 1", pages, assets, zeroAssets)
	}
	checkHealthy(t)

	// Each row changes one persisted field, then restores exactly that field.
	// Multiple affected descendants are expected when notebook/root membership
	// changes. Other invariant sentences may also report the deliberate damage.
	pageHash, assetHash, zeroHash := sha256.Sum256([]byte("原文\r\n")), sha256.Sum256([]byte("file bytes")), sha256.Sum256(nil)
	wrong := sha256.Sum256([]byte("wrong"))
	checkDamaged := func(t *testing.T, want int) {
		t.Helper()
		if got := count(t, tm.pool, sourceHashViolationSQL); got != want {
			t.Errorf("SQL source violations = %d, want exactly %d", got, want)
		}
		if got := sourceHashRebuildMismatches(t, t.Context(), tm.pool); len(got) != want {
			t.Errorf("independent rebuild = %v, want exactly %d mismatches", got, want)
		}
		if got := pageViolations(t, tm.pool); !slices.Contains(got, fmt.Sprintf("%d %s, want none", want, sourceHashGap)) {
			t.Errorf("complete page invariant missed the %d source violations: %v", want, got)
		}
	}
	for _, tt := range []struct {
		name, id     string
		hash, before []byte
	}{
		{"missing page hash", source, nil, pageHash[:]},
		{"wrong page hash", source, wrong[:], pageHash[:]},
		{"missing asset hash", asset.ID, nil, assetHash[:]},
		{"wrong asset hash", asset.ID, wrong[:], assetHash[:]},
		{"missing zero-byte asset hash", zero.ID, nil, zeroHash[:]},
		{"outside", outside, wrong[:], nil},
		{"empty page", empty, zeroHash[:], nil},
		{"source root", wiki.raw, wrong[:], nil},
		{"standard notebook", plain, wrong[:], nil},
		{"deleted source", trash, wrong[:], nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exec(t, "UPDATE nodes SET source_hash = $2 WHERE id = $1", tt.id, tt.hash)
			defer func() {
				exec(t, "UPDATE nodes SET source_hash = $2 WHERE id = $1", tt.id, tt.before)
				checkHealthy(t)
			}()
			checkDamaged(t, 1)
		})
	}
	for _, tt := range []struct {
		name, change, restore string
		args, restoreArgs     []any
	}{
		{"notebook changed to standard", "UPDATE notebooks SET mode = 'standard' WHERE id = $1",
			"UPDATE notebooks SET mode = 'llm_wiki' WHERE id = $1", []any{wiki.id}, []any{wiki.id}},
		{"foreign source root", "UPDATE notebooks SET source_root_id = $2 WHERE id = $1",
			"UPDATE notebooks SET source_root_id = $2 WHERE id = $1", []any{wiki.id, other.raw}, []any{wiki.id, wiki.raw}},
		{"deleted notebook", "UPDATE notebooks SET deleted_at = now() WHERE id = $1",
			"UPDATE notebooks SET deleted_at = NULL WHERE id = $1", []any{wiki.id}, []any{wiki.id}},
		{"deleted source root", "UPDATE nodes SET deleted_at = now() WHERE id = $1",
			"UPDATE nodes SET deleted_at = NULL WHERE id = $1", []any{wiki.raw}, []any{wiki.raw}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exec(t, tt.change, tt.args...)
			defer func() {
				exec(t, tt.restore, tt.restoreArgs...)
				checkHealthy(t)
			}()
			checkDamaged(t, 3)
		})
	}
	// Delete exactly one metadata row. Restore the complete original row,
	// including times, revision and file identity, before the next case.
	for _, tt := range []struct{ name, table, id string }{
		{"missing page metadata", "page_contents", source},
		{"missing empty page metadata", "page_contents", empty},
		{"missing asset metadata", "asset_blobs", asset.ID},
		{"missing zero-byte asset metadata", "asset_blobs", zero.ID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var original []byte
			if err := tm.pool.QueryRow(t.Context(), "DELETE FROM "+tt.table+" WHERE node_id = $1 RETURNING to_jsonb("+tt.table+")", tt.id).
				Scan(&original); err != nil {
				t.Fatal(err)
			}
			defer func() {
				exec(t, "INSERT INTO "+tt.table+" SELECT * FROM jsonb_populate_record(NULL::"+tt.table+", $1::jsonb)", string(original))
				checkHealthy(t)
			}()
			checkDamaged(t, 1)
		})
	}
	// A stored content hash changed together with its derived hash fools the SQL
	// comparison, but cannot fool the byte-based rebuild.
	exec(t, "UPDATE page_contents SET content_hash = $2 WHERE node_id = $1", source, wrong[:])
	exec(t, "UPDATE nodes SET source_hash = $2 WHERE id = $1", source, wrong[:])
	defer func() {
		exec(t, "UPDATE page_contents SET content_hash = $2 WHERE node_id = $1", source, pageHash[:])
		exec(t, "UPDATE nodes SET source_hash = $2 WHERE id = $1", source, pageHash[:])
		checkHealthy(t)
	}()
	if got := count(t, tm.pool, sourceHashViolationSQL); got != 0 {
		t.Errorf("metadata-only SQL has %d violations, want the byte oracle to detect this corruption", got)
	}
	if got := sourceHashRebuildMismatches(t, t.Context(), tm.pool); len(got) != 1 {
		t.Errorf("body-independent corruption check = %v", got)
	}
}
