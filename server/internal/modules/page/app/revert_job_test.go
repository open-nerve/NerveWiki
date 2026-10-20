package app_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// jobScene is a notebook a test builds on the model of
// TestTheJobsStepsReplayToStar, each node of a label, then a batch on it,
// whose changeset the fake store holds as the store records it.
type jobScene struct {
	p      *jobTrial
	m, t0  jobModel
	e      *jobEdit
	labels map[uuid.UUID]string
}

func newJobScene() *jobScene {
	return &jobScene{p: newJobTrial(1), m: newJobModel(), labels: map[uuid.UUID]string{}}
}

// node makes a page, or an attachment, labelled label, named as it is,
// under parent: before the batch, or by it once begun.
func (s *jobScene) node(label string, parent *uuid.UUID, asset bool) uuid.UUID {
	name := strings.TrimSuffix(label, "'")
	var id uuid.UUID
	if s.e == nil {
		id = s.p.create(s.m, parent, jobNameOf(name, asset), asset)
	} else {
		var ok bool
		if id, ok = s.e.create(parent, name, asset); !ok {
			panic("the scene's creation of " + label)
		}
	}
	s.labels[id] = label
	return id
}

// begin starts the batch: the notebook so far is T0.
func (s *jobScene) begin() *jobEdit {
	s.t0 = s.m.clone()
	s.e = s.p.edit(s.m)
	return s.e
}

// load is a fixture holding the notebook after the batch, and the batch's
// changeset.
func (s *jobScene) load() (*fixture, *fakeChangeset) {
	post, items, versions := s.e.done(s.t0)
	return s.p.load(post, items, versions)
}

// describe is each step its kind and label, the subtree of a deletion,
// and + when the next step is joined to it.
func (s *jobScene) describe(steps []app.RevertStep, jobSteps []domain.JobStep) []string {
	var out []string
	for k, st := range steps {
		d := string(st.Kind) + " " + s.labels[st.Node]
		if st.Kind == domain.RevertDelete {
			var sub []string
			for _, id := range st.Nodes {
				sub = append(sub, s.labels[id])
			}
			d += " [" + strings.Join(sub, " ") + "]"
		}
		if jobSteps[k].Joined {
			d += "+"
		}
		out = append(out, d)
	}
	return out
}

// A background revert's job keeps its schedule's steps compact, in their
// order (M9/P3 design 3.22): each segment joined, no more; a page's write
// back right after its member's restore; a restore within a segment of
// steps aside cut to the members its later steps need, the rest of its
// group right after the segment, not joined; a deletion's subtree the one
// at its moment, without what a place moved out before it. Each seq is
// one step of the schedule's, a group's members sharing their restore's;
// a restore's step holds its member alone (no name, no states), a place's
// no nodes.
func TestARevertJobsStepsAreItsSegments(t *testing.T) {
	for _, tt := range []struct {
		name  string
		build func(s *jobScene)
		want  []string
	}{
		{"a page written, then its folder deleted: its write back right after its restore", func(s *jobScene) {
			f := s.node("F", nil, false)
			z := s.node("z", &f, false)
			s.node("w", &f, false)
			e := s.begin()
			e.write(z)
			e.del(f)
		}, []string{"restore F", "restore z+", "content z", "restore w"}},
		{"a step aside's segment, its restore cut to the members it needs", func(s *jobScene) {
			g := s.node("G", nil, false)
			k := s.node("K", &g, false)
			s.node("p1", &g, false)
			s.node("p2", &g, false)
			x, y := s.node("x", &k, false), s.node("y", &k, false)
			e := s.begin()
			e.move(x, nil, "x")
			e.move(y, nil, "y")
			e.del(g)
			g2 := s.node("G'", nil, false)
			e.move(y, &g2, "y")
		}, []string{"aside y+", "delete G' [G']+", "restore G+", "restore K+", "place x+", "place y", "restore p1", "restore p2"}},
		{"a deletion of a folder a place empties first: its subtree then", func(s *jobScene) {
			m := s.node("m", nil, false)
			e := s.begin()
			n := s.node("N", nil, false)
			e.move(m, &n, "m")
		}, []string{"place m", "delete N [N]"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newJobScene()
			tt.build(s)
			f, c := s.load()
			_, _, plan := f.revertJob(t, c.ID, nil, now())
			steps, _ := plan.RevertSteps()
			if got := s.describe(steps, plan.Steps()); !slices.Equal(got, tt.want) {
				t.Errorf("the steps %v, want %v", got, tt.want)
			}
			seqs := map[int]int{}
			for k, st := range plan.Steps() {
				s := steps[k]
				if s.Kind == domain.RevertDelete && st.Nodes != len(s.Nodes) || s.Kind != domain.RevertDelete && st.Nodes != 1 {
					t.Errorf("step %d (%s) of %d nodes, its subtree %d", k, s.Kind, st.Nodes, len(s.Nodes))
				}
				if s.Kind == domain.RevertRestore && (s.Name != "" || s.Nodes != nil) || s.Kind != domain.RevertDelete && s.Nodes != nil {
					t.Errorf("step %d (%s) holds %q, %v; want its node alone", k, s.Kind, s.Name, s.Nodes)
				}
				group := -1
				if s.Kind == domain.RevertRestore {
					group = s.Group
				}
				if g, seen := seqs[s.Seq]; seen && (g < 0 || g != group) {
					t.Errorf("step %d's seq %d is another step's", k, s.Seq)
				}
				seqs[s.Seq] = group
			}
		})
	}
}

// A batch restores its members of a group at once, at the group's first
// step in it (M9/P3 design 3.24, step 6), whatever steps come between them
// (review B-I2): a write back of the group's first member, or a segment of
// steps aside whose restore is cut short, the rest of the group after it;
// the members the batch leaves in the trash become trash items of their
// own, marked once; a later batch restores them as a continuation.
func TestARevertBatchRestoresItsGroupAtOnce(t *testing.T) {
	for _, tt := range []struct {
		name  string
		build func(s *jobScene)
		steps []string
		left  string // the member the first batch leaves
	}{
		{"[restore F, write F, restore c1, c2, c3]", func(s *jobScene) {
			f := s.node("F", nil, false)
			for _, c := range []string{"c1", "c2", "c3"} {
				s.node(c, &f, false)
			}
			e := s.begin()
			e.write(f)
			e.del(f)
		}, []string{"restore F+", "content F", "restore c1", "restore c2", "restore c3"}, "c3"},
		{"[aside y, delete G', restore G (cut), place x, place y, restore p1, p2]", func(s *jobScene) {
			g := s.node("G", nil, false)
			k := s.node("K", &g, false)
			s.node("p1", &g, false)
			s.node("p2", &g, false)
			x, y := s.node("x", &k, false), s.node("y", &k, false)
			e := s.begin()
			e.move(x, nil, "x")
			e.move(y, nil, "y")
			e.del(g)
			g2 := s.node("G'", nil, false)
			e.move(y, &g2, "y")
		}, []string{"aside y+", "delete G' [G']+", "restore G+", "restore K+", "place x+", "place y", "restore p1", "restore p2"}, "p2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newJobScene()
			tt.build(s)
			f, c := s.load()
			w, spec, plan := f.revertJob(t, c.ID, nil, now())
			steps, _ := plan.RevertSteps()
			if got := s.describe(steps, plan.Steps()); !slices.Equal(got, tt.steps) {
				t.Fatalf("the steps %v, want %v", got, tt.steps)
			}
			label := map[string]uuid.UUID{}
			for id, l := range s.labels {
				label[l] = id
			}
			n := len(steps)
			f.rec.calls = nil
			if _, err := w.RunBatch(f.asJob(), spec, plan, 0, n-1); err != nil {
				t.Fatalf("RunBatch(all but the last member) = %v", err)
			}
			var marks []string
			for _, call := range f.rec.calls {
				if strings.HasPrefix(call, "MarkTrashRoots") {
					marks = append(marks, call)
				}
			}
			left := label[tt.left]
			for l, id := range label {
				_, alive := f.store.nodes[id]
				if want := l != tt.left && l != "G'"; alive != want {
					t.Errorf("%s alive %v after the first batch, want %v", l, alive, want)
				}
			}
			if !f.store.trash[left].TrashRoot || !slices.Equal(marks, []string{"MarkTrashRoots 1 in tx"}) {
				t.Errorf("%s in the trash %+v, the marks %v; want it a trash item, marked once", tt.left, f.store.trash[left], marks)
			}
			if _, err := w.RunBatch(f.asJob(), spec, plan, n-1, n); err != nil {
				t.Fatalf("RunBatch(the last member) = %v", err)
			}
			_, deleted := label["G'"]
			if _, ok := f.store.nodes[left]; !ok || len(f.store.trash) != boolInt(deleted) {
				t.Errorf("after the job, %s alive %v, the trash %d nodes; want it back, the trash G' alone if any", tt.left, ok,
					len(f.store.trash))
			}
		})
	}
}

// A batch's steps of one node are never past a unit's bounds (M9/P3
// design 3.23): a page restored and written back, its history past the
// rows a unit restores, is one node, written; a second node with them is
// past it, ErrJobBatchTooLarge before a parse.
func TestARevertJobsLoneNodeIsNeverPastTheBounds(t *testing.T) {
	s := newJobScene()
	f := s.node("F", nil, false)
	c := s.node("c", &f, false)
	e := s.begin()
	e.write(f)
	e.del(f)
	fx, cs := s.load()
	fx.store.history = map[uuid.UUID]int{f: 11, c: 1}
	w, spec := fx.jobs(now(), 0, 10), fx.jobSpec()
	plan, err := w.PlanRevert(fx.asJob(), spec, cs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if steps, _ := plan.RevertSteps(); len(steps) != 3 || !plan.Steps()[0].Joined {
		t.Fatalf("the steps %+v; want F's restore joined to its write back, then c's", steps)
	}
	fx.rec.calls = nil
	if _, err := w.RunBatch(fx.asJob(), spec, plan, 0, 3); !errors.Is(err, domain.ErrJobBatchTooLarge) || fx.called("Facts") {
		t.Errorf("RunBatch(F and c, past the history) = %v; want ErrJobBatchTooLarge before a parse", err)
	}
	if done, err := w.RunBatch(fx.asJob(), spec, plan, 0, 2); err != nil || done.Restored != 1 || done.Written != 1 {
		t.Errorf("RunBatch(F restored and written back, its history past the bound) = %+v, %v; want both done", done, err)
	}
	if got := fx.store.contents[f]; got.Content != s.t0.nodes[f].content || got.Revision != 3 {
		t.Errorf("F's content %q at %d, want T0's at 3", got.Content, got.Revision)
	}
}
