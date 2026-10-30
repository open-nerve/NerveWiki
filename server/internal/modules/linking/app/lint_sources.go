package app

import (
	"context"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// sourceResults is complete before either projection consumes it. It retains
// only the D union and the requested invalid_sources result, never whole-notebook
// JSON, edges, or per-source reference arrays.
type sourceResults struct {
	digested map[uuid.UUID]struct{}
	invalid  preparedRule
}

type lintSourcePage struct {
	id      uuid.UUID
	matcher *domain.SourceSlots
}

func (r *Rules) prepareSources(ctx context.Context, s *ReadScope, plan lintPlan) (*sourceResults, error) {
	needInvalid := slices.Contains(plan.rules, domain.InvalidSources)
	needPending := slices.Contains(plan.rules, domain.UndigestedSources)
	out := &sourceResults{digested: make(map[uuid.UUID]struct{})}
	emitters := make(map[uuid.UUID]struct{}, len(s.emitters))
	for _, id := range s.emitters {
		emitters[id] = struct{}{}
	}
	eligible := s.properties
	if !needInvalid {
		eligible = make([]uuid.UUID, 0, len(s.properties))
		for _, id := range s.properties {
			if _, wiki := emitters[id]; wiki {
				eligible = append(eligible, id)
			}
		}
	}
	invalid := newLintCollector(domain.InvalidSources, plan.after, plan.limit)
	finish := func(page lintSourcePage) error {
		judgments, err := page.matcher.Finish(ctx)
		if err != nil {
			return err
		}
		type identity struct {
			notLink bool
			text    string
		}
		seen := make(map[identity]struct{})
		for _, j := range judgments {
			if err := ctx.Err(); err != nil {
				return err
			}
			target, inArea := s.sourceTarget(j.ResolvedID, j.ResolvedAsset)
			if !j.NotLink && inArea {
				if _, wiki := emitters[page.id]; wiki && needPending && s.digestible(target) {
					out.digested[target.ID] = struct{}{}
				}
				continue
			}
			if !needInvalid {
				continue
			}
			key := identity{notLink: j.NotLink, text: j.FullText}
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			item, pos, err := domain.InvalidSourceItem(page.id, j.FullText, j.NotLink)
			if err != nil {
				return err
			}
			invalid.add(item, pos)
		}
		return nil
	}
	values := r.reads.SourceValues(s.notebook, eligible)
	var previous *uuid.UUID
	for {
		properties, err := values.Next(ctx)
		if err != nil {
			return nil, err
		}
		if len(properties) == 0 {
			break
		}
		pages := make([]lintSourcePage, 0, len(properties))
		specs := make([]SourceSpec, 0, len(properties))
		for _, property := range properties {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			_, member := slices.BinarySearchFunc(eligible, property.PageID, func(a, b uuid.UUID) int { return a.Compare(b) })
			if !member || previous != nil && property.PageID.Compare(*previous) <= 0 {
				return nil, errLintRows
			}
			id := property.PageID
			previous = &id
			matcher, err := domain.NewSourceSlots(ctx, property.Key, property.JSON)
			if err != nil {
				return nil, err
			}
			pages = append(pages, lintSourcePage{id: id, matcher: matcher})
			array, items, hasSlots := matcher.QueryShape()
			if hasSlots {
				specs = append(specs, SourceSpec{PageID: id, Key: property.Key, Array: array, Items: items})
			}
		}
		at := 0
		if len(specs) > 0 {
			edges := r.reads.SourceSlots(s.notebook, specs)
			for {
				rows, err := edges.Next(ctx)
				if err != nil {
					return nil, err
				}
				if len(rows) == 0 {
					break
				}
				for _, row := range rows {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					// A page's matcher survives every 512-row/8MiB cut. A
					// zero-match page is also finished before advancing.
					for at < len(pages) && pages[at].id.Compare(row.SourceID) < 0 {
						if err := finish(pages[at]); err != nil {
							return nil, err
						}
						at++
					}
					if at == len(pages) || pages[at].id != row.SourceID {
						return nil, errLintRows
					}
					if err := pages[at].matcher.Observe(row.PropertyKey, row.Target, row.ResolvedID, row.ResolvedAsset); err != nil {
						return nil, err
					}
				}
			}
		}
		for ; at < len(pages); at++ {
			if err := finish(pages[at]); err != nil {
				return nil, err
			}
		}
	}
	if needPending && len(s.emitters) > 0 {
		bodies := r.reads.Bodies(s.notebook, s.emitters)
		for {
			rows, err := bodies.Next(ctx)
			if err != nil {
				return nil, err
			}
			if len(rows) == 0 {
				break
			}
			for _, row := range rows {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if _, eligible := emitters[row.SourceID]; !eligible {
					return nil, errLintRows
				}
				if target, inArea := s.sourceTarget(&row.ResolvedID, row.ResolvedAsset); inArea && s.digestible(target) {
					out.digested[target.ID] = struct{}{}
				}
			}
		}
	}
	if needInvalid {
		var err error
		out.invalid, err = invalid.finish(ctx, s.binding)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Rules) preparePending(ctx context.Context, s *ReadScope, plan lintPlan, sources *sourceResults) (preparedRule, error) {
	out := newLintCollector(domain.UndigestedSources, plan.after, plan.limit)
	add := func(id uuid.UUID) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		n := s.nodes[id]
		size := n.PageBytes
		if n.Kind == domain.NodeAsset {
			size = 0 // final selected asset IDs are projected once after all rules
		}
		item, pos, err := domain.PendingItem(n.ID, n.Kind, size, n.CreatedAt)
		if err != nil {
			return err
		}
		out.add(item, pos)
		return nil
	}
	for _, id := range s.pendingPages {
		if _, digested := sources.digested[id]; !digested {
			if err := add(id); err != nil {
				return preparedRule{}, err
			}
		}
	}
	assets := make([]uuid.UUID, 0, len(s.assets))
	for _, id := range s.assets {
		if _, digested := sources.digested[id]; !digested {
			assets = append(assets, id)
		}
	}
	slices.SortFunc(assets, func(a, b uuid.UUID) int {
		if n := s.nodes[a].CreatedAt.Compare(s.nodes[b].CreatedAt); n != 0 {
			return n
		}
		return a.Compare(b)
	})
	parents := PendingAssetInput{NotebookID: s.notebook, ParentCandidates: s.parents}
	for len(assets) > 0 {
		if err := ctx.Err(); err != nil {
			return preparedRule{}, err
		}
		n := min(len(assets), LintReadRows)
		ids := slices.Clone(assets[:n])
		lintIDs(ids)
		pending, err := r.store.LintPendingAssets(ctx, parents, ids)
		if err != nil {
			return preparedRule{}, err
		}
		if len(pending) > len(ids) {
			return preparedRule{}, errLintRows
		}
		for i, id := range pending {
			_, member := slices.BinarySearchFunc(ids, id, func(a, b uuid.UUID) int { return a.Compare(b) })
			if !member || i > 0 && pending[i-1].Compare(id) >= 0 {
				return preparedRule{}, errLintRows
			}
			if err := add(id); err != nil {
				return preparedRule{}, err
			}
		}
		assets = assets[n:]
	}
	return out.finish(ctx, s.binding)
}
