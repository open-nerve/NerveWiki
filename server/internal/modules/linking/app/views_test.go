package app_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// view is where the reading view of the page at path, its content at
// revision with links to targets at 0, 10…, finds them lead.
func (w *world) view(path string, revision int, targets ...string) map[int]domain.Resolution {
	w.t.Helper()
	links := make([]app.Link, len(targets))
	for i, target := range targets {
		links[i] = app.Link{SourceID: w.id(path), Start: 10 * i, Target: target}
	}
	views := app.Views{Store: w.store, Pages: w.tree}
	got, err := views.Resolve(context.Background(), app.Page{ID: w.id(path), NotebookID: w.notebook, Revision: revision}, links)
	if err != nil {
		w.t.Fatal(err)
	}
	return got
}

// leads is where links lead, by their starts, 0, 10…: to the pages at
// paths, "" for none.
func (w *world) leads(paths ...string) map[int]domain.Resolution {
	out := map[int]domain.Resolution{}
	for i, p := range paths {
		if p != "" {
			out[10*i] = domain.Resolution{ID: w.id(p)}
		} else {
			out[10*i] = domain.Resolution{}
		}
	}
	return out
}

// misindex has the index say the link of the page at path at start leads
// to the page at to, which a view that reads it shows.
func (w *world) misindex(path string, start int, to string) {
	w.t.Helper()
	l := app.Link{SourceID: w.id(path), Start: start, Resolution: domain.Resolution{ID: w.id(to)}}
	if err := w.store.SetResolutions(context.Background(), []app.Link{l}); err != nil {
		w.t.Fatal(err)
	}
}

// A view of a page without links reads nothing.
func TestAViewWithoutLinksReadsNothing(t *testing.T) {
	w := newWorld(t, "src")
	if got := w.view("src", 1); len(got) != 0 || w.store.viewed != 0 || w.tree.reads != 0 {
		t.Errorf("got %v, viewed %d, read %d", got, w.store.viewed, w.tree.reads)
	}
}

// A view of the revision the index has, of this extractor, is the index's,
// in its one read.
func TestAViewOfTheIndexsRevisionIsTheIndexs(t *testing.T) {
	w := newWorld(t, "A", "B", "src")
	c := w.write("src", []string{"A", "Missing"})
	w.run(c)
	w.misindex("src", 0, "B")
	w.tree.reads = 0
	if got, want := w.view("src", c.Revision, "A", "Missing"), w.leads("B", ""); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if w.store.viewed != 1 || w.tree.reads != 0 {
		t.Errorf("viewed %d, read the pages %d times", w.store.viewed, w.tree.reads)
	}
}

// A view of another revision, of an index of another extractor, or of a
// page the index does not have, resolves anew.
func TestAViewTheIndexIsNotOfResolvesAnew(t *testing.T) {
	w := newWorld(t, "A", "B", "src", "new")
	c := w.write("src", []string{"A", "Missing"})
	w.run(c)
	w.misindex("src", 0, "B")
	if got, want := w.view("src", c.Revision+1, "A", "Missing"), w.leads("A", ""); !reflect.DeepEqual(got, want) {
		t.Errorf("another revision: got %v, want %v", got, want)
	}
	w.store.extractor = domain.Extractor + 1
	if got, want := w.view("src", c.Revision, "A", "Missing"), w.leads("A", ""); !reflect.DeepEqual(got, want) {
		t.Errorf("another extractor: got %v, want %v", got, want)
	}
	if got, want := w.view("new", 1, "B"), w.leads("B"); !reflect.DeepEqual(got, want) {
		t.Errorf("not indexed: got %v, want %v", got, want)
	}
}

// A view's link the index does not have at its start resolves anew, the
// others as the index has them.
func TestAViewsLinkTheIndexHasNotResolvesAnew(t *testing.T) {
	w := newWorld(t, "A", "B", "src")
	c := w.write("src", []string{"A"})
	w.run(c)
	w.misindex("src", 0, "B")
	if got, want := w.view("src", c.Revision, "A", "A"), w.leads("B", "A"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// A page gone as its view resolves anew leads nowhere, and an alias of a
// page gone is none: no error, the view is read again with the event.
func TestAViewOfWhatWentMeanwhileIsNoError(t *testing.T) {
	w := newWorld(t, "X", "Y", "src")
	w.run(w.write("X", nil, "nick"))
	w.run(w.write("Y", nil, "nick"))
	w.run(w.write("src", []string{"nick", "X"}))
	w.tree.nodes[w.id("X")].gone = true
	if got, want := w.view("src", 0, "nick", "X"), w.leads("Y", ""); !reflect.DeepEqual(got, want) {
		t.Errorf("an alias's page gone: got %v, want %v", got, want)
	}
	w.tree.nodes[w.id("src")].gone = true
	if got, want := w.view("src", 0, "nick", "X"), w.leads("", ""); !reflect.DeepEqual(got, want) {
		t.Errorf("the page gone: got %v, want %v", got, want)
	}
}

// failing is the index's tables and the notebook's tree, and an error from
// one of the reads of a view.
type failing struct {
	*store
	*tree
	viewErr, keysErr, aliasesErr, pathsErr error
}

func (f failing) View(ctx context.Context, id uuid.UUID) (app.Indexed, bool, error) {
	if f.viewErr != nil {
		return app.Indexed{}, false, f.viewErr
	}
	return f.store.View(ctx, id)
}

func (f failing) ByKeys(ctx context.Context, notebookID uuid.UUID, keys []string) ([]domain.Node, error) {
	if f.keysErr != nil {
		return nil, f.keysErr
	}
	return f.tree.ByKeys(ctx, notebookID, keys)
}

func (f failing) Aliases(ctx context.Context, notebookID uuid.UUID, keys []string) ([]app.Alias, error) {
	if f.aliasesErr != nil {
		return nil, f.aliasesErr
	}
	return f.store.Aliases(ctx, notebookID, keys)
}

func (f failing) Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]domain.Node, error) {
	if f.pathsErr != nil {
		return nil, f.pathsErr
	}
	return f.tree.Paths(ctx, notebookID, ids)
}

// endsLater is a context that ends after its first look: as one that ends
// while its links resolve.
type endsLater struct {
	context.Context
	looks int
}

func (c *endsLater) Err() error {
	if c.looks++; c.looks > 1 {
		return context.Canceled
	}
	return nil
}

// A view's read that fails is its error, and so is its context ended,
// before or while its links resolve.
func TestAViewsFailureIsItsError(t *testing.T) {
	w := newWorld(t, "A", "src")
	down := errors.New("down")
	links := []app.Link{{SourceID: w.id("src"), Target: "A"}}
	page := app.Page{ID: w.id("src"), NotebookID: w.notebook, Revision: 1}
	for _, f := range []failing{
		{store: w.store, tree: w.tree, viewErr: down},
		{store: w.store, tree: w.tree, keysErr: down},
		{store: w.store, tree: w.tree, aliasesErr: down},
		{store: w.store, tree: w.tree, pathsErr: down},
	} {
		if _, err := (app.Views{Store: f, Pages: f}).Resolve(context.Background(), page, links); !errors.Is(err, down) {
			t.Errorf("Resolve = %v, want %v", err, down)
		}
	}
	views := app.Views{Store: w.store, Pages: w.tree}
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := views.Resolve(ended, page, links); !errors.Is(err, context.Canceled) {
		t.Errorf("Resolve with its context ended = %v", err)
	}
	many := make([]app.Link, app.CheckEvery+1)
	for i := range many {
		many[i] = app.Link{SourceID: w.id("src"), Start: i, Target: "A"}
	}
	if _, err := views.Resolve(&endsLater{Context: context.Background()}, page, many); !errors.Is(err, context.Canceled) {
		t.Errorf("Resolve with its context ended among its links = %v", err)
	}
}

// Links of which no target is a path resolve to none without a read.
func TestAViewOfNoPathReadsNoPages(t *testing.T) {
	w := newWorld(t, "src")
	if got, want := w.view("src", 0, "a//", "./"), w.leads("", ""); !reflect.DeepEqual(got, want) || w.tree.reads != 0 {
		t.Errorf("got %v, read the pages %d times; want %v", got, w.tree.reads, want)
	}
}

// The links of a page to one name written in other forms, relative,
// rooted, up the tree or not, each resolve as written: a view resolves a
// target once by its whole parse (P3B fix check). The name alone is the
// page of it in the source folder's subtree.
func TestAViewsLinksToOneNameResolveAsWritten(t *testing.T) {
	w := newWorld(t, "a", "a/x", "a/b", "a/b/src", "a/b/c", "a/b/c/x")
	got := w.view("a/b/src", 0, "x", "./x", "/x", "../x", "../../x")
	if want := w.leads("a/b/c/x", "", "", "a/x", ""); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
