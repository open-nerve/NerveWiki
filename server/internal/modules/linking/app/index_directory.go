package app

import (
	"container/heap"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

const (
	// MaxIndexTypes and MaxIndexTypesJSONBytes bound the exact catalogue, not
	// the finite first-pass input or the count of pages contributing to it.
	MaxIndexTypes          = 100
	MaxIndexTypesJSONBytes = 32 * 1024
	indexCatalogueBatch    = 512
)

// filters keeps the original parent/tag error order. A non-error empty
// item filter may skip later item work, but REST still collects metadata.
func (l ListIndex) filters(ctx context.Context, notebookID uuid.UUID, q IndexQuery) (children, tagged map[uuid.UUID]bool, empty bool, err error) {
	if q.Parent != nil {
		children, err = l.childPages(ctx, notebookID, *q.Parent)
	} else if q.ParentID != nil || q.AtRoot {
		if q.ParentID != nil {
			var parents map[uuid.UUID]PagePath
			parents, err = l.Paths.Pages(ctx, notebookID, []uuid.UUID{*q.ParentID})
			if err != nil {
				return nil, nil, false, err
			}
			if _, ok := parents[*q.ParentID]; !ok {
				return nil, nil, false, domain.ErrPageNotFound
			}
		}
		var ids []uuid.UUID
		ids, err = l.Paths.ChildPagesAt(ctx, notebookID, q.ParentID)
		children = setOf(ids)
	}
	if err != nil || children != nil && len(children) == 0 {
		return children, nil, err == nil, err
	}
	if q.Tag != nil {
		key, ok := l.TagKey(strings.TrimPrefix(*q.Tag, "#"))
		if !ok {
			return children, nil, true, nil
		}
		var ids []uuid.UUID
		ids, err = l.Reads.TagPages(ctx, notebookID, key)
		if err != nil {
			return nil, nil, false, err
		}
		tagged = setOf(ids)
		if len(ids) == 0 {
			return children, tagged, true, nil
		}
	}
	return children, tagged, false, nil
}

// metadata uses one unlocked state read, after the established parent/tag
// checks. Source membership is complete and strict; the root is not excluded.
func (l ListIndex) metadata(ctx context.Context, notebookID uuid.UUID, exclude bool, out *IndexPage) (map[uuid.UUID]bool, error) {
	state, ok, err := l.Notebooks.StateOf(ctx, notebookID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrNotebookNotFound
	}
	out.Mode = state.Mode
	if state.Mode != "llm_wiki" || state.SourceRootID == nil {
		return nil, nil
	}
	nodes, err := l.Sources.Nodes(ctx, notebookID, *state.SourceRootID)
	if err != nil {
		return nil, err
	}
	out.SourceArea = &IndexSourceArea{RootID: *state.SourceRootID}
	var excluded map[uuid.UUID]bool
	if exclude {
		excluded = make(map[uuid.UUID]bool, len(nodes))
	}
	for _, n := range nodes {
		if n.Asset || n.Bytes > 0 {
			out.SourceArea.Sources++
		}
		if exclude {
			excluded[n.ID] = true
		}
	}
	return excluded, nil
}

// catalogue drains every bounded batch. The retained threshold only decreases:
// a discarded exact value cannot later re-enter, so retained counts are complete
// at EOF without an unbounded set of distinct values. Matching IDs are produced
// before eviction, independently of the public menu's item and byte bounds.
func (l ListIndex) catalogue(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID, typeIs *string, out *IndexPage) (map[uuid.UUID]bool, error) {
	var matching map[uuid.UUID]bool
	if typeIs != nil {
		matching = map[uuid.UUID]bool{}
	}
	keep := make(map[string]*indexTypeEntry, MaxIndexTypes)
	top := indexTypeHeap{}
	keyer := domain.NewIndexKeyer()
	out.TypesComplete = true
	for start := 0; start < len(ids); start += indexCatalogueBatch {
		batch := ids[start:min(start+indexCatalogueBatch, len(ids))]
		rows, err := l.Catalogue.CatalogueHeads(ctx, notebookID, batch)
		if err != nil {
			return nil, err
		}
		if len(rows) != len(batch) {
			return nil, fmt.Errorf("index: incomplete catalogue batch")
		}
		for i, row := range rows {
			if row.ID != batch[i] || !row.Typed && row.FilterType != nil {
				return nil, fmt.Errorf("index: invalid catalogue row")
			}
			if !row.Typed {
				continue
			}
			if row.FilterType == nil {
				out.UnfilterableTypePages++
				out.TypesComplete = false
				continue
			}
			value := *row.FilterType
			if len(value) > domain.MaxIndexType {
				return nil, fmt.Errorf("index: oversized catalogue type")
			}
			if typeIs != nil && value == *typeIs {
				matching[row.ID] = true
			}
			if held := keep[value]; held != nil {
				held.count++
				continue
			}
			entry := &indexTypeEntry{value: value, key: keyer.Key(value), count: 1}
			if len(top) == MaxIndexTypes {
				out.TypesComplete = false
				if compareIndexType(entry, top[0]) >= 0 {
					continue
				}
				removed := heap.Pop(&top).(*indexTypeEntry)
				delete(keep, removed.value)
			}
			keep[value] = entry
			heap.Push(&top, entry)
		}
	}
	slices.SortFunc(top, compareIndexType)
	out.Types = make([]IndexType, len(top))
	for i, e := range top {
		out.Types[i] = IndexType{Type: e.value, Count: e.count}
	}
	for {
		encoded, err := json.Marshal(out.Types)
		if err != nil {
			return nil, fmt.Errorf("index: encode types: %w", err)
		}
		if len(encoded) <= MaxIndexTypesJSONBytes {
			break
		}
		out.Types = out.Types[:len(out.Types)-1]
		out.TypesComplete = false
	}
	return matching, nil
}

type indexTypeEntry struct {
	value, key string
	count      int
}

func compareIndexType(a, b *indexTypeEntry) int {
	if c := strings.Compare(a.key, b.key); c != 0 {
		return c
	}
	return strings.Compare(a.value, b.value)
}

type indexTypeHeap []*indexTypeEntry

func (h indexTypeHeap) Len() int           { return len(h) }
func (h indexTypeHeap) Less(i, j int) bool { return compareIndexType(h[i], h[j]) > 0 }
func (h indexTypeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *indexTypeHeap) Push(v any)        { *h = append(*h, v.(*indexTypeEntry)) }
func (h *indexTypeHeap) Pop() any {
	old := *h
	v := old[len(old)-1]
	old[len(old)-1] = nil
	*h = old[:len(old)-1]
	return v
}
