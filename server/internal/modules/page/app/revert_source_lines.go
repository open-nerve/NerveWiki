package app

import (
	"context"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// sourceLines supplements the content-only fast path with one batch ancestry
// read. The normal planning tree already supplies every structural parent.
func (r revertReads) sourceLines(ctx context.Context, s domain.RevertState, notebook domain.NotebookState) (domain.RevertState, error) {
	if notebook.Mode != domain.ModeLLMWiki || s.Tree != nil {
		return s, nil
	}
	byParent := map[uuid.UUID]uuid.UUID{}
	for id, n := range s.Nodes {
		if n.DeletedAt == nil && n.ParentID != nil {
			byParent[*n.ParentID] = id
		}
	}
	if len(byParent) == 0 {
		return s, nil
	}
	ids := make([]uuid.UUID, 0, len(byParent))
	for _, id := range byParent {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	lines, err := r.trash.TrashLines(ctx, ids)
	if err != nil {
		return domain.RevertState{}, err
	}
	s.Lines = make(map[uuid.UUID][]domain.Ancestor, len(byParent))
	for parent, id := range byParent {
		for _, a := range lines[id] {
			s.Lines[parent] = append(s.Lines[parent], domain.Ancestor{ID: a.ID, Name: a.Name})
		}
	}
	return s, nil
}
