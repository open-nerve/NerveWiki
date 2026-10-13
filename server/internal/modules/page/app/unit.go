package app

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
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
	// restore's.
	Trash         Trash
	Changesets    ChangesetWriter
	SessionWriter SessionWriter
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
	// Message is the message of the changeset the unit creates, none when
	// nil; one it merges into keeps its own.
	Message *string
}

// ErrChangesetReverted is a unit that would merge into a changeset a
// revert was made of (M8/P3 design 3.7): what that changeset changed is
// settled by the revert, so nothing more joins it. It is no problem of the
// API: only a job merges, and it starts a changeset of its own instead.
var ErrChangesetReverted = errors.New("page: the changeset the unit merges into was reverted")

// Outcome is what a unit did: in which workspace, by whom, its changeset
// (zero when the unit changed nothing) and its time.
type Outcome struct {
	WorkspaceID uuid.UUID
	By          uuid.UUID
	ChangesetID uuid.UUID
	At          time.Time
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
// do, the observers. The clock is read once: every row the unit writes has
// its time. Any error rolls the whole unit back and is returned.
func (w *Writer) Run(ctx context.Context, spec UnitSpec, do func(ctx context.Context, u *Unit) error) (Outcome, error) {
	if w.d.Tx.InTx(ctx) {
		return Outcome{}, errors.New("a page write unit opens its own transaction: ctx carries one already")
	}
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
		u = &Unit{w: w, write: Write{
			WorkspaceID: workspaceID, NotebookID: spec.NotebookID, By: actor.UserID, Client: spec.Client,
			Options: spec.Options, At: w.d.Clock.Now(), Kind: kind, RevertsID: spec.RevertsID,
		}, into: spec.Changeset, message: spec.Message, merged: map[uuid.UUID]int{}}
		if err := u.lockInto(ctx); err != nil {
			return err
		}
		if err := do(ctx, u); err != nil {
			return err
		}
		return u.tell(ctx)
	})
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{WorkspaceID: workspaceID, By: actor.UserID, ChangesetID: u.changeset, At: u.write.At}, nil
}

// Unit is a write unit's operations: a use case calls them in Run's do.
type Unit struct {
	w     *Writer
	write Write
	// changeset is the unit's, inserted with its first write, of the
	// write's kind, with message; or into, an earlier unit's it merges into
	// then.
	changeset uuid.UUID
	into      uuid.UUID
	message   *string
	// changes are the unit's, merged by node in the order first changed;
	// merged indexes them.
	changes []domain.Change
	merged  map[uuid.UUID]int
	// deferred are what participants have Run call once the unit is over.
	deferred []func()
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
// the notebook's row, as the units do (v0.1 design 13.1, item 5).
func (u *Unit) lockInto(ctx context.Context) error {
	if u.into == (uuid.UUID{}) {
		return nil
	}
	c, err := u.w.d.Changesets.LockChangeset(ctx, u.into)
	switch {
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
// log read.
func (u *Unit) ensureChangeset(ctx context.Context) error {
	if u.changeset != (uuid.UUID{}) {
		return nil
	}
	if u.into != (uuid.UUID{}) {
		if err := u.w.d.Changesets.TouchChangeset(ctx, u.into, u.write.At); err != nil {
			return err
		}
		u.changeset = u.into
		return nil
	}
	id := uuid.NewV7()
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
// that changes a node twice is its caller's defect. No unit of M4 creates
// a node and deletes it: their item would have neither state, which the
// table refuses (M4/P2 design 3.2).
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
