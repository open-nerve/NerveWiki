package postgresadapter_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// holdingsFixture is alice's notebooks: in acme, solo (hers alone), team
// (hers, with bob an editor, and carol a reader and dave an admin who
// left), and shared
// (bob's and dave's, alice a reader), written against their ids' order;
// one deleted, one she left and one without her, all hers in acme too; lab,
// hers in another workspace.
type holdingsFixture struct {
	s                       *postgresadapter.Store
	pool                    *pgxpool.Pool
	alice, bob, carol, dave uuid.UUID
	acme, other             uuid.UUID
	solo, team, shared, lab uuid.UUID
}

func newHoldingsFixture(t *testing.T) holdingsFixture {
	t.Helper()
	s, pool := newStore(t)
	f := holdingsFixture{s: s, pool: pool}
	f.alice, f.bob, f.carol, f.dave = newAccount(t, pool, "alice@corp.com"), newAccount(t, pool, "bob@corp.com"),
		newAccount(t, pool, "carol@corp.com"), newAccount(t, pool, "dave@corp.com")
	f.acme, f.other = newWorkspace(t, pool, "acme", f.alice), newWorkspace(t, pool, "lab", f.alice)
	ids := []uuid.UUID{uuid.NewV7(), uuid.NewV7(), uuid.NewV7()}
	f.solo, f.team, f.shared = ids[0], ids[1], ids[2]
	create := func(id, workspaceID, admin uuid.UUID) uuid.UUID {
		n := domain.Notebook{ID: id, WorkspaceID: workspaceID, Name: "Notes", Access: shared.AccessNone, CreatedAt: now(), UpdatedAt: now()}
		if err := s.CreateNotebook(context.Background(), n, admin); err != nil {
			t.Fatal(err)
		}
		addMember(t, s, id, admin, shared.NotebookAdmin, admin)
		return id
	}
	create(f.shared, f.acme, f.bob)
	addMember(t, s, f.shared, f.dave, shared.NotebookAdmin, f.bob)
	addMember(t, s, f.shared, f.alice, shared.NotebookReader, f.bob)
	create(f.team, f.acme, f.alice)
	addMember(t, s, f.team, f.bob, shared.NotebookEditor, f.alice)
	for _, m := range []uuid.UUID{
		addMember(t, s, f.team, f.carol, shared.NotebookReader, f.alice), addMember(t, s, f.team, f.dave, shared.NotebookAdmin, f.alice),
	} {
		if err := s.EndMember(context.Background(), m, f.alice, now()); err != nil {
			t.Fatal(err)
		}
	}
	create(f.solo, f.acme, f.alice)
	f.lab = create(uuid.NewV7(), f.other, f.alice)

	gone := create(uuid.NewV7(), f.acme, f.alice)
	exec(t, pool, "UPDATE notebooks SET deleted_at = $2 WHERE id = $1", gone, now())
	exec(t, pool, "UPDATE notebook_members SET deleted_at = $2 WHERE notebook_id = $1", gone, now())
	left := create(uuid.NewV7(), f.acme, f.bob)
	alicesLeft := addMember(t, s, left, f.alice, shared.NotebookAdmin, f.bob)
	if err := s.EndMember(context.Background(), alicesLeft, f.alice, now()); err != nil {
		t.Fatal(err)
	}
	create(uuid.NewV7(), f.acme, f.bob)
	return f
}

// The holdings are alice's active memberships of the workspaces'
// notebooks not deleted, by id, with the active admins and members.
func TestLockHoldings(t *testing.T) {
	f := newHoldingsFixture(t)

	for _, tt := range []struct {
		workspaces []uuid.UUID
		want       []domain.Holding
	}{
		{[]uuid.UUID{f.acme}, []domain.Holding{
			{NotebookID: f.solo, WorkspaceID: f.acme, Role: shared.NotebookAdmin, Admins: 1, Members: 1},
			{NotebookID: f.team, WorkspaceID: f.acme, Role: shared.NotebookAdmin, Admins: 1, Members: 2},
			{NotebookID: f.shared, WorkspaceID: f.acme, Role: shared.NotebookReader, Admins: 2, Members: 3},
		}},
		{[]uuid.UUID{f.acme, f.other}, []domain.Holding{
			{NotebookID: f.solo, WorkspaceID: f.acme, Role: shared.NotebookAdmin, Admins: 1, Members: 1},
			{NotebookID: f.team, WorkspaceID: f.acme, Role: shared.NotebookAdmin, Admins: 1, Members: 2},
			{NotebookID: f.shared, WorkspaceID: f.acme, Role: shared.NotebookReader, Admins: 2, Members: 3},
			{NotebookID: f.lab, WorkspaceID: f.other, Role: shared.NotebookAdmin, Admins: 1, Members: 1},
		}},
	} {
		var got []domain.Holding
		err := inTx(t, f.pool, func(ctx context.Context) error {
			var err error
			got, err = f.s.LockHoldings(ctx, f.alice, tt.workspaces)
			return err
		})
		if err != nil || !slices.Equal(got, tt.want) {
			t.Errorf("LockHoldings(%d workspaces) = %+v, %v; want %+v", len(tt.workspaces), got, err, tt.want)
		}
	}
}

// The holdings' notebooks are locked by id, the order of every write that
// holds more than one: waiting for a higher one, it holds the lower.
func TestLockHoldingsLocksThemByID(t *testing.T) {
	ctx := context.Background()
	f := newHoldingsFixture(t)
	holder, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, "SELECT 1 FROM notebooks WHERE id = $1 FOR SHARE", f.shared); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- inTx(t, f.pool, func(ctx context.Context) error {
			_, err := f.s.LockHoldings(ctx, f.alice, []uuid.UUID{f.acme})
			return err
		})
	}()
	pgtest.WaitForLockWaitsOn(t, f.pool, "notebooks", 1, 10*time.Second)
	probe, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, held := probe.Exec(ctx, "SELECT 1 FROM notebooks WHERE id = $1 FOR SHARE NOWAIT", f.team)
	_ = probe.Rollback(ctx)
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if held == nil {
		t.Error("a lower id was free while the lock waited for the highest, want it locked first")
	}
}

// memberState is a membership's role, end and last update.
type memberState struct {
	Role      string
	EndedAt   *time.Time
	UpdatedBy uuid.UUID
	UpdatedAt time.Time
}

func memberOf(t *testing.T, pool *pgxpool.Pool, notebookID, userID uuid.UUID) memberState {
	t.Helper()
	var m memberState
	if err := pool.QueryRow(context.Background(), "SELECT role, ended_at, updated_by_id, updated_at FROM notebook_members "+
		"WHERE notebook_id = $1 AND user_id = $2", notebookID, userID).Scan(&m.Role, &m.EndedAt, &m.UpdatedBy, &m.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	return m
}

// ownership is a notebook's ownerless columns and last update.
type ownership struct {
	Since       *time.Time
	FormerOwner *uuid.UUID
	UpdatedBy   uuid.UUID
	UpdatedAt   time.Time
}

func ownershipOf(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) ownership {
	t.Helper()
	var o ownership
	if err := pool.QueryRow(context.Background(), "SELECT ownerless_since, former_owner_id, updated_by_id, updated_at FROM notebooks WHERE id = $1",
		id).Scan(&o.Since, &o.FormerOwner, &o.UpdatedBy, &o.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	return o
}

// The end writes alice's memberships of the notebooks alone, and the
// ownerless notebooks keep their last update: it is their last activity.
func TestEndMembershipsAndSetOwnerless(t *testing.T) {
	f := newHoldingsFixture(t)
	carolBefore := memberOf(t, f.pool, f.team, f.carol)

	err := inTx(t, f.pool, func(ctx context.Context) error {
		if err := f.s.EndMembershipsOf(ctx, f.alice, []uuid.UUID{f.solo, f.team}, f.dave, later()); err != nil {
			return err
		}
		return f.s.SetOwnerless(ctx, []uuid.UUID{f.solo, f.team}, f.alice, later())
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []uuid.UUID{f.solo, f.team} {
		if m := memberOf(t, f.pool, id, f.alice); m.EndedAt == nil || !m.EndedAt.Equal(later()) || m.UpdatedBy != f.dave || !m.UpdatedAt.Equal(later()) {
			t.Errorf("alice's membership %+v, want ended by dave at %v", m, later())
		}
		if o := ownershipOf(t, f.pool, id); o.Since == nil || !o.Since.Equal(later()) || o.FormerOwner == nil || *o.FormerOwner != f.alice ||
			o.UpdatedBy != f.alice || !o.UpdatedAt.Equal(now()) {
			t.Errorf("the notebook %+v, want ownerless since %v of alice, its last update alice's at %v", o, later(), now())
		}
	}
	if m := memberOf(t, f.pool, f.shared, f.alice); m.EndedAt != nil {
		t.Errorf("alice's membership of shared %+v, want it active: it was not named", m)
	}
	if m := memberOf(t, f.pool, f.team, f.bob); m.EndedAt != nil {
		t.Errorf("bob's membership of team %+v, want it active", m)
	}
	if m := memberOf(t, f.pool, f.team, f.carol); m.EndedAt == nil || !m.EndedAt.Equal(*carolBefore.EndedAt) || m.UpdatedBy != carolBefore.UpdatedBy ||
		!m.UpdatedAt.Equal(carolBefore.UpdatedAt) {
		t.Errorf("carol's ended membership %+v, want it as it was, %+v", m, carolBefore)
	}
	if o := ownershipOf(t, f.pool, f.shared); o.Since != nil {
		t.Errorf("shared %+v, want it owned", o)
	}
}

// Alice's ownerless notebooks of acme return to her: her memberships
// active again as their admin, by her, and the notebooks owned, keeping
// their last update; not another workspace's, nor another former owner's.
func TestLockOwnerlessOfAndReturnNotebooks(t *testing.T) {
	f := newHoldingsFixture(t)
	orphan := func(id, owner uuid.UUID) {
		t.Helper()
		if err := inTx(t, f.pool, func(ctx context.Context) error {
			if err := f.s.EndMembershipsOf(ctx, owner, []uuid.UUID{id}, owner, now()); err != nil {
				return err
			}
			return f.s.SetOwnerless(ctx, []uuid.UUID{id}, owner, now())
		}); err != nil {
			t.Fatal(err)
		}
	}
	orphan(f.team, f.alice)
	orphan(f.solo, f.alice)
	orphan(f.lab, f.alice)
	orphan(f.shared, f.bob)

	var got []domain.Notebook
	err := inTx(t, f.pool, func(ctx context.Context) error {
		var err error
		if got, err = f.s.LockOwnerlessOf(ctx, f.acme, f.alice); err != nil {
			return err
		}
		return f.s.ReturnNotebooks(ctx, []uuid.UUID{f.solo, f.team}, f.alice, f.alice, later())
	})

	if ids := []uuid.UUID{f.solo, f.team}; err != nil || len(got) != 2 || got[0].ID != ids[0] || got[1].ID != ids[1] {
		t.Fatalf("LockOwnerlessOf(acme, alice) = %+v, %v; want solo and team", got, err)
	}
	for _, id := range []uuid.UUID{f.solo, f.team} {
		if m := memberOf(t, f.pool, id, f.alice); m.Role != "admin" || m.EndedAt != nil || m.UpdatedBy != f.alice || !m.UpdatedAt.Equal(later()) {
			t.Errorf("alice's membership %+v, want an active admin again, by her at %v", m, later())
		}
		if o := ownershipOf(t, f.pool, id); o.Since != nil || o.FormerOwner != nil || !o.UpdatedAt.Equal(now()) {
			t.Errorf("the notebook %+v, want it owned, its last update at %v", o, now())
		}
	}
	for _, id := range []uuid.UUID{f.lab, f.shared} {
		if o := ownershipOf(t, f.pool, id); o.Since == nil {
			t.Errorf("%s %+v, want it still ownerless", id, o)
		}
	}

	// A notebook without her ended membership is no return: she is an
	// active reader of shared, bob's.
	if err := inTx(t, f.pool, func(ctx context.Context) error {
		return f.s.ReturnNotebooks(ctx, []uuid.UUID{f.lab, f.shared}, f.alice, f.alice, later())
	}); err == nil {
		t.Error("ReturnNotebooks(lab, shared) to alice = nil error, want the missing membership")
	}
	if o := ownershipOf(t, f.pool, f.lab); o.Since == nil {
		t.Errorf("lab %+v after the failed return, want it rolled back", o)
	}
}

// An audit event keeps its values, the actor its creator and last updater.
func TestAddAuditEvent(t *testing.T) {
	f := newHoldingsFixture(t)
	e := domain.AuditEvent{ID: uuid.NewV7(), WorkspaceID: f.acme, NotebookID: f.team, NotebookName: "Team", Action: domain.AuditTakenOver,
		FormerOwnerID: f.alice, ActorID: f.bob, At: later()}

	if err := f.s.AddAuditEvent(context.Background(), e); err != nil {
		t.Fatal(err)
	}

	var got domain.AuditEvent
	var action string
	var updatedBy uuid.UUID
	var updatedAt time.Time
	var deletedAt *time.Time
	if err := f.pool.QueryRow(context.Background(), "SELECT id, workspace_id, notebook_id, notebook_name, action, former_owner_id, "+
		"created_by_id, created_at, updated_by_id, updated_at, deleted_at FROM notebook_audit_events").Scan(&got.ID, &got.WorkspaceID, &got.NotebookID,
		&got.NotebookName, &action, &got.FormerOwnerID, &got.ActorID, &got.At, &updatedBy, &updatedAt, &deletedAt); err != nil {
		t.Fatal(err)
	}
	got.Action = domain.AuditAction(action)
	if got.At = got.At.UTC(); got != e || updatedBy != f.bob || !updatedAt.Equal(later()) || deletedAt != nil {
		t.Errorf("the event %+v, last updated by %s at %v, deleted %v; want %+v, by bob at %v", got, updatedBy, updatedAt, deletedAt, e, later())
	}
}
