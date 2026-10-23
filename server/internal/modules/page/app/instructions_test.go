package app_test

import (
	"context"
	"runtime"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// instructionsOf is the use case on f's fakes, counting a byte one.
func (f *fixture) instructionsOf() *app.Instructions {
	return app.NewInstructions(f.store, f.store, f.md, func(s string) int { return len(s) })
}

// write gives page n the content text at revision.
func (f *fixture) write(n domain.Node, text string, revision int) {
	f.store.contents[n.ID] = app.Content{NodeID: n.ID, Content: text, Revision: revision, ByteSize: len(text)}
}

// instructionsText is what Execute answered, as a test compares it: the
// body, cut or not, and the title; "none" when there are none.
func instructionsText(a domain.Agents, ok bool, err error) string {
	switch {
	case err != nil:
		return "error " + err.Error()
	case !ok:
		return "none"
	}
	if a.Cut {
		return a.Title + " (cut): " + a.Body
	}
	return a.Title + ": " + a.Body
}

// reads are how many contents the use case read since the last call.
func (f *fixture) reads() int {
	n := 0
	for _, c := range f.rec.calls {
		if c == "PageContent" {
			n++
		}
	}
	f.rec.calls = nil
	return n
}

// The instructions follow the notebook's root page AGENTS, by its title
// key, through each way to change it (G16): its content written, its
// title's case changed (the key, the node and the revision kept: the
// cached text, the title now), replaced by another page, renamed out and
// in, deleted, restored from the trash (the node, the revision and the
// content back: the cached text), a version restored (a revision). An
// attachment holding the key, a page AGENTS below the root and a content
// empty once its frontmatter is out are none (M9/P1 design 3.15.1).
func TestTheInstructionsFollowTheAgentsPage(t *testing.T) {
	f := newFixture()
	ins := f.instructionsOf()
	ctx := context.Background()
	step := func(name, want string, reads int) {
		t.Helper()
		if got := instructionsText(ins.Execute(ctx, f.eng)); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
		if n := f.reads(); n != reads {
			t.Errorf("%s: %d contents read, want %d", name, n, reads)
		}
	}
	step("no AGENTS page", "none", 0)
	parent := f.page("Docs", nil, 0)
	f.write(f.page("AGENTS", &parent.ID, 0), "# Below the root", 1)
	step("an AGENTS page below the root", "none", 0)
	agents := f.page("AGENTS", nil, 1)
	f.write(agents, "---\ntags: [rules]\n---\n\n# Rules\nOne.\n", 1)
	step("the page", "AGENTS: # Rules\nOne.", 1)
	step("the page again", "AGENTS: # Rules\nOne.", 0)
	f.write(agents, "# Rules\nTwo.", 2)
	step("its content written", "AGENTS: # Rules\nTwo.", 1)
	agents.Name = "agents"
	f.store.nodes[agents.ID] = agents
	step("its title's case changed", "agents: # Rules\nTwo.", 0)
	agents.Name, agents.NameKey = "Old rules", "old rules"
	f.store.nodes[agents.ID] = agents
	other := f.page("AGENTS", nil, 2)
	f.write(other, "# Others", 1)
	step("replaced", "AGENTS: # Others", 1)
	delete(f.store.nodes, other.ID)
	step("the other deleted", "none", 0)
	agents.Name, agents.NameKey = "AGENTS", "agents"
	f.store.nodes[agents.ID] = agents
	step("renamed back in", "AGENTS: # Rules\nTwo.", 0)
	f.store.nodes[other.ID] = other
	delete(f.store.nodes, agents.ID)
	step("the other restored from the trash", "AGENTS: # Others", 0)
	f.write(other, "# Others, as they were", 2)
	step("a version of the other restored", "AGENTS: # Others, as they were", 1)
	f.write(other, "---\na: 1\n---\n\n", 3)
	step("a content of a frontmatter alone", "none", 1)
	delete(f.store.nodes, other.ID)
	f.asset("agents", nil, 3)
	step("an attachment holding the key", "none", 0)
}

// The text is cached under the revision read with the content: a content
// written between the point lookup and its read is cached as the later
// revision's, never under the lookup's, so that the next lookup, of the
// later revision, finds it; a page deleted between them is none, cached
// for nothing (M9/P1 design 3.17.4, M9 design 4.15). Its store is a fake,
// with no database for 4.15's end checks:
// TestTheInstructionsFollowTheAGENTSPage, through serve, ends with them
// (M9 closeout A-D6).
func TestTheInstructionsCacheByTheRevisionRead(t *testing.T) {
	f := newFixture()
	ins := f.instructionsOf()
	ctx := context.Background()
	agents := f.page("AGENTS", nil, 0)
	f.write(agents, "One.", 1)
	f.rec.before = map[string]func(){"PageContent": func() { f.write(agents, "Two.", 2) }}
	if got := instructionsText(ins.Execute(ctx, f.eng)); got != "AGENTS: Two." || f.reads() != 1 {
		t.Fatalf("a content written between the reads: %q", got)
	}
	if got := instructionsText(ins.Execute(ctx, f.eng)); got != "AGENTS: Two." {
		t.Errorf("the next lookup: %q", got)
	}
	if n := f.reads(); n != 0 {
		t.Errorf("the next lookup, of revision 2, read %d contents: the text read was not cached as revision 2's", n)
	}
	f.write(agents, "Three.", 3)
	f.rec.before = map[string]func(){"PageContent": func() { delete(f.store.contents, agents.ID) }}
	if got := instructionsText(ins.Execute(ctx, f.eng)); got != "none" {
		t.Errorf("a page deleted between the reads: %q", got)
	}
	f.write(agents, "Three.", 3)
	if got := instructionsText(ins.Execute(ctx, f.eng)); got != "AGENTS: Three." || f.reads() != 2 {
		t.Errorf("after the deletion's read: %q", got)
	}
}

// The text is cut at MaxInstructionsBytes as the measure counts it, and
// the cache holds the 256 texts last used: the least recently used goes
// first.
func TestTheInstructionsCacheHoldsTheLastUsed(t *testing.T) {
	f := newFixture()
	ins := f.instructionsOf()
	ctx := context.Background()
	long := f.page("AGENTS", nil, 0)
	f.write(long, strings.Repeat("x", domain.MaxInstructionsBytes)+"\nmore", 1)
	a, ok, err := ins.Execute(ctx, f.eng)
	if err != nil || !ok || !a.Cut || len(a.Body) != domain.MaxInstructionsBytes {
		t.Fatalf("a long page: %d bytes, cut %v, %v, %v", len(a.Body), a.Cut, ok, err)
	}
	var notebooks []uuid.UUID
	for range 256 {
		nb := uuid.NewV7()
		notebooks = append(notebooks, nb)
		n := f.page("AGENTS", nil, 0)
		n.NotebookID = nb
		f.store.nodes[n.ID] = n
		f.write(n, "Rules of "+nb.String(), 1)
		if got := instructionsText(ins.Execute(ctx, nb)); got != "AGENTS: Rules of "+nb.String() {
			t.Fatalf("notebook %s: %q", nb, got)
		}
	}
	f.reads()
	for _, nb := range slices.Backward(notebooks) {
		if _, ok, err := ins.Execute(ctx, nb); !ok || err != nil {
			t.Fatal(ok, err)
		}
	}
	if n := f.reads(); n != 0 {
		t.Errorf("the 256 last used read %d contents again, want none", n)
	}
	if _, ok, err := ins.Execute(ctx, f.eng); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if n := f.reads(); n != 1 {
		t.Errorf("the 257th notebook's read %d contents, want its own: it went first", n)
	}
}

// The cache holds its texts, not the contents they were cut from: an
// AGENTS page of 1 MiB written 256 times, each revision read once, leaves
// 256 entries of at most 32 KiB, 8 MiB in all, once the store holds none of
// the contents; held as parts of their contents, they held 256 MiB (M9/P1
// design 3.15.1, S3 review A-I1).
func TestTheInstructionsCacheHoldsItsTextsAlone(t *testing.T) {
	f := newFixture()
	ins := f.instructionsOf()
	ctx := context.Background()
	agents := f.page("AGENTS", nil, 0)
	line := strings.Repeat("x", 99) + "\n"
	before := heapHeld()
	for revision := 1; revision <= 256; revision++ {
		f.write(agents, strings.Repeat(line, 1<<20/len(line)), revision) // a string of its own
		if a, ok, err := ins.Execute(ctx, f.eng); err != nil || !ok || !a.Cut || len(a.Body) > domain.MaxInstructionsBytes {
			t.Fatalf("revision %d: %d bytes, cut %v, %v, %v", revision, len(a.Body), a.Cut, ok, err)
		}
		f.reads()
	}
	f.write(agents, "", 256) // none of the contents now; revision 256 is cached
	if _, ok, err := ins.Execute(ctx, f.eng); !ok || err != nil || f.reads() != 0 {
		t.Fatalf("revision 256 again: %v, %v, not from the cache", ok, err)
	}
	held := heapHeld() - before
	if bound := int64(256 * domain.MaxInstructionsBytes); held > 2*bound {
		t.Errorf("the cache holds %.1f MiB, its bound %.0f MiB", float64(held)/(1<<20), float64(bound)/(1<<20))
	}
	runtime.KeepAlive(ins)
}

// heapHeld is the bytes the heap holds after a collection.
func heapHeld() int64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.HeapAlloc)
}
