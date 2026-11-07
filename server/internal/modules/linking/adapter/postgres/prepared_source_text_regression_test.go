package postgresadapter

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Each persisted field differs independently from the immutable incoming
// literal. Bypassing payload comparison must lose the complete changed row.
func TestPreparedSourceTextLiteralMismatches(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	for _, tc := range []struct {
		name   string
		column int
		old    string
	}{
		{"same", -1, ""},
		{"kind", 4, "link"},
		{"property", 5, "q"},
		{"target", 6, "Other.md"},
		{"anchor", 7, "b"},
		{"display", 8, "e"},
		{"key", 9, "wrong"},
		{"alt", 10, "wrong.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := preparedTextRegressionPage()
			a := uuid.UUID{15: 11}
			data := []any{p.ID, int32(0), int32(7), p.NotebookID, "wikilink", "p",
				"Note.md", "a", "d", "note", "note.md", true, a, true, true}
			if tc.column >= 0 {
				data[tc.column] = tc.old
			}
			preparedTextRegressionSeed(t, pool, [][]any{data})
			incoming := []domain.Link{{Kind: "wikilink", Property: "p", Target: "Note.md",
				Anchor: "a", Display: "d", Start: 0, End: 7, Aliases: true}}
			read, observed, err := preparedTextRegressionRead(t, pool, incoming, "", "")
			if err != nil {
				t.Fatal(err)
			}
			observed.checkCallbacks(t, 1, true)
			wantDelta := preparedTextRegressionEmptyDelta()
			want := []app.Link{{SourceID: p.ID, Start: 0, Target: "Note.md", Aliases: true,
				Resolution: domain.Resolution{ID: a, Ambiguous: true, Asset: true}}}
			if tc.column >= 0 {
				want[0].Resolution = domain.Resolution{}
				wantDelta = gen.InsertLinksParams{SourceID: p.ID, NotebookID: p.NotebookID,
					RangeStarts: []int32{0}, RangeEnds: []int32{7}, Kinds: []string{"wikilink"},
					PropertyKeys: []string{"p"}, InFrontmatter: []bool{true}, Targets: []string{"Note.md"},
					Anchors: []string{"a"}, Displays: []string{"d"}, TargetKeys: []string{"note"},
					TargetAltKeys: []string{"note.md"}, Aliases: []bool{true}}
			}
			if !read.canRetain || !reflect.DeepEqual(read.links, wantDelta) || read.missing != nil ||
				read.assets == nil || len(read.assets) != 0 || !slices.Equal(read.targets, []uuid.UUID{a}) ||
				!slices.Equal(read.inputs, want) {
				t.Fatalf("PREPARED_TEXT_LITERAL: %s read=%+v", tc.name, read)
			}
		})
	}
}

// NULL and valid empty are deliberately interleaved in one native stream.
// A property may validly be empty; the other four nullable incoming fields
// normalize empty to NULL. The target "." has no title keys.
func TestPreparedSourceTextNullAndEmptyLiterals(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	p := preparedTextRegressionPage()
	a := uuid.UUID{15: 11}
	cases := []struct {
		property    any
		quote       byte
		emptyColumn int
		changed     bool
	}{
		{nil, 0, -1, false},
		{"", '"', -1, false},
		{"", 0, -1, true},
		{nil, '"', -1, true},
		{"p", 0, -1, false},
		{"p", 0, 7, true},
		{"p", 0, -1, false},
		{"p", 0, 8, true},
		{"p", 0, -1, false},
		{"p", 0, 9, true},
		{"p", 0, -1, false},
		{"p", 0, 10, true},
	}
	incoming, data, want := make([]domain.Link, len(cases)), make([][]any, len(cases)), make([]app.Link, len(cases))
	for i, tc := range cases {
		property := ""
		if i >= 4 {
			property = "p"
		}
		incoming[i] = domain.Link{Kind: "wikilink", Property: property, Quote: tc.quote, Target: ".", Start: i * 10, End: i*10 + 1}
		data[i] = []any{p.ID, int32(i * 10), int32(i*10 + 1), p.NotebookID, "wikilink", tc.property,
			".", nil, nil, nil, nil, false, a, true, true}
		if tc.emptyColumn >= 0 {
			data[i][tc.emptyColumn] = ""
		}
		want[i] = app.Link{SourceID: p.ID, Start: i * 10, Target: "."}
		if !tc.changed {
			want[i].Resolution = domain.Resolution{ID: a, Ambiguous: true, Asset: true}
		}
	}
	preparedTextRegressionSeed(t, pool, data)
	read, observed, err := preparedTextRegressionRead(t, pool, incoming, "", "")
	if err != nil {
		t.Fatal(err)
	}
	observed.checkCallbacks(t, 12, true)
	wantDelta := gen.InsertLinksParams{SourceID: p.ID, NotebookID: p.NotebookID,
		RangeStarts: []int32{20, 30, 50, 70, 90, 110}, RangeEnds: []int32{21, 31, 51, 71, 91, 111},
		Kinds:        []string{"wikilink", "wikilink", "wikilink", "wikilink", "wikilink", "wikilink"},
		PropertyKeys: []string{"", "", "p", "p", "p", "p"}, InFrontmatter: []bool{false, true, true, true, true, true},
		Targets: []string{".", ".", ".", ".", ".", "."}, Anchors: []string{"", "", "", "", "", ""}, Displays: []string{"", "", "", "", "", ""},
		TargetKeys: []string{"", "", "", "", "", ""}, TargetAltKeys: []string{"", "", "", "", "", ""},
		Aliases: []bool{false, false, false, false, false, false}}
	if !read.canRetain || !reflect.DeepEqual(read.links, wantDelta) || read.missing != nil ||
		!slices.Equal(read.inputs, want) || !slices.Equal(read.assets, []uuid.UUID{a, a}) || !slices.Equal(read.targets, []uuid.UUID{a}) {
		t.Fatalf("PREPARED_TEXT_NULL_IDENTITY: read=%+v", read)
	}
}

func TestPreparedSourceTextAlternatesAcrossNativeBuffers(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	p := preparedTextRegressionPage()
	a, b := uuid.UUID{15: 11}, uuid.UUID{15: 12}
	const n = 128
	incoming, data, want := make([]domain.Link, n), make([][]any, n), make([]app.Link, n)
	for i := range n {
		start := i * 1024
		var property, anchor, display, key, alt any
		var resolved any = a
		link := domain.Link{Kind: "wikilink", Start: start, End: start + 243}
		resolution := domain.Resolution{ID: a, Ambiguous: true, Asset: true}
		switch i % 4 {
		case 0:
			link.Property, link.Target, link.Anchor, link.Display = preparedTextCopyProperty, preparedTextCopyTarget, preparedTextCopyAnchor, preparedTextCopyDisplay
			property, anchor, display, key, alt = preparedTextCopyProperty, preparedTextCopyAnchor, preparedTextCopyDisplay, preparedTextCopyKey, preparedTextCopyAlt
		case 1:
			link.Property, link.Target, link.Anchor, link.Display, link.End = "p", "N.md", "a", "d", start+4
			property, anchor, display, key, alt = "p", "a", "d", "n", "n.md"
			resolved, resolution = b, domain.Resolution{ID: b, Ambiguous: true, Asset: true}
		case 2:
			link.Target, link.End = ".", start+1
			resolved, resolution = nil, domain.Resolution{}
		case 3:
			link.Target, link.End, link.Quote = "N", start+1, '"'
			property, key = "", "n"
		}
		incoming[i] = link
		data[i] = []any{p.ID, int32(start), int32(link.End), p.NotebookID, "wikilink", property, link.Target,
			anchor, display, key, alt, false, resolved, resolution.Ambiguous, resolution.Asset}
		// The expected consumer relation is built from this literal case,
		// never from an equality or collector result.
		target := [4]string{preparedTextCopyTarget, "N.md", ".", "N"}[i%4]
		want[i] = app.Link{SourceID: p.ID, Start: start, Target: target, Resolution: resolution}
	}
	preparedTextRegressionSeed(t, pool, data)
	read, observed, err := preparedTextRegressionRead(t, pool, incoming, "", "")
	if err != nil {
		t.Fatal(err)
	}
	observed.checkCallbacks(t, n, true)
	if !read.canRetain || !reflect.DeepEqual(read.links, preparedTextRegressionEmptyDelta()) || read.missing != nil ||
		read.assets == nil || len(read.assets) != 0 || !slices.Equal(read.targets, []uuid.UUID{a, b}) || !slices.Equal(read.inputs, want) {
		t.Fatalf("PREPARED_TEXT_BUFFER_OWNERSHIP: read=%+v", read)
	}
}

// Standalone native SELECTs characterize the collector boundary where
// production COALESCE/schema constraints cannot emit these descriptors/NULLs.
const preparedTextRegressionSQL = `SELECT true AS can_retain, 0::integer AS range_start, 7::integer AS range_end,
'00000000-0000-0000-0000-000000000001'::uuid AS notebook_id,
'wikilink'::text AS kind, 'p'::text AS property_key, 'Note.md'::text AS target,
'a'::text AS anchor, 'd'::text AS display, 'note'::text AS target_key, 'note.md'::text AS target_alt_key,
true AS aliases, '00000000-0000-0000-0000-00000000000b'::uuid AS resolved_id,
true AS ambiguous, true AS resolved_asset`

func TestPreparedSourceTextNativeFallbacks(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	p := preparedTextRegressionPage()
	a := uuid.UUID{15: 11}
	preparedTextRegressionSeed(t, pool, [][]any{{p.ID, int32(0), int32(7), p.NotebookID, "wikilink", "p",
		"Note.md", "a", "d", "note", "note.md", true, a, true, true}})
	incoming := []domain.Link{{Kind: "wikilink", Property: "p", Target: "Note.md", Anchor: "a", Display: "d", Start: 0, End: 7, Aliases: true}}
	for _, mode := range []string{"standard", "varchar", "custom_codec", "nil_map", "other_map", "nil_conn"} {
		t.Run(mode, func(t *testing.T) {
			query := ""
			if mode == "varchar" {
				query = strings.Replace(preparedTextRegressionSQL, "'wikilink'::text", "'wikilink'::varchar", 1)
			}
			read, observed, err := preparedTextRegressionRead(t, pool, incoming, query, mode)
			if err != nil {
				t.Fatal(err)
			}
			observed.checkCallbacks(t, 1, mode == "standard")
			want := []app.Link{{SourceID: p.ID, Start: 0, Target: "Note.md", Aliases: true, Resolution: domain.Resolution{ID: a, Ambiguous: true, Asset: true}}}
			if !read.canRetain || !reflect.DeepEqual(read.links, preparedTextRegressionEmptyDelta()) || read.missing != nil ||
				read.assets == nil || len(read.assets) != 0 || !slices.Equal(read.targets, []uuid.UUID{a}) || !slices.Equal(read.inputs, want) {
				t.Fatalf("PREPARED_TEXT_FALLBACK: mode=%s read=%+v", mode, read)
			}
		})
	}
}

func TestPreparedSourceTextRequiredNullKeepsFirstNativeError(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	incoming := []domain.Link{{Kind: "wikilink", Property: "p", Target: "Note.md", Anchor: "a", Display: "d", Start: 0, End: 7, Aliases: true}}
	for _, tc := range []struct {
		name, old string
		column    int
		calls     [7]int
	}{
		{"kind", "'wikilink'::text AS kind", 4, [7]int{1, 0, 0, 0, 0, 0, 0}},
		{"target", "'Note.md'::text AS target", 6, [7]int{1, 1, 1, 0, 0, 0, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := strings.Replace(preparedTextRegressionSQL, tc.old, "NULL::text AS "+tc.name, 1)
			query = strings.Replace(query, "true AS aliases", "NULL::boolean AS aliases", 1)
			read, observed, err := preparedTextRegressionRead(t, pool, incoming, query, "")
			var scan pgx.ScanArgError
			if !errors.As(err, &scan) || scan.ColumnIndex != tc.column || scan.FieldName != tc.name ||
				scan.Err.Error() != "cannot scan NULL into *string" || !reflect.DeepEqual(read, preparedLinkRead{}) {
				t.Fatalf("PREPARED_TEXT_FIRST_ERROR: field=%s result=%+v err=%v", tc.name, read, err)
			}
			observed.check(t, 1, 0, 1, 0)
			if observed.problem != "" || observed.calls != tc.calls {
				t.Fatalf("PREPARED_TEXT_CALLBACK_ORDER: counts=%v want=%v problem=%s", observed.calls, tc.calls, observed.problem)
			}
		})
	}
}

func preparedTextRegressionPage() app.Page {
	return app.Page{ID: uuid.UUID{15: 50}, NotebookID: uuid.UUID{15: 1}, Revision: 1}
}

func preparedTextRegressionEmptyDelta() gen.InsertLinksParams {
	p := preparedTextRegressionPage()
	return gen.InsertLinksParams{SourceID: p.ID, NotebookID: p.NotebookID,
		RangeStarts: []int32{}, RangeEnds: []int32{}, Kinds: []string{}, PropertyKeys: []string{}, InFrontmatter: []bool{},
		Targets: []string{}, Anchors: []string{}, Displays: []string{}, TargetKeys: []string{}, TargetAltKeys: []string{}, Aliases: []bool{}}
}

func preparedTextRegressionSeed(t *testing.T, pool *pgxpool.Pool, data [][]any) {
	t.Helper()
	p := preparedTextRegressionPage()
	if _, err := pool.Exec(t.Context(), "TRUNCATE page_links,indexed_pages"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
VALUES ($1,$2,1,2,true)`, p.ID, p.NotebookID); err != nil {
		t.Fatal(err)
	}
	n, err := pool.CopyFrom(t.Context(), pgx.Identifier{"page_links"}, []string{
		"source_id", "range_start", "range_end", "notebook_id", "kind", "property_key", "target", "anchor", "display",
		"target_key", "target_alt_key", "aliases", "resolved_id", "ambiguous", "resolved_asset"}, pgx.CopyFromRows(data))
	if err != nil || n != int64(len(data)) {
		t.Fatalf("literal COPY %d/%d: %v", n, len(data), err)
	}
}

// The read and the overwriting response share a physical connection. Only
// forwarding metadata wrappers are used for the unsupported-metadata cases;
// their Scan/Next/Err/Close remain the real PostgreSQL operations.
func preparedTextRegressionRead(t *testing.T, pool *pgxpool.Pool, values []domain.Link, query, mode string) (read preparedLinkRead, observed *preparedTextCallbackRows, err error) {
	t.Helper()
	p := preparedTextRegressionPage()
	conn, err := pool.Acquire(t.Context())
	if err != nil {
		return read, nil, err
	}
	defer conn.Release()
	if mode == "custom_codec" {
		original, _ := conn.Conn().TypeMap().TypeForOID(pgtype.TextOID)
		conn.Conn().TypeMap().RegisterType(&pgtype.Type{Name: "text", OID: pgtype.TextOID, Codec: preparedTextCustomCodec{}})
		defer conn.Conn().TypeMap().RegisterType(original)
	}
	original := slices.Clone(values)
	incoming := newPreparedLinkFacts(p, values)
	observed = &preparedTextCallbackRows{linksCollectorRows: &linksCollectorRows{stable: true, columns: 15}, mode: mode}
	if mode == "other_map" {
		observed.otherMap = pgtype.NewMap()
	}
	err = postgres.NewTxManager(pool, time.Second).WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
		if err := New(pool).Lock(ctx, p.NotebookID); err != nil {
			return err
		}
		tx, _ := postgres.TxFrom(ctx)
		var readErr error
		if query == "" {
			q := &preparedTextCallbackQuerier{Querier: tx, rows: observed}
			read, readErr = inspectPageLinksForPreparation(ctx, q, gen.InspectPageLinksForPreparationParams{
				SourceID: p.ID, NotebookID: p.NotebookID, Extractor: domain.Extractor}, &incoming)
			if q.calls != 1 {
				t.Fatalf("native production query count=%d", q.calls)
			}
		} else {
			native, err := tx.Query(ctx, query)
			if err != nil {
				return err
			}
			observed.Rows = native
			read, readErr = collectPreparedSource(observed, &incoming)
		}
		if readErr != nil {
			return readErr
		}
		linksCollectorReusable(t, conn)
		var overwrite string
		if err := conn.QueryRow(ctx, "SELECT repeat('z',65536)").Scan(&overwrite); err != nil {
			return err
		}
		if len(overwrite) != 65536 || !slices.Equal(values, original) {
			t.Fatal("native reuse or incoming ownership changed")
		}
		return nil
	})
	return read, observed, err
}

type preparedTextCustomCodec struct{ pgtype.TextCodec }

type preparedTextCallbackQuerier struct {
	postgres.Querier
	rows  *preparedTextCallbackRows
	calls int
}

func (q *preparedTextCallbackQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.calls++
	rows, err := q.Querier.Query(ctx, sql, args...)
	if rows != nil {
		q.rows.Rows = rows
		return q.rows, err
	}
	return rows, err
}

type preparedTextCallbackRows struct {
	*linksCollectorRows
	mode           string
	otherMap       *pgtype.Map
	original, dest [15]any
	callbacks      [7]preparedTextForwardCallback
	calls          [7]int
	optimized      bool
	problem        string
}

func (r *preparedTextCallbackRows) TypeMap() *pgtype.Map {
	switch r.mode {
	case "nil_map":
		return nil
	case "other_map":
		return r.otherMap
	}
	return r.Rows.TypeMap()
}

func (r *preparedTextCallbackRows) Conn() *pgx.Conn {
	if r.mode == "nil_conn" {
		return nil
	}
	return r.Rows.Conn()
}

func (r *preparedTextCallbackRows) Scan(dest ...any) error {
	if r.scans == 0 && len(dest) == 15 {
		copy(r.original[:], dest)
		copy(r.dest[:], dest)
		count := 0
		for i := range r.callbacks {
			scanner, ok := dest[i+4].(pgtype.BytesScanner)
			if !ok {
				continue
			}
			count++
			if _, text := dest[i+4].(pgtype.TextScanner); text {
				r.problem = "optimized destination also implements TextScanner"
			}
			r.callbacks[i] = preparedTextForwardCallback{scanner: scanner, calls: &r.calls[i]}
			r.dest[i+4] = &r.callbacks[i]
		}
		r.optimized = count == 7
		if count != 0 && count != 7 {
			r.problem = "only some text slots use BytesScanner"
		}
	}
	r.stable = r.stable && slices.Equal(dest, r.original[:])
	if r.optimized {
		return r.linksCollectorRows.Scan(r.dest[:]...)
	}
	return r.linksCollectorRows.Scan(dest...)
}

func (r *preparedTextCallbackRows) checkCallbacks(t *testing.T, n int, optimized bool) {
	t.Helper()
	r.check(t, n, 1, n, 1)
	want := 0
	if optimized {
		want = n
	}
	if r.optimized != optimized || r.problem != "" {
		t.Fatalf("native callback path=%t want=%t problem=%s", r.optimized, optimized, r.problem)
	}
	for i, count := range r.calls {
		if count != want {
			t.Fatalf("native text callback %d count=%d want=%d", i, count, want)
		}
	}
}

type preparedTextForwardCallback struct {
	scanner pgtype.BytesScanner
	calls   *int
}

func (c *preparedTextForwardCallback) ScanBytes(src []byte) error {
	*c.calls++
	return c.scanner.ScanBytes(src)
}
