package postgresadapter

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

const (
	preparedTextCopyRows     = 4096
	preparedTextCopyStride   = 1024
	preparedTextCopyProperty = "pppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppppp"
	preparedTextCopyTarget   = "TTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTTT.md"
	preparedTextCopyAnchor   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	preparedTextCopyDisplay  = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	preparedTextCopyKey      = "tttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttt"
	preparedTextCopyAlt      = "tttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttttt.md"
)

// Restoring a string copy for each native comparison column must exceed
// this fixed ceiling. The seed, incoming facts, Keys and complete warm-up
// are outside TotalAlloc; the one measured call includes the real query,
// all 4096 native rows, terminal Err, Close and every collector output.
func TestPreparedSourceDoesNotCopyTextForComparison(t *testing.T) {
	const maxAllocatedBytes uint64 = 2 << 20
	pool, trace := newLinksCollectorPool(t)
	trace.sql = gen.InspectPageLinksForPreparation
	p := app.Page{ID: uuid.UUID{15: 50}, NotebookID: uuid.UUID{15: 1}, Revision: 1}
	firstTarget, lastTarget := uuid.UUID{15: 11}, uuid.UUID{15: 12}

	// The six long strings are fixed literals. The target is a legal
	// 243-byte .md spelling with two independently specified title keys.
	if preparedTextCopyRows > domain.MaxLinks || len(preparedTextCopyTarget) != 243 ||
		len(preparedTextCopyKey) != 240 || len(preparedTextCopyAlt) != 243 ||
		len(preparedTextCopyKey) > domain.MaxKey || len(preparedTextCopyAlt) > domain.MaxKey {
		t.Fatal("PREPARED_TEXT_FIXTURE: fixture no longer fits the existing input/key bounds")
	}
	facts := domain.Facts{FrontmatterValid: true, Links: make([]domain.Link, preparedTextCopyRows)}
	wantInputs := make([]app.Link, preparedTextCopyRows)
	data := make([][]any, preparedTextCopyRows)
	for i := range preparedTextCopyRows {
		start := i * preparedTextCopyStride
		resolved := firstTarget
		if i == preparedTextCopyRows-1 {
			resolved = lastTarget
		}
		facts.Links[i] = domain.Link{Kind: "wikilink", Property: preparedTextCopyProperty,
			Target: preparedTextCopyTarget, Anchor: preparedTextCopyAnchor, Display: preparedTextCopyDisplay,
			Start: start, End: start + 243, Aliases: true}
		wantInputs[i] = app.Link{SourceID: p.ID, Start: start, Target: preparedTextCopyTarget,
			Aliases: true, Resolution: domain.Resolution{ID: resolved, Ambiguous: true, Asset: true}}
		// This independent COPY does not call AddPage, linksParams, Keys or
		// either comparison function to derive the persisted column values.
		data[preparedTextCopyRows-1-i] = []any{p.ID, int32(start), int32(start + 243), p.NotebookID,
			"wikilink", preparedTextCopyProperty, preparedTextCopyTarget, preparedTextCopyAnchor,
			preparedTextCopyDisplay, preparedTextCopyKey, preparedTextCopyAlt, true, resolved, true, true}
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO indexed_pages
    (node_id, notebook_id, revision, extractor, frontmatter_valid)
VALUES ($1, $2, 1, 2, true)`, p.ID, p.NotebookID); err != nil {
		t.Fatal(err)
	}
	copied, err := pool.CopyFrom(t.Context(), pgx.Identifier{"page_links"}, []string{
		"source_id", "range_start", "range_end", "notebook_id", "kind", "property_key",
		"target", "anchor", "display", "target_key", "target_alt_key", "aliases",
		"resolved_id", "ambiguous", "resolved_asset",
	}, pgx.CopyFromRows(data))
	if err != nil || copied != preparedTextCopyRows {
		t.Fatalf("PREPARED_TEXT_FIXTURE: COPY rows=%d, want %d: %v", copied, preparedTextCopyRows, err)
	}

	incoming := newPreparedLinkFacts(p, facts.Links)
	original := slices.Clone(facts.Links)
	if len(incoming.keys) != preparedTextCopyRows || len(incoming.alt) != preparedTextCopyRows {
		t.Fatal("PREPARED_TEXT_FIXTURE: incomplete incoming Keys")
	}
	checkIncoming := func() {
		t.Helper()
		if !slices.Equal(facts.Links, original) {
			t.Fatal("PREPARED_TEXT_ORACLE: caller facts changed")
		}
		for i := range preparedTextCopyRows {
			if incoming.keys[i] != preparedTextCopyKey || incoming.alt[i] != preparedTextCopyAlt {
				t.Fatalf("PREPARED_TEXT_ORACLE: incoming Keys[%d] differ from the literal keys", i)
			}
		}
	}
	checkIncoming()
	wantDelta := gen.InsertLinksParams{SourceID: p.ID, NotebookID: p.NotebookID,
		RangeStarts: []int32{}, RangeEnds: []int32{}, Kinds: []string{}, PropertyKeys: []string{},
		InFrontmatter: []bool{}, Targets: []string{}, Anchors: []string{}, Displays: []string{},
		TargetKeys: []string{}, TargetAltKeys: []string{}, Aliases: []bool{}}
	checkRead := func(read preparedLinkRead) {
		t.Helper()
		if !read.canRetain || !reflect.DeepEqual(read.links, wantDelta) || read.missing != nil ||
			read.assets == nil || len(read.assets) != 0 || cap(read.assets) != 0 ||
			!slices.Equal(read.targets, []uuid.UUID{firstTarget, lastTarget}) ||
			!slices.Equal(read.inputs, wantInputs) {
			t.Fatalf("PREPARED_TEXT_ORACLE: retain=%t delta=%+v missing=%v F=%v targets=%v inputs=%d",
				read.canRetain, read.links, read.missing, read.assets, read.targets, len(read.inputs))
		}
		// An unchanged nonempty source owns eleven nonnil exact-empty
		// delta arrays; no full-sized backing array is retained.
		values := reflect.ValueOf(read.links)
		for i := 0; i < values.NumField(); i++ {
			v := values.Field(i)
			if v.Kind() == reflect.Slice && (v.IsNil() || v.Len() != 0 || v.Cap() != 0) {
				t.Fatalf("PREPARED_TEXT_ORACLE: delta %s is not nonnil exact-empty", values.Type().Field(i).Name)
			}
		}
	}
	wantText := [7][]byte{[]byte("wikilink"), []byte(preparedTextCopyProperty), []byte(preparedTextCopyTarget),
		[]byte(preparedTextCopyAnchor), []byte(preparedTextCopyDisplay), []byte(preparedTextCopyKey), []byte(preparedTextCopyAlt)}
	arg := gen.InspectPageLinksForPreparationParams{SourceID: p.ID, NotebookID: p.NotebookID, Extractor: domain.Extractor}
	conn, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	err = postgres.NewTxManager(pool, time.Second).WithinTx(postgres.OnConn(t.Context(), conn), func(ctx context.Context) error {
		if err := New(pool).Lock(ctx, p.NotebookID); err != nil {
			return err
		}
		tx, _ := postgres.TxFrom(ctx)
		newObserver := func() (*linksCollectorQuerier, *preparedTextCopyNativeRows) {
			native := &preparedTextCopyNativeRows{conn: tx.Conn(), text: wantText}
			rows := &linksCollectorRows{stable: true, columns: 15}
			return &linksCollectorQuerier{
				Querier: &preparedTextCopyQuerier{Querier: tx, rows: native}, rows: rows,
			}, native
		}
		checkObserved := func(q *linksCollectorQuerier, native *preparedTextCopyNativeRows) {
			t.Helper()
			q.rows.check(t, preparedTextCopyRows, 1, preparedTextCopyRows, 1)
			if q.calls != 1 || q.cause != nil || native.seen != preparedTextCopyRows || native.problem != "" {
				t.Fatalf("PREPARED_TEXT_NATIVE: queries=%d cause=%v rows=%d problem=%s",
					q.calls, q.cause, native.seen, native.problem)
			}
			retentionCollectorCheckTrace(t, trace, tx.Conn().PgConn().PID(), p, nil)
		}

		// Warm the exact SQL, native codec plans and network buffers on
		// the same physical connection before opening the allocation window.
		warmQuery, warmNative := newObserver()
		warm, err := inspectPageLinksForPreparation(ctx, warmQuery, arg, &incoming)
		if err != nil {
			return err
		}
		checkObserved(warmQuery, warmNative)
		checkRead(warm)
		checkIncoming()

		measuredQuery, measuredNative := newObserver()
		calls := trace.count()
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		read, err := inspectPageLinksForPreparation(ctx, measuredQuery, arg, &incoming)
		runtime.ReadMemStats(&after)
		allocated := after.TotalAlloc - before.TotalAlloc
		if err != nil {
			return err
		}

		// Every semantic/native assertion precedes the allocation failure.
		checkObserved(measuredQuery, measuredNative)
		if trace.count() != calls+1 {
			t.Fatal("PREPARED_TEXT_NATIVE: measured call did not run exactly one original query")
		}
		checkRead(read)
		checkIncoming()
		linksCollectorReusable(t, conn)
		var overwrite string
		if err := conn.QueryRow(ctx, "SELECT repeat('x', 65536)").Scan(&overwrite); err != nil {
			return err
		}
		if len(overwrite) != 65536 {
			t.Fatal("PREPARED_TEXT_ORACLE: connection reuse did not return the overwrite payload")
		}
		checkRead(read)
		checkIncoming()
		t.Logf("prepared comparison: native rows=%d, literal outputs and connection reuse verified, TotalAlloc=%d bytes, fixed ceiling=%d bytes",
			measuredNative.seen, allocated, maxAllocatedBytes)
		if allocated > maxAllocatedBytes {
			t.Fatalf("PREPARED_TEXT_COPY: complete query+collector allocated %d bytes, fixed maximum %d for %d native rows",
				allocated, maxAllocatedBytes, preparedTextCopyRows)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The extra observer reads the native row while it is valid and forwards
// every operation unchanged. It never returns synthetic rows, changes SQL,
// replaces destinations/results, or retains raw bytes beyond Next.
type preparedTextCopyQuerier struct {
	postgres.Querier
	rows *preparedTextCopyNativeRows
}

func (q *preparedTextCopyQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := q.Querier.Query(ctx, sql, args...)
	if rows != nil {
		q.rows.Rows = rows
		return q.rows, err
	}
	return rows, err
}

type preparedTextCopyNativeRows struct {
	pgx.Rows
	conn    *pgx.Conn
	text    [7][]byte
	seen    int
	problem string
}

func (r *preparedTextCopyNativeRows) Next() bool {
	ok := r.Rows.Next()
	if !ok {
		return false
	}
	r.seen++
	if r.problem != "" {
		return true
	}
	if r.seen == 1 {
		// These are the original 15 column names, OIDs and native default
		// wire formats, checked while FieldDescriptions is still valid.
		want := [...]struct {
			name   string
			oid    uint32
			format int16
		}{
			{"can_retain", pgtype.BoolOID, pgtype.BinaryFormatCode},
			{"range_start", pgtype.Int4OID, pgtype.BinaryFormatCode},
			{"range_end", pgtype.Int4OID, pgtype.BinaryFormatCode},
			{"notebook_id", pgtype.UUIDOID, pgtype.BinaryFormatCode},
			{"kind", pgtype.TextOID, pgtype.TextFormatCode},
			{"property_key", pgtype.TextOID, pgtype.TextFormatCode},
			{"target", pgtype.TextOID, pgtype.TextFormatCode},
			{"anchor", pgtype.TextOID, pgtype.TextFormatCode},
			{"display", pgtype.TextOID, pgtype.TextFormatCode},
			{"target_key", pgtype.TextOID, pgtype.TextFormatCode},
			{"target_alt_key", pgtype.TextOID, pgtype.TextFormatCode},
			{"aliases", pgtype.BoolOID, pgtype.BinaryFormatCode},
			{"resolved_id", pgtype.UUIDOID, pgtype.BinaryFormatCode},
			{"ambiguous", pgtype.BoolOID, pgtype.BinaryFormatCode},
			{"resolved_asset", pgtype.BoolOID, pgtype.BinaryFormatCode},
		}
		fields := r.FieldDescriptions()
		if len(fields) != len(want) || r.Conn() != r.conn || r.TypeMap() != r.conn.TypeMap() {
			r.problem = "native descriptor count, connection or TypeMap identity differs"
			return true
		}
		for i, field := range fields {
			if field.Name != want[i].name || field.DataTypeOID != want[i].oid || field.Format != want[i].format {
				r.problem = fmt.Sprintf("native descriptor %d: name=%q OID=%d format=%d", i, field.Name, field.DataTypeOID, field.Format)
				return true
			}
		}
		typ, exists := r.TypeMap().TypeForOID(pgtype.TextOID)
		if !exists || typ.Name != "text" || typ.OID != pgtype.TextOID {
			r.problem = "native TEXT type is absent or has a different identity"
			return true
		}
		if _, standard := typ.Codec.(pgtype.TextCodec); !standard {
			r.problem = fmt.Sprintf("native TEXT codec is %T, want pgtype.TextCodec", typ.Codec)
			return true
		}
	}
	raw := r.RawValues()
	if len(raw) != 15 || r.seen > preparedTextCopyRows {
		r.problem = fmt.Sprintf("native raw row %d has %d columns", r.seen, len(raw))
		return true
	}
	start := uint32((r.seen - 1) * preparedTextCopyStride)
	book, resolved := uuid.UUID{15: 1}, uuid.UUID{15: 11}
	if r.seen == preparedTextCopyRows {
		resolved = uuid.UUID{15: 12}
	}
	// Every field is non-NULL in this fixture. Validate the raw binary
	// scalars independently; no production comparison defines this oracle.
	if len(raw[0]) != 1 || raw[0][0] != 1 ||
		len(raw[1]) != 4 || binary.BigEndian.Uint32(raw[1]) != start ||
		len(raw[2]) != 4 || binary.BigEndian.Uint32(raw[2]) != start+243 ||
		!bytes.Equal(raw[3], book[:]) ||
		len(raw[11]) != 1 || raw[11][0] != 1 || !bytes.Equal(raw[12], resolved[:]) ||
		len(raw[13]) != 1 || raw[13][0] != 1 || len(raw[14]) != 1 || raw[14][0] != 1 {
		r.problem = fmt.Sprintf("native raw scalar mismatch at row %d", r.seen)
		return true
	}
	for i, want := range r.text {
		if raw[i+4] == nil || !bytes.Equal(raw[i+4], want) {
			r.problem = fmt.Sprintf("native raw text mismatch at row %d, column %d", r.seen, i+4)
			return true
		}
	}
	return true
}
