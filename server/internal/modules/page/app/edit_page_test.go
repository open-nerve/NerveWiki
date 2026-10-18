package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// editor is edit's use case over the fixture.
func (f *fixture) editor() *app.EditPage {
	return app.NewEditPage(f.writer(), f.parser(), f.store, f.store, f.store, app.NewLockHolders(f.store, f.names), f.notebooks,
		f.auth, f.clock, f.logger())
}

// editPage runs edit in notebook as alice from an agent.
func (f *fixture) editPage(notebook uuid.UUID, p app.PageEdit) (app.Edited, error) {
	return f.editor().Execute(f.asAlice(), notebook, p, agent)
}

// count is how many times call was recorded.
func (f *fixture) count(call string) int {
	n := 0
	for _, c := range f.rec.calls {
		if c == call {
			n++
		}
	}
	return n
}

// edit answers in its order (M9/P2 design 3.8.2): the path's, the two
// strings' and the token's 422 before the notebook; the notebook, the
// page, then page.write's 403, so that a reader gets 403 for an old_string
// the page does not hold; then the token, before the match: a stale
// token is 409 whatever old_string; the match's 422; an edit session of
// the page before the parse takes its budget.
func TestEditPageAnswersInOrder(t *testing.T) {
	f := newFixture()
	notes := f.page("Notes", nil, 0)
	f.content(notes.ID, "one two", 2)
	unseen := uuid.NewV7()
	bad := "x"
	edit := func(path, old, new string, base *string) app.PageEdit {
		return app.PageEdit{Path: path, Old: old, New: new, Base: base}
	}
	for _, tt := range []struct {
		name     string
		notebook uuid.UUID
		p        app.PageEdit
		want     string
	}{
		{"a path with ..", unseen, edit("../Notes.md", "", "", &bad), "validation_failed path"},
		{"an attachment's path", unseen, edit("Notes/x.png", "", "", &bad), "validation_failed path"},
		{"an empty old_string", unseen, edit("Notes.md", "", "x", &bad), "validation_failed old_string"},
		{"two strings the same", unseen, edit("Notes.md", "two", "two", &bad), "validation_failed new_string"},
		{"a NUL in new_string", unseen, edit("Notes.md", "two", "2\x00", &bad), "validation_failed new_string"},
		{"a token that is no version", unseen, edit("Notes.md", "two", "2", &bad), "validation_failed base_version"},
		{"a notebook not seen", unseen, edit("Missing.md", "two", "2", nil), "notebook.not_found"},
	} {
		f.rec.calls = nil
		if _, err := f.editPage(tt.notebook, tt.p); codeOf(err) != tt.want || f.called("PageContent") {
			t.Errorf("%s: %v after %v, want %s", tt.name, err, f.rec.calls, tt.want)
		}
	}
	f.grant(domain.ActionRead)
	f.auth.forbidden[domain.ActionWrite] = true
	for _, tt := range []struct {
		name string
		p    app.PageEdit
		want string
	}{
		{"a reader's edit of a path that names nothing", edit("Missing.md", "two", "2", nil), "page.not_found"},
		{"a reader's edit of what the page does not hold", edit("Notes.md", "three", "3", nil), "forbidden"},
	} {
		f.rec.calls = nil
		if _, err := f.editPage(f.eng, tt.p); codeOf(err) != tt.want || f.called("PageContent") {
			t.Errorf("%s: %v after %v, want %s", tt.name, err, f.rec.calls, tt.want)
		}
	}
	f.auth.forbidden = map[shared.Action]bool{}
	f.grant(domain.ActionWrite)
	for _, tt := range []struct {
		name string
		p    app.PageEdit
		want string
	}{
		{"another page's token", edit("Notes.md", "three", "3", tokenOf(uuid.NewV7(), 2)), "page.revision_mismatch"},
		{"a stale token", edit("Notes.md", "three", "3", tokenOf(notes.ID, 1)), "page.revision_mismatch"},
		{"what the page does not hold", edit("Notes.md", "three", "3", tokenOf(notes.ID, 2)), "validation_failed old_string"},
	} {
		f.rec.calls = nil
		if _, err := f.editPage(f.eng, tt.p); codeOf(err) != tt.want || f.took() || f.called("ShareWorkspace in tx") {
			t.Errorf("%s: %v after %v, want %s before the parse", tt.name, err, f.rec.calls, tt.want)
		}
	}
	busy := errors.New("the budget is spent")
	f.budget.err = busy
	f.rec.calls = nil
	if _, err := f.editPage(f.eng, edit("Notes.md", "two", "2", nil)); !errors.Is(err, busy) || !f.called("AliveSessionsOf") ||
		f.called("ShareWorkspace in tx") {
		t.Errorf("an edit with the budget spent = %v after %v, want its error after the locks' read, before the unit", err, f.rec.calls)
	}
	f.budget.err = nil
	f.names[f.alice] = "Alice"
	f.session(notes.ID, f.alice, now().Add(time.Minute))
	f.rec.calls = nil
	_, err := f.editPage(f.eng, edit("Notes.md", "two", "2", nil))
	var paths *domain.LockPaths
	if page, holder, locked := lockOf(err, f.names); !locked || page != notes.ID || holder != "Alice" || f.took() ||
		!errors.As(err, &paths) || paths.Paths[notes.ID] != "Notes.md" {
		t.Errorf("an edit of a page in an edit session = %v after %v; want page.locked with its path, the budget not taken",
			err, f.rec.calls)
	}
	if _, err := f.editor().Execute(t.Context(), f.eng, edit("Notes.md", "two", "2", nil), agent); codeOf(err) != "unauthorized" {
		t.Errorf("no actor: %v", err)
	}
}

// edit replaces old_string where the page holds it once, or everywhere
// with replace_all, on the server: a page's CRLF lines take a string of
// LF ones; the answer is the page's new version, how many it replaced,
// written by the names the page has (M9/P2 design 3.8.2, 3.9).
func TestEditPageReplaces(t *testing.T) {
	for _, tt := range []struct {
		name, content, old, new string
		all                     bool
		want                    string
		replacements            int
	}{
		{"once", "one two three", "two", "2", false, "one 2 three", 1},
		{"everywhere", "a b a b a", "a", "A", true, "A b A b A", 3},
		{"the lines of a CRLF page", "one\r\ntwo\r\nthree\r\n", "one\ntwo", "1\n2", false, "1\r\n2\r\nthree\r\n", 1},
	} {
		f := newFixture()
		f.grant(domain.ActionRead, domain.ActionWrite)
		notes := f.page("Notes", nil, 0)
		f.content(notes.ID, tt.content, 2)
		got, err := f.editPage(f.eng, app.PageEdit{Path: "notes.md", Old: tt.old, New: tt.new, All: tt.all})
		switch {
		case err != nil:
			t.Errorf("%s: %v", tt.name, err)
		case f.store.contents[notes.ID].Content != tt.want:
			t.Errorf("%s: the page holds %q, want %q", tt.name, f.store.contents[notes.ID].Content, tt.want)
		case got.Path != "Notes.md" || got.Node.ID != notes.ID || got.Version != (domain.VersionToken{Node: notes.ID, Revision: 3}) ||
			got.Replacements != tt.replacements || got.Redone || got.Outcome.ChangesetID == (uuid.UUID{}):
			t.Errorf("%s: edit = %+v, want Notes at 3 with %d replacements", tt.name, got, tt.replacements)
		case !strings.Contains(f.logs.String(), `msg="page edited"`) || !strings.Contains(f.logs.String(), "revision=3 replacements="):
			t.Errorf("%s: logs = %s", tt.name, f.logs)
		}
	}
	f := newFixture()
	f.grant(domain.ActionRead, domain.ActionWrite)
	notes := f.page("Notes", nil, 0)
	f.content(notes.ID, "a b a", 2)
	if _, err := f.editPage(f.eng, app.PageEdit{Path: "Notes.md", Old: "a", New: "A"}); fieldProblem(err) != "validation_failed old_string not_allowed" {
		t.Errorf("an edit of a string the page holds twice = %v, want 422 on old_string", err)
	}
}

// An edit that leaves the page as it is writes nothing and makes no
// changeset: the CRLF rule making the two strings one, or a write between
// the edit's read and its unit that made the page what the edit makes it
// (M9/P2 design 3.8.2 step 6).
func TestEditPageThatChangesNothing(t *testing.T) {
	for _, tt := range []struct {
		name, content, old, new string
		between                 string
		revision                int
	}{
		{"the strings made one", "a\r\nb\r\n", "a\nb", "a\r\nb", "", 2},
		{"a write in between", "one two", "two", "2", "one 2", 3},
	} {
		f := newFixture()
		f.grant(domain.ActionRead, domain.ActionWrite)
		notes := f.page("Notes", nil, 0)
		f.content(notes.ID, tt.content, 2)
		if tt.between != "" {
			f.rec.before = map[string]func(){"LockContent in tx": func() { f.writeAs(notes.ID, tt.between) }}
		}
		got, err := f.editPage(f.eng, app.PageEdit{Path: "Notes.md", Old: tt.old, New: tt.new})
		if err != nil || got.Outcome.ChangesetID != (uuid.UUID{}) || len(f.store.changesets) != 0 || got.Redone ||
			got.Version != (domain.VersionToken{Node: notes.ID, Revision: tt.revision}) || strings.Contains(f.logs.String(), "page edited") {
			t.Errorf("%s: edit = %+v, %v; want the page at %d, nothing written", tt.name, got, err, tt.revision)
		}
	}
}

// An edit whose unit finds the page moved on since its read, its content
// written or its path another page's, is made once more from the read,
// without a token: the second on the new content and node, whose 422 is
// its own; moved on again, 409 kept changing. With a token it is 409 at
// once, the content read once (M9/P2 design 3.8.2 step 5). The page put
// at the path is at the revision read: only the node tells it apart.
func TestEditPageRedoesOnceWithoutAToken(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead, domain.ActionWrite)
	notes := f.page("Notes", nil, 0)
	f.content(notes.ID, "one two", 2)
	f.rec.before = map[string]func(){"LockContent in tx": func() { f.writeAs(notes.ID, "one two three") }}
	got, err := f.editPage(f.eng, app.PageEdit{Path: "Notes.md", Old: "two", New: "2"})
	if err != nil || !got.Redone || got.Version.Revision != 4 || f.store.contents[notes.ID].Content != "one 2 three" ||
		f.count("PageContent") != 2 || f.budget.held != 0 || !strings.Contains(f.logs.String(), "redone=true") {
		t.Errorf("an edit after a write in between = %+v, %v after %v; want it made again on the new content", got, err, f.rec.calls)
	}

	f = newFixture()
	f.grant(domain.ActionRead, domain.ActionWrite)
	notes = f.page("Notes", nil, 0)
	f.content(notes.ID, "one two", 2)
	f.rec.before = map[string]func(){"LockContent in tx": func() {
		f.writeAs(notes.ID, "one two three")
		f.rec.before["LockContent in tx"] = func() { f.writeAs(notes.ID, "one two three four") }
	}}
	if _, err := f.editPage(f.eng, app.PageEdit{Path: "Notes.md", Old: "two", New: "2"}); mismatchOf(err) !=
		"Notes.md kept changing "+notes.ID.String()+":4" || f.count("PageContent") != 2 || len(f.store.changesets) != 0 {
		t.Errorf("an edit after two writes in between = %v, want 409 kept changing at the version now", mismatchOf(err))
	}

	f = newFixture()
	f.grant(domain.ActionRead, domain.ActionWrite)
	notes = f.page("Notes", nil, 0)
	f.content(notes.ID, "one two", 2)
	f.rec.before = map[string]func(){"LockContent in tx": func() { f.writeAs(notes.ID, "one two three") }}
	if _, err := f.editPage(f.eng, app.PageEdit{Path: "Notes.md", Old: "two", New: "2", Base: tokenOf(notes.ID, 2)}); mismatchOf(err) !=
		"Notes.md changed "+notes.ID.String()+":3" || f.count("PageContent") != 1 {
		t.Errorf("an edit on its token after a write in between = %v after %v, want 409 changed, read once", mismatchOf(err), f.rec.calls)
	}

	for _, tt := range []struct {
		name, content string
		want          string
	}{
		{"holding old_string", "two of them", ""},
		{"without old_string", "something else", "validation_failed old_string"},
	} {
		f := newFixture()
		f.grant(domain.ActionRead, domain.ActionWrite)
		notes := f.page("Notes", nil, 0)
		f.content(notes.ID, "one two", 2)
		var other domain.Node
		f.rec.before = map[string]func(){"RootByKey in tx": func() {
			delete(f.store.nodes, notes.ID)
			delete(f.store.contents, notes.ID)
			other = f.page("Notes", nil, 0)
			f.content(other.ID, tt.content, 2)
		}}
		got, err := f.editPage(f.eng, app.PageEdit{Path: "Notes.md", Old: "two", New: "2"})
		switch {
		case codeOf(err) != tt.want:
			t.Errorf("an edit whose path names another page in its unit, %s = %v, want %q", tt.name, err, tt.want)
		case err == nil && (got.Node.ID != other.ID || !got.Redone || f.store.contents[other.ID].Content != "2 of them"):
			t.Errorf("an edit whose path names another page in its unit = %+v, want the other page edited", got)
		}
	}
}

// A token's edit whose unit finds another page at the path, at the
// token's revision, is 409 of another node, the version there read again
// outside the unit: nothing is written on it.
func TestEditPageOnATokenOfAPageSwapped(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead, domain.ActionWrite)
	notes := f.page("Notes", nil, 0)
	f.content(notes.ID, "one two", 2)
	var other domain.Node
	f.rec.before = map[string]func(){"RootByKey in tx": func() {
		delete(f.store.nodes, notes.ID)
		delete(f.store.contents, notes.ID)
		other = f.page("Notes", nil, 0)
		f.content(other.ID, "two", 2)
	}}
	_, err := f.editPage(f.eng, app.PageEdit{Path: "Notes.md", Old: "two", New: "2", Base: tokenOf(notes.ID, 2)})
	if mismatchOf(err) != "Notes.md other node "+other.ID.String()+":2" || f.count("PageContent") != 1 ||
		f.store.contents[other.ID].Content != "two" {
		t.Errorf("an edit on its token of a page swapped in its unit = %v, want 409 of another node at its version", mismatchOf(err))
	}
}
