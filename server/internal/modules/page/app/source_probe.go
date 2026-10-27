package app

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// probeSourceWitnesses excludes every planned ID, with each key's initial
// eligible count bounding the only possible excluded index hits. Every key
// starts with one row; subsequent requests spend only its remaining E + 1.
func probeSourceWitnesses(ctx context.Context, reads SourceReads, notebook uuid.UUID, keys []SourceKey, initial map[uuid.UUID]SourceValue) (map[SourceKey]uuid.UUID, error) {
	type cursor struct {
		key           SourceKey
		after         *uuid.UUID
		remaining     int
		started, done bool
	}
	counts := map[SourceKey]int{}
	for _, v := range initial {
		if key, ok := sourceKey(v.Kind, v.BeforeSourceHash); ok {
			counts[key]++
		}
	}
	states := make([]cursor, 0, len(keys))
	seen := map[SourceKey]bool{}
	for _, key := range keys {
		if !seen[key] {
			seen[key] = true
			states = append(states, cursor{key: key, remaining: counts[key]})
		}
	}
	out := map[SourceKey]uuid.UUID{}
	for {
		var probes []SourceProbe
		var at []int
		room := domain.MaxUnitNodes
		for i, s := range states {
			if s.done {
				continue
			}
			take := 1
			if s.started {
				take = min(s.remaining+1, domain.MaxUnitNodes)
			}
			take = min(take, room)
			probes = append(probes, SourceProbe{Key: s.key, After: s.after, Take: take})
			at = append(at, i)
			room -= take
			if room == 0 {
				break
			}
		}
		if len(probes) == 0 {
			return out, nil
		}
		rows, err := reads.ProbeSources(ctx, notebook, probes)
		if err != nil {
			return nil, err
		}
		returned := make([]int, len(probes))
		for _, row := range rows {
			if row.Probe < 0 || row.Probe >= len(probes) {
				return nil, fmt.Errorf("source probe returned an invalid ordinal")
			}
			j := row.Probe
			s := &states[at[j]]
			returned[j]++
			if returned[j] > probes[j].Take {
				return nil, fmt.Errorf("source probe exceeded its take")
			}
			if s.done {
				continue
			}
			if s.after != nil && row.ID.Compare(*s.after) <= 0 {
				return nil, fmt.Errorf("source probe did not advance its cursor")
			}
			id := row.ID
			s.after = &id
			if _, excluded := initial[id]; !excluded {
				out[s.key] = id
				s.done = true
				continue
			}
			s.remaining--
			if s.remaining < 0 {
				return nil, fmt.Errorf("source probe exceeded its initial exclusion count")
			}
		}
		for j, i := range at {
			states[i].started = true
			if returned[j] < probes[j].Take {
				states[i].done = true
			}
		}
	}
}

func sourceKey(kind domain.Kind, hash []byte) (SourceKey, bool) {
	if len(hash) != 32 {
		return SourceKey{}, false
	}
	return SourceKey{Kind: kind, Hash: [32]byte(hash)}, true
}
func sourceValueKey(v SourceValue) (SourceKey, bool) {
	if v.Kind == domain.KindPage && v.Bytes == 0 {
		return SourceKey{}, false
	}
	return sourceKey(v.Kind, v.Hash)
}
