// Package app holds the asset module's use cases and the ports they need.
package app

import (
	"context"
	"errors"
	"io"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// Files is the store of the attachments' files: adapter/files over the
// platform's store.
type Files interface {
	// Create starts the file at key, visible once it commits;
	// domain.ErrStorageFull when the store keeps no room for it.
	Create(ctx context.Context, key string) (FileWriter, error)
	// Open opens the committed file at key, or answers ErrNoFile.
	Open(ctx context.Context, key string) (File, error)
	// Delete removes the file at key; there being none is success.
	Delete(ctx context.Context, key string) error
	// Free tells the free bytes of the store's disk.
	Free(ctx context.Context) (int64, error)
}

// FileWriter writes a file that becomes visible when it commits; a write
// that runs out of room is domain.ErrStorageFull. On a failed Commit the
// file may be at its key all the same: an orphan, which the sweep finds.
type FileWriter interface {
	io.Writer
	Commit() error
	Abort() error
}

// File is a committed file open for reading.
type File interface {
	io.ReadSeekCloser
	Size() int64
	ModTime() time.Time
}

// ErrNoFile is a key with no committed file.
var ErrNoFile = errors.New("asset: no file at the key")

// Sniffer tells what a file's bytes are: adapter/sniff.
type Sniffer interface {
	// Sniff is the type net/http's sniffing finds in a file's first bytes.
	Sniff(head []byte) string
	// Size reads an image's width and height from r, at most a MiB of it;
	// false when it reads none.
	Size(r io.Reader) (width, height int, ok bool)
}

// Rows keeps the attachments' rows: adapter/postgres, in the caller's
// transaction when it has one.
type Rows interface {
	CreateBlob(ctx context.Context, b domain.Blob) error
	// BlobOfNode is the row not deleted of the node, or ErrNoRow.
	BlobOfNode(ctx context.Context, nodeID uuid.UUID) (domain.Blob, error)
	// BlobsOfNodes are the rows not deleted of the nodes, by node; a node
	// without one is not among them.
	BlobsOfNodes(ctx context.Context, nodeIDs []uuid.UUID) (map[uuid.UUID]domain.Blob, error)
}

// Notebooks is what the module reads of notebooks: bootstrap hands it the
// notebook module's.
type Notebooks interface {
	// WorkspaceOf returns the workspace of the notebook not deleted with
	// id, unlocked, and whether there is one.
	WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error)
}

// Nodes is what the module reads of the notebooks' trees: bootstrap
// adapts page's AssetNodes to it.
type Nodes interface {
	// Node is the node id not deleted, of whatever kind; false for none.
	Node(ctx context.Context, id uuid.UUID) (Node, bool, error)
	// Parent reports whether parentID is a page not deleted of notebookID.
	Parent(ctx context.Context, notebookID, parentID uuid.UUID) (bool, error)
	// Assets is the attachments not deleted under parentID (nil: the root)
	// of notebookID, by name key and id, after the cursor's when it is set,
	// at most limit.
	Assets(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID, after *Cursor, limit int) ([]Node, error)
}

// Cursor is where a list of attachments goes on: after the node of this
// name key and id. It is the list cursor's payload.
type Cursor struct {
	NameKey string    `json:"k"`
	ID      uuid.UUID `json:"id"`
}

// ExpiredRows are the rows the purge deletes: adapter/postgres, in the
// purge's transaction.
type ExpiredRows interface {
	// ExpiredBlobs locks up to batch rows deleted before before, the
	// oldest deletions first, skipping the rows another transaction holds.
	ExpiredBlobs(ctx context.Context, before time.Time, batch int) ([]uuid.UUID, error)
	// DeleteBlobs deletes the rows of ids, and tells how many.
	DeleteBlobs(ctx context.Context, ids []uuid.UUID) (int, error)
}

// Deletions delete the attachments' rows with their nodes and notebooks:
// adapter/postgres, in the caller's transaction.
type Deletions interface {
	// DeleteBlobsOfNodes deletes at at the rows not deleted of the nodes.
	DeleteBlobsOfNodes(ctx context.Context, nodeIDs []uuid.UUID, at time.Time) error
	// DeleteBlobsOfNotebooks deletes at at the rows not deleted of the
	// notebooks.
	DeleteBlobsOfNotebooks(ctx context.Context, notebookIDs []uuid.UUID, at time.Time) error
}

// Activity is the attachments' part in a notebook's activity (M3 handoff
// 1): the bytes of its attachments not deleted. Their latest upload is no
// part of it: each is a unit of the tree, whose changeset the pages' part
// already counts as a write at the same time.
type Activity struct {
	Bytes int64
}

// Activities reads the attachments' part in notebooks' activity:
// adapter/postgres, in its caller's read, unlocked. A notebook without an
// attachment is not in its answer.
type Activities interface {
	NotebookActivities(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Activity, error)
}

// StoredFiles is what the orphan sweep needs of the store: adapter/files.
type StoredFiles interface {
	// List calls each with the key of every file of area last modified
	// before before; each's error stops it.
	List(ctx context.Context, area string, before time.Time, each func(key string) error) error
	Delete(ctx context.Context, key string) error
}

// KnownRows tells which blobs have a row: adapter/postgres.
type KnownRows interface {
	// KnownBlobs are the ids among ids with a row, deleted or not.
	KnownBlobs(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error)
}

// Signer signs and checks the addresses of the contents: adapter/mac, with
// the content key, which this layer never holds (v0.1 design 13.1, item
// 25). The time is the caller's, read once for the operation.
type Signer interface {
	// Sign signs the address of node's file blob as of now.
	Sign(now time.Time, node, blob uuid.UUID) Signed
	// Valid reports whether sig signs node's file blob, expiring at e,
	// shown or downloaded, and e is later than now.
	Valid(now time.Time, node, blob uuid.UUID, e int64, download bool, sig string) bool
}

// ErrNoRow is a node without a row not deleted.
var ErrNoRow = errors.New("asset: no row of the node")

// Tree is the page module's writes of the attachments' nodes: bootstrap
// adapts page's TreeWrites to it.
type Tree interface {
	// Check decides as CreateAsset would, unlocked.
	Check(ctx context.Context, n NewNode) error
	// CreateAsset creates the node in a unit that changes the tree; after
	// runs in its transaction once the node is written, and its error rolls
	// the unit back.
	CreateAsset(ctx context.Context, n NewNode, after func(ctx context.Context, n Node) error) (Node, error)
}

// NewNode is an attachment's node to create: under ParentID (nil: the
// notebook's root), named Name, its file Meta, decided on Action from
// Client.
type NewNode struct {
	NotebookID uuid.UUID
	ParentID   *uuid.UUID
	Name       string
	Meta       FileMeta
	Action     shared.Action
	Client     string
}

// FileMeta is what the page module's guards see of a new attachment's
// file.
type FileMeta struct {
	MIME   string
	Bytes  int64
	SHA256 []byte
}

// Node is a node of a notebook's tree.
type Node struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	ParentID   *uuid.UUID
	// Asset is an attachment's node; a page's otherwise.
	Asset     bool
	Name      string
	NameKey   string
	CreatedBy uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}
