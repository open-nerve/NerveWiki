package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// textMeasure counts a byte one, and leaves a slice room bytes.
func textMeasure(room int) app.TextMeasure {
	return app.TextMeasure{Bytes: func(s string) int { return len(s) }, Room: func(string) int { return room }}
}

// findOrProblem is a found node's path, or err's code and hint.
func findOrProblem(found app.Found, err error) string {
	var missing *domain.PathNotFound
	switch {
	case errors.As(err, &missing):
		return codeOf(err) + " " + missing.Path + " " + missing.Hint
	case err != nil:
		return err.Error()
	}
	return domain.PathOf(found.Ancestors, found.Node)
}

// A path finds its node a segment a lookup, by title key; its last
// segment's kind is the node's: an attachment's path finding a page, or a
// page's an attachment, is not found with a hint of the node's path; so is
// a path through an attachment, which has no children (M9 design 4.8, the
// four cases besides the links' rules).
func TestFindByPath(t *testing.T) {
	f := newFixture()
	a := f.page("A", nil, 0)
	f.page("x.png", &a.ID, 0)
	f.asset("x", &a.ID, 1)
	png := f.asset("diagram.png", &a.ID, 2)
	f.page("x.md", &a.ID, 3)
	f.page("Straße", &a.ID, 4)
	_ = png
	paths := app.NewNodePaths(f.store)
	for _, tt := range []struct{ path, want string }{
		{"A.md", "A.md"},
		{"a/STRASSE.md", "A/Straße.md"},
		{"A/x.png.md", "A/x.png.md"},
		{"A/x.md.md", "A/x.md.md"},
		{"A/x", "A/x"},
		{"A/diagram.png", "A/diagram.png"},
		{"A/x.png", `page.not_found A/x.png A page's path ends with .md: "A/x.png.md".`},
		{"A/x.md", `page.not_found A/x.md That is an attachment: its path is "A/x".`},
		{"A", `page.not_found A A page's path ends with .md: "A.md".`},
		{"A/diagram.png/B.md", "page.not_found A/diagram.png/B.md "},
		{"A/missing.md", "page.not_found A/missing.md "},
		{"/B/A.md", "page.not_found /B/A.md "},
	} {
		p, err := domain.ParsePath("path", tt.path, false)
		if err != nil {
			t.Fatal(err)
		}
		if got := findOrProblem(paths.Find(f.asAlice(), f.eng, p)); got != tt.want {
			t.Errorf("Find(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
	f.rec.calls = nil
	p, _ := domain.ParsePath("path", "A/x.png.md", false)
	if _, err := paths.Find(context.Background(), f.eng, p); err != nil || !slices.Equal(f.rec.calls, []string{"RootByKey", "ChildByKey"}) {
		t.Errorf("Find made %v, %v; want a lookup a segment", f.rec.calls, err)
	}
}

// read decides, finds and cuts in its order: the path's 422 before the
// window's, both before the notebook; the notebook before the node; the
// column on its line, which only the content tells, after the 404s
// (M9/P1 design 3.11.1, 6.3).
func TestReadPathAnswersInOrder(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	guide := f.page("Guide", nil, 0)
	f.content(guide.ID, "é\nline two\n", 3)
	read := app.NewReadPath(f.notebooks, f.store, f.store, f.auth)
	one, minus := 1, -1
	unseen := uuid.NewV7()
	for _, tt := range []struct {
		name     string
		notebook uuid.UUID
		path     string
		column   *int
		want     string
	}{
		{"a path ending with /", unseen, "Guide/", &minus, "validation_failed path"},
		{"a negative column", unseen, "Guide.md", &minus, "validation_failed column"},
		{"a notebook not seen", unseen, "Guide.md", &one, "notebook.not_found"},
		{"the zero id", uuid.UUID{}, "Guide.md", &one, "notebook.not_found"},
		{"no such page", f.eng, "Missing.md", &one, "page.not_found"},
		{"a column inside a character", f.eng, "Guide.md", &one, "validation_failed column"},
	} {
		if _, err := read.Execute(f.asAlice(), tt.notebook, tt.path, nil, tt.column, nil, textMeasure(100)); codeOf(err) != tt.want {
			t.Errorf("%s: %v, want %s", tt.name, err, tt.want)
		}
	}
	f.rec.calls = nil
	if _, err := read.Execute(f.asAlice(), uuid.UUID{}, "Guide.md", nil, nil, nil, textMeasure(100)); len(f.rec.calls) != 0 || err == nil {
		t.Errorf("the zero id read %v", f.rec.calls)
	}
	delete(f.auth.grants, domain.ActionRead)
	if _, err := read.Execute(f.asAlice(), f.eng, "Guide.md", nil, nil, nil, textMeasure(100)); codeOf(err) != "notebook.not_found" {
		t.Errorf("a notebook the caller may not read: %v", err)
	}
	if _, err := read.Execute(context.Background(), f.eng, "Guide.md", nil, nil, nil, textMeasure(100)); codeOf(err) != "unauthorized" {
		t.Errorf("no actor: %v", err)
	}
}

// read gives a page's version token, the window asked for and the slice of
// its content within the room the answer leaves; an attachment its node
// alone; a page whose content went since its node was read is not found.
func TestReadPathReadsAPageOrFindsAnAttachment(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	guide := f.page("Guide", nil, 0)
	setup := f.page("Setup", &guide.ID, 0)
	f.content(setup.ID, "one\ntwo\nthree\n", 4)
	png := f.asset("diagram.png", &guide.ID, 1)
	read := app.NewReadPath(f.notebooks, f.store, f.store, f.auth)
	two := 2
	got, err := read.Execute(f.asAlice(), f.eng, "/guide/SETUP.md", &two, nil, nil, textMeasure(4))
	switch {
	case err != nil:
		t.Fatal(err)
	case got.Path != "Guide/Setup.md" || got.Node.ID != setup.ID || got.Page == nil:
		t.Fatalf("read %+v", got)
	case got.Page.Version != (domain.VersionToken{Node: setup.ID, Revision: 4}) || got.Page.Bytes != 14:
		t.Errorf("the page's version and size are %+v, %d", got.Page.Version, got.Page.Bytes)
	case got.Page.Slice != (domain.Slice{Text: "two\n", Lines: 1, TotalLines: 3, NextOffset: 3}):
		t.Errorf("the slice is %+v", got.Page.Slice)
	case got.Page.Window != (domain.Window{Offset: 2, Column: 0, Limit: domain.DefaultReadLines}):
		t.Errorf("the window is %+v", got.Page.Window)
	}
	at, err := read.Execute(f.asAlice(), f.eng, "Guide/diagram.png", nil, nil, nil, textMeasure(4))
	if err != nil || at.Page != nil || at.Node.ID != png.ID || at.Path != "Guide/diagram.png" {
		t.Errorf("the attachment read %+v, %v", at, err)
	}
	delete(f.store.contents, setup.ID)
	if _, err := read.Execute(f.asAlice(), f.eng, "Guide/Setup.md", nil, nil, nil, textMeasure(4)); codeOf(err) != "page.not_found" {
		t.Errorf("a page without its content: %v", err)
	}
	if !strings.Contains(strings.Join(f.rec.calls, " "), "Authorize page.read") {
		t.Errorf("read decided %v, want page.read", f.rec.calls)
	}
}
