package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The module's extension points (M4 design 8; M4/P1 design 3.6; M4/P4
// design 3.7), and its registrant of the notebook module's deletion. M5's
// edit lock is the first registrant: a vetoer of the openings and a guard
// of the writes (M5 design 4.4); the module root's test proves each point
// reaches its path.

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
// where the node was and where it is after, with the content it wrote and
// its facts. A guard gets it before the write, a participant after.
type Step struct {
	Write
	Operation domain.Operation
	Changes   []domain.Change
	// EditSessionID is the edit session a content write is made in (M5:
	// the edit lock is the session's); zero for any other write.
	EditSessionID uuid.UUID
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

// Appender is how a participant adds an operation to the unit it follows:
// M6 rewrites the content of the pages that link to a renamed one.
type Appender interface {
	Rename(ctx context.Context, nodeID uuid.UUID, name string) (domain.Node, error)
	// WriteContent writes a page's content as Unit.WriteContent does, in no
	// edit session: the participant read the page at w.Base in the unit's
	// transaction, and took the new content's facts.
	WriteContent(ctx context.Context, w ContentWrite) (int, error)
}

// Event is a unit's changes, merged by node, the earliest before and the
// latest after, as the observers get them once per unit: the event stream
// publishes one pages event of it (M5 design 8), beside a lock event of
// each edit session the unit opens or ends.
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

// SessionOpening is an edit session about to open, as its vetoers see it:
// the unit's write and the page, and whether the opener takes their own
// session of the page over (M5 design 4.2), which the unit has ended
// before the vetoers run.
type SessionOpening struct {
	Write
	PageID   uuid.UUID
	TakeOver bool
}

// EditSessionVetoer may refuse an edit session's opening (M5: an alive
// session of the page, page.locked; M11: a freeze). It runs in
// the opening's unit after the decision, under the page's gate, the content
// row's lock, so that two openings of a page decide one after the other;
// its error, a *shared.Error the opening declares, rolls the unit back and
// is answered as it is.
type EditSessionVetoer interface {
	VetoEditSession(ctx context.Context, o SessionOpening) error
}

// SessionOpened is an edit session's opening: the session, where its page
// is, its page and owner, and when.
type SessionOpened struct {
	SessionID   uuid.UUID
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	PageID      uuid.UUID
	UserID      uuid.UUID
	At          time.Time
}

// SessionEnded is an edit session's end: the session, where its page is,
// its page and owner, why, by whom and when. Its owner ends it, takes it
// over or deletes its page, alone, with a subtree or with its notebook,
// or an admin of its notebook releases its lock (M5 design 4.2).
type SessionEnded struct {
	SessionID   uuid.UUID
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	PageID      uuid.UUID
	UserID      uuid.UUID
	Reason      domain.EndReason
	By          uuid.UUID
	At          time.Time
}

// EditSessionSubscriber follows the openings of the edit sessions, and the
// ends of those alive when they end (M5: the lock's changes pushed), in
// the opening's or the end's transaction; an error rolls it back. A
// session that expired ended with its lease: no end tells it, and the
// cleanup of the expired ones tells no one.
type EditSessionSubscriber interface {
	EditSessionOpened(ctx context.Context, o SessionOpened) error
	EditSessionEnded(ctx context.Context, e SessionEnded) error
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
// follows them and the notebooks' changesets, at the deletion's time, and
// their edit sessions, whose subscribers follow the alive ones' end (M4/P4
// design 3.7). It runs in the deletion's transaction, which holds the
// notebooks' rows FOR NO KEY UPDATE: no page write of them runs beside it.
type NotebookDeletion struct {
	Pages       NotebookPages
	Subscribers []EditSessionSubscriber
}

// NotebookDeleted follows the deletion d.
func (n NotebookDeletion) NotebookDeleted(ctx context.Context, d NotebookDeleted) error {
	if err := n.Pages.DeleteNotebooksPages(ctx, d.NotebookIDs, d.By, d.At); err != nil {
		return err
	}
	sessions, err := n.Pages.DeleteNotebookSessions(ctx, d.NotebookIDs)
	if err != nil {
		return err
	}
	return tellEnded(ctx, n.Subscribers, d.WorkspaceID, sessions, domain.EndedWithPage, d.By, d.At)
}
