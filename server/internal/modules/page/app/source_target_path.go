package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// SourceTargetPath resolves an optional duplicate hint using the current actor.
type SourceTargetPath struct {
	notebooks Notebooks
	paths     NodePaths
	auth      shared.Authorizer
}

func NewSourceTargetPath(notebooks Notebooks, paths PathStore, auth shared.Authorizer) *SourceTargetPath {
	return &SourceTargetPath{notebooks: notebooks, paths: NewNodePaths(paths), auth: auth}
}
func (r *SourceTargetPath) Execute(ctx context.Context, notebookID, nodeID uuid.UUID) (string, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return "", err
	}
	if err := decideNotebook(ctx, r.notebooks, r.auth, actor, domain.ActionRead, notebookID); err != nil {
		return "", err
	}
	paths, err := r.paths.Paths(ctx, notebookID, []uuid.UUID{nodeID})
	if err != nil {
		return "", err
	}
	path, ok := paths[nodeID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return path, nil
}
