package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func (f *fakeStore) ListOwnerless(ctx context.Context, _ uuid.UUID) ([]app.OwnerlessListed, error) {
	f.record(ctx, "ListOwnerless")
	return f.ownerless, nil
}

func (f *fakeStore) ClearOwnerless(ctx context.Context, notebookIDs []uuid.UUID) error {
	f.record(ctx, fmt.Sprintf("ClearOwnerless %v", notebookIDs))
	return nil
}

// fakeAuditFinder pages events, newest first, as the store does.
type fakeAuditFinder struct {
	*recorder
	events []domain.AuditEvent
}

func (f fakeAuditFinder) ListAuditEvents(ctx context.Context, _ uuid.UUID, after *domain.AuditCursor, size int) ([]domain.AuditEvent, error) {
	f.record(ctx, fmt.Sprintf("ListAuditEvents after %v, %d", after != nil, size))
	var page []domain.AuditEvent
	for _, e := range f.events {
		if after == nil || e.At.Before(after.CreatedAt) || (e.At.Equal(after.CreatedAt) && e.ID.Compare(after.ID) < 0) {
			page = append(page, e)
		}
	}
	return page[:min(size, len(page))], nil
}

// fakeActivity answers its activities.
type fakeActivity map[uuid.UUID]app.NotebookActivity

func (f fakeActivity) NotebookActivities(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]app.NotebookActivity, error) {
	return f, nil
}

// ownerless makes the fixture's notebook ownerless, alice its former owner.
func (f fixture) ownerless() domain.Notebook {
	n := f.notebook
	n.Ownerless = &domain.Ownerless{Since: now().Add(-time.Hour), FormerOwner: f.alice}
	f.store.notebooks[n.ID] = n
	return n
}

func (f fixture) takeOver(audit *fakeAudit) *app.TakeOverNotebook {
	return app.NewTakeOverNotebook(app.TakeOverNotebookDeps{
		Workspaces: f.workspaces, Finder: f.store, Notebooks: f.store, Writer: f.members, Ownerless: f.store, Audit: audit,
		Subscribers: []app.VisibilitySubscriber{f.visibility}, Auth: f.auth, Tx: f.tx, Clock: &tickingClock{}, Logger: f.logger(),
	})
}

func (f fixture) deleteOwnerless(audit *fakeAudit, s *fakeSubscriber) *app.DeleteOwnerlessNotebook {
	return app.NewDeleteOwnerlessNotebook(app.DeleteOwnerlessNotebookDeps{
		Workspaces: f.workspaces, Finder: f.store, Notebooks: f.store, Audit: audit, Subscribers: []app.NotebookDeletionSubscriber{s},
		Auth: f.auth, Tx: f.tx, Clock: &tickingClock{}, Logger: f.logger(),
	})
}

// A workspace admin takes an ownerless notebook over: its membership added,
// restored or raised to admin under the locks and the decision; the
// notebook owned again, the take-over recorded, the visibility told.
func TestTakeOverNotebook(t *testing.T) {
	for _, tt := range []struct {
		name  string
		taker func(f fixture) uuid.UUID
		write func(f fixture) string
	}{
		{"never a member", func(f fixture) uuid.UUID { return f.dana },
			func(f fixture) string { return "AddMember admin by " + f.dana.String() }},
		{"a member once", func(f fixture) uuid.UUID { return f.carol },
			func(f fixture) string { return "RestoreMember admin" + at(f.carol) }},
		{"an editor", func(f fixture) uuid.UUID { return f.bob },
			func(f fixture) string { return "UpdateMemberRole admin" + at(f.bob) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			n := f.ownerless()
			f.grant(domain.ActionTakeOver, shared.WorkspaceAdmin, "")
			taker := tt.taker(f)
			audit := &fakeAudit{recorder: f.rec}

			got, err := f.takeOver(audit).Execute(as(taker), n.ID)

			if err != nil || got.Role != shared.NotebookAdmin || got.MemberCount != 3 || got.Notebook.Ownerless != nil || got.Notebook.ID != n.ID {
				t.Errorf("Execute() = %+v, %v; want the notebook owned, the taker its admin", got, err)
			}
			calls := append([]string{"FindNotebook"}, inTx("ShareByID", "LockNotebook", "Authorize notebook_ownerless.take_over",
				"FindMemberOf", tt.write(f), fmt.Sprintf("ClearOwnerless %v", []uuid.UUID{n.ID}), "AddAuditEvent taken_over",
				"VisibilityChanged visibility", "CountMembers")...)
			if !slices.Equal(f.rec.calls, calls) {
				t.Errorf("calls = %q, want %q", f.rec.calls, calls)
			}
			want := domain.AuditEvent{WorkspaceID: f.acme, NotebookID: n.ID, NotebookName: n.Name, Action: domain.AuditTakenOver,
				FormerOwnerID: f.alice, ActorID: taker, At: now()}
			if len(audit.events) != 1 || !sameEvent(audit.events[0], want) {
				t.Errorf("recorded %+v, want %+v", audit.events, want)
			}
			if target := (shared.Target{WorkspaceID: f.acme}); f.auth.targets[0] != target {
				t.Errorf("target = %+v, want the workspace alone, %+v", f.auth.targets[0], target)
			}
			v := app.VisibilityChange{WorkspaceID: f.acme, UserIDs: []uuid.UUID{taker}, At: now()}
			if len(f.visibility.got) != 1 || !sameChange(f.visibility.got[0], v) {
				t.Errorf("visibility told %+v, want %+v", f.visibility.got, v)
			}
		})
	}
}

// sameEvent compares two audit events but their ids, which the use cases
// make: the made one must be set.
func sameEvent(got, want domain.AuditEvent) bool {
	if got.ID == (uuid.UUID{}) || !got.At.Equal(want.At) {
		return false
	}
	want.ID, want.At = got.ID, got.At
	return got == want
}

// The deletion of an ownerless notebook deletes it as deleteNotebook does
// and records it.
func TestDeleteOwnerlessNotebook(t *testing.T) {
	f := newFixture()
	n := f.ownerless()
	f.grant(domain.ActionDeleteOwnerless, shared.WorkspaceAdmin, "")
	audit, s := &fakeAudit{recorder: f.rec}, &fakeSubscriber{recorder: f.rec, name: "s"}

	err := f.deleteOwnerless(audit, s).Execute(as(f.dana), n.ID)

	calls := append([]string{"FindNotebook"}, inTx("ShareByID", "LockNotebook", "Authorize notebook_ownerless.delete",
		"DeleteNotebook by "+f.dana.String()+" at 2026-10-02T10:00:00Z", "AddAuditEvent deleted", "NotebookDeleted s")...)
	if err != nil || !slices.Equal(f.rec.calls, calls) {
		t.Errorf("Execute() = %v, calls %q; want %q", err, f.rec.calls, calls)
	}
	want := domain.AuditEvent{WorkspaceID: f.acme, NotebookID: n.ID, NotebookName: n.Name, Action: domain.AuditDeleted,
		FormerOwnerID: f.alice, ActorID: f.dana, At: now()}
	if len(audit.events) != 1 || !sameEvent(audit.events[0], want) {
		t.Errorf("recorded %+v, want %+v", audit.events, want)
	}
	if len(s.got) != 1 || !slices.Equal(s.got[0].NotebookIDs, []uuid.UUID{n.ID}) || s.got[0].By != f.dana || !s.got[0].At.Equal(now()) {
		t.Errorf("the deletion's subscriber got %+v, want the notebook by dana at the deletion's time", s.got)
	}
}

// By id, every refusal is notebook.not_found: no such notebook, one
// deleted meanwhile, one the caller cannot see or may not act on at the
// workspace level, one not ownerless under its lock. Nothing is written.
func TestOwnerlessByIDRefusals(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setUp func(f fixture) uuid.UUID
	}{
		{"no such notebook", func(fixture) uuid.UUID { return uuid.NewV7() }},
		{"deleted meanwhile", func(f fixture) uuid.UUID {
			f.grantOwnerless()
			n := f.ownerless()
			f.store.deleted[n.ID] = true
			return n.ID
		}},
		{"not visible", func(f fixture) uuid.UUID { return f.ownerless().ID }},
		{"forbidden: a member of the workspace", func(f fixture) uuid.UUID {
			f.auth.err = shared.Forbidden()
			return f.ownerless().ID
		}},
		{"not ownerless", func(f fixture) uuid.UUID {
			f.grantOwnerless()
			return f.notebook.ID
		}},
	} {
		for op, run := range map[string]func(f fixture, id uuid.UUID) error{
			"take over": func(f fixture, id uuid.UUID) error {
				_, err := f.takeOver(&fakeAudit{recorder: f.rec}).Execute(as(f.dana), id)
				return err
			},
			"delete": func(f fixture, id uuid.UUID) error {
				return f.deleteOwnerless(&fakeAudit{recorder: f.rec}, &fakeSubscriber{recorder: f.rec}).Execute(as(f.dana), id)
			},
		} {
			t.Run(op+", "+tt.name, func(t *testing.T) {
				f := newFixture()
				id := tt.setUp(f)

				err := run(f, id)

				if !errors.Is(err, domain.ErrNotFound) || slices.ContainsFunc(f.rec.calls, isOwnerlessWrite) {
					t.Errorf("Execute() = %v, calls %q; want notebook.not_found, no write", err, f.rec.calls)
				}
			})
		}
	}
}

func (f fixture) grantOwnerless() {
	f.grant(domain.ActionTakeOver, shared.WorkspaceAdmin, "")
	f.grant(domain.ActionDeleteOwnerless, shared.WorkspaceAdmin, "")
}

// isOwnerlessWrite reports whether call writes for a take-over or a
// deletion.
func isOwnerlessWrite(call string) bool {
	for _, w := range []string{"AddMember", "RestoreMember", "UpdateMemberRole", "ClearOwnerless", "DeleteNotebook", "AddAuditEvent"} {
		if len(call) >= len(w) && call[:len(w)] == w {
			return true
		}
	}
	return false
}

func (f fixture) listOwnerless(sources ...app.NotebookActivitySource) *app.ListOwnerlessNotebooks {
	return app.NewListOwnerlessNotebooks(app.ListOwnerlessNotebooksDeps{
		Workspaces: f.workspaces, Notebooks: f.store, Profiles: f.profiles, Activities: sources, Auth: f.auth,
	})
}

// The list is the workspace admins': each notebook with its former owner's
// profile, its bytes summed across the sources and its last activity the
// latest of theirs and its own last update.
func TestListOwnerlessNotebooks(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionListOwnerless, shared.WorkspaceAdmin, "")
	first, second := f.ownerless(), f.ownerless()
	second.ID, second.Ownerless = uuid.NewV7(), &domain.Ownerless{Since: now(), FormerOwner: f.bob}
	f.store.ownerless = []app.OwnerlessListed{{Notebook: first, MemberCount: 2}, {Notebook: second, MemberCount: 0}}
	later, earlier := first.UpdatedAt.Add(time.Hour), second.UpdatedAt.Add(-time.Hour)
	sources := []app.NotebookActivitySource{
		fakeActivity{first.ID: {Bytes: 10, LastWriteAt: &later}, uuid.NewV7(): {Bytes: 99}},
		fakeActivity{first.ID: {Bytes: 5}, second.ID: {Bytes: 7, LastWriteAt: &earlier}},
	}

	for _, tt := range []struct {
		name    string
		sources []app.NotebookActivitySource
		want    [][2]any // bytes, last activity
	}{
		{"two sources", sources, [][2]any{{int64(15), later}, {int64(7), second.UpdatedAt}}},
		{"no source", nil, [][2]any{{int64(0), first.UpdatedAt}, {int64(0), second.UpdatedAt}}},
	} {
		f.rec.calls = nil
		got, err := f.listOwnerless(tt.sources...).Execute(as(f.dana), "acme")

		if err != nil || len(got) != 2 {
			t.Fatalf("%s: Execute() = %+v, %v", tt.name, got, err)
		}
		for i, o := range got {
			if o.SizeBytes != tt.want[i][0] || !o.LastActivityAt.Equal(tt.want[i][1].(time.Time)) {
				t.Errorf("%s: %d is %d bytes, last at %v; want %v", tt.name, i, o.SizeBytes, o.LastActivityAt, tt.want[i])
			}
		}
		if got[0].FormerOwner.DisplayName != "Alice" || got[1].FormerOwner.DisplayName != "Bob" || got[0].MemberCount != 2 {
			t.Errorf("%s: listed %+v, want alice's with 2 members, then bob's", tt.name, got)
		}
		if calls := []string{"FindBySlug acme", "Authorize notebook_ownerless.list", "ListOwnerless", "MemberProfiles"}; !slices.Equal(f.rec.calls, calls) {
			t.Errorf("%s: calls = %q, want %q", tt.name, f.rec.calls, calls)
		}
	}
}

// The workspace's members and guests see the workspace: forbidden. The
// rest do not: workspace.not_found. A former owner without a profile is a
// fault.
func TestListOwnerlessNotebooksRefusals(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setUp func(f fixture)
		slug  string
		is    func(err error) bool
	}{
		{"a member", func(f fixture) { f.auth.err = shared.Forbidden() }, "acme", func(err error) bool { return errors.Is(err, shared.Forbidden()) }},
		{"not visible", func(fixture) {}, "acme", func(err error) bool { return errors.Is(err, domain.ErrWorkspaceNotFound) }},
		{"no such workspace", func(fixture) {}, "beta", func(err error) bool { return errors.Is(err, domain.ErrWorkspaceNotFound) }},
		{"no profile", func(f fixture) {
			f.grant(domain.ActionListOwnerless, shared.WorkspaceAdmin, "")
			f.store.ownerless = []app.OwnerlessListed{{Notebook: f.ownerless()}}
			delete(f.profiles.names, f.alice)
		}, "acme", func(err error) bool { return err != nil && !errors.Is(err, domain.ErrWorkspaceNotFound) }},
	} {
		f := newFixture()
		tt.setUp(f)

		got, err := f.listOwnerless().Execute(as(f.bob), tt.slug)

		if !tt.is(err) || got != nil {
			t.Errorf("%s: Execute() = %+v, %v", tt.name, got, err)
		}
	}
}

func (f fixture) listAudit(events []domain.AuditEvent) *app.ListNotebookAuditEvents {
	return app.NewListNotebookAuditEvents(app.ListNotebookAuditEventsDeps{
		Workspaces: f.workspaces, Audit: fakeAuditFinder{recorder: f.rec, events: events}, Profiles: f.profiles, Auth: f.auth,
	})
}

// The events come a page at a time, newest first, with their former
// owners' and actors' profiles: one row more than the page tells that
// another follows, whose cursor is the page's last event.
func TestListNotebookAuditEvents(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionListAudit, shared.WorkspaceAdmin, "")
	event := func(at time.Time, actor uuid.UUID) domain.AuditEvent {
		return domain.AuditEvent{ID: uuid.NewV7(), WorkspaceID: f.acme, NotebookID: f.notebook.ID, NotebookName: "Engineering",
			Action: domain.AuditTakenOver, FormerOwnerID: f.alice, ActorID: actor, At: at}
	}
	// Two at one time: the id orders them, the greater first.
	events := []domain.AuditEvent{event(now(), f.bob), event(now().Add(-time.Hour), f.carol), event(now().Add(-time.Hour), f.dana)}
	events[1], events[2] = events[2], events[1]
	two := 2

	first, err := f.listAudit(events).Execute(as(f.dana), "acme", &two, nil)

	if err != nil || len(first.Events) != 2 || first.Events[0].Event.ID != events[0].ID || first.Events[1].Event.ID != events[1].ID ||
		first.NextCursor == "" || first.Events[0].Actor.DisplayName != "Bob" || first.Events[0].FormerOwner.DisplayName != "Alice" {
		t.Fatalf("the first page = %+v, %v; want the two newest, their profiles, a cursor", first, err)
	}
	calls := []string{"FindBySlug acme", "Authorize notebook_audit.list", "ListAuditEvents after false, 3", "MemberProfiles"}
	if !slices.Equal(f.rec.calls, calls) {
		t.Errorf("calls = %q, want %q", f.rec.calls, calls)
	}

	second, err := f.listAudit(events).Execute(as(f.dana), "acme", &two, &first.NextCursor)

	if err != nil || len(second.Events) != 1 || second.Events[0].Event.ID != events[2].ID || second.NextCursor != "" {
		t.Errorf("the second page = %+v, %v; want the last event, no cursor", second, err)
	}
	f.rec.calls = nil
	if empty, err := f.listAudit(nil).Execute(as(f.dana), "acme", nil, nil); err != nil || empty.Events != nil || empty.NextCursor != "" ||
		!slices.Equal(f.rec.calls, []string{"FindBySlug acme", "Authorize notebook_audit.list", "ListAuditEvents after false, 51"}) {
		t.Errorf("an empty list = %+v, %v, calls %q; want nothing, 50 by default, no profile read", empty, err, f.rec.calls)
	}
}

// The cursor is judged first, before the workspace: 400; then the
// workspace and the decision; then the limit: 422.
func TestListNotebookAuditEventsRefusals(t *testing.T) {
	zero, bad := 0, "not a cursor"
	for _, tt := range []struct {
		name   string
		grant  bool
		limit  *int
		cursor *string
		is     func(err error) bool
		calls  []string
	}{
		{"a cursor the list cannot read", true, &zero, &bad, func(err error) bool { return errors.Is(err, shared.InvalidCursor()) }, nil},
		{"not visible, the limit wrong", false, &zero, nil, func(err error) bool { return errors.Is(err, domain.ErrWorkspaceNotFound) },
			[]string{"FindBySlug acme", "Authorize notebook_audit.list"}},
		{"a limit out of range", true, &zero, nil, func(err error) bool {
			var se *shared.Error
			return errors.As(err, &se) && se.ProblemStatus() == 422 && se.Fields[0].Field == "limit"
		}, []string{"FindBySlug acme", "Authorize notebook_audit.list"}},
	} {
		f := newFixture()
		if tt.grant {
			f.grant(domain.ActionListAudit, shared.WorkspaceAdmin, "")
		}

		_, err := f.listAudit(nil).Execute(as(f.dana), "acme", tt.limit, tt.cursor)

		if !tt.is(err) || !slices.Equal(f.rec.calls, tt.calls) {
			t.Errorf("%s: Execute() = %v, calls %q; want %q", tt.name, err, f.rec.calls, tt.calls)
		}
	}
}
