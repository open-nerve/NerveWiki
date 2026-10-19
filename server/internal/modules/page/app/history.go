package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The log's reads (M8 design 4.7; M8/P3 design 3.4): a notebook's
// changesets, one of them, and its changes, which share how a changeset is
// found and what each of them tells.

// HistoryReads are what the log's reads need; by path and in a notebook,
// MCP's log and changeset (M9/P2 design 3.16), Paths finding a node by its
// path and writing the paths of a page of changes' nodes.
type HistoryReads struct {
	Notebooks Notebooks
	History   History
	Names     Names
	Auth      shared.Authorizer
	Paths     PathStore
}

// LoggedChangeset is a changeset as the log lists it: with its counts and
// reverts, and its first changes.
type LoggedChangeset struct {
	Changeset domain.Changeset
	Facts     domain.ChangesetFacts
	Changes   []domain.ChangeRow
}

// loggedChanges is how many of each changeset's changes the log lists with
// it, by node id: a summary.
const loggedChanges = 3

// cursorsAfter is the cursor that goes on after each of items, payload's
// of it: MCP's history answers as many items as fit and go on from the
// last of them (M9/P2 design 3.16), a list's next cursor that of the last
// item of its page.
func cursorsAfter[T any](items []T, payload func(T) any) ([]string, error) {
	out := make([]string, len(items))
	for i, it := range items {
		c, err := shared.EncodeCursor(payload(it))
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

// readableChangeset is the changeset id, if the caller may read it (any
// role in its notebook, page.history), unlocked: one that does not exist,
// whose notebook is deleted, or whose notebook the caller has no role in is
// page.changeset_not_found.
func (r HistoryReads) readableChangeset(ctx context.Context, id uuid.UUID) (domain.Changeset, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return domain.Changeset{}, err
	}
	c, err := r.History.FindChangeset(ctx, id)
	if err != nil {
		return domain.Changeset{}, found(err, domain.ErrChangesetNotFound)
	}
	workspaceID, ok, err := r.Notebooks.WorkspaceOf(ctx, c.NotebookID)
	switch {
	case err != nil:
		return domain.Changeset{}, err
	case !ok:
		return domain.Changeset{}, domain.ErrChangesetNotFound
	}
	target := shared.Target{WorkspaceID: workspaceID, NotebookID: c.NotebookID}
	if _, err := authorize(ctx, r.Auth, actor, domain.ActionHistory, target, domain.ErrChangesetNotFound); err != nil {
		return domain.Changeset{}, err
	}
	return c, nil
}

// detail is c as GetChangeset reads it: as the log lists it, and whether
// its first items rename or move a node, which the log's lines do not
// read.
func (r HistoryReads) detail(ctx context.Context, c domain.Changeset) (ChangesetDetail, error) {
	logged, err := r.logged(ctx, []domain.Changeset{c})
	if err != nil {
		return ChangesetDetail{}, err
	}
	moves, err := r.History.ChangesetMoves(ctx, c.ID)
	if err != nil {
		return ChangesetDetail{}, err
	}
	return ChangesetDetail{LoggedChangeset: logged[0], Moves: moves}, nil
}

// changes is a page of the changeset id's changes by node id, after the
// node after, size at most: one change more is read to tell whether
// another follows.
func (r HistoryReads) changes(ctx context.Context, id, after uuid.UUID, size int) (ChangeList, error) {
	changes, err := r.History.ListChanges(ctx, id, after, size+1)
	if err != nil {
		return ChangeList{}, err
	}
	var out ChangeList
	if len(changes) > size {
		changes = changes[:size]
		if out.NextCursor, err = shared.EncodeCursor(domain.ChangeCursor{After: changes[size-1].NodeID}); err != nil {
			return ChangeList{}, err
		}
	}
	out.Changes = changes
	return out, nil
}

// alivePaths are the paths of the nodes of changes alive now in
// notebookID, by id, read once for them all (NodePaths.Paths): a node in
// the trash or gone has none.
func (r HistoryReads) alivePaths(ctx context.Context, notebookID uuid.UUID, changes []domain.ChangeRow) (map[uuid.UUID]string, error) {
	var ids []uuid.UUID
	for _, c := range changes {
		if c.Alive {
			ids = append(ids, c.NodeID)
		}
	}
	if len(ids) == 0 {
		return map[uuid.UUID]string{}, nil
	}
	return NewNodePaths(r.Paths).Paths(ctx, notebookID, ids)
}

// logged are the changesets as the log lists them: their counts and
// reverts, their first changes, and the display names of their actors and
// of the reverted ones' actors, each read once.
func (r HistoryReads) logged(ctx context.Context, changesets []domain.Changeset) ([]LoggedChangeset, error) {
	if len(changesets) == 0 {
		return nil, nil
	}
	ids, actors := make([]uuid.UUID, len(changesets)), []uuid.UUID{}
	seen := map[uuid.UUID]bool{}
	actor := func(id uuid.UUID) {
		if !seen[id] {
			seen[id] = true
			actors = append(actors, id)
		}
	}
	for i, c := range changesets {
		ids[i] = c.ID
		actor(c.By)
		if c.Reverts != nil {
			actor(c.Reverts.By)
		}
	}
	facts, err := r.History.ChangesetFacts(ctx, ids)
	if err != nil {
		return nil, err
	}
	changes, err := r.History.FirstChanges(ctx, ids, loggedChanges)
	if err != nil {
		return nil, err
	}
	names, err := r.Names.DisplayNames(ctx, actors)
	if err != nil {
		return nil, err
	}
	out := make([]LoggedChangeset, len(changesets))
	for i, c := range changesets {
		c.ByName = names[c.By]
		if c.Reverts != nil {
			brief := *c.Reverts
			brief.ByName = names[brief.By]
			c.Reverts = &brief
		}
		out[i] = LoggedChangeset{Changeset: c, Facts: facts[c.ID], Changes: changes[c.ID]}
	}
	return out, nil
}
