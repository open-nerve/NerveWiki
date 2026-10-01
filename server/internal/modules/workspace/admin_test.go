package workspace_test

import (
	"context"
	"log/slog"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// sqlAccountsByEmail stands in for identity's accounts as the
// administrator's commands use them: the account of an address, its row
// shared in the transaction.
type sqlAccountsByEmail struct{ f fixture }

func (a sqlAccountsByEmail) ShareActiveAccountByEmail(ctx context.Context, email string) (uuid.UUID, error) {
	var id uuid.UUID
	err := postgres.DB(ctx, a.f.pool).QueryRow(ctx, "SELECT id FROM users WHERE email = $1 FOR SHARE", email).Scan(&id)
	return id, err
}

func (f fixture) admin(subscribers ...workspace.MembershipRestoreSubscriber) *workspace.Admin {
	return workspace.NewAdmin(workspace.AdminDeps{Pool: f.pool, Tx: postgres.NewTxManager(f.pool, 5*time.Second), Clock: &tickingClock{},
		Logger: slog.New(slog.DiscardHandler), Accounts: sqlAccountsByEmail{f}, MembershipRestoreSubscribers: subscribers})
}

// reactivate-member restores the ended membership with its role, and its
// subscriber reads it restored in the command's transaction, by the
// account itself; its failure rolls the whole command back (M2/P4 design
// 3.3).
func TestReactivateMemberPassesTheRestoreSubscriber(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "the subscriber follows", true: "the subscriber fails"}[fail], func(t *testing.T) {
			f := newFixture(t)
			ended := testNow().Add(-time.Hour)
			f.exec(t, "UPDATE workspace_members SET ended_at = $2 WHERE id = $1", f.bobMembership, ended)
			r := &restorer{f: f, fail: fail}

			got, err := f.admin(r).ReactivateMember(context.Background(), "acme", "bob@corp.com")

			want := []workspace.MembershipRestore{{WorkspaceID: f.acme, UserID: f.bob, Role: "member", By: f.bob, At: testNow()}}
			if !slices.Equal(r.restored, want) || r.endedAt != nil || r.role != "member" {
				t.Errorf("the subscriber saw %+v, the membership ended at %v as %s; want %+v, active, a member", r.restored, r.endedAt, r.role, want)
			}
			switch at := f.endedAt(t, context.Background(), f.bobMembership); {
			case !fail && (err != nil || !got.EndedAt.Equal(ended) || got.Role != "member" || at != nil):
				t.Errorf("ReactivateMember() = %+v, %v, ended %v; want reactivated, ended at %v before", got, err, at, ended)
			case fail && (err == nil || at == nil || !at.Equal(ended)):
				t.Errorf("ReactivateMember() = %+v, %v, ended %v; want the failure, still ended", got, err, at)
			}
		})
	}
}

// The administrator's creation makes the account of the address the
// workspace's admin, by it.
func TestAdminCreatesAWorkspace(t *testing.T) {
	f := newFixture(t)

	w, err := f.admin().CreateWorkspace(context.Background(), "Beta", "beta", "bob@corp.com")

	if err != nil || w.Slug != "beta" {
		t.Fatalf("CreateWorkspace() = %+v, %v", w, err)
	}
	var role string
	var by uuid.UUID
	if err := f.pool.QueryRow(context.Background(), `SELECT m.role, w.created_by_id FROM workspace_members m JOIN workspaces w ON w.id = m.workspace_id
		WHERE w.id = $1 AND m.user_id = $2`, w.ID, f.bob).Scan(&role, &by); err != nil || role != "admin" || by != f.bob {
		t.Errorf("bob in beta: %s, created by %v (%v); want its admin, its creator", role, by, err)
	}
}
