package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// VersionContent is a version and its content, as written.
type VersionContent struct {
	Version domain.Version
	Content string
}

// GetVersion reads a version of a page: GET
// /api/v0/pages/{page_id}/versions/{revision} (M8/P1 design 3.5).
type GetVersion struct {
	r VersionReads
}

// NewGetVersion returns the use case.
func NewGetVersion(r VersionReads) *GetVersion {
	return &GetVersion{r: r}
}

// Execute reads the version of the page id holding revision, with its
// content: the page alive or in the trash, as ListVersions reads it; then
// page.version_not_found when no version holds the revision. The version
// answered has its own revision, which an edit session's version may have
// moved past the one asked for. A read takes no lock and opens no
// transaction: a version the purge took in between is not found.
func (g *GetVersion) Execute(ctx context.Context, id uuid.UUID, revision int) (VersionContent, error) {
	n, err := readableAny(ctx, g.r.Notebooks, g.r.Nodes, g.r.Auth, id, domain.ActionHistory)
	if err != nil {
		return VersionContent{}, err
	}
	v, err := g.r.version(ctx, n.ID, revision)
	if err != nil {
		return VersionContent{}, err
	}
	content, err := g.r.Versions.VersionContent(ctx, v.ID)
	if err != nil {
		return VersionContent{}, found(err, domain.ErrVersionNotFound)
	}
	return VersionContent{Version: v, Content: content}, nil
}
