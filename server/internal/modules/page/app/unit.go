package app

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The write unit (M4 design 4; M4/P1 design 3.6): every page write runs in
// one. Its use case says what it locks and decides on; the unit opens the
// transaction, takes the locks, decides, runs the operations, each through
// the guards, its write, the changeset and the participants, then tells the
// observers once.

// WriterDeps are what the write unit needs.
type WriterDeps struct {
	Tx         Tx
	Clock      Clock
	Auth       shared.Authorizer
	Workspaces Workspaces
	Notebooks  Notebooks
	Nodes      Nodes
	NodeWriter NodeWriter
	// Trash reads a trash item again under the notebook's lock: its
	// restore's, and a revert's; History a changeset and its nodes: a
	// revert's.
	Trash         Trash
	History       History
	Changesets    ChangesetWriter
	SessionWriter SessionWriter
	// Search queues the nodes a unit changed for the search's job (M8/P5
	// design 3.4).
	Search SearchQueue
	// Names gives the name of who released a session's lock (M5 design
	// 4.3).
	Names        Names
	Guards       []WriteGuard
	Participants []Participant
	Observers    []PageObserver
	// The edit sessions' vetoers of an opening, and subscribers of their
	// openings and ends.
	SessionVetoers     []EditSessionVetoer
	SessionSubscribers []EditSessionSubscriber
}

// Writer runs write units.
type Writer struct {
	d WriterDeps
}

// NewWriter returns the writer.
func NewWriter(d WriterDeps) *Writer {
	return &Writer{d: d}
}

// UnitSpec is what a use case tells the unit it runs.
type UnitSpec struct {
	NotebookID uuid.UUID
	// Action is what the unit decides on, at the notebook's level.
	Action shared.Action
	// Tree is a unit that changes the tree: it locks the notebook's row FOR
	// NO KEY UPDATE, so the tree's writes of a notebook run one at a time;
	// the others lock it FOR SHARE.
	Tree    bool
	Client  domain.Client
	Options Options
	// NotFound is the operation's 404 for a notebook the caller cannot
	// see: notebook.not_found or page.not_found, by what its address
	// names.
	NotFound error
	// Kind is the kind of the unit's changeset: an edit unless set.
	Kind domain.ChangesetKind
	// RevertsID is the changeset a revert's unit reverts: set exactly
	// when Kind is a revert.
	RevertsID uuid.UUID
	// Changeset, when set, is the changeset the unit writes in rather
	// than one of its own (M7/P6 design 3.2): an import's first unit's,
	// which its later units merge into. It must be of the notebook, of
	// Kind and RevertsID, by the unit's actor and from its client, and not
	// reverted (M8/P3 design 3.7).
	Changeset uuid.UUID
	// Create has the unit make Changeset with its first write when there
	// is none yet (M8/P4 design 3.3): a job's, whose row names it, which
	// its first unit that writes makes and the later ones merge into. One
	// that writes nothing makes none.
	Create bool
	// Message is the message of the changeset the unit creates, none when
	// nil; one it merges into keeps its own.
	Message *string
	// Background is a background job's unit: an import's, a background
	// restore's or revert's (M8/P5 design 3.4). It queues the nodes it
	// changes for the search in bulk, which no search waits for, the
	// answer saying the index is building meanwhile: a job brings up to
	// tens of thousands of nodes, which the search would otherwise walk
	// before any other. A request's unit queues them exact, found at once.
	Background bool
	// BulkSearch has the unit queue the nodes it changes for the search in
	// bulk, as Background does, and is nothing else of Background's: a
	// batch's unit (M9/P3 design 3.10.1). Its backlog, up to some 1,600
	// nodes, queued exact would leave more of the search's first pages
	// empty with a cursor than the rule allows (its step 0, the queued
	// pages the newest: 16 more of 90 at 1,600 nodes, past 10). Its
	// Outcome says Building when it queued any.
	BulkSearch bool
	// DeferParticipants is a batch's unit (M9/P3 design 3.7.1): its
	// operations call no participant; its use case has them follow the
	// unit's renames and moves once, after all of them (Unit.Participate).
	DeferParticipants bool
}

// ErrChangesetReverted is a unit that would merge into a changeset a
// revert was made of (M8/P3 design 3.7): what that changeset changed is
// settled by the revert, so nothing more joins it. It is no problem of the
// API: only a job merges, and it starts a changeset of its own instead.
var ErrChangesetReverted = errors.New("page: the changeset the unit merges into was reverted")

// Outcome is what a unit did: in which workspace, by whom, its changeset
// (zero when the unit changed nothing) and its time; and whether it queued
// nodes for the search in bulk (Building), which a search finds once its
// job has built them, answering building meanwhile.
type Outcome struct {
	WorkspaceID uuid.UUID
	By          uuid.UUID
	ChangesetID uuid.UUID
	At          time.Time
	Building    bool
}

// Allowed decides spec as Run does, unlocked: the same 404 and 403, for a
// write to answer before it does work that only a writer should cause, as
// parsing its content (M4/P4 review P1). Run decides again under its
// locks.
func (w *Writer) Allowed(ctx context.Context, spec UnitSpec) error {
	actor, err := requireActor(ctx)
	if err != nil {
		return err
	}
	workspaceID, err := w.workspaceOf(ctx, spec)
	if err != nil {
		return err
	}
	_, err = authorize(ctx, w.d.Auth, actor, spec.Action, shared.Target{WorkspaceID: workspaceID, NotebookID: spec.NotebookID}, spec.NotFound)
	return err
}

// requireActor is ctx's actor, which acts with exactly one credential:
// one with none, or more, is the caller's fault (M7/P6 design 3.2).
func requireActor(ctx context.Context) (shared.Actor, error) {
	actor, err := shared.RequireActor(ctx)
	if err == nil && !actor.Valid() {
		err = errors.New("a page write's actor acts with no credential, or with more than one")
	}
	return actor, err
}

// workspaceOf is the workspace of spec's notebook, read unlocked; spec's
// 404 when the notebook is not there.
func (w *Writer) workspaceOf(ctx context.Context, spec UnitSpec) (uuid.UUID, error) {
	workspaceID, ok, err := w.d.Notebooks.WorkspaceOf(ctx, spec.NotebookID)
	switch {
	case err != nil:
		return uuid.UUID{}, err
	case !ok:
		return uuid.UUID{}, spec.NotFound
	}
	return workspaceID, nil
}

// Run runs do in a unit of spec: outside a transaction, the notebook's
// workspace, unlocked; then, in the unit's transaction, the workspace's row
// FOR SHARE, the notebook's, the decision, the changeset it merges into,
// do, the changeset's renames and moves, the search's queue, the
// observers. The clock is read once: every row
// the unit writes has its time. Any error rolls the whole unit back and is
// returned. It refuses a ctx that carries a transaction.
func (w *Writer) Run(ctx context.Context, spec UnitSpec, do func(ctx context.Context, u *Unit) error) (Outcome, error) {
	if w.d.Tx.InTx(ctx) {
		return Outcome{}, errors.New("a page write unit opens its own transaction: ctx carries one already")
	}
	return w.run(ctx, spec, time.Time{}, do)
}

// Seed runs do in a unit of spec within the transaction ctx carries, its
// caller's, at at: the one exception of v0.1 design 13.1 item 1, a
// notebook's template, written in its creation's transaction (M10 design
// 4.4). It refuses a ctx that carries none, and a zero at; the unit reads
// no clock, so that the notebook, its changeset and its pages have one
// time. The rest is Run's: the workspace's row FOR SHARE and the
// notebook's, both re-taken in the caller's transaction, the decision, the
// unit, the search's queue, the observers. Its caller's transaction
// commits or rolls back the unit; the functions its participants deferred
// run when Seed returns, its observers done.
func (w *Writer) Seed(ctx context.Context, spec UnitSpec, at time.Time, do func(ctx context.Context, u *Unit) error) (Outcome, error) {
	switch {
	case !w.d.Tx.InTx(ctx):
		return Outcome{}, errors.New("a seed's unit joins its caller's transaction: ctx carries none")
	case at.IsZero():
		return Outcome{}, errors.New("a seed's unit is at its caller's time: none given")
	}
	return w.run(ctx, spec, at, do)
}

// run is Run's body, the unit's time at, or the clock's one read when at
// is zero.
func (w *Writer) run(ctx context.Context, spec UnitSpec, at time.Time, do func(ctx context.Context, u *Unit) error) (Outcome, error) {
	if !spec.Client.Valid() {
		return Outcome{}, fmt.Errorf("a page write unit of client %q: no such client", spec.Client)
	}
	kind := cmp.Or(spec.Kind, domain.ChangesetEdit)
	if (kind == domain.ChangesetRevert) != (spec.RevertsID != uuid.UUID{}) {
		return Outcome{}, fmt.Errorf("a page write unit of kind %q reverting %s: a revert names what it reverts, nothing else does",
			kind, spec.RevertsID)
	}
	actor, err := requireActor(ctx)
	if err != nil {
		return Outcome{}, err
	}
	workspaceID, err := w.workspaceOf(ctx, spec)
	if err != nil {
		return Outcome{}, err
	}
	var u *Unit
	defer func() {
		if u != nil {
			for _, f := range u.deferred {
				f()
			}
		}
	}()
	err = w.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if ok, err := w.d.Workspaces.ShareByID(ctx, workspaceID); err != nil || !ok {
			return orNotFound(err, spec.NotFound)
		}
		lock := w.d.Notebooks.ShareByID
		if spec.Tree {
			lock = w.d.Notebooks.LockByID
		}
		if ok, err := lock(ctx, spec.NotebookID); err != nil || !ok {
			return orNotFound(err, spec.NotFound)
		}
		target := shared.Target{WorkspaceID: workspaceID, NotebookID: spec.NotebookID}
		if _, err := authorize(ctx, w.d.Auth, actor, spec.Action, target, spec.NotFound); err != nil {
			return err
		}
		if at.IsZero() {
			at = w.d.Clock.Now()
		}
		u = &Unit{w: w, write: Write{
			WorkspaceID: workspaceID, NotebookID: spec.NotebookID, By: actor.UserID, Client: spec.Client,
			Options: spec.Options, At: at, Kind: kind, RevertsID: spec.RevertsID,
		}, into: spec.Changeset, create: spec.Create, message: spec.Message, bulk: spec.Background || spec.BulkSearch,
			merged: map[uuid.UUID]int{}, deferring: spec.DeferParticipants}
		if err := u.lockInto(ctx); err != nil {
			return err
		}
		if err := do(ctx, u); err != nil {
			return err
		}
		if err := u.markMoves(ctx); err != nil {
			return err
		}
		if err := u.queueSearch(ctx); err != nil {
			return err
		}
		return u.tell(ctx)
	})
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{WorkspaceID: workspaceID, By: actor.UserID, ChangesetID: u.changeset, At: u.write.At, Building: u.building}, nil
}

// Unit is a write unit's operations: a use case calls them in Run's do.
type Unit struct {
	w     *Writer
	write Write
	// changeset is the unit's, inserted with its first write, of the
	// write's kind, with message; or into, an earlier unit's it merges into
	// then, or the one it inserts with that id when it is to create it and
	// none is there (fresh).
	changeset uuid.UUID
	into      uuid.UUID
	create    bool
	fresh     bool
	message   *string
	// bulk queues the unit's nodes for the search in bulk: a background
	// job's unit, or a batch's; building is that it queued any.
	bulk     bool
	building bool
	// changes are the unit's, merged by node in the order first changed;
	// merged indexes them.
	changes []domain.Change
	merged  map[uuid.UUID]int
	// deferred are what participants have Run call once the unit is over.
	deferred []func()
	// deferring is a batch's unit, whose participants follow its renames
	// and moves once, at its end: followed are those of its operations, by
	// node in the order first changed, followedAt indexes them; frozen
	// the nodes a move kept the links of (keepLinks, Placement.KeepLinks),
	// none of whose changes are followed; carried the nodes under each
	// moved one, with the nodes that carried them, each once (M9/P3 review
	// A-I2: a folder moved a hundred times is no hundred carriers).
	deferring  bool
	followed   []domain.Change
	followedAt map[uuid.UUID]int
	frozen     map[uuid.UUID]bool
	carried    map[uuid.UUID]map[uuid.UUID]bool
	keepLinks  bool
}

// step is an operation of the unit.
func (u *Unit) step(op domain.Operation, changes ...domain.Change) Step {
	return Step{Write: u.write, Operation: op, Changes: changes}
}

// apply runs an operation whose checks passed: the guards, its write, its
// items in the changeset, and, unless a participant added it, the
// participants.
func (u *Unit) apply(ctx context.Context, s Step, participate bool, write func(ctx context.Context) error) error {
	if err := u.inUnit(ctx); err != nil {
		return err
	}
	for _, g := range u.w.d.Guards {
		if err := g.GuardWrite(ctx, s); err != nil {
			return err
		}
	}
	if err := u.ensureChangeset(ctx); err != nil {
		return err
	}
	if err := write(ctx); err != nil {
		return err
	}
	if err := u.recordItems(ctx, s.Changes); err != nil {
		return err
	}
	if !participate {
		return nil
	}
	if u.deferring {
		u.follow(s)
		return nil
	}
	for _, p := range u.w.d.Participants {
		if err := p.Participate(ctx, s, appender{u}); err != nil {
			return err
		}
	}
	return nil
}

// inUnit refuses a ctx without the unit's transaction: an operation would
// write on the pool, outside the unit and its locks.
func (u *Unit) inUnit(ctx context.Context) error {
	if !u.w.d.Tx.InTx(ctx) {
		return errors.New("a page write unit's operation runs on a context without the unit's transaction")
	}
	return nil
}

// lockInto locks the changeset the unit merges into, when it names one,
// before the unit does anything (M8/P3 design 3.7): not reverted, of the
// notebook, of the unit's kind and reverting what it reverts, by its actor
// and from its client. A revert of it, made before, is
// ErrChangesetReverted; one made after waits for the unit. It locks after
// the notebook's row, as the units do (v0.1 design 13.1, item 5). One that
// is not there yet the unit makes with its first write when it is to
// create it (M8/P4 design 3.3): its job's units run one at a time, under
// the notebook's row, so no other makes it meanwhile.
func (u *Unit) lockInto(ctx context.Context) error {
	if u.into == (uuid.UUID{}) {
		return nil
	}
	c, err := u.w.d.Changesets.LockChangeset(ctx, u.into)
	switch {
	case errors.Is(err, ErrNotFound) && u.create:
		u.fresh = true
		return nil
	case errors.Is(err, ErrNotFound):
		return fmt.Errorf("a page write unit merges into changeset %s: no such changeset", u.into)
	case err != nil:
		return err
	case c.Reverted:
		return ErrChangesetReverted
	case c.NotebookID != u.write.NotebookID || c.Kind != u.write.Kind || c.RevertsID != u.write.RevertsID ||
		c.By != u.write.By || c.Client != u.write.Client:
		return fmt.Errorf("a page write unit merges into changeset %s: another notebook's, kind's, revert's, actor's or client's",
			u.into)
	}
	return nil
}

// ensureChangeset inserts the unit's changeset with its first write, or
// merges the unit into the one it names, which lockInto locked: its
// updated_at moved to the unit's time, which the notebook's activity and
// log read. One it is to create, not there yet, it inserts with the id it
// names.
func (u *Unit) ensureChangeset(ctx context.Context) error {
	if u.changeset != (uuid.UUID{}) {
		return nil
	}
	if u.into != (uuid.UUID{}) && !u.fresh {
		if err := u.w.d.Changesets.TouchChangeset(ctx, u.into, u.write.At); err != nil {
			return err
		}
		u.changeset = u.into
		return nil
	}
	id := u.into
	if !u.fresh {
		id = uuid.NewV7()
	}
	if err := u.w.d.Changesets.CreateChangeset(ctx, Changeset{
		ID: id, NotebookID: u.write.NotebookID, Kind: u.write.Kind, RevertsID: u.write.RevertsID, Client: u.write.Client,
		Message: u.message, By: u.write.By, At: u.write.At,
	}); err != nil {
		return err
	}
	u.changeset = id
	return nil
}

// recordItems merges a step's changes into the unit's, and those that move
// their node into the changeset's items of their nodes, in one write: none
// when none does (M8/P2 design 3.3). A step changes each node once; one
// that changes a node twice is its caller's defect. A batch's unit merges
// more (M9/P3 design 3.5.1): a node it creates and then renames, moves or
// writes the content of is the creation's item; one whose content it
// writes and then deletes is the deletion's, with the revision deleted.
// None creates a node and deletes it, nor deletes one it renamed or moved:
// a batch's plan refuses both (B1, B3), and one that came through would
// be the item the table refuses, of neither state, the defect it is
// (M4/P2 design 3.2). A node moved and moved back is an item of the same
// before and after, which the unit drops at its end (DropUnchanged).
func (u *Unit) recordItems(ctx context.Context, changes []domain.Change) error {
	items := make([]Item, 0, len(changes))
	seen := make(map[uuid.UUID]bool, len(changes))
	for _, c := range changes {
		if seen[c.NodeID] {
			return fmt.Errorf("a page write unit's step changes node %s twice", c.NodeID)
		}
		seen[c.NodeID] = true
		if i, ok := u.merged[c.NodeID]; ok {
			u.changes[i] = u.changes[i].Then(c)
		} else {
			u.merged[c.NodeID] = len(u.changes)
			u.changes = append(u.changes, c)
		}
		if c.Moves() {
			items = append(items, Item{ID: uuid.NewV7(), ChangesetID: u.changeset, Change: c, At: u.write.At})
		}
	}
	if len(items) == 0 {
		return nil
	}
	return u.w.d.Changesets.RecordItems(ctx, items)
}

// follow records s, an operation of a unit whose participants follow it
// at its end (deferring): a rename or a move, merged into the changes they
// follow, unless it keeps links (Placement.KeepLinks), its node then
// followed in none; and the nodes a move carries, with the node that
// carried them.
func (u *Unit) follow(s Step) {
	if s.Operation != domain.OpMove && s.Operation != domain.OpRename {
		return
	}
	if u.followedAt == nil {
		u.followedAt, u.frozen, u.carried = map[uuid.UUID]int{}, map[uuid.UUID]bool{}, map[uuid.UUID]map[uuid.UUID]bool{}
	}
	for _, c := range s.Changes[1:] {
		if u.carried[c.NodeID] == nil {
			u.carried[c.NodeID] = map[uuid.UUID]bool{}
		}
		u.carried[c.NodeID][s.Changes[0].NodeID] = true
	}
	if u.keepLinks {
		u.frozen[s.Changes[0].NodeID] = true
		return
	}
	for _, c := range s.Changes {
		if i, ok := u.followedAt[c.NodeID]; ok {
			u.followed[i] = u.followed[i].Then(c)
		} else {
			u.followedAt[c.NodeID] = len(u.followed)
			u.followed = append(u.followed, c)
		}
	}
}

// Participate has the participants follow the unit's changes once, as one
// operation: those of the nodes alive before and after it that a rename
// or a move changed with every op's update_links true, merged, with no
// revision (M9 design 4.11), and the nodes under the moved ones as they
// are; b. Its appends go through the guards as any; it answers how many
// pages they wrote, which the batch's log line counts.
func (u *Unit) Participate(ctx context.Context, b BatchRewrite) (int, error) {
	if !u.deferring {
		return 0, errors.New("a page write unit's participants follow it at its end: its spec defers them")
	}
	var changes []domain.Change
	for _, c := range u.followed {
		i, ok := u.merged[c.NodeID]
		if u.frozen[c.NodeID] || !ok || u.changes[i].After == nil {
			continue // a node a move kept the links of, or one deleted since
		}
		changes = append(changes, domain.Change{NodeID: c.NodeID, Before: c.Before, After: c.After})
	}
	b.Frozen = slices.SortedFunc(maps.Keys(u.frozen), uuid.UUID.Compare)
	s := u.step(domain.OpMove, changes...)
	s.Batch = &b
	counted := &countedAppender{appender: appender{u}}
	for _, p := range u.w.d.Participants {
		if err := p.Participate(ctx, s, counted); err != nil {
			return 0, err
		}
	}
	return counted.written, nil
}

// countedAppender is the unit's appender, counting the contents it writes.
type countedAppender struct {
	appender
	written int
}

func (a *countedAppender) WriteContent(ctx context.Context, w ContentWrite) (int, error) {
	revision, err := a.appender.WriteContent(ctx, w)
	if err == nil {
		a.written++
	}
	return revision, err
}

// DropUnchanged drops, at a batch's end, what the unit changed and then
// changed back (M9/P3 design 3.5.4): the changeset's items whose node is
// where it was, its parent, name and order; the changeset itself when no
// item and no version is left, the unit then of none; and of its changes,
// those of a node where it was, but for a node whose content it wrote,
// which the observers index again, and one a move carried, while that
// move is one: they follow the node under its new parent.
func (u *Unit) DropUnchanged(ctx context.Context) error {
	if u.changeset != (uuid.UUID{}) {
		dropped, err := u.w.d.Changesets.DropUnchanged(ctx, u.changeset)
		if err != nil {
			return err
		}
		if dropped {
			u.changeset = uuid.UUID{}
		}
	}
	moved := func(ids map[uuid.UUID]bool) bool {
		for id := range ids {
			if i, ok := u.merged[id]; ok && u.changes[i].Moves() {
				return true
			}
		}
		return false
	}
	var kept []domain.Change
	for _, c := range u.changes {
		if c.Before != nil && c.After != nil && c.Before.Same(*c.After) && c.Revision == 0 && !moved(u.carried[c.NodeID]) {
			continue
		}
		kept = append(kept, c)
	}
	u.changes, u.merged = kept, make(map[uuid.UUID]int, len(kept))
	for i, c := range kept {
		u.merged[c.NodeID] = i
	}
	return nil
}

// content is the page nodeID's content text at revision, as the unit
// writes it: its hash and size computed, by the unit's actor at its time.
func (u *Unit) content(nodeID uuid.UUID, text string, revision int) Content {
	sum := sha256.Sum256([]byte(text))
	return Content{NodeID: nodeID, Content: text, Revision: revision, Hash: sum[:], ByteSize: len(text), By: u.write.By, At: u.write.At}
}

// changesetOf has the unit write in the changeset id, an edit session's,
// rather than one of its own: an edit session's writes come first in
// their unit, which is an edit's.
func (u *Unit) changesetOf(id uuid.UUID) error {
	if u.into != (uuid.UUID{}) {
		return errors.New("a page write unit that merges into a changeset writes in an edit session's")
	}
	if u.write.Kind != domain.ChangesetEdit {
		return fmt.Errorf("a page write unit of kind %q writes in an edit session's changeset", u.write.Kind)
	}
	if u.changeset != (uuid.UUID{}) && u.changeset != id {
		return errors.New("a page write unit writes in an edit session's changeset after it wrote in its own")
	}
	u.changeset = id
	return nil
}

// recordRevision records c, a content the unit wrote on base (nil: the
// unit created the page), as its changeset's version of the page.
func (u *Unit) recordRevision(ctx context.Context, c Content, base *int) error {
	return u.w.d.Changesets.RecordRevision(ctx, Revision{
		ID: uuid.NewV7(), ChangesetID: u.changeset, NodeID: c.NodeID, Base: base, Revision: c.Revision, Content: c.Content,
		Hash: c.Hash, ByteSize: c.ByteSize, At: u.write.At,
	})
}

// markMoves marks the unit's changeset as renaming, or moving, a node,
// from its merged changes, those recordItems merges (M9/P3 design 3.20.2):
// a node alive before and after it under another name renames, under
// another parent moves; marks kept once set, as each unit of a job's
// changeset adds its own. A unit of neither, or of no changeset, marks
// none.
func (u *Unit) markMoves(ctx context.Context) error {
	if u.changeset == (uuid.UUID{}) {
		return nil
	}
	var m domain.ChangesetMoves
	for _, c := range u.changes {
		if c.Before != nil && c.After != nil {
			m.Renames = m.Renames || c.Before.Name != c.After.Name
			m.Moves = m.Moves || !domain.SameParent(c.Before.ParentID, c.After.ParentID)
		}
	}
	if !m.Renames && !m.Moves {
		return nil
	}
	return u.w.d.Changesets.MarkChangesetMoves(ctx, u.changeset, m)
}

// queueSearch queues for the search's job the nodes the unit created,
// restored, deleted, renamed or wrote the content of (M8/P5 design 3.4), in
// ascending id order, after all its writes and before the observers (v0.1
// design 13.1, item 5): in bulk for a background job's unit, an import's, a
// background restore's or revert's, and a batch's (M9/P3 design 3.10.1),
// which a search does not wait for, exact for the others. A move or a
// reorder leaves a node's lexemes as they were: its path is no part of a
// match.
func (u *Unit) queueSearch(ctx context.Context) error {
	var ids []uuid.UUID
	for _, c := range u.changes {
		if c.Before == nil || c.After == nil || c.Before.Name != c.After.Name || c.Revision != 0 {
			ids = append(ids, c.NodeID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	u.building = u.bulk
	return u.w.d.Search.QueueSearch(ctx, u.write.NotebookID, ids, u.bulk)
}

// tell has the observers follow the unit's changes, once: none when it
// changed nothing.
func (u *Unit) tell(ctx context.Context) error {
	if len(u.changes) == 0 {
		return nil
	}
	e := Event{Write: u.write, ChangesetID: u.changeset, Changes: u.changes}
	for _, o := range u.w.d.Observers {
		if err := o.PagesChanged(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// appender is the unit as a participant adds to it: its operations call
// no participant.
type appender struct {
	u *Unit
}

func (a appender) Rename(ctx context.Context, nodeID uuid.UUID, name string) (domain.Node, error) {
	return a.u.rename(ctx, nodeID, name, false)
}

func (a appender) WriteContent(ctx context.Context, w ContentWrite) (int, error) {
	w.EditSession = uuid.UUID{}
	return a.u.writeContent(ctx, w, false)
}

func (a appender) Defer(f func()) {
	a.u.deferred = append(a.u.deferred, f)
}
