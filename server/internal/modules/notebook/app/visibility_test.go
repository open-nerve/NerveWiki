package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The notebook's own triggers of the visibility event (M3/P2 design 3.6):
// each after its write, in its transaction, with its value.

func (f fixture) createWith(subscribers ...app.VisibilitySubscriber) *app.CreateNotebook {
	return app.NewCreateNotebook(app.CreateNotebookDeps{
		Workspaces: f.workspaces, Notebooks: f.store, Subscribers: subscribers, Auth: f.auth, Tx: f.tx, Clock: &tickingClock{},
		Logger: f.logger(),
	})
}

func (f fixture) updateWith(subscribers ...app.VisibilitySubscriber) *app.UpdateNotebook {
	return app.NewUpdateNotebook(app.UpdateNotebookDeps{
		Workspaces: f.workspaces, Finder: f.store, Notebooks: f.store, Subscribers: subscribers, Auth: f.auth, Tx: f.tx,
		Clock: &tickingClock{}, Logger: f.logger(),
	})
}

// A new notebook is seen by its creator, and by the workspace's admins and
// members when it is open to them.
func TestCreateNotebookTellsTheVisibility(t *testing.T) {
	for _, tt := range []struct {
		access  *string
		reached bool
	}{{nil, false}, {ptr("none"), false}, {ptr("viewer"), true}, {ptr("editor"), true}} {
		f := newFixture()
		f.grant(domain.ActionCreate, shared.WorkspaceMember, "")

		if _, err := f.createWith(f.visibility).Execute(as(f.alice), "acme", "Notes", tt.access); err != nil {
			t.Fatal(err)
		}

		want := app.VisibilityChange{WorkspaceID: f.acme, UserIDs: []uuid.UUID{f.alice}, Reached: tt.reached, At: now()}
		if len(f.visibility.got) != 1 || !sameChange(f.visibility.got[0], want) {
			t.Errorf("access %v: told %+v, want %+v", tt.access, f.visibility.got, want)
		}
		if i := slices.Index(f.rec.calls, "VisibilityChanged visibility in tx"); i < 0 || !slices.ContainsFunc(f.rec.calls[:i], isAddMember) {
			t.Errorf("access %v: calls = %q, want the visibility told after the writes, in the transaction", tt.access, f.rec.calls)
		}
	}
}

func isAddMember(call string) bool { return len(call) > 9 && call[:9] == "AddMember" }

// An access that crosses none tells of every admin and member of the
// workspace; one between viewer and editor, or a rename, tells no one.
func TestUpdateNotebookTellsTheVisibilityWhenTheAccessCrossesNone(t *testing.T) {
	for _, tt := range []struct {
		from         shared.WorkspaceAccess
		name, access *string
		told         bool
	}{
		{shared.AccessNone, nil, ptr("viewer"), true},
		{shared.AccessEditor, nil, ptr("none"), true},
		{shared.AccessViewer, nil, ptr("editor"), false},
		{shared.AccessNone, ptr("Renamed"), nil, false},
		{shared.AccessNone, nil, ptr("none"), false},
	} {
		f := newFixture()
		f.grant(domain.ActionUpdate, shared.WorkspaceMember, shared.NotebookAdmin)
		n := f.notebook
		n.Access = tt.from
		f.store.notebooks[n.ID] = n

		if _, err := f.updateWith(f.visibility).Execute(as(f.alice), n.ID, tt.name, tt.access); err != nil {
			t.Fatal(err)
		}

		var want []app.VisibilityChange
		if tt.told {
			want = []app.VisibilityChange{{WorkspaceID: f.acme, Reached: true, At: now()}}
		}
		if !slices.EqualFunc(f.visibility.got, want, sameChange) {
			t.Errorf("%s to %v: told %+v, want %+v", tt.from, tt.access, f.visibility.got, want)
		}
	}
}

// The workspace module's addition and role change, forwarded: a default
// role given or taken is told, a guest's coming and a change between admin
// and member are not.
func TestWorkspaceMemberEvents(t *testing.T) {
	const admin, member, guest = shared.WorkspaceAdmin, shared.WorkspaceMember, shared.WorkspaceGuest
	ws, user := uuid.NewV7(), uuid.NewV7()
	for _, tt := range []struct {
		name string
		send func(e app.WorkspaceMemberEvents) error
		told bool
	}{
		{"an admin added", added(ws, user, admin), true},
		{"a member added", added(ws, user, member), true},
		{"a guest added", added(ws, user, guest), false},
		{"a member made a guest", changed(ws, user, member, guest), true},
		{"a guest made an admin", changed(ws, user, guest, admin), true},
		{"an admin made a member", changed(ws, user, admin, member), false},
		{"a member made an admin", changed(ws, user, member, admin), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := &fakeVisibility{recorder: &recorder{}, name: "visibility"}

			if err := tt.send(app.WorkspaceMemberEvents{Subscribers: []app.VisibilitySubscriber{v}}); err != nil {
				t.Fatal(err)
			}

			var want []app.VisibilityChange
			if tt.told {
				want = []app.VisibilityChange{{WorkspaceID: ws, UserIDs: []uuid.UUID{user}, At: now()}}
			}
			if !slices.EqualFunc(v.got, want, sameChange) {
				t.Errorf("told %+v, want %+v", v.got, want)
			}
		})
	}
}

func added(ws, user uuid.UUID, role shared.WorkspaceRole) func(e app.WorkspaceMemberEvents) error {
	return func(e app.WorkspaceMemberEvents) error {
		return e.MembershipAdded(context.Background(), app.WorkspaceMemberAdded{WorkspaceID: ws, UserID: user, Role: role, By: user, At: now()})
	}
}

func changed(ws, user uuid.UUID, from, to shared.WorkspaceRole) func(e app.WorkspaceMemberEvents) error {
	return func(e app.WorkspaceMemberEvents) error {
		return e.MemberRoleChanged(context.Background(), app.WorkspaceRoleChanged{
			WorkspaceID: ws, UserID: user, From: from, To: to, By: uuid.NewV7(), At: now(),
		})
	}
}

// Two subscribers of the visibility (v0.1 design 13.1, item 21): both are
// told, in the order registered; the first error stops the dispatch and is
// the use case's, whose transaction rolls back.
func TestTwoVisibilitySubscribers(t *testing.T) {
	failure := errors.New("the subscriber failed")
	for _, tt := range []struct {
		name    string
		failing string
		told    []string
	}{
		{"both follow", "", []string{"VisibilityChanged a in tx", "VisibilityChanged b in tx"}},
		{"the first fails", "a", []string{"VisibilityChanged a in tx"}},
		{"the second fails", "b", []string{"VisibilityChanged a in tx", "VisibilityChanged b in tx"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRemoveMember, shared.WorkspaceMember, shared.NotebookAdmin)
			a, b := &fakeVisibility{recorder: f.rec, name: "a"}, &fakeVisibility{recorder: f.rec, name: "b"}
			for _, s := range []*fakeVisibility{a, b} {
				if s.name == tt.failing {
					s.err = failure
				}
			}

			err := f.removeMember(a, b).Execute(as(f.alice), f.bobM.ID)

			got := slices.DeleteFunc(slices.Clone(f.rec.calls), func(c string) bool { return len(c) < 17 || c[:17] != "VisibilityChanged" })
			if wantErr := map[bool]error{true: failure, false: nil}[tt.failing != ""]; !errors.Is(err, wantErr) ||
				!slices.Equal(got, tt.told) || f.tx.rolledBack != (tt.failing != "") {
				t.Errorf("Execute() = %v, told %q, rolled back %v; want %v, %q", err, got, f.tx.rolledBack, wantErr, tt.told)
			}
		})
	}
}
