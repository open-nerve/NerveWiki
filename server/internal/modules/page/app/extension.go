package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The module's extension points (M4 design 8; M4/P1 design 3.6), and its
// registrant of the notebook module's deletion. M4 has no registrant of its
// own points: bootstrap hands empty sets, and the module root's test proves
// they reach every write.

// Options are a write's options. UpdateLinks has M6 rewrite the links to a
// renamed or moved page; every write of M4 sets it.
type Options struct {
	UpdateLinks bool
}

// Write is a unit's write as an extension sees it: the notebook, who
// wrote, from where, with which options, at the unit's one time.
type Write struct {
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	By          uuid.UUID
	Client      domain.Client
	Options     Options
	At          time.Time
}

// Step is one operation of a unit: its kind, and each node it changes,
// where the node was and where it is after. A guard gets it before the
// write, a participant after.
type Step struct {
	Write
	Operation domain.Operation
	Changes   []domain.Change
}

// WriteGuard may refuse an operation (M5: the edit lock; M10: a schema's
// invariants; M11: freezes). It runs in the unit's transaction after the
// lock, the decision and the domain's checks, before the write; its error,
// a *shared.Error the operations it guards declare, rolls the unit back
// and is answered as it is.
type WriteGuard interface {
	GuardWrite(ctx context.Context, s Step) error
}

// Participant follows each operation of a unit, after its write and in its
// transaction (M6: rewriting the links to a renamed page), and may add
// operations to the unit through it. The added ones run like any other,
// guards included, but call no participant: participants do not recurse.
// An error rolls the unit back.
type Participant interface {
	Participate(ctx context.Context, s Step, u Appender) error
}

// Appender is how a participant adds an operation to the unit it follows.
// M4/P4 adds the content's write, which M6 appends.
type Appender interface {
	Rename(ctx context.Context, nodeID uuid.UUID, name string) (domain.Node, error)
}

// Event is a unit's changes, merged by node, the earliest before and the
// latest after, as the observers get them once per unit: the transaction
// sends at most one notification (v0.1 design 3.11).
type Event struct {
	Write
	ChangesetID uuid.UUID
	Changes     []domain.Change
}

// PageObserver follows a unit that changed something, when its operations
// are done and in its transaction (M5: the event stream's NOTIFY; M6: the
// link index). An error rolls the unit back.
type PageObserver interface {
	PagesChanged(ctx context.Context, e Event) error
}

// NotebookDeleted is notebooks being deleted, field by field as the
// notebook module's deletion tells it: bootstrap converts
// notebook.NotebookDeletion.
type NotebookDeleted struct {
	WorkspaceID uuid.UUID
	NotebookIDs []uuid.UUID
	By          uuid.UUID
	At          time.Time
}

// NotebookDeletion is the module's registrant of the notebook module's
// deletion (M4/P1 design 3.9): the notebooks' pages not deleted, what
// follows them and the notebooks' changesets, at the deletion's time. It
// runs in the deletion's transaction, which holds the notebooks' rows FOR
// NO KEY UPDATE: no page write of them runs beside it.
type NotebookDeletion struct {
	Pages NotebookPages
}

// NotebookDeleted follows the deletion d.
func (n NotebookDeletion) NotebookDeleted(ctx context.Context, d NotebookDeleted) error {
	return n.Pages.DeleteNotebooksPages(ctx, d.NotebookIDs, d.By, d.At)
}
