package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// rewriting is a world whose pages have contents, parsed as the server
// parses them, and the rewrite over it, with its locks, budget and log.
type rewriting struct {
	*world
	md       *markdown.Markdown
	budget   *markdown.Budget
	contents pageContents
	locks    *locks
	logs     *bytes.Buffer
	rewrite  app.Rewrite
}

func newRewriting(t *testing.T, pages ...string) *rewriting {
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	w := &rewriting{
		world: newWorld(t, pages...), md: md, contents: pageContents{}, locks: &locks{held: map[uuid.UUID]shared.LockHolder{}},
		logs:   &bytes.Buffer{},
		budget: markdown.NewBudget(1<<20, time.Hour, slog.New(slog.DiscardHandler)),
	}
	w.rewrite = app.Rewrite{
		Store: w.store, Pages: w.tree, Contents: w.contents, Locks: w.locks,
		Parser: markdownadapter.NewParser(md, w.budget), Logger: slog.New(slog.NewTextHandler(w.logs, nil)),
	}
	return w
}

// write indexes content as the page at path's.
func (w *rewriting) write(path, content string) {
	w.t.Helper()
	id := w.id(path)
	facts, err := markdownadapter.PageFacts(w.md.Parse([]byte(content)).Facts())
	if err != nil {
		w.t.Fatal(err)
	}
	w.revision++
	w.contents[id] = page{content, w.revision}
	w.run(domain.Change{NodeID: id, Before: w.place(id), After: w.place(id), Revision: w.revision, Facts: facts})
}

// follow has the rewrite follow the operation of changes, the unit
// rewriting links, through an appender that answers err.
func (w *rewriting) follow(err error, changes ...domain.Change) (*appender, error) {
	u := &appender{err: err}
	got := w.rewrite.Participate(context.Background(), app.Moved{
		NotebookID: w.notebook, At: time.Now(), UpdateLinks: true, Changes: changes,
	}, u)
	return u, got
}

// written checks the contents u wrote, by path, on the revisions indexed.
func (w *rewriting) written(u *appender, want map[string]string) {
	w.t.Helper()
	got := map[string]string{}
	for _, x := range u.writes {
		if x.Base != w.contents[x.PageID].revision {
			w.t.Errorf("a write on %d, the index's %d", x.Base, w.contents[x.PageID].revision)
		}
		if _, ok := x.Facts.(markdown.Facts); !ok {
			w.t.Errorf("a write's facts are %T, want the platform's", x.Facts)
		}
		for p, id := range w.tree.names {
			if id == x.PageID {
				got[p] = x.Content
			}
		}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		w.t.Errorf("written %q, want %q", got, want)
	}
}

type page struct {
	content  string
	revision int
}

// pageContents are the pages' contents and revisions.
type pageContents map[uuid.UUID]page

func (c pageContents) Content(_ context.Context, id uuid.UUID) (string, int, error) {
	p, ok := c[id]
	if !ok {
		return "", 0, fmt.Errorf("no content of %s", id)
	}
	return p.content, p.revision, nil
}

// locks are the pages' edit locks, and how often they were read; after
// holds those that appear at the second read.
type locks struct {
	held, after map[uuid.UUID]shared.LockHolder
	reads       int
}

func (l *locks) Of(_ context.Context, ids []uuid.UUID, _ time.Time) ([]shared.LockHolder, error) {
	l.reads++
	if l.reads == 2 && l.after != nil {
		l.held = l.after
	}
	var out []shared.LockHolder
	for _, id := range ids {
		if h, ok := l.held[id]; ok {
			out = append(out, h)
		}
	}
	slices.SortFunc(out, func(a, b shared.LockHolder) int { return a.PageID.Compare(b.PageID) })
	return out, nil
}

// appender records the contents written and the functions deferred; its
// writes answer err.
type appender struct {
	writes   []app.Rewritten
	deferred []func()
	err      error
}

func (a *appender) WriteContent(_ context.Context, w app.Rewritten) error {
	a.writes = append(a.writes, w)
	return a.err
}

func (a *appender) Defer(f func()) { a.deferred = append(a.deferred, f) }

// A rename writes again, each page once, the links of the pages that
// would lead elsewhere: by the name or the path of the page renamed, and
// those under it; a link that leads where it did is left, and a page
// without such links not written. Each write's facts hold their budget
// until the unit ends.
func TestARenameWritesAgainTheLinksThatWouldLeadElsewhere(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "A/x/c", "B", "B/y", "src", "other")
	w.write("src", "[[x]] [[A/x]] [t](A/x.md) [[x/c]] [[A/x/c|c]] [[y]]\n")
	w.write("other", "[[y]] [[B/y]]\n")
	u, err := w.follow(nil, w.rename("A/x", "z"))
	if err != nil {
		t.Fatal(err)
	}
	w.written(u, map[string]string{"src": "[[z]] [[z]] [t](z.md) [[c]] [[c|c]] [[y]]\n"})
	if len(u.deferred) != 1 {
		t.Fatalf("deferred %d, want the one write's release", len(u.deferred))
	}
	if hold, err := w.budget.TakeNow(context.Background(), 1<<20); err == nil {
		hold.Release()
		t.Error("the whole budget taken while a write's facts hold their share")
	}
	u.deferred[0]()
	hold, err := w.budget.TakeNow(context.Background(), 1<<20)
	if err != nil {
		t.Fatalf("the whole budget after the release: %v", err)
	}
	hold.Release()
}

// A link that leads to its page after a rename, but tied with the page
// renamed, its id the least, is written again by its page's path.
func TestALinkARenameMakesAmbiguousIsWrittenAgain(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "C", "C/y", "src")
	w.write("src", "[[x]]\n")
	u, err := w.follow(nil, w.rename("C/y", "x"))
	if err != nil {
		t.Fatal(err)
	}
	w.written(u, map[string]string{"src": "[[A/x]]\n"})
}

// A move writes again the moved page's relative links, which lead from
// its new folder, those to it by a path it left, and those another page of
// its name would take; it reads the moved page's path before the move
// from its former parent's.
func TestAMoveWritesAgainItsLinksAndThoseToIt(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "A/y", "B", "B/C", "C", "C/x", "s", "top")
	w.write("A/x", "[[./y]] [t](../top.md) [[y]]\n")
	w.write("s", "[[A/x]] [Old](A/x.md)\n")
	w.write("C/x", "[[x]]\n") // C/x's own, from C
	changes := w.move("A/x", "B/C")
	u, err := w.follow(nil, changes...)
	if err != nil {
		t.Fatal(err)
	}
	w.written(u, map[string]string{
		"B/C/x": "[[y]] [t](../../top.md) [[y]]\n",
		"s":     "[[B/C/x]] [Old](B/C/x.md)\n",
	})
}

// A rename of the case alone writes again the links that name the page by
// its title not as now written, and only those.
func TestARenameOfTheCaseAloneWritesItsNameAgain(t *testing.T) {
	w := newRewriting(t, "Old", "src")
	w.write("src", "[[Old]] [[old]] [[OLD|t]] [t](Old.md)\n")
	u, err := w.follow(nil, w.rename("Old", "old"))
	if err != nil {
		t.Fatal(err)
	}
	w.written(u, map[string]string{"src": "[[old]] [[old]] [[old|t]] [t](old.md)\n"})
}

// A unit that does not rewrite links, and an operation that is no rename
// or move that relocates a page, write nothing and read no lock.
func TestNothingElseIsWrittenAgain(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "src")
	w.write("src", "[[x]]\n")
	x := w.id("A/x")
	for name, changes := range map[string][]domain.Change{
		"a content":           {{NodeID: x, Before: w.place(x), After: w.place(x), Revision: 9}},
		"a creation":          {{NodeID: uuid.NewV7(), After: &domain.Place{Name: "x"}, Revision: 1}},
		"a deletion":          {{NodeID: x, Before: w.place(x)}},
		"a move among others": {{NodeID: x, Before: w.place(x), After: w.place(x)}},
	} {
		if u, err := w.follow(nil, changes...); err != nil || len(u.writes) != 0 {
			t.Errorf("%s: %d writes, %v", name, len(u.writes), err)
		}
	}
	u := &appender{}
	err := w.rewrite.Participate(context.Background(), app.Moved{NotebookID: w.notebook, At: time.Now(), Changes: []domain.Change{w.rename("A/x", "z")}}, u)
	if err != nil || len(u.writes) != 0 || w.locks.reads != 0 {
		t.Errorf("without UpdateLinks: %d writes, %v, locks read %d times", len(u.writes), err, w.locks.reads)
	}
}

// A page to write that is being edited, the caller's own too, refuses
// the whole operation before anything is parsed: linking.pages_locked
// naming each such page, and nothing written.
func TestALockedPageRefusesTheWhole(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "p", "q", "free")
	w.write("p", "[[A/x]]\n")
	w.write("q", "[[A/x]]\n")
	w.write("free", "[[A/x]]\n")
	ann, bob := uuid.NewV7(), uuid.NewV7()
	w.locks.held[w.id("p")] = shared.LockHolder{PageID: w.id("p"), UserID: ann, DisplayName: "Ann"}
	w.locks.held[w.id("q")] = shared.LockHolder{PageID: w.id("q"), UserID: bob, DisplayName: "Bob"}
	all, err := w.budget.TakeNow(context.Background(), 1<<20) // a parse would be busy
	if err != nil {
		t.Fatal(err)
	}
	defer all.Release()
	u, err := w.follow(nil, w.rename("A/x", "z"))
	var refused *shared.Error
	if !errors.As(err, &refused) || refused.Code != "linking.pages_locked" {
		t.Fatalf("the rename: %v, want linking.pages_locked", err)
	}
	want := []shared.LockHolder{w.locks.held[w.id("p")], w.locks.held[w.id("q")]}
	slices.SortFunc(want, func(a, b shared.LockHolder) int { return a.PageID.Compare(b.PageID) })
	if !slices.Equal(refused.Locks, want) || len(u.writes) != 0 {
		t.Errorf("locks %+v, %d writes; want %+v, none", refused.Locks, len(u.writes), want)
	}
}

// A budget not free now is server_busy at once, nothing written.
func TestABudgetNotFreeIsBusy(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "src")
	w.write("src", "[[A/x]]\n")
	all, err := w.budget.TakeNow(context.Background(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer all.Release()
	u, err := w.follow(nil, w.rename("A/x", "z"))
	var busy *shared.Error
	if !errors.As(err, &busy) || busy.Code != "server_busy" || len(u.writes) != 0 {
		t.Errorf("the rename: %v, %d writes; want server_busy, none", err, len(u.writes))
	}
}

// A page whose index is of another extractor, and one whose content is not
// the index's, are logged and left: their links have no "before" to keep.
// The others are written; the left ones read no lock.
func TestAPageWithoutACurrentIndexIsLeft(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "late", "src")
	w.write("late", "[[A/x]]\n")
	w.write("src", "[[A/x]]\n")
	w.contents[w.id("late")] = page{"[[A/x]] more\n", w.contents[w.id("late")].revision + 1}
	u, err := w.follow(nil, w.rename("A/x", "z"))
	if err != nil {
		t.Fatal(err)
	}
	w.written(u, map[string]string{"src": "[[z]]\n"})
	if !strings.Contains(w.logs.String(), "its content is not the index's") {
		t.Errorf("logged %q, want the page left", w.logs.String())
	}
	w.store.extractor = domain.Extractor + 1
	reads := w.locks.reads
	if u, err := w.follow(nil, w.rename("A/z", "y")); err != nil || len(u.writes) != 0 || w.locks.reads != reads {
		t.Errorf("another extractor's index: %d writes, %v, locks read %d more times", len(u.writes), err, w.locks.reads-reads)
	}
	if !strings.Contains(w.logs.String(), "the index of it is not current") {
		t.Errorf("logged %q, want the pages left", w.logs.String())
	}
}

// A write the edit lock refuses, a session a heartbeat kept alive past the
// precheck, has the rewrite read the locks again: linking.pages_locked,
// naming it.
func TestALockTheGuardFindsIsReadAgain(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "src")
	w.write("src", "[[A/x]]\n")
	held := shared.LockHolder{PageID: w.id("src"), UserID: uuid.NewV7(), DisplayName: "Ann"}
	w.locks.after = map[uuid.UUID]shared.LockHolder{held.PageID: held}
	_, err := w.follow(fmt.Errorf("%w: %w", app.ErrGuardLocked, errors.New("page.locked")), w.rename("A/x", "z"))
	var refused *shared.Error
	if !errors.As(err, &refused) || refused.Code != "linking.pages_locked" || !slices.Equal(refused.Locks, []shared.LockHolder{held}) {
		t.Errorf("the rename: %v, want linking.pages_locked naming %+v", err, held)
	}
}
