package app_test

import (
	"crypto/sha256"
	"errors"
	"fmt"
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

// order puts the node id at order among its siblings.
func (s *jobScene) order(id uuid.UUID, order float64) {
	n := s.m.nodes[id]
	n.order = order
	s.m.nodes[id] = n
}

// content gives the page id content, its revision's.
func (s *jobScene) content(id uuid.UUID, content string) {
	n := s.m.nodes[id]
	n.content = content
	s.m.nodes[id] = n
	s.p.version(id, n.revision, content)
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
		{"a step aside's segment, its restore cut short of a page written after K (review A-I1)", func(s *jobScene) {
			g := s.node("G", nil, false)
			k := s.node("K", &g, false)
			s.node("p1", &g, false)
			p2 := s.node("p2", &g, false)
			x, y := s.node("x", &k, false), s.node("y", &k, false)
			e := s.begin()
			e.move(x, nil, "x")
			e.move(y, nil, "y")
			e.write(p2)
			e.del(g)
			g2 := s.node("G'", nil, false)
			e.move(y, &g2, "y")
		}, []string{"aside y+", "delete G' [G']+", "restore G+", "restore K+", "place x+", "place y", "restore p1", "restore p2+",
			"content p2"}},
		{"a deletion of a folder a place empties first: its subtree then", func(s *jobScene) {
			m := s.node("m", nil, false)
			e := s.begin()
			n := s.node("N", nil, false)
			e.move(m, &n, "m")
		}, []string{"place m", "delete N [N]"}},
		{"a deletion's subtree a level at a time, each level by order, then id (review C-M1)", func(s *jobScene) {
			s.begin()
			n := s.node("N", nil, false)
			a := s.node("a", &n, false)
			s.node("x", &a, false)
			b := s.node("b", &n, false)
			s.node("y", &b, false)
			s.order(b, -1)
		}, []string{"delete N [N b a x y]"}},
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

// transferJob runs a job's plan in batches as transfer cuts them (M9/P3
// design 3.23): each grown a segment at a time within nodes, the first
// always in; past a unit's bounds, it ends at the last segment's end at
// most half way, or the first segment's; a first segment past them stops
// the job. It answers the batches written and what stopped the job.
func transferJob(f *fixture, w *app.JobWrites, spec app.JobSpec, plan app.JobPlan, nodes int) (int, error) {
	steps := plan.Steps()
	end := func(at int) int {
		for at < len(steps)-1 && steps[at].Joined {
			at++
		}
		return at + 1
	}
	weigh := func(from, to int) int {
		n := 0
		for _, s := range steps[from:to] {
			n += s.Nodes
		}
		return n
	}
	batches := 0
	for from := 0; from < len(steps); batches++ {
		first := end(from)
		to, n := first, weigh(from, first)
		for to < len(steps) && n+weigh(to, end(to)) <= nodes {
			to, n = end(to), n+weigh(to, end(to))
		}
		for {
			_, err := w.RunBatch(f.asJob(), spec, plan, from, to)
			if errors.Is(err, domain.ErrJobBatchTooLarge) && to > first {
				half, at := from+(to-from)/2, first
				for k := first; k <= half && k < to; k = end(k) {
					at = k
				}
				to = at
				continue
			}
			if err != nil {
				return batches, fmt.Errorf("batch [%d, %d): %w", from, to, err)
			}
			break
		}
		from = to
	}
	return batches, nil
}

// A page of a group written back leaves a restore within a segment of
// steps aside cut short (M9/P3 design 3.22.2, review A-I1): G holds K,
// first, then 101 pages, the last written; x and y under K. A batch moves
// x and y out, writes the last page, deletes G, makes a new G and moves y
// into it. The segment restores G and K alone, as a place under K needs
// them, the rest after it, the last page's write back right after its
// restore: transfer's batches of 100 nodes write it, and the job ends at
// S*. Were the written page needed, the segment would restore all 103
// pages, past a unit's, and stop the job.
func TestARevertJobsWrittenPageLeavesTheCut(t *testing.T) {
	s := newJobScene()
	g := s.node("G", nil, false)
	k := s.node("K", &g, false)
	var last uuid.UUID
	for i := range 101 {
		last = s.node(fmt.Sprintf("p%03d", i), &g, false)
	}
	x, y := s.node("x", &k, false), s.node("y", &k, false)
	e := s.begin()
	e.move(x, nil, "x")
	e.move(y, nil, "y")
	e.write(last)
	e.del(g)
	g2 := s.node("G'", nil, false)
	e.move(y, &g2, "y")
	f, c := s.load()
	w, spec, plan := f.revertJob(t, c.ID, nil, now())
	steps, _ := plan.RevertSteps()
	got := s.describe(steps, plan.Steps())
	want := []string{"aside y+", "delete G' [G']+", "restore G+", "restore K+", "place x+", "place y"}
	if !slices.Equal(got[:min(len(got), len(want))], want) || !slices.Equal(got[len(got)-2:], []string{"restore p100+", "content p100"}) {
		t.Errorf("the steps %v; want the segment %v, G's other pages after it, p100's write back last", got, want)
	}
	if batches, err := transferJob(f, w, spec, plan, 100); err != nil {
		t.Fatalf("the job, after %d batches: %v", batches, err)
	}
	if bad := s.p.compare(s.t0, s.p.modelOf(f)); bad != "" {
		t.Errorf("the store after the job: %s", bad)
	}
}

// A write back joins the deletion that takes its page alone, right before
// it (M9/P3 design 3.22.2, review A-M1): F holds y and X; h named N, h2
// named h1, h before h2 by id. A batch renames h2 h2x and h h1, makes a
// folder named N of 22 pages of 1,000 links each, writes X, moves y into
// the new N and deletes F. Its revert's revert puts h2 and h back, which
// a round apart frees N for its restore, restores N, puts y back, then
// writes X back and deletes F: X's write back right before the deletion,
// [content X, delete F] the segment, N's 22,000 links outside it, so
// transfer's batches cut its restore and the job ends at S*. Were the
// steps between the write and the deletion joined, N's restore would be
// held in one batch, past a unit's links, and stop the job.
func TestARevertJobsWriteBackJoinsItsDeletionAlone(t *testing.T) {
	s := newJobScene()
	fo := s.node("F", nil, false)
	y := s.node("y", &fo, false)
	x := s.node("X", &fo, false)
	h := s.node("N", nil, false)
	h2 := s.node("h1", nil, false)
	e := s.begin()
	e.move(h2, nil, "h2x")
	e.move(h, nil, "h1")
	n := s.node("N'", nil, false)
	heavy := strings.Repeat("[[a]] ", 1000)
	for i := range 22 {
		q := s.node(fmt.Sprintf("q%02d", i), &n, false)
		nd := s.m.nodes[q]
		nd.content = heavy + q.String()
		s.m.nodes[q] = nd
		s.p.version(q, nd.revision, nd.content)
	}
	e.write(x)
	e.move(y, &n, "y")
	if !e.del(fo) {
		t.Fatal("the batch's deletion of F")
	}
	f, c := s.load()
	star := s.p.modelOf(f) // the notebook as the batch left it, the revert's revert's S*
	w, spec, plan := f.revertJob(t, c.ID, nil, now())
	if batches, err := transferJob(f, w, spec, plan, 100); err != nil {
		t.Fatalf("the revert, after %d batches: %v", batches, err)
	}
	w, spec, plan = f.revertJob(t, f.recorded(spec.Changeset, c.ID).ID, nil, now())
	steps, _ := plan.RevertSteps()
	got := s.describe(steps, plan.Steps())
	joined := 0
	for _, st := range plan.Steps() {
		if st.Joined {
			joined++
		}
	}
	if !slices.Equal(got[len(got)-2:], []string{"content X+", "delete F [F X]"}) || joined != 1 {
		t.Errorf("the revert's revert's steps %v; want X's write back joined to F's deletion alone, right before it", got)
	}
	if batches, err := transferJob(f, w, spec, plan, 100); err != nil {
		t.Fatalf("the revert's revert, after %d batches: %v", batches, err)
	}
	if bad := s.p.compare(star, s.p.modelOf(f)); bad != "" {
		t.Errorf("the store after the revert's revert: %s", bad)
	}
}

// A batch of more than one node restores at most the nodes a unit does
// (M9/P3 design 3.23, review B-S1, C-S2): G holds attachments, then K, x
// and y under K; a batch moves x and y out, deletes G, makes a new G and
// moves y into it. The segment's restore of G goes as far as K, carrying
// every attachment: G, them and K MaxUnitNodes nodes, the job ends at S*;
// one more, its first segment is past the nodes a unit restores, before
// anything is read (review B's N-1, C1-N2), and the job stops there. No
// batch's changeset comes to it, a batch changing at most MaxUnitNodes
// nodes in all: this one is the test's own.
func TestARevertJobsBatchRestoresAtMostAUnitsNodes(t *testing.T) {
	for _, past := range []bool{false, true} {
		t.Run(fmt.Sprint("past ", past), func(t *testing.T) {
			s := newJobScene()
			g := s.node("G", nil, false)
			for i := range domain.MaxUnitNodes - 2 + boolInt(past) {
				s.p.create(s.m, &g, fmt.Sprintf("a%05d.bin", i), true)
			}
			k := s.node("K", &g, false)
			x, y := s.node("x", &k, false), s.node("y", &k, false)
			e := s.begin()
			e.move(x, nil, "x")
			e.move(y, nil, "y")
			e.del(g)
			g2 := s.node("G'", nil, false)
			e.move(y, &g2, "y")
			f, c := s.load()
			w, spec, plan := f.revertJob(t, c.ID, nil, now())
			f.rec.calls = nil
			_, err := transferJob(f, w, spec, plan, 100)
			switch {
			case past && (!errors.Is(err, domain.ErrJobBatchTooLarge) || !strings.Contains(err.Error(), "past the nodes a unit restores") ||
				len(f.rec.calls) > 0):
				t.Errorf("the job = %v, after %v; want its first segment past the nodes a unit restores, before a read", err, f.rec.calls)
			case !past && err != nil:
				t.Errorf("the job = %v; want it done", err)
			case !past:
				if bad := s.p.compare(s.t0, s.p.modelOf(f)); bad != "" {
					t.Errorf("the store after the job: %s", bad)
				}
			}
		})
	}
}

// A batch says which of a unit's bounds it is past (M9/P3 design 3.23,
// review A-M2), by the sizes its plan measured, before it reads anything
// (review A1-I1, B's I-1): a segment that restores G, its pages and K, the
// counterexample's shape of no links and no history, is within a unit's
// 100 pages at 98 pages, and past them at 99; at 3 pages of 3 MiB, the
// first written by the batch, within them, and past a unit's 8 MiB by its
// write back's base, 3 MiB more. A page restored and written back is two
// copies (design 3.18.4): 49 pages each written by the batch are 100
// copies with G and K, within a unit's, and 50 are 102, past them (review
// A2-N2). The segment's sizes are its own when a first batch reverts O's
// renames before it: 3 pages of 3 MiB, past by their bytes alone. A
// segment past them makes no call at all (review B2-N-1). A content the
// trash holds larger than the plan measured, its pages grown to 3 MiB
// since, is past them as read, before a unit parses it, and the bytes read
// within a unit's: the content past them is not read (M9 Codex review D2).
// The bytes read of the trash are a unit's less those of the bases the
// batch writes back: the first page's base of 3 MiB, the three grown to 2
// MiB, 6 MiB, past the 5 MiB the base leaves, not read past them (fix
// round 1, B-M2).
func TestARevertJobsSegmentPastAUnitsPagesSaysSo(t *testing.T) {
	for _, tt := range []struct {
		name    string
		pages   int
		size    int // a page's bytes, 0 for the scene's own
		written int // the pages, from the first, the batch writes before it deletes G
		renamed int // the pages O the batch renames, whose places a first batch reverts
		grown   int // the bytes each page's content in the trash grows to after the plan
		base    int // the bytes of the written pages' contents before the batch, the scene's own when 0
		past    bool
	}{
		{"98 pages", 98, 0, 0, 0, 0, 0, false},
		{"99 pages", 99, 0, 0, 0, 0, 0, true},
		{"3 pages of 3 MiB, the first written back", 3, 3 << 20, 1, 0, 0, 0, true},
		{"49 pages restored and written back", 49, 0, 49, 0, 0, 0, false},
		{"50 pages restored and written back", 50, 0, 50, 0, 0, 0, true},
		{"3 pages of 3 MiB after a first batch", 3, 3 << 20, 0, 4, 0, 0, true},
		{"3 pages grown to 3 MiB in the trash", 3, 0, 0, 0, 3 << 20, 0, true},
		{"3 pages grown to 2 MiB, the first's base 3 MiB", 3, 0, 1, 0, 2 << 20, 3 << 20, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newJobScene()
			var olds []uuid.UUID
			for i := range tt.renamed {
				olds = append(olds, s.node(fmt.Sprintf("O%d", i), nil, false))
			}
			g := s.node("G", nil, false)
			var ps []uuid.UUID
			for i := range tt.pages {
				ps = append(ps, s.node(fmt.Sprintf("p%03d", i), &g, false))
				if tt.size > 0 {
					s.content(ps[i], strings.Repeat("a", tt.size))
				}
				if tt.base > 0 && i < tt.written {
					s.content(ps[i], strings.Repeat("b", tt.base))
				}
			}
			k := s.node("K", &g, false)
			x, y := s.node("x", &k, false), s.node("y", &k, false)
			e := s.begin()
			for i, o := range olds {
				e.move(o, nil, fmt.Sprintf("O%d renamed", i))
			}
			e.move(x, nil, "x")
			e.move(y, nil, "y")
			for _, pg := range ps[:tt.written] {
				e.write(pg)
			}
			e.del(g)
			g2 := s.node("G'", nil, false)
			e.move(y, &g2, "y")
			f, c := s.load()
			w, spec, plan := f.revertJob(t, c.ID, nil, now())
			from := 0
			if tt.renamed > 0 {
				steps, _ := plan.RevertSteps()
				from = slices.IndexFunc(steps, func(s app.RevertStep) bool { return s.Kind != domain.RevertPlace || !slices.Contains(olds, s.Node) })
				if from != tt.renamed {
					t.Fatalf("the steps %v; want O's places first", s.describe(steps, plan.Steps()))
				}
				if _, err := w.RunBatch(f.asJob(), spec, plan, 0, from); err != nil {
					t.Fatalf("RunBatch(O's places) = %v", err)
				}
			}
			for _, pg := range ps {
				if tt.grown > 0 {
					trashed := f.store.trashed[pg]
					trashed.Content = strings.Repeat("a", tt.grown)
					sum := sha256.Sum256([]byte(trashed.Content))
					trashed.Hash = sum[:]
					f.store.trashed[pg] = trashed
				}
			}
			to := from + 1
			for to < len(plan.Steps()) && plan.Steps()[to-1].Joined {
				to++
			}
			f.rec.calls = nil
			_, err := w.RunBatch(f.asJob(), spec, plan, from, to)
			past := errors.Is(err, domain.ErrJobBatchTooLarge) && strings.Contains(err.Error(), "past a unit's pages or their bytes")
			switch {
			case past != tt.past || !tt.past && err != nil:
				t.Errorf("RunBatch(the segment, steps %d to %d) = %v; want it past a unit's pages %v", from, to, err, tt.past)
			case tt.past && tt.grown == 0 && len(f.rec.calls) > 0:
				t.Errorf("RunBatch(the segment) past a unit's pages after %v; want it before any call", f.rec.calls)
			case tt.grown > 0 && (!f.called("TrashContentsWithin") || f.called("Facts")):
				t.Errorf("RunBatch(the segment) past a unit's pages after %v; want it after the trash's contents, before a unit", f.rec.calls)
			case tt.grown > 0 && (len(f.store.read) == 0 || f.store.read[len(f.store.read)-1] > domain.MaxUnitBytes-tt.written*tt.base):
				t.Errorf("RunBatch(the segment) read %v bytes of the trash; want a unit's at most, less the bases' %d", f.store.read,
					tt.written*tt.base)
			}
		})
	}
}

// A lone page's restore and its write back are read whatever their bytes
// (M9 Codex review fix round 1, B-M2), as a unit's bounds let a lone page
// be (design 3.18.4): F, of 5 MiB, written to another 5 MiB by the batch,
// then deleted, its restore and its write back one segment, 10 MiB of
// contents, past a unit's 8 MiB. The batch reads the trash's 5 MiB whole
// and is done: a lone page never stops the job.
func TestARevertJobsLonePageIsReadWhateverItsBytes(t *testing.T) {
	s := newJobScene()
	f := s.node("F", nil, false)
	s.content(f, strings.Repeat("a", domain.MaxContentBytes))
	e := s.begin()
	e.write(f)
	n := e.m.nodes[f]
	n.content = strings.Repeat("b", domain.MaxContentBytes)
	e.m.nodes[f] = n
	e.p.version(f, n.revision, n.content)
	e.del(f)
	fx, c := s.load()
	w, spec, plan := fx.revertJob(t, c.ID, nil, now())
	to := 1
	for to < len(plan.Steps()) && plan.Steps()[to-1].Joined {
		to++
	}
	if steps, _ := plan.RevertSteps(); to != len(steps) || to != 2 {
		t.Fatalf("the steps %v; want F's restore and its write back, one segment", s.describe(steps, plan.Steps()))
	}
	if _, err := w.RunBatch(fx.asJob(), spec, plan, 0, to); err != nil || !slices.Contains(fx.store.read, domain.MaxContentBytes) {
		t.Errorf("RunBatch(F's restore and write back) = %v, the trash read %v; want done, its 5 MiB read", err, fx.store.read)
	}
}

// A later member of a group an earlier batch began, which the batch
// restores and writes back, is checked as the trash holds it (M9/P3 design
// 3.24, step 2; review C-M4, B-N2): F holds c1, c2 and c3, and a batch
// writes c3 and deletes F. The job's first batch restores F, c1 and c2;
// before its second, c3's content in the trash is of another hash than the
// changeset left it at, a state no one can make (a page in the trash is
// never written), the test's own: the second batch stops, c3 changed, and
// writes nothing.
func TestARevertJobsContinuationWrittenBackIsCheckedAsTheTrashHoldsIt(t *testing.T) {
	s := newJobScene()
	fo := s.node("F", nil, false)
	s.node("c1", &fo, false)
	s.node("c2", &fo, false)
	c3 := s.node("c3", &fo, false)
	e := s.begin()
	e.write(c3)
	e.del(fo)
	f, c := s.load()
	w, spec, plan := f.revertJob(t, c.ID, nil, now())
	steps, _ := plan.RevertSteps()
	if got := s.describe(steps, plan.Steps()); !slices.Equal(got, []string{"restore F", "restore c1", "restore c2", "restore c3+", "content c3"}) {
		t.Fatalf("the steps %v; want F's group, c3's write back right after its restore", got)
	}
	if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 3); err != nil {
		t.Fatalf("RunBatch(F, c1, c2) = %v", err)
	}
	trashed := f.store.trashed[c3]
	trashed.Content = "another content"
	sum := sha256.Sum256([]byte(trashed.Content))
	trashed.Hash = sum[:]
	f.store.trashed[c3] = trashed
	f.log.versions[domain.VersionRef{NodeID: c3, Revision: trashed.Revision}] = trashed.Content
	was := snapshotOf(f.store)
	_, err := w.RunBatch(f.asJob(), spec, plan, 3, 5)
	if got := conflictsOf(err); codeOf(err) != "page.revert_conflict" || len(got) != 1 || got[0].NodeID != c3 ||
		got[0].Reason != string(domain.ConflictChanged) || got[0].Name != "c3" {
		t.Errorf("RunBatch(c3 restored and written back) = %v %+v; want c3 changed", err, got)
	}
	if !was.same(snapshotOf(f.store)) {
		t.Error("the stopped batch wrote")
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
	if _, err := w.RunBatch(fx.asJob(), spec, plan, 0, 3); !errors.Is(err, domain.ErrJobBatchTooLarge) ||
		!strings.Contains(err.Error(), "past a unit's rows of history") || fx.called("Facts") {
		t.Errorf("RunBatch(F and c, past the history) = %v; want ErrJobBatchTooLarge past a unit's rows of history, before a parse", err)
	}
	if done, err := w.RunBatch(fx.asJob(), spec, plan, 0, 2); err != nil || done.Restored != 1 || done.Written != 1 {
		t.Errorf("RunBatch(F restored and written back, its history past the bound) = %+v, %v; want both done", done, err)
	}
	if got := fx.store.contents[f]; got.Content != s.t0.nodes[f].content || got.Revision != 3 {
		t.Errorf("F's content %q at %d, want T0's at 3", got.Content, got.Revision)
	}
}
