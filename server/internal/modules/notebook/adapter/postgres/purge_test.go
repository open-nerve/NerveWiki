package postgresadapter_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// purgeFixture is two notebooks deleted before the cutoff, one deleted at
// it and one after, each with its admin and a member deleted with it; and
// a live notebook with its admin and a member.
type purgeFixture struct {
	old, older, edge, recent, live uuid.UUID
}

// cutoff is the time the purge deletes before.
func cutoff() time.Time { return now().Add(24 * time.Hour) }

func newPurgeFixture(t *testing.T, s *postgresadapter.Store, pool *pgxpool.Pool) purgeFixture {
	t.Helper()
	alice := newAccount(t, pool, "alice@corp.com")
	bob := newAccount(t, pool, "bob@corp.com")
	acme := newWorkspace(t, pool, "acme", alice)
	notebook := func(name string) uuid.UUID {
		n := newNotebook(t, s, acme, name, shared.AccessNone, alice)
		addMember(t, s, n.ID, bob, shared.NotebookReader, alice)
		return n.ID
	}
	deleted := func(name string, at time.Time) uuid.UUID {
		id := notebook(name)
		exec(t, pool, "UPDATE notebook_members SET deleted_at = $2 WHERE notebook_id = $1", id, at)
		exec(t, pool, "UPDATE notebooks SET deleted_at = $2 WHERE id = $1", id, at)
		return id
	}
	return purgeFixture{
		old:    deleted("old", cutoff().Add(-time.Microsecond)),
		older:  deleted("older", cutoff().Add(-time.Hour)),
		edge:   deleted("edge", cutoff()),
		recent: deleted("recent", cutoff().Add(time.Hour)),
		live:   notebook("live"),
	}
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
// those deleted before the cutoff. The notebooks' comes after their
// members', which it needs gone.
type purgeStep struct {
	table string
	purge func(context.Context, time.Time, int) (int, error)
	rows  int
}

func purgeSteps(s *postgresadapter.Store) []purgeStep {
	return []purgeStep{
		{"notebook_members", s.PurgeMembers, 4}, // old's and older's admin and member
		{"notebooks", s.PurgeNotebooks, 2},      // old and older
	}
}

// The purge deletes the rows deleted before the cutoff, members first,
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

	left := []uuid.UUID{f.edge, f.recent, f.live}
	if n := countWhere(t, pool, "notebooks", "id = ANY($1)", left); n != 3 || countWhere(t, pool, "notebooks", "true") != 3 {
		t.Errorf("%d notebooks of edge, recent and live are left of %d; want those three alone", n, countWhere(t, pool, "notebooks", "true"))
	}
	if n := countWhere(t, pool, "notebook_members", "notebook_id = ANY($1)", left); n != 6 || countWhere(t, pool, "notebook_members", "true") != 6 {
		t.Errorf("%d members of edge, recent and live are left; want their 6 alone", n)
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

// A row another transaction holds is skipped, not waited for; and a
// notebook whose members a purger skipped stays until a later run takes
// them: deleting it would cascade to the held row and wait for it.
func TestPurgeSkipsALockedRowAndWaitsForTheMembers(t *testing.T) {
	ctx := context.Background()
	s, pool := newStore(t)
	f := newPurgeFixture(t, s, pool)
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, "SELECT 1 FROM notebook_members WHERE notebook_id = $1 LIMIT 1 FOR UPDATE", f.old); err != nil {
		t.Fatal(err)
	}

	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var purged []int
	for _, step := range purgeSteps(s) {
		n, err := step.purge(bounded, cutoff(), 100)
		if err != nil {
			t.Fatalf("purge %s beside the held row: %v", step.table, err)
		}
		purged = append(purged, n)
	}
	if !slices.Equal(purged, []int{3, 1}) || countWhere(t, pool, "notebooks", "id = $1", f.old) != 1 {
		t.Errorf("purged %v, want every member but the held one, and older alone", purged)
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
	if !slices.Equal(purged, []int{1, 1}) {
		t.Errorf("the next run purged %v, want the member held before and old", purged)
	}
}
