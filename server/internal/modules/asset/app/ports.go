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
