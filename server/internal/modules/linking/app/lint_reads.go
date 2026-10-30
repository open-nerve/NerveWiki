package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"uuid"
)

// LintReadRows and LintReadTargetBytes bound one materialized batch, never
// the notebook or its complete source/exclusion sets.
const (
	LintReadRows              = 512
	LintReadTargetBytes int64 = 8 * 1024 * 1024
)

// MaxLintPayloadBytes bounds a single decoded target or serialized JSON
// value, including YAML escaping, aliases and numeric JSONB expansion.
// maxContentBytes is the page module's fixed bound, supplied by bootstrap.
func MaxLintPayloadBytes(maxContentBytes int) int64 {
	c := int64(maxContentBytes)
	return 6*(c+max(c, 100000)) + 512*10000
}

// IndexedHead retains old formats so the consumer can report stale pages.
type IndexedHead struct {
	ID               uuid.UUID
	Extractor        int
	FrontmatterValid bool
}

// EdgePos is an actual position. The zero UUID is a valid source ID.
type EdgePos struct {
	SourceID uuid.UUID
	Start    int32
}

// Compare orders the complete primary key.
func (p EdgePos) Compare(q EdgePos) int {
	if n := bytes.Compare(p.SourceID[:], q.SourceID[:]); n != 0 {
		return n
	}
	if p.Start < q.Start {
		return -1
	}
	if p.Start > q.Start {
		return 1
	}
	return 0
}

// EdgeHead excludes the full target and all property values.
type EdgeHead struct {
	EdgePos
	TargetBytes int64
}

// LintEdge is the exact wanted/ambiguous projection. It does not project
// property_key: an absent field must not be interpreted as stored SQL NULL.
type LintEdge struct {
	EdgePos
	Kind, Target             string
	TargetKey, AltKey        *string
	ResolvedID               *uuid.UUID
	ResolvedAsset, Ambiguous bool
}

// SourcePropertyPos identifies one stored property, including position zero.
type SourcePropertyPos struct {
	PageID   uuid.UUID
	Position int32
}

// SourcePropertyHead contains serialized JSON length, not JSON itself.
type SourcePropertyHead struct {
	SourcePropertyPos
	ValueBytes int64
}

// SourceProperty is the exact first ASCII sources property of a page.
type SourceProperty struct {
	SourcePropertyPos
	Key  string
	JSON []byte
}

// SourceSpec describes the first property's root string or direct array.
// Items is zero for a string and the complete direct length for an array.
type SourceSpec struct {
	PageID uuid.UUID
	Key    string
	Array  bool
	Items  int32
}

// SourceSlotHead has only a root/direct ordinal key of at most 12 bytes.
type SourceSlotHead struct {
	EdgePos
	PropertyKey string
	TargetBytes int64
}

// SourceSlotEdge omits target title keys. PropertyKey preserves SQL NULL
// separately from a legal empty path; no target qualification occurs here.
type SourceSlotEdge struct {
	EdgePos
	PropertyKey              *string
	Target, Kind             string
	ResolvedID               *uuid.UUID
	ResolvedAsset, Ambiguous bool
}

// BodyReference projects no target, property key or property JSON.
type BodyReference struct {
	EdgePos
	ResolvedID    uuid.UUID
	ResolvedAsset bool
}

// LintReadStore is a bounded, transaction-aware data port. Exact reads take
// sorted, unique positions from headers in the same snapshot. No Rows escape.
type LintReadStore interface {
	Heads(context.Context, uuid.UUID) ([]IndexedHead, error)
	WantedEdgeHeads(context.Context, uuid.UUID, []uuid.UUID, *EdgePos, int) ([]EdgeHead, error)
	AmbiguousEdgeHeads(context.Context, uuid.UUID, []uuid.UUID, *EdgePos, int) ([]EdgeHead, error)
	Edges(context.Context, uuid.UUID, []EdgePos) ([]LintEdge, error)
	SourcePropertyHeads(context.Context, uuid.UUID, []uuid.UUID) ([]SourcePropertyHead, error)
	SourceProperties(context.Context, uuid.UUID, []SourcePropertyPos) ([]SourceProperty, error)
	SourceSlotHeads(context.Context, uuid.UUID, []SourceSpec, *EdgePos, int) ([]SourceSlotHead, error)
	SourceSlotEdges(context.Context, uuid.UUID, []EdgePos) ([]SourceSlotEdge, error)
	BodyReferences(context.Context, uuid.UUID, []uuid.UUID, *EdgePos, int) ([]BodyReference, error)
}

// LintReads builds streams within a caller-owned snapshot. The caller must
// discard all accumulated results on any error; streams never own a snapshot.
type LintReads struct {
	store      LintReadStore
	maxPayload int64
}

// NewLintReads binds the read store and fixed content-derived payload bound.
func NewLintReads(store LintReadStore, maxContentBytes int) *LintReads {
	return &LintReads{store: store, maxPayload: MaxLintPayloadBytes(maxContentBytes)}
}

// Heads returns all indexed formats in this notebook.
func (r *LintReads) Heads(ctx context.Context, notebook uuid.UUID) ([]IndexedHead, error) {
	return r.store.Heads(ctx, notebook)
}

// LintBatches returns at most 512 rows and an ordinary 8 MiB payload prefix,
// or one legal larger value. An error is terminal. Next checks cancellation
// even while consuming buffered heads. Empty results mean end of stream.
type LintBatches[T any] struct {
	next func(context.Context) ([]T, error)
	err  error
}

// Next consumes the next bounded prefix; it does not keep prior full values.
func (b *LintBatches[T]) Next(ctx context.Context) ([]T, error) {
	if b.err != nil {
		return nil, b.err
	}
	if err := ctx.Err(); err != nil {
		b.err = err
		return nil, err
	}
	out, err := b.next(ctx)
	if err != nil {
		b.err = err
		return nil, err
	}
	return out, nil
}

func lintBatches[H, T any](maxPayload int64, head func(context.Context, *EdgePos) ([]H, error), pos func(H) EdgePos, size func(H) int64, values func(context.Context, []H) ([]T, error)) *LintBatches[T] {
	var buffered []H
	var after *EdgePos
	done := false
	return &LintBatches[T]{next: func(ctx context.Context) ([]T, error) {
		if done {
			return nil, nil
		}
		if len(buffered) == 0 {
			var err error
			buffered, err = head(ctx, after)
			if err != nil {
				return nil, err
			}
			if len(buffered) > LintReadRows {
				return nil, errors.New("lint reads: too many heads")
			}
			if len(buffered) == 0 {
				done = true
				return nil, nil
			}
			previous := after
			for _, h := range buffered {
				p := pos(h)
				if p.Start < 0 || (previous != nil && p.Compare(*previous) <= 0) {
					return nil, errors.New("lint reads: heads do not advance")
				}
				previous = &p
			}
		}
		n := 0
		var total int64
		for _, h := range buffered {
			bytes := size(h)
			if bytes < 0 || bytes > maxPayload+12 {
				return nil, errors.New("lint reads: payload exceeds derived bound")
			}
			if n > 0 && total+bytes > LintReadTargetBytes {
				break
			}
			total += bytes
			n++
			if total >= LintReadTargetBytes {
				break
			}
		}
		out, err := values(ctx, buffered[:n])
		if err != nil {
			return nil, err
		}
		if len(out) != n {
			return nil, errors.New("lint reads: exact positions disappeared")
		}
		p := pos(buffered[n-1])
		after = &p
		buffered = buffered[n:]
		return out, nil
	}}
}

func edgePositions(heads []EdgeHead) []EdgePos {
	out := make([]EdgePos, len(heads))
	for i, h := range heads {
		out[i] = h.EdgePos
	}
	return out
}
func (r *LintReads) edges(notebook uuid.UUID, heads func(context.Context, *EdgePos) ([]EdgeHead, error)) *LintBatches[LintEdge] {
	return lintBatches(r.maxPayload, heads, func(h EdgeHead) EdgePos { return h.EdgePos }, func(h EdgeHead) int64 { return h.TargetBytes }, func(ctx context.Context, h []EdgeHead) ([]LintEdge, error) {
		return r.store.Edges(ctx, notebook, edgePositions(h))
	})
}

// Wanted reads unresolved keyed edges of current emitters except excluded.
// Excluded is the complete set; it is not limited to a batch's row count.
func (r *LintReads) Wanted(notebook uuid.UUID, excluded []uuid.UUID) *LintBatches[LintEdge] {
	ids := slices.Clone(excluded)
	return r.edges(notebook, func(ctx context.Context, p *EdgePos) ([]EdgeHead, error) {
		return r.store.WantedEdgeHeads(ctx, notebook, ids, p, LintReadRows)
	})
}

// Ambiguous uses its own static qualification and the same bounded projection.
func (r *LintReads) Ambiguous(notebook uuid.UUID, excluded []uuid.UUID) *LintBatches[LintEdge] {
	ids := slices.Clone(excluded)
	return r.edges(notebook, func(ctx context.Context, p *EdgePos) ([]EdgeHead, error) {
		return r.store.AmbiguousEdgeHeads(ctx, notebook, ids, p, LintReadRows)
	})
}

// SourceValues probes every eligible page once, in disjoint groups of 512.
// A page without sources yields no head and never causes a slot query.
func (r *LintReads) SourceValues(notebook uuid.UUID, eligible []uuid.UUID) *LintBatches[SourceProperty] {
	ids := slices.Clone(eligible)
	for i, id := range ids {
		if i > 0 && bytes.Compare(ids[i-1][:], id[:]) >= 0 {
			return &LintBatches[SourceProperty]{err: errors.New("lint reads: source IDs must be unique and sorted")}
		}
	}
	head := func(ctx context.Context, _ *EdgePos) ([]SourcePropertyHead, error) {
		for len(ids) > 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			n := min(len(ids), LintReadRows)
			h, err := r.store.SourcePropertyHeads(ctx, notebook, ids[:n])
			ids = ids[n:]
			if err != nil || len(h) > 0 {
				return h, err
			}
		}
		return nil, nil
	}
	return lintBatches(r.maxPayload, head, func(h SourcePropertyHead) EdgePos { return EdgePos{SourceID: h.PageID, Start: h.Position} }, func(h SourcePropertyHead) int64 { return h.ValueBytes + 7 }, func(ctx context.Context, h []SourcePropertyHead) ([]SourceProperty, error) {
		p := make([]SourcePropertyPos, len(h))
		for i, v := range h {
			p[i] = v.SourcePropertyPos
		}
		return r.store.SourceProperties(ctx, notebook, p)
	})
}

// SourceSlots reads all persisted edges at the supplied root/direct-array
// paths. Cardinality, target qualification and Finish belong to the consumer.
func (r *LintReads) SourceSlots(notebook uuid.UUID, specs []SourceSpec) *LintBatches[SourceSlotEdge] {
	specs = slices.Clone(specs)
	return lintBatches(r.maxPayload, func(ctx context.Context, p *EdgePos) ([]SourceSlotHead, error) {
		return r.store.SourceSlotHeads(ctx, notebook, specs, p, LintReadRows)
	}, func(h SourceSlotHead) EdgePos { return h.EdgePos }, func(h SourceSlotHead) int64 { return h.TargetBytes + int64(len(h.PropertyKey)) }, func(ctx context.Context, h []SourceSlotHead) ([]SourceSlotEdge, error) {
		p := make([]EdgePos, len(h))
		for i, v := range h {
			p[i] = v.EdgePos
		}
		return r.store.SourceSlotEdges(ctx, notebook, p)
	})
}

// Bodies streams only short resolved body references of current emitters.
func (r *LintReads) Bodies(notebook uuid.UUID, emitters []uuid.UUID) *LintBatches[BodyReference] {
	ids := slices.Clone(emitters)
	var after *EdgePos
	done := false
	return &LintBatches[BodyReference]{next: func(ctx context.Context) ([]BodyReference, error) {
		if done {
			return nil, nil
		}
		out, err := r.store.BodyReferences(ctx, notebook, ids, after, LintReadRows)
		if err != nil {
			return nil, err
		}
		if len(out) > LintReadRows {
			return nil, fmt.Errorf("lint reads: %d body references exceed batch", len(out))
		}
		for _, row := range out {
			if row.Start < 0 || (after != nil && row.Compare(*after) <= 0) {
				return nil, errors.New("lint reads: body references do not advance")
			}
			p := row.EdgePos
			after = &p
		}
		done = len(out) == 0
		return out, nil
	}}
}
