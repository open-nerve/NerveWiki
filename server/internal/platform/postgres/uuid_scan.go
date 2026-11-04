package postgres

import (
	"reflect"
	"uuid"

	"github.com/jackc/pgx/v5/pgtype"
)

// tryWrapStdUUIDScanPlan preserves pgx's UUID decoding and NULL handling,
// replacing only its reflect conversion of the standard library UUID pointer.
func tryWrapStdUUIDScanPlan(target any) (pgtype.WrappedScanPlanNextSetter, any, bool) {
	dst, ok := target.(*uuid.UUID)
	if !ok {
		return nil, nil, false
	}
	return &stdUUIDScanPlan{}, (*[16]byte)(dst), true
}

type stdUUIDScanPlan struct {
	next pgtype.ScanPlan
}

func (plan *stdUUIDScanPlan) SetNext(next pgtype.ScanPlan) { plan.next = next }

func (plan *stdUUIDScanPlan) Scan(src []byte, target any) error {
	if dst, ok := target.(*uuid.UUID); ok {
		return plan.next.Scan(src, (*[16]byte)(dst))
	}
	// Rows replans when the destination type changes. Preserve pgx's original
	// conversion behavior if a caller instead reuses this plan directly.
	return plan.next.Scan(src, reflect.ValueOf(target).Convert(reflect.TypeFor[*[16]byte]()).Interface())
}
