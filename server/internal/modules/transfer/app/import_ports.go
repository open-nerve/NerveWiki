package app

import (
	"context"
	"errors"
	"io"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The ports an import writes through (M7/P6 design 3.3, 3.5, 3.6): the
// page module's tree, the asset module's files, and the statistics of
// the tables it fills. The modules do not import each other: bootstrap
// adapts theirs.

// Tree writes an import's nodes: bootstrap adapts the page module's
// TreeWrites.
type Tree interface {
	// CheckContent checks a page's content as its unit would, reading
	// nothing: an error for content past 5 MiB, not UTF-8, or holding
	// NUL.
	CheckContent(content string) error
	// Parse parses a page's content, which CheckContent passed, for its
	// unit, outside it: 503 server_busy when the parse budget does not
	// free up in time. The import releases it once the unit is over.
	Parse(ctx context.Context, content string) (Parsed, error)
	// Import runs do in a unit of the import kind, merged into spec's
	// changeset when it is set, and answers the unit's changeset, none
	// when it wrote nothing. notebook.not_found for a notebook the job
	// can no longer see, forbidden for a reader.
	Import(ctx context.Context, spec ImportSpec, do func(ctx context.Context, u ImportUnit) error) (uuid.UUID, error)
}

// Parsed is a page's content parsed for its unit: how many links it
// holds, and the release of what it keeps of the parse budget.
type Parsed interface {
	Links() int
	Release()
}

// ImportSpec is an import's unit: in NotebookID, deciding on Action, from
// Client, the job's; merged into Changeset when it is set.
type ImportSpec struct {
	NotebookID uuid.UUID
	Action     shared.Action
	Client     domain.Client
	Changeset  uuid.UUID
}

// ImportUnit is an import's unit: each node created last among its
// siblings, its name numbered when a sibling holds it.
type ImportUnit interface {
	// CreatePage creates a page: ErrTooDeep, nothing written, for one
	// deeper than pages go; ErrNoParent.
	CreatePage(ctx context.Context, p ImportedPage) (CreatedNode, error)
	// CreateAsset creates an attachment; after runs in the unit's
	// transaction once the node is written. ErrNoParent.
	CreateAsset(ctx context.Context, a ImportedAsset, after func(ctx context.Context, n CreatedNode) error) (CreatedNode, error)
}

// ImportedPage is an import's page: under ParentID (nil: the root), named
// Name, holding Content, which Parsed parsed.
type ImportedPage struct {
	ParentID *uuid.UUID
	Name     string
	Content  string
	Parsed   Parsed
}

// ImportedAsset is an import's attachment: under ParentID (nil: the
// root), named Name, its file File.
type ImportedAsset struct {
	ParentID *uuid.UUID
	Name     string
	File     File
}

// CreatedNode is a node a unit created: its id, its name as created, and
// when.
type CreatedNode struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
}

// The refusals of a unit's node that leave the unit going.
var (
	// ErrTooDeep is a page deeper than pages go: nothing of it written.
	ErrTooDeep = errors.New("transfer: the page is deeper than pages go")
	// ErrNoParent is a parent that is no page of the notebook: deleted,
	// or moved away, since the import read it.
	ErrNoParent = errors.New("transfer: the parent is no page of the notebook")
)

// Attachments writes an import's attachments' files: bootstrap adapts the
// asset module's Blobs.
type Attachments interface {
	// Put writes the file r brings, named name, as a new blob, its type
	// told by name and its first bytes: ErrTooLarge past maxBytes,
	// domain.ErrStorageFull, or r's error, each leaving no file.
	Put(ctx context.Context, name string, r io.Reader, maxBytes int64) (File, error)
	// Attach writes f's row, owned by o, in the unit ctx carries.
	Attach(ctx context.Context, f File, o Owner) error
	// Drop deletes f's file, which no row holds.
	Drop(ctx context.Context, f File) error
}

// File is an attachment's file Put wrote: its blob's id, its type, its
// size and SHA-256, an image's size in pixels.
type File struct {
	ID            uuid.UUID
	MIME          string
	Bytes         int64
	SHA256        []byte
	Width, Height int
}

// Owner is what Attach gives a file's row: the attachment's node, its
// notebook, who created it and when.
type Owner struct {
	NodeID, NotebookID, CreatedBy uuid.UUID
	CreatedAt                     time.Time
}

// ErrTooLarge is a file larger than its largest: an upload's archive past
// transfer.import_max_bytes, an attachment past asset.max_bytes.
var ErrTooLarge = errors.New("transfer: the file is larger than its largest")

// Analyzer refreshes the planner's statistics of a module's tables an
// import fills: the page, linking and asset modules'.
type Analyzer interface {
	Analyze(ctx context.Context) error
}
