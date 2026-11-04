package postgresadapter_test

import (
	"context"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Persisted adapter values, not parser/source-role qualification. Every
// requested emitter has a current same-book head; rejected edges test only
// the predicates that the private SQL must retain after head qualification.
func ingestOldBodyValueSeed(t *testing.T, n int) (fixture, []uuid.UUID, []app.BodyReference) {
	t.Helper()
	f := newFixture(t)
	zero, late, foreignRows, empty, unrequested := lintID(0), lintID(100), lintID(40), lintID(60), lintID(20)
	source := func(id, linkBook uuid.UUID, count int) []app.BodyReference {
		f.replace(t, app.Page{ID: id, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true})
		// Accepted rows are interleaved with named/empty-property and
		// unresolved peers. Four body kinds, both asset values and a literal
		// non-NULL zero target survive; repeated target IDs are not collapsed.
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,property_key,target,target_key,resolved_id,resolved_asset,ambiguous)
 SELECT $1,4*k+j,4*k+j+1,$2,(ARRAY['wikilink','embed','link','image'])[k%4+1],
        CASE j WHEN 1 THEN 'named' WHEN 2 THEN '' ELSE NULL END,'body','body',
        CASE WHEN j=3 THEN NULL WHEN k=0 THEN $3::uuid ELSE $4::uuid END,
        (j<>3 AND k%2=0),false
 FROM generate_series(0,$5::integer-1) k CROSS JOIN generate_series(0,3) j`, id, linkBook, uuid.Nil(), lintID(9000), count)
		var want []app.BodyReference
		for i := range count {
			target := lintID(9000)
			if i == 0 {
				target = uuid.Nil()
			}
			want = append(want, app.BodyReference{EdgePos: app.EdgePos{SourceID: id, Start: int32(4 * i)}, ResolvedID: target, ResolvedAsset: i%2 == 0})
		}
		return want
	}
	want := append(source(zero, f.eng, min(n, 1024)), source(late, f.eng, max(n-1024, 0))...)
	source(foreignRows, f.ops, 1) // current head does not prove edge notebook.
	source(empty, f.eng, 0)
	source(unrequested, f.eng, 1) // within min/max, outside complete membership.
	return f, []uuid.UUID{late, foreignRows, zero, empty, zero}, want
}

// Exercise real First/After calls to an explicit empty EOF, with a finite
// independently expected batch count. No production pagination or old reader
// is used to construct the expected values or to terminate this test.
func ingestOldBodyConsume(t *testing.T, ctx context.Context, f fixture, r *postgresadapter.IngestReads, tr *ingestOldBodyTrace,
	ids []uuid.UUID, want []app.BodyReference, mode pgx.QueryExecMode, statement, description int) {
	t.Helper()
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		t.Fatal("value fixture lost Tx")
	}
	original := slices.Clone(ids)
	ordered := slices.Clone(ids)
	slices.SortFunc(ordered, uuid.UUID.Compare)
	var after *app.EdgePos
	before, offset := len(tr.observations()), 0
	// Even a short nonempty batch requires the next empty query.
	calls := (len(want)+511)/512 + 1
	for i := range calls {
		take := min(512, len(want)-offset)
		rows, err := r.OldBodyReferences(ctx, f.eng, ids, after, 512)
		if err != nil || len(rows) != take || !slices.Equal(rows, want[offset:offset+take]) {
			t.Fatalf("old-F batch%d: rows=%d want%d offset%d err=%v", i, len(rows), take, offset, err)
		}
		observed := tr.observations()
		if len(observed) != before+i+1 {
			t.Fatalf("old-F extra/missing query: %+v", observed[before:])
		}
		name := ingestOldBodyFirst
		args := []any{mode, f.eng, ordered[0], ordered[len(ordered)-1], int32(512), ids}
		if after != nil {
			name = ingestOldBodyAfter
			args = []any{mode, f.eng, ordered[0], ordered[len(ordered)-1], after.SourceID, after.Start, int32(512), ids}
		}
		call := observed[len(observed)-1]
		lintAssertMode(t, call, name, mode, tx.Conn().PgConn().PID(), statement, description)
		if call.Err != nil || !reflect.DeepEqual(call.Args, args) {
			t.Fatalf("full old-F relation/cursor args: %+v %v want%+v", call.Args, call.Err, args)
		}
		offset += take
		if len(rows) > 0 {
			last := rows[len(rows)-1].EdgePos
			after = &last
		}
	}
	if offset != len(want) || !slices.Equal(ids, original) {
		t.Fatal("old-F incomplete DTOs or modified emitter input")
	}
}

func TestIngestOldBodyReferences(t *testing.T) {
	for _, n := range []int{0, 1, 512, 513, 1024, 1029} {
		t.Run("rows="+strconv.Itoa(n), func(t *testing.T) {
			f, ids, want := ingestOldBodyValueSeed(t, n)
			for _, description := range []int{0, 8} {
				t.Run("description="+strconv.Itoa(description), func(t *testing.T) {
					tr := &ingestOldBodyTrace{}
					pool := lintBodyModePool(t, f, tr, 0, description)
					r := postgresadapter.NewIngestReads(pool, 5<<20)
					mode := pgx.QueryExecModeDescribeExec
					if description > 0 {
						mode = pgx.QueryExecModeCacheDescribe
					}
					err := lintBodyTxManager(pool).WithinTx(t.Context(), func(ctx context.Context) error {
						ingestOldBodyFixtureHeads(t, ctx, f, r, ids)
						ingestOldBodyConsume(t, ctx, f, r, tr, ids, want, mode, 0, description)
						before := len(tr.observations())
						for _, after := range []*app.EdgePos{nil, {SourceID: ids[0], Start: 0}} {
							rows, err := r.OldBodyReferences(ctx, f.eng, nil, after, 512)
							if err != nil || rows != nil {
								t.Fatalf("empty emitter relation: %+v %v", rows, err)
							}
						}
						if len(tr.observations()) != before {
							t.Fatal("empty emitter relation queried")
						}
						ingestOldBodyNoNamed(t, ctx, pool)
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
	t.Run("complete-emitter-relation", func(t *testing.T) {
		f := newFixture(t)
		ids := make([]uuid.UUID, 1029)
		want := make([]app.BodyReference, len(ids))
		for i := range ids {
			ids[i] = lintID(uint64(i))
			want[i] = app.BodyReference{EdgePos: app.EdgePos{SourceID: ids[i]}, ResolvedID: ids[i], ResolvedAsset: true}
		}
		lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT id,$2,1,2,true FROM unnest($1::uuid[]) id`, ids, f.eng)
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,resolved_id,resolved_asset,ambiguous)
 SELECT id,0,1,$2,'link','body','body',id,true,false FROM unnest($1::uuid[]) id`, ids, f.eng)
		slices.Reverse(ids)
		ids = append(ids, lintID(0), lintID(800))
		tr := &ingestOldBodyTrace{}
		pool := lintBodyModePool(t, f, tr, 0, 8)
		r := postgresadapter.NewIngestReads(pool, 5<<20)
		err := lintBodyTxManager(pool).WithinTx(t.Context(), func(ctx context.Context) error {
			ingestOldBodyFixtureHeads(t, ctx, f, r, ids)
			ingestOldBodyConsume(t, ctx, f, r, tr, ids, want, pgx.QueryExecModeCacheDescribe, 0, 8)
			ingestOldBodyNoNamed(t, ctx, pool)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("invalid-read-bounds", func(t *testing.T) {
		f, p, _ := lintBodyModeSeed(t)
		tr := &ingestOldBodyTrace{}
		pool := lintBodyModePool(t, f, tr, 0, 8)
		r := postgresadapter.NewIngestReads(pool, 5<<20)
		err := lintBodyTxManager(pool).WithinTx(t.Context(), func(ctx context.Context) error {
			ingestOldBodyFixtureHeads(t, ctx, f, r, []uuid.UUID{p})
			for _, bad := range []struct {
				take  int
				after *app.EdgePos
			}{{0, nil}, {-1, nil}, {16385, nil}, {512, &app.EdgePos{SourceID: p, Start: -1}}} {
				rows, err := r.OldBodyReferences(ctx, f.eng, []uuid.UUID{p}, bad.after, bad.take)
				if err == nil || rows != nil {
					t.Fatalf("invalid bounds accepted: %+v rows%+v err%v", bad, rows, err)
				}
			}
			if len(tr.observations()) != 0 || len(tr.prepareObservations()) != 0 {
				t.Fatal("invalid bounds emitted SQL")
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}
