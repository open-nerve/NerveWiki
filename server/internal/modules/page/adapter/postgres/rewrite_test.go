package postgresadapter_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

type storedRewriteRevision struct {
	ID        uuid.UUID
	Base      *int
	Revision  int
	Content   string
	Hash      []byte
	Bytes     int
	Rewrite   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func readRewriteRevision(t *testing.T, f fixture, changeset, node uuid.UUID) storedRewriteRevision {
	t.Helper()
	if n := f.count(t, "SELECT count(*) FROM page_revisions WHERE changeset_id = $1 AND node_id = $2", changeset, node); n != 1 {
		t.Fatalf("the changeset's versions of the page = %d, want 1", n)
	}
	var r storedRewriteRevision
	if err := f.pool.QueryRow(context.Background(), `SELECT id, base_revision, revision, content, content_hash, byte_size,
		rewrite, created_at, updated_at FROM page_revisions WHERE changeset_id = $1 AND node_id = $2`, changeset, node).
		Scan(&r.ID, &r.Base, &r.Revision, &r.Content, &r.Hash, &r.Bytes, &r.Rewrite, &r.CreatedAt, &r.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	return r
}

// A version is a rewrite only if every write merged into it was one.
// OR and last-write-wins both lose the ordinary write in the false/true
// case. The other columns keep their original upsert and continuity rules.
func TestRevisionsKeepOnlyAllRewriteWrites(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	for _, tt := range []struct {
		name                string
		first, second, want bool
	}{
		{"true_true", true, true, true},
		{"true_false", true, false, false},
		{"false_true", false, true, false},
		{"false_false", false, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := f.page(t, f.eng, nil, tt.name, 0)
			cs := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now())
			firstHash := sha256.Sum256([]byte("一\n"))
			first := app.Revision{ID: uuid.NewV7(), ChangesetID: cs.ID, NodeID: p.ID, Base: new(1), Revision: 2,
				Content: "一\n", Hash: firstHash[:], ByteSize: 4, Rewrite: tt.first, At: now().Add(time.Second)}
			if err := f.s.RecordRevision(ctx, first); err != nil {
				t.Fatal(err)
			}
			got := readRewriteRevision(t, f, cs.ID, p.ID)
			if got.Rewrite != tt.first {
				t.Errorf("first write rewrite = %t, want %t", got.Rewrite, tt.first)
			}
			if got.ID != first.ID || got.Base == nil || *got.Base != 1 || got.Revision != 2 || got.Content != "一\n" ||
				!bytes.Equal(got.Hash, firstHash[:]) || got.Bytes != 4 || !got.CreatedAt.Equal(first.At) || !got.UpdatedAt.Equal(first.At) {
				t.Errorf("first version = %+v, want the first write's fields", got)
			}

			secondHash := sha256.Sum256([]byte("二次改写\n"))
			second := app.Revision{ID: uuid.NewV7(), ChangesetID: cs.ID, NodeID: p.ID, Base: new(2), Revision: 3,
				Content: "二次改写\n", Hash: secondHash[:], ByteSize: 13, Rewrite: tt.second, At: now().Add(2 * time.Second)}
			if err := f.s.RecordRevision(ctx, second); err != nil {
				t.Fatal(err)
			}
			got = readRewriteRevision(t, f, cs.ID, p.ID)
			if got.Rewrite != tt.want {
				t.Errorf("merged rewrite = %t, want %t", got.Rewrite, tt.want)
			}
			if got.ID != first.ID || got.Base == nil || *got.Base != 1 || !got.CreatedAt.Equal(first.At) ||
				got.Revision != 3 || got.Content != "二次改写\n" || !bytes.Equal(got.Hash, secondHash[:]) || got.Bytes != 13 ||
				!got.UpdatedAt.Equal(second.At) {
				t.Errorf("merged version = %+v, want the first id/base/created time and the second content/revision/updated time", got)
			}

			between := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now().Add(3*time.Second))
			thirdHash := sha256.Sum256([]byte("other"))
			third := app.Revision{ID: uuid.NewV7(), ChangesetID: between.ID, NodeID: p.ID, Base: new(3), Revision: 4,
				Content: "other", Hash: thirdHash[:], ByteSize: 5, Rewrite: true, At: now().Add(3 * time.Second)}
			if err := f.s.RecordRevision(ctx, third); err != nil {
				t.Fatal(err)
			}
			other := readRewriteRevision(t, f, between.ID, p.ID)
			fourthHash := sha256.Sum256([]byte("must not merge"))
			crossed := app.Revision{ID: uuid.NewV7(), ChangesetID: cs.ID, NodeID: p.ID, Base: new(4), Revision: 5,
				Content: "must not merge", Hash: fourthHash[:], ByteSize: 14, Rewrite: !tt.want, At: now().Add(4 * time.Second)}
			if err := f.s.RecordRevision(ctx, crossed); err == nil {
				t.Error("the changeset's version went on from another changeset's revision")
			}
			if after := readRewriteRevision(t, f, cs.ID, p.ID); !reflect.DeepEqual(after, got) {
				t.Errorf("refused merge changed the original version: before %+v, after %+v", got, after)
			}
			if after := readRewriteRevision(t, f, between.ID, p.ID); !reflect.DeepEqual(after, other) {
				t.Errorf("refused merge changed the intervening version: before %+v, after %+v", other, after)
			}
		})
	}
}

// Both readers must take rewrite from the version being reverted, even
// when its base has the opposite marker or has been purged. SQL fixtures
// keep this reader test independent of RecordRevision's propagation.
func TestSourceHistoryReadersKeepTheRevertedVersionsRewrite(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	p := f.page(t, f.eng, nil, "Rewrite", 0)
	q := f.page(t, f.eng, nil, "Ordinary", 1)
	r := f.page(t, f.eng, nil, "Purged base", 2)
	base := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now())
	current := f.changeset(t, f.eng, f.alice, domain.ClientWeb, now().Add(time.Second))
	insert := func(cs, node uuid.UUID, from *int, revision int, content string, rewrite bool) {
		t.Helper()
		sum := sha256.Sum256([]byte(content))
		f.exec(t, `INSERT INTO page_revisions (id, changeset_id, node_id, base_revision, revision, content, content_hash,
			byte_size, rewrite, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)`,
			uuid.NewV7(), cs, node, from, revision, content, sum[:], len(content), rewrite, now())
	}
	insert(base.ID, p.ID, nil, 1, "旧\n", false)
	insert(current.ID, p.ID, new(1), 2, "新的正文\n", true)
	insert(base.ID, q.ID, nil, 1, "旧系统正文\n", true)
	insert(current.ID, q.ID, new(1), 2, "人\n", false)
	insert(base.ID, r.ID, nil, 1, "已经清理\n", false)
	insert(current.ID, r.ID, new(1), 2, "仍是系统正文\n", true)
	f.exec(t, "DELETE FROM page_revisions WHERE changeset_id = $1 AND node_id = $2", base.ID, r.ID)

	pHash := sha256.Sum256([]byte("新的正文\n"))
	qHash := sha256.Sum256([]byte("人\n"))
	rHash := sha256.Sum256([]byte("仍是系统正文\n"))
	want := map[uuid.UUID]domain.ChangesetVersion{
		p.ID: {NodeID: p.ID, Base: new(1), BaseBytes: 4, Revision: 2, Hash: pHash[:], Rewrite: true},
		q.ID: {NodeID: q.ID, Base: new(1), BaseBytes: 16, Revision: 2, Hash: qHash[:], Rewrite: false},
		r.ID: {NodeID: r.ID, Base: new(1), BaseBytes: 0, Revision: 2, Hash: rHash[:], Rewrite: true},
	}
	check := func(t *testing.T, got []domain.ChangesetVersion, ids ...uuid.UUID) {
		t.Helper()
		if len(got) != len(ids) {
			t.Fatalf("versions = %+v, want %d requested versions", got, len(ids))
		}
		seen := make(map[uuid.UUID]bool, len(got))
		for _, g := range got {
			if seen[g.NodeID] {
				t.Errorf("page %s returned twice", g.NodeID)
			}
			seen[g.NodeID] = true
			w, ok := want[g.NodeID]
			if !ok {
				t.Errorf("unexpected page %s", g.NodeID)
				continue
			}
			if g.Rewrite != w.Rewrite {
				t.Errorf("page %s rewrite = %t, want reverted version's %t", g.NodeID, g.Rewrite, w.Rewrite)
			}
			if g.Base == nil || *g.Base != 1 || g.BaseBytes != w.BaseBytes || g.Revision != 2 || !bytes.Equal(g.Hash, w.Hash) {
				t.Errorf("page %s version = %+v, want base bytes %d and the reverted revision/hash", g.NodeID, g, w.BaseBytes)
			}
		}
		for _, id := range ids {
			if !seen[id] {
				t.Errorf("requested page %s missing", id)
			}
		}
	}
	t.Run("full", func(t *testing.T) {
		got, err := f.s.ChangesetVersions(ctx, current.ID)
		if err != nil {
			t.Fatal(err)
		}
		check(t, got, p.ID, q.ID, r.ID)
	})
	t.Run("of", func(t *testing.T) {
		got, err := f.s.ChangesetVersionsOf(ctx, current.ID, []uuid.UUID{p.ID, q.ID, r.ID})
		if err != nil {
			t.Fatal(err)
		}
		check(t, got, p.ID, q.ID, r.ID)
	})
	t.Run("of_subset", func(t *testing.T) {
		got, err := f.s.ChangesetVersionsOf(ctx, current.ID, []uuid.UUID{q.ID, uuid.NewV7()})
		if err != nil {
			t.Fatal(err)
		}
		check(t, got, q.ID)
	})
}

// ContentsNow carries the live byte count, not the UTF-8 rune count, and
// keeps empty pages while omitting deleted and missing ones.
func TestSourceContentsNowCarriesBytes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	p := f.page(t, f.eng, nil, "Written", 0)
	empty := f.page(t, f.eng, nil, "Empty", 1)
	deleted := f.page(t, f.eng, nil, "Deleted", 2)
	f.write(t, p.ID, "中文\n", 7, now())
	f.write(t, deleted.ID, "gone", 3, now())
	if err := f.s.DeleteNodes(ctx, deletion([]uuid.UUID{deleted.ID}, f.alice, now())); err != nil {
		t.Fatal(err)
	}
	missing := uuid.NewV7()
	got, err := f.s.ContentsNow(ctx, []uuid.UUID{p.ID, empty.ID, deleted.ID, missing})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("ContentsNow = %+v, want the written and empty live pages", got)
	}
	sum := sha256.Sum256([]byte("中文\n"))
	if g, ok := got[p.ID]; !ok || g.Revision != 7 || !bytes.Equal(g.Hash, sum[:]) || g.Bytes != 7 {
		t.Errorf("live content = %+v, present %t; want revision 7, its hash and 7 UTF-8 bytes", g, ok)
	}
	emptyHash := sha256.Sum256(nil)
	if g, ok := got[empty.ID]; !ok || g.Revision != 1 || !bytes.Equal(g.Hash, emptyHash[:]) || g.Bytes != 0 {
		t.Errorf("empty content = %+v, present %t; want revision 1, the empty hash and 0 bytes", g, ok)
	}
	for _, id := range []uuid.UUID{deleted.ID, missing} {
		if _, ok := got[id]; ok {
			t.Errorf("non-live page %s returned by ContentsNow", id)
		}
	}
}
