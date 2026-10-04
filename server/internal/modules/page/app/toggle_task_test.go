package app_test

import (
	"errors"
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

// toggle runs toggleTask on the page id as alice from the web.
func (f *fixture) toggle(id uuid.UUID, p app.TaskToggle) (app.PageView, error) {
	return app.NewToggleTask(f.writer(), f.store, f.parser(), f.md, f.logger()).Execute(f.asAlice(), id, p, domain.ClientWeb)
}

// taskList is a content of task items, with a byte order mark, CRLF line
// breaks and a decomposed é around them: the item "a" open at 18, "b"
// done at 27, "c" done in a capital at 36, "d" open with a tab at 45.
const taskList = "\ufeff# Cafe\u0301\r\n\r\n- [ ] a\r\n- [x] b\r\n- [X] c\r\n- [\t] d\r\n"

// taskPage is a page of eng whose content is taskList at revision 2.
func (f *fixture) taskPage() domain.Node {
	n := f.page("Notes", nil, 0)
	f.writeAs(n.ID, taskList)
	return n
}

// A toggle finds the page unlocked and decides, reads its content, parses
// it within the budget and gives the budget back, parses the new content,
// and only then runs the unit, which decides again and writes as a
// content write does; the second parse's budget is released once the unit
// is over.
func TestToggleTaskDecidesReadsParsesThenWrites(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionToggleTask)
	n := f.taskPage()
	if _, err := f.toggle(n.ID, app.TaskToggle{Base: 2, Offset: 18, Checked: true}); err != nil {
		t.Fatal(err)
	}
	take, release := "Take "+strconv.Itoa(len(taskList)), "Release "+strconv.Itoa(len(taskList))
	keep := "KeepFacts " + strconv.Itoa(len(taskList))
	want := []string{"FindNode", "WorkspaceOf", "Authorize page.toggle_task", "PageContent", take, "Facts", keep, "Tasks", release,
		take, "Facts", keep, "Tasks", "WorkspaceOf", "ShareWorkspace in tx", "ShareNotebook in tx", "Authorize page.toggle_task in tx",
		"LockContent in tx", "FindNodeIn in tx", "CreateChangeset in tx", "WriteContent in tx", "RecordRevision in tx"}
	if !slices.Equal(f.rec.calls[:min(len(want), len(f.rec.calls))], want) || f.rec.calls[len(f.rec.calls)-1] != release ||
		f.budget.held != 0 {
		t.Errorf("calls = %v, %d bytes held; want them to begin %v and end with the release", f.rec.calls, f.budget.held, want)
	}
}

// A tick or a clear changes the one byte between the brackets, whatever
// was around it, and writes a version as a content write in no session
// does: the guard sees the step, the observers the new content's facts.
func TestToggleTaskChangesTheOneByte(t *testing.T) {
	for _, tt := range []struct {
		offset  int
		checked bool
		want    string
	}{
		{18, true, "\ufeff# Cafe\u0301\r\n\r\n- [x] a\r\n- [x] b\r\n- [X] c\r\n- [\t] d\r\n"},
		{27, false, "\ufeff# Cafe\u0301\r\n\r\n- [ ] a\r\n- [ ] b\r\n- [X] c\r\n- [\t] d\r\n"},
		{36, false, "\ufeff# Cafe\u0301\r\n\r\n- [ ] a\r\n- [x] b\r\n- [ ] c\r\n- [\t] d\r\n"},
		{45, true, "\ufeff# Cafe\u0301\r\n\r\n- [ ] a\r\n- [x] b\r\n- [X] c\r\n- [x] d\r\n"},
	} {
		f := newFixture()
		f.grant(domain.ActionToggleTask)
		g, o := &guard{recorder: f.rec}, &observer{recorder: f.rec}
		f.guards, f.observers = []app.WriteGuard{g}, []app.PageObserver{o}
		n := f.taskPage()
		v, err := f.toggle(n.ID, app.TaskToggle{Base: 2, Offset: tt.offset, Checked: tt.checked})
		if err != nil {
			t.Fatal(err)
		}
		if got := f.store.contents[n.ID]; got.Content != tt.want || got.Revision != 3 || v.Content.Revision != 3 {
			t.Errorf("toggle %d to %t wrote %q at %d, answered %d; want %q at 3", tt.offset, tt.checked, got.Content, got.Revision,
				v.Content.Revision, tt.want)
		}
		if len(g.steps) != 1 || g.steps[0].Operation != domain.OpContent || g.steps[0].EditSessionID != (uuid.UUID{}) {
			t.Errorf("guarded %+v, want one content step in no session", g.steps)
		}
		if len(o.events) != 1 || o.events[0].Changes[0].Facts != tt.want {
			t.Errorf("observed %+v, want one event with the new content's facts", o.events)
		}
		if logs := f.logs.String(); !strings.Contains(logs, `msg="page task toggled"`) ||
			!strings.Contains(logs, "offset="+strconv.Itoa(tt.offset)) || strings.Contains(logs, "Cafe") {
			t.Errorf("logs %q, want the toggle's ids and offset, not the content", logs)
		}
	}
}

// An item in the state asked for already writes nothing, its character
// kept, a capital X or a tab among them: the page as it is, no guard, no
// event, no log. A write that came while the item was read makes it
// page.revision_mismatch: the page answered is no longer the one checked.
func TestToggleTaskOfAnItemInThatStateWritesNothing(t *testing.T) {
	for _, p := range []app.TaskToggle{{Base: 2, Offset: 18, Checked: false}, {Base: 2, Offset: 27, Checked: true},
		{Base: 2, Offset: 36, Checked: true}, {Base: 2, Offset: 45, Checked: false}} {
		f := newFixture()
		f.grant(domain.ActionToggleTask)
		g, o := &guard{recorder: f.rec, err: domain.ErrLocked}, &observer{recorder: f.rec}
		f.guards, f.observers = []app.WriteGuard{g}, []app.PageObserver{o}
		n := f.taskPage()
		v, err := f.toggle(n.ID, p)
		if err != nil || v.Content.Revision != 2 || v.Node.ID != n.ID {
			t.Errorf("%+v: %+v, %v; want the page at revision 2", p, v.Content, err)
		}
		if f.store.contents[n.ID].Content != taskList || len(g.steps) != 0 || len(o.events) != 0 || f.called("WriteContent in tx") ||
			f.logs.Len() != 0 || f.budget.held != 0 {
			t.Errorf("%+v: guarded %d, events %d, logs %q, %d bytes held; want nothing", p, len(g.steps), len(o.events), f.logs,
				f.budget.held)
		}
	}
}

func TestToggleTaskOfAnItemInThatStateAfterAWriteIsAMismatch(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionToggleTask)
	n := f.taskPage()
	f.md.tasksRead = func() { f.writeAs(n.ID, "- [ ] a\n- [ ] b\n") }
	if _, err := f.toggle(n.ID, app.TaskToggle{Base: 2, Offset: 27, Checked: true}); codeOf(err) != "page.revision_mismatch" {
		t.Errorf("toggleTask = %q, want page.revision_mismatch", codeOf(err))
	}
}

// An offset of no item is out of range; a tick or a clear that would
// leave no item there is not allowed: the offset is an item's.
func TestToggleTaskTellsWhyAnOffsetIsRefused(t *testing.T) {
	for content, want := range map[string]string{"- [ ] a\n": shared.FieldOutOfRange, "- [ ]: /u\n": shared.FieldNotAllowed} {
		f := newFixture()
		f.grant(domain.ActionToggleTask)
		n := f.page("Notes", nil, 0)
		f.writeAs(n.ID, content)
		offset := 3
		if want == shared.FieldOutOfRange {
			offset = 4
		}
		_, err := f.toggle(n.ID, app.TaskToggle{Base: 2, Offset: offset, Checked: true})
		var e *shared.Error
		if !errors.As(err, &e) || len(e.Fields) != 1 || e.Fields[0].Field != "offset" || e.Fields[0].Code != want {
			t.Errorf("%q at %d: %v, want 422 on offset, %s", content, offset, err, want)
		}
	}
}

// The page and the decision come first, then the base (an offset means
// something in its revision only), then the budget and the offset, the
// new content's item, and the guard last.
func TestToggleTaskAnswersItsCodesInOrder(t *testing.T) {
	refusal := shared.NewError(shared.KindConflict, "page.locked", "Locked.")
	for _, tt := range []struct {
		name  string
		setup func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle)
		want  string
	}{
		{"an unknown page", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.grant(domain.ActionToggleTask)
			return uuid.NewV7(), app.TaskToggle{Base: 9, Offset: -1}
		}, "page.not_found"},
		{"a node that is no page", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.grant(domain.ActionToggleTask)
			n.Kind = domain.KindAsset
			f.store.nodes[n.ID] = n
			return n.ID, app.TaskToggle{Base: 2, Offset: 18, Checked: true}
		}, "page.not_found"},
		{"a notebook the caller cannot see", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			return n.ID, app.TaskToggle{Base: 9, Offset: -1}
		}, "page.not_found"},
		{"a reader, before the base and the offset", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.auth.forbidden[domain.ActionToggleTask] = true
			return n.ID, app.TaskToggle{Base: 9, Offset: -1}
		}, "forbidden"},
		{"a page deleted before its content is read", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.grant(domain.ActionToggleTask)
			delete(f.store.contents, n.ID)
			return n.ID, app.TaskToggle{Base: 2, Offset: 18, Checked: true}
		}, "page.not_found"},
		{"a base that is not the page's revision, before the offset", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.grant(domain.ActionToggleTask)
			f.budget.err = shared.ServerBusy(time.Second)
			return n.ID, app.TaskToggle{Base: 1, Offset: -1, Checked: true}
		}, "page.revision_mismatch"},
		{"a budget that is spent", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.grant(domain.ActionToggleTask)
			f.budget.err = shared.ServerBusy(time.Second)
			return n.ID, app.TaskToggle{Base: 2, Offset: -1, Checked: true}
		}, "server_busy"},
		{"an offset of no task item", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.grant(domain.ActionToggleTask)
			return n.ID, app.TaskToggle{Base: 2, Offset: 17, Checked: true}
		}, "validation_failed offset"},
		{"a negative offset", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.grant(domain.ActionToggleTask)
			return n.ID, app.TaskToggle{Base: 2, Offset: -1, Checked: true}
		}, "validation_failed offset"},
		{"an offset past the content", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.grant(domain.ActionToggleTask)
			return n.ID, app.TaskToggle{Base: 2, Offset: len(taskList) + 3, Checked: true}
		}, "validation_failed offset"},
		{"a tick that makes a link reference definition", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.grant(domain.ActionToggleTask)
			f.writeAs(n.ID, "- [ ]: /u\n")
			return n.ID, app.TaskToggle{Base: 3, Offset: 3, Checked: true}
		}, "validation_failed offset"},
		{"a base passed, before the guard", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.grant(domain.ActionToggleTask)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			return n.ID, app.TaskToggle{Base: 1, Offset: 18, Checked: true}
		}, "page.revision_mismatch"},
		{"a tick that makes a link reference definition, before the guard", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.grant(domain.ActionToggleTask)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			f.writeAs(n.ID, "- [ ]: /u\n")
			return n.ID, app.TaskToggle{Base: 3, Offset: 3, Checked: true}
		}, "validation_failed offset"},
		{"the guard last", func(f *fixture, n domain.Node) (uuid.UUID, app.TaskToggle) {
			f.grant(domain.ActionToggleTask)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			return n.ID, app.TaskToggle{Base: 2, Offset: 18, Checked: true}
		}, "page.locked"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			n := f.taskPage()
			id, p := tt.setup(f, n)
			before := f.store.contents[n.ID]
			_, err := f.toggle(id, p)
			if got := codeOf(err); got != tt.want {
				t.Errorf("toggleTask = %q, want %q", got, tt.want)
			}
			if after, ok := f.store.contents[n.ID]; ok && after.Content != before.Content || f.logs.Len() != 0 || f.budget.held != 0 {
				t.Errorf("a refused toggle left %q, logged %q, held %d bytes; want %q, nothing, none", after.Content, f.logs,
					f.budget.held, before.Content)
			}
		})
	}
}
