package app

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// RevertBounds bound a changeset one unit reverts beyond the pages, bytes
// and links any unit parses, as RestoreBounds bound a restore: its items,
// the rows of history of the trash items it restores, and its schedule's
// places, steps aside, deletions and restores. Zero is the domain's,
// MaxUnitNodes, MaxUnitHistory and MaxRevertSteps; only a measurement
// changes them.
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
// version the purge took, then page.revert_too_large past the schedule's
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
	checked, err := r.checks().check(ctx, c, spec, revertBounds{Items: r.bounds.Nodes, Parse: true, History: r.bounds.History,
		Steps: r.bounds.Steps})
	if err != nil {
		return ChangesetReverted{}, err
	}
	plan := checked.plan
	contents, err := r.contents(ctx, plan)
	if err != nil {
		return ChangesetReverted{}, err
	}
	facts, release, err := r.parser.DecidedContents(ctx, contents, domain.ErrRevertTooLarge)
	if err != nil {
		return ChangesetReverted{}, err
	}
	defer release()
	read := RevertRead{Changeset: c.ID, Parsed: parsedContents(contents, facts), Retention: r.reads.Retention, MaxItems: r.bounds.Nodes,
		MaxSteps: r.bounds.Steps}
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
		slog.Int("aside", done.Aside), slog.String("client", string(client)))
	return ChangesetReverted{ChangesetID: outcome.ChangesetID, Changed: true}, nil
}

// checks are the use case's reads as its unlocked check reads them.
func (r *RevertChangeset) checks() revertChecks {
	return revertChecks{reads: r.reads, history: r.history, writer: r.writer}
}

// revertBounds are where a revert's unlocked check stops it (M8/P4 design
// 3.5): Items before the items are read (MaxUnitNodes, a job's
// MaxJobItems); Admit right after it, before anything else is read (a
// job's transfer.busy); Parse at step 6, the unit's pages, bytes and
// History rows of history (the sync's); Steps after the plan's conflicts
// (MaxRevertSteps, the sync's; 0 none); Locks after them, page.locked for
// a page the plan deletes or writes back that an edit session holds (a
// job's). The groups have no bound: P4A measured none needed (M8/P4 design
// 3.11). The 404 of a changeset gone between its reads is spec.NotFound,
// the one the caller's first read answers.
type revertBounds struct {
	Items   int
	Admit   func(context.Context) error
	Parse   bool
	History int
	Steps   int
	Locks   bool
}

// revertChecks are what a revert's unlocked check reads, a request's and a
// background job's (M8/P4 design 3.5); sessions the edit locks, read when
// its bounds have Locks.
type revertChecks struct {
	reads    TrashReads
	history  History
	writer   *Writer
	sessions AliveSessions
}

// revertChecked is what a revert's check read: the state it planned by,
// and its plan, which has no conflict and no version missing.
type revertChecked struct {
	state domain.RevertState
	plan  domain.RevertPlan
}

// check is a revert's check before its unit, unlocked, of the changeset c
// the caller found and is allowed by spec to revert (M8/P3 design 3.6,
// steps 1 to 7; M8/P4 design 3.5), stopping where b says:
// page.changeset_trimmed for one the purge marked; page.revert_too_large
// past b's items, before they are read; then b's Admit; then its items
// and versions, and its row again, page.changeset_trimmed when the purge
// marked or took it meanwhile, spec's 404 or 403 when its notebook went
// meanwhile, or the caller's role; then its nodes now; then the trash
// items it deleted, each read only when its root passes the check, and
// with b's Parse, page.revert_too_large past the pages and bytes it parses
// (a lone page never is) or b's rows of their history; then the state,
// planned: page.revert_conflict, then page.changeset_trimmed for a version
// the purge took, then page.revert_too_large past b's steps, then with b's
// Locks page.locked.
func (r revertChecks) check(ctx context.Context, c domain.Changeset, spec UnitSpec, b revertBounds) (revertChecked, error) {
	if c.Trimmed {
		return revertChecked{}, domain.ErrChangesetTrimmed
	}
	n, err := r.history.CountItems(ctx, c.ID, b.Items+1)
	switch {
	case err != nil:
		return revertChecked{}, err
	case n > b.Items:
		return revertChecked{}, domain.ErrRevertTooLarge
	}
	if b.Admit != nil {
		if err := b.Admit(ctx); err != nil {
			return revertChecked{}, err
		}
	}
	reads := revertReads{history: r.history, trash: r.reads.Trash, nodes: r.reads.Nodes}
	items, versions, err := reads.changeset(ctx, c.ID)
	if errors.Is(err, ErrNotFound) {
		// Gone since it was found: its notebook deleted, or the caller's
		// role ended, answers as the first read would (M8/P3A review B F2);
		// else the purge took it.
		if err := r.writer.Allowed(ctx, spec); err != nil {
			return revertChecked{}, err
		}
		err = domain.ErrChangesetTrimmed
	}
	if err != nil {
		return revertChecked{}, err
	}
	nodes, err := reads.nodesNow(ctx, c.ID, items, versions)
	if err != nil {
		return revertChecked{}, err
	}
	groups, err := reads.trashGroups(ctx, c.NotebookID, domain.RevertGroups(items, nodes))
	if err != nil {
		return revertChecked{}, err
	}
	if b.Parse {
		if err := r.withinBounds(ctx, items, versions, nodes, groups, b.History); err != nil {
			return revertChecked{}, err
		}
	}
	state, err := reads.rest(ctx, c.NotebookID, domain.RevertState{Items: items, Versions: versions, Nodes: nodes, Groups: groups,
		At: r.reads.Clock.Now(), Retention: r.reads.Retention})
	if err != nil {
		return revertChecked{}, err
	}
	plan, err := planned(state)
	switch {
	case err != nil:
		return revertChecked{}, err
	case b.Steps > 0 && plan.Steps() > b.Steps:
		return revertChecked{}, domain.ErrRevertTooLarge
	}
	if b.Locks {
		if err := r.locked(ctx, plan, state); err != nil {
			return revertChecked{}, err
		}
	}
	return revertChecked{state: state, plan: plan}, nil
}

// withinBounds checks a revert parses at most the unit's pages and bytes,
// those of the versions it writes back and of the pages of the trash
// items it restores, a content each, a page restored and written back
// two (a lone page's never past them, M9/P3 design 3.18.4), and restores
// at most history rows of history, its items' roots' at their time:
// page.revert_too_large.
func (r revertChecks) withinBounds(ctx context.Context, items []domain.ChangesetItem, versions []domain.ChangesetVersion,
	nodes map[uuid.UUID]domain.NodeNow, groups map[uuid.UUID][]domain.TrashMember, history int,
) error {
	pages, copies, bytes, rows := map[uuid.UUID]bool{}, 0, 0, 0
	for _, v := range domain.RevertWrites(items, versions) {
		pages[v.NodeID], copies, bytes = true, copies+1, bytes+v.BaseBytes
	}
	for _, g := range groups {
		for _, m := range g {
			if m.Node.Kind == domain.KindPage {
				pages[m.Node.ID], copies, bytes = true, copies+1, bytes+m.Bytes
			}
		}
	}
	if domain.UnitTooLarge(len(pages), copies, bytes, 0) {
		return domain.ErrRevertTooLarge
	}
	for root, g := range groups {
		ids := make([]uuid.UUID, len(g))
		for i, m := range g {
			ids[i] = m.Node.ID
		}
		n, err := r.reads.Trash.TrashHistory(ctx, ids, *nodes[root].DeletedAt, history+1)
		if err != nil {
			return err
		}
		if rows += n; rows > history {
			return domain.ErrRevertTooLarge
		}
	}
	return nil
}

// locked is page.locked for the first page, in the plan's order, that an
// edit session holds and the plan deletes or writes back (M8/P4 design
// 3.5, review B-M3), read once: a deletion's pages, those of its subtree
// at its moment in the schedule (M9/P3 design 3.22.1: a page the revert
// moves out of it first only moves, as an edit lock lets it), held by
// another account's session, the reverter's own ending with their pages,
// as the guard of a deletion has it; a page written back held by any.
func (r revertChecks) locked(ctx context.Context, plan domain.RevertPlan, state domain.RevertState) error {
	actor, err := requireActor(ctx)
	if err != nil {
		return err
	}
	pages := map[uuid.UUID]bool{}
	for _, n := range state.Tree {
		pages[n.ID] = n.Kind == domain.KindPage
	}
	var ids []uuid.UUID
	deleted := map[uuid.UUID]bool{}
	for _, op := range plan.Ops {
		switch op.Kind {
		case domain.RevertDelete:
			for _, id := range op.Subtree {
				if pages[id] {
					ids, deleted[id] = append(ids, id), true
				}
			}
		case domain.RevertContent:
			ids = append(ids, op.NodeID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	alive, err := r.sessions.AliveSessionsOf(ctx, ids, r.reads.Clock.Now())
	if err != nil {
		return err
	}
	held := map[uuid.UUID]EditSession{}
	for _, a := range alive {
		if _, ok := held[a.NodeID]; !ok && (!deleted[a.NodeID] || a.UserID != actor.UserID) {
			held[a.NodeID] = a
		}
	}
	for _, id := range ids {
		if a, ok := held[id]; ok {
			names, err := r.reads.Names.DisplayNames(ctx, []uuid.UUID{a.UserID})
			if err != nil {
				return err
			}
			return domain.Locked(a.NodeID, a.UserID, names[a.UserID])
		}
	}
	return nil
}

// contents are the contents the plan writes back or restores, by page and
// hash: a version's, a page's in the trash at its item's time, both of a
// page it restores and then writes back. One the purge took is
// page.changeset_trimmed, not parsed.
func (r *RevertChangeset) contents(ctx context.Context, plan domain.RevertPlan) (map[domain.ContentRef]string, error) {
	out := map[domain.ContentRef]string{}
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
				out[contentOf(page, content)] = content
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
		out[contentOf(ref.NodeID, v.Content)] = v.Content
	}
	return out, nil
}
