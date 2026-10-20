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
	// The background restores' and reverts' (M8/P4 design 3.3). CheckRestore
	// checks a restore of the trash item of the root nodeID, unlocked, and
	// answers its name; CheckRevert a revert of the changeset, unlocked,
	// calling admit right after its items are counted, before anything else
	// is read, its error answered as it is, and answers the changeset's
	// kind: each the page module's codes in its operation's order.
	CheckRestore(ctx context.Context, notebookID, nodeID uuid.UUID) (string, error)
	CheckRevert(ctx context.Context, notebookID, changesetID uuid.UUID, message *string, admit func(context.Context) error) (string, error)
	// PlanRestore and PlanRevert plan a job as it starts, unlocked: a
	// notebook the job's account can no longer see or write in, and the
	// sentinels below. RunBatch writes the plan's steps from to to (not
	// included) in one unit merged into the job's changeset: 503 to try
	// again, ErrBatchTooLarge to halve, the sentinels below, and the unit's
	// refusals.
	PlanRestore(ctx context.Context, spec JobSpec, nodeID uuid.UUID) (JobPlan, error)
	PlanRevert(ctx context.Context, spec JobSpec, changesetID uuid.UUID) (JobPlan, error)
	RunBatch(ctx context.Context, spec JobSpec, plan JobPlan, from, to int) (BatchDone, error)
}

// JobSpec is a background job's units': its notebook and client, the
// changeset its row names, a revert's message.
type JobSpec struct {
	NotebookID uuid.UUID
	Client     domain.Client
	Changeset  uuid.UUID
	Message    *string
}

// JobPlan is a background job's plan, the page module's, which only it
// reads: its steps, each a write no batch splits, in their order, and the
// nodes they write.
type JobPlan interface {
	Steps() []JobStep
	Total() int
}

// JobStep is a step of a background job: the nodes it writes, the
// content's bytes it parses, and whether the next step is in its batch
// too (M9/P3 design 3.22): a revert's segment, steps no batch splits.
type JobStep struct {
	Nodes  int
	Bytes  int64
	Joined bool
}

// BatchDone is what a background job's batch did: the nodes it wrote; a
// restore's pages and attachments it brought back, those it renamed,
// skipped and where its first batch put the item's root; a revert's nodes
// restored, deleted and put back, and pages written back.
type BatchDone struct {
	Nodes, Pages, Assets, Renamed, Skipped int
	Restored, Deleted, PutBack, Written    int
	Placed                                 *domain.Placement
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
	// Reserved tells whether a later node of the import, a sibling, is
	// named with the key: a name numbered because its own is held takes
	// none of those, which keep theirs (nil: none).
	Reserved func(key string) bool
}

// ImportedAsset is an import's attachment: under ParentID (nil: the
// root), named Name, its file File.
type ImportedAsset struct {
	ParentID *uuid.UUID
	Name     string
	File     File
	// Reserved tells whether a later node of the import, a sibling, is
	// named with the key: a name numbered because its own is held takes
	// none of those, which keep theirs (nil: none).
	Reserved func(key string) bool
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
	// ErrNoParent is a parent that is no page of the notebook: deleted
	// since the import read it.
	ErrNoParent = errors.New("transfer: the parent is no page of the notebook")
)

// ErrChangesetReverted is a unit refused before it wrote anything: a revert
// was made of the changeset the job writes in, its row's, as it ran (M8/P3
// design 3.7). The job stops.
var ErrChangesetReverted = errors.New("transfer: the changeset the job writes in was reverted")

// The background jobs' sentinels (M8/P4 design 3.6), each a stop but the
// first, which halves the batch: the rows of history or links of a batch
// past a unit's; a restore's trash item changed since it started, or past
// the time it can be restored in; a revert's conflicts, which the page
// module's problem it wraps names (shared.Error's Conflicts and
// ConflictCount); its changeset trimmed by the purge; a page of its batch
// under an edit lock, which the problem names (Lock).
var (
	ErrBatchTooLarge    = errors.New("transfer: the batch is past a unit's history or links")
	ErrTrashChanged     = errors.New("transfer: the trash item changed")
	ErrRestoreExpired   = errors.New("transfer: the trash item can no longer be restored")
	ErrRevertConflict   = errors.New("transfer: nodes the changeset changed changed since, or cannot go back")
	ErrChangesetTrimmed = errors.New("transfer: the purge trimmed the changeset")
	ErrLocked           = errors.New("transfer: an edit session holds a page of the batch")
)

// Attachments writes an import's attachments' files: bootstrap adapts the
// asset module's Blobs.
type Attachments interface {
	// Put writes the file r brings, named name, as a new blob, its type
	// told by name and its first bytes: ErrTooLarge past maxBytes,
	// domain.ErrStorageFull, or r's error, each leaving no file. The file
	// is the caller's until Attach or Drop, within its job: the
	// attachments' orphan sweep deletes it a day after it was stored, which
	// no job lasts (Codex M7 review R2).
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
