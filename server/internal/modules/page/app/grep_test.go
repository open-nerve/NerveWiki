package app_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fakeGrep is the search's rows and queue as grep reads them over the
// fixture's store: rows built from the contents by SearchTokens, as the
// job builds them; the pages queued exact or in bulk, which have no row
// yet. between, when set, runs once after the first of SearchBuilding and
// GrepCandidates: the job, building rows, between the call's two
// statements. grown are contents read back other than the states said.
// fresh reads each content as a string of its own, as PostgreSQL's driver
// does, so that what a call keeps of it is its own.
type fakeGrep struct {
	*fakeStore
	rows        map[uuid.UUID]grepRow
	exact, bulk map[uuid.UUID]bool
	between     func()
	grown       map[uuid.UUID]string
	fresh       bool
}

// grepRow is a page's row: its body's lexemes and whether it is partial.
type grepRow struct {
	body    []string
	partial bool
}

func newFakeGrep(s *fakeStore) *fakeGrep {
	return &fakeGrep{fakeStore: s, rows: map[uuid.UUID]grepRow{}, exact: map[uuid.UUID]bool{}, bulk: map[uuid.UUID]bool{},
		grown: map[uuid.UUID]string{}}
}

// build is the job: the rows of ids from their contents, off the queue.
func (f *fakeGrep) build(ids ...uuid.UUID) {
	for _, id := range ids {
		content := f.contents[id].Content
		_, body, partial := domain.SearchTokens(f.nodes[id].Name, &content)
		f.rows[id] = grepRow{body: body, partial: partial}
		delete(f.exact, id)
		delete(f.bulk, id)
	}
}

// buildAll builds every page's row.
func (f *fakeGrep) buildAll() {
	for id, n := range f.nodes {
		if n.Kind == domain.KindPage {
			f.build(id)
		}
	}
}

func (f *fakeGrep) SearchBuilding(ctx context.Context, notebooks []uuid.UUID) (bool, error) {
	f.record(ctx, "SearchBuilding")
	building := false
	for id := range f.bulk {
		building = building || slices.Contains(notebooks, f.nodes[id].NotebookID)
	}
	f.afterFirst()
	return building, nil
}

// afterFirst runs between once.
func (f *fakeGrep) afterFirst() {
	if f.between != nil {
		f.between()
		f.between = nil
	}
}

func (f *fakeGrep) GrepCandidates(ctx context.Context, notebookID uuid.UUID, keys []string) ([]uuid.UUID, error) {
	f.record(ctx, "GrepCandidates")
	var out []uuid.UUID
	for id, row := range f.rows {
		n, ok := f.nodes[id]
		if !ok || n.NotebookID != notebookID {
			continue
		}
		if row.partial || !slices.ContainsFunc(keys, func(k string) bool { return !slices.Contains(row.body, k) }) {
			out = append(out, id)
		}
	}
	for id := range f.exact {
		if f.nodes[id].NotebookID == notebookID {
			out = append(out, id)
		}
	}
	f.afterFirst()
	return out, nil
}

func (f *fakeGrep) GrepContents(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	f.record(ctx, fmt.Sprintf("GrepContents %d", len(ids)))
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if c, ok := f.grown[id]; ok {
			out[id] = c
		} else if c, ok := f.contents[id]; ok {
			out[id] = c.Content
		}
		if c, ok := out[id]; ok && f.fresh {
			out[id] = strings.Clone(c)
		}
	}
	return out, nil
}

// grepper is grep over the fixture and reads, its parses' queue and slots
// production's but smaller, set changing its deps.
func (f *fixture) grepper(reads *fakeGrep, set func(d *app.GrepDeps)) *app.Grep {
	d := app.GrepDeps{Notebooks: f.notebooks, Nodes: f.store, States: f.store, Reads: reads, Auth: f.auth,
		Parses: app.NewGrepParses(4), Slots: app.NewGrepSlots(2), Logger: f.logger(), Clock: f.clock}
	if set != nil {
		set(&d)
	}
	return app.NewGrep(d)
}

// grepFound is a page of grep as "path:line" for each hit and "path:line
// partial" for each line matched only in part.
func grepFound(g app.Grepped) []string {
	var out []string
	for _, found := range g.Found {
		s := fmt.Sprintf("%s:%d", found.Path, found.Line)
		if found.Hit == nil {
			s += " partial"
		}
		out = append(out, s)
	}
	return out
}

// grepAll is every page of grep r from its cursor on, until one has none;
// fails past 100 pages.
func grepAll(t *testing.T, ctx context.Context, g *app.Grep, notebook uuid.UUID, r app.GrepRequest) ([]string, []app.Grepped) {
	t.Helper()
	var all []string
	var pages []app.Grepped
	for range 100 {
		got, err := g.Execute(ctx, notebook, r)
		if err != nil {
			t.Fatalf("grep %q: %v", r.Pattern, err)
		}
		all, pages = append(all, grepFound(got)...), append(pages, got)
		if got.Next == "" {
			return all, pages
		}
		next := got.Next
		r.Cursor = &next
	}
	t.Fatalf("grep %q took more than 100 pages", r.Pattern)
	return nil, nil
}

func strp(s string) *string { return &s }

// TestGrep is the use case on a fake store (M9/P4 design 3.7): the codes
// in their order, the cursor first; the candidates by the keys, and
// building only with them, told when the queue empties between the two
// statements (mutants 164, 166); the glob before the walk's bounds
// (mutant 153); resuming from a line (mutant 165), and from a page renamed
// or deleted since; the batches by the states' sizes, one read each, the
// read's bound judged before a batch (mutant 167), on the bytes read; the
// limit, which is no incomplete (mutant 163); the 501st page and the
// glob's work.
func TestGrep(t *testing.T) {
	t.Run("the codes in order", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionRead)
		reads := newFakeGrep(f.store)
		slots := app.NewGrepSlotsWaiting(1, time.Millisecond)
		release, err := slots.Take(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		g := f.grepper(reads, func(d *app.GrepDeps) { d.Slots = slots })
		unseen, zero := uuid.NewV7(), 0
		for _, row := range []struct {
			name     string
			notebook uuid.UUID
			r        app.GrepRequest
			want     string
		}{
			{"the cursor", unseen, app.GrepRequest{Pattern: "(", Glob: strp("a/[b"), Limit: &zero, Cursor: strp("no")}, "bad_request"},
			{"the pattern", unseen, app.GrepRequest{Pattern: "(", Glob: strp("a/[b"), Limit: &zero}, "validation_failed pattern"},
			{"the glob", unseen, app.GrepRequest{Pattern: "a", Glob: strp("a/[b"), Limit: &zero}, "validation_failed glob"},
			{"the limit", unseen, app.GrepRequest{Pattern: "a", Limit: &zero}, "validation_failed limit"},
			{"the notebook", unseen, app.GrepRequest{Pattern: "a"}, "notebook.not_found"},
			{"the zero notebook", uuid.UUID{}, app.GrepRequest{Pattern: "a"}, "notebook.not_found"},
			{"the slot", f.eng, app.GrepRequest{Pattern: "a"}, "server_busy"},
		} {
			if _, err := g.Execute(f.asAlice(), row.notebook, row.r); codeOf(err) != row.want {
				t.Errorf("%s: %v, want %s", row.name, codeOf(err), row.want)
			}
		}
		if !strings.Contains(f.logs.String(), "grep is saturated") {
			t.Errorf("no saturation logged: %s", f.logs)
		}
	})
	t.Run("candidates and building", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionRead)
		reads := newFakeGrep(f.store)
		built, other := f.page("Built", nil, 0), f.page("Other", nil, 1)
		bulk, exact := f.page("Bulk", nil, 2), f.page("Exact", nil, 3)
		f.content(built.ID, "a needle here", 1)
		f.content(other.ID, "nothing", 1)
		f.content(bulk.ID, "needle", 1)
		f.content(exact.ID, "needle", 1)
		reads.build(built.ID, other.ID)
		reads.bulk[bulk.ID], reads.exact[exact.ID] = true, true
		g := f.grepper(reads, nil)
		got, err := g.Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: "needle"})
		if err != nil {
			t.Fatal(err)
		}
		if found := grepFound(got); !slices.Equal(found, []string{"Built.md:1", "Exact.md:1"}) || !got.Building || got.Incomplete ||
			got.Next != "" || got.Walk.Candidates != 2 || got.Walk.Pages != 2 || got.Walk.Keys != 4 {
			t.Errorf("with keys: %q, %+v", found, got)
		}
		// Without keys, every page, the bulk one too; never building.
		got, err = g.Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: "ne+dle|nope"})
		if err != nil {
			t.Fatal(err)
		}
		if found := grepFound(got); !slices.Equal(found, []string{"Built.md:1", "Bulk.md:1", "Exact.md:1"}) || got.Building ||
			got.Walk.Candidates != -1 || got.Walk.Keys != 0 {
			t.Errorf("without keys: %q, %+v", found, got)
		}
	})
	t.Run("building told when the job empties the queue between", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionRead)
		reads := newFakeGrep(f.store)
		late := f.page("Late", nil, 0)
		f.content(late.ID, "needle", 1)
		reads.bulk[late.ID] = true
		reads.between = func() { reads.build(late.ID) }
		got, err := f.grepper(reads, nil).Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: "needle"})
		if err != nil {
			t.Fatal(err)
		}
		if !got.Building {
			t.Errorf("found %q, building %v: the page queued in bulk is neither found nor told", grepFound(got), got.Building)
		}
	})
	t.Run("the glob before the bounds", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionRead)
		reads := newFakeGrep(f.store)
		for i := range 600 {
			f.content(f.page(fmt.Sprintf("%03d", i), nil, float64(i)).ID, "needle", 1)
		}
		a := f.page("A", nil, 700)
		f.content(f.page("Note", &a.ID, 0).ID, "needle", 1)
		reads.buildAll()
		got, err := f.grepper(reads, nil).Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: "needle", Glob: strp("A/**")})
		if err != nil {
			t.Fatal(err)
		}
		if found := grepFound(got); !slices.Equal(found, []string{"A/Note.md:1"}) || got.Next != "" || got.Walk.Pages != 1 {
			t.Errorf("A/** found %q, %+v", found, got)
		}
	})
	t.Run("resumed from a line", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionRead)
		reads := newFakeGrep(f.store)
		f.content(f.page("Lines", nil, 0).ID, "needle 1\nx\nneedle 3\nneedle 4\n", 1)
		f.content(f.page("More", nil, 1).ID, "needle\n", 1)
		reads.buildAll()
		one := 1
		all, pages := grepAll(t, f.asAlice(), f.grepper(reads, nil), f.eng, app.GrepRequest{Pattern: "needle", Limit: &one})
		if !slices.Equal(all, []string{"Lines.md:1", "Lines.md:3", "Lines.md:4", "More.md:1"}) {
			t.Errorf("one hit a page: %q", all)
		}
		for i, p := range pages[:len(pages)-1] {
			if p.Incomplete || p.Walk.Stop != "limit" {
				t.Errorf("page %d at its limit: incomplete %v, stop %q", i, p.Incomplete, p.Walk.Stop)
			}
		}
	})
	t.Run("resumed after a page renamed or deleted", func(t *testing.T) {
		for _, change := range []string{"renamed", "deleted"} {
			f := newFixture()
			f.grant(domain.ActionRead)
			reads := newFakeGrep(f.store)
			b := f.page("B", nil, 0)
			f.content(b.ID, "needle 1\nneedle 2\n", 1)
			f.content(f.page("C", nil, 1).ID, "needle\n", 1)
			reads.buildAll()
			one := 1
			g := f.grepper(reads, nil)
			first, err := g.Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: "needle", Limit: &one})
			if err != nil || !slices.Equal(grepFound(first), []string{"B.md:1"}) {
				t.Fatalf("%q, %v", grepFound(first), err)
			}
			if change == "renamed" {
				n := f.store.nodes[b.ID]
				n.Name, n.NameKey = "Z", "z"
				f.store.nodes[b.ID] = n
			} else {
				delete(f.store.nodes, b.ID)
				delete(f.store.contents, b.ID)
			}
			all, _ := grepAll(t, f.asAlice(), g, f.eng, app.GrepRequest{Pattern: "needle", Limit: &one, Cursor: &first.Next})
			want := []string{"C.md:1"}
			if change == "renamed" {
				want = append(want, "Z.md:1", "Z.md:2") // after C now: walked again, from its start
			}
			if !slices.Equal(all, want) {
				t.Errorf("B %s: on from %q found %q, want %q", change, "B.md:2", all, want)
			}
		}
	})
	t.Run("batches", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionRead)
		reads := newFakeGrep(f.store)
		var ids []uuid.UUID
		for i := range 3 {
			p := f.page(fmt.Sprintf("P%d", i), nil, float64(i))
			f.content(p.ID, fmt.Sprintf("needle %d", i), 1)
			c := f.store.contents[p.ID]
			c.ByteSize = 3 << 20 // the states say 3 MiB: a batch each
			f.store.contents[p.ID] = c
			ids = append(ids, p.ID)
		}
		reads.buildAll()
		f.rec.calls = nil
		got, err := f.grepper(reads, nil).Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: "needle"})
		if err != nil {
			t.Fatal(err)
		}
		reads1 := slices.DeleteFunc(slices.Clone(f.rec.calls), func(c string) bool { return !strings.HasPrefix(c, "GrepContents") })
		if found := grepFound(got); !slices.Equal(found, []string{"P0.md:1", "P1.md:1", "P2.md:1"}) || len(reads1) != 3 ||
			reads1[0] != "GrepContents 1" {
			t.Errorf("three batches: %q, reads %q", found, reads1)
		}
		// The read's bound before a batch: 3 MiB said, a page read, so the
		// second batch is not read.
		f.rec.calls = nil
		got, err = f.grepper(reads, func(d *app.GrepDeps) { d.MaxRead = 3 << 20 }).Execute(f.asAlice(), f.eng,
			app.GrepRequest{Pattern: "needle"})
		if err != nil {
			t.Fatal(err)
		}
		reads2 := slices.DeleteFunc(slices.Clone(f.rec.calls), func(c string) bool { return !strings.HasPrefix(c, "GrepContents") })
		if found := grepFound(got); !slices.Equal(found, []string{"P0.md:1"}) || len(reads2) != 1 || !got.Incomplete ||
			got.Walk.Stop != "bytes" || cursorOf(t, got.Next) != (domain.GrepCursor{K: "p1.md", N: 1}) {
			t.Errorf("the read's bound: %q, reads %q, %+v", found, reads2, got)
		}
		// A page read back larger than its state says counts as read: the
		// bound stops before the next batch, which its state alone would
		// let in (M9/P4 review C-M10).
		for i, id := range ids {
			c := f.store.contents[id]
			c.ByteSize = []int{1 << 10, 4 << 20, 1 << 10}[i]
			f.store.contents[id] = c
		}
		reads.grown[ids[0]] = "needle\n" + strings.Repeat("x", 3<<20)
		got, err = f.grepper(reads, func(d *app.GrepDeps) { d.MaxRead = 6 << 20 }).Execute(f.asAlice(), f.eng,
			app.GrepRequest{Pattern: "needle"})
		if err != nil {
			t.Fatal(err)
		}
		if found := grepFound(got); !slices.Equal(found, []string{"P0.md:1"}) || got.Walk.Stop != "bytes" || got.Walk.Bytes < 3<<20 {
			t.Errorf("a page grown: %q, %+v", found, got)
		}
	})
	t.Run("the limit", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionRead)
		reads := newFakeGrep(f.store)
		f.content(f.page("L", nil, 0).ID, "needle\nneedle\nneedle\n", 1)
		reads.buildAll()
		two := 2
		got, err := f.grepper(reads, nil).Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: "needle", Limit: &two})
		if err != nil {
			t.Fatal(err)
		}
		if found := grepFound(got); !slices.Equal(found, []string{"L.md:1", "L.md:2"}) || got.Incomplete || got.Walk.Stop != "limit" ||
			cursorOf(t, got.Next) != (domain.GrepCursor{K: "l.md", N: 3}) || got.Found[1].Cursor != got.Next {
			t.Errorf("at the limit: %q, %+v", found, got)
		}
	})
	t.Run("the walk's pages and the glob's work", func(t *testing.T) {
		f := newFixture()
		f.grant(domain.ActionRead)
		reads := newFakeGrep(f.store)
		for i := range 5 {
			f.content(f.page(fmt.Sprintf("P%d", i), nil, float64(i)).ID, "needle", 1)
		}
		reads.buildAll()
		g := f.grepper(reads, func(d *app.GrepDeps) { d.Walk = 3 })
		got, err := g.Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: "needle"})
		if err != nil {
			t.Fatal(err)
		}
		if !got.Incomplete || got.Walk.Stop != "pages" || got.Walk.Pages != 3 || cursorOf(t, got.Next) != (domain.GrepCursor{K: "p2.md"}) {
			t.Errorf("3 pages a walk: %q, %+v", grepFound(got), got)
		}
		if all, _ := grepAll(t, f.asAlice(), g, f.eng, app.GrepRequest{Pattern: "needle"}); len(all) != 5 {
			t.Errorf("3 pages a walk, on to the end: %q", all)
		}
		g = f.grepper(reads, func(d *app.GrepDeps) { d.GlobWork = 1 })
		got, err = g.Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: "needle"})
		if err != nil {
			t.Fatal(err)
		}
		if !got.Incomplete || got.Walk.Stop != "glob" || len(got.Found) != 1 || cursorOf(t, got.Next) != (domain.GrepCursor{K: "p0.md"}) {
			t.Errorf("the glob's work: %q, %+v", grepFound(got), got)
		}
		if all, _ := grepAll(t, f.asAlice(), g, f.eng, app.GrepRequest{Pattern: "needle"}); len(all) != 5 {
			t.Errorf("the glob's work, on to the end: %q", all)
		}
	})
}

// cursorOf decodes a grep's cursor.
func cursorOf(t *testing.T, s string) domain.GrepCursor {
	t.Helper()
	c, err := domain.DecodeGrepCursor(s)
	if err != nil {
		t.Fatalf("cursor %q: %v", s, err)
	}
	return c
}

// TestGrepWalksEveryPartialPage: a page whose row leaves its text out is a
// candidate by !cut, though its lexemes lack the keys; its line is found
// (M9/P4 design 3.4.2).
func TestGrepWalksEveryPartialPage(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	reads := newFakeGrep(f.store)
	long := f.page("Long", nil, 0)
	f.content(long.ID, "x"+strings.Repeat("ab", 20)+"needle"+strings.Repeat("cd", 20), 1)
	f.content(f.page("Plain", nil, 1).ID, "no such word", 1)
	reads.buildAll()
	if row := reads.rows[long.ID]; !row.partial || slices.Contains(row.body, "nee") {
		t.Fatalf("the long run's row: partial %v, lexemes %q", row.partial, row.body)
	}
	got, err := f.grepper(reads, nil).Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if found := grepFound(got); !slices.Equal(found, []string{"Long.md:1"}) || got.Walk.Candidates != 1 {
		t.Errorf("found %q, %+v", found, got)
	}
}

// A call's hits hold their texts, not the contents they were found in
// (M9 design 4.13: a call holds at most a batch of contents): eight pages
// of about 4 MiB, each read as a string of its own, each with a hit on its
// first line, whole in four and cut about the match in the other four,
// leave a result that holds well under a batch; held as parts of their
// lines, the hits held the eight contents, 32 MiB, until the answer was
// written (M9 closeout A-F2).
func TestGrepsHitsHoldTheirTextsAlone(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	reads := newFakeGrep(f.store)
	reads.fresh = true
	filler := strings.Repeat(strings.Repeat("word ", 819)+"\n", 1020)
	for i := range 8 {
		first := fmt.Sprintf("needle %d", i)
		if i%2 == 1 {
			first = strings.Repeat("a ", domain.MaxGrepLineText) + first
		}
		p := f.page(fmt.Sprintf("P%d", i), nil, float64(i))
		f.content(p.ID, first+"\n"+filler, 1)
		reads.exact[p.ID] = true // a candidate, no row of 4 MiB built
	}
	// The work of 32 MiB passes a call's; the read is within MaxGrepRead.
	g := f.grepper(reads, func(d *app.GrepDeps) { d.MaxWork = 1 << 40 })
	before := heapHeld()
	got, err := g.Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	held := heapHeld() - before
	cut := 0
	for _, found := range got.Found {
		if found.Hit != nil && found.Hit.Cut {
			cut++
		}
	}
	if len(got.Found) != 8 || cut != 4 || got.Walk.Pages != 8 || got.Next != "" {
		t.Fatalf("found %q, %d cut, %+v", grepFound(got), cut, got.Walk)
	}
	if held > domain.GrepBatchBytes/4 {
		t.Errorf("the result holds %.1f MiB, more than a quarter of a batch: its hits hold their contents", float64(held)/(1<<20))
	}
	// The store's contents are of the base, held through the measure.
	runtime.KeepAlive(f)
	runtime.KeepAlive(g)
}

// frozenClock reads one time.
type frozenClock struct{ at time.Time }

func (c frozenClock) Now() time.Time { return c.at }

// TestGrepStopsBeforeALineItsWorkWouldPass checks the judgement before
// each line (M9/P4 design 3.6.3): the walk stops before the line whose
// work would pass MaxWork, its cursor that line (mutant 155); the call's
// first line is never judged, so a call goes on a line at least (mutant
// 156); a soft deadline past before the first read still reads and
// matches the first line (mutant 157); a unit's time is GrepUnitTime at
// least, however cheap the lines before; a line not in NFC is judged by
// its NFC, by the work and by the soft deadline.
func TestGrepStopsBeforeALineItsWorkWouldPass(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	reads := newFakeGrep(f.store)
	f.content(f.page("W", nil, 0).ID, "needle 1\nneedle 2\nneedle 3\n", 1)
	reads.buildAll()
	// needle: 8 instructions; a line of 8 bytes, (8 + 2) × 8 = 80 units.
	for _, row := range []struct {
		name    string
		maxWork int64
		found   []string
		next    domain.GrepCursor
	}{
		{"two lines within", 200, []string{"W.md:1", "W.md:2"}, domain.GrepCursor{K: "w.md", N: 3}},
		{"the first line past", 10, []string{"W.md:1"}, domain.GrepCursor{K: "w.md", N: 2}},
	} {
		got, err := f.grepper(reads, func(d *app.GrepDeps) { d.MaxWork = row.maxWork }).Execute(f.asAlice(), f.eng,
			app.GrepRequest{Pattern: "needle"})
		if err != nil {
			t.Fatal(err)
		}
		if found := grepFound(got); !slices.Equal(found, row.found) || !got.Incomplete || got.Walk.Stop != "work" ||
			cursorOf(t, got.Next) != row.next || got.Walk.Work > max(row.maxWork, 80) {
			t.Errorf("%s: %q, %+v", row.name, found, got)
		}
	}
	// A line not in NFC, judged by the bytes it is matched in (review
	// C-I2): "x", then 100 U+1D160, 400 bytes and 1,200 in NFC; zzz, 5
	// instructions; MaxWork the first line's work and the second's as its
	// own bytes would judge it, (1 + 2) × 5 + (400 + 2) × 5 = 2,025. The
	// second line's NFC, (1,200 + 2) × 5 = 6,010, does not fit: the call
	// stops before it, its work within MaxWork. The page is queued exact,
	// a candidate whatever its keys.
	wide := newFakeGrep(f.store)
	wideID := f.page("Wide", nil, 1).ID
	f.content(wideID, "x\n"+strings.Repeat("\U0001D160", 100)+"\n", 1)
	wide.exact[wideID] = true
	got, err := f.grepper(wide, func(d *app.GrepDeps) { d.MaxWork = 2025 }).Execute(f.asAlice(), f.eng,
		app.GrepRequest{Pattern: "zzz", Glob: ptr("Wide.md")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Walk.Work > 2025 || got.Walk.Stop != "work" || cursorOf(t, got.Next) != (domain.GrepCursor{K: "wide.md", N: 2}) {
		t.Errorf("a line not in NFC: %+v, want a stop at line 2 within 2,025 units", got)
	}
	deadline := time.Now().Add(time.Hour)
	ctx, cancel := context.WithDeadline(f.asAlice(), deadline)
	defer cancel()
	// Past the soft deadline before the first read: the first line still.
	got, err = f.grepper(reads, func(d *app.GrepDeps) { d.Clock = frozenClock{deadline} }).Execute(ctx, f.eng,
		app.GrepRequest{Pattern: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if found := grepFound(got); !slices.Equal(found, []string{"W.md:1"}) || got.Walk.Stop != "deadline" ||
		cursorOf(t, got.Next) != (domain.GrepCursor{K: "w.md", N: 2}) {
		t.Errorf("past the soft deadline: %q, %+v", found, got)
	}
	// A microsecond before the soft deadline, the clock still: the lines
	// cost nothing as timed, 80 units of GrepUnitTime as judged, 2 µs.
	soft := deadline.Add(-domain.GrepSoftMargin)
	got, err = f.grepper(reads, func(d *app.GrepDeps) { d.Clock = frozenClock{soft.Add(-time.Microsecond)} }).Execute(ctx, f.eng,
		app.GrepRequest{Pattern: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if found := grepFound(got); !slices.Equal(found, []string{"W.md:1"}) || got.Walk.Stop != "deadline" {
		t.Errorf("a unit's time at least GrepUnitTime: %q, %+v", found, got)
	}
	// The soft deadline judges a line not in NFC by its NFC too (reviews
	// A1-n1, C-f-n1): the Wide page's second line is 2,010 units as its own
	// bytes, 50 µs of GrepUnitTime, and 6,010 as its NFC, 150 µs; 100 µs
	// before the soft deadline, the clock still, the call stops before it.
	got, err = f.grepper(wide, func(d *app.GrepDeps) { d.Clock = frozenClock{soft.Add(-100 * time.Microsecond)} }).Execute(ctx,
		f.eng, app.GrepRequest{Pattern: "zzz", Glob: ptr("Wide.md")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Walk.Stop != "deadline" || got.Walk.Lines != 1 || cursorOf(t, got.Next) != (domain.GrepCursor{K: "wide.md", N: 2}) {
		t.Errorf("a line not in NFC by the soft deadline: %+v, want a stop before line 2", got)
	}
}

// scanClock is a clock that moves only as its lines are scanned: a
// microsecond a byte of each line, before the line is yielded, as
// GrepLines finds a line's end before it yields the line. A match, and a
// read of the clock, take no time.
type scanClock struct{ at time.Time }

func (c *scanClock) Now() time.Time { return c.at }

// lines is domain.GrepLines, the clock moved by each line's scan.
func (c *scanClock) lines(content string, from int, yield func(line int, text string, column int) bool) {
	domain.GrepLines(content, from, func(line int, text string, column int) bool {
		c.at = c.at.Add(time.Duration(len(text)) * time.Microsecond)
		return yield(line, text, column)
	})
}

// TestGrepTimesALinesMatchAlone checks a unit's time (M9/P4 design 3.6.3,
// review C2-m3): it is the time the lines took to match, not the time of
// the scans for their ends between them. A page of a line of 20,000 bytes,
// 160,016 units, past GrepUnitWarmUp, then a line of a million bytes and
// needle; the clock moves a microsecond a byte as each line is scanned,
// and not as anything is matched. A unit timed from one line to the next,
// or across the scan between them, takes in the long line's scan, 1 s
// over the first line's units, about 6.2 µs, and the long line's 8 million
// units about 50 s, past the soft deadline half a minute away: the call
// stops before the long line, no hit, incomplete. Timed by the lines'
// matches, the unit is GrepUnitTime, the long line 0.2 s: the first call
// answers its hit, the walk done.
func TestGrepTimesALinesMatchAlone(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	reads := newFakeGrep(f.store)
	id := f.page("Long", nil, 0).ID
	f.content(id, strings.Repeat("a", 20_000)+"\n"+strings.Repeat("a", 1_000_000)+" needle\n", 1)
	reads.exact[id] = true
	deadline := time.Now().Add(time.Hour)
	ctx, cancel := context.WithDeadline(f.asAlice(), deadline)
	defer cancel()
	clock := &scanClock{at: deadline.Add(-domain.GrepSoftMargin - 30*time.Second)}
	got, err := f.grepper(reads, func(d *app.GrepDeps) { d.Clock = clock }).WithLines(clock.lines).Execute(ctx, f.eng,
		app.GrepRequest{Pattern: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if found := grepFound(got); !slices.Equal(found, []string{"Long.md:2"}) || got.Incomplete || got.Walk.Stop != "done" ||
		got.Walk.Lines != 2 || got.Next != "" {
		t.Errorf("20,000 bytes, then a line of a million bytes: %q, %+v; want its hit in the first call, the walk done", found, got)
	}
}

// pauseClock is a clock that moves only while line 1 is matched: by
// pause at each read, so that line 1's time is one pause, as a GC assist
// or the OS taking the CPU would make it; its lines are domain.GrepLines,
// line 1 told.
type pauseClock struct {
	at     time.Time
	pause  time.Duration
	inLine bool
}

func (c *pauseClock) Now() time.Time {
	if c.inLine {
		c.at = c.at.Add(c.pause)
	}
	return c.at
}

func (c *pauseClock) lines(content string, from int, yield func(line int, text string, column int) bool) {
	domain.GrepLines(content, from, func(line int, text string, column int) bool {
		c.inLine = line == 1
		ok := yield(line, text, column)
		c.inLine = false
		return ok
	})
}

// stepClock is a slow machine's clock: each read moves it by step, so
// that a line's match takes one step.
type stepClock struct {
	at   time.Time
	step time.Duration
}

func (c *stepClock) Now() time.Time {
	c.at = c.at.Add(c.step)
	return c.at
}

// TestGrepTrustsItsUnitPastAWarmUp checks a unit's time (M9/P4 design
// 3.6.3; S2 fix check C3-m1): a call's own timing tells it only once it
// rests on GrepUnitWarmUp of work. A page of "x", then a line of 240 KiB to
// 4 MB and needle, at production's deadline, the soft one 12 s away, and a
// pause of 10 µs to 1 ms in matching "x", its 24 units: timed on them, the
// unit was 0.4 to 42 µs and stopped the long line, of 2 to 32
// million units, at the soft deadline from 10 µs for 4 MB, 20 µs for 2 MB,
// 100 µs for 1 MB, 1 ms for 240 KiB. Below the warm-up the unit is
// GrepUnitTime: the first call answers the long line's hit. So it does
// after a first line of 12,000 bytes, 96,016 units, just under the
// warm-up, and a pause of 40 ms in it, which trusted would tell 417 ns a
// unit and 13 s for the 4 MB line after it (fix check C4-n1): a warm-up of
// 96,016 or less fails the row, and the slow machine's below holds it at
// 160,016 at most.
// Past it, a slow match raises the unit (fix check C3-m2): a machine on
// which the first line, 20,000 bytes and 160,016 units, takes 160 ms, a
// microsecond a unit, 40 times GrepUnitTime, projects the long line of a
// million bytes, 8 million units, to 8 s, past the soft deadline 2 s away,
// and stops before it, incomplete, its cursor that line: at GrepUnitTime
// it would be judged to take 0.2 s, and run to the request's deadline.
func TestGrepTrustsItsUnitPastAWarmUp(t *testing.T) {
	for _, n := range []int{240 << 10, 1_000_000, 2_000_000, 4_000_000} {
		for _, pause := range []time.Duration{10 * time.Microsecond, 20 * time.Microsecond, 100 * time.Microsecond, time.Millisecond} {
			f := newFixture()
			f.grant(domain.ActionRead)
			reads := newFakeGrep(f.store)
			id := f.page("Long", nil, 0).ID
			f.content(id, "x\n"+strings.Repeat("a", n)+" needle\n", 1)
			reads.exact[id] = true
			deadline := time.Now().Add(time.Hour)
			ctx, cancel := context.WithDeadline(f.asAlice(), deadline)
			clock := &pauseClock{at: deadline.Add(-15 * time.Second), pause: pause}
			got, err := f.grepper(reads, func(d *app.GrepDeps) { d.Clock = clock }).WithLines(clock.lines).Execute(ctx, f.eng,
				app.GrepRequest{Pattern: "needle"})
			cancel()
			if err != nil {
				t.Fatal(err)
			}
			if found := grepFound(got); !slices.Equal(found, []string{"Long.md:2"}) || got.Incomplete || got.Walk.Stop != "done" {
				t.Errorf("x, a pause of %v in it, then a line of %d bytes: %q, %+v; want its hit in the first call", pause, n, found, got)
			}
		}
	}
	{
		f := newFixture()
		f.grant(domain.ActionRead)
		reads := newFakeGrep(f.store)
		id := f.page("Warm", nil, 0).ID
		f.content(id, strings.Repeat("a", 12_000)+"\n"+strings.Repeat("a", 4_000_000)+" needle\n", 1)
		reads.exact[id] = true
		deadline := time.Now().Add(time.Hour)
		ctx, cancel := context.WithDeadline(f.asAlice(), deadline)
		clock := &pauseClock{at: deadline.Add(-15 * time.Second), pause: 40 * time.Millisecond}
		got, err := f.grepper(reads, func(d *app.GrepDeps) { d.Clock = clock }).WithLines(clock.lines).Execute(ctx, f.eng,
			app.GrepRequest{Pattern: "needle"})
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if found := grepFound(got); !slices.Equal(found, []string{"Warm.md:2"}) || got.Incomplete || got.Walk.Stop != "done" {
			t.Errorf("12,000 bytes, just under the warm-up, a pause of 40 ms in them, then 4 MB: %q, %+v; want its hit in the first call",
				found, got)
		}
	}
	f := newFixture()
	f.grant(domain.ActionRead)
	reads := newFakeGrep(f.store)
	id := f.page("Slow", nil, 0).ID
	f.content(id, strings.Repeat("a", 20_000)+"\n"+strings.Repeat("a", 1_000_000)+" needle\n", 1)
	reads.exact[id] = true
	deadline := time.Now().Add(time.Hour)
	ctx, cancel := context.WithDeadline(f.asAlice(), deadline)
	defer cancel()
	clock := &stepClock{at: deadline.Add(-domain.GrepSoftMargin - 2*time.Second), step: 160 * time.Millisecond}
	got, err := f.grepper(reads, func(d *app.GrepDeps) { d.Clock = clock }).Execute(ctx, f.eng, app.GrepRequest{Pattern: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if found := grepFound(got); len(found) != 0 || !got.Incomplete || got.Walk.Stop != "deadline" || got.Walk.Lines != 1 ||
		cursorOf(t, got.Next) != (domain.GrepCursor{K: "slow.md", N: 2}) {
		t.Errorf("a slow machine, a microsecond a unit past the warm-up: %q, %+v; want a stop before the long line", found, got)
	}
}

// TestGrepParseQueue checks the parses' queue (M9/P4 design 3.6.1,
// review A2-m1 = C2-M1): 20 calls at once on 2 turns, the fake parse
// counting the parses in flight, which wraps the parse itself: at most 2
// at once, and some waited (mutant 144); a call whose ctx ends while it
// waits answers ctx's error; none answers 503.
func TestGrepParseQueue(t *testing.T) {
	var inFlight, most atomic.Int64
	parse := func(pattern string, ignoreCase bool) (domain.GrepPattern, error) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for m := most.Load(); n > m && !most.CompareAndSwap(m, n); m = most.Load() {
		}
		time.Sleep(3 * time.Millisecond)
		return domain.ParseGrepPattern(pattern, ignoreCase)
	}
	q := app.NewGrepParsesWith(2, parse)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	start := time.Now()
	for range 20 {
		wg.Go(func() {
			_, err := q.Parse(context.Background(), "needle", false)
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("Parse: %v", err)
		}
	}
	if took := time.Since(start); most.Load() != 2 || took < 25*time.Millisecond {
		t.Errorf("20 parses of 3 ms on 2 turns: %d at once at most, %v in all", most.Load(), took)
	}
	// A call that waits past its ctx: ctx's error, no 503.
	started, block := make(chan struct{}), make(chan struct{})
	held := app.NewGrepParsesWith(1, func(string, bool) (domain.GrepPattern, error) {
		close(started)
		<-block
		return domain.GrepPattern{}, nil
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = held.Parse(context.Background(), "a", false)
	}()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, err := held.Parse(ctx, "a", false); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a parse waiting past its ctx: %v", err)
	}
	close(block)
	<-done
}

// TestGrepSlots checks the slots (M9/P4 design 3.6.5), their wait a few
// ms: a call waits for its slot and takes it when it is released meanwhile
// (mutant 154); after the wait, server_busy; a ctx ended first, its error;
// a slot released is taken by the next.
func TestGrepSlots(t *testing.T) {
	s := app.NewGrepSlotsWaiting(1, 200*time.Millisecond)
	release, err := s.Take(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		release()
	}()
	second, err := s.Take(context.Background())
	if err != nil {
		t.Fatalf("a slot released during the wait: %v", err)
	}
	busy := app.NewGrepSlotsWaiting(1, 5*time.Millisecond)
	hold, err := busy.Take(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var se *shared.Error
	if _, err := busy.Take(context.Background()); !errors.As(err, &se) || se.Code != shared.CodeServerBusy || se.RetryDelay != time.Second {
		t.Errorf("past the wait: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := busy.Take(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("a ctx ended first: %v", err)
	}
	hold()
	if next, err := busy.Take(context.Background()); err != nil {
		t.Errorf("the slot released: %v", err)
	} else {
		next()
	}
	second()
}
