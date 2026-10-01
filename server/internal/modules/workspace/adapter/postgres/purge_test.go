package postgresadapter_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// purgeFixture is two workspaces deleted before the cutoff, one deleted at
// it and one after, each with its admin, a member and a pending invitation
// deleted with it; and a live workspace with an invitation withdrawn
// before the cutoff and a pending one.
type purgeFixture struct {
	old, older, edge, recent, live uuid.UUID
	withdrawn, pending             uuid.UUID
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
		older:  deleted("older", cutoff().Add(-time.Hour)),
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

// purgeStep is one purger and how many rows of the fixture it deletes:
// those deleted before the cutoff. The workspaces' comes after its
// children's, which it needs gone.
type purgeStep struct {
	table string
	purge func(context.Context, time.Time, int) (int, error)
	rows  int
}

func purgeSteps(s *postgresadapter.Store) []purgeStep {
	return []purgeStep{
		{"workspace_invitations", s.PurgeInvitations, 3}, // old's, older's, and the one withdrawn from live
		{"workspace_members", s.PurgeMembers, 4},         // old's and older's admin and member
		{"workspaces", s.PurgeWorkspaces, 2},             // old and older
	}
}

// The purge deletes the rows deleted before the cutoff, children first,
// and nothing else: not those deleted at it or after, nor live ones.
func TestPurge(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	f := newPurgeFixture(t, s, pool)

	for _, step := range purgeSteps(s) {
		if n, err := step.purge(ctx, cutoff(), 100); err != nil || n != step.rows {
			t.Errorf("purge %s = %d, %v; want %d", step.table, n, err, step.rows)
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

	for _, step := range purgeSteps(s) {
		var batches []int
		for {
			n, err := step.purge(ctx, cutoff(), 1)
			if err != nil {
				t.Fatal(err)
			}
			batches = append(batches, n)
			if n == 0 || len(batches) > step.rows {
				break
			}
		}
		if len(batches) != step.rows+1 || batches[0] != 1 {
			t.Errorf("purge %s in batches of one = %v, want %d batches of one, then 0", step.table, batches, step.rows)
		}
	}
}

// A row another transaction holds is skipped, not waited for: the purge
// never joins the lock order, and the next run takes the row.
func TestPurgeSkipsALockedRow(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	f := newPurgeFixture(t, s, pool)
	held := map[string]string{
		"workspace_invitations": "SELECT id FROM workspace_invitations WHERE id = $1",
		"workspace_members":     "SELECT id FROM workspace_members WHERE workspace_id = $1 LIMIT 1",
		"workspaces":            "SELECT id FROM workspaces WHERE id = $1",
	}
	args := map[string]uuid.UUID{"workspace_invitations": f.withdrawn, "workspace_members": f.old, "workspaces": f.old}

	for _, step := range purgeSteps(s) {
		holder, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var id uuid.UUID
		if err := holder.QueryRow(ctx, held[step.table]+" FOR UPDATE", args[step.table]).Scan(&id); err != nil {
			t.Fatal(err)
		}
		bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
		n, err := step.purge(bounded, cutoff(), 100)
		cancel()
		if err != nil || n != step.rows-1 || countWhere(t, pool, step.table, "id = $1", id) != 1 {
			t.Errorf("purge %s beside a held row = %d, %v; want %d, the held one left, no wait", step.table, n, err, step.rows-1)
		}
		if err := holder.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		if n, err := step.purge(ctx, cutoff(), 100); err != nil || n != 1 {
			t.Errorf("purge %s once the row is free = %d, %v; want it", step.table, n, err)
		}
	}
}

// A workspace whose children a purger skipped stays until a later run
// takes them: deleting it would cascade to the held row and wait for it.
func TestPurgeWorkspacesWaitsForTheirChildren(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	f := newPurgeFixture(t, s, pool)
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	// Old's invitation, and one of older's members.
	for held, workspace := range map[string]uuid.UUID{
		"SELECT 1 FROM workspace_invitations WHERE workspace_id = $1 FOR UPDATE":     f.old,
		"SELECT 1 FROM workspace_members WHERE workspace_id = $1 LIMIT 1 FOR UPDATE": f.older,
	} {
		if _, err := holder.Exec(ctx, held, workspace); err != nil {
			t.Fatal(err)
		}
	}

	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var purged []int
	for _, step := range purgeSteps(s) {
		n, err := step.purge(bounded, cutoff(), 100)
		if err != nil {
			t.Fatalf("purge %s beside the held rows: %v", step.table, err)
		}
		purged = append(purged, n)
	}
	if !slices.Equal(purged, []int{2, 3, 0}) {
		t.Errorf("purged %v, want every invitation and member but the held ones, and neither workspace", purged)
	}

	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	purged = purged[:0]
	for _, step := range purgeSteps(s) {
		n, err := step.purge(ctx, cutoff(), 100)
		if err != nil {
			t.Fatal(err)
		}
		purged = append(purged, n)
	}
	if !slices.Equal(purged, []int{1, 1, 2}) {
		t.Errorf("the next run purged %v, want the rows held before and both workspaces", purged)
	}
}
