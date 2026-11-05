package app

import (
	"cmp"
	"context"
	"reflect"
	"slices"
	"strings"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// This limits the logical charge of owned staging, not RSS, the parse budget,
// or the existing full-notebook resolution working set. It is per Rebuild.
const rebuildStagingBytes = uint64(512 << 20)

type rebuildStage struct {
	links   [][]domain.Link
	order   []int
	charged uint64
	limit   uint64
	rows    int
}

func newRebuildStage(pages int, requested uint64) *rebuildStage {
	limit := rebuildStagingBytes
	if requested > 0 {
		limit = min(limit, requested)
	}
	if pages < 0 {
		return nil
	}
	// Reserve the later source-order array too, before retaining any page.
	rate := uint64(reflect.TypeFor[[]domain.Link]().Size() + reflect.TypeFor[int]().Size())
	charge, ok := rebuildMul(uint64(pages), rate)
	if !ok || charge > limit {
		return nil
	}
	return &rebuildStage{links: make([][]domain.Link, pages), charged: charge, limit: limit}
}

// add admits the whole page before allocating any owned row/string snapshot.
// False means the caller must drain previous pages and keep this page's Facts
// for ordinary AddPage. A failed admission changes nothing in the stage.
func (s *rebuildStage) add(ctx context.Context, slot int, links []domain.Link) (bool, error) {
	charge, fits := rebuildMul(uint64(len(links)), uint64(reflect.TypeFor[domain.Link]().Size()))
	if !fits {
		return false, nil
	}
	for i, l := range links {
		if i%checkEvery == 0 {
			if err := ctx.Err(); err != nil {
				return false, err
			}
		}
		kind := l.Kind
		switch kind {
		case "wikilink", "embed", "link", "image":
			kind = "" // canonical static literals need no cloned backing
		}
		for _, value := range [...]string{kind, l.Target, l.Property, l.Anchor, l.Display} {
			bytes, ok := rebuildStringBytes(uint64(len(value)))
			if !ok {
				return false, nil
			}
			charge, ok = rebuildAdd(charge, bytes)
			if !ok {
				return false, nil
			}
		}
	}
	if charge > s.limit-s.charged {
		return false, nil
	}
	var owned []domain.Link
	if links != nil {
		owned = make([]domain.Link, len(links))
	}
	for i, l := range links {
		if i%checkEvery == 0 {
			if err := ctx.Err(); err != nil {
				return false, err
			}
		}
		switch l.Kind {
		case "wikilink":
			l.Kind = "wikilink"
		case "embed":
			l.Kind = "embed"
		case "link":
			l.Kind = "link"
		case "image":
			l.Kind = "image"
		default:
			l.Kind = strings.Clone(l.Kind)
		}
		// Empty substrings can still retain their old backing. Clone also
		// normalizes those to the empty literal without allocating.
		l.Target = strings.Clone(l.Target)
		l.Property = strings.Clone(l.Property)
		l.Anchor = strings.Clone(l.Anchor)
		l.Display = strings.Clone(l.Display)
		owned[i] = l
	}
	s.links[slot] = owned
	s.rows += len(owned) // the byte bound is much smaller than the int row limit
	s.charged += charge
	return true, nil
}

// inputs uses the original SQL's source/start order without changing the
// parser's slice or the order in which metadata was read/written. Sorting is
// after first-MaxLinks selection, and the sorted owned rows also drive INSERT.
func (s *rebuildStage) inputs(ctx context.Context, ids []uuid.UUID) ([]Link, error) {
	s.order = make([]int, len(ids))
	for i := range ids {
		if i%checkEvery == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		s.order[i] = i
	}
	slices.SortStableFunc(s.order, func(a, b int) int { return ids[a].Compare(ids[b]) })
	var out []Link
	if s.rows > 0 {
		out = make([]Link, s.rows)
	}
	at := 0
	for _, slot := range s.order {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		links := s.links[slot]
		slices.SortStableFunc(links, func(a, b domain.Link) int { return cmp.Compare(int32(a.Start), int32(b.Start)) })
		for _, l := range links {
			if at%checkEvery == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			out[at] = Link{SourceID: ids[slot], Start: int(int32(l.Start)), Target: l.Target, Aliases: l.Aliases}
			at++
		}
	}
	return out, nil
}

func (s *rebuildStage) release() {
	if s != nil {
		clear(s.links)
		s.links, s.order = nil, nil
		s.rows, s.charged = 0, 0
	}
}

func rebuildStringBytes(n uint64) (uint64, bool) {
	if n == 0 {
		return 0, true
	}
	bytes, ok := rebuildMul(n, 2)
	return max(16, bytes), ok
}

func rebuildMul(a, b uint64) (uint64, bool) {
	if b != 0 && a > ^uint64(0)/b {
		return 0, false
	}
	return a * b, true
}

func rebuildAdd(a, b uint64) (uint64, bool) {
	if b > ^uint64(0)-a {
		return 0, false
	}
	return a + b, true
}
