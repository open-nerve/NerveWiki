package app_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func (f fixture) listMembers() *app.ListMembers {
	return app.NewListMembers(app.ListMembersDeps{Notebooks: f.store, Members: f.members, Profiles: f.profiles, Auth: f.auth})
}

func (f fixture) addMember(subscribers ...app.VisibilitySubscriber) *app.AddMember {
	return app.NewAddMember(app.AddMemberDeps{
		Workspaces: f.workspaces, WorkspaceMembers: f.wsMembers, Finder: f.store, Notebooks: f.store, Members: f.members,
		Profiles: f.profiles, Subscribers: subscribers, Auth: f.auth, Tx: f.tx, Clock: &tickingClock{}, Logger: f.logger(),
	})
}

func (f fixture) updateMember() *app.UpdateMember {
	return app.NewUpdateMember(app.UpdateMemberDeps{
		Workspaces: f.workspaces, Finder: f.store, Notebooks: f.store, Members: f.members, Writer: f.members, Profiles: f.profiles,
		Auth: f.auth, Tx: f.tx, Clock: &tickingClock{}, Logger: f.logger(),
	})
}

func (f fixture) removeMember(subscribers ...app.VisibilitySubscriber) *app.RemoveMember {
	return app.NewRemoveMember(app.RemoveMemberDeps{
		Workspaces: f.workspaces, Finder: f.store, Notebooks: f.store, Members: f.members, Writer: f.members,
		Subscribers: subscribers, Auth: f.auth, Tx: f.tx, Clock: &tickingClock{}, Logger: f.logger(),
	})
}

func (f fixture) leave(subscribers ...app.VisibilitySubscriber) *app.LeaveNotebook {
	return app.NewLeaveNotebook(app.LeaveNotebookDeps{
		Workspaces: f.workspaces, Finder: f.store, Notebooks: f.store, Members: f.members, Subscribers: subscribers,
		Auth: f.auth, Tx: f.tx, Clock: &tickingClock{}, Logger: f.logger(),
	})
}

// listed is m as the list shows it to a caller who sees emails, or not.
func listed(m domain.Member, name string, emails bool) app.ListedMember {
	l := app.ListedMember{Member: m, DisplayName: name}
	if emails {
		email := emailOf(name)
		l.Email = &email
	}
	return l
}

// sameListed compares two listed members, the emails by value.
func sameListed(a, b app.ListedMember) bool {
	emailsAlike := (a.Email == nil) == (b.Email == nil) && (a.Email == nil || *a.Email == *b.Email)
	endedAlike := (a.EndedAt == nil) == (b.EndedAt == nil)
	a.Email, b.Email, a.EndedAt, b.EndedAt = nil, nil, nil, nil
	return a == b && emailsAlike && endedAlike
}

// inTx is calls, each made in the transaction.
func inTx(calls ...string) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c + " in tx"
	}
	return out
}

// at is " by <id> at <the ticking clock's first read>", as the fakes record
// a write.
func at(by uuid.UUID) string { return " by " + by.String() + now().Format(" at 15:04:05.000000") }

// The list is the active members, by when they joined, with their
// profiles: the emails to the workspace's admins and members, not to its
// guests. A read: no transaction.
func TestListMembers(t *testing.T) {
	for _, tt := range []struct {
		ws     shared.WorkspaceRole
		emails bool
	}{{shared.WorkspaceMember, true}, {shared.WorkspaceAdmin, true}, {shared.WorkspaceGuest, false}} {
		t.Run(string(tt.ws), func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionListMembers, tt.ws, shared.NotebookReader)

			got, err := f.listMembers().Execute(as(f.bob), f.notebook.ID)

			want := []app.ListedMember{listed(f.aliceM, "Alice", tt.emails), listed(f.bobM, "Bob", tt.emails)}
			if err != nil || !slices.EqualFunc(got, want, sameListed) {
				t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
			}
			if calls := []string{"FindNotebook", "Authorize notebook_member.list", "ListMembers", "MemberProfiles"}; !slices.Equal(f.rec.calls, calls) {
				t.Errorf("calls = %q, want %q", f.rec.calls, calls)
			}
		})
	}
}

func TestListMembersOfANotebookNotSeen(t *testing.T) {
	for name, id := range map[string]func(f fixture) uuid.UUID{
		"no such notebook": func(fixture) uuid.UUID { return uuid.NewV7() },
		"not visible":      func(f fixture) uuid.UUID { return f.notebook.ID },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture()
			if got, err := f.listMembers().Execute(as(f.dana), id(f)); !errors.Is(err, domain.ErrNotFound) ||
				slices.Contains(f.rec.calls, "ListMembers") {
				t.Errorf("Execute() = %+v, %v, calls %q; want notebook.not_found, no list", got, err, f.rec.calls)
			}
		})
	}
}

// A new member is added, a former one restored keeping when it first
// joined, under the locks, after the decision and the checks; the
// visibility's subscribers are told of the account in the transaction,
// and the profile read after the write.
func TestAddMember(t *testing.T) {
	for _, tt := range []struct {
		name  string
		user  func(f fixture) uuid.UUID
		write func(f fixture) string
		want  func(f fixture, got app.ListedMember) app.ListedMember
	}{
		{"a guest of the workspace, never a member", func(f fixture) uuid.UUID { return f.dana },
			func(f fixture) string { return "AddMember reader by " + f.alice.String() },
			func(f fixture, got app.ListedMember) app.ListedMember {
				return listed(domain.Member{ID: got.ID, NotebookID: f.notebook.ID, UserID: f.dana, Role: shared.NotebookReader, CreatedAt: now()},
					"Dana", true)
			}},
		{"a member once", func(f fixture) uuid.UUID { return f.carol },
			func(f fixture) string { return "RestoreMember reader" + at(f.alice) },
			func(f fixture, _ app.ListedMember) app.ListedMember {
				m := f.carolM
				m.EndedAt = nil
				return listed(m, "Carol", true)
			}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionAddMember, shared.WorkspaceMember, shared.NotebookAdmin)
			user := tt.user(f)

			got, err := f.addMember(f.visibility).Execute(as(f.alice), f.notebook.ID, user, "reader")

			if want := tt.want(f, got); err != nil || !sameListed(got, want) || got.ID == uuid.Nil() {
				t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
			}
			if m := f.members.rows[got.ID]; !m.Active() || m.Role != shared.NotebookReader {
				t.Errorf("the membership = %+v, want an active reader", m)
			}
			calls := append([]string{"FindNotebook"}, inTx("ShareByID", "LockNotebook", "Authorize notebook_member.add", "RoleOf",
				"FindMemberOf", tt.write(f), "VisibilityChanged visibility", "MemberProfiles")...)
			if !slices.Equal(f.rec.calls, calls) {
				t.Errorf("calls = %q, want %q", f.rec.calls, calls)
			}
			v := app.VisibilityChange{WorkspaceID: f.acme, UserIDs: []uuid.UUID{user}, At: now()}
			if len(f.visibility.got) != 1 || !sameChange(f.visibility.got[0], v) {
				t.Errorf("visibility told %+v, want %+v", f.visibility.got, v)
			}
			logs := f.logs.String()
			if !strings.Contains(logs, "notebook member added") || !strings.Contains(logs, got.ID.String()) || strings.Contains(logs, "@") {
				t.Errorf("logs = %q, want the addition by its ids, no email", logs)
			}
		})
	}
}

// sameChange compares two visibility changes.
func sameChange(a, b app.VisibilityChange) bool {
	return a.WorkspaceID == b.WorkspaceID && slices.Equal(a.UserIDs, b.UserIDs) && a.Reached == b.Reached && a.At.Equal(b.At)
}

// The codes come 404, 403, 422 (v0.1 design 13.1, item 4): the values are
// checked under the locks and after the decision, every problem at once;
// nothing is written or told when any refuses.
func TestAddMemberRefusals(t *testing.T) {
	for _, tt := range []struct {
		name     string
		notebook func(f fixture) uuid.UUID
		user     func(f fixture) uuid.UUID
		role     string
		grant    bool
		nb       shared.NotebookRole
		want     error
		problems []string
	}{
		{"no such notebook", func(fixture) uuid.UUID { return uuid.NewV7() }, func(f fixture) uuid.UUID { return f.dana }, "reader",
			true, shared.NotebookAdmin, domain.ErrNotFound, nil},
		{"not visible", nil, func(f fixture) uuid.UUID { return f.dana }, "bogus", false, "", domain.ErrNotFound, nil},
		{"an editor", nil, func(f fixture) uuid.UUID { return f.dana }, "bogus", true, forbidden, shared.Forbidden(), nil},
		{"no member of the workspace", nil, func(f fixture) uuid.UUID { return f.erin }, "reader", true, shared.NotebookAdmin,
			nil, []string{"user_id not_allowed"}},
		{"a member already", nil, func(f fixture) uuid.UUID { return f.bob }, "reader", true, shared.NotebookAdmin,
			nil, []string{"user_id duplicate"}},
		{"a role unknown", nil, func(f fixture) uuid.UUID { return f.dana }, "owner", true, shared.NotebookAdmin,
			nil, []string{"role invalid_format"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			if tt.grant {
				f.grant(domain.ActionAddMember, shared.WorkspaceMember, tt.nb)
			}
			if tt.nb == forbidden {
				f.auth.err = shared.Forbidden()
			}
			id := f.notebook.ID
			if tt.notebook != nil {
				id = tt.notebook(f)
			}

			_, err := f.addMember(f.visibility).Execute(as(f.alice), id, tt.user(f), tt.role)

			if tt.problems != nil {
				if got := problems(t, err); !slices.Equal(got, tt.problems) {
					t.Errorf("problems = %q, want %q", got, tt.problems)
				}
			} else if !errors.Is(err, tt.want) {
				t.Errorf("Execute() = %v, want %v", err, tt.want)
			}
			if wrote := slices.ContainsFunc(f.rec.calls, isMemberWrite); wrote || len(f.visibility.got) != 0 {
				t.Errorf("calls = %q, told %+v; want no write, nothing told", f.rec.calls, f.visibility.got)
			}
		})
	}
}

// forbidden stands for a role the rule refuses: the fake authorizer
// answers 403, as the access module's decision would.
const forbidden shared.NotebookRole = "a role the rule refuses"

// problems are the field problems of err, a 422, as "field code".
func problems(t *testing.T, err error) []string {
	t.Helper()
	var se *shared.Error
	if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed {
		t.Fatalf("err = %v, want validation_failed", err)
	}
	var list []string
	for _, f := range se.Fields {
		list = append(list, f.Field+" "+f.Code)
	}
	return list
}

// isMemberWrite reports whether call writes a membership.
func isMemberWrite(call string) bool {
	for _, w := range []string{"AddMember", "RestoreMember", "UpdateMemberRole", "EndMember"} {
		if strings.HasPrefix(call, w) {
			return true
		}
	}
	return false
}

// The membership is read unlocked for its notebook, whose workspace and
// row are locked; then the decision, the membership read again, the role
// checked, and the write. A role in the notebook changes no one's
// visibility.
func TestUpdateMember(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionUpdateMember, shared.WorkspaceGuest, shared.NotebookAdmin)

	got, err := f.updateMember().Execute(as(f.alice), f.bobM.ID, "reader")

	want := f.bobM
	want.Role = shared.NotebookReader
	if err != nil || !sameListed(got, listed(want, "Bob", false)) {
		t.Errorf("Execute() = %+v, %v; want bob as a reader, no email to a guest", got, err)
	}
	calls := append([]string{"FindActiveMember", "FindNotebook"}, inTx("ShareByID", "LockNotebook",
		"Authorize notebook_member.update", "FindActiveMember", "UpdateMemberRole reader"+at(f.alice), "MemberProfiles")...)
	if !slices.Equal(f.rec.calls, calls) {
		t.Errorf("calls = %q, want %q", f.rec.calls, calls)
	}
	if target := (shared.Target{WorkspaceID: f.acme, NotebookID: f.notebook.ID}); f.auth.targets[0] != target {
		t.Errorf("target = %+v, want %+v", f.auth.targets[0], target)
	}
	if !strings.Contains(f.logs.String(), "notebook member updated") {
		t.Errorf("logs = %q, want the update", f.logs)
	}
}

// A membership not there, ended, ended meanwhile, or whose notebook the
// caller cannot see is member_not_found; the role is checked after the
// decision, then the caller's own membership; none of them writes.
func TestUpdateMemberRefusals(t *testing.T) {
	for _, tt := range []struct {
		name   string
		member func(f fixture) uuid.UUID
		role   string
		grant  bool
		nb     shared.NotebookRole
		setUp  func(f fixture)
		want   error
	}{
		{"no such membership", func(fixture) uuid.UUID { return uuid.NewV7() }, "reader", true, shared.NotebookAdmin, nil, domain.ErrMemberNotFound},
		{"an ended membership", func(f fixture) uuid.UUID { return f.carolM.ID }, "reader", true, shared.NotebookAdmin, nil, domain.ErrMemberNotFound},
		{"ended meanwhile", func(f fixture) uuid.UUID { return f.bobM.ID }, "reader", true, shared.NotebookAdmin,
			func(f fixture) { f.members.vanishing[f.bobM.ID] = true }, domain.ErrMemberNotFound},
		{"its notebook deleted meanwhile", func(f fixture) uuid.UUID { return f.bobM.ID }, "reader", true, shared.NotebookAdmin,
			func(f fixture) { f.store.deleted[f.notebook.ID] = true }, domain.ErrMemberNotFound},
		{"its notebook not visible", func(f fixture) uuid.UUID { return f.bobM.ID }, "bogus", false, "", nil, domain.ErrMemberNotFound},
		{"an editor", func(f fixture) uuid.UUID { return f.bobM.ID }, "bogus", true, forbidden, nil, shared.Forbidden()},
		{"a role unknown, one's own", func(f fixture) uuid.UUID { return f.aliceM.ID }, "owner", true, shared.NotebookAdmin, nil,
			shared.Invalid()},
		{"one's own", func(f fixture) uuid.UUID { return f.aliceM.ID }, "reader", true, shared.NotebookAdmin, nil, domain.ErrOwnMembership},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			if tt.grant {
				f.grant(domain.ActionUpdateMember, shared.WorkspaceMember, tt.nb)
			}
			if tt.nb == forbidden {
				f.auth.err = shared.Forbidden()
			}
			if tt.setUp != nil {
				tt.setUp(f)
			}

			_, err := f.updateMember().Execute(as(f.alice), tt.member(f), tt.role)

			if !errors.Is(err, tt.want) || slices.ContainsFunc(f.rec.calls, isMemberWrite) {
				t.Errorf("Execute() = %v, calls %q; want %v, no write", err, f.rec.calls, tt.want)
			}
		})
	}
}

// A removal ends the membership under the lock and tells the visibility's
// subscribers of the account.
func TestRemoveMember(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRemoveMember, shared.WorkspaceMember, shared.NotebookAdmin)

	if err := f.removeMember(f.visibility).Execute(as(f.alice), f.bobM.ID); err != nil {
		t.Fatal(err)
	}

	if m := f.members.rows[f.bobM.ID]; m.Active() {
		t.Errorf("bob's membership = %+v, want ended", m)
	}
	calls := append([]string{"FindActiveMember", "FindNotebook"}, inTx("ShareByID", "LockNotebook",
		"Authorize notebook_member.remove", "FindActiveMember", "EndMember"+at(f.alice), "VisibilityChanged visibility")...)
	if !slices.Equal(f.rec.calls, calls) {
		t.Errorf("calls = %q, want %q", f.rec.calls, calls)
	}
	v := app.VisibilityChange{WorkspaceID: f.acme, UserIDs: []uuid.UUID{f.bob}, At: now()}
	if len(f.visibility.got) != 1 || !sameChange(f.visibility.got[0], v) {
		t.Errorf("visibility told %+v, want %+v", f.visibility.got, v)
	}
	if !strings.Contains(f.logs.String(), "notebook member removed") {
		t.Errorf("logs = %q, want the removal", f.logs)
	}
}

func TestRemoveMemberRefusals(t *testing.T) {
	for _, tt := range []struct {
		name   string
		member func(f fixture) uuid.UUID
		grant  bool
		nb     shared.NotebookRole
		want   error
	}{
		{"an ended membership", func(f fixture) uuid.UUID { return f.carolM.ID }, true, shared.NotebookAdmin, domain.ErrMemberNotFound},
		{"its notebook not visible", func(f fixture) uuid.UUID { return f.bobM.ID }, false, "", domain.ErrMemberNotFound},
		{"a reader", func(f fixture) uuid.UUID { return f.bobM.ID }, true, forbidden, shared.Forbidden()},
		{"one's own", func(f fixture) uuid.UUID { return f.aliceM.ID }, true, shared.NotebookAdmin, domain.ErrOwnMembership},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			if tt.grant {
				f.grant(domain.ActionRemoveMember, shared.WorkspaceMember, tt.nb)
			}
			if tt.nb == forbidden {
				f.auth.err = shared.Forbidden()
			}

			err := f.removeMember(f.visibility).Execute(as(f.alice), tt.member(f))

			if !errors.Is(err, tt.want) || slices.ContainsFunc(f.rec.calls, isMemberWrite) || len(f.visibility.got) != 0 {
				t.Errorf("Execute() = %v, calls %q; want %v, no write, nothing told", err, f.rec.calls, tt.want)
			}
		})
	}
}

// A member leaves under the lock, the admins counted there (rule one), and
// the visibility's subscribers are told of the caller.
func TestLeaveNotebook(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionLeave, shared.WorkspaceMember, shared.NotebookEditor)

	if err := f.leave(f.visibility).Execute(as(f.bob), f.notebook.ID); err != nil {
		t.Fatal(err)
	}

	if m := f.members.rows[f.bobM.ID]; m.Active() {
		t.Errorf("bob's membership = %+v, want ended", m)
	}
	calls := append([]string{"FindNotebook"}, inTx("ShareByID", "LockNotebook", "Authorize notebook.leave", "FindMemberOf",
		"CountAdmins", "EndMember"+at(f.bob), "VisibilityChanged visibility")...)
	if !slices.Equal(f.rec.calls, calls) {
		t.Errorf("calls = %q, want %q", f.rec.calls, calls)
	}
	v := app.VisibilityChange{WorkspaceID: f.acme, UserIDs: []uuid.UUID{f.bob}, At: now()}
	if len(f.visibility.got) != 1 || !sameChange(f.visibility.got[0], v) {
		t.Errorf("visibility told %+v, want %+v", f.visibility.got, v)
	}
	if !strings.Contains(f.logs.String(), "notebook left") {
		t.Errorf("logs = %q, want the leave", f.logs)
	}
}

// Rule one: the only admin cannot leave, even alone in the notebook; with
// another admin it can. One that uses the notebook by its workspace access
// alone, or whose membership ended, has none to end; one with no role in
// it does not see it. Nothing is written or told when any refuses.
func TestLeaveNotebookRefusals(t *testing.T) {
	alone := func(f fixture) {
		m := f.members.rows[f.bobM.ID]
		ended := now()
		m.EndedAt = &ended
		f.members.rows[f.bobM.ID] = m
	}
	for _, tt := range []struct {
		name   string
		caller func(f fixture) uuid.UUID
		grant  bool
		setUp  func(f fixture)
		want   error
	}{
		{"the only admin", func(f fixture) uuid.UUID { return f.alice }, true, nil, domain.ErrSoleAdmin},
		{"the only admin, alone", func(f fixture) uuid.UUID { return f.alice }, true, alone, domain.ErrSoleAdmin},
		{"by the workspace access alone", func(f fixture) uuid.UUID { return f.dana }, true, nil, domain.ErrMemberNotFound},
		{"a membership ended", func(f fixture) uuid.UUID { return f.carol }, true, nil, domain.ErrMemberNotFound},
		{"not visible", func(f fixture) uuid.UUID { return f.bob }, false, nil, domain.ErrNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			if tt.grant {
				f.grant(domain.ActionLeave, shared.WorkspaceMember, shared.NotebookReader)
			}
			if tt.setUp != nil {
				tt.setUp(f)
			}

			err := f.leave(f.visibility).Execute(as(tt.caller(f)), f.notebook.ID)

			if !errors.Is(err, tt.want) || slices.ContainsFunc(f.rec.calls, isMemberWrite) || len(f.visibility.got) != 0 {
				t.Errorf("Execute() = %v, calls %q; want %v, no write, nothing told", err, f.rec.calls, tt.want)
			}
		})
	}
}

// With a second admin, the first may leave: the count is of the active
// admins under the lock.
func TestLeaveNotebookWithAnotherAdmin(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionLeave, shared.WorkspaceMember, shared.NotebookAdmin)
	m := f.members.rows[f.bobM.ID]
	m.Role = shared.NotebookAdmin
	f.members.rows[f.bobM.ID] = m

	if err := f.leave().Execute(as(f.alice), f.notebook.ID); err != nil || f.members.rows[f.aliceM.ID].Active() {
		t.Errorf("Execute() = %v, alice's membership %+v; want nil, ended", err, f.members.rows[f.aliceM.ID])
	}
}
