package app_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// indexReads is the index's reads of index, faked: each page the index
// holds and its facts, its whole type among them; how many pages link to
// each; the pages of each tag's key. Its calls go to the library's.
type indexReads struct {
	*library
	held    []uuid.UUID
	facts   map[uuid.UUID]app.IndexFacts
	inbound map[uuid.UUID]int
	tagged  map[string][]uuid.UUID
	dropped map[uuid.UUID]bool // IndexPages lists them, their rows gone by IndexItems
}

func (r *indexReads) IndexPages(_ context.Context, _ uuid.UUID, typeIs *string) ([]app.IndexedPage, error) {
	r.call("IndexPages")
	var out []app.IndexedPage
	for _, id := range r.held {
		f := r.facts[id]
		if typeIs != nil && (f.Type == nil || *f.Type != *typeIs) {
			continue
		}
		p := app.IndexedPage{ID: id}
		if f.Type != nil {
			runes := []rune(*f.Type)
			p.TypeStart, p.Typed = string(runes[:min(len(runes), domain.IndexTypeChars)]), true
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *indexReads) IndexItems(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]app.IndexFacts, error) {
	r.call(fmt.Sprintf("IndexItems %d", len(ids)))
	out := map[uuid.UUID]app.IndexFacts{}
	for _, id := range ids {
		if f, ok := r.facts[id]; ok && slices.Contains(r.held, id) && !r.dropped[id] {
			out[id] = f
		}
	}
	return out, nil
}

func (r *indexReads) InboundCounts(_ context.Context, ids []uuid.UUID, most int) (map[uuid.UUID]int, error) {
	r.call("InboundCounts")
	if most != domain.MaxInbound {
		return nil, fmt.Errorf("counted to %d", most)
	}
	out := map[uuid.UUID]int{}
	for _, id := range ids {
		if n, ok := r.inbound[id]; ok {
			out[id] = n
		}
	}
	return out, nil
}

func (r *indexReads) TagPages(_ context.Context, _ uuid.UUID, key string) ([]uuid.UUID, error) {
	r.call("TagPages " + key)
	return r.tagged[key], nil
}

// indexLib is a library with an index of the notebook nb: Projects (type
// area), its child pages Roadmap (type Project, tags plan and plan/q4, a
// summary of 300 bytes), Spec (type project, tag plan) and Notes (no type),
// and Inbox at the root (no type); Gone, of type zeta, is held but put in
// the trash, without a path; Old, of type aaa, has a path and is listed by
// IndexPages, its rows gone when IndexItems reads (S2 review C-n1).
type indexLib struct {
	lib
	paths                                            *paths
	reads                                            *indexReads
	projects, roadmap, spec, notes, inbox, gone, old uuid.UUID
}

func newIndexLib() indexLib {
	l := indexLib{lib: newLibrary()}
	l.paths, l.reads = newPaths(l.library), &indexReads{library: l.library, facts: map[uuid.UUID]app.IndexFacts{},
		inbound: map[uuid.UUID]int{}, tagged: map[string][]uuid.UUID{}, dropped: map[uuid.UUID]bool{}}
	str := func(s string) *string { return &s }
	page := func(path string, f app.IndexFacts) uuid.UUID {
		id := l.paths.add(uuid.NewV7(), path)
		l.reads.held, l.reads.facts[id] = append(l.reads.held, id), f
		return id
	}
	l.projects = page("Projects.md", app.IndexFacts{Type: str("area")})
	l.roadmap = page("Projects/Roadmap.md", app.IndexFacts{Type: str("Project"), Summary: str(strings.Repeat("s", 300)),
		Tags: []string{"plan", "plan/q4"}})
	l.spec = page("Projects/Spec.md", app.IndexFacts{Type: str("project"), Tags: []string{"plan"}})
	l.notes = page("Projects/Notes.md", app.IndexFacts{})
	l.inbox = page("Inbox.md", app.IndexFacts{})
	l.gone = page("Gone.md", app.IndexFacts{Type: str("zeta")})
	l.paths.gone(l.gone)
	l.old = page("Old.md", app.IndexFacts{Type: str("aaa")})
	l.reads.dropped[l.old] = true
	l.reads.inbound[l.roadmap], l.reads.inbound[l.spec] = 2, 1
	l.reads.tagged["plan"] = []uuid.UUID{l.roadmap, l.spec}
	l.reads.tagged["plan/q4"] = []uuid.UUID{l.roadmap}
	return l
}

func (l indexLib) listIndex() app.ListIndex {
	return app.ListIndex{Access: l.access(), Reads: l.reads, Paths: l.paths, TagKey: func(name string) (string, bool) {
		return strings.ToLower(name), name != "" && !strings.ContainsAny(name, " \x00#")
	}}
}

// ordered is the pages of ids in the index's order: those with a type by
// its key and id, then the others by id.
func (l indexLib) ordered(ids ...uuid.UUID) []uuid.UUID {
	slices.SortFunc(ids, func(a, b uuid.UUID) int {
		ka, kb := l.reads.facts[a].Type, l.reads.facts[b].Type
		ca, cb := domain.IndexCursor{ID: a, Typed: ka != nil}, domain.IndexCursor{ID: b, Typed: kb != nil}
		if ka != nil {
			ca.K = domain.IndexKey(*ka)
		}
		if kb != nil {
			cb.K = domain.IndexKey(*kb)
		}
		return domain.CompareIndex(ca, cb)
	})
	return ids
}

// idsOf is the pages of p's items, in order.
func idsOf(p app.IndexPage) []uuid.UUID {
	var out []uuid.UUID
	for _, it := range p.Items {
		out = append(out, it.PageID)
	}
	return out
}

// index judges the cursor, the type, the tag and the parent, the limit,
// the notebook and the parent's page, in this order, each before any read
// the next needs: "" for any filter, a type with NUL or past 1,024 bytes,
// a parent no page's path but "/" (M9/P4 design 3.9.3; 2.1, items 13 and
// 16; review B-M7).
func TestListIndexJudgesInTurn(t *testing.T) {
	l := newIndexLib()
	s := func(v string) *string { return &v }
	bad, zero := "x", 0
	long := strings.Repeat("t", domain.MaxIndexType+1)
	for _, tt := range []struct {
		name  string
		nb    uuid.UUID
		q     app.IndexQuery
		is    error
		field string
		calls []string
	}{
		{name: "a bad cursor", nb: l.hidden, q: app.IndexQuery{Type: s(""), Limit: &zero, Cursor: &bad}, is: shared.InvalidCursor()},
		{name: "an empty type", nb: l.hidden, q: app.IndexQuery{Type: s(""), Limit: &zero}, field: "type"},
		{name: "a type with NUL", nb: l.hidden, q: app.IndexQuery{Type: s("a\x00")}, field: "type"},
		{name: "a type too long", nb: l.hidden, q: app.IndexQuery{Type: &long}, field: "type"},
		{name: "an empty tag", nb: l.hidden, q: app.IndexQuery{Tag: s(""), Limit: &zero}, field: "tag"},
		{name: "an empty parent", nb: l.hidden, q: app.IndexQuery{Parent: s(""), Limit: &zero}, field: "parent"},
		{name: "a bad parent", nb: l.hidden, q: app.IndexQuery{Parent: s("a//b.md"), Limit: &zero}, field: "parent",
			calls: []string{"Check parent"}},
		{name: "a bad limit", nb: l.hidden, q: app.IndexQuery{Parent: s("Projects.md"), Limit: &zero}, field: "limit",
			calls: []string{"Check parent"}},
		{name: "a notebook hidden", nb: l.hidden, q: app.IndexQuery{Parent: s("/")}, is: domain.ErrNotebookNotFound,
			calls: []string{"WorkspaceOf", "Authorize index.list"}},
		{name: "nothing at the parent", nb: l.nb, q: app.IndexQuery{Parent: s("Nothing.md"), Tag: s("plan")}, is: errNothingAtPath,
			calls: []string{"Check parent", "WorkspaceOf", "Authorize index.list", "FindPage parent Nothing.md"}},
	} {
		l.calls = nil
		_, err := l.listIndex().Execute(reader(), tt.nb, tt.q)
		var problem *shared.Error
		switch {
		case tt.is != nil && !errors.Is(err, tt.is):
			t.Errorf("%s: %v, want %v", tt.name, err, tt.is)
		case tt.field != "" && (!errors.As(err, &problem) || problem.Code != shared.CodeValidationFailed ||
			len(problem.Fields) != 1 || problem.Fields[0].Field != tt.field):
			t.Errorf("%s: %v, want 422 at %s", tt.name, err, tt.field)
		}
		if !slices.Equal(l.calls, tt.calls) {
			t.Errorf("%s: calls %v, want %v", tt.name, l.calls, tt.calls)
		}
	}
	at := strings.Repeat("t", domain.MaxIndexType)
	if _, err := l.listIndex().Execute(reader(), l.nb, app.IndexQuery{Type: &at}); err != nil {
		t.Errorf("a type of %d bytes: %v", len(at), err)
	}
	// A tag no page can have is an empty page, the index unread, as GetTag.
	for _, tag := range []string{"a\x00b", "#", "a b"} {
		l.calls = nil
		got, err := l.listIndex().Execute(reader(), l.nb, app.IndexQuery{Tag: &tag})
		if err != nil || got.Items == nil || len(got.Items) != 0 || got.Next != "" ||
			!slices.Equal(l.calls, []string{"WorkspaceOf", "Authorize index.list"}) {
			t.Errorf("the tag %q: %+v, %v after %v; want an empty page and no read", tag, got, err, l.calls)
		}
	}
	if _, err := l.listIndex().Execute(context.Background(), l.nb, app.IndexQuery{}); !errors.Is(err, shared.Unauthenticated()) {
		t.Errorf("without an actor: %v", err)
	}
}

// index lists the pages the index holds, those with a type first by its
// key and id, then the others by id, a page at a time: each with its
// path, type, summary and tags cut to 256 bytes and named in cut, its last
// change, its inbound links and its parent page's path. A page deleted
// since it was read is left out, and the next cursor goes past it; so is
// one whose index rows went between the reads, Old, first by its type
// (M9/P4 design 3.9.3; review B-N3; S2 review C-n1).
func TestListIndex(t *testing.T) {
	l := newIndexLib()
	got, err := l.listIndex().Execute(reader(), l.nb, app.IndexQuery{})
	if err != nil {
		t.Fatal(err)
	}
	all := l.ordered(l.projects, l.roadmap, l.spec, l.notes, l.inbox)
	if !slices.Equal(idsOf(got), all) || got.Next != "" {
		t.Errorf("index = %v, next %q; want %v", idsOf(got), got.Next, all)
	}
	if got.Items[0].PageID != l.projects || got.Items[3].Type != nil || got.Items[4].Type != nil {
		t.Errorf("the typed first: %+v", got.Items)
	}
	str := func(s string) *string { return &s }
	cursor := func(c domain.IndexCursor) string {
		s, err := shared.EncodeCursor(c)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	for _, it := range got.Items {
		var want app.IndexItem
		switch it.PageID {
		case l.roadmap:
			want = app.IndexItem{PageID: l.roadmap, Path: "Projects/Roadmap.md", Type: str("Project"), Summary: str(strings.Repeat("s", 256)),
				Tags: []string{"plan", "plan/q4"}, Inbound: 2, ParentPath: str("Projects.md"), Cut: []string{"summary"},
				Cursor: cursor(domain.IndexCursor{K: "project", Typed: true, ID: l.roadmap})}
		case l.inbox:
			want = app.IndexItem{PageID: l.inbox, Path: "Inbox.md", Tags: []string{}, Cut: []string{},
				Cursor: cursor(domain.IndexCursor{ID: l.inbox})}
		default:
			continue
		}
		want.ChangedAt = l.paths.changed[it.PageID]
		if !reflect.DeepEqual(it, want) {
			t.Errorf("the item %+v\nwant %+v", it, want)
		}
	}

	// The first page of two reads Old and Projects: Old's rows are gone,
	// so it holds Projects alone, the next cursor after it.
	two := 2
	first, err := l.listIndex().Execute(reader(), l.nb, app.IndexQuery{Limit: &two})
	if want := cursor(domain.IndexCursor{K: "area", Typed: true, ID: l.projects}); err != nil ||
		!slices.Equal(idsOf(first), []uuid.UUID{l.projects}) || first.Next != want {
		t.Errorf("the first page of two: %v, next %q, %v; want Projects alone, next %q", idsOf(first), first.Next, err, want)
	}
	// A page at a time: the page Gone, of type zeta, is the last of the
	// typed and deleted since: the next cursor goes past it.
	var walked []uuid.UUID
	var after *string
	for range 10 {
		p, err := l.listIndex().Execute(reader(), l.nb, app.IndexQuery{Limit: &two, Cursor: after})
		if err != nil {
			t.Fatal(err)
		}
		walked = append(walked, idsOf(p)...)
		if p.Next == "" {
			break
		}
		after = &p.Next
	}
	if !slices.Equal(walked, all) {
		t.Errorf("walked %v, want %v", walked, all)
	}
	// After the first of type project: the other and Gone are read.
	p, err := l.listIndex().Execute(reader(), l.nb, app.IndexQuery{Limit: &two, Cursor: str(got.Items[1].Cursor)})
	if err != nil {
		t.Fatal(err)
	}
	if want := cursor(domain.IndexCursor{K: "zeta", Typed: true, ID: l.gone}); !slices.Equal(idsOf(p), all[2:3]) || p.Next != want {
		t.Errorf("a page whose last read is gone: %v, next %q; want %v and the next after Gone, %q", idsOf(p), p.Next, all[2:3], want)
	}
	// After Gone, the last two exactly fill a page of two: no cursor
	// follows them (S2 review C-m2).
	if p, err := l.listIndex().Execute(reader(), l.nb, app.IndexQuery{Limit: &two, Cursor: &p.Next}); err != nil ||
		!slices.Equal(idsOf(p), l.ordered(l.notes, l.inbox)) || p.Next != "" {
		t.Errorf("after Gone: %v, next %q, %v; want the last two and no next", idsOf(p), p.Next, err)
	}
}

// index's filters: a type exactly, read by the statement; a tag or one
// under it, a leading "#" dropped; the child pages of a page, or of the
// root for "/"; together, the pages each keeps (M9/P4 design 3.9.3;
// review B-M7).
func TestListIndexFilters(t *testing.T) {
	l := newIndexLib()
	s := func(v string) *string { return &v }
	for _, tt := range []struct {
		name string
		q    app.IndexQuery
		want []uuid.UUID
	}{
		{"type", app.IndexQuery{Type: s("project")}, []uuid.UUID{l.spec}},
		{"type, in its case", app.IndexQuery{Type: s("Project")}, []uuid.UUID{l.roadmap}},
		{"tag", app.IndexQuery{Tag: s("plan")}, l.ordered(l.roadmap, l.spec)},
		{"tag with its #", app.IndexQuery{Tag: s("#plan")}, l.ordered(l.roadmap, l.spec)},
		{"a tag under another", app.IndexQuery{Tag: s("plan/q4")}, []uuid.UUID{l.roadmap}},
		{"a tag of no page", app.IndexQuery{Tag: s("none")}, nil},
		{"parent", app.IndexQuery{Parent: s("Projects.md")}, l.ordered(l.roadmap, l.spec, l.notes)},
		{"the root", app.IndexQuery{Parent: s("/")}, l.ordered(l.projects, l.inbox)},
		{"a page without children", app.IndexQuery{Parent: s("Inbox.md")}, nil},
		{"tag and parent", app.IndexQuery{Tag: s("plan"), Parent: s("Projects.md")}, l.ordered(l.roadmap, l.spec)},
		{"type and parent", app.IndexQuery{Type: s("area"), Parent: s("/")}, []uuid.UUID{l.projects}},
		{"type and tag", app.IndexQuery{Type: s("area"), Tag: s("plan")}, nil},
	} {
		got, err := l.listIndex().Execute(reader(), l.nb, tt.q)
		if err != nil || !slices.Equal(idsOf(got), tt.want) || got.Items == nil {
			t.Errorf("%s: %v, %v; want %v", tt.name, idsOf(got), err, tt.want)
		}
	}
}

// A page's tags are its first 20, each cut to 256 bytes, cut naming each
// and "tags" for more; its type cut to 256 bytes too (M9 design 4.9).
func TestListIndexCutsTheStrings(t *testing.T) {
	l := newIndexLib()
	tags := []string{strings.Repeat("t", 300)}
	for i := range 20 {
		tags = append(tags, fmt.Sprintf("tag%02d", i))
	}
	typ := strings.Repeat("項", 100)
	l.reads.facts[l.notes] = app.IndexFacts{Type: &typ, Tags: tags}
	got, err := l.listIndex().Execute(reader(), l.nb, app.IndexQuery{Parent: new("Projects.md")})
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(got.Items, func(it app.IndexItem) bool { return it.PageID == l.notes })
	if i < 0 {
		t.Fatalf("no Notes in %v", idsOf(got))
	}
	it := got.Items[i]
	if want := append([]string{strings.Repeat("t", 256)}, tags[1:20]...); !slices.Equal(it.Tags, want) {
		t.Errorf("tags %q, want %q", it.Tags, want)
	}
	if want := []string{"type", "tags[0]", "tags"}; !slices.Equal(it.Cut, want) || it.Type == nil || *it.Type != strings.Repeat("項", 85) {
		t.Errorf("cut %q, type %v; want %q and 85 characters", it.Cut, it.Type, want)
	}
}
