package postgresadapter

import (
	"context"
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// This is an allocation bound for the specialized encoder, not an input cap.
// Larger arguments retain pgx's original encoding and error behavior.
const retentionArrayReserveLimit = 8 << 20

type retentionArrayQuerier struct{ postgres.Querier }

func (q retentionArrayQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx, ok := postgres.TxFrom(ctx)
	actual, actualOK := q.Querier.(pgx.Tx)
	if !ok || !actualOK || actual.Conn() != tx.Conn() || ctx.Err() != nil {
		return q.Querier.Exec(ctx, sql, args...)
	}
	reserve, ok := retentionArrayBudget(sql, args)
	if !ok {
		return q.Querier.Exec(ctx, sql, args...)
	}
	// The maintenance transaction uses its connection serially. Restoration
	// happens even when the original pgx tracer or Exec panics. Neither the
	// SQL/arguments nor pgx's statement/description caches are replaced.
	restore := retentionArrayEncoding(tx.Conn().TypeMap(), reserve)
	if restore != nil {
		defer restore()
	}
	return q.Querier.Exec(ctx, sql, args...)
}

func retentionArrayBudget(sql string, args []any) (int, bool) {
	header, _, _ := strings.Cut(sql, "\n")
	var first, size int
	var oids []uint32
	switch header {
	case "-- name: DeleteMissingPageLinks :exec":
		if len(args) != 2 {
			return 0, false
		}
		first, size, oids = 1, 16, []uint32{pgtype.Int4ArrayOID}
	case "-- name: WritePageLinksRetainingEqualPrefiltered :exec":
		if len(args) != 13 {
			return 0, false
		}
		if _, ok := args[1].(uuid.UUID); !ok {
			return 0, false
		}
		first, size, oids = 2, 32, []uint32{pgtype.Int4ArrayOID, pgtype.Int4ArrayOID,
			pgtype.TextArrayOID, pgtype.TextArrayOID, pgtype.BoolArrayOID, pgtype.TextArrayOID,
			pgtype.TextArrayOID, pgtype.TextArrayOID, pgtype.TextArrayOID, pgtype.TextArrayOID, pgtype.BoolArrayOID}
	default:
		return 0, false
	}
	if _, ok := args[0].(uuid.UUID); !ok {
		return 0, false
	}
	for i, oid := range oids {
		v := args[first+i]
		if !retentionArrayValue(oid, v) {
			return 0, false
		}
		n, ok := retentionArrayPayload(v, retentionArrayReserveLimit-size)
		if !ok {
			return 0, false
		}
		size += n
	}
	return size, true
}

func retentionArrayValue(oid uint32, value any) bool {
	switch oid {
	case pgtype.Int4ArrayOID:
		_, ok := value.([]int32)
		return ok
	case pgtype.TextArrayOID:
		_, ok := value.([]string)
		return ok
	case pgtype.BoolArrayOID:
		_, ok := value.([]bool)
		return ok
	default:
		return false
	}
}

// Bound before multiplying. The caller's small limit also bounds every
// int32 array dimension, element length and accumulated byte count.
func retentionArrayFixedSize(count, width, limit int) (int, bool) {
	if count < 0 || int64(count) > math.MaxInt32 || width <= 0 || limit < 20 || count > (limit-20)/width {
		return 0, false
	}
	return 20 + count*width, true
}

func retentionArrayPayload(value any, limit int) (int, bool) {
	if limit < 0 {
		return 0, false
	}
	switch v := value.(type) {
	case []int32:
		if v == nil {
			return 0, true
		}
		return retentionArrayFixedSize(len(v), 8, limit)
	case []bool:
		if v == nil {
			return 0, true
		}
		return retentionArrayFixedSize(len(v), 5, limit)
	case []string:
		if v == nil {
			return 0, true
		}
		n, ok := retentionArrayFixedSize(len(v), 4, limit)
		if !ok {
			return 0, false
		}
		for _, s := range v {
			if len(s) > limit-n {
				return 0, false
			}
			n += len(s)
		}
		return n, true
	default:
		return 0, false
	}
}

// Return nil without changing any Type if a caller installed custom scalar
// or array semantics. RegisterType invalidates encoding plans, including on
// restore; no encoding memo retains the invocation's codec after restoration.
func retentionArrayEncoding(m *pgtype.Map, reserve int) func() {
	if reserve < 0 || reserve > retentionArrayReserveLimit {
		return nil
	}
	// A caller's custom encode wrapper may give an exact slice new semantics.
	// The platform customizes scan wrappers only; otherwise leave pgx alone.
	standardWrappers := pgtype.NewMap().TryWrapEncodePlanFuncs
	if len(m.TryWrapEncodePlanFuncs) != len(standardWrappers) {
		return nil
	}
	for i, f := range m.TryWrapEncodePlanFuncs {
		if f == nil || reflect.ValueOf(f).Pointer() != reflect.ValueOf(standardWrappers[i]).Pointer() {
			return nil
		}
	}
	oids := [3]uint32{pgtype.Int4ArrayOID, pgtype.TextArrayOID, pgtype.BoolArrayOID}
	values := [3]any{[]int32{}, []string{}, []bool{}}
	var originals [3]*pgtype.Type
	var fallback [3]pgtype.EncodePlan
	for i, oid := range oids {
		t, ok := m.TypeForOID(oid)
		if !ok {
			return nil
		}
		a, ok := t.Codec.(*pgtype.ArrayCodec)
		if !ok || a.ElementType == nil {
			return nil
		}
		scalar, ok := m.TypeForOID(a.ElementType.OID)
		if !ok || scalar != a.ElementType {
			return nil
		}
		standard := false
		switch oid {
		case pgtype.Int4ArrayOID:
			_, standard = scalar.Codec.(pgtype.Int4Codec)
			standard = standard && scalar.OID == pgtype.Int4OID
		case pgtype.TextArrayOID:
			_, standard = scalar.Codec.(pgtype.TextCodec)
			standard = standard && scalar.OID == pgtype.TextOID
		case pgtype.BoolArrayOID:
			_, standard = scalar.Codec.(pgtype.BoolCodec)
			standard = standard && scalar.OID == pgtype.BoolOID
		}
		if !standard {
			return nil
		}
		originals[i] = t
		fallback[i] = m.PlanEncode(oid, pgtype.BinaryFormatCode, values[i])
		if fallback[i] == nil {
			return nil
		}
	}
	for i, oid := range oids {
		t := *originals[i]
		t.Codec = retentionArrayCodec{Codec: t.Codec, oid: oid, plan: retentionArrayPlan{reserve: reserve, element: originals[i].Codec.(*pgtype.ArrayCodec).ElementType.OID, fallback: fallback[i]}}
		m.RegisterType(&t)
	}
	return func() {
		for _, t := range originals {
			m.RegisterType(t)
		}
	}
}

type retentionArrayCodec struct {
	pgtype.Codec
	oid  uint32
	plan retentionArrayPlan
}

func (c retentionArrayCodec) PlanEncode(m *pgtype.Map, oid uint32, format int16, value any) pgtype.EncodePlan {
	if oid == c.oid && format == pgtype.BinaryFormatCode && retentionArrayValue(oid, value) {
		return c.plan
	}
	return c.Codec.PlanEncode(m, oid, format, value)
}

type retentionArrayPlan struct {
	reserve  int
	element  uint32
	fallback pgtype.EncodePlan
}

func (p retentionArrayPlan) Encode(value any, buf []byte) ([]byte, error) {
	size, ok := retentionArrayPayload(value, p.reserve-len(buf))
	if !ok {
		return p.fallback.Encode(value, buf)
	}
	if size == 0 {
		return nil, nil
	} // typed nil slice is SQL NULL, not an empty array
	if cap(buf) < p.reserve {
		next := make([]byte, len(buf), p.reserve)
		copy(next, buf)
		buf = next
	}
	var count int
	switch v := value.(type) {
	case []int32:
		count = len(v)
	case []string:
		count = len(v)
	case []bool:
		count = len(v)
	}
	buf = binary.BigEndian.AppendUint32(buf, 1) // one dimension, including non-nil empty slices
	buf = binary.BigEndian.AppendUint32(buf, 0) // these exact Go slices have no NULL elements
	buf = binary.BigEndian.AppendUint32(buf, p.element)
	buf = binary.BigEndian.AppendUint32(buf, uint32(count))
	buf = binary.BigEndian.AppendUint32(buf, 1) // pgx FlatArray's lower bound
	switch v := value.(type) {
	case []int32:
		for _, x := range v {
			buf = binary.BigEndian.AppendUint32(buf, 4)
			buf = binary.BigEndian.AppendUint32(buf, uint32(x))
		}
	case []string:
		for _, s := range v {
			buf = binary.BigEndian.AppendUint32(buf, uint32(len(s)))
			buf = append(buf, s...)
		}
	case []bool:
		for _, x := range v {
			buf = binary.BigEndian.AppendUint32(buf, 1)
			var b byte
			if x {
				b = 1
			}
			buf = append(buf, b)
		}
	}
	return buf, nil
}
