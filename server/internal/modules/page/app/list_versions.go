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

// VersionReads are what the reads of a page's versions need; by its path,
// MCP's versions (M9/P2 design 3.16.1), Paths finding the page and States
// reading its version now.
type VersionReads struct {
	Notebooks Notebooks
	Nodes     Nodes
	Versions  Versions
	Names     Names
	Auth      shared.Authorizer
	Clock     Clock
	Paths     PathStore
	States    PageStates
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
	below, err := versionsBelow(cursor)
	if err != nil {
		return VersionList{}, err
	}
	n, err := readableAny(ctx, l.r.Notebooks, l.r.Nodes, l.r.Auth, id, domain.ActionHistory)
	if err != nil {
		return VersionList{}, err
	}
	size, err := shared.PageSize(limit)
	if err != nil {
		return VersionList{}, err
	}
	return l.r.list(ctx, n.ID, below, size, cursor == nil)
}

// PathVersionList is versions' list (M9 design 4.9): the page at its
// path, its version now, and a page of its versions, each with the cursor
// that goes on after it.
type PathVersionList struct {
	Path       string
	Node       domain.Node
	Current    domain.VersionToken
	Versions   []domain.Version
	Cursors    []string
	NextCursor string
}

// AtPath lists the versions of the page at path in notebookID as Execute
// lists a page's (M9/P2 design 3.16.1): the page alive, found by its path,
// unlocked. Its order: the actor; the cursor, 400; the path, 422, an
// attachment's refused; the limit, 422; the notebook, notebook.not_found
// (page.history, which every role may: no 403); the page, page.not_found;
// then Execute's reads, and the page's version now, page.not_found when
// the page went since it was found.
func (l *ListVersions) AtPath(ctx context.Context, notebookID uuid.UUID, path string, limit *int, cursor *string) (
	PathVersionList, error,
) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return PathVersionList{}, err
	}
	below, err := versionsBelow(cursor)
	if err != nil {
		return PathVersionList{}, err
	}
	p, err := versionsPath(path)
	if err != nil {
		return PathVersionList{}, err
	}
	size, err := shared.PageSize(limit)
	if err != nil {
		return PathVersionList{}, err
	}
	if err := decideNotebook(ctx, l.r.Notebooks, l.r.Auth, actor, domain.ActionHistory, notebookID); err != nil {
		return PathVersionList{}, err
	}
	at, err := NewNodePaths(l.r.Paths).Find(ctx, notebookID, p)
	if err != nil {
		return PathVersionList{}, err
	}
	list, err := l.r.list(ctx, at.Node.ID, below, size, cursor == nil)
	if err != nil {
		return PathVersionList{}, err
	}
	cursors, err := cursorsAfter(list.Versions, func(v domain.Version) any { return domain.VersionCursor{Below: v.Revision} })
	if err != nil {
		return PathVersionList{}, err
	}
	current, err := l.r.current(ctx, at.Node.ID, p)
	if err != nil {
		return PathVersionList{}, err
	}
	return PathVersionList{Path: domain.PathOf(at.Ancestors, at.Node), Node: at.Node, Current: current,
		Versions: list.Versions, Cursors: cursors, NextCursor: list.NextCursor}, nil
}

// versionsBelow is the revision a page of versions lists below: cursor's,
// from the newest without one. A cursor that does not decode, or names no
// revision a version may have, is 400 bad_request.
func versionsBelow(cursor *string) (int, error) {
	if cursor == nil {
		return domain.MaxRevision + 1, nil // below a revision none has
	}
	var c domain.VersionCursor
	if err := shared.DecodeCursor(*cursor, &c); err != nil {
		return 0, err
	}
	if c.Below < 1 || c.Below > domain.MaxRevision {
		return 0, shared.InvalidCursor()
	}
	return c.Below, nil
}

// versionsPath is path as versions takes it (M9/P2 design 3.16.1): a
// page's, 422 on path otherwise; an attachment's name never ends with
// ".md", so a page's path finds pages alone. The refusal comes before any
// read, so it cannot tell a page's path written without ".md" from an
// attachment's: it tells the rule of the hint read and log give for a
// page there (S2 review B-N1). It quotes no path: MCP's answer holds a
// field's message three times, and with the longest path that is over
// the answer's bound, where a not found's hint is dropped to fit.
func versionsPath(path string) (domain.Path, error) {
	p, err := domain.ParsePath("path", path, false)
	if err != nil {
		return domain.Path{}, err
	}
	if p.Kind != domain.KindPage {
		return domain.Path{}, domain.NotAllowed("path", "is an attachment's path: attachments have no versions. "+domain.PageRule+".")
	}
	return p, nil
}

// list is a page of the page id's versions, newer first, below the
// revision below, size at most: one version more is read to tell whether
// another follows. On the first page the newest is marked editing while
// the edit session whose changeset it is is alive. Their actors' names
// are read at once.
func (r VersionReads) list(ctx context.Context, id uuid.UUID, below, size int, first bool) (VersionList, error) {
	versions, err := r.Versions.ListVersions(ctx, id, below, size+1)
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
	if first && len(versions) > 0 {
		alive, err := r.Versions.AliveChangesetOf(ctx, id, r.Clock.Now())
		if err != nil {
			return VersionList{}, err
		}
		versions[0].Editing = alive != (uuid.UUID{}) && alive == versions[0].Changeset.ID
	}
	if err := named(ctx, r.Names, versions); err != nil {
		return VersionList{}, err
	}
	out.Versions = versions
	return out, nil
}

// current is the page id's version now, as list and read give it: a page
// that went since it was found at p is not found there.
func (r VersionReads) current(ctx context.Context, id uuid.UUID, p domain.Path) (domain.VersionToken, error) {
	states, err := r.States.PageStates(ctx, []uuid.UUID{id})
	if err != nil {
		return domain.VersionToken{}, err
	}
	s, ok := states[id]
	if !ok {
		return domain.VersionToken{}, &domain.PathNotFound{Path: p.Written}
	}
	return domain.VersionToken{Node: id, Revision: s.Revision}, nil
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
