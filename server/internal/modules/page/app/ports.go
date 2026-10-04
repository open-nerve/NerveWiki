// Package app holds the page module's use cases and the ports they need
// (M4 design 4; M4/P1 design 3.6): the write unit every page write runs in,
// and the reads.
package app

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// ErrNotFound is what a repository returns for a missing row.
var ErrNotFound = errors.New("not found")

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// Tx runs a write unit's transaction. A unit opens the outermost one: a
// nested WithinTx joins the outer transaction, which the unit could then
// not commit, nor keep to one event; InTx tells whether ctx carries one
// already.
type Tx interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
	InTx(ctx context.Context) bool
}

// Workspaces is what the module reads of workspaces: bootstrap hands
// workspace.NewWorkspaces to it.
type Workspaces interface {
	// ShareByID locks the workspace not deleted with id FOR SHARE until the
	// transaction ends, and reports whether there is one.
	ShareByID(ctx context.Context, id uuid.UUID) (bool, error)
}

// Notebooks is what the module reads of notebooks: bootstrap hands
// notebook.NewNotebooks to it.
type Notebooks interface {
	// WorkspaceOf returns the workspace of the notebook not deleted with
	// id, unlocked, and whether there is one.
	WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error)
	// ShareByID locks the notebook not deleted with id FOR SHARE until the
	// transaction ends, and reports whether there is one.
	ShareByID(ctx context.Context, id uuid.UUID) (bool, error)
	// LockByID is ShareByID FOR NO KEY UPDATE: a write of the tree.
	LockByID(ctx context.Context, id uuid.UUID) (bool, error)
}

// Content is a page's content as a write leaves it.
type Content struct {
	NodeID   uuid.UUID
	Content  string
	Revision int
	Hash     []byte // SHA-256 of Content
	ByteSize int
	By       uuid.UUID
	At       time.Time
}

// PageContent is a page's content, its version and its SHA-256.
type PageContent struct {
	Content  string
	Revision int
	Hash     []byte
}

// ContentLock is a page's content as its gate holds it: its version, and
// its hash and size, which tell whether a write would change it.
type ContentLock struct {
	Revision int
	Hash     []byte
	ByteSize int
}

// ContentMeta is what a page tells of its content besides the content.
type ContentMeta struct {
	Revision  int
	ByteSize  int
	UpdatedBy uuid.UUID
	UpdatedAt time.Time
}

// Changeset is a write's entry in its notebook's log.
type Changeset struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	Kind       string
	Client     domain.Client
	Message    *string
	By         uuid.UUID
	At         time.Time
}

// Item is a node's change in a changeset: where the node was before the
// changeset and where it is after.
type Item struct {
	ID          uuid.UUID
	ChangesetID uuid.UUID
	Change      domain.Change
	At          time.Time
}

// Revision is a page's content as a changeset leaves it.
type Revision struct {
	ID          uuid.UUID
	ChangesetID uuid.UUID
	NodeID      uuid.UUID
	Base        *int // nil: the changeset created the page
	Revision    int
	Content     string
	Hash        []byte
	ByteSize    int
	At          time.Time
}

// Nodes reads a notebook's nodes: unlocked for a read, under the write
// unit's locks for a write. ErrNotFound for a missing or deleted node.
type Nodes interface {
	FindNode(ctx context.Context, id uuid.UUID) (domain.Node, error)
	// FindNodeIn is FindNode within the notebook.
	FindNodeIn(ctx context.Context, notebookID, id uuid.UUID) (domain.Node, error)
	ListNodes(ctx context.Context, notebookID uuid.UUID) ([]domain.Node, error)
	// Children are a parent's children not deleted, the root's when
	// parentID is nil, in order.
	Children(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID) ([]domain.Node, error)
	// Ancestors are a node's ancestors from the root down to its parent.
	Ancestors(ctx context.Context, id uuid.UUID) ([]domain.Ancestor, error)
	// Subtree is the node id of the notebook and its descendants, level
	// by level; ErrNotFound when the node is missing, deleted or in
	// another notebook.
	Subtree(ctx context.Context, notebookID, id uuid.UUID) (domain.Subtree, error)
	ContentMeta(ctx context.Context, id uuid.UUID) (ContentMeta, error)
	// PageContent is a page's content and its version, read in one
	// statement; ErrNotFound when the content is deleted, as it is with its
	// node.
	PageContent(ctx context.Context, id uuid.UUID) (PageContent, error)
	// LockContent locks the content row of the page id not deleted of the
	// notebook FOR NO KEY UPDATE: the page's gate (M4 design 4). The node's
	// row is not locked. ErrNotFound for no such page.
	LockContent(ctx context.Context, notebookID, id uuid.UUID) (ContentLock, error)
}

// NodeWriter writes nodes in the write unit's transaction. A name its
// siblings hold already is domain.ErrTitleTaken.
type NodeWriter interface {
	CreateNode(ctx context.Context, n domain.Node) error
	CreateContent(ctx context.Context, c Content) error
	// WriteContent writes a page's content, under LockContent's lock.
	WriteContent(ctx context.Context, c Content) error
	RenameNode(ctx context.Context, n domain.Node) error
	// MoveNode puts n under its ParentID at its SortOrder.
	MoveNode(ctx context.Context, n domain.Node) error
	SetSortOrder(ctx context.Context, id uuid.UUID, order float64) error
	// DeleteNodes deletes the nodes ids not deleted and what follows them,
	// at at, by by.
	DeleteNodes(ctx context.Context, ids []uuid.UUID, by uuid.UUID, at time.Time) error
}

// ChangesetWriter records a write's changeset, its items and its versions.
type ChangesetWriter interface {
	CreateChangeset(ctx context.Context, c Changeset) error
	// TouchChangeset moves the changeset id's updated_at to at: an edit
	// session wrote in it again.
	TouchChangeset(ctx context.Context, id uuid.UUID, at time.Time) error
	// RecordItem inserts the item, or moves the after of the node's item in
	// the changeset on, keeping its before. An item that deletes its node
	// is deleted with it, at its time.
	RecordItem(ctx context.Context, it Item) error
	// RecordRevision inserts the version, or updates the page's in the
	// changeset, keeping its base.
	RecordRevision(ctx context.Context, r Revision) error
}

// NotebookPages deletes the pages of deleted notebooks, and their edit
// sessions.
type NotebookPages interface {
	DeleteNotebooksPages(ctx context.Context, ids []uuid.UUID, by uuid.UUID, at time.Time) error
	// DeleteNotebookSessions deletes the notebooks' edit sessions and
	// returns them.
	DeleteNotebookSessions(ctx context.Context, ids []uuid.UUID) ([]EditSession, error)
}

// EditSession is an edit session (M4 design 4): who edits which page from
// where, the changeset its writes go to and the revision it last wrote
// (zero both before its first write), and its lease. A tombstone (M5
// design 4.3), a session taken over or unlocked, has its end's reason, by
// whom and when; a session not ended has none of them.
type EditSession struct {
	ID          uuid.UUID
	NodeID      uuid.UUID
	NotebookID  uuid.UUID
	UserID      uuid.UUID
	Client      domain.Client
	ChangesetID uuid.UUID
	Revision    int
	CreatedAt   time.Time
	ExpiresAt   time.Time
	EndedReason domain.EndReason
	EndedByID   uuid.UUID
	EndedAt     time.Time
}

// Alive reports whether the session holds its page's lock at now: it has
// not ended, and its lease lasts past now (M5 design 4.1).
func (s EditSession) Alive(now time.Time) bool {
	return s.EndedReason == "" && s.ExpiresAt.After(now)
}

// SessionsEnd is an end that leaves tombstones (M5 design 4.2, 4.3): the
// page's sessions alive at At, of UserID alone when it is not nil, end for
// Reason by By, and keep their rows until Until at least.
type SessionsEnd struct {
	NodeID uuid.UUID
	UserID *uuid.UUID
	Reason domain.EndReason
	By     uuid.UUID
	At     time.Time
	Until  time.Time
}

// SessionWriter is what a write unit does to edit sessions, under its
// page's content row's lock. ErrNotFound for no such session.
type SessionWriter interface {
	CreateSession(ctx context.Context, s EditSession) error
	// LockSession locks the session id FOR UPDATE.
	LockSession(ctx context.Context, id uuid.UUID) (EditSession, error)
	// SetSessionWrite records that the session's write went to the
	// changeset and wrote revision.
	SetSessionWrite(ctx context.Context, id, changesetID uuid.UUID, revision int) error
	// DeleteNodeSessions deletes the sessions of the pages ids and returns
	// them.
	DeleteNodeSessions(ctx context.Context, ids []uuid.UUID) ([]EditSession, error)
	// DeleteExpiredSessionsOf deletes the page id's sessions expired at
	// now, tombstones among them: an opening's first step, and an
	// unlock's, so that a heartbeat that read an earlier time finds no row
	// to keep alive.
	DeleteExpiredSessionsOf(ctx context.Context, id uuid.UUID, now time.Time) error
	// EndAliveSessions makes tombstones of the sessions e ends and returns
	// them, ended.
	EndAliveSessions(ctx context.Context, e SessionsEnd) ([]EditSession, error)
}

// Sessions is what a heartbeat and an end do: each one statement on the
// caller's own session alive at now, outside the write units' lock order.
// ErrNotFound for none: missing, expired, or someone else's.
type Sessions interface {
	// FindLiveSession reads the session, unlocked.
	FindLiveSession(ctx context.Context, id, userID uuid.UUID, now time.Time) (EditSession, error)
	// HeartbeatSession keeps the session until until.
	HeartbeatSession(ctx context.Context, id, userID uuid.UUID, now, until time.Time) (EditSession, error)
	// EndSession deletes the session, or the caller's tombstone id, and
	// returns it.
	EndSession(ctx context.Context, id, userID uuid.UUID, now time.Time) (EditSession, error)
	// FindEndedSession reads the caller's tombstone id, unlocked: why a
	// session the others found not alive ended.
	FindEndedSession(ctx context.Context, id, userID uuid.UUID) (EditSession, error)
}

// AliveSessions reads the edit locks of pages: their sessions alive at
// now, by when they opened, unlocked (M5 design 4.4: the unit that reads
// them holds what serializes it with an opening).
type AliveSessions interface {
	AliveSessionsOf(ctx context.Context, ids []uuid.UUID, now time.Time) ([]EditSession, error)
}

// Names reads the accounts' display names, which a lock's problem and read
// give (M5 design 4.5): bootstrap hands identity's directory to it. An id
// of no account is left out, and its name is empty where it is given; the
// sessions' ids reference users, which are never deleted, so none is.
type Names interface {
	DisplayNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// ExpiredSessions deletes the sessions expired at now, at most batch, and
// skips the rows another transaction holds; it returns how many it deleted.
type ExpiredSessions interface {
	DeleteExpiredSessions(ctx context.Context, now time.Time, batch int) (int, error)
}

// ParseBudget bounds the content parsed and rendered at once (M4/P4
// review P2; M6 design 4.7): the largest content's parse can hold some 300
// times its size in memory. Take holds n bytes of it until release; it
// waits a while for them, then answers shared.ServerBusy. The composition
// root hands the module the one budget of the server.
type ParseBudget interface {
	Take(ctx context.Context, n int) (release func(), err error)
}

// Markdown parses and renders a page's content (M4/P3 design 3.9):
// bootstrap hands platform/markdown to it through adapter/markdown.
type Markdown interface {
	// Facts parses content and keeps what the parse found, not its tree
	// (M6 design 4.7).
	Facts(content string) Facts
	// Render is the HTML of content's reading view for page: it parses the
	// content, and the tree lives within the call.
	Render(ctx context.Context, content string, page PageRef) (string, error)
	// Tasks are the content's task items in order (M5/P6 design 3.3); facts
	// is what Facts returned.
	Tasks(facts Facts) []Task
}

// Facts is what a parse of a page's content found, without its tree. The
// use cases do not look into it: they hand it to Tasks, and through the
// unit to the guards, the participants and the observers.
type Facts any

// Task is a task item of a page's content: Offset is the byte position of
// the character between its brackets.
type Task struct {
	Offset  int
	Checked bool
}

// PageRef is the page a Render is for, and the revision of its content
// rendered: an extension fetches its data for it (M6: the link index of
// that revision).
type PageRef struct {
	NotebookID uuid.UUID
	PageID     uuid.UUID
	Revision   int
}
