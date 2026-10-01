package app_test

import (
	"context"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fakeMembers keeps every membership, ended ones too, by id. A membership
// in vanishing ends once it is first found: it ended between the unlocked
// read and the lock.
type fakeMembers struct {
	*recorder
	rows      map[uuid.UUID]domain.Member
	vanishing map[uuid.UUID]bool
}

func (f *fakeMembers) ListMembers(ctx context.Context, notebookID uuid.UUID) ([]domain.Member, error) {
	f.record(ctx, "ListMembers")
	var list []domain.Member
	for _, m := range f.rows {
		if m.NotebookID == notebookID && m.Active() {
			list = append(list, m)
		}
	}
	slices.SortFunc(list, func(a, b domain.Member) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return list, nil
}

func (f *fakeMembers) FindActiveMember(ctx context.Context, id uuid.UUID) (domain.Member, error) {
	f.record(ctx, "FindActiveMember")
	m, ok := f.rows[id]
	if !ok || !m.Active() {
		return domain.Member{}, app.ErrNotFound
	}
	if f.vanishing[id] {
		ended := now()
		m.EndedAt = &ended
		f.rows[id] = m
		delete(f.vanishing, id)
		m.EndedAt = nil
	}
	return m, nil
}

func (f *fakeMembers) FindMemberOf(ctx context.Context, notebookID, userID uuid.UUID) (domain.Member, error) {
	f.record(ctx, "FindMemberOf")
	for _, m := range f.rows {
		if m.NotebookID == notebookID && m.UserID == userID {
			return m, nil
		}
	}
	return domain.Member{}, app.ErrNotFound
}

func (f *fakeMembers) CountAdmins(ctx context.Context, notebookID uuid.UUID) (int, error) {
	f.record(ctx, "CountAdmins")
	n := 0
	for _, m := range f.rows {
		if m.NotebookID == notebookID && m.Active() && m.Role == shared.NotebookAdmin {
			n++
		}
	}
	return n, nil
}

func (f *fakeMembers) AddMember(ctx context.Context, m domain.Member, by uuid.UUID) error {
	f.record(ctx, "AddMember "+string(m.Role)+" by "+by.String())
	f.rows[m.ID] = m
	return nil
}

func (f *fakeMembers) UpdateMemberRole(ctx context.Context, id uuid.UUID, role shared.NotebookRole, by uuid.UUID, at time.Time) error {
	f.record(ctx, "UpdateMemberRole "+string(role)+" by "+by.String()+at.Format(" at 15:04:05.000000"))
	m := f.rows[id]
	m.Role = role
	f.rows[id] = m
	return nil
}

func (f *fakeMembers) EndMember(ctx context.Context, id, by uuid.UUID, at time.Time) error {
	f.record(ctx, "EndMember by "+by.String()+at.Format(" at 15:04:05.000000"))
	m := f.rows[id]
	m.EndedAt = &at
	f.rows[id] = m
	return nil
}

func (f *fakeMembers) RestoreMember(ctx context.Context, id uuid.UUID, role shared.NotebookRole, by uuid.UUID, at time.Time) error {
	f.record(ctx, "RestoreMember "+string(role)+" by "+by.String()+at.Format(" at 15:04:05.000000"))
	m := f.rows[id]
	m.Role, m.EndedAt = role, nil
	f.rows[id] = m
	return nil
}

// fakeWorkspaceMembers answers the roles of acme's active members.
type fakeWorkspaceMembers struct {
	*recorder
	roles map[uuid.UUID]shared.WorkspaceRole
}

func (f fakeWorkspaceMembers) RoleOf(ctx context.Context, _, userID uuid.UUID) (shared.WorkspaceRole, bool, error) {
	f.record(ctx, "RoleOf")
	r, ok := f.roles[userID]
	return r, ok, nil
}

// fakeProfiles answers every account's profile: its name, and its email
// made of it; or err when set.
type fakeProfiles struct {
	*recorder
	names map[uuid.UUID]string
	err   error
}

func (f *fakeProfiles) MemberProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]app.Profile, error) {
	f.record(ctx, "MemberProfiles")
	if f.err != nil {
		return nil, f.err
	}
	out := map[uuid.UUID]app.Profile{}
	for _, id := range ids {
		if name, ok := f.names[id]; ok {
			out[id] = app.Profile{DisplayName: name, Email: emailOf(name)}
		}
	}
	return out, nil
}

// emailOf is the email fakeProfiles gives the account named name.
func emailOf(name string) string { return strings.ToLower(name) + "@corp.com" }

// fakeVisibility records the visibility changes it follows, and answers
// err.
type fakeVisibility struct {
	*recorder
	name string
	err  error
	got  []app.VisibilityChange
}

func (f *fakeVisibility) VisibilityChanged(ctx context.Context, v app.VisibilityChange) error {
	f.record(ctx, "VisibilityChanged "+f.name)
	f.got = append(f.got, v)
	return f.err
}

// team is the notebook's people: alice its admin, bob an editor, carol a
// reader who left; dana a guest of acme and no member of the notebook;
// erin no member of acme. Each is a member of acme but erin.
type team struct {
	members                       *fakeMembers
	wsMembers                     fakeWorkspaceMembers
	profiles                      *fakeProfiles
	visibility                    *fakeVisibility
	alice, bob, carol, dana, erin uuid.UUID
	aliceM, bobM, carolM          domain.Member
}

func newTeam(rec *recorder, n domain.Notebook) team {
	tm := team{alice: uuid.NewV7(), bob: uuid.NewV7(), carol: uuid.NewV7(), dana: uuid.NewV7(), erin: uuid.NewV7()}
	joined := now().Add(-time.Hour)
	member := func(userID uuid.UUID, role shared.NotebookRole, minutes int) domain.Member {
		return domain.Member{ID: uuid.NewV7(), NotebookID: n.ID, UserID: userID, Role: role, CreatedAt: joined.Add(time.Duration(minutes) * time.Minute)}
	}
	tm.aliceM, tm.bobM, tm.carolM = member(tm.alice, shared.NotebookAdmin, 0), member(tm.bob, shared.NotebookEditor, 1),
		member(tm.carol, shared.NotebookReader, 2)
	left := joined.Add(30 * time.Minute)
	tm.carolM.EndedAt = &left
	tm.members = &fakeMembers{recorder: rec, vanishing: map[uuid.UUID]bool{}, rows: map[uuid.UUID]domain.Member{
		tm.aliceM.ID: tm.aliceM, tm.bobM.ID: tm.bobM, tm.carolM.ID: tm.carolM,
	}}
	tm.wsMembers = fakeWorkspaceMembers{recorder: rec, roles: map[uuid.UUID]shared.WorkspaceRole{
		tm.alice: shared.WorkspaceMember, tm.bob: shared.WorkspaceMember, tm.carol: shared.WorkspaceMember, tm.dana: shared.WorkspaceGuest,
	}}
	tm.profiles = &fakeProfiles{recorder: rec, names: map[uuid.UUID]string{
		tm.alice: "Alice", tm.bob: "Bob", tm.carol: "Carol", tm.dana: "Dana", tm.erin: "Erin",
	}}
	tm.visibility = &fakeVisibility{recorder: rec, name: "visibility"}
	return tm
}
