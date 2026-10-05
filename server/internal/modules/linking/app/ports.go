// Package app holds the linking module's use cases, the index's
// maintenance and its rebuild, and the ports they need.
package app

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Page is a page whose facts the index takes: its notebook, and the
// revision of its content they are of.
type Page struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	Revision   int
}

// Dropped is what a page's rows held that the index's maintenance still
// reads once they go: the pages its links resolved to, and its aliases'
// keys, each once.
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

// Pages is what the index reads of a notebook's pages, in the transaction
// ctx carries or on the pool outside one: the page module's, which
// bootstrap wires to it (M6/P3 design 3.3). Attachments and deleted pages
// are never among them.
type Pages interface {
	// ByKeys is the pages of notebookID whose title key is one of keys,
	// each with its path from the root.
	ByKeys(ctx context.Context, notebookID uuid.UUID, keys []string) ([]domain.Node, error)
	// Paths is the pages of notebookID among ids, each with its path from
	// the root.
	Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]domain.Node, error)
	// Subtree is the page id of notebookID and the pages under it.
	Subtree(ctx context.Context, notebookID, id uuid.UUID) ([]domain.Step, error)
}

// LinksChanged is the links event of a unit (M6 design 4.8): the pages
// whose links resolve otherwise, but for those whose content it wrote, and
// the pages whose backlinks changed. Each is empty for none, and nil for
// more than MaxEventPages.
type LinksChanged struct {
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	Pages       []uuid.UUID
	Targets     []uuid.UUID
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
// rewrites links, and what it changed. bootstrap converts the page
// module's step, field by field.
type Moved struct {
	NotebookID  uuid.UUID
	At          time.Time
	UpdateLinks bool
	Changes     []domain.Change
}

// Appender adds a content write to the unit a rewrite follows: the page
// module's, which bootstrap adapts. WriteContent's error is ErrGuardLocked,
// with the guard's own, when the page's edit lock refuses the write.
type Appender interface {
	WriteContent(ctx context.Context, w Rewritten) error
	// Defer has f called once the unit is over, its observers done.
	Defer(f func())
}

// PageContents reads a page's content and its revision, in the
// transaction ctx carries: the page module's, which bootstrap wires.
type PageContents interface {
	Content(ctx context.Context, id uuid.UUID) (string, int, error)
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
// index keeps them and as the page module's write takes them, and the
// release of the parse budget the facts hold.
type Parsed struct {
	Facts   domain.Facts
	Written any
	Release func()
}

// RewriteParser parses the contents a rewrite reads and writes, holding
// the parse budget for each, taken now or not at all: server_busy (M6/P4
// design 4.2). The rewrite holds its notebook's row, which writers waiting
// on the budget would wait behind.
type RewriteParser interface {
	ParseNow(ctx context.Context, content string) (Parsed, error)
}
