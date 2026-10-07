package app

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Tx runs a function in a transaction: nested, in the caller's.
type Tx interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Contents is what a rebuild reads and repairs of a notebook's pages: the
// page module's, which bootstrap wires to it (M6/P3 design 3.3).
type Contents interface {
	// PageIDs is the pages not deleted of notebookID, by id.
	PageIDs(ctx context.Context, notebookID uuid.UUID) ([]uuid.UUID, error)
	// Content is the content of the page id and its revision; false for
	// no such page.
	Content(ctx context.Context, id uuid.UUID) (string, int, bool, error)
	// Rekey takes the title keys of notebookID's nodes anew from their
	// names; when siblings would share one, it changes none and returns
	// them.
	Rekey(ctx context.Context, notebookID uuid.UUID) ([]Clash, error)
}

// Clash is siblings whose names would share a title key, by their ids.
type Clash []uuid.UUID

// Notebooks is what a rebuild reads and locks of the notebooks: the
// notebook module's.
type Notebooks interface {
	// WorkspaceOf is the workspace of the notebook not deleted id, and
	// whether there is one.
	WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error)
	// LockByID locks the notebook not deleted id FOR NO KEY UPDATE until
	// the transaction ends, and tells whether there is one.
	LockByID(ctx context.Context, id uuid.UUID) (bool, error)
}

// Parser takes a content's facts as the index keeps them, within the
// parse budget.
type Parser interface {
	Facts(ctx context.Context, content string) (domain.Facts, error)
}

// ErrNoNotebook is a rebuild of a notebook that is not there, or is
// deleted.
var ErrNoNotebook = errors.New("linking: no such notebook")

// Rebuilt is a notebook's rebuild: its pages, their links and those that
// resolve to no page; or, when it rebuilt nothing, the siblings whose title
// keys would clash.
type Rebuilt struct {
	Pages      int
	Links      int
	Unresolved int
	Clashes    []Clash
}

// Rebuild rebuilds a notebook's index from its pages (nervewiki reindex,
// M6/P3 design 3.6).
type Rebuild struct {
	Tx        Tx
	Index     Index
	Contents  Contents
	Notebooks Notebooks
	Parser    Parser
}

// Notebook rebuilds the index of notebookID in one transaction, which
// holds the notebook's row FOR NO KEY UPDATE, as a write of its tree does,
// then its lock of the index: the notebook's writes wait. It takes the
// title keys anew first; when siblings would clash it changes nothing and
// returns them. Then it drops the notebook's rows, takes each page's facts,
// resolves every link, and publishes a links event that lists no page:
// any may have changed.
func (r Rebuild) Notebook(ctx context.Context, notebookID uuid.UUID) (Rebuilt, error) {
	var out Rebuilt
	err := r.Tx.WithinTx(ctx, func(ctx context.Context) error {
		workspaceID, ok, err := r.Notebooks.WorkspaceOf(ctx, notebookID)
		if err == nil && ok {
			ok, err = r.Notebooks.LockByID(ctx, notebookID)
		}
		if err != nil {
			return err
		}
		if !ok {
			return ErrNoNotebook
		}
		store := r.Index.Store
		if err := store.Lock(ctx, notebookID); err != nil {
			return err
		}
		clashes, err := r.Contents.Rekey(ctx, notebookID)
		if err != nil || len(clashes) > 0 {
			out.Clashes = clashes
			return err
		}
		if err := store.DeleteNotebooks(ctx, []uuid.UUID{notebookID}); err != nil {
			return err
		}
		ids, err := r.Contents.PageIDs(ctx, notebookID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := r.page(ctx, notebookID, id); err != nil {
				return err
			}
		}
		links, err := store.Links(ctx, notebookID, domain.Reach{Sources: ids})
		if err != nil {
			return err
		}
		resolved, err := r.Index.resolve(ctx, notebookID, links)
		if err != nil {
			return err
		}
		if err := store.SetResolutions(ctx, resolved); err != nil {
			return err
		}
		out = Rebuilt{Pages: len(ids), Links: len(links), Unresolved: len(links) - len(resolved)}
		return r.Index.Publisher.LinksChanged(ctx, LinksChanged{WorkspaceID: workspaceID, NotebookID: notebookID})
	})
	return out, err
}

// page has the rows of the page id, which has none, hold the facts of its
// content, its links resolved to none.
func (r Rebuild) page(ctx context.Context, notebookID, id uuid.UUID) error {
	content, revision, ok, err := r.Contents.Content(ctx, id)
	switch {
	case err != nil:
		return err
	case !ok:
		// The notebook's row, which the rebuild holds, keeps its pages.
		return fmt.Errorf("the content of page %s, which the rebuild read, is gone", id)
	}
	facts, err := r.Parser.Facts(ctx, content)
	if err != nil {
		return err
	}
	return r.Index.Store.AddPage(ctx, Page{ID: id, NotebookID: notebookID, Revision: revision}, facts)
}
