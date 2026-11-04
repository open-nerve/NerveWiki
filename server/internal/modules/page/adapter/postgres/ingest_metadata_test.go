package postgresadapter_test

import (
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestIngestMetadata(t *testing.T) {
	t.Run("raw_eof_zero_id", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "raw", 0)
		f.exec(t, `INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at,deleted_at)
            SELECT ('00000000-0000-0000-0000-'||lpad(to_hex(i),12,'0'))::uuid,$1,$2,
                CASE WHEN i%5=0 THEN 'asset' ELSE 'page' END,'n'||i,'n'||i,i,$3,$3,now(),now(),
                CASE WHEN i%3=1 THEN now() ELSE NULL END FROM generate_series(0,510) i`, f.eng, root.ID, f.alice)
		f.exec(t, `INSERT INTO page_contents(node_id,content,revision,content_hash,byte_size,updated_by_id,updated_at,deleted_at)
            SELECT id,'',1,sha256(''),0,created_by_id,now(),deleted_at FROM nodes WHERE notebook_id=$1 AND id<>$2 AND kind='page'`, f.eng, root.ID)
		first, err := f.s.Heads(t.Context(), f.eng, root.ID, nil, 512)
		if err != nil || len(first.Rows) != 512 || first.Done || first.Rows[0].ID != uuid.Nil() || first.Last == nil || *first.Last != root.ID {
			t.Fatalf("first raw batch=%+v, err%v", first, err)
		}
		second, err := f.s.Heads(t.Context(), f.eng, root.ID, first.Last, 512)
		if err != nil || len(second.Rows) != 0 || !second.Done || second.Last != nil {
			t.Fatalf("explicit EOF=%+v, err%v", second, err)
		}
		if first.Rows[0].Kind != domain.KindAsset || !first.Rows[0].Live || !first.Rows[0].InSource {
			t.Fatalf("legal zero-ID source asset=%+v", first.Rows[0])
		}
		gone := 0
		for _, h := range first.Rows {
			if !h.Live {
				gone++
			}
		}
		if gone != 170 {
			t.Fatalf("raw scan omitted trash: %d, want170", gone)
		}
	})
	t.Run("now_absence_and_current", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "raw", 0)
		s := f.page(t, f.eng, &root.ID, "S", 0)
		other := f.page(t, f.ops, nil, "Other", 0)
		ids := []uuid.UUID{s.ID, other.ID, uuid.Nil()}
		slices.SortFunc(ids, uuid.UUID.Compare)
		rows, err := f.s.Now(t.Context(), f.eng, root.ID, ids)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 3 {
			t.Fatalf("point rows %v", rows)
		}
		for _, r := range rows {
			if r.ID == s.ID {
				if !r.Exists || !r.Live || !r.InSource || !r.HasContent || r.Revision != 1 {
					t.Fatalf("source %+v", r)
				}
			} else if r.Exists || r.Live {
				t.Fatalf("absent/foreign %+v", r)
			}
		}
	})
	t.Run("missing_content", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "raw", 0)
		p := f.page(t, f.eng, nil, "P", 1)
		f.exec(t, "DELETE FROM page_contents WHERE node_id=$1", p.ID)
		if _, err := f.s.Now(t.Context(), f.eng, root.ID, []uuid.UUID{p.ID}); err == nil {
			t.Fatal("live page missing content vanished from Now")
		}
		if _, err := f.s.Heads(t.Context(), f.eng, root.ID, nil, 512); err == nil {
			t.Fatal("live page missing content vanished from raw coverage")
		}
	})
	t.Run("legal_asset_depth_and_invalid_ancestry", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "raw", 0)
		parent := root.ID
		for range 9 {
			p := f.page(t, f.eng, &parent, "Deep", 0)
			parent = p.ID
		}
		a := uuid.NewV7()
		f.exec(t, `INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at)
            VALUES($1,$2,$3,'asset','a.txt','a.txt',0,$4,$4,now(),now())`, a, f.eng, parent, f.alice)
		rows, err := f.s.Now(t.Context(), f.eng, root.ID, []uuid.UUID{a})
		if err != nil || len(rows) != 1 || !rows[0].InSource {
			t.Fatalf("legal depth11 asset=%+v, %v", rows, err)
		}
		bad := f.page(t, f.eng, &parent, "Too deep", 1)
		if _, err := f.s.Now(t.Context(), f.eng, root.ID, []uuid.UUID{bad.ID}); err == nil {
			t.Fatal("depth11 page accepted")
		}
	})
}
