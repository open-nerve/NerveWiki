package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
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

// PathVersion is a version as versions reads it, as ReadPath reads a page
// (M9/P2 design 3.16.1): the page at its path and its version now, the
// version's own revision, maybe above the one asked for, the window asked
// for, the slice of the version's content in it, and the content's bytes.
type PathVersion struct {
	Path     string
	Node     domain.Node
	Current  domain.VersionToken
	Revision int
	Window   domain.Window
	Slice    domain.Slice
	Bytes    int
}

// AtPath reads the version of the page at path in notebookID holding
// revision, as Execute finds a page's and ReadPath cuts a page's content
// (M9/P2 design 3.16.1): the page alive, found by its path, unlocked;
// whether the version is being edited and who wrote it are not read. Its
// order: the actor; the path, 422, an attachment's refused; the revision,
// 422 outside 1–MaxRevision; the window, 422; the notebook,
// notebook.not_found (page.history); the page, page.not_found; the
// version, page.version_not_found; the column on its line, 422; the slice,
// within m's room; then the page's version now, page.not_found when the
// page went since it was found.
func (g *GetVersion) AtPath(ctx context.Context, notebookID uuid.UUID, path string, revision int, offset, column, limit *int,
	m TextMeasure,
) (PathVersion, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return PathVersion{}, err
	}
	p, err := versionsPath(path)
	if err != nil {
		return PathVersion{}, err
	}
	if err := domain.CheckRevision(revision); err != nil {
		return PathVersion{}, err
	}
	w, err := domain.CheckWindow(offset, column, limit)
	if err != nil {
		return PathVersion{}, err
	}
	if err := decideNotebook(ctx, g.r.Notebooks, g.r.Auth, actor, domain.ActionHistory, notebookID); err != nil {
		return PathVersion{}, err
	}
	at, err := NewNodePaths(g.r.Paths).Find(ctx, notebookID, p)
	if err != nil {
		return PathVersion{}, err
	}
	v, content, err := findVersion(ctx, g.r.Versions, at.Node.ID, revision)
	if err != nil {
		return PathVersion{}, err
	}
	out := PathVersion{Path: domain.PathOf(at.Ancestors, at.Node), Node: at.Node, Revision: v.Revision, Window: w,
		Bytes: len(content)}
	if out.Slice, err = domain.Cut(content, w, m.Room(out.Path), m.Bytes); err != nil {
		return PathVersion{}, err
	}
	if out.Current, err = g.r.current(ctx, at.Node.ID, p); err != nil {
		return PathVersion{}, err
	}
	return out, nil
}
