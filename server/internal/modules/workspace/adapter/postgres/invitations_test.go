package postgresadapter_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func invite(t *testing.T, s *postgresadapter.Store, workspaceID uuid.UUID, email string, at time.Time, by uuid.UUID) domain.Invitation {
	t.Helper()
	inv := domain.Invitation{ID: uuid.NewV7(), WorkspaceID: workspaceID, Email: email, Role: shared.WorkspaceMember, CreatedAt: at}
	if err := s.CreateInvitation(context.Background(), inv, by); err != nil {
		t.Fatal(err)
	}
	return inv
}

// invitationRow is what an invitation's row holds beside what the domain
// reads.
type invitationRow struct {
	CreatedBy, UpdatedBy  uuid.UUID
	UpdatedAt             time.Time
	AcceptedAt, DeletedAt *time.Time
}

func readInvitation(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) invitationRow {
	t.Helper()
	var r invitationRow
	err := pool.QueryRow(context.Background(),
		"SELECT created_by_id, updated_by_id, updated_at, accepted_at, deleted_at FROM workspace_invitations WHERE id = $1", id).
		Scan(&r.CreatedBy, &r.UpdatedBy, &r.UpdatedAt, &r.AcceptedAt, &r.DeletedAt)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func at(p *time.Time, want time.Time) bool { return p != nil && p.Equal(want) }

func TestCreateInvitationWritesTheAuditColumns(t *testing.T) {
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)

	inv := invite(t, s, acme.ID, "dana@corp.com", now(), alice)

	if got, err := s.FindPendingInvitation(context.Background(), inv.ID); err != nil || got != inv {
		t.Errorf("FindPendingInvitation() = %+v, %v; want %+v", got, err, inv)
	}
	if r := readInvitation(t, pool, inv.ID); r.CreatedBy != alice || r.UpdatedBy != alice || !r.UpdatedAt.Equal(now()) ||
		r.AcceptedAt != nil || r.DeletedAt != nil {
		t.Errorf("the row: %+v; want created and updated by alice at %v, pending", r, now())
	}
}

// A workspace has at most one pending invitation per address; one that is
// no longer pending frees the address, and another workspace's does not
// hold it.
func TestAPendingInvitationPerAddress(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	other := newWorkspace(t, s, "other", "Other", alice)
	first := invite(t, s, acme.ID, "dana@corp.com", now(), alice)

	again := domain.Invitation{ID: uuid.NewV7(), WorkspaceID: acme.ID, Email: "dana@corp.com", Role: shared.WorkspaceGuest, CreatedAt: later()}
	if err := s.CreateInvitation(ctx, again, alice); !errors.Is(err, domain.ErrAlreadyInvited) {
		t.Errorf("CreateInvitation(an address with a pending invitation) = %v, want ErrAlreadyInvited", err)
	}
	invite(t, s, other.ID, "dana@corp.com", now(), alice)

	if err := s.DeleteInvitation(ctx, first.ID, alice, later()); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateInvitation(ctx, again, alice); err != nil {
		t.Errorf("CreateInvitation() after the deletion = %v", err)
	}
	if err := s.AcceptInvitation(ctx, again.ID, alice, later()); err != nil {
		t.Fatal(err)
	}
	invite(t, s, acme.ID, "dana@corp.com", later(), alice)
}

// The pending invitations of the workspace, newest first, then by id.
func TestListPendingInvitations(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	other := newWorkspace(t, s, "other", "Other", alice)
	// Erin's was written after frank's, at the same time: the id decides.
	dana := invite(t, s, acme.ID, "dana@corp.com", now(), alice)
	frank := invite(t, s, acme.ID, "frank@corp.com", later(), alice)
	erin := invite(t, s, acme.ID, "erin@corp.com", later(), alice)
	deleted := invite(t, s, acme.ID, "gus@corp.com", later(), alice)
	accepted := invite(t, s, acme.ID, "hal@corp.com", later(), alice)
	invite(t, s, other.ID, "ivy@corp.com", later(), alice)
	if err := s.DeleteInvitation(ctx, deleted.ID, alice, later()); err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptInvitation(ctx, accepted.ID, alice, later()); err != nil {
		t.Fatal(err)
	}

	got, err := s.ListPendingInvitations(ctx, acme.ID)

	if want := []domain.Invitation{erin, frank, dana}; err != nil || !slices.Equal(got, want) {
		t.Errorf("ListPendingInvitations() = %+v, %v; want %+v", got, err, want)
	}
}

// Only a pending invitation of a workspace not deleted is found.
func TestFindPendingInvitation(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	gone := newWorkspace(t, s, "gone", "Gone", alice)
	deleted := invite(t, s, acme.ID, "dana@corp.com", now(), alice)
	accepted := invite(t, s, acme.ID, "erin@corp.com", now(), alice)
	// A deleted workspace's invitation that was not deleted with it.
	orphan := invite(t, s, gone.ID, "dana@corp.com", now(), alice)
	if err := s.DeleteInvitation(ctx, deleted.ID, alice, later()); err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptInvitation(ctx, accepted.ID, alice, later()); err != nil {
		t.Fatal(err)
	}
	exec(t, pool, "UPDATE workspaces SET deleted_at = $2 WHERE id = $1", gone.ID, now())

	for name, id := range map[string]uuid.UUID{
		"deleted": deleted.ID, "accepted": accepted.ID, "of a deleted workspace": orphan.ID, "unknown": uuid.NewV7(),
	} {
		if got, err := s.FindPendingInvitation(ctx, id); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("FindPendingInvitation(%s) = %+v, %v; want ErrNotFound", name, got, err)
		}
	}
}

// The lock waits for a transaction that holds the invitation, and then sees
// what it committed: an acceptance leaves no pending row.
func TestLockPendingInvitationWaitsForTheRowAndSeesAnAcceptance(t *testing.T) {
	for _, commit := range []bool{true, false} {
		t.Run(map[bool]string{true: "acceptance committed", false: "acceptance rolled back"}[commit], func(t *testing.T) {
			ctx := context.Background()
			s, pool := newStore(t)
			alice := newAccount(t, pool, "alice@corp.com")
			acme := newWorkspace(t, s, "acme", "Acme", alice)
			inv := invite(t, s, acme.ID, "dana@corp.com", now(), alice)
			holder, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = holder.Rollback(ctx) }()
			if _, err := holder.Exec(ctx, "UPDATE workspace_invitations SET accepted_at = $2, deleted_at = $2 WHERE id = $1", inv.ID, now()); err != nil {
				t.Fatal(err)
			}

			type locked struct {
				inv domain.Invitation
				err error
			}
			done := make(chan locked, 1)
			go func() {
				var got locked
				if err := inTx(t, pool, func(ctx context.Context) error {
					got.inv, got.err = s.LockPendingInvitation(ctx, inv.ID)
					return nil
				}); err != nil {
					got.err = err
				}
				done <- got
			}()
			pgtest.WaitForLockWaitsOn(t, pool, "workspace_invitations", 1, 10*time.Second)
			if commit {
				err = holder.Commit(ctx)
			} else {
				err = holder.Rollback(ctx)
			}
			if err != nil {
				t.Fatal(err)
			}

			got := <-done
			switch {
			case commit && !errors.Is(got.err, app.ErrNotFound):
				t.Errorf("lock after the acceptance = %+v, %v; want ErrNotFound", got.inv, got.err)
			case !commit && (got.err != nil || got.inv != inv):
				t.Errorf("lock after the rollback = %+v, %v; want %+v", got.inv, got.err, inv)
			}
		})
	}
}

// lockNowait reports whether another transaction can take lock on the rows
// of table with id at once: false when a lock it conflicts with is held.
func lockNowait(t *testing.T, pool *pgxpool.Pool, table, lock string, id uuid.UUID) bool {
	t.Helper()
	err := inTx(t, pool, func(ctx context.Context) error {
		_, err := postgres.DB(ctx, pool).Exec(ctx, "SELECT 1 FROM "+table+" WHERE id = $1 FOR "+lock+" NOWAIT", id)
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return true
}

// The invitation's lock holds the invitation's row FOR UPDATE, and not its
// workspace's row: the caller holds that already, in the mode it chose.
func TestLockPendingInvitationLocksTheInvitationAlone(t *testing.T) {
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	inv := invite(t, s, acme.ID, "dana@corp.com", now(), alice)

	err := inTx(t, pool, func(ctx context.Context) error {
		if _, err := s.LockPendingInvitation(ctx, inv.ID); err != nil {
			return err
		}
		if lockNowait(t, pool, "workspace_invitations", "KEY SHARE", inv.ID) {
			t.Error("the invitation's row lets a FOR KEY SHARE through, want it held FOR UPDATE")
		}
		if !lockNowait(t, pool, "workspaces", "UPDATE", acme.ID) {
			t.Error("the workspace's row is locked too, want the invitation's alone")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The share lock lets another share lock through, and holds a change of
// the workspace back.
func TestShareWorkspaceLocksForShare(t *testing.T) {
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	shares := map[string]func(ctx context.Context) (domain.Workspace, error){
		"by slug": func(ctx context.Context) (domain.Workspace, error) { return s.ShareWorkspaceBySlug(ctx, "acme") },
		"by id":   func(ctx context.Context) (domain.Workspace, error) { return s.ShareWorkspaceByID(ctx, acme.ID) },
	}
	for name, share := range shares {
		err := inTx(t, pool, func(ctx context.Context) error {
			if got, err := share(ctx); err != nil || got != acme {
				t.Errorf("%s: share = %+v, %v; want %+v", name, got, err, acme)
			}
			if !lockNowait(t, pool, "workspaces", "SHARE", acme.ID) {
				t.Errorf("%s: another FOR SHARE waits, want it through", name)
			}
			if lockNowait(t, pool, "workspaces", "NO KEY UPDATE", acme.ID) {
				t.Errorf("%s: a FOR NO KEY UPDATE goes through, want it held back", name)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// The share lock waits for a change of the workspace, and then sees what it
// committed: a deletion leaves no row.
func TestShareWorkspaceWaitsForAChangeAndSeesADeletion(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, "UPDATE workspaces SET deleted_at = $2 WHERE id = $1", acme.ID, now()); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- inTx(t, pool, func(ctx context.Context) error {
			_, err := s.ShareWorkspaceByID(ctx, acme.ID)
			return err
		})
	}()
	pgtest.WaitForLockWaitsOn(t, pool, "workspaces", 1, 10*time.Second)
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if err := <-done; !errors.Is(err, app.ErrNotFound) {
		t.Errorf("share after the deletion = %v, want ErrNotFound", err)
	}
	err = inTx(t, pool, func(ctx context.Context) error {
		_, err := s.ShareWorkspaceBySlug(ctx, "acme")
		return err
	})
	if !errors.Is(err, app.ErrNotFound) {
		t.Errorf("ShareWorkspaceBySlug(deleted) = %v, want ErrNotFound", err)
	}
}

func TestFindWorkspaceByID(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)

	if got, err := s.FindWorkspaceByID(ctx, acme.ID); err != nil || got != acme {
		t.Errorf("FindWorkspaceByID() = %+v, %v; want %+v", got, err, acme)
	}
	exec(t, pool, "UPDATE workspaces SET deleted_at = $2 WHERE id = $1", acme.ID, now())
	for name, id := range map[string]uuid.UUID{"deleted": acme.ID, "unknown": uuid.NewV7()} {
		if got, err := s.FindWorkspaceByID(ctx, id); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("FindWorkspaceByID(%s) = %+v, %v; want ErrNotFound", name, got, err)
		}
	}
}

// A deletion and an acceptance write who and when; an acceptance is a
// deletion at the same time.
func TestDeleteAndAcceptInvitationWriteTheAuditColumns(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	dana := newAccount(t, pool, "dana@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	deleted := invite(t, s, acme.ID, "erin@corp.com", now(), alice)
	accepted := invite(t, s, acme.ID, "dana@corp.com", now(), alice)

	if err := s.DeleteInvitation(ctx, deleted.ID, alice, later()); err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptInvitation(ctx, accepted.ID, dana, later()); err != nil {
		t.Fatal(err)
	}

	if r := readInvitation(t, pool, deleted.ID); !at(r.DeletedAt, later()) || r.AcceptedAt != nil || r.UpdatedBy != alice ||
		!r.UpdatedAt.Equal(later()) {
		t.Errorf("the deleted one: %+v; want deleted at %v by alice, not accepted", r, later())
	}
	if r := readInvitation(t, pool, accepted.ID); !at(r.DeletedAt, later()) || !at(r.AcceptedAt, later()) || r.UpdatedBy != dana ||
		!r.UpdatedAt.Equal(later()) || r.CreatedBy != alice {
		t.Errorf("the accepted one: %+v; want accepted and deleted at %v by dana", r, later())
	}
}

// A membership's end deletes the pending invitations of the workspaces
// named to the address, and no other.
func TestDeleteInvitationsTo(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	beta := newWorkspace(t, s, "beta", "Beta", alice)
	other := newWorkspace(t, s, "other", "Other", alice)
	inAcme := invite(t, s, acme.ID, "dana@corp.com", now(), alice)
	inBeta := invite(t, s, beta.ID, "dana@corp.com", now(), alice)
	inOther := invite(t, s, other.ID, "dana@corp.com", now(), alice)
	toErin := invite(t, s, acme.ID, "erin@corp.com", now(), alice)
	before := invite(t, s, beta.ID, "frank@corp.com", now(), alice)
	if err := s.DeleteInvitation(ctx, before.ID, alice, now()); err != nil {
		t.Fatal(err)
	}
	exec(t, pool, "UPDATE workspace_invitations SET email = 'dana@corp.com' WHERE id = $1", before.ID)

	if err := s.DeleteInvitationsTo(ctx, []uuid.UUID{acme.ID, beta.ID}, "dana@corp.com", alice, later()); err != nil {
		t.Fatal(err)
	}

	for _, id := range []uuid.UUID{inAcme.ID, inBeta.ID} {
		if r := readInvitation(t, pool, id); !at(r.DeletedAt, later()) || r.AcceptedAt != nil || r.UpdatedBy != alice || !r.UpdatedAt.Equal(later()) {
			t.Errorf("invitation %s: %+v; want deleted at %v by alice", id, r, later())
		}
	}
	for name, id := range map[string]uuid.UUID{"of a workspace not named": inOther.ID, "to another address": toErin.ID} {
		if r := readInvitation(t, pool, id); r.DeletedAt != nil {
			t.Errorf("the invitation %s was deleted: %+v", name, r)
		}
	}
	if r := readInvitation(t, pool, before.ID); !at(r.DeletedAt, now()) {
		t.Errorf("an invitation deleted before changed: %+v", r)
	}
}

// A workspace's deletion deletes its pending invitations, and no other
// workspace's.
func TestDeleteInvitationsOf(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	alice := newAccount(t, pool, "alice@corp.com")
	acme := newWorkspace(t, s, "acme", "Acme", alice)
	other := newWorkspace(t, s, "other", "Other", alice)
	dana := invite(t, s, acme.ID, "dana@corp.com", now(), alice)
	erin := invite(t, s, acme.ID, "erin@corp.com", now(), alice)
	accepted := invite(t, s, acme.ID, "frank@corp.com", now(), alice)
	elsewhere := invite(t, s, other.ID, "dana@corp.com", now(), alice)
	if err := s.AcceptInvitation(ctx, accepted.ID, alice, now()); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteInvitationsOf(ctx, acme.ID, alice, later()); err != nil {
		t.Fatal(err)
	}

	for _, id := range []uuid.UUID{dana.ID, erin.ID} {
		if r := readInvitation(t, pool, id); !at(r.DeletedAt, later()) || r.UpdatedBy != alice || !r.UpdatedAt.Equal(later()) {
			t.Errorf("invitation %s: %+v; want deleted at %v by alice", id, r, later())
		}
	}
	if r := readInvitation(t, pool, accepted.ID); !at(r.DeletedAt, now()) || !at(r.AcceptedAt, now()) {
		t.Errorf("the accepted invitation changed: %+v", r)
	}
	if r := readInvitation(t, pool, elsewhere.ID); r.DeletedAt != nil {
		t.Errorf("another workspace's invitation was deleted: %+v", r)
	}
}
