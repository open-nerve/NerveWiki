package postgresadapter_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
)

// orphan ends owner's membership of notebook id and makes it ownerless at
// at, as the end of a workspace membership does.
func (f holdingsFixture) orphan(t *testing.T, id, owner uuid.UUID, at time.Time) {
	t.Helper()
	if err := inTx(t, f.pool, func(ctx context.Context) error {
		if err := f.s.EndMembershipsOf(ctx, owner, []uuid.UUID{id}, owner, at); err != nil {
			return err
		}
		return f.s.SetOwnerless(ctx, []uuid.UUID{id}, owner, at)
	}); err != nil {
		t.Fatal(err)
	}
}

// The list is the workspace's ownerless notebooks not deleted, the earliest
// to become so first, then by id, each with its active members; the reads
// of one notebook carry its ownerless state.
func TestListOwnerless(t *testing.T) {
	f := newHoldingsFixture(t)
	ctx := context.Background()
	f.orphan(t, f.team, f.alice, later())
	f.orphan(t, f.solo, f.alice, now())
	f.orphan(t, f.lab, f.alice, now())
	f.orphan(t, f.shared, f.bob, later())
	exec(t, f.pool, "UPDATE notebooks SET deleted_at = $2 WHERE id = $1", f.shared, later())

	got, err := f.s.ListOwnerless(ctx, f.acme)

	if err != nil || len(got) != 2 || got[0].Notebook.ID != f.solo || got[1].Notebook.ID != f.team ||
		got[0].MemberCount != 0 || got[1].MemberCount != 1 {
		t.Fatalf("ListOwnerless(acme) = %+v, %v; want solo with no member, then team with bob", got, err)
	}
	if o := got[1].Notebook.Ownerless; o == nil || !o.Since.Equal(later()) || o.FormerOwner != f.alice {
		t.Errorf("team's ownerless state %+v, want alice's since %v", o, later())
	}
	// The same instant: the id orders them.
	f.orphan(t, f.lab, f.alice, now())
	exec(t, f.pool, "UPDATE notebooks SET workspace_id = $2, ownerless_since = $3 WHERE id = $1", f.lab, f.acme, now())
	if got, err := f.s.ListOwnerless(ctx, f.acme); err != nil || len(got) != 3 || got[0].Notebook.ID.Compare(got[1].Notebook.ID) > 0 {
		t.Errorf("ListOwnerless(acme) = %+v, %v; want the two of one instant by id", got, err)
	}

	locked := domain.Notebook{}
	if err := inTx(t, f.pool, func(ctx context.Context) error {
		var err error
		locked, err = f.s.LockNotebook(ctx, f.team)
		return err
	}); err != nil || locked.Ownerless == nil || locked.Ownerless.FormerOwner != f.alice {
		t.Errorf("LockNotebook(team) = %+v, %v; want its ownerless state", locked, err)
	}
	if found, err := f.s.FindNotebook(ctx, f.shared); err == nil {
		t.Errorf("FindNotebook(a deleted one) = %+v", found)
	}
	if found, err := f.s.FindNotebook(ctx, f.solo); err != nil || found.Ownerless == nil {
		t.Errorf("FindNotebook(solo) = %+v, %v; want its ownerless state", found, err)
	}
}

// The audit events come newest first, then by id, a page at a time: those
// of the workspace not deleted.
func TestListAuditEvents(t *testing.T) {
	f := newHoldingsFixture(t)
	ctx := context.Background()
	event := func(workspaceID uuid.UUID, at time.Time) domain.AuditEvent {
		e := domain.AuditEvent{ID: uuid.NewV7(), WorkspaceID: workspaceID, NotebookID: f.team, NotebookName: "Team",
			Action: domain.AuditReturned, FormerOwnerID: f.alice, ActorID: f.alice, At: at}
		if err := f.s.AddAuditEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
		return e
	}
	older := event(f.acme, now())
	first, second := event(f.acme, later()), event(f.acme, later())
	event(f.other, later())
	gone := event(f.acme, later())
	exec(t, f.pool, "UPDATE notebook_audit_events SET deleted_at = $2 WHERE id = $1", gone.ID, later())

	page, err := f.s.ListAuditEvents(ctx, f.acme, nil, 2)
	if err != nil || len(page) != 2 || page[0].ID != second.ID || page[1].ID != first.ID {
		t.Fatalf("the first page = %+v, %v; want the two of later, the greater id first", page, err)
	}
	if e := page[0]; !e.At.Equal(later()) || e.Action != domain.AuditReturned || e.ActorID != f.alice || e.NotebookName != "Team" {
		t.Errorf("the event %+v, want its values back", e)
	}
	rest, err := f.s.ListAuditEvents(ctx, f.acme, &domain.AuditCursor{CreatedAt: page[1].At, ID: page[1].ID}, 2)
	if ids := idsOf(rest); err != nil || !slices.Equal(ids, []uuid.UUID{older.ID}) {
		t.Errorf("the page after %s = %v, %v; want the older one alone", page[1].ID, ids, err)
	}
}

func idsOf(events []domain.AuditEvent) []uuid.UUID {
	ids := make([]uuid.UUID, len(events))
	for i, e := range events {
		ids[i] = e.ID
	}
	return ids
}
