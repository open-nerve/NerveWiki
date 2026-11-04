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
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// This file uses only pre-existing product interfaces. It can be compiled on
// the original tree: take=16384 must fail behaviorally there, not at build time.
// Each source has at most 10000 body rows, including in the 16385-row case.
func ingestOldBodyBoundedSeed(t *testing.T, n int) (fixture, []uuid.UUID, []app.BodyReference) {
	t.Helper()
	f, ids, _ := ingestOldBodyValueSeed(t, 0)
	zero, late, target := lintID(0), lintID(100), lintID(9000)
	lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,resolved_id,resolved_asset,ambiguous)
 SELECT CASE WHEN k<10000 THEN $1::uuid ELSE $2::uuid END,4*(k%10000),4*(k%10000)+1,$3,
        (ARRAY['wikilink','embed','link','image'])[k%4+1],'body','body',
        CASE WHEN k%10000=0 THEN $1::uuid ELSE $4::uuid END,(k%2=0),false
 FROM generate_series(0,$5::integer-1) k`, zero, late, f.eng, target, n)
	var want []app.BodyReference
	for i := range n {
		source, local, resolved := zero, i%10000, target
		if i >= 10000 {
			source = late
		}
		if local == 0 {
			resolved = zero
		}
		want = append(want, app.BodyReference{EdgePos: app.EdgePos{SourceID: source, Start: int32(4 * local)}, ResolvedID: resolved, ResolvedAsset: i%2 == 0})
	}
	return f, ids, want
}

func TestIngestOldBodyBoundedValues(t *testing.T) {
	for _, n := range []int{0, 1, 511, 512, 513, 16383, 16384, 16385} {
		t.Run("rows="+strconv.Itoa(n), func(t *testing.T) {
			f, ids, want := ingestOldBodyBoundedSeed(t, n)
			original := slices.Clone(ids)
			ordered := slices.Clone(ids)
			slices.SortFunc(ordered, uuid.UUID.Compare)
			for _, description := range []int{0, 8} {
				t.Run("description="+strconv.Itoa(description), func(t *testing.T) {
					tr := &ingestOldBodyTrace{}
					pool := lintBodyModePool(t, f, tr, 0, description)
					reader := postgresadapter.NewIngestReads(pool, 5<<20)
					mode := pgx.QueryExecModeDescribeExec
					if description > 0 {
						mode = pgx.QueryExecModeCacheDescribe
					}
					err := lintBodyTxManager(pool).WithinTx(t.Context(), func(ctx context.Context) error {
						ingestOldBodyFixtureHeads(t, ctx, f, reader, ids)
						tx, ok := postgres.TxFrom(ctx)
						if !ok {
							t.Fatal("lost actual Tx")
						}
						var after *app.EdgePos
						offset := 0
						// Literal private SQL bound, independent of the new product constant.
						calls := (n+16383)/16384 + 1
						for i := range calls {
							got, err := reader.OldBodyReferences(ctx, f.eng, ids, after, 16384)
							length := min(16384, n-offset)
							if err != nil || len(got) != length || cap(got) > 16384 || !slices.Equal(got, want[offset:offset+length]) || length == 0 && got != nil {
								t.Fatalf("private raw batch%d length/cap=%d/%d want%d offset%d err=%v", i, len(got), cap(got), length, offset, err)
							}
							seen := tr.observations()
							if len(seen) != i+1 {
								t.Fatalf("native calls%d want%d", len(seen), i+1)
							}
							name := ingestOldBodyFirst
							args := []any{mode, f.eng, ordered[0], ordered[len(ordered)-1], int32(16384), ids}
							if after != nil {
								name = ingestOldBodyAfter
								args = []any{mode, f.eng, ordered[0], ordered[len(ordered)-1], after.SourceID, after.Start, int32(16384), ids}
							}
							call := seen[len(seen)-1]
							lintAssertMode(t, call, name, mode, tx.Conn().PgConn().PID(), 0, description)
							if call.Err != nil || !reflect.DeepEqual(call.Args, args) {
								t.Fatalf("actual native parameters/error: %+v %v want%+v", call.Args, call.Err, args)
							}
							offset += length
							if length > 0 {
								pos := want[offset-1].EdgePos
								after = &pos
							}
						}
						if offset != n || !slices.Equal(ids, original) || tx.Conn().PgConn().IsBusy() {
							t.Fatal("incomplete values, changed emitter input or unclosed native rows")
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
}

func TestIngestOldBodyPrivateBoundKeepsPublic512(t *testing.T) {
	f, ids, _ := ingestOldBodyBoundedSeed(t, 513)
	tr := &lintTrace{}
	pool := lintBodyModePool(t, f, tr, 0, 8)
	reader := postgresadapter.NewLintReads(pool, 5<<20)
	err := lintBodyTxManager(pool).WithinTx(t.Context(), func(ctx context.Context) error {
		for _, take := range []int{513, 16384} {
			rows, err := reader.BodyReferences(ctx, f.eng, ids, nil, take)
			if err == nil || rows != nil {
				t.Fatalf("public Body accepted private-sized take%d: rows%d err=%v", take, len(rows), err)
			}
		}
		if len(tr.observations()) != 0 || len(tr.prepareObservations()) != 0 {
			t.Fatal("rejected public oversized batch emitted SQL")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
