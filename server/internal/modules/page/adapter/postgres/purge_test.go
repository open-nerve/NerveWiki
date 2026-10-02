package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"slices"
	"testing"
	"time"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// cutoff is the time the purge deletes before.
func cutoff() time.Time { return now().Add(24 * time.Hour) }

// purgeFixture is eng's pages in groups, each with a changeset of its own
// that holds an item and a version of each of its pages, deleted when they
// are: the tree a → b → c deleted before the cutoff, the page g deleted
// before it too, d deleted at it, e after it, and f live.
type purgeFixture struct {
	fixture
	a, b, c, g uuid.UUID
	// older is g's changeset.
	older uuid.UUID
}

// followers are the tables of the rows that follow a node.
func followers() []string { return []string{"page_contents", "page_revisions", "changeset_items"} }

func newPurgeFixture(t *testing.T) purgeFixture {
	t.Helper()
	ctx := context.Background()
	f := purgeFixture{fixture: newFixture(t)}
	group := func(deleted bool, at time.Duration, nodes ...domain.Node) uuid.UUID {
		cs := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: "edit", Client: domain.ClientWeb, By: f.alice, At: now()}
		if err := f.s.CreateChangeset(ctx, cs); err != nil {
			t.Fatal(err)
		}
		for _, n := range nodes {
			state := n.State()
			if err := f.s.RecordItem(ctx, app.Item{ID: uuid.NewV7(), ChangesetID: cs.ID, Change: domain.Change{NodeID: n.ID, After: &state}, At: now()}); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(nil)
			if err := f.s.RecordRevision(ctx, app.Revision{ID: uuid.NewV7(), ChangesetID: cs.ID, NodeID: n.ID, Revision: 1, Hash: sum[:], At: now()}); err != nil {
				t.Fatal(err)
			}
		}
		if !deleted {
			return cs.ID
		}
		for _, n := range nodes {
			f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", n.ID, cutoff().Add(at))
			for _, table := range followers() {
				f.exec(t, "UPDATE "+table+" SET deleted_at = $2 WHERE node_id = $1", n.ID, cutoff().Add(at))
			}
		}
		f.exec(t, "UPDATE changesets SET deleted_at = $2 WHERE id = $1", cs.ID, cutoff().Add(at))
		return cs.ID
	}
	a := f.page(t, f.eng, nil, "A", 0)
	b := f.page(t, f.eng, &a.ID, "B", 0)
	c := f.page(t, f.eng, &b.ID, "C", 0)
	g := f.page(t, f.eng, nil, "G", 1)
	f.a, f.b, f.c, f.g = a.ID, b.ID, c.ID, g.ID
	group(true, -time.Microsecond, a, b, c)
	f.older = group(true, -time.Hour, g)
	group(true, 0, f.page(t, f.eng, nil, "D", 2))
	group(true, time.Hour, f.page(t, f.eng, nil, "E", 3))
	group(false, 0, f.page(t, f.eng, nil, "F", 4))
	return f
}

// purgeStep is one purger and how many rows of the fixture it deletes:
// those of a, b, c and g, and their two changesets.
type purgeStep struct {
	table string
	purge func(context.Context, time.Time, int) (int, error)
	rows  int
}

func purgeSteps(s *postgresadapter.Store) []purgeStep {
	return []purgeStep{
		{"changeset_items", s.PurgeChangesetItems, 4},
		{"page_revisions", s.PurgePageRevisions, 4},
		{"page_contents", s.PurgePageContents, 4},
		{"nodes", s.PurgeNodes, 4},
		{"changesets", s.PurgeChangesets, 2},
	}
}

// run runs the purgers once each, in their order, and returns how many
// rows each deleted. A purger that waits for a lock fails within 5s.
func run(t *testing.T, s *postgresadapter.Store) []int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var purged []int
	for _, step := range purgeSteps(s) {
		n, err := step.purge(ctx, cutoff(), 100)
		if err != nil {
			t.Fatalf("purge %s: %v", step.table, err)
		}
		purged = append(purged, n)
	}
	return purged
}

// The purge deletes the rows deleted before the cutoff, what follows a
// node first, and nothing else: not those deleted at it or after, nor
// live ones. One run clears a tree of three levels.
func TestPurge(t *testing.T) {
	f := newPurgeFixture(t)
	if purged, want := run(t, f.s), []int{4, 4, 4, 4, 2}; !slices.Equal(purged, want) {
		t.Errorf("purged %v, want %v", purged, want)
	}
	for _, table := range append([]string{"nodes"}, followers()...) {
		if got := f.count(t, "SELECT count(*) FROM "+table); got != 3 {
			t.Errorf("%s left = %d, want d's, e's and f's", table, got)
		}
	}
	if got := f.count(t, "SELECT count(*) FROM changesets"); got != 3 {
		t.Errorf("changesets left = %d, want d's, e's and f's", got)
	}
}

// Each purger deletes a batch at most: the job calls it again until a batch
// comes back short.
func TestPurgeTakesBatches(t *testing.T) {
	ctx := context.Background()
	f := newPurgeFixture(t)
	for _, step := range purgeSteps(f.s) {
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

// hold begins a transaction that locks the rows sql selects, until the
// test rolls it back.
func (f purgeFixture) hold(t *testing.T, sqls ...string) func() {
	t.Helper()
	ctx := context.Background()
	holder, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	for _, sql := range sqls {
		if _, err := holder.Exec(ctx, sql+" FOR UPDATE"); err != nil {
			t.Fatal(err)
		}
	}
	return func() {
		if err := holder.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

// A held node is skipped, not waited for, and its ancestors stay with it:
// they are not leaves. The other leaves go, and a later run takes the rest.
func TestPurgeSkipsAHeldNodeAndKeepsItsAncestors(t *testing.T) {
	f := newPurgeFixture(t)
	release := f.hold(t, "SELECT 1 FROM nodes WHERE id = '"+f.c.String()+"'")
	if purged, want := run(t, f.s), []int{4, 4, 4, 1, 2}; !slices.Equal(purged, want) {
		t.Errorf("purged %v beside the held c, want %v: g's node alone", purged, want)
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE id = ANY($1)", []uuid.UUID{f.a, f.b, f.c}); got != 3 {
		t.Errorf("%d of a, b and c are left, want all three", got)
	}
	release()
	if purged, want := run(t, f.s), []int{0, 0, 0, 3, 0}; !slices.Equal(purged, want) {
		t.Errorf("the next run purged %v, want %v: the tree", purged, want)
	}
}

// A held row is skipped, not waited for, and so is what needs it gone: a
// node while a row that follows it is held, a changeset while one of its
// items or versions is; deleting either would cascade to the held row and
// wait for it. A later run takes the rest.
func TestPurgeKeepsWhatAHeldRowNeeds(t *testing.T) {
	for _, tt := range []struct {
		name  string
		held  string                         // the table and the key column of the row held,
		of    func(f purgeFixture) uuid.UUID // whose key this is
		first []int                          // items, versions, contents, nodes, changesets
		next  []int
	}{
		// Not g, whose content is held; the tree, and both changesets.
		{"g's content", "page_contents WHERE node_id", func(f purgeFixture) uuid.UUID { return f.g },
			[]int{4, 4, 3, 3, 2}, []int{0, 0, 1, 1, 0}},
		// Not g, nor its changeset, which its version is in.
		{"g's version", "page_revisions WHERE node_id", func(f purgeFixture) uuid.UUID { return f.g },
			[]int{4, 3, 4, 3, 1}, []int{0, 1, 0, 1, 1}},
		// Not c, nor its ancestors, nor the tree's changeset; g and its own.
		{"c's item", "changeset_items WHERE node_id", func(f purgeFixture) uuid.UUID { return f.c },
			[]int{3, 4, 4, 1, 1}, []int{1, 0, 0, 3, 1}},
		// Everything but g's changeset itself.
		{"g's changeset", "changesets WHERE id", func(f purgeFixture) uuid.UUID { return f.older },
			[]int{4, 4, 4, 4, 1}, []int{0, 0, 0, 0, 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newPurgeFixture(t)
			release := f.hold(t, "SELECT 1 FROM "+tt.held+" = '"+tt.of(f).String()+"'")
			if purged := run(t, f.s); !slices.Equal(purged, tt.first) {
				t.Errorf("purged %v beside %s held, want %v", purged, tt.name, tt.first)
			}
			release()
			if purged := run(t, f.s); !slices.Equal(purged, tt.next) {
				t.Errorf("the next run purged %v, want %v", purged, tt.next)
			}
		})
	}
}

// A subtree deleted in a live notebook goes in one run with the items of
// the changeset that deleted it; the changesets stay with their notebook.
func TestPurgeTakesADeletedSubtree(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	a := f.page(t, f.eng, nil, "A", 0)
	b := f.page(t, f.eng, &a.ID, "B", 0)
	c := f.page(t, f.eng, &b.ID, "C", 0)
	created := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: "edit", Client: domain.ClientWeb, By: f.alice, At: now()}
	deleted := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: "edit", Client: domain.ClientWeb, By: f.alice, At: cutoff().Add(-time.Microsecond)}
	for _, cs := range []app.Changeset{created, deleted} {
		if err := f.s.CreateChangeset(ctx, cs); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []domain.Node{a, b, c} {
		state := n.State()
		if err := f.s.RecordItem(ctx, app.Item{ID: uuid.NewV7(), ChangesetID: created.ID, Change: domain.Change{NodeID: n.ID, After: &state}, At: now()}); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(nil)
		if err := f.s.RecordRevision(ctx, app.Revision{ID: uuid.NewV7(), ChangesetID: created.ID, NodeID: n.ID, Revision: 1, Hash: sum[:], At: now()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.s.DeleteNodes(ctx, []uuid.UUID{a.ID, b.ID, c.ID}, f.alice, deleted.At); err != nil {
		t.Fatal(err)
	}
	for _, n := range []domain.Node{a, b, c} {
		state := n.State()
		if err := f.s.RecordItem(ctx, app.Item{ID: uuid.NewV7(), ChangesetID: deleted.ID, Change: domain.Change{NodeID: n.ID, Before: &state}, At: deleted.At}); err != nil {
			t.Fatal(err)
		}
	}
	if purged, want := run(t, f.s), []int{6, 3, 3, 3, 0}; !slices.Equal(purged, want) {
		t.Errorf("purged %v, want %v: both changesets' items, the tree and what follows it", purged, want)
	}
	if got := f.count(t, "SELECT count(*) FROM changesets"); got != 2 {
		t.Errorf("changesets left = %d, want both", got)
	}
}
