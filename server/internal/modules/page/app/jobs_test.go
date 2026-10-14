package app_test

import (
	"context"
	"errors"
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
// worker cuts them, and answers each batch's, up to the first error.
func (f *fixture) runJob(w *app.JobWrites, spec app.JobSpec, plan app.JobPlan, size int) ([]app.BatchDone, error) {
	var out []app.BatchDone
	for from := 0; from < len(plan.Steps()); from += size {
		done, err := w.RunBatch(f.asJob(), spec, plan, from, min(from+size, len(plan.Steps())))
		if err != nil {
			return out, err
		}
		out = append(out, done)
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
		"CreateChangeset in tx", "RestoreNodes in tx", "RecordItems 3 in tx", "MarkTrashRoots 3 in tx", "Release 5", "Release 4",
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
// is ErrBatchTooLarge before any content is parsed, and so is one whose
// pages hold more links than a unit's once they are; a lone step never.
func TestABatchTooLargeIsHalved(t *testing.T) {
	f := newBoxFixture()
	f.store.history = map[uuid.UUID]int{f.box.ID: 6, f.a.ID: 6}
	w, spec := f.jobs(now(), 0, 10), f.jobSpec()
	plan, err := w.PlanRestore(f.asJob(), spec, f.box.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.rec.calls = nil
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 2); !errors.Is(err, domain.ErrBatchTooLarge) || f.called("Facts") {
		t.Errorf("RunBatch(two steps past the history) = %v, calls %v; want ErrBatchTooLarge before a parse", err, f.rec.calls)
	}
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 1); err != nil {
		t.Errorf("RunBatch(the root alone) = %v, want it restored", err)
	}
	trashed := f.store.trash[f.a.ID]
	f.store.trashed[f.a.ID] = app.Content{NodeID: f.a.ID, Content: strings.Repeat("[[x]]", 10_001), Revision: 2}
	f.store.trashed[f.a1.ID] = app.Content{NodeID: f.a1.ID, Content: strings.Repeat("[[x]]", 10_000), Revision: 2}
	f.store.history = nil
	if _, err := w.RunBatch(f.asJob(), spec, plan, 1, 3); !errors.Is(err, domain.ErrBatchTooLarge) {
		t.Errorf("RunBatch(two pages past the links) = %v, want ErrBatchTooLarge", err)
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
