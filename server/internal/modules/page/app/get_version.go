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
// content, read at once: the page alive or in the trash, as ListVersions
// reads it; then page.version_not_found when no version holds the
// revision. The version answered has its own revision, which an edit
// session's version may have moved past the one asked for; it is being
// edited while it is the page's newest and its session is alive. A read
// takes no lock and opens no transaction.
func (g *GetVersion) Execute(ctx context.Context, id uuid.UUID, revision int) (VersionContent, error) {
	n, err := readableAny(ctx, g.r.Notebooks, g.r.Nodes, g.r.Auth, id, domain.ActionHistory)
	if err != nil {
		return VersionContent{}, err
	}
	v, content, err := findVersion(ctx, g.r.Versions, n.ID, revision)
	if err != nil {
		return VersionContent{}, err
	}
	alive, err := g.r.Versions.AliveChangesetOf(ctx, n.ID, g.r.Clock.Now())
	if err != nil {
		return VersionContent{}, err
	}
	if alive != (uuid.UUID{}) && alive == v.Changeset.ID {
		newest, err := g.r.Versions.ListVersions(ctx, n.ID, domain.MaxRevision+1, 1)
		if err != nil {
			return VersionContent{}, err
		}
		v.Editing = len(newest) == 1 && newest[0].ID == v.ID
	}
	versions := []domain.Version{v}
	if err := named(ctx, g.r.Names, versions); err != nil {
		return VersionContent{}, err
	}
	return VersionContent{Version: versions[0], Content: content}, nil
}
