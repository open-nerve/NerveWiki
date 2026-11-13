package page_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
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

// randomNode is a node of TestTheSourceRebuildOnRandomTrees' model: a page
// with its content, or an attachment with its file's bytes, as generated.
type randomNode struct {
	id       uuid.UUID
	name     string
	asset    bool
	content  string
	parent   *randomNode
	children []*randomNode
	// below is how far under raw the node is, 1 for raw's children; 0
	// outside raw, raw itself included.
	below   int
	old     []byte // the hash SQL wrote it, none when nil
	deleted bool
}

// source is whether the rebuild hashes n, as the model has it: a strict
// live descendant of raw, a page with content or an attachment, a
// zero-byte one too.
func (n *randomNode) source() bool {
	return !n.deleted && n.below > 0 && (n.asset || n.content != "")
}

// The approval's rebuild on random trees (M11 design section 3, item 6;
// decision 303): a property test beside the fixed shape of
// TestTheSourceRebuildIsTheInvariant, its expectation the generator's own
// model, not the statements under test. Each of 40 shapes, its seed fixed
// and named on a failure, is a standard notebook: raw at its root, under
// it a tree a few levels deep of pages with content, empty pages, pages
// with content and children, pages that are only folders, and attachments,
// zero-byte ones among them; some nodes deleted through the unit, a
// deleted page's subtree with it; about half the nodes, under raw and
// outside it, raw itself among them, given an old hash by SQL, which the
// unit's deletion clears; and another notebook whose sources have hashes.
// Rebuilt in one transaction, one SQL read of every node's
// (id, source_hash) equals the model's: under raw, each strict live
// descendant that is a page with content its content's hash, an old one
// written over, each attachment its file's SHA-256; this notebook's other
// nodes none, the empty pages and folders under raw with an old hash
// cleared; every other node, the other notebooks' and the earlier
// shapes', as it was. The rebuild answers how many it set. The shapes the
// mutants of the rebuild show in are counted, each in enough of them: an
// empty page under raw, a source two levels down or more, a source
// deleted through the unit, a node outside raw with an old hash, a node
// under raw that is no source with an old hash, and one that is raw's
// child (M11 closeout, review A-M2: without them, a ForgetSources that
// spares raw's children passed).
func TestTheSourceRebuildOnRandomTrees(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	router := f.router(t, nil, nil, nil)
	tx := postgres.NewTxManager(f.pool, 5*time.Second)
	rebuild := page.NewSourceRebuild(f.pool, blobInfos{f.pool})
	hexOf := func(s string) string {
		h := sha256.Sum256([]byte(s))
		return hex.EncodeToString(h[:])
	}
	// want is every node's source hash, in hex, "-" for none; names, its
	// name, for the failures.
	want := map[uuid.UUID]string{f.notes: "-"}
	names := map[uuid.UUID]string{f.notes: "Notes"}
	hashes := func(t *testing.T) map[uuid.UUID]string {
		t.Helper()
		rows, err := f.pool.Query(ctx, "SELECT id, coalesce(encode(source_hash, 'hex'), '-') FROM nodes")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[uuid.UUID]string{}
		for rows.Next() {
			var id uuid.UUID
			var hash string
			if err := rows.Scan(&id, &hash); err != nil {
				t.Fatal(err)
			}
			out[id] = hash
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	// differ is how got differs from want, by name, sorted.
	differ := func(got, want map[uuid.UUID]string) []string {
		var out []string
		for id, w := range want {
			if g, ok := got[id]; !ok || g != w {
				out = append(out, fmt.Sprintf("%s: %q, want %q", names[id], g, w))
			}
		}
		for id, g := range got {
			if _, ok := want[id]; !ok {
				out = append(out, fmt.Sprintf("%s (%s): %q, not generated", names[id], id, g))
			}
		}
		slices.Sort(out)
		return out
	}
	// seen counts the shapes that hold each kind of node a mutant shows in.
	seen := map[string]int{}
	const shapes = 40
	for seed := uint64(1); seed <= shapes; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0x50ce))
		b := &pgx.Batch{}
		order := 0
		var all []*randomNode
		// add queues n's rows in nb, its hash old.
		add := func(nb uuid.UUID, n *randomNode) {
			n.id = uuid.NewV7()
			order++
			kind := "page"
			if n.asset {
				kind = "asset"
			}
			var parent *uuid.UUID
			if n.parent != nil {
				parent = &n.parent.id
				n.parent.children = append(n.parent.children, n)
			}
			b.Queue(`INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
					created_at, updated_at, source_hash)
				VALUES ($1, $2, $3, $4, $5, lower($5), $6, $7, $7, $8, $8, $9)`, n.id, nb, parent, kind, n.name, order, f.alice, testNow(), n.old)
			if n.asset {
				h := sha256.Sum256([]byte(n.content))
				b.Queue(`INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at)
					VALUES ($1, $2, $3, 'application/octet-stream', $4, $5, $6, $7)`, uuid.NewV7(), n.id, nb, len(n.content), h[:], f.alice, testNow())
			} else {
				b.Queue(`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
					VALUES ($1, $2, 1, sha256(convert_to($2, 'UTF8')), octet_length($2), $3, $4)`, n.id, n.content, f.alice, testNow())
			}
			names[n.id] = fmt.Sprintf("seed %d %s", seed, n.name)
		}
		// oldOf is an old hash, none half the time.
		oldOf := func() []byte {
			if rng.IntN(2) == 0 {
				return nil
			}
			h := sha256.Sum256(fmt.Appendf(nil, "an old hash %d", rng.Uint64()))
			return h[:]
		}
		// grow adds up to most children under parent in nb, below says how
		// far under raw they are (0 outside it), down to depth more levels.
		var grow func(nb uuid.UUID, parent *randomNode, below, most, depth int, old bool)
		grow = func(nb uuid.UUID, parent *randomNode, below, most, depth int, old bool) {
			for range rng.IntN(most + 1) {
				i := len(all)
				n := &randomNode{parent: parent, below: below, name: fmt.Sprintf("p%d", i)}
				switch rng.IntN(6) {
				case 0: // an attachment, a zero-byte one half the time
					n.asset, n.name = true, fmt.Sprintf("a%d.bin", i)
					if rng.IntN(2) > 0 {
						n.content = fmt.Sprintf("file %d of seed %d", i, seed)
					}
				case 1, 2: // an empty page
				default:
					n.content = fmt.Sprintf("Page %d of seed %d.", i, seed)
				}
				if old {
					n.old = oldOf()
				}
				all = append(all, n)
				add(nb, n)
				if !n.asset && depth > 1 && rng.IntN(2) == 0 {
					next := below
					if below > 0 {
						next++
					}
					grow(nb, n, next, 3, depth-1, old)
				}
			}
		}
		nb, other := uuid.NewV7(), uuid.NewV7()
		for _, n := range []struct {
			id   uuid.UUID
			name string
		}{{nb, fmt.Sprintf("Shape %d", seed)}, {other, fmt.Sprintf("Other %d", seed)}} {
			b.Queue("INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $4, $5, $5)",
				n.id, f.acme, n.name, f.alice, testNow())
		}
		raw := &randomNode{name: "raw", old: oldOf()}
		if rng.IntN(2) == 0 {
			raw.content = fmt.Sprintf("The sources of seed %d.", seed)
		}
		all = append(all, raw)
		add(nb, raw)
		grow(nb, raw, 1, 10, 4, true)
		grow(nb, nil, 0, 4, 3, true)
		ours := len(all)
		// The other notebook: its own raw, its sources hashed, as an LLM
		// Wiki's are, or not.
		otherRaw := &randomNode{name: "raw"}
		all = append(all, otherRaw)
		add(other, otherRaw)
		grow(other, otherRaw, 1, 4, 3, true)
		if err := f.pool.SendBatch(ctx, b).Close(); err != nil {
			t.Fatalf("seed %d: the shape's rows: %v", seed, err)
		}
		// Some of this notebook's nodes deleted through the unit, which
		// deletes each with its subtree and clears its hash, raw kept.
		var deletedSome func(n *randomNode)
		deletedSome = func(n *randomNode) {
			n.deleted = true
			for _, c := range n.children {
				deletedSome(c)
			}
		}
		for _, n := range all[:ours] {
			if n == raw || n.deleted || rng.IntN(6) > 0 {
				continue
			}
			if rec := f.request(router, "session", http.MethodDelete, "/api/v0/nodes/"+n.id.String(), ""); rec.Code != http.StatusNoContent {
				t.Fatalf("seed %d: delete %s through its unit = %d %s", seed, n.name, rec.Code, rec.Body)
			}
			deletedSome(n)
		}
		// The model: before, each node its old hash, but those deleted;
		// after, this notebook's sources their hashes, its other nodes none.
		before, after := maps.Clone(want), maps.Clone(want)
		sources := 0
		kinds := map[string]bool{}
		for i, n := range all {
			before[n.id], after[n.id] = "-", "-"
			if n.old != nil && !n.deleted {
				before[n.id], after[n.id] = hex.EncodeToString(n.old), hex.EncodeToString(n.old)
			}
			if i >= ours {
				continue
			}
			after[n.id] = "-"
			if n.source() {
				after[n.id] = hexOf(n.content)
				sources++
			}
			switch {
			case n.below > 0 && n.deleted && (n.asset || n.content != ""):
				kinds["a source deleted through the unit"] = true
			case n.deleted:
			case n.below > 0 && !n.asset && n.content == "" && len(n.children) == 0:
				kinds["an empty page under raw"] = true
			case n.below > 0 && !n.asset && n.content == "":
				kinds["a folder under raw"] = true
			case n.below > 0 && !n.asset && len(n.children) > 0:
				kinds["a page with content and children under raw"] = true
			case n.below > 0 && n.asset && n.content == "":
				kinds["a zero-byte attachment under raw"] = true
			case n != raw && n.below == 0 && n.old != nil:
				kinds["a node outside raw with an old hash"] = true
			}
			if n.source() && n.below > 1 {
				kinds["a source two levels down or more"] = true
			}
			if !n.deleted && n.below > 0 && !n.source() && n.old != nil {
				kinds["a node under raw that is no source with an old hash"] = true
				if n.below == 1 {
					kinds["a child of raw that is no source with an old hash"] = true
				}
			}
		}
		for k := range kinds {
			seen[k]++
		}
		if d := differ(hashes(t), before); len(d) > 0 {
			t.Fatalf("seed %d: the hashes before the rebuild differ from the shape's:\n%s", seed, strings.Join(d, "\n"))
		}

		var set int
		if err := tx.WithinTx(ctx, func(ctx context.Context) error {
			var err error
			set, err = rebuild.Rebuild(ctx, nb, raw.id)
			return err
		}); err != nil {
			t.Fatalf("seed %d: Rebuild = %v", seed, err)
		}
		if d := differ(hashes(t), after); len(d) > 0 {
			t.Fatalf("seed %d: the hashes rebuilt differ from the model's:\n%s", seed, strings.Join(d, "\n"))
		}
		if set != sources {
			t.Fatalf("seed %d: Rebuild set %d, want %d", seed, set, sources)
		}
		want = after
	}
	for _, k := range []string{"an empty page under raw", "a folder under raw", "a page with content and children under raw",
		"a zero-byte attachment under raw", "a source two levels down or more", "a source deleted through the unit",
		"a node outside raw with an old hash", "a node under raw that is no source with an old hash",
		"a child of raw that is no source with an old hash"} {
		if seen[k] < 10 {
			t.Errorf("%d of %d shapes hold %s, want 10 at least", seen[k], shapes, k)
		}
	}
	t.Logf("shapes holding each kind: %v", seen)
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
