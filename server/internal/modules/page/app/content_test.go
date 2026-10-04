package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
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

// put runs putPageContent on the page id as alice from the web.
func (f *fixture) put(id uuid.UUID, p app.ContentPut) (app.PageView, error) {
	return app.NewPutPageContent(f.writer(), f.store, f.parser(), f.logger()).Execute(f.asAlice(), id, p, domain.ClientWeb)
}

// session opens an edit session of the page id for user, alive until
// expires, without recording a call.
func (f *fixture) session(id, user uuid.UUID, expires time.Time) app.EditSession {
	s := app.EditSession{ID: uuid.NewV7(), NodeID: id, NotebookID: f.eng, UserID: user, Client: domain.ClientWeb,
		CreatedAt: now().Add(-time.Minute), ExpiresAt: expires}
	f.store.sessions[s.ID] = s
	return s
}

// writeAs has someone else write the page id's content to text, outside
// the test's units, as a write in between does.
func (f *fixture) writeAs(id uuid.UUID, text string) {
	c := f.store.contents[id]
	sum := sha256.Sum256([]byte(text))
	c.Content, c.Revision, c.Hash, c.ByteSize = text, c.Revision+1, sum[:], len(text)
	f.store.contents[id] = c
}

// A content write finds the page unlocked and decides, then takes the
// content's facts within the budget, outside the unit (M4/P4 review P1,
// P2), and gives the budget back before the unit (M6 design 4.7); the unit
// shares the workspace's row and the notebook's, decides again, and locks
// the page's gate before it reads the node again: its content row, not the
// node's.
func TestPutPageContentDecidesThenParsesThenLocks(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionWrite)
	n := f.page("Notes", nil, 0)
	if _, err := f.put(n.ID, app.ContentPut{Content: "# Notes\n", Base: 1}); err != nil {
		t.Fatal(err)
	}
	want := []string{"FindNode", "WorkspaceOf", "Authorize page.write", "Take 8", "Facts", "Release 8", "WorkspaceOf",
		"ShareWorkspace in tx", "ShareNotebook in tx", "Authorize page.write in tx", "LockContent in tx", "FindNodeIn in tx",
		"CreateChangeset in tx", "WriteContent in tx", "RecordRevision in tx"}
	if !slices.Equal(f.rec.calls[:len(want)], want) || f.budget.held != 0 {
		t.Errorf("calls = %v, %d bytes held; want them to begin %v", f.rec.calls, f.budget.held, want)
	}
}

// Only a writer makes the server parse a content (M4/P4 review P1): a page
// or a notebook the caller cannot see, and a reader, are answered before
// the parse, the budget untouched; their codes are the unit's.
func TestOnlyAWriterMakesTheContentParsed(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(f *fixture)
		want  string
	}{
		{"a notebook the caller cannot see", func(*fixture) {}, "page.not_found"},
		{"a notebook deleted", func(f *fixture) {
			f.grant(domain.ActionWrite, domain.ActionCreate)
			delete(f.notebooks.workspaces, f.eng)
		}, "page.not_found"},
		{"a reader", func(f *fixture) {
			f.auth.forbidden[domain.ActionWrite], f.auth.forbidden[domain.ActionCreate] = true, true
		}, "forbidden"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			n := f.page("Notes", nil, 0)
			tt.setup(f)
			_, putErr := f.put(n.ID, app.ContentPut{Content: "x", Base: 1})
			_, createErr := f.create(app.PageDraft{Title: "New", Content: "x"})
			createWant := tt.want
			if createWant == "page.not_found" {
				createWant = "notebook.not_found"
			}
			if codeOf(putErr) != tt.want || codeOf(createErr) != createWant || f.called("Facts") || f.budget.held != 0 ||
				slices.ContainsFunc(f.rec.calls, func(c string) bool { return strings.HasPrefix(c, "Take") }) {
				t.Errorf("put %q, create %q, calls %v; want %q and %q, no take and no parse", codeOf(putErr), codeOf(createErr),
					f.rec.calls, tt.want, createWant)
			}
		})
	}
}

// A budget that does not free up answers the write's 503 before the parse
// and the unit; a write that fails in its unit took and released the
// budget before it.
func TestAContentWriteTakesTheBudget(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionWrite)
	n := f.page("Notes", nil, 0)
	f.budget.err = shared.ServerBusy(time.Second)
	if _, err := f.put(n.ID, app.ContentPut{Content: "x", Base: 1}); codeOf(err) != "server_busy" || f.called("Facts") ||
		f.called("ShareWorkspace in tx") {
		t.Errorf("put = %q, calls %v; want server_busy before the parse and the unit", codeOf(err), f.rec.calls)
	}
	f.budget.err = nil
	if _, err := f.put(n.ID, app.ContentPut{Content: "x", Base: 7}); codeOf(err) != "page.revision_mismatch" || f.budget.held != 0 ||
		f.budget.released != 1 {
		t.Errorf("put = %q, %d bytes held, %d released; want the 409 with the budget released", codeOf(err), f.budget.held, f.budget.released)
	}
}

// A parse that panics gives its bytes back as the panic goes on: the
// platform answers it 500 and the server runs on, its budget whole (P4
// fix check, finding 1).
func TestAParseThatPanicsGivesTheBudgetBack(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionWrite)
	n := f.page("Notes", nil, 0)
	f.md.panics = true
	recovered := func() (r any) {
		defer func() { r = recover() }()
		_, _ = f.put(n.ID, app.ContentPut{Content: "xyz", Base: 1})
		return nil
	}()
	if recovered == nil || f.budget.held != 0 || f.budget.released != 1 {
		t.Errorf("a panicking parse: recovered %v, %d bytes held, %d released; want the panic and the budget whole", recovered,
			f.budget.held, f.budget.released)
	}
}

// A write leaves the content at the next revision, with its hash and size,
// by the writer at the unit's time, in a changeset of its own with the
// page's version on its base; the content alone moves no node, so the
// changeset has no item. The guard and the observer get the write with its
// parse; the answer is the page as the unit left it.
func TestPutPageContentWritesAVersion(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionWrite)
	g, o := &guard{recorder: f.rec}, &observer{recorder: f.rec}
	f.guards, f.observers = []app.WriteGuard{g}, []app.PageObserver{o}
	n := f.page("Notes", nil, 0)
	text := "# Notes\r\n\r\nCafe\xcc\x81 \t\n"
	v, err := f.put(n.ID, app.ContentPut{Content: text, Base: 1})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(text))
	c := f.store.contents[n.ID]
	if c.Content != text || c.Revision != 2 || !bytes.Equal(c.Hash, sum[:]) || c.ByteSize != len(text) || c.By != f.alice || !c.At.Equal(now()) {
		t.Errorf("content = %+v, want the text at revision 2, by alice at %v", c, now())
	}
	if v.Content.Revision != 2 || v.Content.ByteSize != len(text) || v.Node.ID != n.ID {
		t.Errorf("answer = %+v, want the page at revision 2", v)
	}
	if len(f.store.changesets) != 1 || f.store.changesets[0].Client != domain.ClientWeb || len(f.store.items) != 0 {
		t.Errorf("changesets %+v, items %+v; want one changeset of the web without items", f.store.changesets, f.store.items)
	}
	r := f.store.revisions
	if len(r) != 1 || r[0].Base == nil || *r[0].Base != 1 || r[0].Revision != 2 || r[0].Content != text || r[0].ChangesetID != f.store.changesets[0].ID {
		t.Errorf("versions = %+v, want revision 2 on 1 in the changeset", r)
	}
	state := n.State()
	want := domain.Change{NodeID: n.ID, Before: &state, After: &state, Revision: 2, Facts: text}
	if len(g.steps) != 1 || g.steps[0].Operation != domain.OpContent || !sameChange(g.steps[0].Changes[0], want) ||
		g.steps[0].EditSessionID != (uuid.UUID{}) {
		t.Errorf("the guard saw %+v, want the content's write with its parse, in no session", g.steps)
	}
	if len(o.events) != 1 || !sameChange(o.events[0].Changes[0], want) {
		t.Errorf("events = %+v, want the content's write with its parse", o.events)
	}
}

// sameChange compares two changes, their states by value.
func sameChange(a, b domain.Change) bool {
	same := func(x, y *domain.TreeState) bool { return x == nil && y == nil || x != nil && y != nil && x.Same(*y) }
	return a.NodeID == b.NodeID && same(a.Before, b.Before) && same(a.After, b.After) && a.Revision == b.Revision && a.Facts == b.Facts
}

// A content the page holds already writes nothing, whatever the base: no
// guard, changeset, event or log; the answer is the page as it is.
func TestPutPageContentOfTheSameContentWritesNothing(t *testing.T) {
	for _, base := range []int{2, 1, 7} {
		f := newFixture()
		f.grant(domain.ActionWrite)
		g, o := &guard{recorder: f.rec}, &observer{recorder: f.rec}
		f.guards, f.observers = []app.WriteGuard{g}, []app.PageObserver{o}
		n := f.page("Notes", nil, 0)
		f.writeAs(n.ID, "same\n")
		v, err := f.put(n.ID, app.ContentPut{Content: "same\n", Base: base})
		if err != nil || v.Content.Revision != 2 {
			t.Errorf("on base %d: %+v, %v; want the page at revision 2", base, v.Content, err)
		}
		if len(g.steps) != 0 || len(o.events) != 0 || len(f.store.changesets) != 0 || f.called("WriteContent in tx") || f.logs.Len() != 0 {
			t.Errorf("on base %d: guarded %d, events %d, changesets %d, logs %q; want nothing", base, len(g.steps), len(o.events),
				len(f.store.changesets), f.logs)
		}
	}
}

// The content's values come before the page; then the page, the decision,
// the session, the base, and the guard last.
func TestPutPageContentAnswersItsCodesInOrder(t *testing.T) {
	refusal := shared.NewError(shared.KindConflict, "page.locked", "Locked.")
	for _, tt := range []struct {
		name  string
		setup func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut)
		want  string
	}{
		{"a content over 5 MB before all", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			return uuid.NewV7(), app.ContentPut{Content: strings.Repeat("a", domain.MaxContentBytes+1), Base: 9}
		}, "validation_failed content"},
		{"a NUL before all", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			return uuid.NewV7(), app.ContentPut{Content: "\x00", Base: 9}
		}, "validation_failed content"},
		{"an unknown page", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			return uuid.NewV7(), app.ContentPut{Content: "x", Base: 9}
		}, "page.not_found"},
		{"a node that is no page", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			n.Kind = domain.KindAsset
			f.store.nodes[n.ID] = n
			return n.ID, app.ContentPut{Content: "x", Base: 1}
		}, "page.not_found"},
		{"a notebook the caller cannot see", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			return n.ID, app.ContentPut{Content: "x", Base: 9}
		}, "page.not_found"},
		{"a notebook deleted while it waited", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			f.notebooks.gone[f.eng] = true
			return n.ID, app.ContentPut{Content: "x", Base: 9}
		}, "page.not_found"},
		{"a reader", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.auth.forbidden[domain.ActionWrite] = true
			return n.ID, app.ContentPut{Content: "x", Base: 9, EditSession: uuid.NewV7()}
		}, "forbidden"},
		{"a page deleted while it waited", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			delete(f.store.contents, n.ID)
			return n.ID, app.ContentPut{Content: "x", Base: 9, EditSession: uuid.NewV7()}
		}, "page.not_found"},
		{"a session that is not there before the base", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			return n.ID, app.ContentPut{Content: "x", Base: 9, EditSession: uuid.NewV7()}
		}, "page.edit_session_ended"},
		{"someone else's session", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			s := f.session(n.ID, uuid.NewV7(), now().Add(time.Minute))
			return n.ID, app.ContentPut{Content: "x", Base: 1, EditSession: s.ID}
		}, "page.edit_session_ended"},
		{"a session opened from another client", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			s := f.session(n.ID, f.alice, now().Add(time.Minute))
			s.Client = domain.ClientAPI
			f.store.sessions[s.ID] = s
			return n.ID, app.ContentPut{Content: "x", Base: 1, EditSession: s.ID}
		}, "page.edit_session_ended"},
		{"a session that is not there, with the content the page holds", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			f.writeAs(n.ID, "same")
			return n.ID, app.ContentPut{Content: "same", Base: 2, EditSession: uuid.NewV7()}
		}, "page.edit_session_ended"},
		{"another page's session", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			s := f.session(f.page("Other", nil, 1).ID, f.alice, now().Add(time.Minute))
			return n.ID, app.ContentPut{Content: "x", Base: 1, EditSession: s.ID}
		}, "page.edit_session_ended"},
		{"a session that expires at the unit's time", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			s := f.session(n.ID, f.alice, now())
			return n.ID, app.ContentPut{Content: "x", Base: 1, EditSession: s.ID}
		}, "page.edit_session_ended"},
		{"someone else's tombstone", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			s := f.tombstone(f.session(n.ID, uuid.NewV7(), now().Add(time.Minute)), domain.EndedTakenOver, f.alice)
			return n.ID, app.ContentPut{Content: "x", Base: 9, EditSession: s.ID}
		}, "page.edit_session_ended"},
		{"a tombstone opened from another client", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			s := f.session(n.ID, f.alice, now().Add(time.Minute))
			s.Client = domain.ClientAPI
			s = f.tombstone(s, domain.EndedTakenOver, f.alice)
			return n.ID, app.ContentPut{Content: "x", Base: 9, EditSession: s.ID}
		}, "page.edit_session_ended"},
		{"a session taken over, before the base", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			s := f.tombstone(f.session(n.ID, f.alice, now().Add(time.Minute)), domain.EndedTakenOver, f.alice)
			return n.ID, app.ContentPut{Content: "x", Base: 9, EditSession: s.ID}
		}, "page.edit_session_taken_over"},
		{"a session unlocked, its lease run out", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			s := f.tombstone(f.session(n.ID, f.alice, now()), domain.EndedUnlocked, uuid.NewV7())
			return n.ID, app.ContentPut{Content: "x", Base: 9, EditSession: s.ID}
		}, "page.edit_session_unlocked"},
		{"a base that is not the page's revision", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			f.writeAs(n.ID, "theirs")
			return n.ID, app.ContentPut{Content: "mine", Base: 1}
		}, "page.revision_mismatch"},
		{"the guard last", func(f *fixture, n domain.Node) (uuid.UUID, app.ContentPut) {
			f.grant(domain.ActionWrite)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			s := f.session(n.ID, f.alice, now().Add(time.Microsecond))
			return n.ID, app.ContentPut{Content: "mine", Base: 1, EditSession: s.ID}
		}, "page.locked"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			n := f.page("Notes", nil, 0)
			id, p := tt.setup(f, n)
			before := f.store.contents[n.ID]
			_, err := f.put(id, p)
			if got := codeOf(err); got != tt.want {
				t.Errorf("putPageContent = %q, want %q", got, tt.want)
			}
			if after, ok := f.store.contents[n.ID]; ok && after.Revision != before.Revision || f.logs.Len() != 0 {
				t.Errorf("a refused write left revision %d, logged %q; want %d and nothing", after.Revision, f.logs, before.Revision)
			}
			if f.called("Facts") && !slices.Equal(f.rec.calls[:6], []string{"FindNode", "WorkspaceOf", "Authorize page.write", "Take " +
				strconv.Itoa(len(p.Content)), "Facts", "Release " + strconv.Itoa(len(p.Content))}) || f.budget.held != 0 {
				t.Errorf("calls = %v, %d bytes held; want the decision, the take, the parse and the release first", f.rec.calls,
					f.budget.held)
			}
		})
	}
}

// An edit session's writes are one changeset, with one version of the
// page that keeps the first base: its later writes move the changeset's
// time on. Someone else's write in between has the session's next write
// start a changeset of its own, on the page's revision then.
func TestAnEditSessionsWritesAreOneChangeset(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionWrite)
	g := &guard{recorder: f.rec}
	f.guards = []app.WriteGuard{g}
	n := f.page("Notes", nil, 0)
	s := f.session(n.ID, f.alice, now().Add(time.Hour))
	for i, text := range []string{"one", "two"} {
		if _, err := f.put(n.ID, app.ContentPut{Content: text, Base: 1 + i, EditSession: s.ID}); err != nil {
			t.Fatal(err)
		}
	}
	first := f.store.changesets[0].ID
	if len(f.store.changesets) != 1 || len(f.store.revisions) != 1 {
		t.Fatalf("changesets %+v, versions %+v; want one of each", f.store.changesets, f.store.revisions)
	}
	if r := f.store.revisions[0]; *r.Base != 1 || r.Revision != 3 || r.Content != "two" {
		t.Errorf("the version = %+v, want two at revision 3 on 1", r)
	}
	if at, ok := f.store.touched[first]; !ok || !at.Equal(now().Add(time.Microsecond)) {
		t.Errorf("the changeset's later write = %v, %v; want it moved to the second unit's time", at, ok)
	}
	if got := f.store.sessions[s.ID]; got.ChangesetID != first || got.Revision != 3 {
		t.Errorf("the session = %+v, want changeset %s at revision 3", got, first)
	}
	if len(g.steps) != 2 || g.steps[1].EditSessionID != s.ID {
		t.Errorf("the guard saw %+v, want the session's writes in it", g.steps)
	}

	f.writeAs(n.ID, "theirs")
	if _, err := f.put(n.ID, app.ContentPut{Content: "three", Base: 4, EditSession: s.ID}); err != nil {
		t.Fatal(err)
	}
	if len(f.store.changesets) != 2 || len(f.store.revisions) != 2 {
		t.Fatalf("after a write in between: changesets %d, versions %d; want a second of each", len(f.store.changesets), len(f.store.revisions))
	}
	second := f.store.changesets[1].ID
	if r := f.store.revisions[1]; r.ChangesetID != second || *r.Base != 4 || r.Revision != 5 {
		t.Errorf("the second version = %+v, want revision 5 on 4 in a changeset of its own", r)
	}
	if got := f.store.sessions[s.ID]; got.ChangesetID != second || got.Revision != 5 {
		t.Errorf("the session = %+v, want the second changeset at revision 5", got)
	}
	if _, err := f.put(n.ID, app.ContentPut{Content: "four", Base: 5, EditSession: s.ID}); err != nil {
		t.Fatal(err)
	}
	if len(f.store.changesets) != 2 || f.store.revisions[1].Revision != 6 || *f.store.revisions[1].Base != 4 {
		t.Errorf("changesets %d, the second version %+v; want the session's next write in the second", len(f.store.changesets), f.store.revisions[1])
	}
}

// A participant's content write, added to a unit, runs through the guards,
// writes the next revision in the unit's changeset, joins its event, and
// calls no participant; a session it names is not its.
func TestAParticipantWritesAContent(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	renamed, linking := f.page("Renamed", nil, 0), f.page("Linking", nil, 1)
	g, o := &guard{recorder: f.rec}, &observer{recorder: f.rec}
	s := f.session(linking.ID, uuid.NewV7(), now().Add(time.Hour))
	p := &participant{recorder: f.rec, write: &app.ContentWrite{NodeID: linking.ID, Content: "[[Moved]]", Facts: "links", Base: 1,
		EditSession: s.ID}}
	f.guards, f.observers, f.partakers = []app.WriteGuard{g}, []app.PageObserver{o}, []app.Participant{p}
	if _, err := f.run(f.asAlice(), domain.ClientWeb, func(ctx context.Context, u *app.Unit) error {
		_, err := u.Rename(ctx, renamed.ID, "Moved")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if p.revision != 2 || f.store.contents[linking.ID].Content != "[[Moved]]" || len(p.steps) != 1 {
		t.Errorf("the participant wrote revision %d, content %q, followed %d steps; want 2, its content, the rename alone",
			p.revision, f.store.contents[linking.ID].Content, len(p.steps))
	}
	if len(g.steps) != 2 || g.steps[1].Operation != domain.OpContent || g.steps[1].EditSessionID != (uuid.UUID{}) {
		t.Errorf("the guard saw %+v, want the rename, then the content's write in no session", g.steps)
	}
	if len(f.store.changesets) != 1 || len(f.store.revisions) != 1 || f.store.revisions[0].ChangesetID != f.store.changesets[0].ID {
		t.Errorf("changesets %+v, versions %+v; want the content's version in the one changeset", f.store.changesets, f.store.revisions)
	}
	if len(o.events) != 1 || len(o.events[0].Changes) != 2 || o.events[0].Changes[1].Facts != "links" {
		t.Errorf("events = %+v, want one with the rename and the content's write with its parse", o.events)
	}
}

// Its log tells the ids of the write, its revision and session, never the
// title or the content.
func TestPutPageContentLogsTheIDsNotTheContent(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionWrite)
	n := f.page("Zebrafish", nil, 0)
	s := f.session(n.ID, f.alice, now().Add(time.Hour))
	if _, err := f.put(n.ID, app.ContentPut{Content: "Okapi", Base: 1, EditSession: s.ID}); err != nil {
		t.Fatal(err)
	}
	logs := f.logs.String()
	for _, want := range []string{"page content written", f.acme.String(), f.eng.String(), n.ID.String(), f.store.changesets[0].ID.String(),
		f.alice.String(), "client=web", "revision=2", "edit_session_id=" + s.ID.String()} {
		if !strings.Contains(logs, want) {
			t.Errorf("log %q lacks %q", logs, want)
		}
	}
	if strings.Contains(logs, "Zebrafish") || strings.Contains(logs, "Okapi") {
		t.Errorf("log %q tells the title or the content", logs)
	}
}

// A page's content reads as written, with its revision and hash; a page
// the caller cannot see is page.not_found.
func TestGetPageContent(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	n := f.page("Notes", nil, 0)
	f.writeAs(n.ID, "a\r\nb")
	got, err := app.NewGetPageContent(f.notebooks, f.store, f.auth).Execute(f.asAlice(), n.ID)
	sum := sha256.Sum256([]byte("a\r\nb"))
	if err != nil || got.Content != "a\r\nb" || got.Revision != 2 || !bytes.Equal(got.Hash, sum[:]) {
		t.Errorf("getPageContent = %+v, %v; want the content at revision 2 with its hash", got, err)
	}
	f.auth.grants[domain.ActionRead] = false
	if _, err := app.NewGetPageContent(f.notebooks, f.store, f.auth).Execute(f.asAlice(), n.ID); codeOf(err) != "page.not_found" {
		t.Errorf("getPageContent of a notebook not seen = %v, want page.not_found", err)
	}
}

// A page created with a content holds it at revision 1, and its creation
// carries the parse.
func TestCreatePageWithAContent(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	o := &observer{recorder: f.rec}
	f.observers = []app.PageObserver{o}
	v, err := f.create(app.PageDraft{Title: "Notes", Content: "# Notes\n"})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("# Notes\n"))
	if c := f.store.contents[v.Node.ID]; c.Content != "# Notes\n" || c.Revision != 1 || !bytes.Equal(c.Hash, sum[:]) || v.Content.ByteSize != 8 {
		t.Errorf("content = %+v, answer %+v; want it at revision 1, 8 bytes", c, v.Content)
	}
	if r := f.store.revisions[0]; r.Content != "# Notes\n" || r.Base != nil {
		t.Errorf("version = %+v, want the content without a base", r)
	}
	if len(o.events) != 1 || o.events[0].Changes[0].Facts != "# Notes\n" {
		t.Errorf("events = %+v, want the creation with its parse", o.events)
	}
}
