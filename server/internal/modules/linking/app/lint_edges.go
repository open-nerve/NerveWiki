package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

type wantedGroup struct {
	item domain.Item
	pos  domain.Position
}

func (s *ReadScope) generalEmitter(id uuid.UUID) bool {
	if _, current := s.current[id]; !current {
		return false
	}
	_, source := s.source[id]
	return s.mode != domain.ModeLLM || !source
}

// prepareWanted answers the wanted rule and, for a plan that asks for it,
// each general emitter's count of distinct qualified targets (seen, recorded
// before the source changes and at the stream's end); nil otherwise.
func (r *Rules) prepareWanted(ctx context.Context, s *ReadScope, plan lintPlan) (preparedRule, map[uuid.UUID]int, error) {
	groups := make(map[string]*wantedGroup)
	seen := make(map[string]struct{})
	var byPage map[uuid.UUID]int
	if plan.byPage {
		byPage = make(map[uuid.UUID]int)
	}
	var source uuid.UUID
	haveSource := false
	var parents []string
	count := func() {
		if byPage != nil && haveSource && len(seen) > 0 {
			byPage[source] = len(seen)
		}
	}
	stream := r.reads.Wanted(s.notebook, s.excluded)
	for {
		rows, err := stream.Next(ctx)
		if err != nil {
			return preparedRule{}, nil, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if err := ctx.Err(); err != nil {
				return preparedRule{}, nil, err
			}
			if !s.generalEmitter(row.SourceID) || row.ResolvedID != nil || row.TargetKey == nil {
				continue
			}
			if !haveSource || source != row.SourceID {
				count()
				source, haveSource = row.SourceID, true
				clear(seen)
				parents, err = s.parentKeys(ctx, source)
				if err != nil {
					return preparedRule{}, nil, err
				}
			}
			canonical, qualified, err := domain.WantedTarget(ctx, row.Target, row.Kind, parents, s.mode == domain.ModeLLM, s.presence)
			if err != nil {
				return preparedRule{}, nil, err
			}
			if !qualified {
				continue
			}
			if _, counted := seen[canonical]; counted {
				continue
			}
			seen[canonical] = struct{}{}
			if group, exists := groups[canonical]; exists {
				group.item.Pages++
				group.pos.Pages++
				continue
			}
			// Keep the complete canonical map key, but never retain each
			// group's representative raw. Its cut/hash are made exactly once,
			// after qualification, at the first actual source/range position.
			item, pos, err := domain.WantedItem(canonical, row.Target, 1)
			if err != nil {
				return preparedRule{}, nil, err
			}
			groups[canonical] = &wantedGroup{item: item, pos: pos}
		}
	}
	count()
	out := newLintCollector(domain.Wanted, plan.after, plan.limit)
	for _, group := range groups {
		if err := ctx.Err(); err != nil {
			return preparedRule{}, nil, err
		}
		out.add(group.item, group.pos)
	}
	result, err := out.finish(ctx, s.binding)
	if err != nil {
		return preparedRule{}, nil, err
	}
	return result, byPage, nil
}

// prepareAreaWanted is an LLM contract's second pass (M11 design 4.7 gate 6):
// the area's current pages, which the wanted rule never counts, read in
// their own stream and judged as the first pass judges, their counts going
// into byPage only, never into the rule's items or count.
func (r *Rules) prepareAreaWanted(ctx context.Context, s *ReadScope, byPage map[uuid.UUID]int) error {
	if byPage == nil {
		return errLintQuery
	}
	pages := make([]uuid.UUID, 0, len(s.excluded))
	for _, id := range s.excluded {
		if _, current := s.current[id]; current {
			pages = append(pages, id)
		}
	}
	seen := make(map[string]struct{})
	var source uuid.UUID
	haveSource := false
	var parents []string
	count := func() {
		if haveSource && len(seen) > 0 {
			byPage[source] = len(seen)
		}
	}
	stream := r.reads.WantedOf(s.notebook, pages)
	for {
		rows, err := stream.Next(ctx)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			if row.ResolvedID != nil || row.TargetKey == nil {
				continue
			}
			if _, area := s.current[row.SourceID]; !area {
				return errLintRows
			}
			if _, inArea := s.source[row.SourceID]; !inArea {
				return errLintRows
			}
			if !haveSource || source != row.SourceID {
				count()
				source, haveSource = row.SourceID, true
				clear(seen)
				parents, err = s.parentKeys(ctx, source)
				if err != nil {
					return err
				}
			}
			canonical, qualified, err := domain.WantedTarget(ctx, row.Target, row.Kind, parents, s.mode == domain.ModeLLM, s.presence)
			if err != nil {
				return err
			}
			if qualified {
				seen[canonical] = struct{}{}
			}
		}
	}
	count()
	return nil
}

func (r *Rules) prepareAmbiguous(ctx context.Context, s *ReadScope, plan lintPlan) (preparedRule, error) {
	out := newLintCollector(domain.Ambiguous, plan.after, plan.limit)
	seen := make(map[string]struct{})
	var source uuid.UUID
	haveSource := false
	stream := r.reads.Ambiguous(s.notebook, s.excluded)
	for {
		rows, err := stream.Next(ctx)
		if err != nil {
			return preparedRule{}, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if err := ctx.Err(); err != nil {
				return preparedRule{}, err
			}
			if !s.generalEmitter(row.SourceID) || !row.Ambiguous {
				continue
			}
			if row.ResolvedID != nil && !row.ResolvedAsset {
				if _, current := s.current[*row.ResolvedID]; !current {
					continue
				}
			}
			if !haveSource || source != row.SourceID {
				source, haveSource = row.SourceID, true
				clear(seen)
			}
			if _, duplicate := seen[row.Target]; duplicate {
				continue
			}
			seen[row.Target] = struct{}{}
			item, pos, err := domain.AmbiguousItem(source, row.Target)
			if err != nil {
				return preparedRule{}, err
			}
			out.add(item, pos)
		}
	}
	return out.finish(ctx, s.binding)
}
