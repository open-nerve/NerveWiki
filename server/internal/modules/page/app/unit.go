package app

import (
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
	Tx           Tx
	Clock        Clock
	Auth         shared.Authorizer
	Workspaces   Workspaces
	Notebooks    Notebooks
	Nodes        Nodes
	NodeWriter   NodeWriter
	Changesets   ChangesetWriter
	Guards       []WriteGuard
	Participants []Participant
	Observers    []PageObserver
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
}

// Outcome is what a unit did: in which workspace, by whom, its changeset
// (zero when the unit changed nothing) and its time.
type Outcome struct {
	WorkspaceID uuid.UUID
	By          uuid.UUID
	ChangesetID uuid.UUID
	At          time.Time
}

// Run runs do in a unit of spec: outside a transaction, the notebook's
// workspace, unlocked; then, in the unit's transaction, the workspace's row
// FOR SHARE, the notebook's, the decision, do, the observers. The clock is
// read once: every row the unit writes has its time. Any error rolls the
// whole unit back and is returned.
func (w *Writer) Run(ctx context.Context, spec UnitSpec, do func(ctx context.Context, u *Unit) error) (Outcome, error) {
	if w.d.Tx.InTx(ctx) {
		return Outcome{}, errors.New("a page write unit opens its own transaction: ctx carries one already")
	}
	if !spec.Client.Valid() {
		return Outcome{}, fmt.Errorf("a page write unit of client %q: no such client", spec.Client)
	}
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Outcome{}, err
	}
	workspaceID, ok, err := w.d.Notebooks.WorkspaceOf(ctx, spec.NotebookID)
	switch {
	case err != nil:
		return Outcome{}, err
	case !ok:
		return Outcome{}, spec.NotFound
	}
	var u *Unit
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
			Options: spec.Options, At: w.d.Clock.Now(),
		}, merged: map[uuid.UUID]int{}}
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
	// changeset is the unit's, inserted with its first write.
	changeset uuid.UUID
	// changes are the unit's, merged by node in the order first changed;
	// merged indexes them.
	changes []domain.Change
	merged  map[uuid.UUID]int
}

// step is an operation of the unit.
func (u *Unit) step(op domain.Operation, changes ...domain.Change) Step {
	return Step{Write: u.write, Operation: op, Changes: changes}
}

// apply runs an operation whose checks passed: the guards, its write, its
// items in the changeset, and, unless a participant added it, the
// participants.
func (u *Unit) apply(ctx context.Context, s Step, participate bool, write func(ctx context.Context) error) error {
	// A ctx without the unit's transaction would write on the pool, outside
	// the unit and its locks.
	if !u.w.d.Tx.InTx(ctx) {
		return errors.New("a page write unit's operation runs on a context without the unit's transaction")
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
	for _, c := range s.Changes {
		if err := u.recordItem(ctx, c); err != nil {
			return err
		}
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

// ensureChangeset inserts the unit's changeset with its first write.
func (u *Unit) ensureChangeset(ctx context.Context) error {
	if u.changeset != (uuid.UUID{}) {
		return nil
	}
	id := uuid.NewV7()
	if err := u.w.d.Changesets.CreateChangeset(ctx, Changeset{
		ID: id, NotebookID: u.write.NotebookID, Kind: "edit", Client: u.write.Client, By: u.write.By, At: u.write.At,
	}); err != nil {
		return err
	}
	u.changeset = id
	return nil
}

// recordItem merges c into the unit's changes, and into the changeset's
// item of its node when it moves the node.
func (u *Unit) recordItem(ctx context.Context, c domain.Change) error {
	if i, ok := u.merged[c.NodeID]; ok {
		u.changes[i] = u.changes[i].Then(c)
	} else {
		u.merged[c.NodeID] = len(u.changes)
		u.changes = append(u.changes, c)
	}
	if !c.Moves() {
		return nil
	}
	return u.w.d.Changesets.RecordItem(ctx, Item{ID: uuid.NewV7(), ChangesetID: u.changeset, Change: c, At: u.write.At})
}

// content is the page nodeID's content text at revision, as the unit
// writes it: its hash and size computed, by the unit's actor at its time.
func (u *Unit) content(nodeID uuid.UUID, text string, revision int) Content {
	sum := sha256.Sum256([]byte(text))
	return Content{NodeID: nodeID, Content: text, Revision: revision, Hash: sum[:], ByteSize: len(text), By: u.write.By, At: u.write.At}
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
