package page_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// blobInfos is the attachments' files' metadata read from asset_blobs by
// SQL, as the asset module's AttachmentInfos reads them, in the caller's
// transaction: page does not import asset, and noAttachments knows none.
type blobInfos struct{ pool *pgxpool.Pool }

func (b blobInfos) Of(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]page.AttachmentInfo, error) {
	rows, err := postgres.DB(ctx, b.pool).Query(ctx, "SELECT node_id, mime, byte_size, sha256 FROM asset_blobs WHERE node_id = ANY($1)", ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uuid.UUID]page.AttachmentInfo{}
	for rows.Next() {
		var id uuid.UUID
		var info page.AttachmentInfo
		if err := rows.Scan(&id, &info.MIME, &info.Bytes, &info.SHA256); err != nil {
			return nil, err
		}
		out[id] = info
	}
	return out, rows.Err()
}

// The approval's rebuild is the invariant (M11/P3 design 3.4, M10 design
// 4.8): in a standard notebook, raw's strict live descendants, a page with
// content its content's hash, the zero-byte attachment its file's too;
// raw itself, an empty page, a page deleted through the unit (its hash
// cleared as it was deleted) none; outside raw, a page with an old hash
// cleared. Each hash its own; the answer the number it set; another
// notebook's hashes as they were. An attachment without its file's row is
// an error, the caller's transaction rolled back, nothing changed; outside
// a transaction, an error.
func TestTheSourceRebuildIsTheInvariant(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	sum := func(s string) []byte {
		h := sha256.Sum256([]byte(s))
		return h[:]
	}
	ops := uuid.NewV7()
	f.exec(t, "INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Ops', $3, $3, $4, $4)",
		ops, f.acme, f.alice, testNow())
	ids := map[string]uuid.UUID{}
	order := 0
	node := func(nb uuid.UUID, name, kind string, parent *uuid.UUID, hash []byte) uuid.UUID {
		t.Helper()
		id := uuid.NewV7()
		order++
		f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
				created_at, updated_at, source_hash)
			VALUES ($1, $2, $3, $4, $5, lower($5), $6, $7, $7, $8, $8, $9)`, id, nb, parent, kind, name, order, f.alice, testNow(), hash)
		ids[name] = id
		return id
	}
	pageOf := func(nb uuid.UUID, name, content string, parent *uuid.UUID, hash []byte) uuid.UUID {
		t.Helper()
		id := node(nb, name, "page", parent, hash)
		f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
			VALUES ($1, $2, 1, sha256(convert_to($2, 'UTF8')), octet_length($2), $3, $4)`, id, content, f.alice, testNow())
		return id
	}
	assetOf := func(nb uuid.UUID, name, content string, parent *uuid.UUID, hash []byte) uuid.UUID {
		t.Helper()
		id := node(nb, name, "asset", parent, hash)
		f.exec(t, `INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at)
			VALUES ($1, $2, $3, 'application/octet-stream', $4, $5, $6, $7)`, uuid.NewV7(), id, nb, len(content), sum(content), f.alice, testNow())
		return id
	}
	raw := pageOf(f.eng, "raw", "The raw sources.", nil, nil)
	pageOf(f.eng, "Alpha", "Alpha's body.", &raw, nil)
	empty := pageOf(f.eng, "Empty", "", &raw, nil)
	pageOf(f.eng, "Deep", "Two levels down.", &empty, nil)
	parent := pageOf(f.eng, "Parent", "A body over a child.", &raw, nil)
	pageOf(f.eng, "Kid", "", &parent, nil)
	assetOf(f.eng, "file.bin", "a file's bytes", &raw, nil)
	assetOf(f.eng, "zero.bin", "", &raw, nil)
	gone := pageOf(f.eng, "Gone", "Deleted through its unit.", &raw, nil)
	pageOf(f.eng, "Outside", "Not a source.", nil, sum("an old hash"))
	opsRaw := pageOf(ops, "OpsRaw", "", nil, nil)
	pageOf(ops, "OpsSource", "Another notebook's source.", &opsRaw, sum("Another notebook's source."))
	assetOf(ops, "ops.bin", "another file", &opsRaw, sum("another file"))
	if rec := f.serve(t, "session", http.MethodDelete, "/api/v0/nodes/"+gone.String(), "", nil, nil, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete Gone through its unit = %d %s", rec.Code, rec.Body)
	}
	// hashes are each node's source hash, by name.
	hashes := func() map[string]string {
		t.Helper()
		rows, err := f.pool.Query(ctx, "SELECT name, coalesce(encode(source_hash, 'hex'), '-') FROM nodes")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]string{}
		for rows.Next() {
			var name, hash string
			if err := rows.Scan(&name, &hash); err != nil {
				t.Fatal(err)
			}
			out[name] = hash
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	hexOf := func(s string) string { return hex.EncodeToString(sum(s)) }
	before := map[string]string{"Notes": "-", "raw": "-", "Alpha": "-", "Empty": "-", "Deep": "-", "Parent": "-", "Kid": "-",
		"file.bin": "-", "zero.bin": "-", "Gone": "-", "Outside": hexOf("an old hash"), "OpsRaw": "-",
		"OpsSource": hexOf("Another notebook's source."), "ops.bin": hexOf("another file")}
	if got := hashes(); !maps.Equal(got, before) {
		t.Fatalf("the hashes before = %v, want %v", got, before)
	}
	tx := postgres.NewTxManager(f.pool, 5*time.Second)
	rebuild := page.NewSourceRebuild(f.pool, blobInfos{f.pool})
	if _, err := rebuild.Rebuild(ctx, f.eng, raw); err == nil {
		t.Error("Rebuild outside a transaction = nil, want an error")
	}

	// The file of file.bin gone from asset_blobs: an error, rolled back.
	var blob uuid.UUID
	if err := f.pool.QueryRow(ctx, "DELETE FROM asset_blobs WHERE node_id = $1 RETURNING id", ids["file.bin"]).Scan(&blob); err != nil {
		t.Fatal(err)
	}
	err := tx.WithinTx(ctx, func(ctx context.Context) error {
		_, err := rebuild.Rebuild(ctx, f.eng, raw)
		return err
	})
	if err == nil {
		t.Error("Rebuild with an attachment's file gone = nil, want an error")
	}
	if got := hashes(); !maps.Equal(got, before) {
		t.Errorf("the hashes after a rebuild rolled back = %v, want %v", got, before)
	}
	f.exec(t, `INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at)
		VALUES ($1, $2, $3, 'application/octet-stream', $4, $5, $6, $7)`, blob, ids["file.bin"], f.eng, len("a file's bytes"),
		sum("a file's bytes"), f.alice, testNow())

	var n int
	if err := tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		n, err = rebuild.Rebuild(ctx, f.eng, raw)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Notes": "-", "raw": "-", "Alpha": hexOf("Alpha's body."), "Empty": "-", "Deep": hexOf("Two levels down."),
		"Parent": hexOf("A body over a child."), "Kid": "-", "file.bin": hexOf("a file's bytes"), "zero.bin": hexOf(""), "Gone": "-",
		"Outside": "-", "OpsRaw": "-", "OpsSource": hexOf("Another notebook's source."), "ops.bin": hexOf("another file")}
	if got := hashes(); !maps.Equal(got, want) {
		t.Errorf("the hashes rebuilt = %v\nwant %v", got, want)
	}
	if n != 5 {
		t.Errorf("Rebuild set %d, want 5: Alpha, Deep, Parent, file.bin and zero.bin", n)
	}
}

// A rebuild without the attachments' infos is a fault of its wiring, as
// the module's (TestTheModuleRefusesNoAttachmentInfos).
func TestTheSourceRebuildRefusesNoAttachmentInfos(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewSourceRebuild(nil attachments) did not panic")
		}
	}()
	page.NewSourceRebuild(nil, nil)
}
