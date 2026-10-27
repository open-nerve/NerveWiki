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
// wrote, from where, with which options, at the unit's one time; the kind
// of its changeset, and the changeset a revert reverts (M8 design 4.1:
// M10's and M11's guards tell a restore or a revert from an edit).
type Write struct {
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	By          uuid.UUID
	Client      domain.Client
	Options     Options
	At          time.Time
	Kind        domain.ChangesetKind
	RevertsID   uuid.UUID
	// Notebook is the notebook's mode and source root, read once under
	// the notebook's row lock as the unit began (M10 design 4.6): the
	// mode guard's contract and its prechecks'.
	Notebook domain.NotebookState
	// Grants is the unit's memo of the actions it asked beyond its own
	// decision (Grants): the unit's own, a pointer, since each step and
	// each guard has the Write by value.
	Grants *Grants
}

// Step is one operation of a unit: its kind, and each node it changes,
// where the node was and where it is after, with the content it wrote and
// its facts. A guard gets it before the write, a participant after.
type Step struct {
	Write
	Operation domain.Operation
	Changes   []domain.Change
	Content   *StepContent
	Rewrite   bool
	Lines     *StepLines
	Kinds     map[uuid.UUID]domain.Kind
	// AreaHashes is prepared once before guards and maintained after the write.
	AreaHashes map[uuid.UUID]SourceValue
	// EditSessionID is the edit session a content write is made in (M5:
	// the edit lock is the session's); zero for any other write.
	EditSessionID uuid.UUID
	// Asset is the file of the attachment a creation makes (M7/P2 design
	// 3.3), for the guards (M10: a schema's rules of a notebook's files);
	// nil for any other operation.
	Asset *AssetMeta
	// Batch is what a batch's deferred step has its participants follow
	// besides its changes (M9/P3 design 3.7.1); nil for any other step.
	Batch *BatchRewrite
}

// StepContent is the content written and the bytes it replaces. Merged counts
// the source writes a batch coalesced into this operation.
type StepContent struct {
	Hash                       []byte
	Bytes, BeforeBytes, Merged int
}

// StepLines are the operation root's ancestors on each side. Nil Lines means
// not supplied; an empty side is the known notebook root. Guards only read it.
type StepLines struct {
	Before, After []domain.Ancestor
	// AfterParents supplies each external parent's line when skips leave
	// multiple restore roots in one step. Changed parents use Changes instead.
	AfterParents map[uuid.UUID][]domain.Ancestor
}

// BatchRewrite is what a batch's deferred participants follow besides its
// changes (M9 design 4.11; M9/P3 design 3.7): each page the batch's
// content ops decided, alive after it, whether or not the unit wrote a
// version of it (Codex review R1); the paths the moved and renamed nodes'
// former parents had as the unit began; the nodes the unit deleted; the
// most pages the rewrite may write, a page of Written its content ops wrote
// counting WrittenWeight (M9/P3 review B-M3), one they put back 1 (Codex
// review R1), the most links (review B1-M3), and the most bytes of the
// pages it writes (M9/P3 fix round 5, B2-M1); and the first op that renames
// or moves with update_links, which a refusal names.
type BatchRewrite struct {
	Written       map[uuid.UUID]WrittenPage
	Parents       map[uuid.UUID][]domain.PathStep
	Gone          []uuid.UUID
	MaxPages      int
	WrittenWeight int
	MaxLinks      int
	MaxBytes      int
	FirstMove     int
	// Frozen is the nodes an op of the batch renamed or moved with
	// update_links false, in id order, which Unit.Participate sets: the
	// links that led to them are left as they are, in every page, whatever
	// their ancestors' changes (M9 design 4.11; M9/P3 review B-M1).
	Frozen []uuid.UUID
	// CreatedAfter is, for each node an op of the batch renamed or moved
	// with update_links, the nodes it created after the last such op of
	// it: no link reached them then, so the rewrite of the pages it did not
	// write leaves them out of that node's subtree, which it reads as the
	// unit left it (domain.BatchPlan.CreatedAfterMoves; M9 design 4.11: a
	// creation only resolves again; M9/P3 review B1-M1).
	CreatedAfter map[uuid.UUID][]uuid.UUID
	// FrozenMoves is the renames and moves of the nodes of Frozen, merged
	// node by node over the batch (domain.BatchPlan.FrozenMovesBetween): not
	// followed, but where they were, which the paths before of the nodes
	// under them that the rewrite follows take their names and former
	// parents from, those parents' paths among Parents (M9/P3 review
	// B1-N1: a Markdown link's text that was the old path is recognized).
	FrozenMoves []domain.Change
}

// WrittenPage is a page a batch's content ops decided, for its deferred
// rewrite: where its links led at its last content op, on the plan's tree
// after that op; its
// own changes, the renames and moves after that op, merged node by node
// with no revision, of the nodes alive then, or created since, and alive
// after the unit, that no op after it moves with update_links false: those
// nodes alone, the participant reading the subtree of each it renames or
// moves once for the batch (M9/P3 design 3.7.2 (e)), so that a moved
// folder's nodes are held once, not once a page; and the facts DecidedAll
// kept for its final content, which the rewrite takes its links from
// instead of parsing it again. Frozen is the nodes the ops after that op
// rename or move with update_links false, whose links it leaves;
// CreatedAfter, for each node they rename or move, the nodes they create
// after the last that does, out of its subtree (BatchRewrite.CreatedAfter).
// Unwritten tells a page its content ops wrote no version of, though its
// rewrite may: they left it the bytes it held before the batch, an edit
// back to them after a move rewrote it too (Codex review R1). Its links are
// followed from its last content op all the same, as separate calls' index
// has them then; its rewrite weighs as a page of the index's, as it is in
// separate calls: WrittenWeight is the cost of a page's write and rewrite
// in the unit (M9/P3 review B-M3), and its content ops did not write this
// one.
type WrittenPage struct {
	Before       domain.LinksBefore
	Changes      []domain.Change
	Facts        Facts
	Frozen       []uuid.UUID
	CreatedAfter map[uuid.UUID][]uuid.UUID
	Unwritten    bool
}

// AssetMeta is what an attachment's file is: its type, as the server
// determined it, its size and its SHA-256.
type AssetMeta struct {
	MIME   string
	Bytes  int64
	SHA256 []byte
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
	// Defer has f called once the unit is over, its observers done and its
	// transaction committed or rolled back, in a seed's unit when Seed
	// returns: M6 gives back then the parse budget the facts of the
	// contents it wrote hold, which the observers read (M6/P4 design 4.1).
	Defer(f func())
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
// follows them and the notebooks' changesets, at the deletion's time, the
// nodes it deleted queued for the search's job (M8/P5 design 3.4), and
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
	if err := n.Pages.QueueSearchOfNotebooks(ctx, d.NotebookIDs, d.At); err != nil {
		return err
	}
	sessions, err := n.Pages.DeleteNotebookSessions(ctx, d.NotebookIDs)
	if err != nil {
		return err
	}
	return tellEnded(ctx, n.Subscribers, d.WorkspaceID, sessions, domain.EndedWithPage, d.By, d.At)
}
