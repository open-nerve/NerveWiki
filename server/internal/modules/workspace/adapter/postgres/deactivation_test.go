package postgresadapter_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A deactivation's lock (M2/P4 design 3.1) takes the workspaces of the
// account's active memberships, and no other: not those it has left, nor
// the deleted ones, nor those whose member row alone is deleted. It holds
// them FOR NO KEY UPDATE: the foreign keys' FOR KEY SHARE goes through.
func TestLockWorkspacesOf(t *testing.T) {
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	zeta := newWorkspace(t, s, "zeta", "Zeta", alice) // created first: the id order is not the names'
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	left := newWorkspace(t, s, "left", "Left", bob)
	ended := addMember(t, s, left.ID, alice, shared.WorkspaceMember, bob)
	exec(t, pool, "UPDATE workspace_members SET ended_at = $2 WHERE id = $1", ended, now())
	gone := newWorkspace(t, s, "gone", "Gone", alice)
	exec(t, pool, "UPDATE workspaces SET deleted_at = $2 WHERE id = $1", gone.ID, now())
	newWorkspace(t, s, "other", "Other", bob)

	err := inTx(t, pool, func(ctx context.Context) error {
		got, err := s.LockWorkspacesOf(ctx, alice)
		if err != nil {
			return err
		}
		if want := []domain.Workspace{zeta, acme}; !slices.Equal(got, want) {
			t.Errorf("LockWorkspacesOf() = %+v, want %+v, by id", got, want)
		}
		for _, w := range []domain.Workspace{zeta, acme} {
			if lockNowait(t, pool, "workspaces", "SHARE", w.ID) || !lockNowait(t, pool, "workspaces", "KEY SHARE", w.ID) {
				t.Errorf("%s: want FOR NO KEY UPDATE: held against FOR SHARE, not against FOR KEY SHARE", w.Slug)
			}
		}
		if !lockNowait(t, pool, "workspaces", "UPDATE", left.ID) {
			t.Error("the workspace alice left is locked, want it alone")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The workspaces are locked one after another in id order: while the lock
// waits for the second, it holds the first; while it waits for the first,
// it holds nothing yet. Two deactivations thus never wait for each other in
// a ring. A workspace deleted while the lock waited for it is left out.
func TestLockWorkspacesOfLocksInIDOrder(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	first := newWorkspace(t, s, "first", "First", alice)
	second := newWorkspace(t, s, "second", "Second", alice)

	for _, tt := range []struct {
		held, other domain.Workspace
		otherHeld   bool
	}{{second, first, true}, {first, second, false}} {
		holder, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := holder.Exec(ctx, "SELECT 1 FROM workspaces WHERE id = $1 FOR NO KEY UPDATE", tt.held.ID); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			done <- inTx(t, pool, func(ctx context.Context) error {
				_, err := s.LockWorkspacesOf(ctx, alice)
				return err
			})
		}()
		pgtest.WaitForLockWaitsOn(t, pool, "workspaces", 1, 10*time.Second)
		if held := !lockNowait(t, pool, "workspaces", "NO KEY UPDATE", tt.other.ID); held != tt.otherHeld {
			t.Errorf("waiting for %s, %s held: %v; want %v", tt.held.Slug, tt.other.Slug, held, tt.otherHeld)
		}
		if err := holder.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}

	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, "UPDATE workspaces SET deleted_at = $2 WHERE id = $1", second.ID, now()); err != nil {
		t.Fatal(err)
	}
	type locked struct {
		ws  []domain.Workspace
		err error
	}
	done := make(chan locked, 1)
	go func() {
		var got locked
		got.err = inTx(t, pool, func(ctx context.Context) error {
			var err error
			got.ws, err = s.LockWorkspacesOf(ctx, alice)
			return err
		})
		done <- got
	}()
	pgtest.WaitForLockWaitsOn(t, pool, "workspaces", 1, 10*time.Second)
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := <-done; got.err != nil || !slices.Equal(got.ws, []domain.Workspace{first}) {
		t.Errorf("after the deletion = %+v, %v; want %s alone", got.ws, got.err, first.Slug)
	}
}

// Rule two's view counts the active admins and members of each workspace,
// the account included, for the account's active memberships of the
// workspaces asked: not an ended one, nor one not asked for.
func TestListStandings(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	carol := newAccount(t, pool, "carol@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	addMember(t, s, acme.ID, bob, shared.WorkspaceAdmin, alice)
	addMember(t, s, acme.ID, carol, shared.WorkspaceGuest, alice)
	beta := newWorkspace(t, s, "beta", "Beta", bob)
	addMember(t, s, beta.ID, alice, shared.WorkspaceMember, bob)
	ended := addMember(t, s, beta.ID, carol, shared.WorkspaceAdmin, bob)
	exec(t, pool, "UPDATE workspace_members SET ended_at = $2 WHERE id = $1", ended, now())
	solo := newWorkspace(t, s, "solo", "Solo", alice)
	left := newWorkspace(t, s, "left", "Left", bob)
	gone := addMember(t, s, left.ID, alice, shared.WorkspaceAdmin, bob)
	exec(t, pool, "UPDATE workspace_members SET ended_at = $2 WHERE id = $1", gone, now())
	unasked := newWorkspace(t, s, "unasked", "Unasked", alice)

	got, err := s.ListStandings(ctx, alice, []uuid.UUID{acme.ID, beta.ID, solo.ID, left.ID})
	want := []domain.Standing{
		{WorkspaceID: acme.ID, Slug: "acme", Role: shared.WorkspaceAdmin, Admins: 2, Members: 3},
		{WorkspaceID: beta.ID, Slug: "beta", Role: shared.WorkspaceMember, Admins: 1, Members: 2},
		{WorkspaceID: solo.ID, Slug: "solo", Role: shared.WorkspaceAdmin, Admins: 1, Members: 1},
	}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("ListStandings() = %+v, %v; want %+v (not %s)", got, err, want, unasked.Slug)
	}
	if got, err := s.ListStandings(ctx, alice, nil); err != nil || len(got) != 0 {
		t.Errorf("ListStandings(none) = %+v, %v; want none", got, err)
	}
}
