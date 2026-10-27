package app_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// lister is list over the fixture, its glob's work at most maxWork.
func (f *fixture) lister(maxWork int64) *app.ListByPattern {
	return app.NewListByPattern(f.notebooks, f.store, f.store, fakeAttachments{recorder: f.rec, infos: f.infos}, f.auth).
		WithMaxWork(maxWork)
}

// listed are a listing's items' paths.
func listed(l app.Listing) []string {
	out := make([]string, len(l.Items))
	for i, item := range l.Items {
		out[i] = item.Path
	}
	return out
}

// guideTree is the fixture's notebook with Guide at the root and its
// children, made in this order, their orders neither their names' nor
// their ids': Wide first, then Setup, Alpha, Long; an attachment between
// Setup and Alpha; Setup has a child.
func (f *fixture) guideTree() (guide, setup, png domain.Node) {
	guide = f.page("Guide", nil, 0)
	setup = f.page("Setup", &guide.ID, 1)
	f.page("Alpha", &guide.ID, 3)
	f.page("Long", &guide.ID, 4)
	f.page("Wide", &guide.ID, 0.5)
	png = f.asset("diagram.png", &guide.ID, 2)
	f.infos = map[uuid.UUID]app.AttachmentInfo{png.ID: {MIME: "image/png", Bytes: 345}}
	f.page("Deep", &setup.ID, 0)
	return guide, setup, png
}

// A path ending with "/" lists a page's children, the root's for none, in
// the tree's order, sort order then id, as the web's tree; a glob lists in
// path-key order, ignoring case. Each item has its path, kind, children,
// a page's version and size, an attachment's type and size (M9/P1 design
// 3.10.2, 6.3).
func TestListByPatternAnswersInOrder(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionList)
	_, setup, png := f.guideTree()
	f.content(setup.ID, "twelve bytes", 3)
	list := f.lister(domain.MaxGlobWork)
	for _, tt := range []struct{ pattern, want string }{
		{"", "Guide.md"},
		{"/", "Guide.md"},
		{"Guide/", "Guide/Wide.md Guide/Setup.md Guide/diagram.png Guide/Alpha.md Guide/Long.md"},
		{"/guide/", "Guide/Wide.md Guide/Setup.md Guide/diagram.png Guide/Alpha.md Guide/Long.md"},
		{"Guide/Setup/", "Guide/Setup/Deep.md"},
		{"**/*.md", "Guide.md Guide/Alpha.md Guide/Long.md Guide/Setup.md Guide/Setup/Deep.md Guide/Wide.md"},
		{"guide/*.PNG", "Guide/diagram.png"},
		{"Guide/**", "Guide/Alpha.md Guide/diagram.png Guide/Long.md Guide/Setup.md Guide/Setup/Deep.md Guide/Wide.md"}, // by the keys
		{"Guide/Setup.md", "Guide/Setup.md"},
		{"**/Deep.md", "Guide/Setup/Deep.md"},
		{"Nothing/**", ""},
	} {
		got, err := list.Execute(f.asAlice(), f.eng, tt.pattern, nil, nil)
		if err != nil || strings.Join(listed(got), " ") != tt.want || got.Next != "" {
			t.Errorf("list(%q) = %q, %q, %v; want %q", tt.pattern, listed(got), got.Next, err, tt.want)
		}
	}
	got, err := list.Execute(f.asAlice(), f.eng, "Guide/", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]app.Listed{}
	for _, item := range got.Items {
		byPath[item.Path] = item
	}
	switch s, p := byPath["Guide/Setup.md"], byPath["Guide/diagram.png"]; {
	case !s.HasChildren || s.Page == nil || s.Page.Revision != 3 || s.Page.Bytes != 12 || s.Attachment != nil:
		t.Errorf("Setup is %+v", s)
	case p.HasChildren || p.Page != nil || p.Attachment == nil || (p.Attachment.MIME != "image/png" || p.Attachment.Bytes != 345 || len(p.Attachment.SHA256) != 0) ||
		p.Node.ID != png.ID:
		t.Errorf("the attachment is %+v", p)
	case byPath["Guide/Alpha.md"].HasChildren:
		t.Errorf("Alpha has children")
	}
	// A page whose only child is an attachment has children: list
	// "Guide/Alpha/" finds it (M9/P1 S2 review, C-M1).
	alpha := byPath["Guide/Alpha.md"].Node.ID
	f.infos[f.asset("table.csv", &alpha, 0).ID] = app.AttachmentInfo{MIME: "text/csv", Bytes: 9}
	got, err = list.Execute(f.asAlice(), f.eng, "Guide/", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range got.Items {
		if want := item.Path == "Guide/Setup.md" || item.Path == "Guide/Alpha.md"; item.HasChildren != want {
			t.Errorf("%s has children: %v, want %v", item.Path, item.HasChildren, want)
		}
	}
}

// A page of a list goes on from its cursor: neither an item repeated nor
// one skipped, for children and for a glob alike; the last page has no
// cursor (M9/P1 design 3.10.2).
func TestListByPatternPagesOnFromItsCursor(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionList)
	f.guideTree()
	// Siblings of one sort order, which a restore or a revert gives back
	// while a page placed meanwhile holds it (M9/P1 S2 review, A-M1): by
	// id after the order, the ids neither the names' order nor the order
	// they were stored in: Y's id is the least, stored last.
	least := uuid.NewV7()
	tied := f.page("Tied", nil, 1)
	f.page("A", &tied.ID, 1)
	f.page("Z", &tied.ID, 2)
	f.page("X", &tied.ID, 2)
	y := f.page("Y", &tied.ID, 2)
	f.page("B", &tied.ID, 3)
	delete(f.store.nodes, y.ID)
	content := f.store.contents[y.ID]
	delete(f.store.contents, y.ID)
	y.ID, content.NodeID = least, least
	f.store.nodes[least], f.store.contents[least] = y, content
	list := f.lister(domain.MaxGlobWork)
	for pattern, want := range map[string]string{
		"Guide/": "Guide/Wide.md Guide/Setup.md Guide/diagram.png Guide/Alpha.md Guide/Long.md",
		"Tied/":  "Tied/A.md Tied/Y.md Tied/Z.md Tied/X.md Tied/B.md",
		"**": "Guide.md Guide/Alpha.md Guide/diagram.png Guide/Long.md Guide/Setup.md Guide/Setup/Deep.md Guide/Wide.md " +
			"Tied.md Tied/A.md Tied/B.md Tied/X.md Tied/Y.md Tied/Z.md",
		"Guide/*": "Guide/Alpha.md Guide/diagram.png Guide/Long.md Guide/Setup.md Guide/Wide.md",
	} {
		for _, size := range []int{1, 2, 3} {
			var got []string
			var cursor *string
			for range 20 {
				l, err := list.Execute(f.asAlice(), f.eng, pattern, &size, cursor)
				if err != nil {
					t.Fatalf("list(%q, %d): %v", pattern, size, err)
				}
				if len(l.Items) > size {
					t.Fatalf("list(%q, %d) answered %d items", pattern, size, len(l.Items))
				}
				got = append(got, listed(l)...)
				if l.Next == "" {
					break
				}
				if l.Next != l.Items[len(l.Items)-1].Cursor {
					t.Errorf("list(%q, %d): the page's cursor is not its last item's", pattern, size)
				}
				cursor = &l.Next
			}
			if strings.Join(got, " ") != want {
				t.Errorf("list(%q) by %d = %q, want %q", pattern, size, got, want)
			}
		}
	}
}

// The two cursors are two payloads, each decoded into the one the pattern
// lists by: a children's cursor is no glob's, nor the other way round, nor
// is {} either: 400 bad_request on cursor (M9/P1 design 3.10.2, 6.3).
func TestTheListCursorsRefuseTheOtherShape(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionList)
	f.guideTree()
	list := f.lister(domain.MaxGlobWork)
	one := 1
	children, err := list.Execute(f.asAlice(), f.eng, "Guide/", &one, nil)
	if err != nil || children.Next == "" {
		t.Fatal(children, err)
	}
	glob, err := list.Execute(f.asAlice(), f.eng, "**", &one, nil)
	if err != nil || glob.Next == "" {
		t.Fatal(glob, err)
	}
	empty, err := shared.EncodeCursor(struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ pattern, cursor string }{
		{"Guide/", glob.Next}, {"**", children.Next}, {"Guide/", empty}, {"**", empty}, {"**", "nope"},
	} {
		if _, err := list.Execute(f.asAlice(), f.eng, tt.pattern, nil, &tt.cursor); codeOf(err) != "bad_request" {
			t.Errorf("list(%q) with another's cursor: %v", tt.pattern, err)
		}
	}
}

// list's problems come in their order: the pattern's 422, the cursor's
// 400, the limit's 422, the notebook's 404, which the zero id is without a
// read, then the page of a path ending with "/" (M9/P1 design 3.10.4).
func TestListByPatternAnswersInItsOrder(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionList)
	guide, _, _ := f.guideTree()
	list := f.lister(domain.MaxGlobWork)
	zero := 0
	bad := "nope"
	unseen := uuid.NewV7()
	for _, tt := range []struct {
		name     string
		notebook uuid.UUID
		pattern  string
		cursor   *string
		limit    *int
		want     string
	}{
		{"a pattern no glob", unseen, "a**b", &bad, &zero, "validation_failed pattern"},
		{"a pattern with a NUL", unseen, "a\x00b", &bad, &zero, "validation_failed pattern"},
		{"a path with a NUL", unseen, "a\x00b/", &bad, &zero, "validation_failed pattern"},
		{"a cursor", unseen, "**", &bad, &zero, "bad_request"},
		{"a limit", unseen, "**", nil, &zero, "validation_failed limit"},
		{"a notebook not seen", unseen, "**", nil, nil, "notebook.not_found"},
		{"no such page", f.eng, "Missing/", nil, nil, "page.not_found"},
		{"an attachment's children", f.eng, "Guide/diagram.png/", nil, nil, "page.not_found"},
	} {
		if _, err := list.Execute(f.asAlice(), tt.notebook, tt.pattern, tt.limit, tt.cursor); codeOf(err) != tt.want {
			t.Errorf("%s: %v, want %s", tt.name, err, tt.want)
		}
	}
	f.rec.calls = nil
	if _, err := list.Execute(f.asAlice(), uuid.UUID{}, "**", nil, nil); codeOf(err) != "notebook.not_found" || len(f.rec.calls) != 0 {
		t.Errorf("the zero id: %v after %v", err, f.rec.calls)
	}
	f.rec.calls = nil
	if _, err := list.Execute(f.asAlice(), f.eng, "Guide/", nil, nil); err != nil ||
		!slices.Equal(f.rec.calls, []string{"WorkspaceOf", "Authorize node.list", "ListNodes", "PageStates 4", "AttachmentInfos 1",
			"AliveNodes 1"}) {
		t.Errorf("list read %v, %v; want the decision, the tree, then the page's states and infos", f.rec.calls, err)
	}
	f.asset("Notes", &guide.ID, 5)
	for pattern, hint := range map[string]string{
		"Guide/diagram.png/": `That is an attachment: its path is "Guide/diagram.png".`,
		"Guide.md/":          `Guide.md is a page; list "Guide/" for its children.`,
		"Guide/Setup.MD/":    `Guide/Setup.md is a page; list "Guide/Setup/" for its children.`,
		"Guide/notes.md/":    `That is an attachment: its path is "Guide/Notes".`,
		"Guide/Alpha/Gone/":  "",
		"Missing.md/":        "",
	} {
		_, err := list.Execute(f.asAlice(), f.eng, pattern, nil, nil)
		var missing *domain.PathNotFound
		if !errors.As(err, &missing) || missing.Path != pattern || missing.Hint != hint {
			t.Errorf("list(%q): %v, %+v; want the hint %q", pattern, err, missing, hint)
		}
	}
}

// A glob without wildcards that names nothing, while it names a page with
// ".md", comes with a hint: list the page's children with "/" (M9/P1
// design 3.10.2); one that names something, or a glob, comes without.
func TestAListOfAPageWithoutItsMdHints(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionList)
	f.guideTree()
	list := f.lister(domain.MaxGlobWork)
	for pattern, want := range map[string]string{
		"Guide":           `Guide.md is a page; list "Guide/" for its children.`,
		"guide/setup":     `Guide/Setup.md is a page; list "Guide/Setup/" for its children.`,
		"Guide.md":        "",
		"Guide/missing":   "",
		"Gui?e":           "",
		"Guide/diagram.p": "",
	} {
		got, err := list.Execute(f.asAlice(), f.eng, pattern, nil, nil)
		if err != nil || got.Hint != want {
			t.Errorf("list(%q) hinted %q, %v; want %q", pattern, got.Hint, err, want)
		}
	}
}

// A node deleted between the tree's read and its state's is left out: a
// page by its content, an attachment by its node, whose info asset gives
// deleted or not (M9/P1 design 3.10.3).
func TestAListLeavesOutWhatWentMeanwhile(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionList)
	_, setup, png := f.guideTree()
	f.rec.before = map[string]func(){
		"PageStates 4": func() { delete(f.store.contents, setup.ID) },
		"AliveNodes 1": func() { delete(f.store.nodes, png.ID) },
	}
	got, err := f.lister(domain.MaxGlobWork).Execute(f.asAlice(), f.eng, "Guide/", nil, nil)
	if err != nil || strings.Join(listed(got), " ") != "Guide/Wide.md Guide/Alpha.md Guide/Long.md" {
		t.Errorf("list = %q, %v", listed(got), err)
	}
}

// A costly glob over a tree of 200 nodes stops at its bound of work with
// no item and a cursor; resumed call after call, it lists what one walk
// without a bound lists, nothing repeated or skipped, each cursor strictly
// after the last, in at most as many calls as nodes (M9/P1 design 3.13).
func TestTheGlobsWorkBoundPagesTheList(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionList)
	var parents []*uuid.UUID
	parents = append(parents, nil)
	for i := range 200 {
		parent := parents[i%len(parents)]
		name := fmt.Sprintf("n%03dx%cy%s", i, 'a'+rune(i%26), strings.Repeat("z", i%7))
		if i%9 == 0 {
			name = fmt.Sprintf("n%03d", i)
		}
		n := f.page(name, parent, float64(i))
		if i%3 == 0 {
			parents = append(parents, &n.ID)
		}
	}
	const pattern = "**/*x?y*/**"
	whole, err := f.lister(1<<62).Execute(f.asAlice(), f.eng, pattern, new(domain.MaxListLimit), nil)
	if err != nil || len(whole.Items) == 0 || whole.Next != "" {
		t.Fatalf("the whole list = %d items, %q, %v", len(whole.Items), whole.Next, err)
	}
	list := f.lister(2 * domain.GlobEntryWork)
	first, err := list.Execute(f.asAlice(), f.eng, pattern, nil, nil)
	if err != nil || first.Next == "" {
		t.Fatalf("the first call = %q, %q, %v", listed(first), first.Next, err)
	}
	var got []string
	var cursor *string
	last := ""
	calls := 0
	for {
		if calls++; calls > 201 {
			t.Fatal("the list does not end within as many calls as nodes, and one")
		}
		l, err := list.Execute(f.asAlice(), f.eng, pattern, nil, cursor)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, listed(l)...)
		if l.Next == "" {
			break
		}
		var k domain.GlobCursor
		if err := shared.DecodeCursor(l.Next, &k); err != nil || k.K <= last {
			t.Fatalf("the cursor %q is not after %q: %v", k.K, last, err)
		}
		last, cursor = k.K, &l.Next
	}
	if !slices.Equal(got, listed(whole)) {
		t.Errorf("resumed in %d calls: %q\nwhole: %q", calls, got, listed(whole))
	}
	if len(first.Items) != 0 {
		t.Errorf("the first call answered %d items, want none: its work ran out first", len(first.Items))
	}
}
