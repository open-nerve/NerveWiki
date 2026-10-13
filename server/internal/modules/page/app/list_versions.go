package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// VersionList is a page of a page's versions, and the cursor of the next,
// "" after the last page.
type VersionList struct {
	Versions   []domain.Version
	NextCursor string
}

// VersionReads are what the reads of a page's versions need.
type VersionReads struct {
	Notebooks Notebooks
	Nodes     Nodes
	Versions  Versions
	Names     Names
	Auth      shared.Authorizer
	Clock     Clock
}

// ListVersions lists a page's versions, a page of them at a time: GET
// /api/v0/pages/{page_id}/versions (M8/P1 design 3.5).
type ListVersions struct {
	r VersionReads
}

// NewListVersions returns the use case.
func NewListVersions(r VersionReads) *ListVersions {
	return &ListVersions{r: r}
}

// Execute returns the page of the page id's versions that limit and cursor
// ask for, newer-first, from the newest when cursor is nil; the page alive
// or in the trash. A cursor that does not decode, or names no revision a
// version may have, is 400 bad_request, judged first; then the page and
// the decision (page.history: any role in the notebook); then the limit,
// 422 outside 1–100. One version more than limit is read to tell whether
// another follows. The newest version is marked editing while the edit
// session whose changeset it is is alive. A read takes no lock and opens
// no transaction.
func (l *ListVersions) Execute(ctx context.Context, id uuid.UUID, limit *int, cursor *string) (VersionList, error) {
	// From the newest: below a revision none has.
	below := domain.MaxRevision + 1
	if cursor != nil {
		var c domain.VersionCursor
		if err := shared.DecodeCursor(*cursor, &c); err != nil {
			return VersionList{}, err
		}
		if c.Below < 1 || c.Below > domain.MaxRevision {
			return VersionList{}, shared.InvalidCursor()
		}
		below = c.Below
	}
	n, err := readableAny(ctx, l.r.Notebooks, l.r.Nodes, l.r.Auth, id, domain.ActionHistory)
	if err != nil {
		return VersionList{}, err
	}
	size, err := shared.PageSize(limit)
	if err != nil {
		return VersionList{}, err
	}
	versions, err := l.r.Versions.ListVersions(ctx, n.ID, below, size+1)
	if err != nil {
		return VersionList{}, err
	}
	var out VersionList
	if len(versions) > size {
		versions = versions[:size]
		if out.NextCursor, err = shared.EncodeCursor(domain.VersionCursor{Below: versions[size-1].Revision}); err != nil {
			return VersionList{}, err
		}
	}
	if cursor == nil && len(versions) > 0 {
		alive, err := l.r.Versions.AliveChangesetOf(ctx, n.ID, l.r.Clock.Now())
		if err != nil {
			return VersionList{}, err
		}
		versions[0].Editing = alive != (uuid.UUID{}) && alive == versions[0].Changeset.ID
	}
	if err := named(ctx, l.r.Names, versions); err != nil {
		return VersionList{}, err
	}
	out.Versions = versions
	return out, nil
}

// named fills the display names of the versions' changesets' actors, read
// at once.
func named(ctx context.Context, names Names, versions []domain.Version) error {
	if len(versions) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(versions))
	seen := map[uuid.UUID]bool{}
	for _, v := range versions {
		if !seen[v.Changeset.By] {
			seen[v.Changeset.By] = true
			ids = append(ids, v.Changeset.By)
		}
	}
	got, err := names.DisplayNames(ctx, ids)
	if err != nil {
		return err
	}
	for i := range versions {
		versions[i].Changeset.ByName = got[versions[i].Changeset.By]
	}
	return nil
}

// findVersion is the page pageID's version holding revision (M8 design
// 4.3), and its content: the first at or above it, if its base is below
// it. domain.ErrVersionNotFound when no version holds it.
func findVersion(ctx context.Context, versions Versions, pageID uuid.UUID, revision int) (domain.Version, string, error) {
	if revision < 1 || revision > domain.MaxRevision {
		return domain.Version{}, "", domain.ErrVersionNotFound
	}
	v, content, err := versions.FindVersion(ctx, pageID, revision)
	switch {
	case err != nil:
		return domain.Version{}, "", found(err, domain.ErrVersionNotFound)
	case !v.Holds(revision):
		return domain.Version{}, "", domain.ErrVersionNotFound
	}
	return v, content, nil
}
