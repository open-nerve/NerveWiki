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

// agent is the client of MCP's writes in these tests: domain.MCPClient's
// of an agent named so.
const agent domain.Client = "mcp:agent"

// writePath runs write's use case in notebook as alice from an agent.
func (f *fixture) writePath(notebook uuid.UUID, p app.PathWrite) (app.Written, error) {
	return app.NewWritePath(f.writer(), f.parser(), f.store, f.store, f.store, f.notebooks, f.auth, f.logger()).
		Execute(f.asAlice(), notebook, p, agent)
}

// tokenOf is the version token of the page id at revision, as given.
func tokenOf(id uuid.UUID, revision int) *string {
	return new(domain.VersionToken{Node: id, Revision: revision}.String())
}

// mismatchOf is err's page.revision_mismatch as the path, why and the
// version there; err's code otherwise.
func mismatchOf(err error) string {
	var m *domain.VersionMismatch
	if !errors.As(err, &m) {
		return codeOf(err)
	}
	why := map[domain.Mismatch]string{domain.OtherNode: "other node", domain.Changed: "changed", domain.KeptChanging: "kept changing"}
	return fmt.Sprintf("%s %s %s", m.Path, why[m.Why], m.Current)
}

// inTx reports whether the call ran in a unit's transaction, and not
// before it.
func (f *fixture) inTx(call string) bool {
	return f.called(call+" in tx") && !f.called(call)
}

// took reports whether the parse's budget was taken.
func (f *fixture) took() bool {
	return slices.ContainsFunc(f.rec.calls, func(c string) bool { return strings.HasPrefix(c, "Take ") })
}

// write answers in its order (M9/P2 design 3.8.1): the path's, the
// content's and the token's 422 before the notebook, so that a bad token
// in a notebook not seen is 422; to create, the notebook and page.create's
// 403, then the parse's 503, before the unit; to replace, the notebook,
// the page, then page.write's 403: a reader gets page.not_found for a path
// that names nothing, 403 for a page's.
func TestWritePathAnswersInOrder(t *testing.T) {
	f := newFixture()
	guide := f.page("Guide", nil, 0)
	unseen := uuid.NewV7()
	bad := "x"
	for _, tt := range []struct {
		name     string
		notebook uuid.UUID
		p        app.PathWrite
		want     string
	}{
		{"an empty segment", unseen, app.PathWrite{Path: "Guide//x.md", Base: &bad}, "validation_failed path"},
		{"an attachment's path", unseen, app.PathWrite{Path: "Guide/x.png", Base: &bad}, "validation_failed path"},
		{"a NUL in the content", unseen, app.PathWrite{Path: "Guide.md", Content: "a\x00", Base: &bad}, "validation_failed content"},
		{"a token that is no version", unseen, app.PathWrite{Path: "Guide.md", Base: &bad}, "validation_failed base_version"},
		{"a notebook not seen, to create", unseen, app.PathWrite{Path: "New.md"}, "notebook.not_found"},
		{"a notebook not seen, to replace", unseen, app.PathWrite{Path: "Missing.md", Base: tokenOf(guide.ID, 1)}, "notebook.not_found"},
		{"the zero id", uuid.UUID{}, app.PathWrite{Path: "Guide.md", Base: tokenOf(guide.ID, 1)}, "notebook.not_found"},
		{"no role to create", f.eng, app.PathWrite{Path: "New.md"}, "notebook.not_found"},
	} {
		f.rec.calls = nil
		if _, err := f.writePath(tt.notebook, tt.p); codeOf(err) != tt.want || f.took() {
			t.Errorf("%s: %v after %v, want %s", tt.name, err, f.rec.calls, tt.want)
		}
	}
	f.grant(domain.ActionRead)
	f.auth.forbidden[domain.ActionCreate], f.auth.forbidden[domain.ActionWrite] = true, true
	for _, tt := range []struct {
		name string
		p    app.PathWrite
		want string
	}{
		{"a reader's creation", app.PathWrite{Path: "New.md", Content: "new"}, "forbidden"},
		{"a reader's write of a path that names nothing", app.PathWrite{Path: "Missing.md", Base: tokenOf(guide.ID, 1)}, "page.not_found"},
		{"a reader's write of a page", app.PathWrite{Path: "Guide.md", Content: "new", Base: tokenOf(guide.ID, 1)}, "forbidden"},
	} {
		f.rec.calls = nil
		if _, err := f.writePath(f.eng, tt.p); codeOf(err) != tt.want || f.took() {
			t.Errorf("%s: %v after %v, want %s", tt.name, err, f.rec.calls, tt.want)
		}
	}
	f.auth.forbidden = map[shared.Action]bool{}
	f.grant(domain.ActionCreate, domain.ActionWrite)
	busy := errors.New("the budget is spent")
	f.budget.err = busy
	for _, p := range []app.PathWrite{{Path: "New.md", Content: "new"}, {Path: "Guide.md", Content: "new", Base: tokenOf(guide.ID, 1)}} {
		f.rec.calls = nil
		if _, err := f.writePath(f.eng, p); !errors.Is(err, busy) || f.called("ShareWorkspace in tx") {
			t.Errorf("a write of %s with the budget spent = %v after %v, want its error before the unit", p.Path, err, f.rec.calls)
		}
	}
	f.budget.err = nil
	f.rec.calls = nil
	if _, err := f.writePath(f.eng, app.PathWrite{Path: "Guide.md", Content: "new", Base: tokenOf(guide.ID, 1)}); err != nil ||
		!slices.Equal(f.rec.calls[:6], []string{"WorkspaceOf", "Authorize page.read", "RootByKey", "WorkspaceOf", "Authorize page.write", "Take 3"}) ||
		!f.called("RootByKey in tx") {
		t.Errorf("a write made %v, %v; want the notebook, the page, the decision, the parse, then the path found again in the unit",
			f.rec.calls, err)
	}
	if _, err := app.NewWritePath(f.writer(), f.parser(), f.store, f.store, f.store, f.notebooks, f.auth, f.logger()).
		Execute(t.Context(), f.eng, app.PathWrite{Path: "Guide.md"}, agent); codeOf(err) != "unauthorized" {
		t.Errorf("no actor: %v", err)
	}
}

// write without a token creates the page with its missing parents in one
// unit, one changeset and one event: the parents empty, the page with the
// content, each answered with its path from the root down; a parent there
// is found by its key and named as it is (M9/P2 design 3.8.1). The path
// is walked in the unit.
func TestWritePathCreatesThePageAndItsParents(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	o := &observer{recorder: f.rec}
	f.observers = []app.PageObserver{o}
	got, err := f.writePath(f.eng, app.PathWrite{Path: "A/B/C.md", Content: "# C\n"})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, c := range got.Created {
		paths = append(paths, c.Path)
	}
	page := got.Created[len(got.Created)-1].Node
	switch {
	case !slices.Equal(paths, []string{"A.md", "A/B.md", "A/B/C.md"}):
		t.Errorf("created %v, want A, then B, then C", paths)
	case *got.Created[1].Node.ParentID != got.Created[0].Node.ID || *page.ParentID != got.Created[1].Node.ID:
		t.Errorf("created %+v, want each under the one before", got.Created)
	case got.Path != "A/B/C.md" || got.Node.ID != page.ID || got.Version != (domain.VersionToken{Node: page.ID, Revision: 1}):
		t.Errorf("write = %s %s %s, want C at its first version", got.Path, got.Node.ID, got.Version)
	case f.store.contents[page.ID].Content != "# C\n" || f.store.contents[got.Created[0].Node.ID].Content != "" ||
		f.store.contents[got.Created[1].Node.ID].Content != "":
		t.Errorf("the contents are %+v, want the page's alone written", f.store.contents)
	case len(f.store.changesets) != 1 || got.Outcome.ChangesetID != f.store.changesets[0].ID || len(o.events) != 1:
		t.Errorf("the write made %d changesets and %d events, want one of each, answered", len(f.store.changesets), len(o.events))
	case !f.inTx("RootByKey") || slices.Index(f.rec.calls, "Take 4") > slices.Index(f.rec.calls, "ShareWorkspace in tx"):
		t.Errorf("calls = %v, want the parse before the unit, the path walked in it", f.rec.calls)
	case !strings.Contains(f.logs.String(), `msg="page created"`) || !strings.Contains(f.logs.String(), "pages=3"):
		t.Errorf("logs = %s, want the creation with its pages", f.logs)
	}

	f = newFixture()
	f.grant(domain.ActionCreate)
	guide := f.page("Guide", nil, 0)
	got, err = f.writePath(f.eng, app.PathWrite{Path: "guide/setup/Intro.md", Content: "intro"})
	if err != nil || got.Path != "Guide/setup/Intro.md" || len(got.Created) != 2 || got.Created[0].Path != "Guide/setup.md" ||
		*got.Created[0].Node.ParentID != guide.ID {
		t.Errorf("a write under Guide = %+v, %v; want setup and Intro created under it, named by Guide's name", got, err)
	}
}

// write without a token at a page there writes nothing when the page
// holds the content already, its version the answer; another content
// there, of another size or the same, is page.title_taken at the path,
// and so is an attachment that holds the name (M9/P2 design 3.8.1, 2.1
// item 11).
func TestWritePathAtAPageThere(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	guide := f.page("Guide", nil, 0)
	f.content(guide.ID, "# Guide\n", 3)
	f.asset("img.png", &guide.ID, 0)
	got, err := f.writePath(f.eng, app.PathWrite{Path: "GUIDE.md", Content: "# Guide\n"})
	if err != nil || got.Path != "Guide.md" || got.Version != (domain.VersionToken{Node: guide.ID, Revision: 3}) ||
		got.Created != nil || got.Outcome.ChangesetID != (uuid.UUID{}) || len(f.store.changesets) != 0 ||
		strings.Contains(f.logs.String(), "page created") {
		t.Errorf("a write of the same content = %+v, %v after %v; want Guide as it is, nothing written", got, err, f.rec.calls)
	}
	for _, tt := range []struct {
		name, path, content string
		asset               bool
	}{
		{"another content", "Guide.md", "other", false},
		// The same size, another hash (M9/P2 S1 review C-M3).
		{"another content of the same size", "Guide.md", "# Guidf\n", false},
		{"an attachment's name", "Guide/img.png.md", "other", true},
	} {
		_, err := f.writePath(f.eng, app.PathWrite{Path: tt.path, Content: tt.content})
		var taken *domain.PathTaken
		if codeOf(err) != "page.title_taken" || !errors.As(err, &taken) || taken.Path != tt.path || taken.Asset != tt.asset {
			t.Errorf("%s: write = %v, want page.title_taken at %s, an attachment's %v", tt.name, err, tt.path, tt.asset)
		}
	}
	if len(f.store.changesets) != 0 {
		t.Errorf("the refusals wrote %d changesets", len(f.store.changesets))
	}
}

// Each step's rules refuse in the unit before anything is written: a
// missing ancestor ending with .md, a found one so titled being created
// under; an attachment on the way; a path of more pages than MaxDepth
// (M9/P2 design 3.8.1).
func TestWritePathRefusesWhatItCannotCreate(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	guide := f.page("Guide", nil, 0)
	f.asset("img.png", &guide.ID, 0)
	for _, tt := range []struct{ path, want string }{
		{"Notes.md/Child.md", "validation_failed path not_allowed"},
		{"Guide/img.png/Child.md", "validation_failed path not_allowed"},
		{"A/B/C/D/E/F/G/H/I/J/K.md", "page.too_deep"},
	} {
		if _, err := f.writePath(f.eng, app.PathWrite{Path: tt.path, Content: "x"}); fieldProblem(err) != tt.want ||
			f.called("CreateNode in tx") {
			t.Errorf("write %s = %v after %v, want %s before any write", tt.path, err, f.rec.calls, tt.want)
		}
	}
	notes := f.page("Notes.md", nil, 1)
	got, err := f.writePath(f.eng, app.PathWrite{Path: "Notes.md/Child.md", Content: "x"})
	if err != nil || got.Path != "Notes.md/Child.md" || len(got.Created) != 1 || *got.Node.ParentID != notes.ID {
		t.Errorf("a write under a page titled Notes.md = %+v, %v; want Child created under it", got, err)
	}
	ten := "A/B/C/D/E/F/G/H/I/J.md"
	if got, err := f.writePath(f.eng, app.PathWrite{Path: ten, Content: "x"}); err != nil || len(got.Created) != 10 {
		t.Errorf("a write of %s = %+v, %v; want ten pages", ten, got, err)
	}
}

// write with a token writes the page its path names under the lock, which
// must be the token's, on the token's revision: another page there, a
// node deleted or of another notebook among them, is 409 of another node;
// a revision written since 409 changed, with the version there; a content
// the page holds already is nothing written, whatever the revision
// (M9/P2 design 3.8.1).
func TestWritePathReplacesOnItsToken(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content string
		base    func(guide uuid.UUID) *string
		want    string
	}{
		{"another page's token", "new", func(uuid.UUID) *string { return tokenOf(uuid.NewV7(), 3) }, "guide.md other node %s:3"},
		{"a token of a revision before", "new", func(g uuid.UUID) *string { return tokenOf(g, 2) }, "guide.md changed %s:3"},
		{"a token of a revision not yet", "new", func(g uuid.UUID) *string { return tokenOf(g, 4) }, "guide.md changed %s:3"},
		{"the content there, on a revision before", "old", func(g uuid.UUID) *string { return tokenOf(g, 2) }, "unchanged 3"},
		{"the content there, on its revision", "old", func(g uuid.UUID) *string { return tokenOf(g, 3) }, "unchanged 3"},
		{"its version", "new", func(g uuid.UUID) *string { return tokenOf(g, 3) }, "written 4"},
	} {
		f := newFixture()
		f.grant(domain.ActionRead, domain.ActionWrite)
		guide := f.page("Guide", nil, 0)
		f.content(guide.ID, "old", 3)
		got, err := f.writePath(f.eng, app.PathWrite{Path: "guide.md", Content: tt.content, Base: tt.base(guide.ID)})
		answer := mismatchOf(err)
		switch {
		case err != nil:
		case got.Outcome.ChangesetID == (uuid.UUID{}):
			answer = fmt.Sprintf("unchanged %d", got.Version.Revision)
		default:
			answer = fmt.Sprintf("written %d", got.Version.Revision)
		}
		want := tt.want
		if strings.Contains(want, "%s") {
			want = fmt.Sprintf(want, guide.ID)
		}
		if answer != want {
			t.Errorf("%s: write = %q, want %q", tt.name, answer, want)
		}
		if err == nil && (got.Path != "Guide.md" || got.Node.ID != guide.ID || got.Version.Node != guide.ID || got.Created != nil) {
			t.Errorf("%s: write = %+v, want Guide's version", tt.name, got)
		}
		if logged := strings.Contains(f.logs.String(), "page content written"); logged != strings.HasPrefix(want, "written") ||
			logged && !strings.Contains(f.logs.String(), "revision=4") {
			t.Errorf("%s: logs = %s", tt.name, f.logs)
		}
	}
}
