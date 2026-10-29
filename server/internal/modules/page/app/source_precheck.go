package app

import (
	"context"
	"fmt"
	"iter"
	"maps"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type SourcePrecheckDeps struct {
	Snapshots   shared.Snapshots
	Sources     SourceReads
	Attachments AttachmentInfos
}
type SourcePrecheck struct{ d SourcePrecheckDeps }

func NewSourcePrecheck(d SourcePrecheckDeps) *SourcePrecheck { return &SourcePrecheck{d: d} }

// Check is exclusively a lock-free entry. The sequence is replayable: each
// traversal starts from the same initial tree, never the prior traversal's
// mutated simulation. A bounded plan's facts are copied once; larger plans
// retain only per-ID metadata and per-key witnesses between traversals.
// Permission reads must use the context passed to ask, which owns the snapshot.
func (p *SourcePrecheck) Check(ctx context.Context, notebookID uuid.UUID, notebook domain.NotebookState, facts iter.Seq[domain.StepFacts], ask func(context.Context, shared.Action) error) error {
	if notebook.Mode != domain.ModeLLMWiki {
		return nil
	}
	return p.d.Snapshots.WithinSnapshot(ctx, func(ctx context.Context) error {
		return p.check(ctx, notebookID, notebook, facts, func(action shared.Action) error { return ask(ctx, action) })
	})
}
func (p *SourcePrecheck) CheckJob(ctx context.Context, notebookID uuid.UUID, notebook domain.NotebookState, checked revertChecked, plan JobPlan, cuts [][2]int, ask func(context.Context, shared.Action) error) error {
	return p.Check(ctx, notebookID, notebook, jobRevertFacts(checked, plan, cuts, notebook.SourceRootID), ask)
}

// check also serves a caller already inside its single read-only snapshot.
func (p *SourcePrecheck) check(ctx context.Context, notebookID uuid.UUID, notebook domain.NotebookState, facts iter.Seq[domain.StepFacts], ask func(shared.Action) error) error {
	initial := map[uuid.UUID]SourceValue{}
	assets := map[uuid.UUID]bool{}
	work := false
	var replay sourceFactReplay
	for step := range facts {
		replay.remember(step)
		for _, n := range step.Nodes {
			initial[n.ID] = SourceValue{ID: n.ID, Kind: n.Kind}
			if sourceAdmission(step, n) {
				work = true
				if n.Kind == domain.KindAsset {
					assets[n.ID] = true
				}
			}
		}
	}
	facts = replay.sequence(facts)
	if !work {
		return checkRevertFacts(facts, notebook, ask)
	}
	ids := slices.SortedFunc(maps.Keys(initial), uuid.UUID.Compare)
	for chunk := range slices.Chunk(ids, domain.MaxUnitNodes) {
		heads, err := p.d.Sources.SourceHeads(ctx, notebookID, chunk)
		if err != nil {
			return err
		}
		for _, id := range chunk {
			v, ok := heads[id]
			if !ok || v.Kind != initial[id].Kind {
				return fmt.Errorf("source precheck metadata missing for %s", id)
			}
			if v.Kind == domain.KindAsset && len(v.BeforeSourceHash) == 32 {
				v.Hash = v.BeforeSourceHash
			}
			initial[id] = v
		}
	}
	for chunk := range slices.Chunk(slices.SortedFunc(maps.Keys(assets), uuid.UUID.Compare), domain.MaxUnitNodes) {
		infos, err := p.d.Attachments.Of(ctx, chunk)
		if err != nil {
			return err
		}
		for _, id := range chunk {
			info, ok := infos[id]
			if !ok {
				return fmt.Errorf("source precheck asset metadata missing for %s", id)
			}
			v := initial[id]
			v.Hash, v.Bytes = info.SHA256, info.Bytes
			initial[id] = v
		}
	}
	// Every key that may be admitted is known before probing. Content facts
	// update only this pass's body map; the final pass starts afresh below.
	bodies := maps.Clone(initial)
	unique := map[SourceKey]bool{}
	var keys []SourceKey
	for step := range facts {
		for _, n := range step.Nodes {
			v := sourceBody(bodies[n.ID], step, n)
			bodies[n.ID] = v
			if sourceAdmission(step, n) {
				key, ok, err := checkedSourceKey(v)
				if err != nil {
					return err
				}
				if ok && !unique[key] {
					unique[key] = true
					keys = append(keys, key)
				}
			}
		}
	}
	witnesses, err := probeSourceWitnesses(ctx, p.d.Sources, notebookID, keys, initial)
	if err != nil {
		return err
	}
	return simulateSourceFacts(notebook, facts, initial, witnesses, ask)
}
func sourceBody(v SourceValue, step domain.StepFacts, n domain.NodeFacts) SourceValue {
	if step.Content != nil && n.Kind == domain.KindPage {
		v.Hash, v.Bytes = step.Content.Hash, int64(step.Content.Bytes)
	}
	return v
}
func checkedSourceKey(v SourceValue) (SourceKey, bool, error) {
	if len(v.Hash) != 32 || v.Bytes < 0 {
		return SourceKey{}, false, fmt.Errorf("invalid source metadata for node %s", v.ID)
	}
	key, ok := sourceValueKey(v)
	return key, ok, nil
}

func simulateSourceFacts(notebook domain.NotebookState, facts iter.Seq[domain.StepFacts], initial map[uuid.UUID]SourceValue, witnesses map[SourceKey]uuid.UUID, ask func(shared.Action) error) error {
	bodies := maps.Clone(initial)
	current := map[uuid.UUID]SourceKey{}
	members := map[SourceKey]map[uuid.UUID]bool{}
	add := func(id uuid.UUID, key SourceKey) {
		if members[key] == nil {
			members[key] = map[uuid.UUID]bool{}
		}
		members[key][id] = true
		current[id] = key
	}
	for id, v := range initial {
		if key, ok := sourceKey(v.Kind, v.BeforeSourceHash); ok {
			add(id, key)
		}
	}
	for step := range facts {
		verdict := domain.CheckContract(notebook, step)
		if verdict.Err != nil {
			return domain.Refuse(verdict.Err, verdict.NodeID)
		}
		for _, n := range step.Nodes {
			if key, ok := current[n.ID]; ok {
				delete(members[key], n.ID)
				delete(current, n.ID)
			}
		}
		for _, n := range step.Nodes {
			v := sourceBody(bodies[n.ID], step, n)
			bodies[n.ID] = v
			if n.After == nil || !n.After.InArea {
				continue
			}
			key, eligible := sourceValueKey(v)
			if !eligible {
				continue
			}
			if sourceAdmission(step, n) {
				if id, ok := witnesses[key]; ok {
					return domain.Refuse(domain.SourceDuplicate(&id), n.ID)
				}
				var first uuid.UUID
				found := false
				for id := range members[key] {
					if !found || id.Compare(first) < 0 {
						first, found = id, true
					}
				}
				if found {
					old := initial[first]
					baseline, eligible := sourceKey(old.Kind, old.BeforeSourceHash)
					var target *uuid.UUID
					if eligible && baseline == key {
						target = &first
					}
					return domain.Refuse(domain.SourceDuplicate(target), n.ID)
				}
			}
			add(n.ID, key)
		}
		if verdict.Needs != "" {
			err := ask(verdict.Needs)
			if isForbidden(err) {
				return domain.Refuse(domain.ErrSourceAdminOnly, verdict.NodeID)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
