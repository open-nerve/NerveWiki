package domain

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ChangesetKind is what kind of write a changeset records (v0.1 design
// 3.8): an edit, or from M7 an import, whose units all write in its first
// unit's changeset (M7/P6 design 3.2); from M8 a version restored, a trash
// item restored, a changeset reverted (M8 design 4.2).
type ChangesetKind string

// The kinds of changeset.
const (
	ChangesetEdit           ChangesetKind = "edit"
	ChangesetImport         ChangesetKind = "import"
	ChangesetVersionRestore ChangesetKind = "version_restore"
	ChangesetTrashRestore   ChangesetKind = "trash_restore"
	// ChangesetRevert reverts another changeset, which it names.
	ChangesetRevert ChangesetKind = "revert"
)

// A notebook's log (M8 design 4.7; M8/P3 design 3.4): its changesets newer
// first, each with its counts, its reverts and its first changes; a
// changeset's changes by node.

// MaxLogCount is how far the log counts a changeset's items and versions
// (M8 design 4.14): MaxLogCount is as many or more.
const MaxLogCount = 1000

// MaxRevertedBy is how many of a changeset's reverts the log names, the
// newest.
const MaxRevertedBy = 100

// MaxMessageBytes is how long a changeset's message is at most, in bytes.
const MaxMessageBytes = 4096

// Changeset is a changeset as the log lists it: who made it, by name,
// which the reads fill, from which client and with what message; when it
// was first and last written; whether the purge trimmed it, which can no
// longer be reverted whole; and a revert's changeset, RevertsID, with its
// brief while the purge has not taken it.
type Changeset struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	Kind       ChangesetKind
	Client     Client
	Message    *string
	By         uuid.UUID
	ByName     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Trimmed    bool
	RevertsID  uuid.UUID
	Reverts    *ChangesetBrief
}

// ChangesetBrief is a changeset another names, a revert the one it
// reverts: its kind, who made it, by name, and when.
type ChangesetBrief struct {
	ID        uuid.UUID
	Kind      ChangesetKind
	By        uuid.UUID
	ByName    string
	CreatedAt time.Time
}

// ChangesetFacts are what the log counts of a changeset: its items, the
// nodes it changed in the tree, and its versions, the pages whose content
// it wrote, each to MaxLogCount; and the changesets that reverted it, the
// newest first, MaxRevertedBy at most.
type ChangesetFacts struct {
	Nodes      int
	Contents   int
	RevertedBy []uuid.UUID
}

// ChangesetMoves tell whether a changeset renames a node or moves one to
// another parent, of all its items (M9/P3 design 3.20): the web's revert
// warns that the links written to the new place stay.
type ChangesetMoves struct {
	Renames bool
	Moves   bool
}

// ChangesetCursor is a page of a notebook's log's cursor: the next page
// lists the changesets created before At, or at At with an id below ID: the
// last one listed.
type ChangesetCursor struct {
	At time.Time `json:"at"`
	ID uuid.UUID `json:"id"`
}

// MarshalJSON writes the cursor with At in UTC, so that each position has
// one spelling: shared.DecodeCursor refuses another.
func (c ChangesetCursor) MarshalJSON() ([]byte, error) {
	type plain ChangesetCursor
	return json.Marshal(plain{At: c.At.UTC(), ID: c.ID})
}

// ChangesetFrom is the cursor above every changeset: the first page's. A
// cursor at no time would read as one at the first time (pgx sends no
// infinity).
func ChangesetFrom() ChangesetCursor {
	return ChangesetCursor{At: time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), ID: uuid.Max()}
}

// LogFilter is what of a notebook's log a read keeps: one actor's
// changesets, one client's or every MCP client's, those that changed a
// node; nil and empty keep all.
type LogFilter struct {
	Actor  *uuid.UUID
	Client Client
	AllMCP bool
	PageID *uuid.UUID
}

// ParseClientFilter is the log's client filter s: web, api, cli, mcp for
// every MCP client, or an MCP client's mcp:<name> as the changesets table
// takes it. Anything else, bytes that are no UTF-8 first, is 422 on
// client: the database would refuse them, 500.
func ParseClientFilter(s string) (client Client, allMCP bool, err error) {
	switch {
	case !utf8.ValidString(s):
		return "", false, clientFilterInvalid("is not UTF-8")
	case s == "mcp":
		return "", true, nil
	case !Client(s).Valid():
		return "", false, clientFilterInvalid("is none of web, api, cli, mcp, or mcp: and a client's name")
	}
	return Client(s), false, nil
}

func clientFilterInvalid(message string) error {
	return shared.Invalid(shared.FieldError{Field: "client", Code: shared.FieldInvalidFormat, Message: message})
}

// ParseActor is the log's actor s (M9 design 4.9): "me", the caller me,
// which an agent has no other way to name, or an account's id as
// uuid.Parse reads one; 422 on actor otherwise. The account is not looked
// up, as REST's actor is not: one of no changeset keeps none.
func ParseActor(s string, me uuid.UUID) (uuid.UUID, error) {
	if s == "me" {
		return me, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, shared.Invalid(shared.FieldError{Field: "actor", Code: shared.FieldInvalidFormat,
			Message: `is an account's id or "me"`})
	}
	return id, nil
}

// CheckMessage checks a changeset's message, given: 1–MaxMessageBytes
// bytes of UTF-8 without NUL, which the changesets table takes; 422 on
// message otherwise.
func CheckMessage(s string) error {
	f := shared.FieldError{Field: "message"}
	switch {
	case s == "":
		f.Code, f.Message = shared.FieldTooShort, "must not be empty"
	case len(s) > MaxMessageBytes:
		f.Code, f.Message = shared.FieldTooLong, "is longer than 4096 bytes"
	case !utf8.ValidString(s) || strings.ContainsRune(s, 0):
		f.Code, f.Message = shared.FieldInvalidFormat, "is not UTF-8 without NUL"
	default:
		return nil
	}
	return shared.Invalid(f)
}

// ChangeCursor is a page of a changeset's changes' cursor: the next page
// lists the nodes of ids above After, the last one listed.
type ChangeCursor struct {
	After uuid.UUID `json:"after"`
}

// ChangesetItem is a node a changeset changed in the tree (v0.1 design
// 3.8): where it was before (nil: the changeset created or restored it)
// and after (nil: the changeset deleted it); Restored, a restore from the
// trash; and Revision, a page's revision as the item left it where its
// changeset wrote no version of it: a creation's, a restore's and a
// deletion's (zero for an attachment, or a deletion before M8/P3).
type ChangesetItem struct {
	NodeID   uuid.UUID
	Before   *TreeState
	After    *TreeState
	Restored bool
	Revision int
}

// ChangesetVersion is a page's version a changeset wrote: on Base (nil:
// the changeset created the page), whose content is BaseBytes long, to
// Revision, of content of hash Hash. Rewrite says every write of this
// changeset's version was a system rewrite, not whether its base was one.
type ChangesetVersion struct {
	NodeID    uuid.UUID
	Base      *int
	BaseBytes int
	Rewrite   bool
	Revision  int
	Hash      []byte
}

// VersionRef is a page's version by the revision it wrote.
type VersionRef struct {
	NodeID   uuid.UUID
	Revision int
}

// ContentRef is a page's content by its hash: what a revert parses, as a
// page may come back with one content and be written back with another
// in one revert (M9 design 4.12; M9/P3 design 3.18.3).
type ContentRef struct {
	NodeID uuid.UUID
	Hash   string // hex, as a map key
}

// ChangeKind is what a changeset did to a node, as its changes tell it.
type ChangeKind string

// The kinds of change: a node created, brought back from the trash,
// deleted, moved to another parent (renamed too, perhaps), renamed under
// its parent, put elsewhere among its siblings; or a page's content
// written alone.
const (
	ChangeCreated   ChangeKind = "created"
	ChangeRestored  ChangeKind = "restored"
	ChangeDeleted   ChangeKind = "deleted"
	ChangeMoved     ChangeKind = "moved"
	ChangeRenamed   ChangeKind = "renamed"
	ChangeReordered ChangeKind = "reordered"
	ChangeContent   ChangeKind = "content"
)

// ChangeRow is a node a changeset changed, as its changes list it: the node
// as it is now, alive or not; where its item had it before and after, both
// nil without an item (a content written alone), as an item has one at
// least; whether the item is a restore; and the version it wrote, nil for
// none.
type ChangeRow struct {
	NodeID   uuid.UUID
	NodeKind Kind
	Name     string
	Alive    bool
	Before   *ChangePlace
	After    *ChangePlace
	Restored bool
	Content  *ChangeVersion
}

// ChangePlace is where an item put a node: under ParentID, nil at the
// notebook's root, whose name is ParentName, nil once the purge took the
// parent; named Name at SortOrder.
type ChangePlace struct {
	ParentID   *uuid.UUID
	ParentName *string
	Name       string
	SortOrder  float64
}

// ChangeVersion is a version a changeset wrote of a page: on Base, nil
// when it created the page, to Revision.
type ChangeVersion struct {
	Base     *int
	Revision int
}

// Change is what the changeset did to the node: content without an item;
// with one, restored or created without a before, deleted without an
// after, moved to another parent, renamed, or reordered.
func (r ChangeRow) Change() ChangeKind {
	switch {
	case r.Before == nil && r.After == nil:
		return ChangeContent
	case r.Before == nil && r.Restored:
		return ChangeRestored
	case r.Before == nil:
		return ChangeCreated
	case r.After == nil:
		return ChangeDeleted
	case !SameParent(r.Before.ParentID, r.After.ParentID):
		return ChangeMoved
	case r.Before.Name != r.After.Name:
		return ChangeRenamed
	}
	return ChangeReordered
}
