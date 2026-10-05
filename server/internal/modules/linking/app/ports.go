// Package app holds the linking module's use cases, the index's
// maintenance and its rebuild, and the ports they need.
package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
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

// Reach is the links a change may resolve anew (M6/P3 design 3.4, step 6):
// those whose target's keys meet Keys, those resolved to one of Targets,
// and those written in one of Sources.
type Reach struct {
	Keys    []string
	Targets []uuid.UUID
	Sources []uuid.UUID
}

// Alias is a page with an alias, by its key.
type Alias struct {
	PageID uuid.UUID
	Key    string
}

// Store is the index's tables (M6/P3 design 3.2), in the transaction ctx
// carries.
type Store interface {
	// Lock takes the notebook's lock of the index until the transaction
	// ends: its maintenance runs one at a time (M6 design 4.5).
	Lock(ctx context.Context, notebookID uuid.UUID) error
	// ReplacePage has the page's rows hold f, its links resolved to none,
	// and returns what its rows held before.
	ReplacePage(ctx context.Context, p Page, f domain.Facts) (Dropped, error)
	// DeletePages deletes the rows of the pages ids, and returns what they
	// held.
	DeletePages(ctx context.Context, ids []uuid.UUID) (Dropped, error)
	// DeleteNotebooks deletes the rows of the notebooks ids.
	DeleteNotebooks(ctx context.Context, ids []uuid.UUID) error
	// Links is the links of notebookID that r reaches.
	Links(ctx context.Context, notebookID uuid.UUID, r Reach) ([]Link, error)
	// Aliases is the pages of notebookID with an alias whose key is one of
	// keys.
	Aliases(ctx context.Context, notebookID uuid.UUID, keys []string) ([]Alias, error)
	// SetResolutions has each of links, by its page and start, resolve as
	// it says.
	SetResolutions(ctx context.Context, links []Link) error
}
