package app

import (
	"context"
	"errors"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// SourcePreparationUnit is one locked Index.PagesChanged call, never a cache
// key shared between calls. Its nonzero-size identity binds each prepared plan.
type SourcePreparationUnit struct{ NotebookID uuid.UUID }

// SourceLinks supplies every final row of one written source. A non-nil
// envelope with no Links is complete empty; nil means the SQL must supply it.
type SourceLinks struct {
	SourceID uuid.UUID
	Links    []Link
}

// SourcePreparations is explicitly wired beside Store. NewSource performs no
// SQL and returns nil when its input gate cannot prepare. Facts remain owned
// by this write unit and read-only until Apply returns. ReadBefore is called
// only for a source whose complete Before/current-head proof has succeeded.
type SourcePreparations interface {
	NewSource(context.Context, *SourcePreparationUnit, Page, domain.Facts) PreparedSource
	LinksExcept(context.Context, uuid.UUID, domain.Reach, []uuid.UUID) ([]Link, error)
}

// PreparedSource belongs to the supplied unit and actual transaction. A
// successful ReadBefore reaches native EOF/Err/Close before returning every
// asset-flagged old body ID, still including ineligible metadata targets.
// Apply consumes once, at the original Replace stage. A complete negative
// preparation runs legacy directly, never another Inspect. Neither method
// returns partial results after a native error; old resolution is only a
// comparison baseline and every supplied link must be freshly resolved.
type PreparedSource interface {
	ReadBefore(context.Context, *SourcePreparationUnit) ([]uuid.UUID, error)
	Apply(context.Context, *SourcePreparationUnit) (Dropped, *SourceLinks, error)
}

type sourcePreparationSet struct {
	factory  SourcePreparations
	unit     *SourcePreparationUnit
	changes  map[uuid.UUID]domain.Change
	plans    map[uuid.UUID]PreparedSource
	supplied []*SourceLinks
}

func newSourcePreparationSet(e PagesChanged, factory SourcePreparations) *sourcePreparationSet {
	if factory == nil {
		return nil
	}
	counts := make(map[uuid.UUID]int, len(e.Changes))
	for _, c := range e.Changes {
		counts[c.NodeID]++
	}
	p := &sourcePreparationSet{factory: factory, unit: &SourcePreparationUnit{NotebookID: e.NotebookID},
		changes: make(map[uuid.UUID]domain.Change), plans: make(map[uuid.UUID]PreparedSource)}
	for _, c := range e.Changes {
		if counts[c.NodeID] == 1 && c.Before != nil && c.After != nil && c.Revision != 0 {
			p.changes[c.NodeID] = c
		}
	}
	return p
}

func (p *sourcePreparationSet) plan(ctx context.Context, id uuid.UUID) bool {
	if p == nil {
		return false
	}
	c, ok := p.changes[id]
	if !ok {
		return false
	}
	page := Page{ID: id, NotebookID: p.unit.NotebookID, Revision: c.Revision}
	plan := p.factory.NewSource(ctx, p.unit, page, c.Facts)
	if plan == nil {
		return false
	}
	p.plans[id] = plan
	return true
}

func (u *ingestUnit) replace(ctx context.Context, store Store, page Page, facts domain.Facts) (Dropped, error) {
	if u == nil || u.prepared == nil || u.prepared.plans[page.ID] == nil {
		return store.ReplacePage(ctx, page, facts)
	}
	p := u.prepared
	old, supplied, err := p.plans[page.ID].Apply(ctx, p.unit)
	if err != nil {
		return Dropped{}, err
	}
	delete(p.plans, page.ID)
	if supplied != nil {
		if supplied.SourceID != page.ID {
			return Dropped{}, errors.New("prepared source: wrong supplied source")
		}
		p.supplied = append(p.supplied, supplied)
	}
	return old, nil
}

func (u *ingestUnit) links(ctx context.Context, store Store, notebook uuid.UUID, reach domain.Reach) ([]Link, error) {
	if u == nil || u.prepared == nil || len(u.prepared.supplied) == 0 {
		return store.Links(ctx, notebook, reach)
	}
	p := u.prepared
	// Sort only source envelopes. Each source's range order belongs to its
	// preparation, and the residual SQL is ordered already; never sort 4M rows.
	slices.SortFunc(p.supplied, func(a, b *SourceLinks) int { return a.SourceID.Compare(b.SourceID) })
	ids := make([]uuid.UUID, len(p.supplied))
	for i, s := range p.supplied {
		ids[i] = s.SourceID
	}
	residual, err := p.factory.LinksExcept(ctx, notebook, reach, ids)
	if err != nil {
		return nil, err
	}
	out, err := mergeSourceLinks(p.supplied, residual)
	p.supplied = nil
	return out, err
}

// Merge complete source blocks and residual native rows in linear time.
// residual is the complete native LinksExcept relation ordered by source/start;
// that SQL ordering contract is the same as Store.Links.
// An excluded source in residual is a protocol failure, even at a new start.
func mergeSourceLinks(supplied []*SourceLinks, residual []Link) ([]Link, error) {
	n := len(residual)
	for i, s := range supplied {
		if i > 0 && supplied[i-1].SourceID.Compare(s.SourceID) >= 0 {
			return nil, errors.New("prepared sources: unordered sources")
		}
		for j, row := range s.Links {
			if row.SourceID != s.SourceID || j > 0 && s.Links[j-1].Start >= row.Start {
				return nil, errors.New("prepared sources: unordered source rows")
			}
		}
		n += len(s.Links)
	}
	out := make([]Link, 0, n)
	at := 0
	for _, s := range supplied {
		for at < len(residual) && residual[at].SourceID.Compare(s.SourceID) < 0 {
			out = append(out, residual[at])
			at++
		}
		if at < len(residual) && residual[at].SourceID == s.SourceID {
			return nil, errors.New("prepared sources: residual repeats supplied source")
		}
		out = append(out, s.Links...)
	}
	out = append(out, residual[at:]...)
	return out, nil
}
