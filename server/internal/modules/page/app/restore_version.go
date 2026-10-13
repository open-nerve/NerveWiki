package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// VersionRestored is what a restore did: the page's revision as it leaves
// it, and its changeset, zero when the page held the version's content
// already and nothing was written.
type VersionRestored struct {
	Revision    int
	ChangesetID uuid.UUID
}

// RestoreVersion writes a version's content as its page's: POST
// /api/v0/pages/{page_id}/versions/{revision}/restore (M8/P1 design 3.5).
type RestoreVersion struct {
	reads  VersionReads
	writer *Writer
	parser *ContentParser
	logger *slog.Logger
}

// NewRestoreVersion returns the use case.
func NewRestoreVersion(reads VersionReads, writer *Writer, parser *ContentParser, logger *slog.Logger) *RestoreVersion {
	return &RestoreVersion{reads: reads, writer: writer, parser: parser, logger: logger}
}

// Execute writes the content of the page id's version holding revision as
// the page's, on base, the revision the caller read the page at, from
// client, in a unit of its own changeset, a version_restore (M8 design
// 4.4). The codes come in M8 design 4.14's order: a page that does not
// exist, is in the trash, or whose notebook the caller has no role in is
// page.not_found, unlocked; then page.version_not_found; then the write is
// decided, forbidden for a reader, and the content parsed within the
// budget, server_busy; then the unit writes it as a content write does:
// nothing when the page holds it already, whatever the base;
// page.revision_mismatch for a base that is not the page's revision; then
// the guards, page.locked while an edit session holds the page's lock.
func (r *RestoreVersion) Execute(ctx context.Context, id uuid.UUID, revision, base int, client domain.Client) (VersionRestored, error) {
	n, err := readableAny(ctx, r.reads.Notebooks, r.reads.Nodes, r.reads.Auth, id, domain.ActionHistory)
	switch {
	case err != nil:
		return VersionRestored{}, err
	case n.DeletedAt != nil:
		return VersionRestored{}, domain.ErrNotFound
	}
	v, err := findVersion(ctx, r.reads.Versions, n.ID, revision)
	if err != nil {
		return VersionRestored{}, err
	}
	content, err := r.reads.Versions.VersionContent(ctx, v.ID)
	if err != nil {
		return VersionRestored{}, found(err, domain.ErrVersionNotFound)
	}
	spec := UnitSpec{NotebookID: n.NotebookID, Action: domain.ActionRestoreVersion, Client: client,
		Options: Options{UpdateLinks: true}, NotFound: domain.ErrNotFound, Kind: domain.ChangesetVersionRestore}
	facts, release, err := r.parser.Parse(ctx, spec, content)
	if err != nil {
		return VersionRestored{}, err
	}
	defer release()
	var out VersionRestored
	outcome, err := r.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		revision, err := u.WriteContent(ctx, ContentWrite{NodeID: n.ID, Content: content, Facts: facts, Base: base})
		out.Revision = revision
		return err
	})
	if err != nil {
		return VersionRestored{}, err
	}
	out.ChangesetID = outcome.ChangesetID
	if outcome.ChangesetID != (uuid.UUID{}) {
		r.logger.InfoContext(ctx, "page content restored", append(logged(outcome, n.Node, client),
			slog.Int("revision", out.Revision), slog.Int("restored_revision", v.Revision))...)
	}
	return out, nil
}
