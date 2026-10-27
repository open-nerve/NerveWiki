package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// A present nil is a known ineligible baseline. Absence means this unit has
// not changed the node; its current eligible key is still its baseline.
type sourceMemo struct{ before map[uuid.UUID]*SourceKey }

func (m *sourceMemo) remember(id uuid.UUID, key *SourceKey) {
	if m.before == nil {
		m.before = map[uuid.UUID]*SourceKey{}
	}
	if _, ok := m.before[id]; ok {
		return
	}
	if key != nil {
		k := *key
		key = &k
	}
	m.before[id] = key
}
func (m *sourceMemo) target(id uuid.UUID, key SourceKey, current *SourceKey) *uuid.UUID {
	baseline := current
	if m != nil {
		if old, ok := m.before[id]; ok {
			baseline = old
		}
	}
	if baseline == nil || *baseline != key {
		return nil
	}
	target := id
	return &target
}
func sourceAdmission(f domain.StepFacts, n domain.NodeFacts) bool {
	return n.After != nil && n.After.InArea && (n.Before == nil || !n.Before.InArea || f.Content != nil && !f.Rewrite)
}

// checkSourceDuplicates runs in the caller's existing transaction, never a
// nested snapshot. Internal and indexed witnesses use the same baseline rule.
func (g *ModeGuard) checkSourceDuplicates(ctx context.Context, s Step) error {
	facts := s.contractFacts()
	var keys []SourceKey
	initial := map[uuid.UUID]SourceValue{}
	for _, n := range facts.Nodes {
		if sourceAdmission(facts, n) {
			if v, ok := s.AreaHashes[n.ID]; ok {
				if key, eligible := sourceValueKey(v); eligible {
					keys = append(keys, key)
					initial[n.ID] = v
				}
			}
		}
	}
	if len(keys) == 0 {
		return nil
	}
	if !s.Tree {
		if err := g.hashes.LockSources(ctx, s.NotebookID); err != nil {
			return err
		}
	}
	witnesses, err := probeSourceWitnesses(ctx, g.hashes, s.NotebookID, keys, initial)
	if err != nil {
		return err
	}
	earlier := map[SourceKey]uuid.UUID{}
	for _, n := range facts.Nodes {
		v, ok := initial[n.ID]
		if !ok {
			continue
		}
		key, _ := sourceValueKey(v)
		if id, ok := earlier[key]; ok {
			old := initial[id]
			before, eligible := sourceKey(old.Kind, old.BeforeSourceHash)
			var current *SourceKey
			if eligible {
				current = &before
			}
			return domain.Refuse(domain.SourceDuplicate(s.sources.target(id, key, current)), n.ID)
		}
		if id, ok := witnesses[key]; ok {
			return domain.Refuse(domain.SourceDuplicate(s.sources.target(id, key, &key)), n.ID)
		}
		earlier[key] = n.ID
	}
	return nil
}
