package app

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// RevertBounds bound a changeset one unit reverts beyond the pages, bytes
// and links any unit parses, as RestoreBounds bound a restore: its items,
// the rows of history of the trash items it restores, and its round's
// places, deletions and restores. Zero is the domain's, MaxUnitNodes,
// MaxUnitHistory and MaxRevertSteps; only a measurement changes them.
type RevertBounds struct {
	Nodes, History, Steps int
}

// ChangesetReverted is what a revert did: its changeset, zero when it
// changed nothing, as nothing needed to.
type ChangesetReverted struct {
	ChangesetID uuid.UUID
	Changed     bool
}

// RevertChangeset reverts a changeset whole: POST
// /api/v0/changesets/{changeset_id}/revert (M8 design 4.8; M8/P3 design
// 3.6).
type RevertChangeset struct {
	reads   TrashReads
	history History
	writer  *Writer
	parser  *ContentParser
	bounds  RevertBounds
	logger  *slog.Logger
}

// NewRevertChangeset returns the use case, the domain's bound where
// bounds has none.
func NewRevertChangeset(reads TrashReads, history History, writer *Writer, parser *ContentParser, bounds RevertBounds,
	logger *slog.Logger,
) *RevertChangeset {
	bounds.Nodes = cmp.Or(bounds.Nodes, domain.MaxUnitNodes)
	bounds.History = cmp.Or(bounds.History, domain.MaxUnitHistory)
	bounds.Steps = cmp.Or(bounds.Steps, domain.MaxRevertSteps)
	return &RevertChangeset{reads: reads, history: history, writer: writer, parser: parser, bounds: bounds, logger: logger}
}

// Execute reverts the changeset id, from client, with message (nil:
// none) on the revert's changeset, in a unit of its own, a revert. The
// codes come in M8/P3 design 3.6's order, all but the unit's unlocked: a
// message that breaks its rules is 422 validation_failed; a changeset
// that does not exist, whose notebook is deleted, or whose notebook the
// caller has no role in is page.changeset_not_found; a reader forbidden;
// one the purge marked page.changeset_trimmed; one of more items than the
// unit's bound page.revert_too_large, before they are read; then its
// items and versions, and its row again, page.changeset_trimmed when the
// purge marked or took it meanwhile, page.changeset_not_found when its
// notebook went meanwhile, or the caller's role; then the trash items it
// deleted, each read only when its root passes the check, and
// page.revert_too_large past the pages and bytes it parses (a lone page
// never is) or the rows of their history; then the state, planned once
// before parsing: page.revert_conflict, then page.changeset_trimmed for a
// version the purge took, then page.revert_too_large past the round's
// steps; then page.changeset_trimmed for a content to write back or
// restore the purge took; then the contents parsed, largest first,
// server_busy when their waits run out, and page.revert_too_large past
// the unit's links, known only once they are; then the unit, which reads
// and plans again under the lock (Unit.Revert): page.changeset_trimmed,
// page.revert_too_large, page.revert_conflict, page.changeset_trimmed,
// page.revert_too_large, then the guards' page.locked. A restore's rows
// the purge took meanwhile, page.trash_changed in the unit, is
// page.changeset_trimmed here: the purge trimmed the changeset. Nothing
// to change writes nothing: no changeset, changed false.
func (r *RevertChangeset) Execute(ctx context.Context, id uuid.UUID, message *string, client domain.Client) (ChangesetReverted, error) {
	if message != nil {
		if err := domain.CheckMessage(*message); err != nil {
			return ChangesetReverted{}, err
		}
	}
	c, err := r.history.FindChangeset(ctx, id)
	if err != nil {
		return ChangesetReverted{}, found(err, domain.ErrChangesetNotFound)
	}
	spec := UnitSpec{NotebookID: c.NotebookID, Action: domain.ActionRevert, Tree: true, Client: client,
		NotFound: domain.ErrChangesetNotFound, Kind: domain.ChangesetRevert, RevertsID: c.ID, Message: message}
	if err := r.writer.Allowed(ctx, spec); err != nil {
		return ChangesetReverted{}, err
	}
	if c.Trimmed {
		return ChangesetReverted{}, domain.ErrChangesetTrimmed
	}
	n, err := r.history.CountItems(ctx, c.ID, r.bounds.Nodes+1)
	switch {
	case err != nil:
		return ChangesetReverted{}, err
	case n > r.bounds.Nodes:
		return ChangesetReverted{}, domain.ErrRevertTooLarge
	}
	reads := revertReads{history: r.history, trash: r.reads.Trash, nodes: r.reads.Nodes}
	items, versions, err := reads.changeset(ctx, c.ID)
	if errors.Is(err, ErrNotFound) {
		// Gone since it was found: its notebook deleted, or the caller's
		// role ended, answers as the first read would (M8/P3A review B F2);
		// else the purge took it.
		if err := r.writer.Allowed(ctx, spec); err != nil {
			return ChangesetReverted{}, err
		}
		err = domain.ErrChangesetTrimmed
	}
	if err != nil {
		return ChangesetReverted{}, err
	}
	nodes, groups, err := reads.groups(ctx, c.NotebookID, items, versions)
	if err != nil {
		return ChangesetReverted{}, err
	}
	if err := r.withinBounds(ctx, items, versions, nodes, groups); err != nil {
		return ChangesetReverted{}, err
	}
	state, err := reads.rest(ctx, c.NotebookID, domain.RevertState{Items: items, Versions: versions, Nodes: nodes, Groups: groups,
		At: r.reads.Clock.Now(), Retention: r.reads.Retention})
	if err != nil {
		return ChangesetReverted{}, err
	}
	plan, err := planned(state)
	switch {
	case err != nil:
		return ChangesetReverted{}, err
	case plan.Steps() > r.bounds.Steps:
		return ChangesetReverted{}, domain.ErrRevertTooLarge
	}
	contents, err := r.contents(ctx, plan)
	if err != nil {
		return ChangesetReverted{}, err
	}
	facts, release, err := r.parser.DecidedAll(ctx, contents, domain.ErrRevertTooLarge)
	if err != nil {
		return ChangesetReverted{}, err
	}
	defer release()
	read := RevertRead{Changeset: c.ID, Parsed: make(map[uuid.UUID]ParsedContent, len(contents)), Retention: r.reads.Retention,
		MaxItems: r.bounds.Nodes, MaxSteps: r.bounds.Steps}
	for page, content := range contents {
		sum := sha256.Sum256([]byte(content))
		read.Parsed[page] = ParsedContent{Hash: sum[:], Content: content, Facts: facts[page]}
	}
	var done RevertDone
	outcome, err := r.writer.Run(ctx, spec, func(ctx context.Context, u *Unit) error {
		var err error
		done, err = u.Revert(ctx, read)
		return err
	})
	switch {
	case errors.Is(err, domain.ErrTrashChanged):
		return ChangesetReverted{}, domain.ErrChangesetTrimmed
	case err != nil:
		return ChangesetReverted{}, err
	case outcome.ChangesetID == uuid.UUID{}:
		return ChangesetReverted{}, nil
	}
	r.logger.InfoContext(ctx, "changeset reverted", slog.String("workspace_id", outcome.WorkspaceID.String()),
		slog.String("user_id", outcome.By.String()), slog.String("changeset_id", outcome.ChangesetID.String()),
		slog.String("reverted_id", c.ID.String()), slog.String("notebook_id", c.NotebookID.String()), slog.Int("placed", done.Placed),
		slog.Int("deleted", done.Deleted), slog.Int("restored", done.Restored), slog.Int("written", done.Written),
		slog.String("client", string(client)))
	return ChangesetReverted{ChangesetID: outcome.ChangesetID, Changed: true}, nil
}

// withinBounds checks a revert parses at most the unit's pages and bytes,
// those of the versions it writes back and of the pages of the trash
// items it restores, a page each, and restores at most the bound of rows
// of history, its items' roots' at their time: page.revert_too_large.
func (r *RevertChangeset) withinBounds(ctx context.Context, items []domain.ChangesetItem, versions []domain.ChangesetVersion,
	nodes map[uuid.UUID]domain.NodeNow, groups map[uuid.UUID][]domain.TrashMember,
) error {
	pages, bytes, history := 0, 0, 0
	for _, v := range domain.RevertWrites(items, versions) {
		pages, bytes = pages+1, bytes+v.BaseBytes
	}
	for _, g := range groups {
		for _, m := range g {
			if m.Node.Kind == domain.KindPage {
				pages, bytes = pages+1, bytes+m.Bytes
			}
		}
	}
	if domain.UnitTooLarge(pages, bytes, 0) {
		return domain.ErrRevertTooLarge
	}
	for root, g := range groups {
		ids := make([]uuid.UUID, len(g))
		for i, m := range g {
			ids[i] = m.Node.ID
		}
		n, err := r.reads.Trash.TrashHistory(ctx, ids, *nodes[root].DeletedAt, r.bounds.History+1)
		if err != nil {
			return err
		}
		if history += n; history > r.bounds.History {
			return domain.ErrRevertTooLarge
		}
	}
	return nil
}

// contents are the contents the plan writes back or restores, by page: a
// version's, a page's in the trash at its item's time. One the purge took
// is page.changeset_trimmed, not parsed.
func (r *RevertChangeset) contents(ctx context.Context, plan domain.RevertPlan) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	var refs []domain.VersionRef
	for _, op := range plan.Ops {
		switch op.Kind {
		case domain.RevertContent:
			refs = append(refs, domain.VersionRef{NodeID: op.NodeID, Revision: op.Base})
		case domain.RevertRestore:
			var pages []uuid.UUID
			for _, m := range op.Group {
				if m.Node.Kind == domain.KindPage {
					pages = append(pages, m.Node.ID)
				}
			}
			trashed, err := r.reads.Trash.TrashContents(ctx, pages, op.DeletedAt)
			switch {
			case err != nil:
				return nil, err
			case len(trashed) != len(pages):
				return nil, domain.ErrChangesetTrimmed
			}
			for page, content := range trashed {
				out[page] = content
			}
		}
	}
	if len(refs) == 0 {
		return out, nil
	}
	versions, err := r.history.VersionContents(ctx, refs)
	switch {
	case err != nil:
		return nil, err
	case len(versions) != len(refs):
		return nil, domain.ErrChangesetTrimmed
	}
	for ref, v := range versions {
		out[ref.NodeID] = v.Content
	}
	return out, nil
}
