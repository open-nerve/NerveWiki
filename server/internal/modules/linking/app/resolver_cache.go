package app

import (
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

const (
	maxParsedTargets        = 10_000
	maxFolderTargets        = 10_000
	targetOverflow   uint16 = 65535
	targetBlock             = 256
)

// parsedTargetRows retains one small reference per input. Zero is invalid,
// 1..10000 refer to cached immutable values, and targetOverflow consumes the
// next overflow value. Even after the raw-parse cache fills, every valid row
// owns its complete parse; readers never reparse or cap the input relation.
// Overflow blocks keep all-unique inputs as values, not one heap object per row.
type parsedTargetRows struct {
	refs     []uint16
	cache    map[string]uint16
	values   []domain.Target
	overflow [][]domain.Target
}

func newParsedTargetRows(n int) *parsedTargetRows {
	return &parsedTargetRows{refs: make([]uint16, n), cache: make(map[string]uint16)}
}

func (r *parsedTargetRows) add(i int, raw string) (domain.Target, bool) {
	if ref, ok := r.cache[raw]; ok {
		r.refs[i] = ref
		if ref == 0 {
			return domain.Target{}, false
		}
		return r.values[int(ref)-1], true
	}
	t, ok := domain.ParseTarget(raw)
	var ref uint16
	if len(r.cache) < maxParsedTargets {
		if ok {
			r.values = append(r.values, t)
			ref = uint16(len(r.values))
		}
		r.cache[raw] = ref // Invalid raw targets count toward the same cache bound.
	} else if ok {
		ref = targetOverflow
		if len(r.overflow) == 0 || len(r.overflow[len(r.overflow)-1]) == targetBlock {
			r.overflow = append(r.overflow, make([]domain.Target, 0, targetBlock))
		}
		last := len(r.overflow) - 1
		r.overflow[last] = append(r.overflow[last], t)
	}
	r.refs[i] = ref
	return t, ok
}

// Each full pass starts overflow at zero and calls at once per input row,
// before skipping a missing source. That preserves uncached rows' alignment.
func (r *parsedTargetRows) at(i int, overflow *int) (domain.Target, bool) {
	switch ref := r.refs[i]; ref {
	case 0:
		return domain.Target{}, false
	case targetOverflow:
		at := *overflow
		*overflow = at + 1
		return r.overflow[at/targetBlock][at%targetBlock], true
	default:
		return r.values[int(ref)-1], true
	}
}

type resolutionKey struct {
	source uuid.UUID
	target domain.TargetKey
}

// Unordered inputs keep the original whole-call source cache. Ordered inputs
// share fresh results along an identical complete-folder run; source holds only
// targets that did not fit the shared map, preserving same-source reuse for them.
// Neither candidates nor paths change while these maps are used in this call.
type resolutionMemo struct {
	ordered     bool
	source      map[resolutionKey]domain.Resolution
	folder      []domain.Step
	folderKnown bool
	shared      map[domain.TargetKey]domain.Resolution
}

func newResolutionMemo(ordered bool) *resolutionMemo {
	return &resolutionMemo{ordered: ordered, source: make(map[resolutionKey]domain.Resolution)}
}

// begin is called at each ordered source boundary, even for invalid targets.
// An absent source breaks the run without itself reporting missing: that stays
// at the original parsed/source check. Known empty folders are valid folders.
func (m *resolutionMemo) begin(path []domain.Step, exists bool) {
	clear(m.source)
	folder := path[:max(len(path)-1, 0)]
	if !exists || !m.folderKnown || !slices.Equal(folder, m.folder) {
		clear(m.shared)
	}
	m.folder, m.folderKnown = folder, exists
}

func (m *resolutionMemo) get(source uuid.UUID, target domain.TargetKey) (domain.Resolution, bool) {
	if m.ordered {
		if r, ok := m.shared[target]; ok {
			return r, true
		}
	}
	r, ok := m.source[resolutionKey{source, target}]
	return r, ok
}

// Put follows alias construction (including missing aliases) and a real fresh
// Resolve. Shared entries never leave during a folder run, so they need no
// source copy. A full shared map must not disable the original source cache.
func (m *resolutionMemo) put(source uuid.UUID, target domain.TargetKey, r domain.Resolution) {
	if m.ordered && m.folderKnown && len(m.shared) < maxFolderTargets {
		if m.shared == nil {
			m.shared = make(map[domain.TargetKey]domain.Resolution)
		}
		m.shared[target] = r
		return
	}
	m.source[resolutionKey{source, target}] = r
}
