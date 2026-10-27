package app_test

import (
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

// These are legal module history fixtures, not API histories: A's stored
// version is a system rewrite J->H, followed by deletion. Its undo restores
// H then writes J. B holds J, or H in the opposite-order case. All hashes,
// versions, trash groups and byte sizes come from the existing job model.
type sourceDuplicateJob struct {
	scene           *jobScene
	f               *fixture
	changeset       *fakeChangeset
	spy             *guard
	raw, root, a, b uuid.UUID
}

func newSourceDuplicateJob(t *testing.T, opposite, independent bool) sourceDuplicateJob {
	t.Helper()
	s := newJobScene()
	raw := s.node("raw", nil, false)
	s.content(raw, "")
	root, parent := uuid.UUID{}, raw
	if !independent {
		root = s.node("F", &raw, false)
		s.content(root, "")
		parent = root
	}
	a, b := s.node("A", &parent, false), s.node("B", &parent, false)
	s.content(a, "J")
	if opposite {
		s.content(b, "H")
	} else {
		s.content(b, "J")
	}
	e := s.begin()
	e.write(a)
	s.content(a, "H")
	if independent {
		if !e.del(a) || !e.del(b) {
			t.Fatal("fixture must delete the two independent roots")
		}
	} else if !e.del(root) {
		t.Fatal("fixture must delete F as one connected group")
	}
	f, c := s.load()
	if len(c.versions) != 1 || c.versions[0].NodeID != a || c.versions[0].Base == nil || *c.versions[0].Base != 1 || c.versions[0].Revision != 2 {
		t.Fatalf("fixture rewrite history=%+v, want A revision 1->2", c.versions)
	}
	c.versions[0].Rewrite = true
	spy := sourceJobFixture(f, raw)
	checkFakeSourceRebuild(t, f, 0)
	return sourceDuplicateJob{scene: s, f: f, changeset: c, spy: spy, raw: raw, root: root, a: a, b: b}
}

func sourceDuplicateJobCuts(t *testing.T, cuts [][2]int, independent bool) app.JobBatches {
	t.Helper()
	return func(steps []domain.JobStep) [][2]int {
		want := []bool{false, true, false, false}
		if independent {
			want = []bool{true, false, false}
		}
		if len(steps) != len(want) {
			t.Fatalf("steps=%v, want %d compact steps", steps, len(want))
		}
		for i, step := range steps {
			if step.Joined != want[i] {
				t.Fatalf("step %d Joined=%t, want %t", i, step.Joined, want[i])
			}
		}
		return slices.Clone(cuts)
	}
}

func assertSourceJobDuplicate(t *testing.T, err error, duplicate *uuid.UUID) {
	t.Helper()
	var problem *shared.Error
	if !errors.As(err, &problem) || problem.Code != "page.source_duplicate" {
		t.Fatalf("error=%v, want source_duplicate", err)
	}
	if problem.DuplicateOf == nil && duplicate != nil || problem.DuplicateOf != nil && (duplicate == nil || *problem.DuplicateOf != *duplicate) {
		t.Fatalf("duplicate target=%v, want %v", problem.DuplicateOf, duplicate)
	}
}

func assertSourceJobCompact(t *testing.T, x sourceDuplicateJob, plan app.JobPlan) {
	t.Helper()
	steps, _ := plan.RevertSteps()
	want := []string{"restore F", "restore A+", "content A", "restore B"}
	if got := x.scene.describe(steps, plan.Steps()); !slices.Equal(got, want) {
		t.Fatalf("compact=%v, want handwritten order %v", got, want)
	}
}

func assertSourceJobBodies(t *testing.T, x sourceDuplicateJob, a, b string) {
	t.Helper()
	count := 0
	for _, pair := range []struct {
		id   uuid.UUID
		body string
	}{{x.a, a}, {x.b, b}} {
		_, alive := x.f.store.nodes[pair.id]
		if pair.body == "" {
			if _, trashed := x.f.store.trash[pair.id]; alive || !trashed {
				t.Errorf("%s alive=%t trash=%t, want still in trash", x.scene.labels[pair.id], alive, trashed)
			}
			continue
		}
		count++
		if !alive || x.f.store.contents[pair.id].Content != pair.body {
			t.Errorf("%s alive=%t body=%q, want %q", x.scene.labels[pair.id], alive, x.f.store.contents[pair.id].Content, pair.body)
		}
	}
	checkFakeSourceRebuild(t, x.f, count)
}

func TestSourceJobPrechecksKeepOppositeCutOutcomes(t *testing.T) {
	for _, opposite := range []bool{false, true} {
		for _, split := range []bool{false, true} {
			for _, route := range []string{"check", "plan"} {
				t.Run(fmt.Sprintf("opposite_%t/split_%t/%s", opposite, split, route), func(t *testing.T) {
					x := newSourceDuplicateJob(t, opposite, false)
					cuts := [][2]int{{0, 4}}
					if split {
						cuts = [][2]int{{0, 3}, {3, 4}}
					}
					batches := sourceDuplicateJobCuts(t, cuts, false)
					w := x.f.jobs(now(), 0, 0)
					var err error
					if route == "check" {
						_, err = w.CheckRevert(x.f.asAlice(), x.f.eng, x.changeset.ID, nil, nil, batches)
					} else {
						var plan app.JobPlan
						plan, err = w.PlanRevert(x.f.asJob(), x.f.jobSpec(), x.changeset.ID, batches)
						if err == nil {
							assertSourceJobCompact(t, x, plan)
						}
					}
					if split != opposite {
						// A was trash at the snapshot's start. A simulated earlier
						// nominal batch is not an actual committed baseline source.
						assertSourceJobDuplicate(t, err, nil)
					} else if err != nil {
						t.Fatal(err)
					}
					if len(x.f.store.changesets) != 0 || x.f.called("Facts") {
						t.Fatalf("precheck parsed/wrote: %v", x.f.rec.calls)
					}
					assertSourceJobBodies(t, x, "", "")
				})
			}
		}
	}
}

func TestSourceJobBatchesUseActualGroupAdmissionOrder(t *testing.T) {
	for _, opposite := range []bool{false, true} {
		for _, split := range []bool{false, true} {
			t.Run(fmt.Sprintf("opposite_%t/split_%t", opposite, split), func(t *testing.T) {
				x := newSourceDuplicateJob(t, opposite, false)
				w, spec := x.f.jobs(now(), 0, 0), x.f.jobSpec()
				allowed := [][2]int{{0, 4}}
				if opposite {
					allowed = [][2]int{{0, 3}, {3, 4}}
				}
				plan, err := w.PlanRevert(x.f.asJob(), spec, x.changeset.ID, sourceDuplicateJobCuts(t, allowed, false))
				if err != nil {
					t.Fatal(err)
				}
				assertSourceJobCompact(t, x, plan)
				// The opposite large cut is a direct module guard comparison,
				// not an API path around its rejected nominal start.
				if split {
					first, err := w.RunBatch(x.f.asJob(), spec, plan, 0, 3)
					if err != nil || first.Restored != 2 || first.Written != 1 {
						t.Fatalf("prefix=%+v error=%v, want F/A restored then A written J", first, err)
					}
					assertSourceJobBodies(t, x, "J", "")
					_, err = w.RunBatch(x.f.asJob(), spec, plan, 3, 4)
					if !opposite {
						assertSourceJobDuplicate(t, err, &x.a)
						assertSourceJobBodies(t, x, "J", "")
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					assertSourceJobBodies(t, x, "J", "H")
				} else {
					done, err := w.RunBatch(x.f.asJob(), spec, plan, 0, 4)
					if opposite {
						assertSourceJobDuplicate(t, err, nil)
						assertSourceJobBodies(t, x, "", "")
						return
					}
					if err != nil || done.Restored != 3 || done.Written != 1 {
						t.Fatalf("large batch=%+v error=%v, want all restored before rewrite", done, err)
					}
					assertSourceJobBodies(t, x, "J", "J")
					var trace []string
					for _, step := range x.spy.steps {
						line := string(step.Operation)
						for _, change := range step.Changes {
							line += " " + x.scene.labels[change.NodeID]
						}
						trace = append(trace, line)
					}
					if want := []string{"restore F A B", "content A"}; !slices.Equal(trace, want) {
						t.Fatalf("actual guarded actions=%v, want %v", trace, want)
					}
				}
			})
		}
	}
}

// The literal cuts are the existing worker halver's segment boundaries.
// This test invokes real page bounds and writes; it does not claim to run
// transfer's orchestration. No ErrJobBatchTooLarge is injected.
func TestSourceJobHistoryLimitExposesHalvedDuplicate(t *testing.T) {
	x := newSourceDuplicateJob(t, false, false)
	// F/B each retain revision 1 and their deletion item; A retains
	// revisions 1/2 and its deletion item. These are row counts supplied
	// by the fake TrashHistory repository, not a stubbed size-limit error.
	x.f.store.history = map[uuid.UUID]int{x.root: 2, x.a: 3, x.b: 2}
	w, spec := x.f.jobs(now(), 0, 4), x.f.jobSpec()
	plan, err := w.PlanRevert(x.f.asJob(), spec, x.changeset.ID, sourceDuplicateJobCuts(t, [][2]int{{0, 4}}, false))
	if err != nil {
		t.Fatal(err)
	}
	assertSourceJobCompact(t, x, plan)
	x.f.rec.calls = nil
	if _, err := w.RunBatch(x.f.asJob(), spec, plan, 0, 4); !errors.Is(err, domain.ErrJobBatchTooLarge) || !strings.Contains(err.Error(), "history") {
		t.Fatalf("first large batch=%v, want real history limit", err)
	}
	if x.f.called("Facts") || len(x.f.store.changesets) != 0 {
		t.Fatalf("history rejection parsed/wrote: %v", x.f.rec.calls)
	}
	assertSourceJobBodies(t, x, "", "")
	if done, err := w.RunBatch(x.f.asJob(), spec, plan, 0, 1); err != nil || done.Restored != 1 {
		t.Fatalf("first halved prefix=%+v, %v", done, err)
	}
	if _, alive := x.f.store.nodes[x.root]; !alive {
		t.Fatal("F prefix was not committed")
	}
	x.f.rec.calls = nil
	if _, err := w.RunBatch(x.f.asJob(), spec, plan, 1, 4); !errors.Is(err, domain.ErrJobBatchTooLarge) || !strings.Contains(err.Error(), "history") {
		t.Fatalf("new-from remainder=%v, want real history limit", err)
	}
	if x.f.called("Facts") {
		t.Fatalf("second history rejection parsed: %v", x.f.rec.calls)
	}
	if done, err := w.RunBatch(x.f.asJob(), spec, plan, 1, 3); err != nil || done.Restored != 1 || done.Written != 1 {
		t.Fatalf("joined A prefix=%+v, %v", done, err)
	}
	assertSourceJobBodies(t, x, "J", "")
	_, err = w.RunBatch(x.f.asJob(), spec, plan, 3, 4)
	assertSourceJobDuplicate(t, err, &x.a)
	assertSourceJobBodies(t, x, "J", "")
}

func TestSourceJobBusyRetriesTheSameCut(t *testing.T) {
	x := newSourceDuplicateJob(t, false, false)
	w, spec := x.f.jobs(now(), 0, 0), x.f.jobSpec()
	plan, err := w.PlanRevert(x.f.asJob(), spec, x.changeset.ID, sourceDuplicateJobCuts(t, [][2]int{{0, 4}}, false))
	if err != nil {
		t.Fatal(err)
	}
	x.f.budget.err = shared.ServerBusy(time.Second)
	_, err = w.RunBatch(x.f.asJob(), spec, plan, 0, 4)
	var problem *shared.Error
	if !errors.As(err, &problem) || problem.Kind != shared.KindUnavailable {
		t.Fatalf("parse budget refusal=%v, want 503", err)
	}
	if len(x.f.store.changesets) != 0 || x.f.budget.held != 0 {
		t.Fatal("busy attempt changed persistent state or retained parser budget")
	}
	assertSourceJobBodies(t, x, "", "")
	x.f.budget.err = nil
	done, err := w.RunBatch(x.f.asJob(), spec, plan, 0, 4)
	if err != nil || done.Restored != 3 || done.Written != 1 {
		t.Fatalf("same-cut retry=%+v, %v", done, err)
	}
	assertSourceJobBodies(t, x, "J", "J")
	if x.f.budget.held != 0 {
		t.Fatalf("parser budget retained=%d", x.f.budget.held)
	}
}

func TestSourceJobIndependentGroupsDoNotCoalesceForDuplicates(t *testing.T) {
	for _, cuts := range [][][2]int{{{0, 3}}, {{0, 2}, {2, 3}}} {
		t.Run(fmt.Sprint(cuts), func(t *testing.T) {
			for _, route := range []string{"check", "plan"} {
				x := newSourceDuplicateJob(t, false, true)
				w := x.f.jobs(now(), 0, 0)
				batches := sourceDuplicateJobCuts(t, cuts, true)
				var err error
				if route == "check" {
					_, err = w.CheckRevert(x.f.asAlice(), x.f.eng, x.changeset.ID, nil, nil, batches)
				} else {
					_, err = w.PlanRevert(x.f.asJob(), x.f.jobSpec(), x.changeset.ID, batches)
				}
				assertSourceJobDuplicate(t, err, nil)
				assertSourceJobBodies(t, x, "", "")
			}
		})
	}
	// The synchronous plan restores both independent groups before the
	// write-back; this is module evidence, distinct from root's API A-I1.
	x := newSourceDuplicateJob(t, false, true)
	if _, err := x.f.revert(x.changeset.ID, nil, 0); err != nil {
		t.Fatal(err)
	}
	assertSourceJobBodies(t, x, "J", "J")
}

// A deletion is the compact plan's only step whose Nodes can exceed one.
// N and its eight children were really created by the reverted changeset;
// no weight, compact step, or Joined flag is patched into the plan.
func newWeightedSourceDuplicateJob(t *testing.T) (sourceDuplicateJob, []uuid.UUID, []uuid.UUID) {
	t.Helper()
	s := newJobScene()
	raw := s.node("raw", nil, false)
	s.content(raw, "")
	root := s.node("F", &raw, false)
	s.content(root, "")
	a, b := s.node("A", &root, false), s.node("B", &root, false)
	s.content(a, "J")
	s.content(b, "J")
	var tail []uuid.UUID
	for _, label := range []string{"C", "D", "E", "G"} {
		id := s.node(label, &root, false)
		s.content(id, "")
		tail = append(tail, id)
	}
	e := s.begin()
	e.write(a)
	s.content(a, "H")
	if !e.del(root) {
		t.Fatal("fixture must delete F as one connected group")
	}
	n := s.node("N", nil, false)
	s.content(n, "")
	created := []uuid.UUID{n}
	for i := 1; i <= 8; i++ {
		id := s.node(fmt.Sprintf("n%d", i), &n, false)
		s.content(id, "")
		created = append(created, id)
	}
	f, c := s.load()
	rewrites := 0
	for i := range c.versions {
		v := &c.versions[i]
		if v.NodeID == a {
			if v.Base == nil || *v.Base != 1 || v.Revision != 2 {
				t.Fatalf("fixture rewrite=%+v, want A revision 1->2", v)
			}
			v.Rewrite = true
			rewrites++
		}
	}
	if rewrites != 1 || len(c.versions) != 10 {
		t.Fatalf("versions=%+v, want one rewrite and nine created pages", c.versions)
	}
	spy := sourceJobFixture(f, raw)
	// A retains revisions 1/2 plus its deletion item; the other six
	// members retain revision 1 plus their deletion item. No size error
	// is injected: JobWrites sums these repository row counts itself.
	f.store.history = map[uuid.UUID]int{root: 2, a: 3, b: 2}
	for _, id := range tail {
		f.store.history[id] = 2
	}
	checkFakeSourceRebuild(t, f, 0)
	return sourceDuplicateJob{scene: s, f: f, changeset: c, spy: spy, raw: raw, root: root, a: a, b: b}, created, tail
}

// These literal paths distinguish index halving from halving by cumulative
// node weights. The companion transfer test invokes the real worker halver;
// this test invokes real page history bounds and duplicate guards, without
// claiming that its handwritten cuts execute transfer's scheduler.
func TestSourceJobWeightedHalvingChangesDuplicateOutcome(t *testing.T) {
	for _, weighted := range []bool{false, true} {
		t.Run(map[bool]string{false: "step_index", true: "node_weight"}[weighted], func(t *testing.T) {
			x, created, tail := newWeightedSourceDuplicateJob(t)
			w, spec := x.f.jobs(now(), 0, 7), x.f.jobSpec()
			batches := func(steps []domain.JobStep) [][2]int {
				want := []domain.JobStep{{Nodes: 9}, {Nodes: 1}, {Nodes: 1, Bytes: 1, Joined: true},
					{Nodes: 1, Bytes: 1}, {Nodes: 1, Bytes: 1}, {Nodes: 1}, {Nodes: 1}, {Nodes: 1}, {Nodes: 1}}
				if !slices.Equal(steps, want) {
					t.Fatalf("weighted compact sizes=%+v, want %+v", steps, want)
				}
				return [][2]int{{0, 9}}
			}
			if _, err := w.CheckRevert(x.f.asAlice(), x.f.eng, x.changeset.ID, nil, nil, batches); err != nil {
				t.Fatalf("nominal admission: %v", err)
			}
			plan, err := w.PlanRevert(x.f.asJob(), spec, x.changeset.ID, batches)
			if err != nil {
				t.Fatal(err)
			}
			steps, _ := plan.RevertSteps()
			wantCompact := []string{"delete N [N n1 n2 n3 n4 n5 n6 n7 n8]", "restore F", "restore A+", "content A",
				"restore B", "restore C", "restore D", "restore E", "restore G"}
			if got := x.scene.describe(steps, plan.Steps()); !slices.Equal(got, wantCompact) {
				t.Fatalf("weighted compact=%v, want %v", got, wantCompact)
			}
			if len(x.spy.steps) != 0 || len(x.f.store.changesets) != 0 || x.f.called("Facts") {
				t.Fatal("nominal admission must not parse, guard or write")
			}
			type attempt struct {
				from, to int
				history  bool
				refused  bool
				deleted  int
				restored int
				written  int
			}
			attempts := []attempt{{from: 0, to: 9, history: true}, {from: 0, to: 4, deleted: 9, restored: 2, written: 1},
				{from: 4, to: 9, history: true}, {from: 4, to: 6, refused: true}}
			if weighted {
				attempts = []attempt{{from: 0, to: 9, history: true}, {from: 0, to: 1, deleted: 9},
					{from: 1, to: 9, history: true}, {from: 1, to: 5, restored: 3, written: 1},
					{from: 5, to: 9, history: true}, {from: 5, to: 7, restored: 2}, {from: 7, to: 9, restored: 2}}
			}
			live, removed := map[uuid.UUID]bool{}, false
			for _, cut := range attempts {
				if cut.from > 0 && plan.Steps()[cut.from-1].Joined || cut.to < len(steps) && plan.Steps()[cut.to-1].Joined {
					t.Fatalf("handwritten cut [%d,%d) splits Joined", cut.from, cut.to)
				}
				x.f.rec.calls = nil
				guarded := len(x.spy.steps)
				done, err := w.RunBatch(x.f.asJob(), spec, plan, cut.from, cut.to)
				switch {
				case cut.history:
					if !errors.Is(err, domain.ErrJobBatchTooLarge) || !strings.Contains(err.Error(), "history") {
						t.Fatalf("[%d,%d)=%+v, %v; want real history refusal", cut.from, cut.to, done, err)
					}
					if x.f.called("Facts") || len(x.spy.steps) != guarded {
						t.Fatalf("history refusal parsed/guarded: %v", x.f.rec.calls)
					}
				case cut.refused:
					assertSourceJobDuplicate(t, err, &x.a)
					if len(x.spy.steps) != guarded {
						t.Fatal("duplicate B/C restore passed the source guard")
					}
				default:
					if err != nil || done.Deleted != cut.deleted || done.Restored != cut.restored || done.Written != cut.written ||
						done.Nodes != cut.deleted+cut.restored+cut.written {
						t.Fatalf("[%d,%d)=%+v, %v; want deleted/restored/written %d/%d/%d", cut.from, cut.to,
							done, err, cut.deleted, cut.restored, cut.written)
					}
					removed = removed || cut.deleted > 0
					for i, id := range []uuid.UUID{x.root, x.a, x.b, tail[0], tail[1], tail[2], tail[3]} {
						at := []int{1, 2, 4, 5, 6, 7, 8}[i]
						if cut.from <= at && at < cut.to {
							live[id] = true
						}
					}
				}
				for _, id := range append([]uuid.UUID{x.root, x.a, x.b}, tail...) {
					_, alive := x.f.store.nodes[id]
					_, trash := x.f.store.trash[id]
					if alive != live[id] || trash == live[id] {
						t.Fatalf("after [%d,%d), %s alive/trash=%t/%t, want alive=%t", cut.from, cut.to,
							x.scene.labels[id], alive, trash, live[id])
					}
				}
				for _, id := range created {
					_, alive := x.f.store.nodes[id]
					_, trash := x.f.store.trash[id]
					if alive == removed || trash != removed {
						t.Fatalf("created %s alive/trash=%t/%t, want removed=%t", x.scene.labels[id], alive, trash, removed)
					}
				}
				a, b := "", ""
				if live[x.a] {
					a = "J"
				}
				if live[x.b] {
					b = "J"
				}
				assertSourceJobBodies(t, x, a, b)
			}
			if weighted {
				if diff := x.scene.p.compare(x.scene.t0, x.scene.p.modelOf(x.f)); diff != "" {
					t.Fatalf("weighted alternative did not finish the revert: %s", diff)
				}
			}
		})
	}
}
