package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// withSourceHashes prepares the guard and writer's shared metadata before any
// write. Membership comes from each changed parent, including restore forests.
// Source-out only needs the changed IDs; unchanged membership needs no work
// unless this step writes content. Deletion clears in the deletion SQL itself.
func (u *Unit) withSourceHashes(ctx context.Context, s Step) (Step, error) {
	if s.Notebook.Mode != domain.ModeLLMWiki || len(s.Changes) == 0 || s.Operation == domain.OpDelete {
		return s, nil
	}
	var read []uuid.UUID
	facts := s.contractFacts()
	for _, n := range facts.Nodes {
		before, after := n.Before != nil && n.Before.InArea, n.After != nil && n.After.InArea
		maintain := before != after || after && s.Content != nil
		if !maintain {
			continue
		}
		if s.AreaHashes == nil {
			s.AreaHashes = make(map[uuid.UUID]SourceValue)
		}
		v := SourceValue{ID: n.ID, Kind: n.Kind}
		if after {
			switch {
			case s.Operation == domain.OpCreate && n.Kind == domain.KindPage && s.Content != nil:
				v.Hash, v.Bytes = s.Content.Hash, int64(s.Content.Bytes)
			case s.Operation == domain.OpCreate && n.Kind == domain.KindAsset && s.Asset != nil:
				// The blob callback has not run yet. Its facts belong to this step.
				v.Hash, v.Bytes = s.Asset.SHA256, s.Asset.Bytes
			default:
				read = append(read, n.ID)
			}
		}
		s.AreaHashes[n.ID] = v
	}
	for ids := range slices.Chunk(read, domain.MaxUnitNodes) {
		heads, err := u.w.d.Sources.SourceHeads(ctx, s.NotebookID, ids)
		if err != nil {
			return Step{}, err
		}
		var assets []uuid.UUID
		for _, id := range ids {
			v, ok := heads[id]
			if !ok || v.Kind != s.AreaHashes[id].Kind {
				return Step{}, fmt.Errorf("source metadata missing for node %s", id)
			}
			if v.Kind == domain.KindAsset {
				assets = append(assets, id)
			} else if s.Content != nil {
				v.Hash, v.Bytes = s.Content.Hash, int64(s.Content.Bytes)
			}
			s.AreaHashes[id] = v
		}
		if len(assets) > 0 {
			infos, err := u.w.d.Attachments.Of(ctx, assets)
			if err != nil {
				return Step{}, err
			}
			for _, id := range assets {
				info, ok := infos[id]
				if !ok {
					return Step{}, fmt.Errorf("source attachment metadata missing for node %s", id)
				}
				v := s.AreaHashes[id]
				v.Hash, v.Bytes = info.SHA256, info.Bytes
				s.AreaHashes[id] = v
			}
		}
	}
	// Only eligible after-values require metadata; source-out's nil is intentional.
	for _, n := range facts.Nodes {
		v, ok := s.AreaHashes[n.ID]
		if !ok || n.After == nil || !n.After.InArea {
			continue
		}
		if len(v.Hash) != sha256.Size || v.Bytes < 0 {
			return Step{}, fmt.Errorf("invalid source metadata for node %s", n.ID)
		}
		if v.Kind == domain.KindPage && v.Bytes == 0 {
			v.Hash = nil
			s.AreaHashes[n.ID] = v
		}
	}
	return s, nil
}

// writeSourceHashes runs after the row/content write and before item recording
// or participants, so subsequent operations in this unit see the current value.
func (u *Unit) writeSourceHashes(ctx context.Context, s Step) error {
	if len(s.AreaHashes) == 0 {
		return nil
	}
	values := make([]SourceValue, 0, len(s.AreaHashes))
	for _, change := range s.Changes {
		if value, ok := s.AreaHashes[change.NodeID]; ok {
			values = append(values, value)
		}
	}
	for chunk := range slices.Chunk(values, domain.MaxUnitNodes) {
		old, err := u.w.d.Sources.SetSourceHashes(ctx, s.NotebookID, chunk)
		if err != nil {
			return err
		}
		for id, v := range old {
			u.rememberSource(id, v)
		}
	}
	return nil
}

func (u *Unit) rememberSource(id uuid.UUID, v SourceValue) {
	key, ok := sourceKey(v.Kind, v.BeforeSourceHash)
	var before *SourceKey
	if ok {
		before = &key
	}
	u.write.sources.remember(id, before)
}

// Remember known ineligible arrivals before restoring/creating their rows.
// Deletions capture the derived key before their existing SQL clears it.
func (u *Unit) rememberSourceBefore(ctx context.Context, s Step) error {
	if s.Notebook.Mode != domain.ModeLLMWiki {
		return nil
	}
	var ids []uuid.UUID
	for _, n := range s.contractFacts().Nodes {
		if _, known := u.write.sources.before[n.ID]; known {
			continue
		}
		if n.Before == nil || !n.Before.InArea {
			u.write.sources.remember(n.ID, nil)
			continue
		}
		if s.Operation == domain.OpDelete {
			ids = append(ids, n.ID)
		}
	}
	for chunk := range slices.Chunk(ids, domain.MaxUnitNodes) {
		rows, err := u.w.d.Sources.SourceBaselines(ctx, s.NotebookID, chunk)
		if err != nil {
			return err
		}
		for _, id := range chunk {
			v, ok := rows[id]
			if !ok {
				return fmt.Errorf("source baseline missing for %s", id)
			}
			u.rememberSource(id, v)
		}
	}
	return nil
}
