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
	"github.com/open-nerve/NerveWiki/server/internal/shared"
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
	// FindBySlug returns the id of the workspace not deleted with slug,
	// unlocked, and whether there is one: a search names its workspace.
	FindBySlug(ctx context.Context, slug string) (uuid.UUID, bool, error)
	// ShareByID locks the workspace not deleted with id FOR SHARE until the
	// transaction ends, and reports whether there is one.
	ShareByID(ctx context.Context, id uuid.UUID) (bool, error)
}

// VisibleNotebooks lists the notebooks an account sees: bootstrap hands
// notebook.NewVisibleNotebooks to it.
type VisibleNotebooks interface {
	// VisibleIn lists the ids of the notebooks not deleted of workspaceID
	// that userID, of role there, has a role in, unlocked.
	VisibleIn(ctx context.Context, workspaceID, userID uuid.UUID, role shared.WorkspaceRole) ([]uuid.UUID, error)
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

// Changeset is a write's entry in its notebook's log; a revert's names the
// changeset it reverts. Reverted, as LockChangeset reads it, tells whether
// a revert of it was made.
type Changeset struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	Kind       domain.ChangesetKind
	RevertsID  uuid.UUID
	Client     domain.Client
	Message    *string
	By         uuid.UUID
	At         time.Time
	Reverted   bool
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

// AnyNode is a node alive or in the trash: DeletedAt is when it was
// deleted, nil while it is alive; TrashRoot whether it is a trash item's
// root.
type AnyNode struct {
	domain.Node
	DeletedAt *time.Time
	TrashRoot bool
}

// Nodes reads a notebook's nodes: unlocked for a read, under the write
// unit's locks for a write. ErrNotFound for a missing or deleted node.
type Nodes interface {
	FindNode(ctx context.Context, id uuid.UUID) (domain.Node, error)
	// FindAnyNode is FindNode alive or in the trash (M8/P1 design 3.4):
	// ErrNotFound for a missing node alone.
	FindAnyNode(ctx context.Context, id uuid.UUID) (AnyNode, error)
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

// PathStore looks nodes up by their title keys: a path's segments, one
// point lookup each (M9/P1 design 3.9.2), the node not deleted, false for
// none; and writes nodes' paths. It reads in the transaction ctx carries,
// unlocked when it carries none.
type PathStore interface {
	RootByKey(ctx context.Context, notebookID uuid.UUID, key string) (domain.Node, bool, error)
	ChildByKey(ctx context.Context, notebookID, parentID uuid.UUID, key string) (domain.Node, bool, error)
	// PathsOf is the paths of ids' nodes not deleted of notebookID, by id,
	// as domain.PathOfNames writes them, in one statement whatever their
	// number (M9/P2 design 3.7). A node deleted, purged or of another
	// notebook has none.
	PathsOf(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error)
	// LinePaths is the paths of ids' nodes not deleted of notebookID, by
	// id, as PathsOf reads them, each step its node's id, title key and
	// name, its own last: a batch's former parents as its unit begins
	// (M9/P3 design 3.6.2).
	LinePaths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]domain.PathStep, error)
}

// AgentsStore finds a notebook's AGENTS page: its root node whose title
// key is TitleKey("AGENTS"), with its title now and its content's
// revision, unlocked.
type AgentsStore interface {
	AgentsPage(ctx context.Context, notebookID uuid.UUID, key string) (node uuid.UUID, kind domain.Kind, title string, revision int, ok bool, err error)
}

// OverviewStore reads a notebook's overview for MCP's open_notebook,
// unlocked (M9/P1 design 3.17.3).
type OverviewStore interface {
	// NotebookCounts are the notebook's pages and attachments not deleted.
	NotebookCounts(ctx context.Context, notebookID uuid.UUID) (pages, attachments int64, err error)
	// NotebookLastChange is the tree's last write: its changesets' latest
	// updated_at, the pages' part of the web's activity; nil for a
	// notebook without a changeset.
	NotebookLastChange(ctx context.Context, notebookID uuid.UUID) (*time.Time, error)
	// FirstChildren are the root's first n children, in order.
	FirstChildren(ctx context.Context, notebookID uuid.UUID, n int) ([]domain.Node, error)
}

// BatchLinks resolves and rewrites links for a batch's plan, on the tree
// the plan simulates (M9 design 4.1, 4.11; M9/P3 design 3.7.4):
// linking's, which bootstrap adapts from linking.NewBatchLinks(pool). It
// reads the index's rows and aliases as the batch began, on the pool; the
// tree and the parse are its caller's.
type BatchLinks interface {
	// Indexed is where the links of the page id led by the index, at its
	// revision, with their targets at their paths on tree, the plan's
	// before the batch: false when the index is not of revision or not of
	// the current extractor (M6's skips), its links then not rewritten.
	Indexed(ctx context.Context, notebookID, id uuid.UUID, revision int, tree domain.PlanTree) (domain.LinksBefore, bool, error)
	// Reached reports whether changes, the renames and moves of a batch's
	// ops before a page's first content op, a write, reach a link of the
	// page id by the index at revision, on tree after them, as Rewrite
	// would write it again (M9/P3 review A-I1), those that led to the
	// nodes of frozen left, the nodes of createdAfter out of the subtrees
	// of the nodes they key (BatchRewrite.CreatedAfter): the plan then
	// reads the page whole and compares the write with its content
	// rewritten for them. False when the index is not of revision or not
	// of the current extractor, as Indexed. What it reads of tree it keeps
	// in reads.
	Reached(ctx context.Context, notebookID, id uuid.UUID, revision int, changes []domain.Change, frozen []uuid.UUID,
		createdAfter map[uuid.UUID][]uuid.UUID, tree domain.PlanTree, reads *TreeReads) (bool, error)
	// Resolve is where each link of q's content, the page's at its path on
	// q's tree, leads there, by where its target starts, with the targets
	// as they are there.
	Resolve(ctx context.Context, q LinksOn) (domain.LinksBefore, error)
	// Rewrite is q's content with its links that led where q.Before tells
	// written again, as the participant writes them, so that each leads
	// where it led on q's tree after q's changes.
	Rewrite(ctx context.Context, q LinksMoved) (string, error)
}

// LinksOn is a content of a page of a batch's plan to resolve its links
// on: the notebook and the page; the plan's tree at the moment, whose
// deleted nodes' aliases are none (PlanTree.Gone); and the plan's parse,
// which admits a content within the plan's bounds and keeps its facts by
// page and hash (ContentParser.Planned).
type LinksOn struct {
	NotebookID uuid.UUID
	PageID     uuid.UUID
	Content    string
	Tree       domain.PlanTree
	Parse      func(ctx context.Context, content string) (Facts, error)
}

// LinksMoved is a content of a page of a batch's plan to rewrite, on the
// tree after the op it is rewritten for (LinksOn): where its links led
// when its content was read (Before), and the renames and moves since,
// merged node by node; Frozen the nodes those ops rename or move with
// update_links false, the links that led to them left as they are (M9
// design 4.11; M9/P3 review B-M1); CreatedAfter, for each node those ops
// rename or move, the nodes they create after the last that does, out of
// its subtree (BatchRewrite.CreatedAfter); and the reads of its tree kept
// for the calls on it (TreeReads).
type LinksMoved struct {
	LinksOn
	Before       domain.LinksBefore
	Changes      []domain.Change
	Frozen       []uuid.UUID
	CreatedAfter map[uuid.UUID][]uuid.UUID
	Reads        *TreeReads
}

// TreeReads is what the links read of one tree of a batch's plan, kept
// for the calls on that tree: the content ops between two of the batch's
// moves or deletes share one, the creations between them aside, so that a
// folder moved before them is read once, not once an op (M9/P3 review
// B-N2; fix check 2, A's F2-1). Held is the links' own, opaque here.
type TreeReads struct {
	Held any
}

// PageStates reads the pages' contents as MCP's list tells them, and
// which of its attachments are alive.
type PageStates interface {
	// PageStates reads each page's version, size and last write of ids,
	// alive, unlocked: one statement, planned with its argument (M9
	// design 4.1). A page without its content alive is not in the answer.
	PageStates(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]PageState, error)
	// AliveNodes reads which nodes of ids are not deleted, unlocked: one
	// statement, planned with its argument. list's attachments, whose
	// infos AttachmentInfos gives deleted or not (M9/P1 design 3.10.3).
	AliveNodes(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error)
}

// PageState is a page's content as list tells it, and its hash, by which
// a batch's plan tells a write of the bytes a page holds (M9/P3 design
// 3.4.4).
type PageState struct {
	Revision  int
	Bytes     int
	UpdatedAt time.Time
	Hash      []byte
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
	// RelocateNode puts n under its ParentID at its SortOrder, named its
	// Name, in one statement (M9 design 4.10).
	RelocateNode(ctx context.Context, n domain.Node) error
	SetSortOrder(ctx context.Context, id uuid.UUID, order float64) error
	// DeleteNodes deletes the deletion's nodes not deleted and what follows
	// them, its root a trash item with its counts. A node deleted at its
	// time already under one of them is an error: a unit deletes from the
	// top down.
	DeleteNodes(ctx context.Context, d Deletion) error
	// RestoreNodes brings the restoration's nodes back from the trash, with
	// what was deleted with them, and counts what it brought back.
	RestoreNodes(ctx context.Context, r Restoration) (RestoredRows, error)
	// MarkTrashRoots makes the nodes of a trash item deleted at at, each
	// still deleted then and no root, trash items of their own with their
	// counts (M8/P4 design 3.4): those a background restore's batch leaves
	// in the trash under the nodes it brought back. It counts those it
	// marked.
	MarkTrashRoots(ctx context.Context, roots []TrashMark, at time.Time) (int, error)
}

// TrashMark is a node a background restore's batch leaves in the trash,
// made a trash item: the pages and attachments of its subtree, itself
// included, and the levels of pages it spans (domain.GroupCounts).
type TrashMark struct {
	ID            uuid.UUID
	Pages, Assets int
	Height        int
}

// Deletion is a subtree's deletion (M8/P2 design 3.3): its nodes IDs, Root
// first among them, which becomes a trash item holding Pages pages and
// Assets attachments that span Height levels of pages; by By at At.
type Deletion struct {
	Root          uuid.UUID
	IDs           []uuid.UUID
	Pages, Assets int
	Height        int
	By            uuid.UUID
	At            time.Time
}

// Restoration is a trash item's restore (M8/P2 design 3.5): its nodes IDs,
// still deleted at DeletedAt, each named Names, its key NameKeys, the Root
// among them put under ParentID (nil: the notebook's root) at SortOrder;
// by By at At.
type Restoration struct {
	Root      uuid.UUID
	ParentID  *uuid.UUID
	SortOrder float64
	IDs       []uuid.UUID
	Names     []string
	NameKeys  []string
	DeletedAt time.Time
	By        uuid.UUID
	At        time.Time
}

// RestoredRows are what a restore brought back: its nodes; its pages'
// contents, and the versions at their contents' revisions; the versions
// that created a page; and the gaps in its pages' chains of versions, a
// version whose base no version of its page brought back has.
type RestoredRows struct {
	Nodes, Contents, CurrentVersions, Creations, Gaps int
}

// ChangesetWriter records a write's changeset, its items and its versions.
type ChangesetWriter interface {
	CreateChangeset(ctx context.Context, c Changeset) error
	// LockChangeset reads the changeset id not deleted, locked until the
	// transaction ends: a unit merges into it (M7/P6 design 3.2), unless it
	// was reverted (M8/P3 design 3.7). Its Message and At are not read.
	// ErrNotFound for none.
	LockChangeset(ctx context.Context, id uuid.UUID) (Changeset, error)
	// TouchChangeset moves the changeset id's updated_at to at: an edit
	// session wrote in it again.
	TouchChangeset(ctx context.Context, id uuid.UUID, at time.Time) error
	// RecordItems inserts the items of one changeset at one time, each a
	// node's of its own, or moves the after of the node's item in the
	// changeset on, keeping its before, its restore and its revision, in one
	// write. An item that deletes its node is deleted with it, at its time,
	// with the revision of its page's content deleted at that time, which a
	// merge into it takes (M8/P3 design 3.4).
	RecordItems(ctx context.Context, items []Item) error
	// RecordRevision inserts the version, or updates the page's in the
	// changeset, keeping its base, when r is on the version's revision: one
	// on another's write would break the chain of the page's versions, an
	// error.
	RecordRevision(ctx context.Context, r Revision) error
	// DropUnchanged deletes the items of the changeset id whose node is
	// where it was, its parent, name and order, then the changeset when it
	// holds no item and no version, which it answers (M9/P3 design 3.5.4):
	// a batch's node moved and moved back.
	DropUnchanged(ctx context.Context, id uuid.UUID) (bool, error)
	// MarkChangesetMoves sets that the changeset id renames, or moves, a
	// node, as m has it, each kept once set (M9/P3 design 3.20): the units
	// of a job's changeset each add theirs.
	MarkChangesetMoves(ctx context.Context, id uuid.UUID, m domain.ChangesetMoves) error
}

// Versions reads a page's versions (M8/P1 design 3.4), alive or in the
// trash, unlocked.
type Versions interface {
	// ListVersions is the page's versions newer-first, below the revision
	// below, at most limit; each with its changeset, without its display
	// name. No content.
	ListVersions(ctx context.Context, pageID uuid.UUID, below int, limit int) ([]domain.Version, error)
	// FindVersion is the page's first version at or above revision, and
	// its content as written, read at once: the one version that may hold
	// it, as each version's base is the revision of the one before.
	// ErrNotFound for none.
	FindVersion(ctx context.Context, pageID uuid.UUID, revision int) (domain.Version, string, error)
	// AliveChangesetOf is the changeset of the page's edit session alive
	// at now, zero when none is or it has not written: the page's newest
	// version is being edited while it is that changeset's.
	AliveChangesetOf(ctx context.Context, pageID uuid.UUID, now time.Time) (uuid.UUID, error)
}

// Trash reads a notebook's trash (M8/P2 design 3.4), unlocked, or in a
// unit's transaction. Its times compare with time.Time.Equal: the
// answer's may be in another location than the argument's (a test's
// pool's).
type Trash interface {
	// ListTrash is the notebook's trash items deleted after since, newer
	// first, below the cursor below, at most limit.
	ListTrash(ctx context.Context, notebookID uuid.UUID, since time.Time, below domain.TrashCursor, limit int) (
		[]domain.TrashRoot, error)
	// FindTrashRoot is the trash item of the root id, as ListTrash gives
	// it; ErrNotFound when id is no trash item's root.
	FindTrashRoot(ctx context.Context, id uuid.UUID) (domain.TrashRoot, error)
	// TrashRootOf is the root of the trash item the node id is in;
	// ErrNotFound when it is in none.
	TrashRootOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	// TrashLines are the roots ids' lines: each one's ancestors, deleted or
	// not, from the notebook's root down to its parent, none for a root at
	// the notebook's root.
	TrashLines(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.TrashAncestor, error)
	// TrashGroup is the trash item of the root id of the notebook deleted at
	// at: the root and its descendants deleted at at, level by level, each
	// level in order; ErrNotFound when id is not of the notebook deleted at
	// at.
	TrashGroup(ctx context.Context, notebookID, id uuid.UUID, at time.Time) ([]domain.TrashMember, error)
	// TrashContents are the contents of the pages ids deleted at at, by
	// page: one the purge took is not among them.
	TrashContents(ctx context.Context, ids []uuid.UUID, at time.Time) (map[uuid.UUID]string, error)
	// TrashHistory is how many versions and items, of every changeset, of
	// the nodes ids were deleted at at, each of the two counted to limit
	// at most: the rows a restore writes besides the nodes'.
	TrashHistory(ctx context.Context, ids []uuid.UUID, at time.Time, limit int) (int, error)
	// The background jobs' reads (M8/P4 design 3.3), each a level of many
	// nodes in one statement. TrashNodesAt are the nodes of the notebook,
	// each deleted at its time in at, by id: one no longer deleted then is
	// not among them. TrashGroups are the trash items of the roots of the
	// notebook, each deleted at its time, by root, as TrashGroup reads
	// each: one not deleted at its time is not among them. TrashShape is
	// the group of the trash item of the root id deleted at at below the
	// root, as a background restore's plan keeps it: a level after
	// another, each by parent, a parent's children in order.
	TrashNodesAt(ctx context.Context, notebookID uuid.UUID, at map[uuid.UUID]time.Time) (map[uuid.UUID]TrashNode, error)
	TrashGroups(ctx context.Context, notebookID uuid.UUID, roots map[uuid.UUID]time.Time) (map[uuid.UUID][]domain.TrashMember, error)
	TrashShape(ctx context.Context, notebookID, id uuid.UUID, at time.Time) ([]domain.ShapeNode, error)
}

// TrashNode is a node in the trash as TrashNodesAt reads it: as its group
// holds it (its level unread), and whether it is a trash item's root, with
// the root's counts.
type TrashNode struct {
	domain.TrashMember
	Root          bool
	Pages, Assets int
	Height        int
}

// History reads a notebook's log and what a revert reads of a changeset
// (M8/P3 design 3.4), unlocked or in a unit's transaction. Its times
// compare with time.Time.Equal, as the trash's do.
type History interface {
	// FindChangeset is the changeset id of a live notebook, with the brief
	// of the one it reverts while that one is there, its display names
	// empty; ErrNotFound for none.
	FindChangeset(ctx context.Context, id uuid.UUID) (domain.Changeset, error)
	// ListChangesets is the notebook's changesets the filter keeps, newer
	// first, below the cursor below, at most limit, as FindChangeset reads
	// each.
	ListChangesets(ctx context.Context, notebookID uuid.UUID, f domain.LogFilter, below domain.ChangesetCursor, limit int) (
		[]domain.Changeset, error)
	// ChangesetFacts are the changesets' counts, each to MaxLogCount, and
	// their reverts, newest first, MaxRevertedBy at most, by changeset.
	ChangesetFacts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.ChangesetFacts, error)
	// ChangesetMoves is whether the changeset renames or moves a node, of
	// all its items, as its units marked it (M9/P3 design 3.20): the single
	// changeset's.
	ChangesetMoves(ctx context.Context, id uuid.UUID) (domain.ChangesetMoves, error)
	// ListChanges are the changeset's changes by node id after after, at
	// most limit; FirstChanges the first n of each changeset's, by
	// changeset.
	ListChanges(ctx context.Context, id, after uuid.UUID, limit int) ([]domain.ChangeRow, error)
	FirstChanges(ctx context.Context, ids []uuid.UUID, n int) (map[uuid.UUID][]domain.ChangeRow, error)
	// The revert's reads (M8/P3 design 3.6): the changeset's items counted
	// to limit; its items and its versions, by node; nodes as they are
	// now, alive or not, a root with its group's counts and whether its
	// deletion undid the restore of the trash item the changeset reverted
	// made (one the purge took is not among them); the alive pages'
	// contents' revisions and hashes; versions' hashes and contents (a
	// version the purge took is not among them).
	CountItems(ctx context.Context, id uuid.UUID, limit int) (int, error)
	ChangesetItems(ctx context.Context, id uuid.UUID) ([]domain.ChangesetItem, error)
	ChangesetVersions(ctx context.Context, id uuid.UUID) ([]domain.ChangesetVersion, error)
	// ChangesetItemsOf and ChangesetVersionsOf are those of the nodes ids
	// alone: a background revert's batch's (M8/P4 design 3.5).
	ChangesetItemsOf(ctx context.Context, id uuid.UUID, ids []uuid.UUID) ([]domain.ChangesetItem, error)
	ChangesetVersionsOf(ctx context.Context, id uuid.UUID, ids []uuid.UUID) ([]domain.ChangesetVersion, error)
	NodesNow(ctx context.Context, ids []uuid.UUID, reverted uuid.UUID) (map[uuid.UUID]domain.NodeNow, error)
	ContentsNow(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.ContentNow, error)
	VersionHashes(ctx context.Context, refs []domain.VersionRef) (map[domain.VersionRef][]byte, error)
	VersionContents(ctx context.Context, refs []domain.VersionRef) (map[domain.VersionRef]PageContent, error)
}

// AttachmentInfo is an attachment's type and size.
type AttachmentInfo struct {
	MIME  string
	Bytes int64
}

// AttachmentInfos reads the attachments' type and size, deleted or not, in
// the caller's read (M8 design 4.1): bootstrap hands the asset module's to
// it. A node without a row of its file is not in the answer.
type AttachmentInfos interface {
	Of(ctx context.Context, nodeIDs []uuid.UUID) (map[uuid.UUID]AttachmentInfo, error)
}

// AttachmentReader reads an attachment for MCP's read: bootstrap adapts
// asset's AttachmentReads to it, asset.not_found to page.not_found and its
// ErrNoFile to ErrNoAttachmentFile (M9/P1 design 3.11.2).
type AttachmentReader interface {
	// Read reads the attachment nodeID, decided on asset.read: its row,
	// link and download address.
	Read(ctx context.Context, nodeID uuid.UUID) (Attachment, error)
	// Content reads a's file, at most max bytes, with the sides known of
	// it: the row's, or read from its bytes; 0 and 0 when neither.
	Content(ctx context.Context, a Attachment, max int64) (data []byte, width, height int, err error)
}

// Attachment is an attachment as read tells it: its node and file, its
// name, type and how a link writes it ("" without an extension), its size
// and SHA-256, its row's sides (0 when the server read none), and its
// content's download address, absolute, with when it expires: "" without
// server.public_url.
type Attachment struct {
	NodeID, BlobID   uuid.UUID
	Name, MIME, Link string
	Bytes            int64
	SHA256           []byte
	Width, Height    int
	URL              string
	Expires          time.Time
}

// ErrNoAttachmentFile is an attachment whose file is not in the store.
var ErrNoAttachmentFile = errors.New("page: the attachment's file is not in the store")

// NotebookPages deletes the pages of deleted notebooks, and their edit
// sessions.
type NotebookPages interface {
	DeleteNotebooksPages(ctx context.Context, ids []uuid.UUID, by uuid.UUID, at time.Time) error
	// QueueSearchOfNotebooks queues for the search's job, exact, the
	// nodes of the notebooks ids that their deletion deleted, those deleted
	// at its time at (M8/P5 design 3.4).
	QueueSearchOfNotebooks(ctx context.Context, ids []uuid.UUID, at time.Time) error
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

// ParseBudget bounds the content parsed and rendered at once, and the
// facts kept of it (M4/P4 review P2; M6 design 4.7): the largest content's
// parse can hold some 300 times its size in memory, its facts a tenth of
// that. Take holds a content's n bytes for its parse; it waits a while for
// them, then answers shared.ServerBusy. The composition root hands the
// module the one budget of the server.
type ParseBudget interface {
	Take(ctx context.Context, n int) (BudgetHold, error)
}

// BudgetHold is the bytes of the budget a content holds: KeepFacts gives
// back what its parse held beyond what facts, its facts, hold; Release all
// of it.
type BudgetHold interface {
	KeepFacts(facts Facts)
	Release()
}

// Markdown parses and renders a page's content (M4/P3 design 3.9):
// bootstrap hands platform/markdown to it through adapter/markdown.
type Markdown interface {
	// Facts parses content and keeps what the parse found, not its tree
	// (M6 design 4.7).
	Facts(content string) Facts
	// Render is content's reading view for page: it parses the content,
	// and the tree lives within the call.
	Render(ctx context.Context, content string, page PageRef) (Rendered, error)
	// Tasks are the content's task items in order (M5/P6 design 3.3); facts
	// is what Facts returned.
	Tasks(facts Facts) []Task
	// Links is how many links the content holds that facts, what Facts
	// returned, took: an import's unit holds at most so many (M7/P6
	// design 3.3).
	Links(facts Facts) int
	// FrontmatterEnd is where content's frontmatter ends, past its closing
	// line, as the parse reads it; 0 without one (M9/P1 design 3.15.3).
	FrontmatterEnd(content string) int
}

// Rendered is a reading view: its HTML, and when the attachments'
// addresses in it start expiring, zero for none (M7/P3 design 5.7).
type Rendered struct {
	HTML    string
	Expires time.Time
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
// that revision); Deleted, a page in the trash (M8/P2 design 3.7).
type PageRef struct {
	NotebookID uuid.UUID
	PageID     uuid.UUID
	Revision   int
	Deleted    bool
}

// SearchQueue queues the nodes a write changed for the search's job (M8/P5
// design 3.4), in the write's transaction.
type SearchQueue interface {
	// QueueSearch queues ids, nodes of notebookID in ascending order, each
	// with a new version; exact unless bulk and every earlier queueing of
	// the node still queued was bulk.
	QueueSearch(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID, bulk bool) error
}

// QueuedNode is a node in the search's queue, of its notebook, at the
// version it was queued with.
type QueuedNode struct {
	ID, NotebookID uuid.UUID
	Version        int64
}

// SearchSource is what the search's job decides on for a queued node: the
// node as it is now, gone when it is deleted or not there; a page's
// content's revision and size, zero for an attachment; and its row as the
// job built it, nil when there is none.
type SearchSource struct {
	ID       uuid.UUID
	Gone     bool
	Kind     domain.Kind
	Name     string
	Revision int
	Bytes    int
	Built    *BuiltSearchRow
}

// BuiltSearchRow is a node's row of the search as the job built it: the
// revision of the content it read, zero for an attachment's, the name and
// tokenizer, and when.
type BuiltSearchRow struct {
	Revision  int
	Name      string
	Tokenizer string
	At        time.Time
}

// SearchRow is a node's row of the search as the job writes it: its
// lexemes, a page's body too (nil for an attachment's), of the content at
// Revision (zero for an attachment).
type SearchRow struct {
	NodeID    uuid.UUID
	Revision  int
	Name      string
	Tokenizer string
	BuiltAt   time.Time
	Title     []string
	Body      []string
}

// SearchIndex is the search's rows and queue as its job keeps them (M8/P5
// design 3.4). Its statements' errors carry their SQLSTATE alone: a row's
// lexemes are a page's text.
type SearchIndex interface {
	// LockSearch takes the job's lock, on a connection of its own, without
	// waiting: false when another run holds it. The run's statements run on
	// that connection, under the context it returns, so that the lock is
	// the run's own (M8/P5A review B-M4). release gives it back.
	LockSearch(ctx context.Context) (run context.Context, release func(), ok bool, err error)
	// QueuedSearch reads the nodes queued exact after the node after.ID,
	// by id; or, when bulk is set, those queued in bulk after after, by
	// notebook (after.NotebookID), then id; at most limit nodes, unlocked.
	QueuedSearch(ctx context.Context, bulk bool, after QueuedNode, limit int) ([]QueuedNode, error)
	// SearchSources reads the nodes ids as SearchSource says, by id.
	SearchSources(ctx context.Context, ids []uuid.UUID) ([]SearchSource, error)
	// SearchContents reads the contents of the live pages ids, with their
	// revisions: a page deleted since is not in the answer.
	SearchContents(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]PageText, error)
	// WriteSearchRows writes rows and deletes the rows of the nodes
	// deleted, in ctx's transaction.
	WriteSearchRows(ctx context.Context, rows []SearchRow, deleted []uuid.UUID) error
	// DequeueSearch takes the nodes off the queue at the versions read,
	// those another transaction holds left there.
	DequeueSearch(ctx context.Context, nodes []QueuedNode) error
	// AnalyzeSearch updates the rows' statistics.
	AnalyzeSearch(ctx context.Context) error
}

// PageText is a page's content at its revision.
type PageText struct {
	Content  string
	Revision int
}

// SearchScope is what a search's statements read: the notebooks searched,
// the query's keys and patterns, the cursor's position below which the
// tier goes on, and, for the body tier's, Hits: its candidates by their
// lexemes, as SearchHitCount counted them in the read's snapshot.
type SearchScope struct {
	Notebooks []uuid.UUID
	Query     domain.SearchQuery
	Below     domain.SearchCursor
	Hits      int
}

// SearchTitleRow is a node of the title tier: the content's first hit,
// its character from 1 (0 when the content holds none or the node is an
// attachment), the excerpt the SQL cut around it, and the content's
// characters.
type SearchTitleRow struct {
	Node    SearchNode
	SortAt  time.Time
	At      int
	Excerpt string
	Chars   int
}

// SearchNode is a node a search found.
type SearchNode struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	Kind       domain.Kind
	Name       string
}

// SearchBodyRow is a page of the body tier's walk: its place in it, from 1,
// how many the walk took (one more than walk: more to walk), whether it
// passed, and when it did its first hit as SearchTitleRow's.
type SearchBodyRow struct {
	SearchTitleRow
	Seq, Walked int
	Passed      bool
}

// SearchReads are a search's statements (M8/P5 design 3.5), each in the
// read-only snapshot ctx carries. Their errors carry their SQLSTATE alone:
// the patterns and keys are the user's text.
type SearchReads interface {
	// SearchNFC is texts in NFC as the statements read a content in it,
	// in their order: a query's terms Go's NFC changed
	// (domain.SearchQuery.Written).
	SearchNFC(ctx context.Context, texts []string) ([]string, error)
	// SearchTitles reads the title tier below s.Below, at most limit nodes.
	SearchTitles(ctx context.Context, s SearchScope, limit int) ([]SearchTitleRow, error)
	// SearchHitCount counts the body tier's candidates by their lexemes:
	// the pages of s's notebooks whose row holds s.Query's body keys and
	// titled keys, read by those notebooks' keys alone (M8 Codex review
	// R1): no other notebook's rows change what it reads. The body
	// tier's statements read them all, by the count (SearchScope.Hits),
	// which tells their plans how many there are: a count of another
	// snapshot may leave some out.
	SearchHitCount(ctx context.Context, s SearchScope) (int, error)
	// SearchBodies walks the body tier below s.Below: at most walk pages'
	// contents read, until limit pass.
	SearchBodies(ctx context.Context, s SearchScope, walk, limit int) ([]SearchBodyRow, error)
	// SearchTitleCount counts the title tier from its top to limit, and
	// the pages among them.
	SearchTitleCount(ctx context.Context, s SearchScope, limit int) (titles, pages int, err error)
	// SearchBodyCount counts the body tier's candidates, its pages by
	// their lexemes or queued, to limit, their names unchecked.
	SearchBodyCount(ctx context.Context, s SearchScope, limit int) (int, error)
	// SearchBuilding reports whether the notebooks hold nodes queued in
	// bulk, which no search finds yet.
	SearchBuilding(ctx context.Context, notebooks []uuid.UUID) (bool, error)
}
