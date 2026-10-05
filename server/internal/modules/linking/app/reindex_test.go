package app_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// contents are the pages' contents, by path, and the clashes Rekey finds.
type contents struct {
	w       *world
	texts   map[uuid.UUID]string
	clashes []app.Clash
	rekeyed bool
}

func (c *contents) PageIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for id, n := range c.w.tree.nodes {
		if !n.gone {
			out = append(out, id)
		}
	}
	return out, nil
}

func (c *contents) Content(_ context.Context, id uuid.UUID) (string, int, error) {
	return c.texts[id], 7, nil
}

func (c *contents) Rekey(context.Context, uuid.UUID) ([]app.Clash, error) {
	c.rekeyed = true
	return c.clashes, nil
}

// notebooks has one notebook, locked once read.
type notebooks struct {
	id, workspace uuid.UUID
	locked        bool
}

func (n *notebooks) WorkspaceOf(_ context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	return n.workspace, id == n.id, nil
}

func (n *notebooks) LockByID(_ context.Context, id uuid.UUID) (bool, error) {
	n.locked = id == n.id
	return n.locked, nil
}

// parser reads a content's facts as its words: each word a wikilink's
// target, a word "alias:x" an alias.
type parser struct{}

func (parser) Facts(_ context.Context, content string) (domain.Facts, error) {
	var f domain.Facts
	var aliases []string
	for i, word := range strings.Fields(content) {
		if alias, ok := strings.CutPrefix(word, "alias:"); ok {
			aliases = append(aliases, alias)
			continue
		}
		f.Links = append(f.Links, domain.Link{Kind: "wikilink", Target: word, Start: 10 * i, End: 10*i + len(word)})
	}
	f.Aliases = domain.AliasesOf(aliases)
	return f, nil
}

// tx runs fn in no transaction: the store is in memory.
type tx struct{}

func (tx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

func newRebuild(w *world, texts map[string]string) (app.Rebuild, *contents, *notebooks) {
	c := &contents{w: w, texts: map[uuid.UUID]string{}}
	for path, text := range texts {
		c.texts[w.id(path)] = text
	}
	nb := &notebooks{id: w.notebook, workspace: uuid.NewV7()}
	return app.Rebuild{Tx: tx{}, Index: w.index, Contents: c, Notebooks: nb, Parser: parser{}}, c, nb
}

// A rebuild drops the notebook's rows, those of a page no longer there
// too, and takes each page's facts anew,
// at its revision, resolving every link, under the notebook's row lock and
// then its lock of the index; it tells its pages, links and those that
// resolve to none, and publishes a links event that lists no page.
func TestARebuildTakesEveryPageAnew(t *testing.T) {
	w := newWorld(t, "A", "A/note", "src", "nick", "gone")
	w.run(w.write("src", []string{"stale"}))
	w.run(w.write("gone", []string{"A"}))
	w.tree.nodes[w.id("gone")].gone = true
	r, c, nb := newRebuild(w, map[string]string{"src": "note A/note missing nick", "A": "alias:nick"})
	w.store.locked, w.published.events = nil, nil
	got, err := r.Notebook(context.Background(), w.notebook)
	if err != nil {
		t.Fatal(err)
	}
	if want := (app.Rebuilt{Pages: 4, Links: 4, Unresolved: 1}); !reflect.DeepEqual(got, want) {
		t.Errorf("rebuilt %+v, want %+v", got, want)
	}
	if !nb.locked || !c.rekeyed || !reflect.DeepEqual(w.store.locked, []uuid.UUID{w.notebook}) {
		t.Errorf("the rebuild locked the row %v, rekeyed %v, locked the index %v", nb.locked, c.rekeyed, w.store.locked)
	}
	w.resolves("src", 0, "A/note", false)
	w.resolves("src", 10, "A/note", false)
	w.resolves("src", 20, "", false)
	w.resolves("src", 30, "nick", false)
	if _, ok := w.store.facts[w.id("gone")]; ok {
		t.Error("the rows of a page no longer there stay")
	}
	if want := []app.LinksChanged{{WorkspaceID: nb.workspace, NotebookID: w.notebook}}; !reflect.DeepEqual(w.published.events, want) {
		t.Errorf("published %+v, want %+v", w.published.events, want)
	}
}

// A rebuild whose siblings' title keys would clash changes nothing and
// tells them; one of no notebook is ErrNoNotebook.
func TestARebuildStopsAtAClashOrNoNotebook(t *testing.T) {
	w := newWorld(t, "src", "B")
	w.run(w.write("src", []string{"B"}))
	r, c, _ := newRebuild(w, map[string]string{"src": "missing"})
	clash := app.Clash{{ID: w.id("B"), Name: "B"}, {ID: w.id("src"), Name: "b"}}
	c.clashes = []app.Clash{clash}
	w.published.events = nil
	got, err := r.Notebook(context.Background(), w.notebook)
	if err != nil || !reflect.DeepEqual(got, app.Rebuilt{Clashes: []app.Clash{clash}}) {
		t.Errorf("a clash's rebuild = %+v, %v", got, err)
	}
	w.resolves("src", 0, "B", false)
	if len(w.published.events) != 0 {
		t.Errorf("a clash's rebuild published %+v", w.published.events)
	}
	if _, err := r.Notebook(context.Background(), uuid.NewV7()); !errors.Is(err, app.ErrNoNotebook) {
		t.Errorf("the rebuild of no notebook = %v, want ErrNoNotebook", err)
	}
}
