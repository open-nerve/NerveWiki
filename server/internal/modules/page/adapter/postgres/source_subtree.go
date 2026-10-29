package postgresadapter

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// SubtreeWithin reads a source-entry subtree, including its root, only
// when it fits max. Each query returns at most the remaining space plus
// one witness of overflow. Complete trees keep Subtree's global order.
func (s *Store) SubtreeWithin(ctx context.Context, notebookID, id uuid.UUID, max int) (domain.Subtree, bool, error) {
	if max < 1 || max > domain.MaxUnitNodes {
		return nil, false, fmt.Errorf("source subtree: invalid bound %d", max)
	}
	root, err := s.queries(ctx).FindNodeIn(ctx, gen.FindNodeInParams{ID: id, NotebookID: notebookID})
	if err != nil {
		return nil, false, notFound("source subtree", err)
	}
	out := domain.Subtree{{Level: 1, Node: nodeOf(gen.FindNodeRow(root))}}
	parents := []uuid.UUID{root.ID}
	levels := s.planned(ctx)
	for level := 2; level <= subtreeLevels; level++ {
		take := max - len(out) + 1
		rows, err := levels.SourceChildrenWithin(ctx, gen.SourceChildrenWithinParams{
			NotebookID: notebookID, Parents: parents, MaxRows: int32(take),
		})
		if err != nil {
			return nil, false, fmt.Errorf("source subtree: %w", err)
		}
		if len(rows) == take {
			return nil, false, nil
		}
		if len(rows) == 0 {
			slices.SortFunc(out, func(a, b domain.SubtreeNode) int {
				if order := cmp.Compare(a.Level, b.Level); order != 0 {
					return order
				}
				if order := cmp.Compare(a.Node.SortOrder, b.Node.SortOrder); order != 0 {
					return order
				}
				return bytes.Compare(a.Node.ID[:], b.Node.ID[:])
			})
			return out, true, nil
		}
		// A child has one parent. Starting with the verified root, every
		// next parent list is unique, in this notebook, and at most max.
		parents = make([]uuid.UUID, len(rows))
		for i, r := range rows {
			out = append(out, domain.SubtreeNode{Level: level, Node: nodeOf(gen.FindNodeRow(r))})
			parents[i] = r.ID
		}
	}
	return nil, false, fmt.Errorf("source subtree: tree is not exhausted within %d levels", subtreeLevels)
}
