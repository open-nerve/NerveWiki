package postgresadapter_test

import (
	"slices"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Literal source runs use the same real schema/Store/Tx fixture. Each source
// has at most 10000 rows, and source zero is a real first key, not a sentinel.
func newSetWriteRuns(t *testing.T, runs []int) setWriteFixture {
	t.Helper()
	f := newSetWriteFixture(t, 0)
	f.sources = make([]uuid.UUID, len(runs))
	counts := make([]int32, len(runs))
	total := 0
	for i, n := range runs {
		if n < 1 || n > 10000 {
			t.Fatalf("invalid literal source run %d", n)
		}
		f.sources[i], counts[i] = lintID(uint64(i)), int32(n)
		total += n
	}
	lintExec(t, f.fixture, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT unnest($1::uuid[]),$2,1,$3,true ON CONFLICT(node_id) DO NOTHING`, f.sources, f.eng, domain.Extractor)
	lintExec(t, f.fixture, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,resolved_id,ambiguous,resolved_asset)
 SELECT u.id,k,k+1,$3,'wikilink','x','x',$4,false,false
 FROM unnest($1::uuid[],$2::integer[]) AS u(id,n) CROSS JOIN LATERAL generate_series(0,u.n-1) k`, f.sources, counts, f.eng, lintID(240))
	var err error
	f.before, err = f.s.Links(t.Context(), f.eng, domain.Reach{Sources: f.sources})
	if err != nil || len(f.before) != total {
		t.Fatalf("literal source-run seed rows=%d want%d err=%v", len(f.before), total, err)
	}
	f.desired = slices.Clone(f.before)
	at := 0
	for i, n := range runs {
		for start := range n {
			if f.before[at].SourceID != lintID(uint64(i)) || f.before[at].Start != start {
				t.Fatalf("literal seed key at%d=%+v", at, f.before[at])
			}
			switch at % 4 {
			case 0:
				f.desired[at].Resolution = domain.Resolution{}
			case 1:
				f.desired[at].Resolution = domain.Resolution{ID: lintID(250)}
			case 2:
				f.desired[at].Resolution = domain.Resolution{ID: lintID(250), Ambiguous: true}
			case 3:
				f.desired[at].Resolution = domain.Resolution{ID: lintID(251), Asset: true}
			}
			at++
		}
	}
	return f
}

// Splitting every source separately, sending a mixed-source group to the new
// SQL, dropping a short tail, or shifting any of the five arrays fails against
// exact native inputs and the complete committed image.
func TestSetResolutionsCoalescesShortSources(t *testing.T) {
	for _, name := range []string{"many_short", "short_long_tail"} {
		t.Run(name, func(t *testing.T) {
			runs := []int{1, 1, 8447, 1}
			lengths := []int{256, 4096, 4096, 2}
			queries := []string{"SetResolutions", "SetOrderedSourceResolutions", "SetOrderedSourceResolutions", "SetResolutions"}
			if name == "many_short" {
				runs = make([]int, 769)
				for i := range runs {
					runs[i] = 1
				}
				lengths = []int{256, 256, 256, 1}
				queries = []string{"SetResolutions", "SetResolutions", "SetResolutions", "SetResolutions"}
			}
			f := newSetWriteRuns(t, runs)
			pid, err := f.set(t.Context(), f.desired)
			if err != nil {
				t.Fatal(err)
			}
			f.assertRows(t, f.desired)
			calls := f.assertCalls(t, f.desired, lengths, pid, pgx.QueryExecModeCacheDescribe, queries...)
			for i, c := range calls {
				if c.Err != nil || c.Rows != int64(lengths[i]) {
					t.Fatalf("source-run actual completed call%d=%+v", i, c)
				}
			}
			if name == "short_long_tail" {
				// The first old query consumes two short sources plus 254 long
				// rows; the final old query joins long tail1 to the last short1.
				first := []setWriteKey{{uuid.UUID{}, 0}, {lintID(2), 254}, {lintID(2), 4350}, {lintID(2), 8446}}
				last := []setWriteKey{{lintID(2), 253}, {lintID(2), 4349}, {lintID(2), 8445}, {lintID(3), 0}}
				for i, c := range calls {
					if c.First != first[i] || c.Last != last[i] {
						t.Fatalf("literal mixed-run endpoints call%d=%+v want%+v/%+v", i, c, first[i], last[i])
					}
				}
			}
		})
	}
}
