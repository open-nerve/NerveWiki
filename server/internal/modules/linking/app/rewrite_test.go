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
		MaxContent: 1 << 10,
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

func (c pageContents) Content(_ context.Context, id uuid.UUID) (string, int, bool, error) {
	p, ok := c[id]
	return p.content, p.revision, ok, nil
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

// A page being edited whose links the operation does not write does not
// refuse it (M6/P4 review r3-1): a value of the
// aliases that led to the page renamed; and, a rename of the case alone, a
// link written as now, one by an alias, and one by the title's path not as
// now written, which alone is written again.
func TestAPageBeingEditedThatIsNotWrittenDoesNotRefuse(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "Old", "p", "now", "alias", "path")
	w.write("Old", "---\naliases: [Al]\n---\n")
	w.write("p", "---\naliases: [\"[[A/x]]\"]\n---\n")
	w.write("now", "[[old]]\n")
	w.write("alias", "[[Al]]\n")
	w.write("path", "[[Old]]\n")
	for _, page := range []string{"p", "now", "alias"} {
		w.locks.held[w.id(page)] = shared.LockHolder{PageID: w.id(page), UserID: uuid.NewV7(), DisplayName: "Ann"}
	}
	u, err := w.follow(nil, w.rename("A/x", "z"))
	if err != nil {
		t.Fatalf("the rename: %v, want it to pass", err)
	}
	w.written(u, map[string]string{})
	u, err = w.follow(nil, w.rename("Old", "old"))
	if err != nil {
		t.Fatalf("the rename of the case: %v, want it to pass", err)
	}
	w.written(u, map[string]string{"path": "[[old]]\n"})
}

// A page whose writing would hold more than MaxContent, and one whose
// writing does not read back as its links, are logged and left, the
// operation going on; their parses give their share of the budget back
// (M6/P4 review r2-2, R1-1). One whose writing with the links' texts would
// hold more, but with their targets alone not, is written so (M6/P4 fix
// check c1-3).
func TestAPageThatCannotBeWrittenIsLeft(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "Other", "big", "near", "math", "src")
	w.write("big", "[[A/x]]\n"+strings.Repeat("a", 1<<10-8))
	near := strings.Repeat("a", 1<<10-18)
	w.write("near", "[A/x](A/x.md)\n"+near)
	w.write("math", "costs $5 [[Other]] [[A/x]]\n")
	w.write("src", "[[A/x]]\n")
	u, err := w.follow(nil, w.rename("A/x", "Dollar$"))
	if err != nil {
		t.Fatalf("the rename: %v, want it to pass", err)
	}
	w.written(u, map[string]string{"near": "[A/x](Dollar$.md)\n" + near, "src": "[[Dollar$]]\n"})
	for _, logged := range []string{"it would hold more than a page may", "no writing reads back as its links"} {
		if !strings.Contains(w.logs.String(), logged) {
			t.Errorf("the log %q, want %q", w.logs, logged)
		}
	}
	for _, f := range u.deferred {
		f()
	}
	hold, err := w.budget.TakeNow(context.Background(), 1<<20)
	if err != nil {
		t.Fatalf("the whole budget after the unit: %v", err)
	}
	hold.Release()
}

// A page left says why, at the level its why takes: one whose writing
// with the links' texts does not read back, and whose writing with their
// targets alone would hold more than a page may, at the error a writing
// read back and not kept takes, with the sizes (M6/P4 fix check c4-5,
// c5-3); one whose writings would all hold more, at a warning.
func TestAPageLeftSaysWhy(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "B", "B/q$", "big", "src")
	src := "[A/x](x.md)\n" + strings.Repeat("a", 1022-12)
	w.write("src", src)
	w.write("big", "[[A/x]]\n"+strings.Repeat("a", 1<<10-8))
	u, err := w.follow(nil, w.rename("A/x", "q$"))
	if err != nil || len(u.writes) != 0 {
		t.Fatalf("the rename: %v, %d writes; want it to pass, writing none", err, len(u.writes))
	}
	for _, line := range []string{
		`level=ERROR msg="the links of a page are not rewritten: no writing reads back as its links and aliases" page_id=` +
			w.id("src").String() + " bytes=1022 written=1025",
		`level=WARN msg="the links of a page are not rewritten: it would hold more than a page may" page_id=` +
			w.id("big").String() + " bytes=1024 written=1025",
	} {
		if !strings.Contains(w.logs.String(), line) {
			t.Errorf("the log\n%s\nwant a line with\n%s", w.logs, line)
		}
	}
}

// A page whose aliases a YAML alias repeats from the key a link is written
// in has its body written again alone, its property links left and logged:
// writing them would change the aliases (M6/P4 fix check c5-1). So has one
// whose writings with its property links would hold more than a page may,
// logged with the sizes (c6-2). A link is logged by where it starts, its
// target not: it holds a title (Codex review R2).
func TestAPageWhoseFrontmatterWouldChangeHasItsBodyWritten(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "src", "near")
	front := "---\nx: &x '[[A/x]]'\naliases: *x\n---\n"
	w.write("src", front+"[[A/x]] [t](A/x.md)\n")
	near := "---\nup: '[[A/x]]'\n---\n[[A/x]]\n"
	pad := strings.Repeat("a", 1023-len(near))
	w.write("near", near+pad)
	u, err := w.follow(nil, w.rename("A/x", "zzzz"))
	if err != nil {
		t.Fatal(err)
	}
	w.written(u, map[string]string{
		"src":  front + "[[zzzz]] [t](zzzz.md)\n",
		"near": "---\nup: '[[A/x]]'\n---\n[[zzzz]]\n" + pad,
	})
	for _, line := range []string{
		`level=ERROR msg="a link is not rewritten: no writing of the page with it was kept" page_id=` +
			w.id("src").String() + " start=13\n",
		`level=ERROR msg="a link is not rewritten: no writing of the page with it was kept" page_id=` +
			w.id("near").String() + " start=11 bytes=1023 written=1025\n",
	} {
		if !strings.Contains(w.logs.String(), line) {
			t.Errorf("the log\n%s\nwant a line with\n%s", w.logs, line)
		}
	}
	if strings.Contains(w.logs.String(), "A/x") {
		t.Errorf("the log\n%s\nholds a title", w.logs)
	}
}

// A link a writing of the body alone leaves is logged with the size of the
// last writing with it that was too large, whatever the others did, and
// the body's writings (M6/P4 fix check c7-1, c8-1, c9): the writings do not
// tell why the others were not kept.
func TestALinkLeftIsLoggedWithTheSizeOfAWritingWithIt(t *testing.T) {
	for _, tt := range []struct {
		name, from, to, page string
		pages                []string
		written              string
		start, left, at      int // at: the size logged
	}{
		{
			"the writings with the frontmatter too large, the body's with the texts not read back", "A/x", "qq$qq",
			"---\na: '[[A/x]]'\nb: '[[A/x]]'\nc: '[[A/x]]'\n---\n[x](A/x.md)\n", []string{"A", "A/x"},
			"---\na: '[[A/x]]'\nb: '[[A/x]]'\nc: '[[A/x]]'\n---\n[x](qq$qq.md)\n", 1018, 3, 1026,
		},
		{
			"the body's writing with the texts too large too", "A/x", "qq$qq",
			"---\na: '[[A/x]]'\nb: '[[A/x]]'\nc: '[[A/x]]'\n---\n[x](A/x.md)\n", []string{"A", "A/x"},
			"---\na: '[[A/x]]'\nb: '[[A/x]]'\nc: '[[A/x]]'\n---\n[x](qq$qq.md)\n", 1022, 3, 1030,
		},
		{
			"the writing with them all not read back, with the targets too large", "Deep/Folder/x", "y$",
			"---\nup: '[[x]]'\n---\n[Deep/Folder/x](x.md)\n", []string{"Deep", "Deep/Folder", "Deep/Folder/x", "Other", "Other/y$"},
			"---\nup: '[[x]]'\n---\n[Deep/Folder/x](Deep/Folder/y$.md)\n", 1005, 1, 1031,
		},
		{
			"the writing with them all too large, with the targets not read back for the aliases", "A/x", "zzzz",
			"---\nx: &x '[[A/x]]'\naliases: *x\n---\n[x](A/x.md)\n", []string{"A", "A/x"},
			"---\nx: &x '[[A/x]]'\naliases: *x\n---\n[zzzz](zzzz.md)\n", 1020, 1, 1025,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newRewriting(t, append(tt.pages, "near")...)
			pad := strings.Repeat("a", tt.start-len(tt.page))
			w.write("near", tt.page+pad)
			u, err := w.follow(nil, w.rename(tt.from, tt.to))
			if err != nil {
				t.Fatal(err)
			}
			w.written(u, map[string]string{"near": tt.written + pad})
			logged := `level=ERROR msg="a link is not rewritten: no writing of the page with it was kept" page_id=` + w.id("near").String()
			sizes := fmt.Sprintf(" bytes=%d written=%d", tt.start, tt.at)
			lines := strings.Split(strings.TrimSpace(w.logs.String()), "\n")
			for _, line := range lines {
				if !strings.Contains(line, logged) || !strings.HasSuffix(line, sizes) {
					t.Errorf("logged %q, want %q ending with %q", line, logged, sizes)
				}
			}
			if len(lines) != tt.left {
				t.Errorf("%d lines logged, want one for each of the %d property links", len(lines), tt.left)
			}
		})
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

// A page whose content is gone when the rewrite reads it fails the unit:
// the unit's lock of the notebook keeps its pages, so it is a defect, not
// a page to leave (M6/P5 design 7).
func TestAPageGoneUnderTheRewriteFailsIt(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "src")
	w.write("src", "[[A/x]]\n")
	delete(w.contents, w.id("src"))
	if u, err := w.follow(nil, w.rename("A/x", "z")); err == nil || !strings.Contains(err.Error(), "is gone") || len(u.writes) != 0 {
		t.Errorf("Participate = %v, %d writes; want the page gone", err, len(u.writes))
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

// A lock the guard finds and the locks read again do not, its session
// ended since, is the one linking.pages_locked names, not the guard's own
// page.locked, which the operation does not answer (M6/P4 review r2-4).
func TestALockTheGuardFindsGoneSinceIsTheOneNamed(t *testing.T) {
	w := newRewriting(t, "A", "A/x", "src")
	w.write("src", "[[A/x]]\n")
	held := shared.LockHolder{PageID: w.id("src"), UserID: uuid.NewV7(), DisplayName: "Ann"}
	guard := &shared.Error{Kind: shared.KindConflict, Code: "page.locked", Lock: &held}
	_, err := w.follow(fmt.Errorf("%w: %w", app.ErrGuardLocked, guard), w.rename("A/x", "z"))
	var refused *shared.Error
	if !errors.As(err, &refused) || refused.Code != "linking.pages_locked" || !slices.Equal(refused.Locks, []shared.LockHolder{held}) {
		t.Errorf("the rename: %v, want linking.pages_locked naming %+v", err, held)
	}
}
