package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// jobs are the background jobs' writes over the fixture, the trash read
// at at, of maxItems and maxHistory (0: the domain's).
func (f *fixture) jobs(at time.Time, maxItems, maxHistory int) *app.JobWrites {
	return app.NewJobWrites(f.trashReads(at), f.log, f.writer(), f.parser(), f.store, maxItems, maxHistory)
}

// asJob is a context with alice as the job acting for her.
func (f *fixture) asJob() context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: f.alice, JobID: uuid.NewV7()})
}

// jobSpec is a job's of eng from the web, with its row's changeset.
func (f *fixture) jobSpec() app.JobSpec {
	return app.JobSpec{NotebookID: f.eng, Client: domain.ClientWeb, Changeset: uuid.NewV7()}
}

// runJob runs the plan's steps a batch of size at a time, as the job's
// worker cuts them, each grown to the end of its segment (M9/P3 design
// 3.23), and answers each batch's, up to the first error.
func (f *fixture) runJob(w *app.JobWrites, spec app.JobSpec, plan app.JobPlan, size int) ([]app.BatchDone, error) {
	var out []app.BatchDone
	steps := plan.Steps()
	for from := 0; from < len(steps); {
		to := min(from+size, len(steps))
		for to < len(steps) && steps[to-1].Joined {
			to++
		}
		done, err := w.RunBatch(f.asJob(), spec, plan, from, to)
		if err != nil {
			return out, err
		}
		out, from = append(out, done), to
	}
	return out, nil
}

// restoredMeanwhile brings the trashed nodes ids back with their
// contents, as another's restore would, without recording a call.
func (f *fixture) restoredMeanwhile(ids ...uuid.UUID) {
	for _, id := range ids {
		f.store.nodes[id] = f.store.trash[id].Node
		delete(f.store.trash, id)
		if c, ok := f.store.trashed[id]; ok {
			f.store.contents[id] = c
			delete(f.store.trashed, id)
		}
	}
}

// boxFixture is a trash item deleted an hour ago, Box at the root: its
// pages A (A1 and A2 under it) and B, and its attachment c.png, a group in
// pre-order Box, A, A1, A2, B, c.png.
type boxFixture struct {
	*fixture
	box, a, a1, a2, b, c domain.Node
	at                   time.Time
}

func newBoxFixture() boxFixture {
	f := boxFixture{fixture: newFixture(), at: now().Add(-time.Hour)}
	f.grant(domain.ActionRestore, domain.ActionRevert)
	f.names[f.alice] = "Alice"
	f.box = f.page("Box", nil, 0)
	f.a = f.page("A", &f.box.ID, 0)
	f.a1 = f.page("A1", &f.a.ID, 0)
	f.a2 = f.page("A2", &f.a.ID, 1)
	f.b = f.page("B", &f.box.ID, 1)
	f.c = f.asset("c.png", &f.box.ID, 2)
	for _, n := range []domain.Node{f.box, f.a, f.a1, f.a2, f.b} {
		f.content(n.ID, "# "+n.Name, 2)
	}
	f.trashed(f.box.ID, f.at, f.alice)
	return f
}

// A background restore plans its item's group in pre-order, a node a
// step, and restores it a batch at a time, each a unit of the job's
// changeset, a trash_restore its first batch makes with the row's id and
// its later ones merge into: the first puts the root where it was, and
// makes the nodes under what it restored that it left in the trash items
// of their own, with their counts; the next restores them where they were.
func TestARestoreJobRestoresInBatches(t *testing.T) {
	f := newBoxFixture()
	w, spec := f.jobs(now(), 0, 0), f.jobSpec()
	plan, err := w.PlanRestore(f.asJob(), spec, f.box.ID)
	if err != nil {
		t.Fatal(err)
	}
	steps := plan.Steps()
	if len(steps) != 6 || plan.Total() != 6 || steps[0] != (domain.JobStep{Nodes: 1, Bytes: 5}) || steps[5] != (domain.JobStep{Nodes: 1}) {
		t.Fatalf("PlanRestore() = %+v, total %d; want six steps of a node each, a page's of its bytes", steps, plan.Total())
	}
	f.rec.calls = nil
	first, err := w.RunBatch(f.asJob(), spec, plan, 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"TrashHistory", "TrashContents", "Take 5", "Facts", "KeepFacts 5", "Links", "Take 4", "Facts", "KeepFacts 4", "Links",
		"Take 3", "Facts", "KeepFacts 3", "Links", "WorkspaceOf", "ShareWorkspace in tx", "LockNotebook in tx",
		"Authorize node.restore in tx", "LockChangeset in tx", "TrashNodesAt 3 in tx", "TrashLines 1 in tx", "Children in tx",
		"CreateChangeset in tx", "RestoreNodes in tx", "RecordItems 3 in tx", "MarkTrashRoots 3 in tx", "QueueSearch in tx", "Release 5", "Release 4",
		"Release 3"}
	if !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v\nwant %v", f.rec.calls, want)
	}
	if first.Pages != 3 || first.Assets != 0 || first.Nodes != 3 || first.Placed == nil || first.Placed.Reason != domain.PlacedOriginal ||
		first.Placed.ParentID != nil || first.Renamed != 0 || first.Skipped != 0 {
		t.Errorf("the first batch = %+v; want Box, A and A1 back, Box at the root as it was", first)
	}
	if cs := f.store.changesets; len(cs) != 1 || cs[0].ID != spec.Changeset || cs[0].Kind != domain.ChangesetTrashRestore ||
		cs[0].By != f.alice {
		t.Errorf("changesets = %+v, want the job's trash_restore, of its row's id", cs)
	}
	for _, c := range []struct {
		n      domain.Node
		counts [3]int
	}{{f.a2, [3]int{1, 0, 1}}, {f.b, [3]int{1, 0, 1}}, {f.c, [3]int{0, 1, 0}}} {
		if n := f.store.trash[c.n.ID]; !n.TrashRoot || f.store.counts[c.n.ID] != c.counts {
			t.Errorf("%s in the trash = %+v, counts %v; want an item of its own of %v", c.n.Name, n, f.store.counts[c.n.ID], c.counts)
		}
	}
	second, err := w.RunBatch(f.asJob(), spec, plan, 3, 6)
	if err != nil {
		t.Fatal(err)
	}
	if second.Pages != 2 || second.Assets != 1 || second.Placed != nil {
		t.Errorf("the second batch = %+v; want A2, B and c.png back", second)
	}
	if len(f.store.trash) != 0 || len(f.store.changesets) != 1 || f.store.touched[spec.Changeset].IsZero() {
		t.Errorf("trash %v, changesets %+v; want all back, the second batch merged into the first's changeset", f.store.trash,
			f.store.changesets)
	}
	for _, n := range []domain.Node{f.box, f.a, f.a1, f.a2, f.b, f.c} {
		got := f.store.nodes[n.ID]
		if !sameParent(got.ParentID, n.ParentID) || got.Name != n.Name || got.SortOrder != n.SortOrder {
			t.Errorf("%s back = %+v, want where it was", n.Name, got)
		}
		if it := f.store.items[n.ID]; it.ChangesetID != spec.Changeset || it.Change.Restored == nil {
			t.Errorf("%s's item = %+v, want the job's restore", n.Name, it)
		}
	}
	if f.store.items[f.a2.ID].At.Equal(f.store.items[f.box.ID].At) {
		t.Errorf("the items of two batches at one time %s", f.store.items[f.box.ID].At)
	}
	// A background job's nodes are queued for the search in bulk, the
	// search building until its job has built them (M8/P5A review C-I1).
	if q := f.store.queued; len(q) != 2 || !q[0].bulk || !q[1].bulk || len(q[0].ids)+len(q[1].ids) != 6 {
		t.Errorf("queued %+v; want each batch's nodes in bulk", q)
	}
}

// A batch checks its members again under the lock: the time the item can
// be restored in at the unit's, the root of the first; a member someone
// restored (an item a batch before it left) is skipped, one still in the
// trash restored under its parent alive, no deeper than pages go, named
// apart from the siblings alive there; the frontier still in the trash.
func TestARestoreJobsBatchChecksUnderTheLock(t *testing.T) {
	for _, tt := range []struct {
		name string
		// between runs between the first batch, Box, A and A1, and the
		// second, A2, B and c.png.
		between func(f boxFixture)
		before  func(f boxFixture) // before the first
		code    string
		check   func(t *testing.T, f boxFixture, done []app.BatchDone)
	}{
		{name: "an item a batch left, restored meanwhile", between: func(f boxFixture) {
			f.restoredMeanwhile(f.b.ID)
		}, check: func(t *testing.T, f boxFixture, done []app.BatchDone) {
			if done[1].Skipped != 1 || done[1].Pages != 1 || done[1].Assets != 1 {
				t.Errorf("the second batch = %+v; want B skipped", done[1])
			}
		}},
		{name: "a name taken where it goes back", between: func(f boxFixture) {
			f.page("b", &f.box.ID, 9)
		}, check: func(t *testing.T, f boxFixture, done []app.BatchDone) {
			if done[1].Renamed != 1 || f.store.nodes[f.b.ID].Name != "B 2" {
				t.Errorf("the second batch = %+v, B named %q; want B 2", done[1], f.store.nodes[f.b.ID].Name)
			}
		}},
		{name: "the root's name taken", before: func(f boxFixture) {
			f.page("box", nil, 9)
		}, check: func(t *testing.T, f boxFixture, done []app.BatchDone) {
			if done[0].Renamed != 1 || f.store.nodes[f.box.ID].Name != "Box 2" {
				t.Errorf("the first batch = %+v, Box named %q; want Box 2", done[0], f.store.nodes[f.box.ID].Name)
			}
		}},
		{name: "its parent deleted", between: func(f boxFixture) {
			f.trashed(f.a.ID, now(), f.alice)
		}, code: "no parent"},
		{name: "its parent moved deeper", between: func(f boxFixture) {
			var parent *uuid.UUID
			for range domain.MaxDepth - 2 {
				n := f.page("Level", parent, 0)
				parent = &n.ID
			}
			box := f.store.nodes[f.box.ID]
			box.ParentID = parent
			f.store.nodes[f.box.ID] = box
		}, code: "page.too_deep"},
		{name: "the item restored before the first", before: func(f boxFixture) {
			f.restoredMeanwhile(f.box.ID)
		}, code: "page.trash_changed"},
		{name: "the root's counts not the plan's", before: func(f boxFixture) {
			c := f.store.counts[f.box.ID]
			c[0]++
			f.store.counts[f.box.ID] = c
		}, code: "page.trash_changed"},
		{name: "a node the first leaves an item already", before: func(f boxFixture) {
			n := f.store.trash[f.b.ID]
			n.TrashRoot = true
			f.store.trash[f.b.ID], f.store.counts[f.b.ID] = n, [3]int{1, 0, 1}
		}, code: "page.trash_changed", check: func(t *testing.T, f boxFixture, done []app.BatchDone) {
			if len(done) != 0 {
				t.Errorf("batches %+v written, want the first refused", done)
			}
		}},
		{name: "a frontier member no item", between: func(f boxFixture) {
			n := f.store.trash[f.b.ID]
			n.TrashRoot = false
			f.store.trash[f.b.ID] = n
		}, code: "page.trash_changed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newBoxFixture()
			w, spec := f.jobs(now(), 0, 0), f.jobSpec()
			plan, err := w.PlanRestore(f.asJob(), spec, f.box.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tt.before != nil {
				tt.before(f)
			}
			var done []app.BatchDone
			d, err := w.RunBatch(f.asJob(), spec, plan, 0, 3)
			if err == nil {
				done = append(done, d)
				if tt.between != nil {
					tt.between(f)
				}
				d, err = w.RunBatch(f.asJob(), spec, plan, 3, 6)
				done = append(done, d)
			}
			switch {
			case tt.code == "no parent" && !errors.Is(err, app.ErrNoParent):
				t.Fatalf("RunBatch() = %v, want ErrNoParent", err)
			case tt.code != "" && tt.code != "no parent" && codeOf(err) != tt.code:
				t.Fatalf("RunBatch() = %v, want %s", err, tt.code)
			case tt.code == "" && err != nil:
				t.Fatal(err)
			}
			if tt.check != nil {
				tt.check(t, f, done)
			}
		})
	}
}

// A background restore puts its item's root where Restore does (M8/P4
// design 3.4; M8/P4A review C-M1): under its parent alive at its own
// order, as at the root where it was; last at the notebook's root, its
// parent in the trash or its pages too deep under it, numbered when a
// sibling alive there holds its name. Its first batch tells where. A
// member at a time, the later batches put the rest under it, Deep's Leaf
// at the tenth level and its attachment below it (M8/P4A review C-M2).
func TestARestoreJobPlacesItsRootAsRestoreDoes(t *testing.T) {
	for _, tt := range []struct {
		name   string
		setup  func(f trashFixture) uuid.UUID
		parent func(f trashFixture) *uuid.UUID
		reason domain.PlacementReason
		named  string
	}{
		{"its parent alive", func(f trashFixture) uuid.UUID { return f.q.ID },
			func(f trashFixture) *uuid.UUID { return &f.p.ID }, domain.PlacedOriginal, "Q"},
		{"at the root, as it was", func(f trashFixture) uuid.UUID { return f.photo.ID },
			func(trashFixture) *uuid.UUID { return nil }, domain.PlacedOriginal, "photo.png"},
		{"as deep as it fits, an attachment below its deepest page", func(f trashFixture) uuid.UUID { return f.deep.ID },
			func(f trashFixture) *uuid.UUID { return f.deep.ParentID }, domain.PlacedOriginal, "Deep"},
		{"its parent in the trash", func(f trashFixture) uuid.UUID { return f.doc.ID },
			func(trashFixture) *uuid.UUID { return nil }, domain.PlacedParentInTrash, "Doc"},
		{"too deep", func(f trashFixture) uuid.UUID {
			top := f.page("Top", nil, 9)
			level := f.store.nodes[f.level.ID]
			level.ParentID = &top.ID
			f.store.nodes[f.level.ID] = level
			return f.deep.ID
		}, func(trashFixture) *uuid.UUID { return nil }, domain.PlacedTooDeep, "Deep"},
		{"a sibling of its name at the root", func(f trashFixture) uuid.UUID {
			f.page("Doc", nil, 8)
			return f.doc.ID
		}, func(trashFixture) *uuid.UUID { return nil }, domain.PlacedParentInTrash, "Doc 2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newTrashFixture()
			f.grant(domain.ActionRestore)
			id := tt.setup(f)
			before := f.store.trash[id].Node
			var last float64
			for _, n := range f.store.nodes {
				if n.ParentID == nil {
					last = max(last, n.SortOrder)
				}
			}
			group := f.groupUnder(id)
			w, spec := f.jobs(now(), 0, 0), f.jobSpec()
			plan, err := w.PlanRestore(f.asJob(), spec, id)
			if err != nil {
				t.Fatal(err)
			}
			done, err := f.runJob(w, spec, plan, 1)
			if err != nil {
				t.Fatalf("a member a batch = %+v, %v; want the item back", done, err)
			}
			n, parent, placed := f.store.nodes[id], tt.parent(f), done[0].Placed
			if placed == nil || placed.Reason != tt.reason || !domain.SameParent(placed.ParentID, parent) ||
				!domain.SameParent(n.ParentID, parent) || n.Name != tt.named || done[0].Renamed != boolInt(tt.named != before.Name) {
				t.Errorf("the first batch %+v placed %+v as %+v; want under %v as %q, %s", done[0], placed, n, parent, tt.named, tt.reason)
			}
			moved := tt.reason != domain.PlacedOriginal
			if moved && n.SortOrder != last+1 || !moved && n.SortOrder != before.SortOrder {
				t.Errorf("order %v, want %v: its own back where it was, after the root's last at the root", n.SortOrder, last+1)
			}
			for _, m := range group {
				if got, ok := f.store.nodes[m.ID]; !ok || !domain.SameParent(got.ParentID, m.ParentID) || got.Name != m.Name {
					t.Errorf("%s = %+v, want back under its parent", m.Name, got)
				}
			}
		})
	}
}

// groupUnder are the members of the trash item of the root id but the
// root: the nodes under it deleted at its time.
func (f trashFixture) groupUnder(id uuid.UUID) []domain.Node {
	var out []domain.Node
	at := *f.store.trash[id].DeletedAt
	for _, n := range f.store.trash {
		if !n.DeletedAt.Equal(at) {
			continue
		}
		for p := n.ParentID; p != nil; p = f.store.trash[*p].ParentID {
			if *p == id {
				out = append(out, n.Node)
				break
			}
			if _, ok := f.store.trash[*p]; !ok {
				break
			}
		}
	}
	return out
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// A member a batch restored made an item of its own, which someone
// restores whole: the next batch skips it and its members it holds, and
// leaves the members under it past the batch out of the trash items it
// makes, alive as they are; the batch after skips them (M8/P4 deviation
// 8; M8/P4A review C-M3).
func TestARestoreJobLeavesTheMembersUnderASkippedOneAlone(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRestore)
	box := f.page("Box", nil, 0)
	a := f.page("A", &box.ID, 0)
	f.page("A1", &a.ID, 0)
	f.page("A2", &a.ID, 1)
	b := f.page("B", &box.ID, 1)
	b1 := f.page("B1", &b.ID, 0)
	b2 := f.page("B2", &b.ID, 1)
	f.trashed(box.ID, now().Add(-time.Hour), f.alice)
	w, spec := f.jobs(now(), 0, 0), f.jobSpec()
	plan, err := w.PlanRestore(f.asJob(), spec, box.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 3); err != nil {
		t.Fatal(err)
	}
	f.restoredMeanwhile(b.ID, b1.ID, b2.ID)
	second, err := w.RunBatch(f.asJob(), spec, plan, 3, 6)
	if err != nil || second.Pages != 1 || second.Skipped != 2 {
		t.Fatalf("the batch of A2, B and B1 = %+v, %v; want A2 back, B and B1 skipped", second, err)
	}
	if _, ok := f.store.nodes[b2.ID]; !ok || slices.ContainsFunc(f.store.marks, func(m app.TrashMark) bool { return m.ID == b2.ID }) {
		t.Errorf("B2 marked an item of its own %v; want it alive, left alone", f.store.marks)
	}
	if third, err := w.RunBatch(f.asJob(), spec, plan, 6, 7); err != nil || third.Skipped != 1 || len(f.store.trash) != 0 {
		t.Errorf("the batch of B2 = %+v, %v, trash %v; want it skipped, the trash empty", third, err, f.store.trash)
	}
}

// An item of its root alone, restored by someone before the first batch,
// An item of its root alone, restored by someone before the first batch,
// is no longer the item the job restores: page.trash_changed, not a
// member skipped. A plan whose group is not the one its root counts is
// page.trash_changed too.
func TestARestoreJobsItemChangedBeforeItsFirstBatch(t *testing.T) {
	f := newBoxFixture()
	lone := f.page("Lone", nil, 5)
	f.trashed(lone.ID, f.at, f.alice)
	w, spec := f.jobs(now(), 0, 0), f.jobSpec()
	plan, err := w.PlanRestore(f.asJob(), spec, lone.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.restoredMeanwhile(lone.ID)
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 1); codeOf(err) != "page.trash_changed" {
		t.Errorf("RunBatch(the item restored meanwhile) = %v, want page.trash_changed", err)
	}
	c := f.store.counts[f.box.ID]
	c[1]++
	f.store.counts[f.box.ID] = c
	if _, err := w.PlanRestore(f.asJob(), spec, f.box.ID); codeOf(err) != "page.trash_changed" {
		t.Errorf("PlanRestore(counts of another group) = %v, want page.trash_changed", err)
	}
}

// The unit judges the item's time at its own: a batch past the time it
// can be restored in is page.restore_expired, nothing written.
func TestARestoreJobsBatchPastTheTimeIsExpired(t *testing.T) {
	f := newBoxFixture()
	at := now().Add(-retention + domain.TrashMargin - 30*time.Minute)
	n := f.store.trash[f.box.ID]
	n.DeletedAt = &at
	f.store.trash[f.box.ID] = n
	for _, id := range []uuid.UUID{f.a.ID, f.a1.ID, f.a2.ID, f.b.ID, f.c.ID} {
		m := f.store.trash[id]
		m.DeletedAt = &at
		f.store.trash[id] = m
	}
	w, spec := f.jobs(now().Add(-time.Hour), 0, 0), f.jobSpec()
	plan, err := w.PlanRestore(f.asJob(), spec, f.box.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 6); codeOf(err) != "page.restore_expired" || len(f.store.changesets) != 0 {
		t.Errorf("RunBatch() past the time = %v, changesets %v; want page.restore_expired, nothing", err, f.store.changesets)
	}
	if _, err := f.jobs(now(), 0, 0).PlanRestore(f.asJob(), spec, f.box.ID); codeOf(err) != "page.restore_expired" {
		t.Errorf("PlanRestore() past the time = %v, want page.restore_expired", err)
	}
}

// A batch of more than one step past the rows of history a unit restores
// is ErrJobBatchTooLarge before any content is parsed, and so is one whose
// pages hold more links than a unit's once they are; a lone step never,
// though its own history is past the bound: its deletion was one unit, and
// no batch is smaller (M8/P4A review C-M4).
func TestABatchTooLargeIsHalved(t *testing.T) {
	f := newBoxFixture()
	f.store.history = map[uuid.UUID]int{f.box.ID: 11, f.a.ID: 6}
	w, spec := f.jobs(now(), 0, 10), f.jobSpec()
	plan, err := w.PlanRestore(f.asJob(), spec, f.box.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.rec.calls = nil
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 2); !errors.Is(err, domain.ErrJobBatchTooLarge) || f.called("Facts") {
		t.Errorf("RunBatch(two steps past the history) = %v, calls %v; want ErrJobBatchTooLarge before a parse", err, f.rec.calls)
	}
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 1); err != nil {
		t.Errorf("RunBatch(the root alone, its history past the bound) = %v, want it restored", err)
	}
	trashed := f.store.trash[f.a.ID]
	f.store.trashed[f.a.ID] = app.Content{NodeID: f.a.ID, Content: strings.Repeat("[[x]]", 10_001), Revision: 2}
	f.store.trashed[f.a1.ID] = app.Content{NodeID: f.a1.ID, Content: strings.Repeat("[[x]]", 10_000), Revision: 2}
	f.store.history = nil
	if _, err := w.RunBatch(f.asJob(), spec, plan, 1, 3); !errors.Is(err, domain.ErrJobBatchTooLarge) {
		t.Errorf("RunBatch(two pages past the links) = %v, want ErrJobBatchTooLarge", err)
	}
	if n := f.store.trash[f.a.ID]; !n.DeletedAt.Equal(*trashed.DeletedAt) || f.budget.held != 0 {
		t.Errorf("A after a batch too large = %+v, the budget holding %d; want it in the trash, nothing held", n, f.budget.held)
	}
}

// A background restore's request checks the item unlocked, in
// startTrashRestore's order, and answers its name; its plan, as the job
// starts, refuses an item changed since.
func TestCheckRestore(t *testing.T) {
	f := newBoxFixture()
	other := uuid.NewV7()
	f.notebooks.workspaces[other] = f.acme
	q := f.page("Q", nil, 4)
	if name, err := f.jobs(now(), 0, 0).CheckRestore(f.asAlice(), f.eng, f.box.ID); err != nil || name != "Box" {
		t.Errorf("CheckRestore(Box) = %q, %v; want Box", name, err)
	}
	for _, tt := range []struct {
		name     string
		notebook uuid.UUID
		node     uuid.UUID
		setup    func()
		code     string
	}{
		{"no notebook", uuid.NewV7(), f.box.ID, nil, "notebook.not_found"},
		{"a reader", f.eng, f.box.ID, func() {
			f.auth.grants[domain.ActionRestore], f.auth.forbidden[domain.ActionRestore] = false, true
		}, "forbidden"},
		{"another notebook's node", other, f.box.ID, nil, "page.not_found"},
		{"no node", f.eng, uuid.NewV7(), nil, "page.not_found"},
		{"alive", f.eng, q.ID, nil, "page.not_in_trash"},
		{"within an item", f.eng, f.a.ID, nil, "page.not_trash_root"},
	} {
		if tt.setup != nil {
			tt.setup()
		}
		if _, err := f.jobs(now(), 0, 0).CheckRestore(f.asAlice(), tt.notebook, tt.node); codeOf(err) != tt.code {
			t.Errorf("%s: CheckRestore() = %v, want %s", tt.name, err, tt.code)
		}
		f.auth.forbidden = map[shared.Action]bool{}
		f.grant(domain.ActionRestore)
	}
	if _, err := f.jobs(now().Add(retention), 0, 0).CheckRestore(f.asAlice(), f.eng, f.box.ID); codeOf(err) != "page.restore_expired" {
		t.Errorf("CheckRestore(past its time) = %v, want page.restore_expired", err)
	}
	f.restoredMeanwhile(f.box.ID)
	if _, err := f.jobs(now(), 0, 0).PlanRestore(f.asJob(), f.jobSpec(), f.box.ID); codeOf(err) != "page.trash_changed" {
		t.Errorf("PlanRestore(Box restored meanwhile) = %v, want page.trash_changed", err)
	}
}

// revertJob plans the background revert of the changeset id as alice's
// job, with message, the trash read at at, and answers its writes, spec
// and plan.
func (f *fixture) revertJob(t *testing.T, id uuid.UUID, message *string, at time.Time) (*app.JobWrites, app.JobSpec, app.JobPlan) {
	t.Helper()
	w, spec := f.jobs(at, 0, 0), f.jobSpec()
	spec.Message = message
	plan, err := w.PlanRevert(f.asJob(), spec, id)
	if err != nil {
		t.Fatal(err)
	}
	return w, spec, plan
}

// A background revert plans its round as the request does, a step each
// node a restore brings back, its group in pre-order; each batch is a unit
// of the job's changeset, a revert of the changeset with the message, its
// first batch's: the group's first planned again whole, its root put back
// where the changeset deleted it, the members left in the trash items of
// their own; the later members each checked alone, no plan of theirs,
// and put back where they were.
func TestARevertJobRestoresAGroupInBatches(t *testing.T) {
	f := newRevertFixture()
	message := "Undo the clean-up"
	w, spec, plan := f.revertJob(t, f.c.ID, &message, now())
	if steps := plan.Steps(); len(steps) != 3 || plan.Total() != 3 || steps[1].Bytes != int64(len("# Doc [[Other]]")) {
		t.Fatalf("PlanRevert() = %+v; want Box, Doc and scan.pdf a step each, Doc's of its bytes", steps)
	}
	var done []app.BatchDone
	var second []string
	for from := range 3 {
		f.rec.calls = nil
		d, err := w.RunBatch(f.asJob(), spec, plan, from, from+1)
		if err != nil {
			t.Fatal(err)
		}
		done = append(done, d)
		if from == 1 {
			second = f.rec.calls
		}
	}
	if done[0].Restored != 1 || done[1].Restored != 1 || done[2].Restored != 1 || done[0].Nodes != 1 {
		t.Errorf("batches = %+v; want a node restored each", done)
	}
	// The second batch, Doc's, reads its item and its trash, not the tree.
	if slices.Contains(second, "ListNodes in tx") || slices.ContainsFunc(second, func(c string) bool { return strings.HasPrefix(c, "NodesNow") }) ||
		!slices.Contains(second, "ChangesetItemsOf 1 in tx") || !slices.Contains(second, "TrashNodesAt 1 in tx") ||
		!slices.Contains(second, "VersionHashes 1 in tx") {
		t.Errorf("the second batch's calls %v; want Doc's item, its trash and its version's hash, no plan", second)
	}
	cs := f.store.changesets
	if len(cs) != 1 || cs[0].ID != spec.Changeset || cs[0].Kind != domain.ChangesetRevert || cs[0].RevertsID != f.c.ID ||
		cs[0].Message == nil || *cs[0].Message != message || f.store.touched[spec.Changeset].IsZero() {
		t.Errorf("changesets %+v; want the job's revert of C with its message, the later batches merged into it", cs)
	}
	for _, n := range []domain.Node{f.box, f.doc, f.scan} {
		if got, ok := f.store.nodes[n.ID]; !ok || !sameParent(got.ParentID, n.ParentID) || got.Name != n.Name {
			t.Errorf("%s = %+v, want back where it was", n.Name, got)
		}
	}
	// A background job's nodes are queued for the search in bulk (M8/P5A
	// review C-I1).
	if q := f.store.queued; len(q) != 3 || slices.ContainsFunc(q, func(q queuing) bool { return !q.bulk || len(q.ids) != 1 }) {
		t.Errorf("queued %+v; want each batch's node in bulk", q)
	}
}

// A group's later member is checked alone, as it goes back where the
// changeset deleted it from, without a number (M8/P4 design 3.5): one
// someone restored there is skipped; elsewhere, changed; its parent gone,
// its name held there, too deep, past its time, or no item of its counts
// any more, each a conflict.
func TestARevertJobsContinuationIsCheckedAlone(t *testing.T) {
	for _, tt := range []struct {
		name    string
		between func(f revertFixture) *uuid.UUID
		reason  domain.ConflictReason
	}{
		{"restored where it was", func(f revertFixture) *uuid.UUID {
			f.restoredMeanwhile(f.doc.ID)
			return nil
		}, ""},
		{"restored elsewhere", func(f revertFixture) *uuid.UUID {
			f.restoredMeanwhile(f.doc.ID)
			doc := f.store.nodes[f.doc.ID]
			doc.Name, doc.NameKey = "Doc 2", "doc 2"
			f.store.nodes[f.doc.ID] = doc
			return nil
		}, domain.ConflictChanged},
		{"no item of its counts", func(f revertFixture) *uuid.UUID {
			f.store.counts[f.doc.ID] = [3]int{2, 0, 1}
			return nil
		}, domain.ConflictChanged},
		{"past its time", func(f revertFixture) *uuid.UUID {
			f.clock.reads += int(retention / time.Microsecond)
			return nil
		}, domain.ConflictExpired},
		{"its parent deleted again", func(f revertFixture) *uuid.UUID {
			f.trashed(f.box.ID, now(), f.alice)
			return nil
		}, domain.ConflictParentGone},
		{"its name held", func(f revertFixture) *uuid.UUID {
			holder := f.page("doc", &f.box.ID, 5)
			return &holder.ID
		}, domain.ConflictTitleTaken},
		{"too deep", func(f revertFixture) *uuid.UUID {
			var parent *uuid.UUID
			for range domain.MaxDepth - 1 {
				n := f.page("Level", parent, 0)
				parent = &n.ID
			}
			box := f.store.nodes[f.box.ID]
			box.ParentID = parent
			f.store.nodes[f.box.ID] = box
			return nil
		}, domain.ConflictTooDeep},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newRevertFixture()
			w, spec, plan := f.revertJob(t, f.c.ID, nil, now())
			if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 1); err != nil {
				t.Fatal(err)
			}
			holder := tt.between(f)
			done, err := w.RunBatch(f.asJob(), spec, plan, 1, 3)
			if tt.reason == "" {
				if err != nil || done.Skipped != 1 || done.Restored != 1 {
					t.Fatalf("RunBatch() = %+v, %v; want Doc skipped, scan.pdf restored", done, err)
				}
				return
			}
			var e *shared.Error
			if !errors.As(err, &e) || e.Code != "page.revert_conflict" || len(e.Conflicts) == 0 {
				t.Fatalf("RunBatch() = %v, want page.revert_conflict", err)
			}
			c := e.Conflicts[0]
			if c.NodeID != f.doc.ID || c.Name != "Doc" || c.Reason != string(tt.reason) || holder != nil && (c.HolderID == nil ||
				*c.HolderID != *holder) {
				t.Errorf("conflicts %+v, want Doc's %s", e.Conflicts, tt.reason)
			}
			if _, back := f.store.nodes[f.scan.ID]; back {
				t.Errorf("scan.pdf restored by a batch that stopped")
			}
		})
	}
}

// A group's later members and the first of another, in one batch: the
// plan of the batch holds the other's alone, so the members are no
// conflict of it (M8/P4 review A-M3).
func TestARevertJobsContinuationBesideANewTop(t *testing.T) {
	f := newRevertFixture()
	other := f.store.nodes[f.other.ID]
	f.trashed(f.other.ID, f.deletedAt, f.alice)
	f.c.items = append(f.c.items, domain.ChangesetItem{NodeID: other.ID, Before: new(other.State()), Revision: 1})
	f.log.versions[domain.VersionRef{NodeID: other.ID, Revision: 1}] = ""
	w, spec, plan := f.revertJob(t, f.c.ID, nil, now())
	if len(plan.Steps()) != 4 {
		t.Fatalf("PlanRevert() = %+v, want Box's group and Other", plan.Steps())
	}
	done, err := f.runJob(w, spec, plan, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 2 || done[1].Restored != 2 || len(f.store.trash) != 0 {
		t.Errorf("batches %+v, trash %v; want all back, the second batch scan.pdf and Other", done, f.store.trash)
	}
}

// A group someone restored where it was and deleted again, unchanged,
// since the job's plan: the batch that begins it plans it at the time it
// is deleted at now, as a request's revert would, and reads its pages'
// contents and counts its history there; the later batches read its
// members at that time once the first commits (M8/P4A review A-M1, fix
// check K-1). A batch of the whole group, and batches of a member each;
// then the group's history past the bound at its new time.
func TestARevertJobOfAGroupDeletedAgainSinceThePlan(t *testing.T) {
	for _, size := range []int{3, 1} {
		f := newRevertFixture()
		w, spec, plan := f.revertJob(t, f.c.ID, nil, now())
		f.restoredMeanwhile(f.box.ID, f.doc.ID, f.scan.ID)
		f.trashed(f.box.ID, now().Add(-time.Minute), f.alice)
		done, err := f.runJob(w, spec, plan, size)
		if err != nil {
			t.Fatalf("batches of %d: runJob() = %+v, %v; want Box's group back", size, done, err)
		}
		restored := 0
		for _, d := range done {
			restored += d.Restored
		}
		if restored != 3 || len(f.store.trash) != 0 {
			t.Errorf("batches of %d: %+v, trash %v; want the three restored", size, done, f.store.trash)
		}
		for _, n := range []domain.Node{f.box, f.doc, f.scan} {
			if got, ok := f.store.nodes[n.ID]; !ok || !sameParent(got.ParentID, n.ParentID) || got.Name != n.Name {
				t.Errorf("batches of %d: %s = %+v, want back where it was", size, n.Name, got)
			}
		}
	}
	f := newRevertFixture()
	w, spec := f.jobs(now(), 0, 10), f.jobSpec()
	plan, err := w.PlanRevert(f.asJob(), spec, f.c.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.restoredMeanwhile(f.box.ID, f.doc.ID, f.scan.ID)
	f.trashed(f.box.ID, now().Add(-time.Minute), f.alice)
	f.store.history = map[uuid.UUID]int{f.box.ID: 6, f.doc.ID: 6}
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 3); !errors.Is(err, domain.ErrJobBatchTooLarge) {
		t.Errorf("RunBatch(the group, 12 rows of history at its new time, 10 a unit) = %v, want ErrJobBatchTooLarge", err)
	}
	if _, err := f.runJob(w, spec, plan, 1); err != nil {
		t.Errorf("a member a batch = %v, want the group back", err)
	}
}

// A group's later member that cannot go back keeps the members under it
// in the same batch, at any depth, out of the conflicts: they are none of
// their own, as a request's revert has a group's, and the cause comes
// first (M8/P4A review A-M3, fix check K-2).
func TestARevertJobsContinuationListsTheCauseAlone(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRevert)
	f.names[f.alice] = "Alice"
	box := f.page("Box", nil, 0)
	a := f.page("A", &box.ID, 0)
	a1 := f.page("A1", &a.ID, 0)
	f.page("A1a", &a1.ID, 0)
	c := f.deletion(box.ID, now().Add(-time.Hour))
	w, spec, plan := f.revertJob(t, c.ID, nil, now())
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 1); err != nil {
		t.Fatal(err)
	}
	holder := f.page("a", &box.ID, 3)
	_, err := w.RunBatch(f.asJob(), spec, plan, 1, 4)
	var e *shared.Error
	if !errors.As(err, &e) || e.Code != "page.revert_conflict" || e.ConflictCount == nil {
		t.Fatalf("RunBatch(A, A1 and A1a) = %v, want page.revert_conflict", err)
	}
	if len(e.Conflicts) != 1 || *e.ConflictCount != 1 || e.Conflicts[0].NodeID != a.ID ||
		e.Conflicts[0].Reason != string(domain.ConflictTitleTaken) || e.Conflicts[0].HolderID == nil || *e.Conflicts[0].HolderID != holder.ID {
		t.Errorf("conflicts %+v, count %d; want A's name held by a alone", e.Conflicts, *e.ConflictCount)
	}
}

// A group's later members, a member a batch, go back as deep as pages
// go: a page at the tenth level, an attachment below it, which is no
// level (M8/P4A review C-M2).
func TestARevertJobsContinuationAtTheDeepestLevel(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRevert)
	f.names[f.alice] = "Alice"
	var parent *uuid.UUID
	for range domain.MaxDepth - 2 {
		n := f.page("Level", parent, 0)
		parent = &n.ID
	}
	deep := f.page("Deep", parent, 0)
	leaf := f.page("Leaf", &deep.ID, 0)
	png := f.asset("leaf.png", &leaf.ID, 1)
	c := f.deletion(deep.ID, now().Add(-time.Hour))
	w, spec, plan := f.revertJob(t, c.ID, nil, now())
	done, err := f.runJob(w, spec, plan, 1)
	if err != nil || len(done) != 3 {
		t.Fatalf("a member a batch = %+v, %v; want Deep, Leaf and leaf.png back", done, err)
	}
	for _, n := range []domain.Node{deep, leaf, png} {
		if got, ok := f.store.nodes[n.ID]; !ok || !sameParent(got.ParentID, n.ParentID) {
			t.Errorf("%s = %+v, want back where it was", n.Name, got)
		}
	}
}

// creation makes the changeset of the creation of the pages ids, in this
// order, each where it is.
func (f *fixture) creation(ids ...uuid.UUID) *fakeChangeset {
	var items []domain.ChangesetItem
	var versions []domain.ChangesetVersion
	for _, id := range ids {
		n := f.store.nodes[id]
		items = append(items, domain.ChangesetItem{NodeID: id, After: new(n.State())})
		c := f.store.contents[id]
		versions = append(versions, f.log.version(id, 0, c.Revision, c.Content))
	}
	return f.log.changeset(domain.ChangesetEdit, f.alice, now(), items, versions...)
}

// A batch plans its own steps' nodes again: a node put under a top it
// deletes since is children_added, and stops it; a later step's change
// does not stop the batches before it.
func TestARevertJobsBatchPlansItsOwnNodes(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRevert)
	folder := f.page("Folder", nil, 0)
	x := f.page("X", &folder.ID, 0)
	y := f.page("Y", nil, 1)
	c := f.creation(folder.ID, x.ID, y.ID)
	w, spec, plan := f.revertJob(t, c.ID, nil, now())
	if steps := plan.Steps(); len(steps) != 2 || steps[0].Nodes != 2 || steps[1].Nodes != 1 {
		t.Fatalf("PlanRevert() = %+v; want Folder's subtree, then Y", steps)
	}
	renamed := f.store.nodes[y.ID]
	renamed.Name, renamed.NameKey = "Y2", "y2"
	f.store.nodes[y.ID] = renamed
	if done, err := w.RunBatch(f.asJob(), spec, plan, 0, 1); err != nil || done.Deleted != 2 {
		t.Fatalf("RunBatch(Folder) = %+v, %v; want its subtree deleted, Y's change not its", done, err)
	}
	if _, err := w.RunBatch(f.asJob(), spec, plan, 1, 2); codeOf(err) != "page.revert_conflict" {
		t.Errorf("RunBatch(Y) = %v, want page.revert_conflict", err)
	}

	g := newFixture()
	g.grant(domain.ActionRevert)
	folder = g.page("Folder", nil, 0)
	x = g.page("X", &folder.ID, 0)
	c = g.creation(folder.ID, x.ID)
	w, spec, plan = g.revertJob(t, c.ID, nil, now())
	z := g.page("Z", &folder.ID, 1)
	var e *shared.Error
	if _, err := w.RunBatch(g.asJob(), spec, plan, 0, 1); !errors.As(err, &e) || len(e.Conflicts) != 1 ||
		e.Conflicts[0].NodeID != z.ID || e.Conflicts[0].Reason != string(domain.ConflictChildrenAdded) {
		t.Errorf("RunBatch(Folder, Z put under it) = %v; want Z children_added", err)
	}
}

// A batch names a node it finds changed by its name in S*, as the job's
// start and the web's request do (RevertConflict; M9/P3 S3 review B-I1):
// M8's rename of P to Q, renamed again to R before the job's batch, is
// changed, named P by the batch and by a start planned again.
func TestARevertJobsBatchNamesAChangeAsTheStartDoes(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRevert)
	p := f.page("P", nil, 0)
	c := f.renaming(p.ID, "Q")
	w, spec, plan := f.revertJob(t, c.ID, nil, now())
	n := f.store.nodes[p.ID]
	n.Name, n.NameKey = "R", "r"
	f.store.nodes[p.ID] = n
	var e *shared.Error
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 1); !errors.As(err, &e) || len(e.Conflicts) != 1 || e.Conflicts[0].NodeID != p.ID ||
		e.Conflicts[0].Reason != string(domain.ConflictChanged) || e.Conflicts[0].Name != "P" {
		t.Errorf("RunBatch(P renamed R since the plan) = %v %+v; want P changed, named P", err, conflictsOf(err))
	}
	if _, err := w.PlanRevert(f.asJob(), spec, c.ID); !errors.As(err, &e) || len(e.Conflicts) != 1 || e.Conflicts[0].Name != "P" {
		t.Errorf("PlanRevert(P renamed R) = %v %+v; want P changed, named P", err, conflictsOf(err))
	}
}

// A batch names a page it writes back by its name in S* too (review B's
// M-1): the batch reads the page's version, not its item. A changeset
// renames X to X2, writes it, and makes a new page X at the root, so X's
// place waits for that page's deletion: [delete the new X], [write X
// back], [put X back], a batch each. Someone writes X2 after the first:
// the second batch finds it changed, named X, as a start planned again
// names it (with the new X, which the first batch deleted), not X2.
func TestARevertJobsBatchNamesAPageItWritesBackAsTheStartDoes(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRevert)
	x := f.page("X", nil, 0)
	f.content(x.ID, "x before", 1)
	was := x.State()
	n := f.store.nodes[x.ID]
	n.Name, n.NameKey = "X2", "x2"
	f.store.nodes[x.ID] = n
	f.log.versions[domain.VersionRef{NodeID: x.ID, Revision: 1}] = "x before"
	f.content(x.ID, "x by C", 2)
	made := f.page("X", nil, 1)
	f.content(made.ID, "new", 1)
	c := f.log.changeset(domain.ChangesetEdit, f.alice, now(), []domain.ChangesetItem{
		{NodeID: x.ID, Before: &was, After: new(n.State())}, {NodeID: made.ID, After: new(made.State()), Revision: 1}},
		f.log.version(x.ID, 1, 2, "x by C"), f.log.version(made.ID, 0, 1, "new"))
	w, spec, plan := f.revertJob(t, c.ID, nil, now())
	steps, _ := plan.RevertSteps()
	if len(steps) != 3 || steps[0].Kind != domain.RevertDelete || steps[1].Kind != domain.RevertContent || steps[1].Node != x.ID ||
		plan.Steps()[0].Joined || plan.Steps()[1].Joined {
		t.Fatalf("the steps %v, want the new X's deletion, X's write back and its place, apart", steps)
	}
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 1); err != nil {
		t.Fatalf("RunBatch(the new X's deletion) = %v", err)
	}
	f.content(x.ID, "x by someone", 3)
	var e *shared.Error
	if _, err := w.RunBatch(f.asJob(), spec, plan, 1, 2); !errors.As(err, &e) || len(e.Conflicts) != 1 || e.Conflicts[0].NodeID != x.ID ||
		e.Conflicts[0].Reason != string(domain.ConflictChanged) || e.Conflicts[0].Name != "X" {
		t.Errorf("RunBatch(X's write back, X2 written since) = %v %+v; want X changed, named X", err, conflictsOf(err))
	}
	_, err := w.PlanRevert(f.asJob(), spec, c.ID)
	if i := slices.IndexFunc(conflictsOf(err), func(c shared.Conflict) bool { return c.NodeID == x.ID }); i < 0 ||
		conflictsOf(err)[i].Name != "X" {
		t.Errorf("PlanRevert(X2 written since) = %v %+v; want X changed, named X", err, conflictsOf(err))
	}
}

// conflictsOf are err's conflicts, none when it is no problem of the API.
func conflictsOf(err error) []shared.Conflict {
	var e *shared.Error
	if !errors.As(err, &e) {
		return nil
	}
	return e.Conflicts
}

// A batch's write refused by the edit lock stops it as it is, page.locked
// naming the holder; a title taken in its place, which its plan found
// free, is a defect, no problem of the API; the restore's rows the purge
// took, page.trash_changed in the unit, is page.changeset_trimmed, as a
// later member's version the purge took between two batches.
func TestARevertJobsBatchErrors(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRevert)
	bob := uuid.NewV7()
	f.names[bob] = "Bob"
	folder := f.page("Folder", nil, 0)
	c := f.creation(folder.ID)
	_, spec, plan := f.revertJob(t, c.ID, nil, now())
	f.guards = []app.WriteGuard{app.NewEditLock(f.store, f.names)}
	w := f.jobs(now(), 0, 0)
	f.session(folder.ID, bob, now().Add(time.Hour))
	var e *shared.Error
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 1); !errors.As(err, &e) || e.Code != "page.locked" || e.Lock == nil ||
		e.Lock.UserID != bob || e.Lock.DisplayName != "Bob" {
		t.Errorf("RunBatch(a page Bob edits) = %v, want page.locked naming Bob", err)
	}

	g := newFixture()
	g.grant(domain.ActionRevert)
	p := g.page("P", nil, 0)
	c = g.renaming(p.ID, "Q")
	w, spec, plan = g.revertJob(t, c.ID, nil, now())
	g.store.errs = map[string]error{"RenameNode": domain.ErrTitleTaken}
	if _, err := w.RunBatch(g.asJob(), spec, plan, 0, 1); err == nil || errors.As(err, &e) {
		t.Errorf("RunBatch(a title taken in its place) = %v, want a defect, unclassified", err)
	}
	h := newFixture()
	h.grant(domain.ActionRevert)
	q := h.page("Q", nil, 0)
	h.content(q.ID, "before", 2)
	c = h.writing(q.ID, "after")
	w, spec, plan = h.revertJob(t, c.ID, nil, now())
	h.store.errs = map[string]error{"WriteContent": domain.ErrRevisionMismatch}
	if _, err := w.RunBatch(h.asJob(), spec, plan, 0, 1); err == nil || errors.As(err, &e) {
		t.Errorf("RunBatch(a revision written meanwhile) = %v, want a defect, unclassified", err)
	}

	r := newRevertFixture()
	w, spec, plan = r.revertJob(t, r.c.ID, nil, now())
	r.store.rows = &app.RestoredRows{}
	if _, err := w.RunBatch(r.asJob(), spec, plan, 0, 1); codeOf(err) != "page.changeset_trimmed" {
		t.Errorf("RunBatch(rows the purge took) = %v, want page.changeset_trimmed", err)
	}
	v := newRevertFixture()
	w, spec, plan = v.revertJob(t, v.c.ID, nil, now())
	if _, err := w.RunBatch(v.asJob(), spec, plan, 0, 1); err != nil {
		t.Fatal(err)
	}
	for ref := range v.log.versions {
		if ref.NodeID == v.doc.ID {
			delete(v.log.versions, ref)
		}
	}
	if _, err := w.RunBatch(v.asJob(), spec, plan, 1, 3); codeOf(err) != "page.changeset_trimmed" {
		t.Errorf("RunBatch(Doc's version purged after the first batch) = %v, want page.changeset_trimmed", err)
	}
}

// A background revert does what S2's job refused until its batches kept
// segments whole (M9/P3 design section 0, 3.22.2): a step aside to its
// node's place, a page restored then written back, a page written back
// then deleted, each a segment, joined, its batches a step each grown to
// keep it whole, the notebook as before the changeset. A request reverts
// each too, its log line counting its steps, those aside too (M9/P3
// design 3.21.3).
func TestARevertJobKeepsItsSegmentsWhole(t *testing.T) {
	var written, deleted uuid.UUID // the pages of rows 2 and 3, by id, as each build made them last
	for _, tt := range []struct {
		name   string
		build  func(f *fixture) uuid.UUID
		joined []bool
		done   func(f *fixture) string // what is not as before the changeset
		steps  string                  // the request's log line's counts
	}{
		{"a swap of names: a step aside", func(f *fixture) uuid.UUID {
			a, b := f.page("A", nil, 5), f.page("B", nil, 6)
			sa, sb := a.State(), b.State()
			a.Name, a.NameKey, b.Name, b.NameKey = "B", "b", "A", "a"
			f.store.nodes[a.ID], f.store.nodes[b.ID] = a, b
			return f.log.changeset(domain.ChangesetEdit, f.alice, now(), []domain.ChangesetItem{
				{NodeID: a.ID, Before: &sa, After: new(a.State())}, {NodeID: b.ID, Before: &sb, After: new(b.State())}}).ID
		}, []bool{true, true, false}, func(f *fixture) string {
			var names []string
			for _, n := range f.store.nodes {
				names = append(names, fmt.Sprintf("%s@%v", n.Name, n.SortOrder))
			}
			slices.Sort(names)
			if !slices.Equal(names, []string{"A@5", "B@6"}) {
				return fmt.Sprintf("the pages %v", names)
			}
			return ""
		}, "placed=2 deleted=0 restored=0 written=0 aside=1"},
		{"a page written, then deleted: restored, then written back", func(f *fixture) uuid.UUID {
			p := f.page("P", nil, 0)
			f.content(p.ID, "before", 1)
			f.content(p.ID, "written", 2)
			at := now().Add(-time.Hour)
			f.trashed(p.ID, at, f.alice)
			written = p.ID
			return f.log.changeset(domain.ChangesetEdit, f.alice, at, []domain.ChangesetItem{{NodeID: p.ID, Before: new(p.State()),
				Revision: 2}}, f.log.version(p.ID, 1, 2, "written")).ID
		}, []bool{true, false}, func(f *fixture) string {
			_, alive := f.store.nodes[written]
			if c, ok := f.store.contents[written]; !alive || !ok || c.Content != "before" || c.Revision != 3 || len(f.store.trash) != 0 {
				return fmt.Sprintf("P alive %v, its content %q at %d, the trash %v", alive, c.Content, c.Revision, f.store.trash)
			}
			return ""
		}, "placed=0 deleted=0 restored=1 written=1 aside=0"},
		{"a page restored and written, a revert's: written back, then deleted", func(f *fixture) uuid.UUID {
			p := f.page("P", nil, 0)
			f.content(p.ID, "restored", 1)
			f.content(p.ID, "written", 2)
			deleted = p.ID
			return f.log.changeset(domain.ChangesetRevert, f.alice, now(), []domain.ChangesetItem{{NodeID: p.ID, After: new(p.State()),
				Restored: true, Revision: 2}}, f.log.version(p.ID, 1, 2, "written")).ID
		}, []bool{true, false}, func(f *fixture) string {
			if c, ok := f.store.trashed[deleted]; !ok || c.Content != "restored" || len(f.store.nodes) != 0 {
				return fmt.Sprintf("P in the trash %v, its content there %q, alive %v", ok, c.Content, f.store.nodes)
			}
			return ""
		}, "placed=0 deleted=1 restored=0 written=1 aside=0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRevert)
			id := tt.build(f)
			w, spec, plan := f.revertJob(t, id, nil, now())
			var joined []bool
			for _, s := range plan.Steps() {
				joined = append(joined, s.Joined)
			}
			if !slices.Equal(joined, tt.joined) {
				t.Errorf("the steps joined %v, want %v", joined, tt.joined)
			}
			if done, err := f.runJob(w, spec, plan, 1); err != nil || len(done) != 1 {
				t.Fatalf("runJob(a step a batch) = %+v, %v; want one batch, the segment whole", done, err)
			}
			if bad := tt.done(f); bad != "" {
				t.Errorf("after the job: %s", bad)
			}
			g := newFixture()
			g.grant(domain.ActionRevert)
			if _, err := g.revert(tt.build(g), nil, 0); err != nil {
				t.Errorf("the request's revert = %v, want it done", err)
			}
			if logs := g.logs.String(); !strings.Contains(logs, "changeset reverted") || !strings.Contains(logs, tt.steps) {
				t.Errorf("the request's log %q, want %s", logs, tt.steps)
			}
		})
	}
}

// A revert of nothing to change plans no step: its job writes nothing.
func TestARevertJobOfNothingToChange(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRevert)
	p := f.page("P", nil, 0)
	f.content(p.ID, "same", 2)
	c := f.writing(p.ID, "other")
	f.content(p.ID, "same", 3)
	c.versions[0].Hash = hashOf("same")
	_, _, plan := f.revertJob(t, c.ID, nil, now())
	if len(plan.Steps()) != 0 || plan.Total() != 0 {
		t.Errorf("PlanRevert() = %+v, want no step", plan.Steps())
	}
}

// A background revert's request checks the changeset unlocked in
// startRevert's order (M8/P4 design 3.5): the message, the notebook and
// the decision, the changeset of the notebook, its mark, its items past
// MaxJobItems before admit, which runs before anything else is read; then
// as the request's check, its conflicts and the versions the purge took;
// then the pages it deletes or writes back an edit session holds, the
// first in the plan's order: a deletion's held by another account, a
// write back's by any.
func TestCheckRevertAnswersItsCodesInOrder(t *testing.T) {
	empty := ""
	busy := shared.NewError(shared.KindConflict, "transfer.busy", "busy")
	for _, tt := range []struct {
		name     string
		setup    func(f revertFixture) (uuid.UUID, uuid.UUID)
		message  *string
		maxItems int
		admit    error
		want     string
	}{
		{"an empty message", func(f revertFixture) (uuid.UUID, uuid.UUID) { return uuid.NewV7(), f.c.ID }, &empty, 0, nil,
			shared.CodeValidationFailed + " message"},
		{"no notebook", func(f revertFixture) (uuid.UUID, uuid.UUID) { return uuid.NewV7(), f.c.ID }, nil, 0, nil, "notebook.not_found"},
		{"a reader", func(f revertFixture) (uuid.UUID, uuid.UUID) {
			delete(f.auth.grants, domain.ActionRevert)
			f.auth.forbidden[domain.ActionRevert] = true
			return f.eng, uuid.NewV7()
		}, nil, 0, nil, "forbidden"},
		{"another notebook's changeset", func(f revertFixture) (uuid.UUID, uuid.UUID) {
			other := uuid.NewV7()
			f.notebooks.workspaces[other] = f.acme
			return other, f.c.ID
		}, nil, 0, nil, "page.changeset_not_found"},
		{"trimmed, busy", func(f revertFixture) (uuid.UUID, uuid.UUID) {
			f.c.Trimmed = true
			return f.eng, f.c.ID
		}, nil, 0, busy, "page.changeset_trimmed"},
		{"past the items, busy", func(f revertFixture) (uuid.UUID, uuid.UUID) { return f.eng, f.c.ID }, nil, 2, busy,
			"page.revert_too_large"},
		{"at the items, busy", func(f revertFixture) (uuid.UUID, uuid.UUID) { return f.eng, f.c.ID }, nil, 3, busy, "transfer.busy"},
		{"its notebook deleted between its items and its row", func(f revertFixture) (uuid.UUID, uuid.UUID) {
			f.rec.before = map[string]func(){"ChangesetVersions": func() {
				f.c.gone = true
				delete(f.notebooks.workspaces, f.eng)
			}}
			return f.eng, f.c.ID
		}, nil, 0, nil, "notebook.not_found"},
		{"a conflict", func(f revertFixture) (uuid.UUID, uuid.UUID) {
			f.store.nodes[f.other.ID] = domain.Node{ID: f.other.ID, NotebookID: f.eng, Kind: domain.KindPage, Name: "box", NameKey: "box"}
			return f.eng, f.c.ID
		}, nil, 0, nil, "page.revert_conflict"},
		{"a version the purge took", func(f revertFixture) (uuid.UUID, uuid.UUID) {
			delete(f.log.versions, domain.VersionRef{NodeID: f.doc.ID, Revision: 3})
			return f.eng, f.c.ID
		}, nil, 0, nil, "page.changeset_trimmed"},
		{"none of it", func(f revertFixture) (uuid.UUID, uuid.UUID) { return f.eng, f.c.ID }, nil, 0, nil, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newRevertFixture()
			notebook, id := tt.setup(f)
			admitted := false
			kind, err := f.jobs(now(), tt.maxItems, 0).CheckRevert(f.asAlice(), notebook, id, tt.message, func(context.Context) error {
				admitted = true
				f.rec.calls = append(f.rec.calls, "Admit")
				return tt.admit
			})
			if codeOf(err) != tt.want {
				t.Fatalf("CheckRevert() = %q, %v; want %s", kind, err, tt.want)
			}
			if tt.want == "" && kind != domain.ChangesetEdit {
				t.Errorf("CheckRevert() = %q, want the changeset's kind", kind)
			}
			if i := slices.Index(f.rec.calls, "Admit"); admitted && (f.rec.calls[i-1] != "CountItems" || i != len(f.rec.calls)-1 && tt.admit != nil) {
				t.Errorf("calls %v; want admit right after the items' count, nothing read after a refusal", f.rec.calls)
			}
		})
	}
}

// The edit locks a background revert's request refuses (M8/P4 design 3.5,
// review B-M3): a page it deletes another account edits; a page it writes
// back anyone edits; the reverter's own session on a page it deletes ends
// with it; the first in the plan's order.
func TestCheckRevertRefusesTheLockedPages(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRevert)
	bob := uuid.NewV7()
	f.names[bob], f.names[f.alice] = "Bob", "Alice"
	folder := f.page("Folder", nil, 0)
	x := f.page("X", &folder.ID, 0)
	created := f.creation(folder.ID, x.ID)
	p := f.page("P", nil, 1)
	f.content(p.ID, "one", 1)
	written := f.writing(p.ID, "two")
	check := func(id uuid.UUID) error {
		_, err := f.jobs(now(), 0, 0).CheckRevert(f.asAlice(), f.eng, id, nil, nil)
		return err
	}
	f.session(x.ID, f.alice, now().Add(time.Hour))
	if err := check(created.ID); err != nil {
		t.Errorf("CheckRevert(a deletion of the reverter's own page) = %v, want none", err)
	}
	f.session(p.ID, f.alice, now().Add(time.Hour))
	var e *shared.Error
	if err := check(written.ID); !errors.As(err, &e) || e.Code != "page.locked" || e.Lock.PageID != p.ID || e.Lock.UserID != f.alice {
		t.Errorf("CheckRevert(a write back of the reverter's page) = %v, want page.locked by Alice", err)
	}
	f.session(x.ID, bob, now().Add(time.Hour).Add(time.Minute))
	f.session(folder.ID, bob, now().Add(time.Hour))
	if err := check(created.ID); !errors.As(err, &e) || e.Code != "page.locked" || e.Lock.PageID != folder.ID ||
		e.Lock.DisplayName != "Bob" {
		t.Errorf("CheckRevert(a deletion Bob edits) = %v, want page.locked of Folder, the first, by Bob", err)
	}
	// The job's plan checks the locks again: one taken since the start
	// stops it, nothing written.
	if _, err := f.jobs(now(), 0, 0).PlanRevert(f.asJob(), f.jobSpec(), created.ID); !errors.As(err, &e) || e.Code != "page.locked" ||
		e.Lock.PageID != folder.ID {
		t.Errorf("PlanRevert(a deletion Bob edits) = %v, want page.locked of Folder", err)
	}
}
