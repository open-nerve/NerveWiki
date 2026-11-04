package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

const (
	uuidScanTextA = "00112233-4455-6677-8899-aabbccddeeff"
	uuidScanTextB = "fedcba98-7654-3210-ffff-001122334455"
)

// Values are independent of the UUID decoder used by the pool.
func uuidScanValues() (a, b, max uuid.UUID) {
	return uuid.UUID{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		uuid.UUID{0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10, 0xff, 0xff, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		uuid.UUID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
}

func TestPoolUUIDScansPreserveValuesAndConnections(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t, pgtest.NewEmptyDatabase(t))
	conns := make([]*pgxpool.Conn, pool.Config().MaxConns)
	t.Cleanup(func() {
		for _, c := range conns {
			if c != nil {
				c.Release()
			}
		}
	})
	// Keep every lease: each check must reach a different physical connection.
	for i := range conns {
		c, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		conns[i] = c
		uuidScanPoolPolicy(t, c.Conn(), uuid.UUID{15: byte(i + 1)})
	}
	for _, format := range []int16{pgtype.TextFormatCode, pgtype.BinaryFormatCode} {
		t.Run(fmt.Sprintf("format_%d", format), func(t *testing.T) {
			uuidScanWire(t, conns[0].Conn(), format)
		})
	}
	own := conns[0].Hijack()
	conns[0] = nil
	t.Cleanup(func() {
		if err := own.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	uuidScanPoolPolicy(t, own, uuid.UUID{15: 9})
}

func uuidScanPoolPolicy(t *testing.T, conn *pgx.Conn, want uuid.UUID) {
	t.Helper()
	var got uuid.UUID
	var instant time.Time
	var jit string
	err := conn.QueryRow(context.Background(),
		"SELECT $1::uuid, '2026-09-25 18:00:00+08'::timestamptz, current_setting('jit')", want).Scan(&got, &instant, &jit)
	if err != nil || got != want || jit != "off" || instant.Location() != time.UTC ||
		!instant.Equal(time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("pool policy: UUID %v, timestamp %v, jit %q, error %v", got, instant, jit, err)
	}
}

func uuidScanWire(t *testing.T, conn *pgx.Conn, format int16) {
	t.Helper()
	ctx := context.Background()
	a, b, max := uuidScanValues()
	zero := uuid.UUID{}
	formats := pgx.QueryResultFormatsByOID{pgtype.UUIDOID: format, pgtype.UUIDArrayOID: format}
	rows, err := conn.Query(ctx, `SELECT id, maybe FROM (VALUES
 (1, '00112233-4455-6677-8899-aabbccddeeff'::uuid, '00000000-0000-0000-0000-000000000000'::uuid),
 (2, 'fedcba98-7654-3210-ffff-001122334455'::uuid, NULL::uuid),
 (3, '00000000-0000-0000-0000-000000000000'::uuid, '00112233-4455-6677-8899-aabbccddeeff'::uuid),
 (4, 'ffffffff-ffff-ffff-ffff-ffffffffffff'::uuid, 'fedcba98-7654-3210-ffff-001122334455'::uuid),
 (5, 'ffffffff-ffff-ffff-ffff-ffffffffffff'::uuid, '00112233-4455-6677-8899-aabbccddeeff'::uuid),
 (6, NULL::uuid, 'fedcba98-7654-3210-ffff-001122334455'::uuid)
) AS t(n, id, maybe) ORDER BY n`, formats)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	uuidScanFields(t, rows, format, pgtype.UUIDOID, pgtype.UUIDOID)
	ids := [4]uuid.UUID{max, max, max, a}
	seed0, seed1, seed2, seed3 := max, max, max, max
	maybe := [4]*uuid.UUID{&seed0, &seed1, &seed2, &seed3}
	wantIDs := [4]uuid.UUID{a, b, zero, max}
	wantMaybe := [4]*uuid.UUID{&zero, nil, &a, &b}
	texts := [4]string{uuidScanTextA, uuidScanTextB, "00000000-0000-0000-0000-000000000000", "ffffffff-ffff-ffff-ffff-ffffffffffff"}
	maybeTexts := [4]string{texts[2], "", uuidScanTextA, uuidScanTextB}
	for i := range ids {
		if !rows.Next() {
			t.Fatalf("row %d missing: %v", i, rows.Err())
		}
		wantRaw := []byte(texts[i])
		if format == pgtype.BinaryFormatCode {
			wantRaw = wantIDs[i][:]
		}
		uuidScanEqual(t, "raw UUID", rows.RawValues()[0], wantRaw)
		var wantMaybeRaw []byte
		if wantMaybe[i] != nil {
			wantMaybeRaw = []byte(maybeTexts[i])
			if format == pgtype.BinaryFormatCode {
				wantMaybeRaw = wantMaybe[i][:]
			}
		}
		uuidScanEqual(t, "raw nullable UUID", rows.RawValues()[1], wantMaybeRaw)
		if err := rows.Scan(&ids[i], &maybe[i]); err != nil {
			t.Fatal(err)
		}
	}
	// The same cached Rows plans have now written four different addresses.
	uuidScanEqual(t, "retained UUIDs", ids, wantIDs)
	uuidScanEqual(t, "nullable values/zero/NULL", maybe, wantMaybe)
	if !rows.Next() {
		t.Fatalf("type-change row missing: %v", rows.Err())
	}
	var typed pgtype.UUID
	var last *uuid.UUID
	if err := rows.Scan(&typed, &last); err != nil {
		t.Fatal(err)
	}
	uuidScanEqual(t, "changed destination type", typed, pgtype.UUID{Bytes: [16]byte(max), Valid: true})
	uuidScanEqual(t, "nullable ordinary UUID", last, &a)
	if !rows.Next() {
		t.Fatalf("NULL row missing: %v", rows.Err())
	}
	failed, untouchedSeed := max, max
	untouched := &untouchedSeed
	err = rows.Scan(&failed, &untouched)
	baseline := max
	cause := pgtype.NewMap().Scan(pgtype.UUIDOID, format, nil, &baseline)
	var column pgx.ScanArgError
	if cause == nil || err == nil || !errors.As(err, &column) || column.ColumnIndex != 0 || column.FieldName != "id" ||
		reflect.TypeOf(column.Err) != reflect.TypeOf(cause) || column.Err.Error() != cause.Error() ||
		failed != max || untouched != &untouchedSeed || untouchedSeed != max {
		t.Fatalf("NULL Scan error/state: %v, column %#v, first %v, second %v; baseline %v", err, column, failed, untouched, cause)
	}
	rows.Close()
	var terminal pgx.ScanArgError
	if rows.Err() == nil || rows.Err().Error() != err.Error() || !errors.As(rows.Err(), &terminal) ||
		terminal.ColumnIndex != 0 || terminal.FieldName != "id" || terminal.Err == nil || terminal.Err.Error() != cause.Error() {
		t.Fatalf("Rows must retain the Scan error: %v", rows.Err())
	}
	// A failed scan leaves this same acquired connection usable; also retain
	// parameter encoding and unrelated integer/text scans.
	var recovered uuid.UUID
	var encodedArray []uuid.UUID
	var number int32
	var text string
	wantArray := []uuid.UUID{zero, a, max, a}
	err = conn.QueryRow(ctx, "SELECT $1::uuid, $2::uuid[], $3::int4, $4::text",
		formats, b, wantArray, int32(37), "after UUID scan error").Scan(&recovered, &encodedArray, &number, &text)
	if err != nil || recovered != b || number != 37 || text != "after UUID scan error" {
		t.Fatalf("same-connection recovery/encoding: %v, %v, %d, %q", err, recovered, number, text)
	}
	uuidScanEqual(t, "encoded UUID array", encodedArray, wantArray)

	arrays, err := conn.Query(ctx, `SELECT
 ARRAY['00112233-4455-6677-8899-aabbccddeeff'::uuid, '00000000-0000-0000-0000-000000000000'::uuid, 'ffffffff-ffff-ffff-ffff-ffffffffffff'::uuid, '00112233-4455-6677-8899-aabbccddeeff'::uuid],
 ARRAY['fedcba98-7654-3210-ffff-001122334455'::uuid, NULL::uuid, '00000000-0000-0000-0000-000000000000'::uuid],
 ARRAY['00112233-4455-6677-8899-aabbccddeeff'::uuid, 'fedcba98-7654-3210-ffff-001122334455'::uuid],
 ARRAY[ARRAY['00112233-4455-6677-8899-aabbccddeeff'::uuid, '00000000-0000-0000-0000-000000000000'::uuid], ARRAY['ffffffff-ffff-ffff-ffff-ffffffffffff'::uuid, 'fedcba98-7654-3210-ffff-001122334455'::uuid]],
 '[0:1]={00112233-4455-6677-8899-aabbccddeeff,fedcba98-7654-3210-ffff-001122334455}'::uuid[],
 NULL::uuid[], '{}'::uuid[]`, formats)
	if err != nil {
		t.Fatal(err)
	}
	defer arrays.Close()
	uuidScanFields(t, arrays, format, pgtype.UUIDArrayOID, pgtype.UUIDArrayOID, pgtype.UUIDArrayOID,
		pgtype.UUIDArrayOID, pgtype.UUIDArrayOID, pgtype.UUIDArrayOID, pgtype.UUIDArrayOID)
	if !arrays.Next() {
		t.Fatalf("array row missing: %v", arrays.Err())
	}
	var flat []uuid.UUID
	var nullable []*uuid.UUID
	var fixed [2]uuid.UUID
	var matrix [][]uuid.UUID
	var bounded pgtype.Array[uuid.UUID]
	absent, empty := []uuid.UUID{max}, []uuid.UUID{max}
	if err := arrays.Scan(&flat, &nullable, &fixed, &matrix, &bounded, &absent, &empty); err != nil {
		t.Fatal(err)
	}
	uuidScanEqual(t, "array order/duplicates/zero", flat, []uuid.UUID{a, zero, max, a})
	uuidScanEqual(t, "nullable elements", nullable, []*uuid.UUID{&b, nil, &zero})
	uuidScanEqual(t, "fixed array", fixed, [2]uuid.UUID{a, b})
	uuidScanEqual(t, "multidimensional array", matrix, [][]uuid.UUID{{a, zero}, {max, b}})
	uuidScanEqual(t, "lower bounds", bounded, pgtype.Array[uuid.UUID]{Elements: []uuid.UUID{a, b}, Dims: []pgtype.ArrayDimension{{Length: 2, LowerBound: 0}}, Valid: true})
	uuidScanEqual(t, "NULL array", absent, ([]uuid.UUID)(nil))
	uuidScanEqual(t, "empty array", empty, []uuid.UUID{})
	if arrays.Next() || arrays.Err() != nil {
		t.Fatalf("array row termination: %v", arrays.Err())
	}
}

// The pool map is obtained from a real NewPool connection. A pristine pgx map
// supplies the compatibility oracle for errors, panics and partial writes.
func TestPoolUUIDScansMatchPGXOnExceptionalInputs(t *testing.T) {
	pool := newPool(t, pgtest.NewEmptyDatabase(t))
	lease, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	gotMap, original := lease.Conn().TypeMap(), pgtype.NewMap()
	a, _, max := uuidScanValues()
	zero := uuid.UUID{}
	scalar := func() any { v := max; return &v }
	nullable := func() any { v := max; p := &v; return &p }
	payloads := []struct {
		name   string
		format int16
		src    []byte
		bad    bool
	}{
		{"binary_null", pgtype.BinaryFormatCode, nil, false},
		{"binary_zero", pgtype.BinaryFormatCode, zero[:], false},
		{"binary_value", pgtype.BinaryFormatCode, a[:], false},
		{"binary_short", pgtype.BinaryFormatCode, a[:15], true},
		{"binary_long", pgtype.BinaryFormatCode, append(append([]byte(nil), a[:]...), 0), true},
		{"text_null", pgtype.TextFormatCode, nil, false},
		{"text_dashed", pgtype.TextFormatCode, []byte(uuidScanTextA), false},
		{"text_compact", pgtype.TextFormatCode, []byte("00112233445566778899aabbccddeeff"), false},
		{"text_separators", pgtype.TextFormatCode, []byte("00112233_4455_6677_8899_aabbccddeeff"), false},
		{"text_empty", pgtype.TextFormatCode, []byte{}, true},
		{"text_bad_hex", pgtype.TextFormatCode, []byte("g0112233-4455-6677-8899-aabbccddeeff"), true},
		{"text_braces", pgtype.TextFormatCode, []byte("{" + uuidScanTextA + "}"), true},
		{"text_urn", pgtype.TextFormatCode, []byte("urn:uuid:" + uuidScanTextA), true},
	}
	for _, tc := range payloads {
		for _, ptr := range []struct {
			name     string
			make     func() any
			nullable bool
		}{{"scalar", scalar, false}, {"nullable", nullable, true}} {
			t.Run(tc.name+"/"+ptr.name, func(t *testing.T) {
				out := uuidCompareMapScan(t, gotMap, original, pgtype.UUIDOID, tc.format, tc.src, ptr.make)
				wantError := tc.bad || (tc.src == nil && !ptr.nullable)
				if out.PanicType != "" || (out.Error != "") != wantError {
					t.Fatalf("unexpected pgx scalar outcome: %+v, want error %v", out, wantError)
				}
			})
		}
	}
	for _, format := range []int16{pgtype.TextFormatCode, pgtype.BinaryFormatCode} {
		src := []byte(uuidScanTextA)
		if format == pgtype.BinaryFormatCode {
			src = a[:]
		}
		for _, target := range []struct {
			name string
			make func() any
		}{
			{"untyped_nil", func() any { return nil }},
			{"typed_nil", func() any { return (*uuid.UUID)(nil) }},
			{"typed_double_nil", func() any { return (**uuid.UUID)(nil) }},
			{"byte16", func() any { v := [16]byte(max); return &v }},
			{"defined_byte16", func() any { v := uuidScanOther(max); return &v }},
			{"pgtype_UUID", func() any { return &pgtype.UUID{Bytes: [16]byte(max), Valid: true} }},
			{"string", func() any { v := "seed"; return &v }},
			{"sql_scanner", func() any { return &uuidScanSQLScanner{Value: "seed"} }},
			{"uuid_scanner", func() any { return &uuidScanUUIDScanner{Value: pgtype.UUID{Bytes: [16]byte(max), Valid: true}} }},
		} {
			for _, null := range []bool{false, true} {
				t.Run(fmt.Sprintf("targets_%d/%s/null_%t", format, target.name, null), func(t *testing.T) {
					input := src
					if null {
						input = nil
					}
					uuidCompareMapScan(t, gotMap, original, pgtype.UUIDOID, format, input, target.make)
				})
			}
		}
		gotPlan := gotMap.PlanScan(pgtype.UUIDOID, format, new(uuid.UUID))
		wantPlan := original.PlanScan(pgtype.UUIDOID, format, new(uuid.UUID))
		if gotPlan == nil || wantPlan == nil {
			t.Fatal("UUID plan missing")
		}
		for _, target := range []struct {
			name string
			make func() any
		}{
			{"byte16", func() any { v := [16]byte(max); return &v }},
			{"defined_byte16", func() any { v := uuidScanOther(max); return &v }},
			{"wrong_string", func() any { v := "seed"; return &v }},
			{"nil", func() any { return nil }},
		} {
			t.Run(fmt.Sprintf("direct_plan_%d/%s", format, target.name), func(t *testing.T) {
				got, want := target.make(), target.make()
				gotOut := uuidScanCapture(func() error { return gotPlan.Scan(src, got) })
				wantOut := uuidScanCapture(func() error { return wantPlan.Scan(src, want) })
				uuidScanEqual(t, "direct-plan outcome", gotOut, wantOut)
				uuidScanEqual(t, "direct-plan destination", got, want)
			})
		}
		for _, value := range []struct {
			name  string
			oid   uint32
			value any
		}{
			{"scalar", pgtype.UUIDOID, a},
			{"array", pgtype.UUIDArrayOID, []uuid.UUID{a, zero, max}},
			{"unknown_array", 0, []uuid.UUID{a}},
		} {
			t.Run(fmt.Sprintf("encode_%d/%s", format, value.name), func(t *testing.T) {
				var got, want []byte
				gotOut := uuidScanCapture(func() error { var e error; got, e = gotMap.Encode(value.oid, format, value.value, nil); return e })
				wantOut := uuidScanCapture(func() error { var e error; want, e = original.Encode(value.oid, format, value.value, nil); return e })
				uuidScanEqual(t, "encode outcome", gotOut, wantOut)
				uuidScanEqual(t, "encoded bytes", got, want)
				if gotOut.PanicType != "" || (gotOut.Error != "") != (value.oid == 0) {
					t.Fatalf("encode outcome %+v, unknown OID %v", gotOut, value.oid == 0)
				}
			})
		}
	}
	for _, tc := range []struct {
		name   string
		oid    uint32
		format int16
		src    []byte
	}{
		{"int4", pgtype.Int4OID, pgtype.BinaryFormatCode, []byte{0, 0, 0, 7}},
		{"text", pgtype.TextOID, pgtype.TextFormatCode, []byte(uuidScanTextA)},
		{"unknown", 0, pgtype.TextFormatCode, []byte(uuidScanTextA)},
		{"invalid_format", pgtype.UUIDOID, 7, a[:]},
	} {
		t.Run("other_oid_or_format/"+tc.name, func(t *testing.T) {
			uuidCompareMapScan(t, gotMap, original, tc.oid, tc.format, tc.src, scalar)
		})
	}

	// Use real PostgreSQL array payloads as inputs, without writing a second
	// binary array encoder/decoder in the test.
	raw, rawNull := uuidScanRawArrays(t, lease.Conn())
	flat := func() any { v := []uuid.UUID{max}; return &v }
	pointerElements := func() any { v := max; s := []*uuid.UUID{&v}; return &s }
	bounded := func() any {
		return &pgtype.Array[uuid.UUID]{Elements: []uuid.UUID{max}, Dims: []pgtype.ArrayDimension{{Length: 1, LowerBound: 7}}, Valid: true}
	}
	for _, tc := range []struct {
		name      string
		format    int16
		src       []byte
		make      func() any
		wantError bool
	}{
		{"binary_header", pgtype.BinaryFormatCode, raw[:10], bounded, true},
		{"binary_tail", pgtype.BinaryFormatCode, raw[:len(raw)-1], flat, true},
		{"binary_trailing", pgtype.BinaryFormatCode, append(append([]byte(nil), raw...), 0), bounded, true},
		{"binary_null_element", pgtype.BinaryFormatCode, rawNull, flat, true},
		{"binary_nullable_element", pgtype.BinaryFormatCode, rawNull, pointerElements, false},
		{"text_bad_element", pgtype.TextFormatCode, []byte("{" + uuidScanTextA + ",not-a-uuid}"), flat, true},
		{"text_bad_nullable_element", pgtype.TextFormatCode, []byte("{" + uuidScanTextA + ",not-a-uuid}"), pointerElements, true},
		{"text_null_element", pgtype.TextFormatCode, []byte("{" + uuidScanTextA + ",NULL," + uuidScanTextB + "}"), flat, true},
		{"text_dimensions", pgtype.TextFormatCode, []byte("[0:2]={" + uuidScanTextA + "," + uuidScanTextB + "}"), bounded, true},
		{"text_fixed_length", pgtype.TextFormatCode, []byte("{" + uuidScanTextA + "," + uuidScanTextB + "," + uuidScanTextA + "}"), func() any { v := [2]uuid.UUID{max, max}; return &v }, true},
	} {
		t.Run("array_payload/"+tc.name, func(t *testing.T) {
			out := uuidCompareMapScan(t, gotMap, original, pgtype.UUIDArrayOID, tc.format, tc.src, tc.make)
			if out.PanicType != "" || (out.Error != "") != tc.wantError {
				t.Fatalf("unexpected pgx array outcome: %+v, want error %v", out, tc.wantError)
			}
		})
	}
}

func uuidScanRawArrays(t *testing.T, conn *pgx.Conn) ([]byte, []byte) {
	t.Helper()
	rows, err := conn.Query(context.Background(), `SELECT
 ARRAY['00112233-4455-6677-8899-aabbccddeeff'::uuid, 'fedcba98-7654-3210-ffff-001122334455'::uuid],
 ARRAY['00112233-4455-6677-8899-aabbccddeeff'::uuid, NULL::uuid, 'fedcba98-7654-3210-ffff-001122334455'::uuid]`,
		pgx.QueryResultFormats{pgtype.BinaryFormatCode, pgtype.BinaryFormatCode})
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	uuidScanFields(t, rows, pgtype.BinaryFormatCode, pgtype.UUIDArrayOID, pgtype.UUIDArrayOID)
	if !rows.Next() {
		t.Fatalf("binary array payloads missing: %v", rows.Err())
	}
	raw := append([]byte(nil), rows.RawValues()[0]...)
	rawNull := append([]byte(nil), rows.RawValues()[1]...)
	if len(raw) < 11 || len(rawNull) < 11 || rows.Next() || rows.Err() != nil {
		t.Fatalf("invalid array payload source/termination: %d, %d, %v", len(raw), len(rawNull), rows.Err())
	}
	return raw, rawNull
}

func uuidScanFields(t *testing.T, rows pgx.Rows, format int16, oids ...uint32) {
	t.Helper()
	fields := rows.FieldDescriptions()
	if len(fields) != len(oids) {
		t.Fatalf("field count %d, want %d", len(fields), len(oids))
	}
	for i, oid := range oids {
		if fields[i].DataTypeOID != oid || fields[i].Format != format {
			t.Fatalf("field %d: OID/format %d/%d, want %d/%d", i, fields[i].DataTypeOID, fields[i].Format, oid, format)
		}
	}
}

func uuidScanEqual(t *testing.T, label string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: got %#v, want %#v", label, got, want)
	}
}

type uuidScanOutcome struct {
	ErrorType string
	Error     string
	PanicType string
	Panic     string
}

func uuidScanCapture(run func() error) (out uuidScanOutcome) {
	defer func() {
		if p := recover(); p != nil {
			out.PanicType, out.Panic = fmt.Sprintf("%T", p), fmt.Sprint(p)
		}
	}()
	if err := run(); err != nil {
		out.ErrorType, out.Error = fmt.Sprintf("%T", err), err.Error()
	}
	return out
}

func uuidCompareMapScan(t *testing.T, gotMap, original *pgtype.Map, oid uint32, format int16, src []byte, makeTarget func() any) uuidScanOutcome {
	t.Helper()
	got, want := makeTarget(), makeTarget()
	gotOut := uuidScanCapture(func() error { return gotMap.Scan(oid, format, src, got) })
	wantOut := uuidScanCapture(func() error { return original.Scan(oid, format, src, want) })
	uuidScanEqual(t, "map scan outcome", gotOut, wantOut)
	uuidScanEqual(t, "map scan destination", got, want)
	return gotOut
}

type uuidScanOther [16]byte

type uuidScanSQLScanner struct {
	Value any
	Calls int
}

func (s *uuidScanSQLScanner) Scan(value any) error {
	s.Value, s.Calls = value, s.Calls+1
	return nil
}

type uuidScanUUIDScanner struct {
	Value pgtype.UUID
	Calls int
}

func (s *uuidScanUUIDScanner) ScanUUID(value pgtype.UUID) error {
	s.Value, s.Calls = value, s.Calls+1
	return nil
}
