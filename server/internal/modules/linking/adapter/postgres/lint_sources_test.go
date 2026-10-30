package postgresadapter_test

import (
	"encoding/binary"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func lintID(n uint64) uuid.UUID { var id uuid.UUID; binary.BigEndian.PutUint64(id[8:], n); return id }

func TestLintSources(t *testing.T) {
	t.Run("first-key-and-json-items-data", func(t *testing.T) {
		f := newFixture(t)
		r := postgresadapter.NewLintReads(f.pool, 5*1024*1024)
		a, b, old, invalid, foreign := lintID(1), lintID(2), lintID(3), lintID(4), lintID(5)
		for _, p := range []app.Page{{ID: a, NotebookID: f.eng, Revision: 1}, {ID: b, NotebookID: f.eng, Revision: 1}, {ID: old, NotebookID: f.eng, Revision: 1}, {ID: invalid, NotebookID: f.eng, Revision: 1}, {ID: foreign, NotebookID: f.ops, Revision: 1}} {
			f.replace(t, p, domain.Facts{FrontmatterValid: p.ID != invalid, Properties: []domain.Property{{Key: "ſources", Value: []byte(`"wrong"`)}, {Key: "SoUrCeS", Value: []byte(`["[[x]]",{"0":"[[object]]"},["[[nested]]"],3]`)}, {Key: "sources", Value: []byte(`"later"`)}}})
		}
		lintExec(t, f, `UPDATE indexed_pages SET extractor=1 WHERE node_id=$1`, old)
		heads, err := r.SourcePropertyHeads(t.Context(), f.eng, []uuid.UUID{a, b, old, invalid, foreign})
		expectedJSON := `["[[x]]", {"0": "[[object]]"}, ["[[nested]]"], 3]`
		if err != nil || len(heads) != 2 || heads[0].PageID != a || heads[0].Position != 1 || heads[0].ValueBytes != int64(len(expectedJSON)) || heads[1].PageID != b {
			t.Fatalf("first ASCII sources heads: %+v, %v", heads, err)
		}
		values, err := r.SourceProperties(t.Context(), f.eng, []app.SourcePropertyPos{heads[0].SourcePropertyPos, heads[1].SourcePropertyPos})
		if err != nil || len(values) != 2 || values[0].Key != "SoUrCeS" || string(values[0].JSON) != expectedJSON {
			t.Fatalf("exact JSON: %+v, %v", values, err)
		}
	})
	t.Run("short-heads-long-nonslot-key", func(t *testing.T) {
		f := newFixture(t)
		r := postgresadapter.NewLintReads(f.pool, 5*1024*1024)
		p := uuid.UUID{}
		f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, originFacts(t, "---\nSources: ['[[a]]','[[b]]']\nSources.0: '[[outside]]'\nSources.00: '[[leading]]'\nSources.2: '[[past]]'\nSources.1.deep: '[[deep]]'\n---\n[[body]]"))
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,property_key,target,target_key,ambiguous) VALUES($1,100000,100001,$2,'wikilink',$3,'long','long',false)`, p, f.eng, "Sources."+strings.Repeat("x", 1024*1024))
		specs := []app.SourceSpec{{PageID: p, Key: "Sources", Array: true, Items: 2}}
		first, err := r.SourceSlotHeads(t.Context(), f.eng, specs, nil, 1)
		if err != nil || len(first) != 1 || first[0].PropertyKey != "Sources.0" {
			t.Fatalf("slot first: %+v, %v", first, err)
		}
		after, err := r.SourceSlotHeads(t.Context(), f.eng, specs, &first[0].EdgePos, 512)
		if err != nil || len(after) != 2 || after[0].PropertyKey != "Sources.1" || after[1].PropertyKey != "Sources.0" {
			t.Fatalf("all persisted collisions, short paths only: %+v, %v", after, err)
		}
		edges, err := r.SourceSlotEdges(t.Context(), f.eng, []app.EdgePos{first[0].EdgePos, after[0].EdgePos, after[1].EdgePos})
		if err != nil || len(edges) != 3 || edges[0].Target != "a" || edges[1].Target != "b" || edges[2].Target != "outside" {
			t.Fatalf("slot raw cardinality: %+v, %v", edges, err)
		}
		f.resolve(t, p, domain.Resolution{ID: uuid.NewV7(), Asset: true}, int(after[1].Start))
		again, err := r.SourceSlotHeads(t.Context(), f.eng, specs, nil, 512)
		if err != nil || len(again) != 3 {
			t.Fatalf("asset qualification changed raw cardinality: %+v, %v", again, err)
		}
		tr := &lintTrace{}
		pool := lintPool(t, f, tr, 1)
		empty, err := postgresadapter.NewLintReads(pool, 5*1024*1024).SourceSlotHeads(t.Context(), f.eng, nil, nil, 512)
		if err != nil || len(empty) != 0 || len(tr.names()) != 0 {
			t.Fatalf("no sources issued a query: %+v, %v, %v", empty, err, tr.names())
		}
		for _, bad := range [][]app.SourceSpec{{{PageID: p, Key: "sources.deep"}}, {{PageID: p, Key: "sources", Items: 1}}, {{PageID: p, Key: "sources", Array: true, Items: 10001}}, {specs[0], specs[0]}} {
			if _, err := r.SourceSlotHeads(t.Context(), f.eng, bad, nil, 512); err == nil {
				t.Fatalf("invalid spec accepted: %+v", bad)
			}
		}
	})
	t.Run("cross-byte-row-batch-data", func(t *testing.T) {
		f := newFixture(t)
		p := uuid.UUID{}
		f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true})
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,property_key,target,target_key,ambiguous)
   SELECT $1,k,k+1,$2,'wikilink','sources.0',repeat('x',CASE WHEN k IN(510,511,512) THEN 5*1024*1024 ELSE 1 END),'x',false FROM generate_series(0,1024) k`, p, f.eng)
		tr := &lintTrace{}
		pool := lintPool(t, f, tr, 1)
		reads := app.NewLintReads(postgresadapter.NewLintReads(pool, 5*1024*1024), 5*1024*1024)
		batches := reads.SourceSlots(f.eng, []app.SourceSpec{{PageID: p, Key: "sources", Array: true, Items: 1}})
		var sizes []int
		var count int
		for {
			rows, err := batches.Next(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) == 0 {
				break
			}
			sizes = append(sizes, len(rows))
			var size int
			for _, row := range rows {
				if row.Start != int32(count) || row.PropertyKey == nil || *row.PropertyKey != "sources.0" {
					t.Fatalf("slot progress at %d: %+v", count, row)
				}
				count++
				size += len(row.Target) + len(*row.PropertyKey)
			}
			if size > 8*1024*1024 && len(rows) != 1 {
				t.Fatalf("slot bytes %d, rows %d", size, len(rows))
			}
		}
		if count != 1025 || !reflect.DeepEqual(sizes, []int{511, 1, 512, 1}) {
			t.Fatalf("slot cuts: %v, rows %d", sizes, count)
		}
		// One source spans all row/byte cuts. Its future SourceSlots owner must
		// preserve cardinality until Finish, never infer uniqueness per batch.
		if calls := tr.names(); slices.Equal(calls, []string{}) {
			t.Fatal("no production slot calls")
		}
	})
	t.Run("json-buffer-and-complete-eligible-set", func(t *testing.T) {
		f := newFixture(t)
		var ids []uuid.UUID
		for n := uint64(0); n < 1030; n++ {
			ids = append(ids, lintID(n))
		}
		lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid) SELECT unnest($1::uuid[]),$2,1,2,true`, ids, f.eng)
		lintExec(t, f, `INSERT INTO page_properties(source_id,position,notebook_id,key,value) SELECT node_id,0,$1,'sources',to_jsonb(repeat('x',1024*1024)) FROM indexed_pages WHERE notebook_id=$1 AND node_id >= $2`, f.eng, ids[1010])
		tr := &lintTrace{}
		pool := lintPool(t, f, tr, 1)
		reads := app.NewLintReads(postgresadapter.NewLintReads(pool, 5*1024*1024), 5*1024*1024)
		batches := reads.SourceValues(f.eng, ids)
		var sizes []int
		var got []uuid.UUID
		for {
			rows, err := batches.Next(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) == 0 {
				break
			}
			sizes = append(sizes, len(rows))
			var bytes int
			for _, row := range rows {
				got = append(got, row.PageID)
				bytes += len(row.JSON) + len(row.Key)
			}
			if bytes > 8*1024*1024 && len(rows) != 1 {
				t.Fatalf("JSON bytes=%d, rows=%d", bytes, len(rows))
			}
		}
		if !slices.Equal(got, ids[1010:]) || !reflect.DeepEqual(sizes, []int{7, 7, 6}) {
			t.Fatalf("complete source data: IDs %d sizes %v", len(got), sizes)
		}
		calls := tr.names()
		want := []string{"LintFirstSourceHeads", "LintFirstSourceHeads", "LintFirstSourceValues", "LintFirstSourceValues", "LintFirstSourceHeads", "LintFirstSourceValues"}
		if !reflect.DeepEqual(calls, want) {
			t.Fatalf("JSON buffer calls = %v; want %v", calls, want)
		}
	})
}
