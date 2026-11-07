package postgresadapter

import (
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Each expected row is literal, independent of SQL arm selection. The mixed
// reach catches repeated overlap rows and reversed ordering. The unique-arm
// cases catch missing arms and losing a later match when an earlier predicate
// is UNKNOWN; all-nil input alone cannot catch that loss. The pairwise overlaps
// isolate each predecessor exclusion so another exclusion cannot mask its loss.
func TestLinksReachedUnionSemantics(t *testing.T) {
	pool, trace := newLinksCollectorPool(t)
	book, foreign := uuid.UUID{15: 200}, uuid.UUID{15: 201}
	p1, p2, p3, p4 := uuid.UUID{15: 1}, uuid.UUID{15: 2}, uuid.UUID{15: 3}, uuid.UUID{15: 4}
	r1, r2, r3 := uuid.UUID{15: 11}, uuid.UUID{15: 12}, uuid.UUID{15: 13}
	p5, r4 := uuid.UUID{15: 5}, uuid.UUID{15: 14}
	fKey, fAlt, fTarget, fSource, fOverlap := uuid.UUID{15: 21}, uuid.UUID{15: 22}, uuid.UUID{15: 23}, uuid.UUID{15: 24}, uuid.UUID{15: 25}
	// Source and range order are deliberately reversed. Empty text and zero
	// UUID rows also detect coercing native NULL array elements into values.
	linksCollectorCopy(t, pool, [][]any{
		{fOverlap, int32(0), int32(1), foreign, "link", "foreign-overlap", "overlap", "overlap", r1, false, false, false},
		{fSource, int32(0), int32(1), foreign, "link", "foreign-source", nil, nil, nil, false, false, false},
		{fTarget, int32(0), int32(1), foreign, "link", "foreign-target", nil, nil, r2, false, false, false},
		{fAlt, int32(0), int32(1), foreign, "link", "foreign-alt", nil, "alt", nil, false, false, false},
		{fKey, int32(0), int32(1), foreign, "link", "foreign-key", "key", nil, nil, false, false, false},
		{p5, int32(0), int32(1), book, "link", "alt-target", nil, "alt-target", r4, false, true, true},
		{p4, int32(20), int32(21), book, "link", "zero-target", nil, nil, uuid.UUID{}, true, true, true},
		{p4, int32(10), int32(11), book, "link", "empty-alt", nil, "", nil, false, false, false},
		{p4, int32(0), int32(1), book, "link", "empty-key", "", nil, nil, false, false, false},
		{p3, int32(0), int32(1), book, "link", "target-only", nil, nil, r2, true, true, false},
		{p2, int32(20), int32(21), book, "link", "alt-only", nil, "alt", nil, false, false, true},
		{p2, int32(10), int32(11), book, "link", "key-only", "key", nil, r3, false, true, false},
		{p1, int32(20), int32(21), book, "link", "source-only", nil, nil, nil, false, false, true},
		{p1, int32(10), int32(11), book, "link", "overlap", "overlap", "overlap", r1, true, false, true},
		{uuid.UUID{}, int32(0), int32(1), book, "link", "zero-source", nil, nil, nil, false, false, false},
	})
	overlap := app.Link{SourceID: p1, Start: 10, Target: "overlap", Resolution: domain.Resolution{ID: r1, Ambiguous: true}, Aliases: true}
	source := app.Link{SourceID: p1, Start: 20, Target: "source-only", Aliases: true}
	key := app.Link{SourceID: p2, Start: 10, Target: "key-only", Resolution: domain.Resolution{ID: r3, Asset: true}}
	alt := app.Link{SourceID: p2, Start: 20, Target: "alt-only", Aliases: true}
	target := app.Link{SourceID: p3, Start: 0, Target: "target-only", Resolution: domain.Resolution{ID: r2, Ambiguous: true, Asset: true}}
	altTarget := app.Link{SourceID: p5, Start: 0, Target: "alt-target", Resolution: domain.Resolution{ID: r4, Asset: true}, Aliases: true}
	wantAll := []app.Link{overlap, source, key, alt, target}
	conn, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	store := New(pool)
	for _, tc := range []struct {
		name  string
		reach domain.Reach
		want  []app.Link
	}{
		{"all_nil", domain.Reach{}, nil},
		{"all_empty", domain.Reach{Keys: []string{}, Targets: []uuid.UUID{}, Sources: []uuid.UUID{}}, nil},
		{"key_nil_other_arms", domain.Reach{Keys: []string{"key"}}, []app.Link{key}},
		{"key_empty_other_arms", domain.Reach{Keys: []string{"key"}, Targets: []uuid.UUID{}, Sources: []uuid.UUID{}}, []app.Link{key}},
		{"alt_nil_other_arms", domain.Reach{Keys: []string{"alt"}}, []app.Link{alt}},
		{"alt_empty_other_arms", domain.Reach{Keys: []string{"alt"}, Targets: []uuid.UUID{}, Sources: []uuid.UUID{}}, []app.Link{alt}},
		{"target_nil_other_arms", domain.Reach{Targets: []uuid.UUID{r2}}, []app.Link{target}},
		{"target_empty_other_arms", domain.Reach{Keys: []string{}, Targets: []uuid.UUID{r2}, Sources: []uuid.UUID{}}, []app.Link{target}},
		{"source_nil_other_arms", domain.Reach{Sources: []uuid.UUID{p1}}, []app.Link{overlap, source}},
		{"source_empty_other_arms", domain.Reach{Keys: []string{}, Targets: []uuid.UUID{}, Sources: []uuid.UUID{p1}}, []app.Link{overlap, source}},
		{"source_key_overlap", domain.Reach{Keys: []string{"key"}, Sources: []uuid.UUID{p2}}, []app.Link{key, alt}},
		{"source_alt_overlap", domain.Reach{Keys: []string{"alt"}, Sources: []uuid.UUID{p2}}, []app.Link{key, alt}},
		{"key_alt_overlap", domain.Reach{Keys: []string{"overlap"}}, []app.Link{overlap}},
		{"source_resolved_overlap", domain.Reach{Targets: []uuid.UUID{r2}, Sources: []uuid.UUID{p3}}, []app.Link{target}},
		{"key_resolved_overlap", domain.Reach{Keys: []string{"key"}, Targets: []uuid.UUID{r3}}, []app.Link{key}},
		{"alt_resolved_overlap", domain.Reach{Keys: []string{"alt-target"}, Targets: []uuid.UUID{r4}}, []app.Link{altTarget}},
		{"all_arms_once_and_ordered", domain.Reach{Keys: []string{"overlap", "key", "alt"}, Targets: []uuid.UUID{r1, r2}, Sources: []uuid.UUID{p1, fSource, fOverlap}}, wantAll},
		{"duplicate_inputs", domain.Reach{Keys: []string{"alt", "key", "overlap", "key", "alt", "overlap"}, Targets: []uuid.UUID{r2, r1, r2, r1}, Sources: []uuid.UUID{fOverlap, p1, fSource, p1, fOverlap}}, wantAll},
		{"empty_keys_are_values", domain.Reach{Keys: []string{""}}, []app.Link{
			{SourceID: p4, Start: 0, Target: "empty-key"}, {SourceID: p4, Start: 10, Target: "empty-alt"},
		}},
		{"zero_uuids_are_values", domain.Reach{Targets: []uuid.UUID{{}}, Sources: []uuid.UUID{{}}}, []app.Link{
			{SourceID: uuid.UUID{}, Start: 0, Target: "zero-source"},
			{SourceID: p4, Start: 20, Target: "zero-target", Resolution: domain.Resolution{Ambiguous: true, Asset: true}, Aliases: true},
		}},
		{"foreign_sources_only", domain.Reach{Sources: []uuid.UUID{fSource}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := domain.Reach{Keys: slices.Clone(tc.reach.Keys), Targets: slices.Clone(tc.reach.Targets), Sources: slices.Clone(tc.reach.Sources)}
			before := trace.count()
			got, err := store.Links(postgres.OnConn(t.Context(), conn), book, tc.reach)
			if err != nil || got == nil || !slices.Equal(got, tc.want) || cap(got) != len(tc.want) {
				t.Fatalf("Links=%+v cap=%d err=%v; want exact literal %+v", got, cap(got), err, tc.want)
			}
			if trace.count() != before+1 || !reflect.DeepEqual(tc.reach, original) {
				t.Fatal("Links changed caller arrays or skipped/repeated its native query")
			}
			trace.checkLast(t, conn.Conn().PgConn().PID(), book, original, nil)
			linksCollectorReusable(t, conn)
		})
	}

	// Public Reach has no nullable element type. Run the exact generated SQL
	// and original collector with native nullable arrays for this SQL boundary.
	// The existing Rows observer still forwards real EOF, Err and Close.
	for _, tc := range []struct {
		name             string
		keys             []pgtype.Text
		targets, sources []pgtype.UUID
		want             []app.Link
	}{
		{"null_elements_only", []pgtype.Text{{}}, []pgtype.UUID{{}}, []pgtype.UUID{{}}, nil},
		{"key_after_null_source", []pgtype.Text{{}, {String: "key", Valid: true}}, []pgtype.UUID{{}}, []pgtype.UUID{{}}, []app.Link{key}},
		{"alt_after_unknown_key_and_source", []pgtype.Text{{}, {String: "alt", Valid: true}}, nil, []pgtype.UUID{{}}, []app.Link{alt}},
		{"target_after_unknown_keys_and_sources", []pgtype.Text{{}}, []pgtype.UUID{{}, {Bytes: r2, Valid: true}}, []pgtype.UUID{{}}, []app.Link{target}},
		{"source_after_unknown_keys_and_targets", []pgtype.Text{{}}, []pgtype.UUID{{}}, []pgtype.UUID{{}, {Bytes: p1, Valid: true}}, []app.Link{overlap, source}},
		{"mixed_null_and_duplicate_elements", []pgtype.Text{{}, {String: "overlap", Valid: true}, {String: "key", Valid: true}, {String: "alt", Valid: true}, {String: "key", Valid: true}},
			[]pgtype.UUID{{}, {Bytes: r1, Valid: true}, {Bytes: r2, Valid: true}, {Bytes: r1, Valid: true}},
			[]pgtype.UUID{{}, {Bytes: p1, Valid: true}, {Bytes: fSource, Valid: true}, {Bytes: fOverlap, Valid: true}, {Bytes: p1, Valid: true}}, wantAll},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := []any{pgx.QueryExecModeDescribeExec, book, slices.Clone(tc.keys), slices.Clone(tc.targets), slices.Clone(tc.sources)}
			before := trace.count()
			native, err := postgres.Planned(conn).Query(t.Context(), gen.LinksReached, book, tc.keys, tc.targets, tc.sources)
			if err != nil {
				t.Fatal(err)
			}
			rows := &linksCollectorRows{Rows: native, stable: true}
			var buffer linksReachedBuffer
			got, err := buffer.read(rows)
			if err != nil || got == nil || !slices.Equal(got, tc.want) || cap(got) != len(tc.want) {
				t.Fatalf("native nullable arrays=%+v cap=%d err=%v; want literal %+v", got, cap(got), err, tc.want)
			}
			rows.check(t, len(tc.want), 1, len(tc.want), 1)
			if trace.count() != before+1 || !reflect.DeepEqual([]any{pgx.QueryExecModeDescribeExec, book, tc.keys, tc.targets, tc.sources}, original) {
				t.Fatal("nullable SQL changed caller arrays or skipped/repeated Query")
			}
			trace.mu.Lock()
			call, active := *trace.calls[len(trace.calls)-1], trace.active
			trace.mu.Unlock()
			if call.pid != conn.Conn().PgConn().PID() || !reflect.DeepEqual(call.args, original) ||
				!slices.Equal(call.prepares, []string{""}) || call.ends != 1 || call.err != nil || active != nil {
				t.Fatalf("nullable SQL did not retain four native arguments/unnamed prepare/end: %+v", call)
			}
			linksCollectorReusable(t, conn)
		})
	}
}
