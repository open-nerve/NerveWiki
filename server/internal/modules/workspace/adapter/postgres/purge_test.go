package postgresadapter_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// purgeFixture is a workspace deleted before the cutoff, one deleted at
// it and one after, each with a member and a pending invitation deleted
// with it; and a live workspace with an invitation withdrawn before the
// cutoff and a pending one.
type purgeFixture struct {
	old, edge, recent, live uuid.UUID
	withdrawn, pending      uuid.UUID
}

// cutoff is the time the purge deletes before.
func cutoff() time.Time { return now().Add(24 * time.Hour) }

func newPurgeFixture(t *testing.T, s *postgresadapter.Store, pool *pgxpool.Pool) purgeFixture {
	t.Helper()
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	deleted := func(slug string, at time.Time) uuid.UUID {
		w := newWorkspace(t, s, slug, slug, alice)
		addMember(t, s, w.ID, bob, shared.WorkspaceMember, alice)
		invite(t, s, w.ID, "dana@corp.com", now(), alice)
		for _, table := range []string{"workspace_invitations", "workspace_members"} {
			exec(t, pool, "UPDATE "+table+" SET deleted_at = $2 WHERE workspace_id = $1", w.ID, at)
		}
		exec(t, pool, "UPDATE workspaces SET deleted_at = $2 WHERE id = $1", w.ID, at)
		return w.ID
	}
	f := purgeFixture{
		old:    deleted("old", cutoff().Add(-time.Microsecond)),
		edge:   deleted("edge", cutoff()),
		recent: deleted("recent", cutoff().Add(time.Hour)),
	}
	live := newWorkspace(t, s, "live", "Live", alice)
	f.live = live.ID
	f.withdrawn = invite(t, s, live.ID, "erin@corp.com", now(), alice).ID
	exec(t, pool, "UPDATE workspace_invitations SET deleted_at = $2 WHERE id = $1", f.withdrawn, cutoff().Add(-time.Hour))
	f.pending = invite(t, s, live.ID, "dana@corp.com", now(), alice).ID
	return f
}

// countWhere counts the rows of table that match where.
func countWhere(t *testing.T, pool *pgxpool.Pool, table, where string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE "+where, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// The purge deletes the rows deleted before the cutoff, children first,
// and nothing else: not those deleted at it or after, nor live ones.
func TestPurge(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	f := newPurgeFixture(t, s, pool)

	for _, step := range []struct {
		name  string
		purge func(context.Context, time.Time, int) (int, error)
		want  int
	}{
		{"invitations", s.PurgeInvitations, 2}, // old's, and the one withdrawn from live
		{"members", s.PurgeMembers, 2},         // old's admin and member
		{"workspaces", s.PurgeWorkspaces, 1},   // old
	} {
		if n, err := step.purge(ctx, cutoff(), 100); err != nil || n != step.want {
			t.Errorf("purge %s = %d, %v; want %d", step.name, n, err, step.want)
		}
	}

	if n := countWhere(t, pool, "workspaces", "id = ANY($1)", []uuid.UUID{f.edge, f.recent, f.live}); n != 3 ||
		countWhere(t, pool, "workspaces", "true") != 3 {
		t.Errorf("%d workspaces of edge, recent and live are left of %d; want those three alone", n, countWhere(t, pool, "workspaces", "true"))
	}
	if n := countWhere(t, pool, "workspace_members", "workspace_id = ANY($1)", []uuid.UUID{f.edge, f.recent, f.live}); n != 5 ||
		countWhere(t, pool, "workspace_members", "true") != 5 {
		t.Errorf("%d members of edge, recent and live are left; want their 5 alone", n)
	}
	if n := countWhere(t, pool, "workspace_invitations", "id = $1 OR workspace_id = ANY($2)", f.pending, []uuid.UUID{f.edge, f.recent}); n != 3 ||
		countWhere(t, pool, "workspace_invitations", "true") != 3 {
		t.Errorf("%d invitations of edge, recent and live's pending one are left; want those three alone", n)
	}
}

// Each statement deletes a batch at most: the job calls it again until a
// batch comes back short.
func TestPurgeTakesBatches(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	newPurgeFixture(t, s, pool)

	var got []int
	for range 3 {
		n, err := s.PurgeInvitations(ctx, cutoff(), 1)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, n)
	}
	if got[0] != 1 || got[1] != 1 || got[2] != 0 {
		t.Errorf("batches of one = %v, want 1, 1, then 0", got)
	}
}

// A row another transaction holds is skipped, not waited for: the purge
// never joins the lock order, and the next run takes the row.
func TestPurgeSkipsALockedRow(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	f := newPurgeFixture(t, s, pool)
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, "SELECT 1 FROM workspace_invitations WHERE id = $1 FOR UPDATE", f.withdrawn); err != nil {
		t.Fatal(err)
	}

	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	n, err := s.PurgeInvitations(bounded, cutoff(), 100)
	if err != nil || n != 1 || countWhere(t, pool, "workspace_invitations", "id = $1", f.withdrawn) != 1 {
		t.Errorf("purge beside a held row = %d, %v; want old's purged, the held one left, no wait", n, err)
	}
}
