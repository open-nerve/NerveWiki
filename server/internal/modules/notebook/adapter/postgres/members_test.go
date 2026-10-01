package postgresadapter_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// memberAt adds userID to notebookID with role, joined at createdAt, with
// the id given.
func memberAt(t *testing.T, s *postgresadapter.Store, id, notebookID, userID uuid.UUID, role shared.NotebookRole, createdAt time.Time,
) domain.Member {
	t.Helper()
	m := domain.Member{ID: id, NotebookID: notebookID, UserID: userID, Role: role, CreatedAt: createdAt}
	if err := s.AddMember(context.Background(), m, userID); err != nil {
		t.Fatal(err)
	}
	return m
}

// The list is the active members, by when they first joined, then by id:
// each pair is written against the order it must come in. Ended members and
// those of another notebook are not listed.
func TestListMembers(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	acme := newWorkspace(t, pool, "acme", alice)
	n := newNotebook(t, s, acme, "Engineering", shared.AccessNone, alice)
	other := newNotebook(t, s, acme, "Other", shared.AccessNone, alice)
	account := func(email string) uuid.UUID { return newAccount(t, pool, email) }
	joinedLast := memberAt(t, s, uuid.NewV7(), n.ID, account("last@corp.com"), shared.NotebookEditor, later())
	low, high := uuid.NewV7(), uuid.NewV7()
	second := memberAt(t, s, high, n.ID, account("second@corp.com"), shared.NotebookReader, now().Add(time.Minute))
	first := memberAt(t, s, low, n.ID, account("first@corp.com"), shared.NotebookReader, now().Add(time.Minute))
	ended := memberAt(t, s, uuid.NewV7(), n.ID, account("ended@corp.com"), shared.NotebookReader, now())
	exec(t, pool, "UPDATE notebook_members SET ended_at = $2 WHERE id = $1", ended.ID, now())
	addMember(t, s, other.ID, account("elsewhere@corp.com"), shared.NotebookReader, alice)

	got, err := s.ListMembers(ctx, n.ID)

	admin, _ := s.FindMemberOf(ctx, n.ID, alice)
	want := []domain.Member{admin, first, second, joinedLast}
	if err != nil || !slices.EqualFunc(got, want, sameMember) {
		t.Errorf("ListMembers() = %+v, %v; want %+v", got, err, want)
	}
}

// sameMember compares two memberships, their times by the instant.
func sameMember(a, b domain.Member) bool {
	endedAlike := (a.EndedAt == nil) == (b.EndedAt == nil) && (a.EndedAt == nil || a.EndedAt.Equal(*b.EndedAt))
	return a.ID == b.ID && a.NotebookID == b.NotebookID && a.UserID == b.UserID && a.Role == b.Role &&
		a.CreatedAt.Equal(b.CreatedAt) && endedAlike
}

// FindActiveMember finds an active membership alone; FindMemberOf finds the
// pair's row, ended or not, and neither finds the rows of a deleted
// notebook.
func TestFindMembers(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	acme := newWorkspace(t, pool, "acme", alice)
	n := newNotebook(t, s, acme, "Engineering", shared.AccessNone, alice)
	bobs := memberAt(t, s, uuid.NewV7(), n.ID, bob, shared.NotebookEditor, now())
	gone := newNotebook(t, s, acme, "Gone", shared.AccessNone, alice)
	gonesAdmin, _ := s.FindMemberOf(ctx, gone.ID, alice)
	exec(t, pool, "UPDATE notebooks SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	exec(t, pool, "UPDATE notebook_members SET deleted_at = $2 WHERE notebook_id = $1", gone.ID, now())

	if got, err := s.FindActiveMember(ctx, bobs.ID); err != nil || !sameMember(got, bobs) {
		t.Errorf("FindActiveMember(active) = %+v, %v; want %+v", got, err, bobs)
	}
	exec(t, pool, "UPDATE notebook_members SET ended_at = $2 WHERE id = $1", bobs.ID, later())
	if got, err := s.FindActiveMember(ctx, bobs.ID); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("FindActiveMember(ended) = %+v, %v; want ErrNotFound", got, err)
	}
	endedAt := later()
	if got, err := s.FindMemberOf(ctx, n.ID, bob); err != nil || !sameMember(got, domain.Member{
		ID: bobs.ID, NotebookID: n.ID, UserID: bob, Role: shared.NotebookEditor, CreatedAt: now(), EndedAt: &endedAt,
	}) {
		t.Errorf("FindMemberOf(ended) = %+v, %v; want bob's, ended at %v", got, err, endedAt)
	}
	for name, find := range map[string]func() (domain.Member, error){
		"FindMemberOf(never a member)":  func() (domain.Member, error) { return s.FindMemberOf(ctx, gone.ID, bob) },
		"FindMemberOf(deleted)":         func() (domain.Member, error) { return s.FindMemberOf(ctx, gone.ID, alice) },
		"FindActiveMember(deleted)":     func() (domain.Member, error) { return s.FindActiveMember(ctx, gonesAdmin.ID) },
		"FindActiveMember(no such row)": func() (domain.Member, error) { return s.FindActiveMember(ctx, uuid.NewV7()) },
	} {
		if got, err := find(); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("%s = %+v, %v; want ErrNotFound", name, got, err)
		}
	}
}

// Only the notebook's active admins count.
func TestCountAdmins(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	acme := newWorkspace(t, pool, "acme", alice)
	n := newNotebook(t, s, acme, "Engineering", shared.AccessNone, alice)
	memberAt(t, s, uuid.NewV7(), n.ID, newAccount(t, pool, "bob@corp.com"), shared.NotebookAdmin, now())
	memberAt(t, s, uuid.NewV7(), n.ID, newAccount(t, pool, "carol@corp.com"), shared.NotebookEditor, now())
	left := memberAt(t, s, uuid.NewV7(), n.ID, newAccount(t, pool, "dan@corp.com"), shared.NotebookAdmin, now())
	exec(t, pool, "UPDATE notebook_members SET ended_at = $2 WHERE id = $1", left.ID, now())
	other := newNotebook(t, s, acme, "Other", shared.AccessNone, alice)
	memberAt(t, s, uuid.NewV7(), other.ID, newAccount(t, pool, "erin@corp.com"), shared.NotebookAdmin, now())

	if got, err := s.CountAdmins(ctx, n.ID); err != nil || got != 2 {
		t.Errorf("CountAdmins() = %d, %v; want 2: alice and bob", got, err)
	}
}

// A role change, an end and a restore write their author and time; a
// restore makes the row active with the role, and keeps when the account
// first joined.
func TestMemberWrites(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	n := newNotebook(t, s, newWorkspace(t, pool, "acme", alice), "Engineering", shared.AccessNone, alice)
	m := memberAt(t, s, uuid.NewV7(), n.ID, bob, shared.NotebookReader, now())
	step := func(minutes int) time.Time { return now().Add(time.Duration(minutes) * time.Minute) }
	check := func(what string, role shared.NotebookRole, endedAt *time.Time, at time.Time) {
		t.Helper()
		got, err := s.FindMemberOf(ctx, n.ID, bob)
		if want := (domain.Member{ID: m.ID, NotebookID: n.ID, UserID: bob, Role: role, CreatedAt: now(), EndedAt: endedAt}); err != nil ||
			!sameMember(got, want) {
			t.Errorf("after %s: %+v, %v; want %+v", what, got, err, want)
		}
		if r, want := readRow(t, pool, "notebook_members", m.ID), (row{CreatedBy: bob, UpdatedBy: alice, CreatedAt: now(), UpdatedAt: at}); !sameRow(r, want) {
			t.Errorf("after %s: the row = %+v, want %+v", what, r, want)
		}
	}

	if err := s.UpdateMemberRole(ctx, m.ID, shared.NotebookEditor, alice, step(1)); err != nil {
		t.Fatal(err)
	}
	check("the role change", shared.NotebookEditor, nil, step(1))
	if err := s.EndMember(ctx, m.ID, alice, step(2)); err != nil {
		t.Fatal(err)
	}
	ended := step(2)
	check("the end", shared.NotebookEditor, &ended, step(2))
	if err := s.RestoreMember(ctx, m.ID, shared.NotebookAdmin, alice, step(3)); err != nil {
		t.Fatal(err)
	}
	check("the restore", shared.NotebookAdmin, nil, step(3))
}
