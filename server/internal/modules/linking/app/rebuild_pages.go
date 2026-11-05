package app

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func (r Rebuild) rebuildPages(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (int, int, error) {
	stage := newRebuildStage(len(ids), r.StagingLimit)
	defer stage.release()
	for i, id := range ids {
		p, f, err := r.pageFacts(ctx, notebookID, id)
		if err != nil {
			return 0, 0, err
		}
		if stage != nil {
			admitted, err := stage.add(ctx, i, f.Links)
			if err != nil {
				return 0, 0, err
			}
			if !admitted {
				if err := r.drainRebuildLinks(ctx, notebookID, ids[:i], stage); err != nil {
					return 0, 0, err
				}
				stage.release()
				stage = nil
			}
		}
		if stage == nil {
			err = r.Index.Store.AddPage(ctx, p, f)
		} else {
			err = r.Writes.AddRebuildMetadata(ctx, p, RebuildMetadata{
				FrontmatterValid: f.FrontmatterValid, Tags: f.Tags, Properties: f.Properties, Aliases: f.Aliases,
			})
		}
		if err != nil {
			return 0, 0, err
		}
	}
	if stage != nil {
		return r.resolveRebuildStage(ctx, notebookID, ids, stage)
	}
	// A drained notebook returns to the complete original read/resolve/write
	// path. The stage no longer retains row or string references here.
	links, err := r.Index.Store.Links(ctx, notebookID, domain.Reach{Sources: ids})
	if err != nil {
		return 0, 0, err
	}
	resolved, err := r.Index.resolve(ctx, notebookID, links)
	if err != nil {
		return 0, 0, err
	}
	if err := r.Index.Store.SetResolutions(ctx, resolved); err != nil {
		return 0, 0, err
	}
	return len(links), len(links) - len(resolved), nil
}

func (r Rebuild) drainRebuildLinks(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID, stage *rebuildStage) error {
	for i, id := range ids {
		if err := r.Writes.AddRebuildUnresolvedLinks(ctx, Page{ID: id, NotebookID: notebookID}, stage.links[i]); err != nil {
			return err
		}
		stage.links[i] = nil
	}
	return nil
}

func (r Rebuild) resolveRebuildStage(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID, stage *rebuildStage) (int, int, error) {
	rs, missing, err := r.rebuildResolutions(ctx, notebookID, ids, stage)
	if err != nil {
		return 0, 0, err
	}
	if len(missing) > 0 {
		return 0, 0, fmt.Errorf("linking: the pages %v of links or aliases are not pages of %s", missing, notebookID)
	}
	unresolved := 0
	for i, result := range rs {
		if i%checkEvery == 0 {
			if err := ctx.Err(); err != nil {
				return 0, 0, err
			}
		}
		if result.ID == uuid.Nil() {
			unresolved++
		}
	}
	first := 0
	for _, slot := range stage.order {
		end := first + len(stage.links[slot])
		if err := r.Writes.AddRebuildResolvedLinks(ctx, Page{ID: ids[slot], NotebookID: notebookID}, stage.links[slot], rs[first:end]); err != nil {
			return 0, 0, err
		}
		stage.links[slot] = nil
		first = end
	}
	return len(rs), unresolved, nil
}

// Keep the temporary Link array scoped to resolution. INSERT retains only
// the stage's complete owned rows and the resulting Resolution array.
func (r Rebuild) rebuildResolutions(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID, stage *rebuildStage) ([]domain.Resolution, []uuid.UUID, error) {
	inputs, err := stage.inputs(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	return resolutions(ctx, r.Index.Store, r.Index.Pages, notebookID, inputs, nil, nil)
}
