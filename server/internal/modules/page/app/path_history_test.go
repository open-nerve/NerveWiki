package app_test

import (
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The history's reads by path and in a notebook, MCP's versions, log and
// changeset (M9/P2 design 3.16, 3.17.4): each lists what REST's read of
// the same node or changeset does, each item with the cursor that goes on
// after it; each entry answers its codes in MCP's order, a fault answered
// before every later one.

// pathVersionReads are the versions' reads over the fixture, by path too.
func (f *fixture) pathVersionReads() app.VersionReads {
	r := f.versionReads()
	r.Paths, r.States = f.store, f.store
	return r
}

// pathHistoryReads are the log's reads over the fixture, by path too.
func (f *fixture) pathHistoryReads() app.HistoryReads {
	r := f.historyReads()
	r.Paths = f.store
	return r
}

// decoded is the payload cursor holds.
func decoded[T any](t *testing.T, cursor string) T {
	t.Helper()
	var p T
	if err := shared.DecodeCursor(cursor, &p); err != nil {
		t.Fatalf("the cursor %q: %v", cursor, err)
	}
	return p
}

// versions lists the versions of the page at its path, a page at a time,
// as Execute lists them by its id: the same versions, the same next
// cursor; each version's cursor goes on after it, the last one's the
// next page's; the page's version now; its path by the names found. The
// page is found a segment a lookup and decided on its notebook, outside a
// transaction.
func TestListVersionsAtPathListsAsExecute(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionHistory)
	f.names[f.alice] = "Alice"
	guide := f.page("Guide", nil, 0)
	n := f.page("Notes", &guide.ID, 0)
	f.history(n.ID, 5)
	list, two := app.NewListVersions(f.pathVersionReads()), 2
	var cursor *string
	for page := 0; ; page++ {
		f.rec.calls = nil
		got, err := list.AtPath(f.asAlice(), f.eng, "guide/NOTES.md", &two, cursor)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"WorkspaceOf", "Authorize page.history", "RootByKey", "ChildByKey", "ListVersions", "PageStates 1"}
		if page == 0 {
			want = slices.Insert(want, 5, "AliveChangesetOf")
		}
		if !slices.Equal(f.rec.calls, want) {
			t.Errorf("page %d: calls = %v, want %v", page, f.rec.calls, want)
		}
		byID, err := app.NewListVersions(f.versionReads()).Execute(f.asAlice(), n.ID, &two, cursor)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.EqualFunc(got.Versions, byID.Versions, func(a, b domain.Version) bool { return a == b }) ||
			got.NextCursor != byID.NextCursor {
			t.Errorf("page %d: %v, next %q; Execute's %v, next %q", page, revisionsOf(got.Versions), got.NextCursor,
				revisionsOf(byID.Versions), byID.NextCursor)
		}
		if got.Path != "Guide/Notes.md" || got.Node.ID != n.ID || got.Current != (domain.VersionToken{Node: n.ID, Revision: 5}) {
			t.Errorf("page %d: %s, %s, %v; want Guide/Notes.md, its node, its version 5", page, got.Path, got.Node.ID, got.Current)
		}
		if len(got.Cursors) != len(got.Versions) {
			t.Fatalf("page %d: %d cursors of %d versions", page, len(got.Cursors), len(got.Versions))
		}
		for i, v := range got.Versions {
			if c := decoded[domain.VersionCursor](t, got.Cursors[i]); c.Below != v.Revision {
				t.Errorf("page %d: version %d's cursor goes on below %d", page, v.Revision, c.Below)
			}
		}
		if got.NextCursor == "" {
			if page != 2 || !slices.Equal(revisionsOf(got.Versions), []int{1}) {
				t.Errorf("the last page is %d's, %v; want the third, version 1", page, revisionsOf(got.Versions))
			}
			break
		}
		if got.Cursors[len(got.Cursors)-1] != got.NextCursor {
			t.Errorf("page %d: the last version's cursor %q is not the next page's %q", page, got.Cursors[1], got.NextCursor)
		}
		cursor = &got.NextCursor
	}
	from := app.NewListVersions(f.pathVersionReads())
	first, err := from.AtPath(f.asAlice(), f.eng, "Guide/Notes.md", &two, nil)
	if err != nil {
		t.Fatal(err)
	}
	on, err := from.AtPath(f.asAlice(), f.eng, "Guide/Notes.md", &two, &first.Cursors[0])
	if err != nil || !slices.Equal(revisionsOf(on.Versions), []int{4, 3}) {
		t.Errorf("on from the first version's cursor = %v, %v; want 4 and 3", revisionsOf(on.Versions), err)
	}
}

// versions' list answers its codes in their order: the cursor's 400, the
// path's 422, an attachment's path refused, the limit's 422, all before
// any read; the notebook's 404; the page's, also when the page went since
// it was found.
func TestListVersionsAtPathAnswersItsCodesInOrder(t *testing.T) {
	type faults struct{ cursor, path, limit, notebook, page bool }
	for _, tt := range []struct {
		name string
		f    faults
		want string
	}{
		{"a cursor that does not decode", faults{true, true, true, true, true}, "bad_request"},
		{"an attachment's path", faults{false, true, true, true, true}, "validation_failed path"},
		{"a limit of 101", faults{false, false, true, true, true}, "validation_failed limit"},
		{"a notebook not seen", faults{false, false, false, true, true}, "notebook.not_found"},
		{"no page at the path", faults{false, false, false, false, true}, "page.not_found"},
		{"none", faults{}, ""},
	} {
		f := newFixture()
		n := f.page("Notes", nil, 0)
		f.history(n.ID, 2)
		path, limit := "Notes.md", 2
		var cursor *string
		switch {
		case tt.f.path:
			path = "Notes.png"
		case tt.f.page:
			path = "Missing.md"
		}
		if tt.f.cursor {
			cursor = new("x")
		}
		if tt.f.limit {
			limit = 101
		}
		if !tt.f.notebook {
			f.grant(domain.ActionHistory)
		}
		f.rec.calls = nil
		_, err := app.NewListVersions(f.pathVersionReads()).AtPath(f.asAlice(), f.eng, path, &limit, cursor)
		if codeOf(err) != tt.want {
			t.Errorf("%s = %v, want %q", tt.name, err, tt.want)
		}
		if strings.HasPrefix(tt.want, "bad_request") || strings.HasPrefix(tt.want, "validation_failed") {
			if len(f.rec.calls) != 0 {
				t.Errorf("%s read %v first", tt.name, f.rec.calls)
			}
		}
	}
	f := newFixture()
	f.grant(domain.ActionHistory)
	n := f.page("Notes", nil, 0)
	f.history(n.ID, 2)
	for _, below := range []int{0, domain.MaxRevision + 1} {
		cursor, err := shared.EncodeCursor(domain.VersionCursor{Below: below})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := app.NewListVersions(f.pathVersionReads()).AtPath(f.asAlice(), f.eng, "Notes.md", nil, &cursor); codeOf(err) != "bad_request" {
			t.Errorf("a cursor below %d = %v, want bad_request", below, err)
		}
	}
	for _, path := range []string{"Notes/", "a/./b.md", ""} {
		if _, err := app.NewListVersions(f.pathVersionReads()).AtPath(f.asAlice(), f.eng, path, nil, nil); codeOf(err) != "validation_failed path" {
			t.Errorf("the path %q = %v, want 422 on path", path, err)
		}
	}
	f.rec.before = map[string]func(){"PageStates 1": func() { delete(f.store.contents, n.ID) }}
	if _, err := app.NewListVersions(f.pathVersionReads()).AtPath(f.asAlice(), f.eng, "Notes.md", nil, nil); codeOf(err) != "page.not_found" {
		t.Errorf("a page gone once found = %v, want page.not_found", err)
	}
}

// versions on a path without ".md", maybe a page's written without it, is
// 422 on path before any read, for the list and a version, with the rule
// of the hint read and log give for a page there (S2 review B-N1).
func TestVersionsOfAPathWithoutMDHintAPagesPath(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionHistory)
	guide := f.page("Guide", nil, 0)
	n := f.page("Notes", &guide.ID, 0)
	f.history(n.ID, 2)
	f.rec.calls = nil
	_, list := app.NewListVersions(f.pathVersionReads()).AtPath(f.asAlice(), f.eng, "Guide/Notes", nil, nil)
	_, read := app.NewGetVersion(f.pathVersionReads()).AtPath(f.asAlice(), f.eng, "Guide/Notes", 1, nil, nil, nil, textMeasure(100))
	want := shared.FieldError{Field: "path", Code: shared.FieldNotAllowed,
		Message: "is an attachment's path: attachments have no versions. A page's path ends with .md."}
	for name, err := range map[string]error{"the list": list, "a version": read} {
		var e *shared.Error
		if !errors.As(err, &e) || e.Code != shared.CodeValidationFailed || !slices.Equal(e.Fields, []shared.FieldError{want}) {
			t.Errorf("%s of Guide/Notes = %v, want 422 %+v", name, err, want)
		}
	}
	if len(f.rec.calls) != 0 {
		t.Errorf("the refusals read %v first", f.rec.calls)
	}
}

// A version reads by its page's path as REST's by its page's id: the
// version holding the revision, its own revision, maybe above the one
// asked for; its content cut as read cuts a page's; the page's version
// now; neither whether it is being edited nor who wrote it is read.
func TestGetVersionAtPathReadsTheVersion(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionHistory)
	guide := f.page("Guide", nil, 0)
	n := f.page("Notes", &guide.ID, 0)
	first := f.version(n.ID, nil, 1, "one\ntwo\nthree\n", domain.ChangesetEdit, f.alice)
	f.version(n.ID, &first.Revision, 4, "four\n", domain.ChangesetEdit, f.alice)
	c := f.store.contents[n.ID]
	c.Revision = 4
	f.store.contents[n.ID] = c
	get := app.NewGetVersion(f.pathVersionReads())
	offset, limit := 2, 1
	got, err := get.AtPath(f.asAlice(), f.eng, "guide/notes.md", 1, &offset, nil, &limit, textMeasure(100))
	if err != nil {
		t.Fatal(err)
	}
	want := app.PathVersion{Path: "Guide/Notes.md", Node: n, Current: domain.VersionToken{Node: n.ID, Revision: 4}, Revision: 1,
		Window: domain.Window{Offset: 2, Limit: 1}, Slice: domain.Slice{Text: "two\n", Lines: 1, TotalLines: 3, NextOffset: 3},
		Bytes: 14}
	if got != want {
		t.Errorf("version 1 at Guide/Notes.md = %+v, want %+v", got, want)
	}
	if want := []string{"WorkspaceOf", "Authorize page.history", "RootByKey", "ChildByKey", "FindVersion", "PageStates 1"}; !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v, want %v", f.rec.calls, want)
	}
	for _, revision := range []int{2, 3, 4} {
		if got, err := get.AtPath(f.asAlice(), f.eng, "Guide/Notes.md", revision, nil, nil, nil, textMeasure(100)); err != nil ||
			got.Revision != 4 || got.Slice.Text != "four\n" || got.Bytes != 5 {
			t.Errorf("revision %d = %+v, %v; want version 4's content, its own revision", revision, got, err)
		}
	}
	f.rec.calls = nil
	byID, err := app.NewGetVersion(f.versionReads()).Execute(f.asAlice(), n.ID, 3)
	if err != nil || byID.Version.Revision != 4 || byID.Content != "four\n" {
		t.Errorf("Execute of revision 3 = %+v, %v; want the same version", byID, err)
	}
}

// A version's read answers its codes in their order: the path's 422, an
// attachment's path refused; the revision's out of its range; the
// window's; all before any read; the notebook's 404; the page's; the
// version's; the column on its line, which only the content tells; the
// page's 404 when it went once found.
func TestGetVersionAtPathAnswersItsCodesInOrder(t *testing.T) {
	type faults struct{ path, revision, window, notebook, page, version, column bool }
	for _, tt := range []struct {
		name string
		f    faults
		want string
	}{
		{"an attachment's path", faults{true, true, true, true, true, true, true}, "validation_failed path"},
		{"a revision of 0", faults{false, true, true, true, true, true, true}, "validation_failed revision"},
		{"an offset of 0", faults{false, false, true, true, true, true, true}, "validation_failed offset"},
		{"a notebook not seen", faults{false, false, false, true, true, true, true}, "notebook.not_found"},
		{"no page at the path", faults{false, false, false, false, true, true, true}, "page.not_found"},
		{"no version of the revision", faults{false, false, false, false, false, true, true}, "page.version_not_found"},
		{"a column past its line", faults{false, false, false, false, false, false, true}, "validation_failed column"},
		{"none", faults{}, ""},
	} {
		f := newFixture()
		n := f.page("Notes", nil, 0)
		f.history(n.ID, 2)
		path, revision, offset, column := "Notes.md", 1, 1, 0
		switch {
		case tt.f.path:
			path = "Notes.png"
		case tt.f.page:
			path = "Missing.md"
		}
		if tt.f.revision {
			revision = 0
		} else if tt.f.version {
			revision = 3
		}
		if tt.f.window {
			offset = 0
		}
		if tt.f.column {
			column = 9
		}
		if !tt.f.notebook {
			f.grant(domain.ActionHistory)
		}
		f.rec.calls = nil
		_, err := app.NewGetVersion(f.pathVersionReads()).AtPath(f.asAlice(), f.eng, path, revision, &offset, &column, nil, textMeasure(100))
		if codeOf(err) != tt.want {
			t.Errorf("%s = %v, want %q", tt.name, err, tt.want)
		}
		if strings.HasPrefix(tt.want, "validation_failed") && tt.want != "validation_failed column" && len(f.rec.calls) != 0 {
			t.Errorf("%s read %v first", tt.name, f.rec.calls)
		}
	}
	for _, revision := range []int{-1, domain.MaxRevision + 1} {
		f := newFixture()
		if _, err := app.NewGetVersion(f.pathVersionReads()).AtPath(f.asAlice(), f.eng, "Notes.md", revision, nil, nil, nil,
			textMeasure(100)); codeOf(err) != "validation_failed revision" {
			t.Errorf("revision %d = %v, want 422 on revision", revision, err)
		}
	}
	f := newFixture()
	f.grant(domain.ActionHistory)
	n := f.page("Notes", nil, 0)
	f.history(n.ID, 2)
	f.rec.before = map[string]func(){"PageStates 1": func() { delete(f.store.contents, n.ID) }}
	if _, err := app.NewGetVersion(f.pathVersionReads()).AtPath(f.asAlice(), f.eng, "Notes.md", 1, nil, nil, nil, textMeasure(100)); codeOf(err) != "page.not_found" {
		t.Errorf("a page gone once found = %v, want page.not_found", err)
	}
}

// The log by path, by "me" and the other filters, lists what Execute
// lists by the node's id and the account's: the same changesets, the same
// next cursor, a page at a time; each changeset's cursor goes on after
// it. The node is found by its path, a page or an attachment, unlocked.
func TestListChangesetsAtPathListsAsExecute(t *testing.T) {
	f := newLogFixture()
	png := f.asset("photo.png", &f.a.ID, 0)
	at := f.madeAt.Add(time.Minute)
	pngState := domain.TreeState{ParentID: &f.a.ID, Name: "photo.png"}
	uploaded := f.log.changeset(domain.ChangesetEdit, f.bob, at, []domain.ChangesetItem{{NodeID: png.ID, After: &pngState}})
	me, bob, mcp, web := "me", f.bob.String(), "mcp", "web"
	uc := app.NewListChangesets(f.pathHistoryReads())
	for _, tt := range []struct {
		name string
		q    app.PathLogQuery
		byID app.LogQuery
	}{
		{"all", app.PathLogQuery{}, app.LogQuery{}},
		{"A's", app.PathLogQuery{Path: new("a.md")}, app.LogQuery{PageID: &f.a.ID}},
		{"the attachment's", app.PathLogQuery{Path: new("A/photo.png")}, app.LogQuery{PageID: &png.ID}},
		{"mine", app.PathLogQuery{Actor: &me}, app.LogQuery{Actor: &f.alice}},
		{"bob's", app.PathLogQuery{Actor: &bob}, app.LogQuery{Actor: &f.bob}},
		{"the agents'", app.PathLogQuery{Client: &mcp}, app.LogQuery{Client: &mcp}},
		{"bob's from the web of A", app.PathLogQuery{Actor: &bob, Client: &web, Path: new("A.md")},
			app.LogQuery{Actor: &f.bob, Client: &web, PageID: &f.a.ID}},
	} {
		limit := 2
		var cursor *string
		for page := 0; page < 5; page++ {
			q, byID := tt.q, tt.byID
			q.Limit, q.Cursor, byID.Limit, byID.Cursor = &limit, cursor, &limit, cursor
			got, err := uc.AtPath(f.asAlice(), f.eng, q)
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			want, err := app.NewListChangesets(f.historyReads()).Execute(f.asAlice(), f.eng, byID)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(ids(got.Changesets), ids(want.Changesets)) || got.NextCursor != want.NextCursor {
				t.Errorf("%s, page %d = %v, next %q; Execute's %v, next %q", tt.name, page, ids(got.Changesets), got.NextCursor,
					ids(want.Changesets), want.NextCursor)
			}
			if len(got.Cursors) != len(got.Changesets) {
				t.Fatalf("%s: %d cursors of %d changesets", tt.name, len(got.Cursors), len(got.Changesets))
			}
			for i, c := range got.Changesets {
				p := decoded[domain.ChangesetCursor](t, got.Cursors[i])
				if !p.At.Equal(c.Changeset.CreatedAt) || p.ID != c.Changeset.ID {
					t.Errorf("%s: changeset %s's cursor goes on after %+v", tt.name, c.Changeset.ID, p)
				}
			}
			if got.NextCursor == "" {
				break
			}
			if got.Cursors[len(got.Cursors)-1] != got.NextCursor {
				t.Errorf("%s, page %d: the last changeset's cursor is not the next page's", tt.name, page)
			}
			cursor = &got.NextCursor
		}
	}
	got, err := uc.AtPath(f.asAlice(), f.eng, app.PathLogQuery{Path: new("A/photo.png")})
	if err != nil || !slices.Equal(ids(got.Changesets), []uuid.UUID{uploaded.ID}) {
		t.Errorf("the attachment's log = %v, %v; want its upload", ids(got.Changesets), err)
	}
	f.rec.calls = nil
	if _, err := uc.AtPath(f.asAlice(), f.eng, app.PathLogQuery{Path: new("A.md"), Actor: &me}); err != nil ||
		!slices.Equal(f.rec.calls[:4], []string{"WorkspaceOf", "Authorize page.history", "RootByKey", "ListChangesets actor page"}) {
		t.Errorf("calls = %v, %v; want the notebook, the node by its path, then the log", f.rec.calls, err)
	}
}

// The log by path answers its codes in their order: the cursor's 400; the
// path's, the actor's, the client's and the limit's 422, each before the
// next, all before any read; the notebook's 404; the node's, also for a
// node in the trash, which has no path.
func TestListChangesetsAtPathAnswersItsCodesInOrder(t *testing.T) {
	type faults struct{ cursor, path, actor, client, limit, notebook, node bool }
	for _, tt := range []struct {
		name string
		f    faults
		want string
	}{
		{"a cursor that does not decode", faults{true, true, true, true, true, true, true}, "bad_request"},
		{"a path ending with /", faults{false, true, true, true, true, true, true}, "validation_failed path"},
		{"an actor neither me nor an id", faults{false, false, true, true, true, true, true}, "validation_failed actor"},
		{"a client of none", faults{false, false, false, true, true, true, true}, "validation_failed client"},
		{"a limit of 0", faults{false, false, false, false, true, true, true}, "validation_failed limit"},
		{"a notebook not seen", faults{false, false, false, false, false, true, true}, "notebook.not_found"},
		{"no node at the path", faults{false, false, false, false, false, false, true}, "page.not_found"},
		{"none", faults{}, ""},
	} {
		f := newLogFixture()
		q := app.PathLogQuery{Path: new("A.md"), Actor: new("me"), Client: new("web"), Limit: new(1)}
		switch {
		case tt.f.path:
			q.Path = new("A/")
		case tt.f.node:
			q.Path = new("Missing.md")
		}
		if tt.f.cursor {
			q.Cursor = new("x")
		}
		if tt.f.actor {
			q.Actor = new("you")
		}
		if tt.f.client {
			q.Client = new("mobile")
		}
		if tt.f.limit {
			q.Limit = new(0)
		}
		if tt.f.notebook {
			delete(f.auth.grants, domain.ActionHistory)
		}
		f.rec.calls = nil
		_, err := app.NewListChangesets(f.pathHistoryReads()).AtPath(f.asAlice(), f.eng, q)
		if codeOf(err) != tt.want {
			t.Errorf("%s = %v, want %q", tt.name, err, tt.want)
		}
		if strings.HasPrefix(tt.want, "bad_request") || strings.HasPrefix(tt.want, "validation_failed") {
			if len(f.rec.calls) != 0 {
				t.Errorf("%s read %v first", tt.name, f.rec.calls)
			}
		}
	}
	f := newLogFixture()
	f.trashed(f.b.ID, now(), f.alice)
	if _, err := app.NewListChangesets(f.pathHistoryReads()).AtPath(f.asAlice(), f.eng, app.PathLogQuery{Path: new("B.md")}); codeOf(err) != "page.not_found" {
		t.Errorf("a page in the trash = %v, want page.not_found", err)
	}
}

// changeset in a notebook reads the changeset as GetChangeset does, on its
// first page alone, and its changes as ListChanges does, each with its
// cursor, the paths of the nodes alive read once a page, on every page;
// Start goes on as no cursor does, without the changeset. A changeset of
// another notebook, one the caller reads there, is not found here; the
// zero id is not read.
func TestGetChangesetInNotebook(t *testing.T) {
	f := newLogFixture()
	c := f.page("C", &f.a.ID, 0)
	f.trashed(f.b.ID, now(), f.alice)
	cState := c.State()
	three := f.log.changeset(domain.ChangesetEdit, f.bob, now(), []domain.ChangesetItem{
		{NodeID: f.a.ID, Before: new(f.a.State()), After: &domain.TreeState{Name: "A3"}},
		{NodeID: f.b.ID, After: new(f.b.State())}, {NodeID: c.ID, After: &cState}}, f.log.version(c.ID, 0, 1, "c"))
	uc := app.NewGetChangeset(f.pathHistoryReads())
	two := 2
	f.rec.calls = nil
	first, err := uc.InNotebook(f.asAlice(), f.eng, three.ID, &two, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := app.NewGetChangeset(f.historyReads()).Execute(f.asAlice(), three.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Detail == nil || first.Detail.Changeset.ID != three.ID || first.Detail.Moves != want.Moves ||
		!slices.EqualFunc(first.Detail.Changes, want.Changes, sameChangeRow) || first.Detail.Facts.Nodes != 3 {
		t.Errorf("the first page's changeset = %+v, want GetChangeset's %+v", first.Detail, want)
	}
	byID, err := app.NewListChanges(f.historyReads()).Execute(f.asAlice(), three.ID, &two, nil)
	if err != nil || !slices.EqualFunc(first.Changes, byID.Changes, sameChangeRow) || first.NextCursor != byID.NextCursor ||
		len(first.Cursors) != 2 || first.Cursors[1] != first.NextCursor {
		t.Errorf("the first page's changes = %+v, next %q, cursors %q; ListChanges' %+v, %q, %v", first.Changes, first.NextCursor,
			first.Cursors, byID.Changes, byID.NextCursor, err)
	}
	for i, ch := range first.Changes {
		if p := decoded[domain.ChangeCursor](t, first.Cursors[i]); p.After != ch.NodeID {
			t.Errorf("change %d's cursor goes on after %s, want %s", i, p.After, ch.NodeID)
		}
	}
	// pathsOf are the paths a page of changes has: its alive nodes', none
	// for B in the trash; readOnce whether the calls since f.rec.calls was
	// emptied read them at once, and none for none.
	paths := map[uuid.UUID]string{f.a.ID: "A.md", c.ID: "A/C.md"}
	pathsOf := func(changes []domain.ChangeRow) map[uuid.UUID]string {
		out := map[uuid.UUID]string{}
		for _, ch := range changes {
			if p, ok := paths[ch.NodeID]; ok {
				out[ch.NodeID] = p
			}
		}
		return out
	}
	readOnce := func(want map[uuid.UUID]string) bool {
		var reads []string
		for _, call := range f.rec.calls {
			if strings.HasPrefix(call, "PathsOf ") {
				reads = append(reads, call)
			}
		}
		if len(want) == 0 {
			return len(reads) == 0
		}
		return slices.Equal(reads, []string{"PathsOf " + strconv.Itoa(len(want))})
	}
	if want := pathsOf(first.Changes); !maps.Equal(first.Paths, want) || !f.called("ChangesetMoves") || !readOnce(want) {
		t.Errorf("the first page's paths = %v after %v; want %v, read once, and the changeset's moves", first.Paths, f.rec.calls,
			want)
	}
	f.rec.calls = nil
	start, err := uc.InNotebook(f.asAlice(), f.eng, three.ID, &two, &first.Start)
	if err != nil || start.Detail != nil || !slices.EqualFunc(start.Changes, first.Changes, sameChangeRow) ||
		start.NextCursor != first.NextCursor || f.called("ChangesetMoves") || f.called("ChangesetFacts 1") ||
		!maps.Equal(start.Paths, first.Paths) || !readOnce(start.Paths) {
		t.Errorf("from Start = %+v, %v after %v; want the first page's changes and paths without the changeset", start, err,
			f.rec.calls)
	}
	f.rec.calls = nil
	rest, err := uc.InNotebook(f.asAlice(), f.eng, three.ID, &two, &first.NextCursor)
	if err != nil || rest.Detail != nil || len(rest.Changes) != 1 || rest.NextCursor != "" ||
		rest.Changes[0].NodeID != slices.MaxFunc([]uuid.UUID{f.a.ID, f.b.ID, c.ID}, uuid.UUID.Compare) {
		t.Errorf("the second page = %+v, %v; want the last change alone", rest, err)
	}
	if want := pathsOf(rest.Changes); !maps.Equal(rest.Paths, want) || !readOnce(want) {
		t.Errorf("the second page's paths = %v after %v, want %v, read once", rest.Paths, f.rec.calls, want)
	}
	// Between them, the pages give both alive nodes' paths, C's under A.
	both := maps.Clone(first.Paths)
	maps.Copy(both, rest.Paths)
	if !maps.Equal(both, paths) {
		t.Errorf("the pages' paths are %v, want %v", both, paths)
	}
	other := uuid.NewV7()
	f.notebooks.workspaces[other] = f.acme
	f.elsewhere.NotebookID = other
	if _, err := app.NewGetChangeset(f.historyReads()).Execute(f.asAlice(), f.elsewhere.ID); err != nil {
		t.Fatalf("the other notebook's changeset through REST = %v, want it read", err)
	}
	if _, err := uc.InNotebook(f.asAlice(), f.eng, f.elsewhere.ID, nil, nil); codeOf(err) != "page.changeset_not_found" {
		t.Errorf("another notebook's changeset = %v, want page.changeset_not_found", err)
	}
	if got, err := uc.InNotebook(f.asAlice(), other, f.elsewhere.ID, nil, nil); err != nil || got.Detail.Changeset.ID != f.elsewhere.ID {
		t.Errorf("the changeset in its notebook = %+v, %v; want it", got, err)
	}
	f.rec.calls = nil
	if _, err := uc.InNotebook(f.asAlice(), f.eng, uuid.UUID{}, nil, nil); codeOf(err) != "page.changeset_not_found" ||
		f.called("FindChangeset") {
		t.Errorf("the zero id = %v after %v, want page.changeset_not_found without a read", err, f.rec.calls)
	}
}

// changeset in a notebook answers its codes in their order: the cursor's
// 400, the limit's 422, before any read; the notebook's 404; the
// changeset's, none, gone with its notebook, or another notebook's.
func TestGetChangesetInNotebookAnswersItsCodesInOrder(t *testing.T) {
	type faults struct{ cursor, limit, notebook, changeset bool }
	for _, tt := range []struct {
		name string
		f    faults
		want string
	}{
		{"a cursor that does not decode", faults{true, true, true, true}, "bad_request"},
		{"a limit of 0", faults{false, true, true, true}, "validation_failed limit"},
		{"a notebook not seen", faults{false, false, true, true}, "notebook.not_found"},
		{"no changeset", faults{false, false, false, true}, "page.changeset_not_found"},
		{"none", faults{}, ""},
	} {
		f := newLogFixture()
		id, limit := f.made.ID, 1
		var cursor *string
		if tt.f.cursor {
			cursor = new("x")
		}
		if tt.f.limit {
			limit = 0
		}
		if tt.f.notebook {
			delete(f.auth.grants, domain.ActionHistory)
		}
		if tt.f.changeset {
			id = uuid.NewV7()
		}
		f.rec.calls = nil
		_, err := app.NewGetChangeset(f.pathHistoryReads()).InNotebook(f.asAlice(), f.eng, id, &limit, cursor)
		if codeOf(err) != tt.want {
			t.Errorf("%s = %v, want %q", tt.name, err, tt.want)
		}
		if (tt.f.cursor || tt.f.limit) && len(f.rec.calls) != 0 {
			t.Errorf("%s read %v first", tt.name, f.rec.calls)
		}
	}
	f := newLogFixture()
	for _, c := range []*fakeChangeset{f.gone, f.elsewhere} {
		if _, err := app.NewGetChangeset(f.pathHistoryReads()).InNotebook(f.asAlice(), f.eng, c.ID, nil, nil); codeOf(err) !=
			"page.changeset_not_found" {
			t.Errorf("changeset %s = %v, want page.changeset_not_found", c.ID, err)
		}
	}
}

// sameChangeRow reports whether two changes are the same: their node and
// what the change did.
func sameChangeRow(a, b domain.ChangeRow) bool {
	return a.NodeID == b.NodeID && a.Change() == b.Change() && a.Name == b.Name && a.Alive == b.Alive
}
