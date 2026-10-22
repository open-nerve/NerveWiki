// Package app holds the linking module's use cases, the index's
// maintenance and its rebuild, and the ports they need.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Page is a page whose facts the index takes: its notebook, and the
// revision of its content they are of. Deleted is a page in the trash,
// whose view resolves its links: the index has none of it (M8/P2 design
// 3.7).
type Page struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	Revision   int
	Deleted    bool
}

// Dropped is what a page's rows held that the index's maintenance still
// reads once they go: the pages and attachments its links resolved to, and
// its aliases' keys, each once.
type Dropped struct {
	Targets   []uuid.UUID
	AliasKeys []string
}

// Link is a link of the index as its resolution reads and writes it: the
// page it is written in, where its target is written, the target, and
// where it resolves to.
type Link struct {
	SourceID   uuid.UUID
	Start      int
	Target     string
	Resolution domain.Resolution
	// Aliases is a link that is a value of its page's aliases, which a
	// rewrite leaves (M6/P4 design 2).
	Aliases bool
}

// Indexed is a page's index as its reading view reads it: the revision of
// the content and the extractor its rows are of, and where each of its
// links resolves to, by where its target starts.
type Indexed struct {
	Revision    int
	Extractor   int
	Resolutions map[int]domain.Resolution
}

// Alias is a page with an alias, by its key.
type Alias struct {
	PageID uuid.UUID
	Key    string
}

// Store is the index's tables (M6/P3 design 3.2), in the transaction ctx
// carries, or on the pool outside one (a reading view's, Views).
type Store interface {
	// Lock takes the notebook's lock of the index until the transaction
	// ends: its maintenance runs one at a time (M6 design 4.5).
	Lock(ctx context.Context, notebookID uuid.UUID) error
	// ReplacePage has the page's rows hold f, its links resolved to none,
	// and returns what its rows held before.
	ReplacePage(ctx context.Context, p Page, f domain.Facts) (Dropped, error)
	// AddPage has the rows of a page that has none hold f, its links
	// resolved to none: a rebuild's, after DeleteNotebooks.
	AddPage(ctx context.Context, p Page, f domain.Facts) error
	// DeletePages deletes the rows of the pages ids, and returns what they
	// held.
	DeletePages(ctx context.Context, ids []uuid.UUID) (Dropped, error)
	// DeleteNotebooks deletes the rows of the notebooks ids.
	DeleteNotebooks(ctx context.Context, ids []uuid.UUID) error
	// Links is the links of notebookID that r reaches.
	Links(ctx context.Context, notebookID uuid.UUID, r domain.Reach) ([]Link, error)
	// Aliases is the pages of notebookID with an alias whose key is one of
	// keys.
	Aliases(ctx context.Context, notebookID uuid.UUID, keys []string) ([]Alias, error)
	// AliasKeys is the keys of the aliases of the pages ids, each once.
	AliasKeys(ctx context.Context, ids []uuid.UUID) ([]string, error)
	// SetResolutions has each of links, by its page and start, resolve as
	// it says.
	SetResolutions(ctx context.Context, links []Link) error
	// IndexedOf is the revision and the extractor of the rows of the pages
	// among ids the index has, by id (M6/P4 design 4.1).
	IndexedOf(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Indexed, error)
	// View is the index of the page id, in one statement; false for a page
	// the index does not have.
	View(ctx context.Context, id uuid.UUID) (Indexed, bool, error)
}

// Reads is what the reads of the index read of its tables (M6/P5), on the
// pool, each in one statement: one snapshot.
type Reads interface {
	// Backlinks is the pages that link to target, but for target itself,
	// whose id is after after, at most size of them, by id; of each, its
	// links there counted up to count, and the ranges of the first
	// contexts of them.
	Backlinks(ctx context.Context, target, after uuid.UUID, size, count, contexts int) ([]Backlink, error)
	// Properties is the properties of the page id; false for a page the
	// index does not have.
	Properties(ctx context.Context, id uuid.UUID) (Properties, bool, error)
	// Tags is the tags of notebookID, by key.
	Tags(ctx context.Context, notebookID uuid.UUID) ([]Tag, error)
	// TagPages is the pages of notebookID with the tag of key or one under
	// it, by id.
	TagPages(ctx context.Context, notebookID uuid.UUID, key string) ([]uuid.UUID, error)
	// NotebookAliases is the aliases of the pages of notebookID, by page,
	// each page's by key.
	NotebookAliases(ctx context.Context, notebookID uuid.UUID) (map[uuid.UUID][]string, error)
	// Aliases is the pages of notebookID with an alias whose key is one of
	// keys, as Store has them.
	Aliases(ctx context.Context, notebookID uuid.UUID, keys []string) ([]Alias, error)
}

// IndexReads are what index reads of the link index (M9/P4 design 3.9.3),
// on the pool, each in one statement.
type IndexReads interface {
	// IndexPages is the pages of notebookID the index holds, each with
	// the start of its type (IndexTypeChars characters); with typeIs, only
	// those whose type is that string. What it reads of a type is bounded
	// as its answer is, whatever the type's length (S2 review B-I1).
	IndexPages(ctx context.Context, notebookID uuid.UUID, typeIs *string) ([]IndexedPage, error)
	// IndexItems is the types, summaries and first tags of the pages ids.
	IndexItems(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]IndexFacts, error)
	// InboundCounts is how many other pages link to each of ids, at most
	// most; a page none links to is not among them.
	InboundCounts(ctx context.Context, ids []uuid.UUID, most int) (map[uuid.UUID]int, error)
	// TagPages is as GetTag reads it.
	TagPages(ctx context.Context, notebookID uuid.UUID, key string) ([]uuid.UUID, error)
}

// IndexedPage is a page the index holds and the start of its type, the
// first property whose key is "type" but for ASCII case when its value is
// a string; Typed false for none.
type IndexedPage struct {
	ID        uuid.UUID
	TypeStart string
	Typed     bool
}

// IndexFacts is what index shows of a page's properties and tags: its type
// and its summary, each the first of its key but for ASCII case when its
// value is a string, nil for none, their first domain.IndexStartChars
// characters, which tell a string MaxIndexString cuts from one whole; its
// first tags by key, one more than domain.MaxIndexTags at most, each its
// first 512 characters.
type IndexFacts struct {
	Type, Summary *string
	Tags          []string
}

// NodePaths is what MCP's backlinks and index read of a notebook's paths,
// on the pool: the page module's, which bootstrap wires (M9/P4 design
// 3.9.1). It decides nothing: the use cases decided the notebook.
type NodePaths interface {
	// Check is the path's 422 at field, before any read, as page's tools
	// check theirs: the segments' check (M9 design 4.8).
	Check(field, path string) error
	// FindPage is the id of the page at path in notebookID: page.not_found
	// in page's tools' words for none, and for an attachment's path, which
	// names no page; the use cases answer it as it is.
	FindPage(ctx context.Context, notebookID uuid.UUID, field, path string) (uuid.UUID, error)
	// Paths is the paths of ids' nodes alive in notebookID, by id, in one
	// statement; a node deleted has none.
	Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error)
	// Pages is Paths of ids' pages, each with when its content last
	// changed; an attachment and a page deleted have none.
	Pages(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]PagePath, error)
	// ChildPages is the pages alive of notebookID whose parent is parent,
	// the root's for uuid.UUID{}, in no order.
	ChildPages(ctx context.Context, notebookID, parent uuid.UUID) ([]uuid.UUID, error)
}

// PagePath is a page's path and its content's last change.
type PagePath struct {
	Path      string
	ChangedAt time.Time
}

// Backlink is a page that links to another as the index has it: the
// revision of the content its rows are of and the extractor they are by,
// zero for no rows; how many of its links lead there, counted up to a
// bound; and where the targets of the first of them start and end, by
// start.
type Backlink struct {
	SourceID  uuid.UUID
	Revision  int
	Extractor int
	Links     int
	Ranges    []domain.Range
}

// Properties is a page's properties as the index has them: whether its
// frontmatter is valid, its properties in the order written, and its
// property links, by where they start.
type Properties struct {
	Valid      bool
	Properties []Property
	Links      []PropertyLink
	// AssetsExpire is when the earliest of the links' addresses expires,
	// zero for none (M7/P4 design 4.3).
	AssetsExpire time.Time
}

// Property is a property of a page: its key and its value's JSON.
type Property struct {
	Key   string
	Value json.RawMessage
}

// PropertyLink is a property link of a page: its property's path and the
// page or the attachment it resolves to, the zero id for none, Asset
// telling an attachment (M7/P3 design 4.6), and an attachment's content's
// address, signed, "" for none (M7/P3 design 5.6), and whether the browser
// shows the content there (M7/P4 design 4.3).
type PropertyLink struct {
	Key    string
	NodeID uuid.UUID
	Asset  bool
	URL    string
	Inline bool
}

// AttachmentAddress is an attachment's content's address, signed, whether
// the browser shows the content there or downloads it, and when the
// address expires.
type AttachmentAddress struct {
	URL     string
	Inline  bool
	Expires time.Time
}

// AttachmentAddresses is the addresses of the attachments' contents: the
// asset module's, which bootstrap wires (M7/P3 design 5.6; M7/P4 design
// 4.3).
type AttachmentAddresses interface {
	// Addresses is the address, signed, of each of ids that is an
	// attachment of notebookID not deleted, by id; one it is not is left
	// out. It decides no access: ids are a page's links', for a reader of
	// the page.
	Addresses(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]AttachmentAddress, error)
}

// Tag is a tag of a notebook as most of its pages write it, and how many
// pages have it.
type Tag struct {
	Tag   string
	Pages int
}

// Pages is what the index reads of a notebook's pages and attachments, the
// nodes a link may lead to, in the transaction ctx carries or on the pool
// outside one: the page module's, which bootstrap wires to it (M6/P3 design
// 3.3; M7/P3 design 4.3). Deleted nodes are never among them.
type Pages interface {
	// ByKeys is the pages and attachments of notebookID whose title key is
	// one of keys, each with its path from the root and its kind.
	ByKeys(ctx context.Context, notebookID uuid.UUID, keys []string) ([]domain.Node, error)
	// Paths is the pages and attachments of notebookID among ids, each with
	// its path from the root and its kind.
	Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]domain.Node, error)
	// Subtree is the node id of notebookID and the pages and attachments
	// under it.
	Subtree(ctx context.Context, notebookID, id uuid.UUID) ([]domain.Step, error)
	// SourcePath is the page id of notebookID, deleted or not, with its
	// path from the root through its ancestors, deleted or not; false for
	// none (M8/P2 design 3.7: the view of a page in the trash).
	SourcePath(ctx context.Context, notebookID, id uuid.UUID) (domain.Node, bool, error)
}

// LinksChanged is the links event of a unit (M6 design 4.8): the pages
// whose links resolve otherwise, but for those whose content it wrote, and
// the pages and attachments whose backlinks changed (M7/P3 design 4.3).
// Each is empty for none, and nil for more than MaxEventPages. Aliases
// tells that the unit changed the aliases of the notebook's pages, a key
// links resolve by: a link the index does not hold, of a version of a page
// (M8 design 4.13), may resolve otherwise now, which no set names (M8
// Codex review R3).
type LinksChanged struct {
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	Pages       []uuid.UUID
	Targets     []uuid.UUID
	Aliases     bool
}

// MaxEventPages is the most pages a links event lists in each of its sets,
// as a pages event does.
const MaxEventPages = 20

// Publisher publishes the links events, in the caller's transaction.
type Publisher interface {
	LinksChanged(ctx context.Context, e LinksChanged) error
}

// Moved is an operation of a page write unit as a rewrite of links follows
// it (M6/P4 design 4.1): its notebook, its unit's time, whether the unit
// rewrites links, and what it changed. bootstrap converts the page module's
// step, field by field. A batch's deferred rewrite has the rest (M9/P3
// design 3.7): each page the batch wrote, alive after it, which is followed
// from its own content and changes; the paths the moved nodes' former
// parents had as the unit began; the nodes it deleted; the most pages the
// rewrite may write, none for 0 (a unit of one operation), a page of Written
// with a link its own changes meet counting WrittenWeight, 1 for 0 (M9/P3
// review B-M3: its rewrite costs more than an index page's; B1-M2: one with
// none counts nothing); the most links it may write, none for 0 (M9/P3
// review B1-M3); the most bytes of the pages it writes, none for 0 (M9/P3
// fix round 5, B2-M1: the facts each keeps of the parse budget follow
// them); and the nodes an op of it renamed or moved with
// update_links false, the links that led to them left as they are, in every
// page, whatever their ancestors' changes (M9 design 4.11; M9/P3 review
// B-M1), and their renames and moves merged, not followed, but which the
// paths before of the nodes under them take their names and former parents
// from (M9/P3 review B1-N1); and, for each node an op of it renamed or
// moved, the nodes it created after the last such op of the node, which the
// subtree spread reads of it leaves out: that op reached none of them, and a
// creation only resolves links again (M9 design 4.11; M9/P3 review B1-M1).
type Moved struct {
	NotebookID    uuid.UUID
	At            time.Time
	UpdateLinks   bool
	Changes       []domain.Change
	Written       map[uuid.UUID]WrittenPage
	Parents       map[uuid.UUID][]domain.Step
	Gone          []uuid.UUID
	MaxPages      int
	WrittenWeight int
	MaxLinks      int
	MaxBytes      int
	Frozen        []uuid.UUID
	FrozenMoves   []domain.Change
	CreatedAfter  map[uuid.UUID][]uuid.UUID
}

// WrittenPage is a page a batch wrote, as its deferred rewrite follows it
// (M9/P3 design 3.7.2): where its links led at its last content op, on the
// plan's tree then; its own changes, the renames and moves after that op,
// merged node by node, of the nodes those ops moved alone; the facts of
// its final content, which its links are taken from, unparsed; the nodes
// those ops rename or move with update_links false, whose links it leaves
// (Moved.Frozen); and, for each node they rename or move, the nodes they
// create after the last that does, out of its subtree
// (Moved.CreatedAfter).
type WrittenPage struct {
	Before       LinksBefore
	Changes      []domain.Change
	Facts        domain.Facts
	Frozen       []uuid.UUID
	CreatedAfter map[uuid.UUID][]uuid.UUID
}

// LinksBefore is where a page's links led at a moment of a batch's plan:
// each link, by where its target starts, and the nodes they led to, at
// their paths from the root then.
type LinksBefore struct {
	Leads   map[int]domain.Resolution
	Targets map[uuid.UUID][]domain.Step
}

// ErrTooManyRewrites is a batch's deferred rewrite past Moved.MaxPages,
// Moved.MaxLinks or Moved.MaxBytes (M9/P3 design 3.7.2; review B1-M3; fix
// round 5, B2-M1): counted before the tree is read, and any page parsed or
// refused for its lock; or one whose next parse would not fit in the
// parse budget beside the shares its own pages' facts keep (fix round 6,
// B3-M1). TooManyRewrites carries the count; bootstrap answers it as the
// page module's page.batch_too_large.
var ErrTooManyRewrites = errors.New("linking: the batch's renames and moves would rewrite more links or pages than it may")

// TooManyRewrites is ErrTooManyRewrites with the pages the rewrite would
// write, at most, as the bound counts them, and the bound; Weight is what
// a page the batch wrote counted, when one was and counted more than one.
// Links, when set, is the links it would write, past Max, Moved.MaxLinks,
// its pages within theirs; Bytes, when set, the bytes of the pages it
// would write, past Max, Moved.MaxBytes, its pages and links within
// theirs. Kept, when set, is the bytes of the parse budget the facts of
// the pages it wrote keep with the take of the parse it needs next, past
// Max, the budget's size: that parse can never be taken beside them,
// whatever others give back (M9/P3 fix round 6, B3-M1).
type TooManyRewrites struct {
	Pages, Max, Weight, Links, Bytes, Kept int
}

func (e *TooManyRewrites) Error() string { return ErrTooManyRewrites.Error() }
func (e *TooManyRewrites) Unwrap() error { return ErrTooManyRewrites }

// Appender adds a content write to the unit a rewrite follows: the page
// module's, which bootstrap adapts. WriteContent's error is ErrGuardLocked,
// with the guard's own, when the page's edit lock refuses the write.
type Appender interface {
	WriteContent(ctx context.Context, w Rewritten) error
	// Defer has f called once the unit is over, its observers done.
	Defer(f func())
}

// PageContents reads a page's content and its revision, in the
// transaction ctx carries or on the pool outside one: the page module's,
// which bootstrap wires. False for a page deleted.
type PageContents interface {
	Content(ctx context.Context, id uuid.UUID) (string, int, bool, error)
}

// PageSizes reads the sizes of pages' contents, their heads, not the
// contents, in the transaction ctx carries: the page module's, which
// bootstrap wires. A batch's deferred rewrite counts the bytes of the
// pages it would write by them before it reads any (M9/P3 fix round 5,
// B2-M1).
type PageSizes interface {
	// ContentSizes is the bytes of the content of each page of ids, alive,
	// by id, in one statement; a page without its content alive is not in
	// it.
	ContentSizes(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int, error)
}

// Rewritten is a page's content a rewrite writes on its revision Base,
// with its facts as the page module's write takes them.
type Rewritten struct {
	PageID  uuid.UUID
	Base    int
	Content string
	Facts   any
}

// ErrGuardLocked is a content write a rewrite adds that the page's edit
// lock refuses: a session the precheck found expired that a heartbeat,
// its clock read earlier, kept alive (M6 design 4.6, item 4).
var ErrGuardLocked = errors.New("linking: the edit lock refuses a page the rewrite writes")

// Locks reads the edit locks of pages, alive at now, one a page, by page
// id: the page module's, which bootstrap wires.
type Locks interface {
	Of(ctx context.Context, ids []uuid.UUID, now time.Time) ([]shared.LockHolder, error)
}

// Parsed is a content's parse a rewrite reads or writes: its facts as the
// index keeps them and as the page module's write takes them, the bytes of
// the parse budget the facts hold (Kept), and the release of them.
type Parsed struct {
	Facts   domain.Facts
	Written any
	Kept    int
	Release func()
}

// RewriteParser parses the contents a rewrite reads and writes, holding
// the parse budget for each, taken now or not at all: server_busy (M6/P4
// design 4.2). The rewrite holds its notebook's row, which writers waiting
// on the budget would wait behind.
type RewriteParser interface {
	ParseNow(ctx context.Context, content string) (Parsed, error)
	// Takes is the bytes ParseNow takes of the budget for a content of n
	// bytes, as it parses, and the budget's size (M9/P3 fix round 6,
	// B3-M1).
	Takes(n int) (take, size int)
}
