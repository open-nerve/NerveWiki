package app_test

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
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

// The index the observer keeps is its rebuild, as the whole program's
// TestTheIndexIsItsRebuild checks through serve, but over the fakes, many
// times as fast, and with units of up to three writes, merged by node as
// the page module merges them: the M9 batch's units, which serve does not
// make yet. Titles repeat, differ by case, by "ß" or by length alone; some
// nodes are attachments, created, renamed, moved and deleted as pages are,
// some of a page's title (M7/P3 design 4.3). A unit of a single rename or
// move has the participant follow it first, with the unit's memo, as a
// tree unit does: the observer takes the links it read when it wrote no
// page (M12/P4/S7 design 4.3 item 6), and never when it did; the pages it
// would write are dropped, as such a unit's observer does not take them.
// The participant reads the contents as Markdown, which finds no link in
// the writer's words: each seed makes two units whose participant writes a
// page (writing), a wikilink to a page the unit renames a parent of or
// moves, counted (code review B, N1).
func TestTheIndexOfUnitsOfManyWritesIsItsRebuild(t *testing.T) {
	assets := 0 // the steps after which a link resolves to an attachment
	// The single renames and moves whose observer took the links the
	// participant read, and of those the ones after which a link leads
	// elsewhere: 824 and 73 (742 and 64 before the units writing makes,
	// whose pages the random ones then pick among too); the units whose
	// participant would have written pages: 600, writing's, of which the
	// random ones make none. The bounds are two thirds of them (05-lessons,
	// section 1).
	taken, takenMoved, written := 0, 0, 0 // written: the units whose participant would have written pages
	for seed := range uint64(300) {
		r := newRandomUnits(t, seed)
		isItsRebuild := func(step int, what string) {
			if got, want := linksOf(r.w), linksOf(r.rebuilt(t)); !slices.Equal(got, want) {
				t.Fatalf("seed %d, after step %d (%s), the index is not its rebuild:\nkept    %v\nrebuilt %v", seed, step, what, got, want)
			}
		}
		for step := range 40 {
			// A step that makes a writing unit checks its random one first,
			// so that each is told apart (code review B, round 2, N1).
			what := r.unit(3)
			if what != "" {
				isItsRebuild(step, what)
			}
			if step%20 == 13 {
				what = r.writing(step)
				isItsRebuild(step, what)
			}
			if what == "" {
				continue
			}
			if slices.ContainsFunc(r.w.store.links, func(l app.Link) bool { return l.Resolution.Asset }) {
				assets++
			}
		}
		taken, takenMoved, written = taken+r.taken, takenMoved+r.takenMoved, written+r.participantsWrote
	}
	if assets < 1000 {
		t.Errorf("a link resolved to an attachment after %d steps: too few to tell", assets)
	}
	t.Logf("single renames and moves whose observer took the participant's links: %d, a link led elsewhere after %d; whose "+
		"participant would have written pages: %d", taken, takenMoved, written)
	if taken < 824*2/3 || takenMoved < 73*2/3 || written < 600*2/3 {
		t.Errorf("the observer took the participant's links in %d units, %d of them leading a link elsewhere; the participant "+
			"would have written pages in %d: too few to tell", taken, takenMoved, written)
	}
}

// randomUnits makes random units of a world's pages, their contents in
// texts, as the fake parser reads them; the rewrite, the units'
// participant, over the same world, which a single rename or move has
// follow it first; how many of those the observer took the links of, and
// after how many a link led elsewhere; the units whose participant would
// have written pages.
type randomUnits struct {
	w                 *world
	rnd               *rand.Rand
	md                *markdown.Markdown
	texts             map[uuid.UUID]string
	rewrite           app.Rewrite
	logged            *loggedTakes
	taken, takenMoved int
	participantsWrote int
}

func newRandomUnits(t *testing.T, seed uint64) *randomUnits {
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	r := &randomUnits{w: newWorld(t), rnd: rand.New(rand.NewPCG(seed, 9)), md: md, texts: map[uuid.UUID]string{}, logged: &loggedTakes{}}
	contents := randomContents{r}
	r.rewrite = app.Rewrite{
		Store: r.w.store, Pages: r.w.tree, Contents: contents, Sizes: contents, Locks: &locks{held: map[uuid.UUID]shared.LockHolder{}},
		Parser: markdownadapter.NewParser(md, markdown.NewBudget(1<<20, time.Hour, slog.New(slog.DiscardHandler))),
		Logger: slog.New(slog.DiscardHandler), MaxContent: 1 << 10,
	}
	r.w.index.Logger = slog.New(r.logged)
	return r
}

// randomContents is the pages' texts as the rewrite reads their contents,
// on the revisions the index has.
type randomContents struct{ r *randomUnits }

func (c randomContents) Content(_ context.Context, id uuid.UUID) (string, int, bool, error) {
	text, ok := c.r.texts[id]
	return text, c.r.w.store.revisions[id], ok, nil
}

func (c randomContents) ContentSizes(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	out := map[uuid.UUID]int{}
	for _, id := range ids {
		out[id] = len(c.r.texts[id])
	}
	return out, nil
}

// loggedTakes is a log handler that counts the observer's debug line of the
// links it took.
type loggedTakes struct{ n int }

func (*loggedTakes) Enabled(context.Context, slog.Level) bool { return true }
func (c *loggedTakes) Handle(_ context.Context, r slog.Record) error {
	if r.Message == app.UnitReachMessage {
		c.n++
	}
	return nil
}
func (c *loggedTakes) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *loggedTakes) WithGroup(string) slog.Handler      { return c }

func (r *randomUnits) pick(from ...string) string {
	return from[r.rnd.IntN(len(from))]
}

// content is a random content: links to random targets, and aliases.
func (r *randomUnits) content() string {
	var words []string
	for range r.rnd.IntN(4) {
		words = append(words, r.pick("A", "B", "AB", "note", "dup", "A/dup", "B/note", "A/B/dup", "../dup", "./note",
			"../../A", "/A", "/B/dup", "Note.md", "note.md", "A/Note.md", "nick", "Nick", "Straße", "strasse", "missing",
			"dup/note", "./dup/note", "../B", "x.png", "A/x.png", "X.PNG", "x.png.md", "./x.png", "note.png", "noext",
			"dup.png"))
	}
	for _, a := range []string{"nick", "dup", "B", "Straße", "note.md"} {
		if r.rnd.IntN(4) == 0 {
			words = append(words, "alias:"+a)
		}
	}
	return strings.Join(words, " ")
}

// unit has the observer follow a unit of one to most writes, merged by
// node, and tells them; none when the writes it drew were refused.
func (r *randomUnits) unit(most int) string {
	w := r.w
	var merged []domain.Change
	at := map[uuid.UUID]int{}
	record := func(c domain.Change) {
		i, ok := at[c.NodeID]
		if !ok {
			at[c.NodeID] = len(merged)
			merged = append(merged, c)
			return
		}
		m := &merged[i]
		m.After = c.After
		if c.Revision != 0 {
			m.Revision, m.Facts = c.Revision, c.Facts
		}
	}
	var what, kinds []string
	for range 1 + r.rnd.IntN(most) {
		ids := r.live()
		pages := slices.DeleteFunc(slices.Clone(ids), func(id uuid.UUID) bool { return w.tree.nodes[id].asset })
		pick := func() uuid.UUID { return ids[r.rnd.IntN(len(ids))] }
		parent := func() *uuid.UUID {
			if len(pages) == 0 || r.rnd.IntN(3) == 0 {
				return nil
			}
			p := pages[r.rnd.IntN(len(pages))]
			return &p
		}
		name := r.pick("A", "B", "AB", "note", "Note.md", "dup", "Straße", "STRASSE", "nick", "x.png", "note.png")
		assetName := r.pick("x.png", "X.PNG", "note.png", "dup.png", "noext", "A")
		switch op := r.rnd.IntN(12); {
		case len(ids) < 4 || op < 3:
			p := parent()
			if !r.free(p, name, uuid.Nil()) {
				continue
			}
			id := uuid.NewV7()
			w.tree.nodes[id] = &node{parent: p, name: name}
			record(r.written(id, nil))
			what = append(what, fmt.Sprintf("create %s: %q", name, r.texts[id]))
			kinds = append(kinds, "other")
		case op < 5:
			p := parent()
			if !r.free(p, assetName, uuid.Nil()) {
				continue
			}
			id := uuid.NewV7()
			w.tree.nodes[id] = &node{parent: p, name: assetName, asset: true}
			record(domain.Change{NodeID: id, After: w.place(id)})
			what = append(what, "attach "+assetName)
			kinds = append(kinds, "other")
		case op < 7:
			id := pick()
			n := w.tree.nodes[id]
			if n.asset {
				name = assetName
			}
			// An attachment's name keeps its extension, as the server keeps it
			// (CheckAssetRename).
			keeps := !n.asset || !strings.Contains(n.name, ".") || strings.Contains(name, ".")
			if n.name == name || !keeps || !r.free(n.parent, name, id) {
				continue
			}
			before := w.place(id)
			n.name = name
			record(domain.Change{NodeID: id, Before: before, After: w.place(id)})
			what, kinds = append(what, fmt.Sprintf("rename %s to %s", before.Name, name)), append(kinds, "relocate")
		case op < 9:
			id, p := pick(), parent()
			if p != nil && slices.Contains(r.subtree(id), *p) || !r.free(p, w.tree.nodes[id].name, id) {
				continue
			}
			before := w.place(id)
			w.tree.nodes[id].parent = p
			record(domain.Change{NodeID: id, Before: before, After: w.place(id)})
			for _, d := range r.subtree(id)[1:] {
				record(domain.Change{NodeID: d, Before: w.place(d), After: w.place(d)})
			}
			what, kinds = append(what, "move "+before.Name), append(kinds, "relocate")
		case op < 10:
			id := pick()
			for _, d := range r.subtree(id) {
				record(domain.Change{NodeID: d, Before: w.place(d)})
				w.tree.nodes[d].gone = true
			}
			what = append(what, "delete "+w.tree.nodes[id].name)
			kinds = append(kinds, "other")
		default:
			if len(pages) == 0 {
				continue
			}
			id := pages[r.rnd.IntN(len(pages))]
			record(r.written(id, w.place(id)))
			what = append(what, fmt.Sprintf("write %s: %q", w.tree.nodes[id].name, r.texts[id]))
			kinds = append(kinds, "other")
		}
	}
	if len(merged) == 0 {
		return ""
	}
	if !slices.Equal(kinds, []string{"relocate"}) {
		w.run(merged...)
		return strings.Join(what, "; ")
	}
	r.relocate(merged, what[0])
	return strings.Join(what, "; ")
}

// relocate has the participant, then the observer, follow merged, a
// single rename or move (what), with the unit's memo: the observer takes
// the links the participant read unless it would have written a page.
func (r *randomUnits) relocate(merged []domain.Change, what string) {
	w := r.w
	mem := &memo{}
	u := &appender{}
	ctx := context.Background()
	if err := r.rewrite.Participate(ctx, app.Moved{NotebookID: w.notebook, At: time.Now(), UpdateLinks: true, Tree: true, Changes: merged, Memo: mem}, u); err != nil {
		w.t.Fatalf("the participant of %s: %v", what, err)
	}
	before, logged := linksOf(w), r.logged.n
	w.published.events = nil
	if err := w.index.PagesChanged(ctx, app.PagesChanged{WorkspaceID: uuid.NewV7(), NotebookID: w.notebook, Changes: merged, Memo: mem}); err != nil {
		w.t.Fatal(err)
	}
	if mem.len() != 0 {
		w.t.Fatalf("the memo holds %d after %s", mem.len(), what)
	}
	if len(u.writes) > 0 {
		r.participantsWrote++
		if r.logged.n > logged {
			w.t.Fatalf("the participant of %s would have written %d pages, and the observer took the links it read", what, len(u.writes))
		}
		return
	}
	if r.logged.n > logged {
		r.taken++
		if !slices.Equal(before, linksOf(w)) {
			r.takenMoved++
		}
	}
}

// writing makes, for step, a page nearN at the root, farN under it and
// toN, whose wikilink leads to nearN/farN, then a unit that renames nearN
// or moves farN to the root, after which the link leads nowhere unless
// the participant writes toN again; and tells it.
func (r *randomUnits) writing(step int) string {
	w := r.w
	near := r.page(nil, fmt.Sprintf("near%d", step), "")
	far := r.page(&near, fmt.Sprintf("far%d", step), "")
	r.page(nil, fmt.Sprintf("to%d", step), fmt.Sprintf("[[near%d/far%d]]", step, step))
	id, what := near, fmt.Sprintf("rename near%d", step)
	before := w.place(id)
	if step%40 < 20 {
		w.tree.nodes[near].name = fmt.Sprintf("near%dx", step)
	} else {
		id, what, before = far, fmt.Sprintf("move far%d", step), w.place(far)
		w.tree.nodes[far].parent = nil
	}
	r.relocate([]domain.Change{{NodeID: id, Before: before, After: w.place(id)}}, what)
	return what
}

// page creates the page name under parent with content, its facts the
// world's (randomParser), and has the observer follow it; it answers its
// id.
func (r *randomUnits) page(parent *uuid.UUID, name, content string) uuid.UUID {
	id := uuid.NewV7()
	r.w.tree.nodes[id] = &node{parent: parent, name: name}
	r.texts[id] = content
	r.w.revision++
	f, err := randomParser{r.md}.Facts(context.Background(), content)
	if err != nil {
		r.w.t.Fatal(err)
	}
	r.w.run(domain.Change{NodeID: id, After: r.w.place(id), Revision: r.w.revision, Facts: f})
	return id
}

// randomParser is the random world's parser of contents: words, as the
// fake parser reads them, but a content that starts with a wikilink, which
// Markdown reads (writing's), as the participant reads it.
type randomParser struct{ md *markdown.Markdown }

func (p randomParser) Facts(ctx context.Context, content string) (domain.Facts, error) {
	if !strings.HasPrefix(content, "[[") {
		return parser{}.Facts(ctx, content)
	}
	return markdownadapter.PageFacts(p.md.Parse([]byte(content)).Facts())
}

// written is the change of a random content of the page id, before at
// before (nil: created).
func (r *randomUnits) written(id uuid.UUID, before *domain.Place) domain.Change {
	r.texts[id] = r.content()
	r.w.revision++
	f, _ := parser{}.Facts(context.Background(), r.texts[id])
	return domain.Change{NodeID: id, Before: before, After: r.w.place(id), Revision: r.w.revision, Facts: f}
}

// live is the pages and attachments not deleted, by id.
func (r *randomUnits) live() []uuid.UUID {
	var out []uuid.UUID
	for id, n := range r.w.tree.nodes {
		if !n.gone {
			out = append(out, id)
		}
	}
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}

// free tells whether parent has no node but self with name's key.
func (r *randomUnits) free(parent *uuid.UUID, name string, self uuid.UUID) bool {
	for id, n := range r.w.tree.nodes {
		if id != self && !n.gone && sameParent(n.parent, parent) && shared.TitleKey(n.name) == shared.TitleKey(name) {
			return false
		}
	}
	return true
}

func (r *randomUnits) subtree(id uuid.UUID) []uuid.UUID {
	sub, _ := r.w.tree.Subtree(context.Background(), r.w.notebook, id)
	out := make([]uuid.UUID, len(sub))
	for i, s := range sub {
		out[i] = s.ID
	}
	return out
}

// rebuilt is the world of the index rebuilt from the same tree and texts.
func (r *randomUnits) rebuilt(t *testing.T) *world {
	t.Helper()
	b := &world{t: t, tree: r.w.tree, store: newStore(), published: &publisher{}, notebook: r.w.notebook}
	b.index = app.Index{Store: b.store, Pages: b.tree, Publisher: b.published}
	rebuild, c, _ := newRebuild(b, nil)
	c.texts, rebuild.Parser = r.texts, randomParser{r.md}
	if _, err := rebuild.Notebook(context.Background(), r.w.notebook); err != nil {
		t.Fatal(err)
	}
	return b
}

func sameParent(a, b *uuid.UUID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// linksOf is w's links, a line each, sorted.
func linksOf(w *world) []string {
	out := make([]string, len(w.store.links))
	for i, l := range w.store.links {
		out[i] = fmt.Sprintf("%s@%d %s -> %s %v %v", l.SourceID, l.Start, l.Target, l.Resolution.ID, l.Resolution.Ambiguous, l.Resolution.Asset)
	}
	slices.Sort(out)
	return out
}
