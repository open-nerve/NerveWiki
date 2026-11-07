package postgresadapter

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math"
	"reflect"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// These literals catch wrong array element OIDs, NULL/empty conflation,
// rune lengths, sign extension and corruption of a preceding parameter.
func TestRetentionArrayEncodingLiteralBytes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		oid   uint32
		value any
		want  string
		null  bool
	}{
		{"int_null", pgtype.Int4ArrayOID, []int32(nil), "", true},
		{"int_empty", pgtype.Int4ArrayOID, []int32{}, "0000000100000000000000170000000000000001", false},
		{"int_bounds", pgtype.Int4ArrayOID, []int32{0, -1, math.MinInt32, math.MaxInt32}, "0000000100000000000000170000000400000001000000040000000000000004ffffffff0000000480000000000000047fffffff", false},
		{"bool_null", pgtype.BoolArrayOID, []bool(nil), "", true},
		{"bool_empty", pgtype.BoolArrayOID, []bool{}, "0000000100000000000000100000000000000001", false},
		{"bool_values", pgtype.BoolArrayOID, []bool{false, true}, "000000010000000000000010000000020000000100000001000000000101", false},
		{"text_null", pgtype.TextArrayOID, []string(nil), "", true},
		{"text_empty", pgtype.TextArrayOID, []string{}, "0000000100000000000000190000000000000001", false},
		{"text_values", pgtype.TextArrayOID, []string{"", "雪", "a\"\\"}, "00000001000000000000001900000003000000010000000000000003e99baa0000000361225c", false},
		// PostgreSQL, not this encoder, still rejects invalid text bytes.
		{"text_invalid_bytes", pgtype.TextArrayOID, []string{"\x00", "\xff"}, "0000000100000000000000190000000200000001000000010000000001ff", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := pgtype.NewMap()
			restore := retentionArrayEncoding(m, 256)
			if restore == nil {
				t.Fatal("standard array codecs were not accepted")
			}
			defer restore()
			prefix := []byte{0xaa, 0x55}
			got, err := m.Encode(tc.oid, pgtype.BinaryFormatCode, tc.value, prefix)
			if err != nil {
				t.Fatal(err)
			}
			if tc.null {
				if got != nil {
					t.Fatalf("SQL NULL encoded as %x", got)
				}
			} else {
				want, err := hex.DecodeString("aa55" + tc.want)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("encoded %x, want literal %x", got, want)
				}
			}
			if !bytes.Equal(prefix, []byte{0xaa, 0x55}) {
				t.Fatal("prefix mutated")
			}
			original, err := pgtype.NewMap().Encode(tc.oid, pgtype.BinaryFormatCode, tc.value, []byte{0xaa, 0x55})
			if err != nil || !bytes.Equal(got, original) || (got == nil) != (original == nil) {
				t.Fatalf("pgx comparison=%x, %v; specialized=%x", original, err, got)
			}
		})
	}
}

func retentionArrayLiteralArgs() []any {
	return []any{uuid.UUID{}, uuid.UUID{15: 1}, []int32{2}, []int32{9}, []string{"link"}, []string{""}, []bool{false},
		[]string{"雪"}, []string{""}, []string{"a\"\\"}, []string{"雪"}, []string{""}, []bool{true}}
}

// This uses pgx's real builder, not a substitute encoder. 319 is the hand
// sum: 32 UUID bytes, 11*20 array headers, 54 element framing/fixed bytes,
// and 13 text bytes. Saved parameter views must survive the next Build.
func TestRetentionArrayEncodingOneArenaPreservesParameterViews(t *testing.T) {
	args := retentionArrayLiteralArgs()
	const sql = "-- name: WritePageLinksRetainingEqualPrefiltered :exec\nSELECT 1"
	budget, ok := retentionArrayBudget(sql, args)
	if !ok || budget != 319 {
		t.Fatalf("budget=%d/%t, want 319/true", budget, ok)
	}
	m := pgtype.NewMap()
	restore := retentionArrayEncoding(m, budget)
	if restore == nil {
		t.Fatal("standard codecs rejected")
	}
	defer restore()
	sd := &pgconn.StatementDescription{ParamOIDs: []uint32{2950, 2950, 1007, 1007, 1009, 1009, 1000, 1009, 1009, 1009, 1009, 1009, 1000}}
	var builder pgx.ExtendedQueryBuilder
	if err := builder.Build(m, sd, args); err != nil {
		t.Fatal(err)
	}
	wantLengths := []int{16, 16, 28, 28, 28, 24, 25, 27, 24, 27, 27, 24, 25}
	if len(builder.ParamValues) != len(wantLengths) {
		t.Fatal("parameter count changed")
	}
	arena := builder.ParamValues[2][:cap(builder.ParamValues[2])]
	if len(arena) != 287 {
		t.Fatalf("first array capacity=%d, want 319-32", len(arena))
	}
	saved := make([][]byte, len(wantLengths))
	views := make([][]byte, len(wantLengths))
	offset := 0
	for i, want := range wantLengths {
		v := builder.ParamValues[i]
		if len(v) != want || builder.ParamFormats[i] != 1 {
			t.Fatalf("parameter %d: %x format=%d", i, v, builder.ParamFormats[i])
		}
		if i >= 2 {
			if &arena[offset] != &v[0] {
				t.Fatalf("parameter %d has a different arena", i)
			}
			offset += want
		}
		saved[i], views[i] = bytes.Clone(v), v
	}
	if offset != 287 {
		t.Fatalf("array payload=%d", offset)
	}
	if !reflect.DeepEqual(args, retentionArrayLiteralArgs()) {
		t.Fatal("input args mutated")
	}
	next := retentionArrayLiteralArgs()
	next[7] = []string{"ABC"}
	if err := builder.Build(m, sd, next); err != nil {
		t.Fatal(err)
	}
	for i := range views {
		if !bytes.Equal(views[i], saved[i]) {
			t.Fatalf("next Build overwrote prior parameter %d", i)
		}
	}
}

func TestRetentionArrayEncodingBoundsAndExactShape(t *testing.T) {
	for _, tc := range []struct {
		count, width, limit, want int
		ok                        bool
	}{
		{0, 8, 20, 20, true}, {1, 8, 28, 28, true}, {1, 8, 27, 0, false},
		{1048573, 8, 8388608, 8388604, true}, {1048574, 8, 8388608, 0, false},
		{math.MaxInt, 8, 8 << 20, 0, false}, {math.MaxInt32, 8, 8 << 20, 0, false},
		{-1, 8, 20, 0, false}, {1, 0, 20, 0, false}, {0, 8, 19, 0, false},
	} {
		got, ok := retentionArrayFixedSize(tc.count, tc.width, tc.limit)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("size(%+v)=%d/%t", tc, got, ok)
		}
	}
	for _, tc := range []struct {
		sql  string
		args []any
		want int
		ok   bool
	}{
		{"-- name: DeleteMissingPageLinks :exec\nSELECT 1", []any{uuid.UUID{}, []int32(nil)}, 16, true},
		{"-- name: DeleteMissingPageLinks :exec\nSELECT 1", []any{uuid.UUID{}, []int32{}}, 36, true},
		{"-- name: DeleteMissingPageLinks :exec\nSELECT 1", []any{uuid.UUID{}, []int32{0, -1}}, 52, true},
		{"-- name: DeleteMissingPageLinksMore :exec\nSELECT 1", []any{uuid.UUID{}, []int32{1}}, 0, false},
		{"-- name: InsertLinks :exec\nSELECT 1", retentionArrayLiteralArgs(), 0, false},
		{"-- name: DeleteMissingPageLinks :exec\nSELECT 1", []any{uuid.UUID{}, []int64{1}}, 0, false},
		{"-- name: DeleteMissingPageLinks :exec\nSELECT 1", []any{uuid.UUID{}}, 0, false},
		{"-- name: DeleteMissingPageLinks :exec\nSELECT 1", []any{[16]byte{}, []int32{1}}, 0, false},
	} {
		got, ok := retentionArrayBudget(tc.sql, tc.args)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("budget(%q, %T)=%d/%t", tc.sql, tc.args, got, ok)
		}
	}
	m := pgtype.NewMap()
	restore := retentionArrayEncoding(m, 20)
	if restore == nil {
		t.Fatal("standard codecs rejected")
	}
	defer restore()
	// A value larger than the reserve must delegate, not invent an encoding error.
	got, err := m.Encode(pgtype.TextArrayOID, pgtype.BinaryFormatCode, []string{"longer"}, nil)
	want, e := pgtype.NewMap().Encode(pgtype.TextArrayOID, pgtype.BinaryFormatCode, []string{"longer"}, nil)
	if err != nil || e != nil || !bytes.Equal(got, want) {
		t.Fatalf("over-reserve fallback=%x/%v, want %x/%v", got, err, want, e)
	}
}

func TestRetentionArrayEncodingRestoresTypesAndMemo(t *testing.T) {
	m := pgtype.NewMap()
	oids := []uint32{pgtype.Int4ArrayOID, pgtype.TextArrayOID, pgtype.BoolArrayOID}
	values := []any{[]int32{1}, []string{"x"}, []bool{true}}
	before := make([]*pgtype.Type, len(oids))
	plans := make([]reflect.Type, len(oids))
	for i, oid := range oids {
		before[i], _ = m.TypeForOID(oid)
		plans[i] = reflect.TypeOf(m.PlanEncode(oid, 1, values[i]))
	}
	restore := retentionArrayEncoding(m, 256)
	if restore == nil {
		t.Fatal("standard codecs rejected")
	}
	for i, oid := range oids {
		if _, ok := m.PlanEncode(oid, 1, values[i]).(retentionArrayPlan); !ok {
			t.Fatalf("OID %d was not specialized", oid)
		}
	}
	restore()
	for i, oid := range oids {
		after, _ := m.TypeForOID(oid)
		if after != before[i] || reflect.TypeOf(m.PlanEncode(oid, 1, values[i])) != plans[i] {
			t.Fatalf("OID %d retained Type/encoding memo", oid)
		}
	}
	// A custom scalar codec is authoritative, even if the array still points
	// to the original scalar Type. It must prevent this optimization entirely.
	original, _ := m.TypeForOID(pgtype.TextOID)
	custom := &pgtype.Type{Name: original.Name, OID: original.OID, Codec: &pgtype.TextFormatOnlyCodec{Codec: pgtype.TextCodec{}}}
	m.RegisterType(custom)
	if restore := retentionArrayEncoding(m, 256); restore != nil {
		restore()
		t.Fatal("custom scalar codec was replaced")
	}
	for i, oid := range oids {
		after, _ := m.TypeForOID(oid)
		if after != before[i] {
			t.Fatalf("partial install leaked for OID %d", oid)
		}
	}
}

// Only the three exact slice types in binary format are special. Nullable
// elements, non-default dimensions, text mode and other input types retain pgx.
func TestRetentionArrayEncodingDelegatesOtherValues(t *testing.T) {
	m := pgtype.NewMap()
	restore := retentionArrayEncoding(m, 256)
	if restore == nil {
		t.Fatal("standard codecs rejected")
	}
	defer restore()
	for _, tc := range []struct {
		oid    uint32
		format int16
		value  any
	}{
		{pgtype.Int4ArrayOID, 0, []int32{-1, 2}},
		{pgtype.TextArrayOID, 0, []string{"雪", "", "a\"\\"}},
		{pgtype.Int4ArrayOID, 1, []int64{7}},
		{pgtype.TextArrayOID, 1, []pgtype.Text{{String: "x", Valid: true}, {}}},
		{pgtype.Int4ArrayOID, 1, pgtype.Array[int32]{Elements: []int32{7}, Dims: []pgtype.ArrayDimension{{Length: 1, LowerBound: 4}}, Valid: true}},
		{pgtype.BoolArrayOID, 1, pgtype.FlatArray[bool]{true}},
	} {
		got, err := m.Encode(tc.oid, tc.format, tc.value, nil)
		want, e := pgtype.NewMap().Encode(tc.oid, tc.format, tc.value, nil)
		if err != nil || e != nil || !bytes.Equal(got, want) {
			t.Fatalf("delegate %T/%d=%x/%v, want %x/%v", tc.value, tc.format, got, err, want, e)
		}
	}
}

type retentionArrayRejectPlan struct{ cause error }

func (p retentionArrayRejectPlan) Encode(any, []byte) ([]byte, error) { return nil, p.cause }

type retentionArrayRejectCodec struct {
	pgtype.Codec
	cause error
}

func (c retentionArrayRejectCodec) PlanEncode(*pgtype.Map, uint32, int16, any) pgtype.EncodePlan {
	return retentionArrayRejectPlan{cause: c.cause}
}

func TestRetentionArrayEncodingPreservesDelegatedErrorCause(t *testing.T) {
	m := pgtype.NewMap()
	typ, _ := m.TypeForOID(pgtype.Int4ArrayOID)
	cause := errors.New("custom array encoder failure")
	original := retentionArrayRejectCodec{Codec: typ.Codec, cause: cause}
	wrapped := retentionArrayCodec{Codec: original, oid: pgtype.Int4ArrayOID}
	for _, tc := range []struct {
		format int16
		value  any
	}{
		{0, []int32{1}}, {1, []int64{1}}, {1, pgtype.FlatArray[int32]{1}},
	} {
		plan := wrapped.PlanEncode(m, pgtype.Int4ArrayOID, tc.format, tc.value)
		_, err := plan.Encode(tc.value, nil)
		//nolint:errorlint // Delegation must preserve the original encoder error object.
		if err != cause {
			t.Fatalf("delegated cause=%v, want original %v", err, cause)
		}
	}
	custom := *typ
	custom.Codec = original
	m.RegisterType(&custom)
	if restore := retentionArrayEncoding(m, 256); restore != nil {
		restore()
		t.Fatal("custom array semantics were replaced")
	}
	_, err := m.Encode(pgtype.Int4ArrayOID, pgtype.BinaryFormatCode, []int32{1}, nil)
	if !errors.Is(err, cause) {
		t.Fatalf("custom TypeMap cause=%v", err)
	}
}

func TestRetentionArrayEncodingLeavesCustomWrappers(t *testing.T) {
	m := pgtype.NewMap()
	before, _ := m.TypeForOID(pgtype.Int4ArrayOID)
	m.TryWrapEncodePlanFuncs = append(m.TryWrapEncodePlanFuncs, func(any) (pgtype.WrappedEncodePlanNextSetter, any, bool) { return nil, nil, false })
	if restore := retentionArrayEncoding(m, 256); restore != nil {
		restore()
		t.Fatal("custom encode wrappers were bypassed")
	}
	after, _ := m.TypeForOID(pgtype.Int4ArrayOID)
	if after != before {
		t.Fatal("custom-wrapper fallback changed Type")
	}
}
