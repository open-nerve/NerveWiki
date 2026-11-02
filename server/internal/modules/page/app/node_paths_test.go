package app_test

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Walk finds a path's keys from the root, a lookup a segment, up to the
// first that names nothing (M9/P2 design 3.7): all of them, some, none; an
// attachment before the last segment ends the walk, its segment told, the
// nodes ending with it; an attachment at the last segment is found, its
// kind its caller's to judge.
func TestWalkFindsThePathsNodesFromTheRoot(t *testing.T) {
	f := newFixture()
	a := f.page("A", nil, 0)
	b := f.page("Straße", &a.ID, 0)
	f.asset("img.png", &a.ID, 1)
	f.page("C", &b.ID, 0)
	paths := app.NewNodePaths(f.store, f.notebooks)
	for _, tt := range []struct {
		path string
		want string // the names found, and the segment of an attachment on the way
	}{
		{"A/STRASSE/C.md", "A/Straße/C"},
		{"a/strasse/missing/D.md", "A/Straße"},
		{"Missing/B.md", ""},
		{"A/img.png/C.md", "A/img.png on 2"},
		{"A/img.png/missing/x.md", "A/img.png on 2"},
		{"A/missing/img.png/y.md", "A"},
		{"A/img.png", "A/img.png"},
	} {
		p, err := domain.ParsePath("path", tt.path, false)
		if err != nil {
			t.Fatal(err)
		}
		found, err := paths.Walk(f.asAlice(), f.eng, p.Keys)
		names := make([]string, len(found))
		for i, n := range found {
			names[i] = n.Name
		}
		got := strings.Join(names, "/")
		var on *domain.AttachmentOnPath
		switch {
		case errors.As(err, &on):
			got += fmt.Sprintf(" on %d", on.Segment)
		case err != nil:
			got += " " + err.Error()
		}
		if got != tt.want {
			t.Errorf("Walk(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
	if found, err := paths.Walk(f.asAlice(), uuid.NewV7(), []string{"a"}); !errors.Is(err, domain.ErrNotebookNotFound) || len(found) != 0 {
		t.Errorf("Walk in another notebook = %v, %v; want notebook.not_found", found, err)
	}
}

// Paths writes the paths of the notebook's nodes among ids, by id, with
// PathStore's PathsOf; a node of none has none.
func TestPathsWritesThePathsOfTheNodes(t *testing.T) {
	f := newFixture()
	a := f.page("A", nil, 0)
	img := f.asset("img.png", &a.ID, 0)
	got, err := app.NewNodePaths(f.store, f.notebooks).Paths(f.asAlice(), f.eng, []uuid.UUID{a.ID, img.ID, uuid.NewV7()})
	if want := map[uuid.UUID]string{a.ID: "A.md", img.ID: "A/img.png"}; err != nil || !maps.Equal(got, want) {
		t.Errorf("Paths = %v, %v; want %v", got, err, want)
	}
	f.store.errs = map[string]error{"PathsOf": shared.ServerBusy(0)}
	if _, err := app.NewNodePaths(f.store, f.notebooks).Paths(f.asAlice(), f.eng, []uuid.UUID{a.ID}); !errors.Is(err, shared.ServerBusy(0)) {
		t.Errorf("Paths when the store fails = %v, want its error", err)
	}
}

// Both point reads and a write's walk refuse a reserved root key before
// asking for a real node; the same names below a real page remain usable.
func TestVirtualPathLookup(t *testing.T) {
	f := newFixture()
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki}}
	real := f.page("Real", nil, 0)
	for _, name := range []string{"index", "log", "index.md", "log.md"} {
		f.page(name, &real.ID, 0)
	}
	paths := app.NewNodePaths(f.store, f.notebooks)
	for _, raw := range []string{"index", "log", "index.md", "log.md", "index.md.md", "log.md.md", "index/Child.md", "log.md/Child.md"} {
		path, err := domain.ParsePath("path", raw, false)
		if err != nil {
			t.Fatal(err)
		}
		f.rec.calls = nil
		if _, err := paths.Find(f.asAlice(), f.eng, path); !errors.Is(err, domain.ErrVirtualFile) {
			t.Errorf("Find(%q) = %v, want page.virtual_file", raw, err)
		}
		if !slices.Equal(f.rec.calls, []string{"StateOf"}) {
			t.Errorf("virtual Find read entity store: %v", f.rec.calls)
		}
		f.rec.calls = nil
		if _, err := paths.Walk(f.asAlice(), f.eng, path.Keys); !errors.Is(err, domain.ErrVirtualFile) {
			t.Errorf("Walk(%q) = %v, want page.virtual_file", raw, err)
		}
	}
	for _, raw := range []string{"Real/index.md", "Real/log.md", "Real/index.md.md", "Real/log.md.md"} {
		path, err := domain.ParsePath("path", raw, false)
		if err != nil {
			t.Fatal(err)
		}
		if found, err := paths.Find(f.asAlice(), f.eng, path); err != nil || found.Node.ParentID == nil || *found.Node.ParentID != real.ID {
			t.Errorf("nested Find(%q) = %+v, %v; want the real child", raw, found, err)
		}
	}
	f.notebooks.states[f.eng] = domain.NotebookState{Mode: domain.ModeStandard}
	for _, name := range []string{"index", "log", "index.md", "log.md"} {
		real := f.page(name, nil, 0)
		path, err := domain.ParsePath("path", name+".md", false)
		if err != nil {
			t.Fatal(err)
		}
		found, err := paths.Find(f.asAlice(), f.eng, path)
		if err != nil || found.Node.ID != real.ID {
			t.Errorf("standard %q: %+v %v", name, found, err)
		}
	}

}
