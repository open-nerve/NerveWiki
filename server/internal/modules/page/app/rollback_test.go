package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// rollbackReads are a rollback's reads over the fixture's store and a
// baseline, as upgrade.sql has them, of the test's own: the holders are
// the ancestors of the baseline nodes alive; a free node a new one alive
// that is no holder; what is left judged from the baseline's rows.
type rollbackReads struct {
	f        *fixture
	baseline []domain.BaselineNode
}

// of is the baseline row of id.
func (r *rollbackReads) of(id uuid.UUID) (domain.BaselineNode, bool) {
	b, ok := r.index()[id]
	return b, ok
}

// index is the baseline by node.
func (r *rollbackReads) index() map[uuid.UUID]domain.BaselineNode {
	out := make(map[uuid.UUID]domain.BaselineNode, len(r.baseline))
	for _, b := range r.baseline {
		out[b.NodeID] = b
	}
	return out
}

// holders are the ancestors of the baseline nodes alive.
func (r *rollbackReads) holders() map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	for _, b := range r.baseline {
		n, ok := r.f.store.nodes[b.NodeID]
		for ok && n.ParentID != nil && !out[*n.ParentID] {
			out[*n.ParentID] = true
			n, ok = r.f.store.nodes[*n.ParentID]
		}
	}
	return out
}

func (r *rollbackReads) RollbackLeft(ctx context.Context, _, _ uuid.UUID) (domain.RollbackLeft, error) {
	r.f.rec.record(ctx, "RollbackLeft")
	var l domain.RollbackLeft
	for _, b := range r.baseline {
		n, ok := r.f.store.nodes[b.NodeID]
		if !ok || !n.State().Same(b.State()) {
			l.Nodes++
		}
		if c, ok := r.f.store.contents[b.NodeID]; ok && b.Kind == domain.KindPage && !bytes.Equal(c.Hash, b.Hash) {
			l.Pages++
		}
	}
	index := r.index()
	for id := range r.f.store.nodes {
		if _, ok := index[id]; !ok {
			l.Created++
		}
	}
	return l, nil
}

func (r *rollbackReads) RollbackFreeForest(ctx context.Context, _, _ uuid.UUID) ([]domain.FreeNode, error) {
	r.f.rec.record(ctx, "RollbackFreeForest")
	holders, index := r.holders(), r.index()
	var out []domain.FreeNode
	for id, n := range r.f.store.nodes {
		if _, ok := index[id]; !ok && !holders[id] {
			out = append(out, domain.FreeNode{ID: id, ParentID: n.ParentID})
		}
	}
	return out, nil
}

func (r *rollbackReads) RollbackBaseline(ctx context.Context, _ uuid.UUID) ([]domain.BaselineNode, error) {
	r.f.rec.record(ctx, "RollbackBaseline")
	return slices.Clone(r.baseline), nil
}

func (r *rollbackReads) RollbackHolders(ctx context.Context, _ uuid.UUID) ([]domain.Node, error) {
	r.f.rec.record(ctx, "RollbackHolders")
	index := r.index()
	var out []domain.Node
	for id := range r.holders() {
		if _, ok := index[id]; !ok {
			out = append(out, r.f.store.nodes[id])
		}
	}
	return out, nil
}

func (r *rollbackReads) RollbackNow(ctx context.Context, _ uuid.UUID) (map[uuid.UUID]domain.NodeNow, map[uuid.UUID]domain.ContentNow, error) {
	r.f.rec.record(ctx, "RollbackNow")
	now, contents := map[uuid.UUID]domain.NodeNow{}, map[uuid.UUID]domain.ContentNow{}
	for _, b := range r.baseline {
		if n, ok := r.f.store.nodes[b.NodeID]; ok {
			now[b.NodeID] = domain.NodeNow{Node: n}
		}
		if c, ok := r.f.store.contents[b.NodeID]; ok && b.Kind == domain.KindPage {
			contents[b.NodeID] = domain.ContentNow{Revision: c.Revision, Hash: c.Hash, Bytes: c.ByteSize}
		}
	}
	return now, contents, nil
}

// rollbacks are the job writes of a rollback over the fixture, of reads,
// their deletions and moves through store, MaxRollbackItems maxItems and
// MaxUnitNodes maxNodes (0: the domain's).
func (f *fixture) rollbacks(store *checkedStore, reads app.RollbackReads, maxNodes, maxItems int) *app.JobWrites {
	return f.checkedWrites(store, app.JobRollbacks{Reads: reads, Action: changeMode, MaxNodes: maxNodes, MaxItems: maxItems})
}

// rollingBack sets eng being upgraded by alice, rolling back, the upgrade
// up.
func (f *fixture) rollingBack(up uuid.UUID, state domain.UpgradeState) {
	if f.notebooks.states == nil {
		f.notebooks.states = map[uuid.UUID]domain.NotebookState{}
	}
	f.notebooks.states[f.eng] = domain.NotebookState{Mode: domain.ModeStandard,
		Upgrade: domain.NotebookUpgrade{ID: up, State: state, By: f.alice}}
}

// plant adds a node of eng to the store at state, a page of content at
// revision 1 (none for an attachment), and answers it.
func (f *fixture) plant(id uuid.UUID, kind domain.Kind, state domain.TreeState, content string) domain.Node {
	n := domain.Node{ID: id, NotebookID: f.eng, ParentID: state.ParentID, Kind: kind, Name: state.Name, NameKey: shared.TitleKey(state.Name),
		SortOrder: state.SortOrder, CreatedBy: f.alice, UpdatedBy: f.alice, CreatedAt: now(), UpdatedAt: now()}
	f.store.nodes[id] = n
	if kind == domain.KindPage {
		sum := sha256.Sum256([]byte(content))
		f.store.contents[id] = app.Content{NodeID: id, Content: content, Revision: 1, Hash: sum[:], ByteSize: len(content), By: f.alice,
			At: now()}
		f.log.versions[domain.VersionRef{NodeID: id, Revision: 1}] = content
	}
	return n
}

// baselineOf is the store's node id as a baseline row records it now.
func (f *fixture) baselineOf(id uuid.UUID) domain.BaselineNode {
	n := f.store.nodes[id]
	b := domain.BaselineNode{NodeID: id, Kind: n.Kind, ParentID: n.ParentID, Name: n.Name, SortOrder: n.SortOrder}
	if c, ok := f.store.contents[id]; ok {
		b.Revision, b.Hash, b.Bytes = c.Revision, c.Hash, c.ByteSize
	}
	return b
}

// rewrite writes content to the page id at its next revision, a version.
func (f *fixture) rewrite(id uuid.UUID, content string) {
	c := f.store.contents[id]
	sum := sha256.Sum256([]byte(content))
	c.Revision, c.Content, c.Hash, c.ByteSize = c.Revision+1, content, sum[:], len(content)
	f.store.contents[id] = c
	f.log.versions[domain.VersionRef{NodeID: id, Revision: c.Revision}] = content
}

// runPass runs the plan's steps cut by cuts and answers what the batches
// did, up to the first error: a batch past a unit's bounds halved, at its
// segments' ends, as the job's worker halves it.
func (f *fixture) runPass(w *app.JobWrites, spec app.JobSpec, plan app.JobPlan, cuts [][2]int) (app.BatchDone, error) {
	var all app.BatchDone
	steps := plan.Steps()
	for len(cuts) > 0 {
		cut := cuts[0]
		done, err := w.RunBatch(f.asJob(), spec, plan, cut[0], cut[1])
		if errors.Is(err, domain.ErrJobBatchTooLarge) {
			mid := cut[0] + (cut[1]-cut[0])/2
			for mid < cut[1] && mid > cut[0] && steps[mid-1].Joined {
				mid++
			}
			if mid > cut[0] && mid < cut[1] {
				cuts = append([][2]int{{cut[0], mid}, {mid, cut[1]}}, cuts[1:]...)
				continue
			}
		}
		if err != nil {
			return all, err
		}
		cuts = cuts[1:]
		all.Nodes, all.Deleted, all.PutBack, all.Written = all.Nodes+done.Nodes, all.Deleted+done.Deleted, all.PutBack+done.PutBack,
			all.Written+done.Written
	}
	return all, nil
}

// A rollback goes a pass at a time (M11/P1 design 3.18): a pass that
// deletes the free subtrees and nothing else, many in a unit; then one
// pass planned from the baseline, which puts it all back, the holders
// deleted; then Done, no second plan. Each unit is the job's changeset,
// a rollback's, with its message. The upgrade no longer the notebook's,
// or not rolling back, is ErrRollbackGone, nothing read past the state;
// the starter no longer an admin, forbidden first; past MaxRollbackItems,
// page.revert_too_large.
func TestARollbackPassAtATime(t *testing.T) {
	f := newFixture()
	f.grant(changeMode)
	up := uuid.NewV7()
	f.rollingBack(up, domain.UpgradeRollingBack)
	at := func(parent *uuid.UUID, name string, order float64) domain.TreeState {
		return domain.TreeState{ParentID: parent, Name: name, SortOrder: order}
	}
	guide := f.plant(uuid.NewV7(), domain.KindPage, at(nil, "Guide", 1), "guide")
	install := f.plant(uuid.NewV7(), domain.KindPage, at(&guide.ID, "Install", 1), "install")
	notes := f.plant(uuid.NewV7(), domain.KindPage, at(nil, "Notes", 2), "notes")
	shot := f.plant(uuid.NewV7(), domain.KindAsset, at(&guide.ID, "shot.png", 2), "")
	reads := &rollbackReads{f: f}
	for _, id := range []uuid.UUID{guide.ID, install.ID, notes.ID, shot.ID} {
		reads.baseline = append(reads.baseline, f.baselineOf(id))
	}
	// The agent: scratch pages, two free subtrees; a folder holding
	// Install; Notes renamed; Guide written; the attachment reordered.
	scratch := f.plant(uuid.NewV7(), domain.KindPage, at(nil, "Scratch", 3), "s")
	f.plant(uuid.NewV7(), domain.KindPage, at(&scratch.ID, "Scratch 1", 1), "s1")
	f.plant(uuid.NewV7(), domain.KindPage, at(nil, "Draft", 4), "d")
	topics := f.plant(uuid.NewV7(), domain.KindPage, at(nil, "Topics", 5), "t")
	i := f.store.nodes[install.ID]
	i.ParentID = &topics.ID
	f.store.nodes[install.ID] = i
	n := f.store.nodes[notes.ID]
	n.Name, n.NameKey = "Notes 2", shared.TitleKey("Notes 2")
	f.store.nodes[notes.ID] = n
	f.rewrite(guide.ID, "guide, the agent's")
	s := f.store.nodes[shot.ID]
	s.SortOrder = 7
	f.store.nodes[shot.ID] = s

	store := &checkedStore{fakeStore: f.store}
	w, spec := f.rollbacks(store, reads, 0, 0), f.jobSpec()
	spec.UpgradeID = up
	plan, pass, err := w.PlanRollback(f.asJob(), spec)
	if err != nil || !pass.Deletes || pass.Done || pass.Items != 0 || pass.Left != (domain.RollbackLeft{Nodes: 3, Pages: 1, Created: 4}) {
		t.Fatalf("the first pass = %+v, %v; want one that deletes, of 3 nodes, 1 page and 4 new left", pass, err)
	}
	if steps := plan.Steps(); len(steps) != 1 || steps[0].Nodes != 3 {
		t.Fatalf("the first pass's steps %+v, want the three free nodes in one", steps)
	}
	done, err := f.runPass(w, spec, plan, [][2]int{{0, 1}})
	if err != nil || done.Deleted != 3 || done.Nodes != 3 || len(store.bad) > 0 {
		t.Fatalf("the first pass did %+v, %v, %v; want three deleted", done, err, store.bad)
	}
	if _, ok := f.store.nodes[scratch.ID]; ok || len(f.store.changesets) != 1 {
		t.Fatalf("after the first pass Scratch is alive or %d changesets, want it gone and one", len(f.store.changesets))
	}
	if c := f.store.changesets[0]; c.ID != spec.Changeset || c.Kind != domain.ChangesetRollback || c.Message == nil ||
		*c.Message != "Upgrade rollback" || c.Client != domain.ClientWeb {
		t.Errorf("the job's changeset %+v, want a rollback's, Upgrade rollback", c)
	}

	plan, pass, err = w.PlanRollback(f.asJob(), spec)
	if err != nil || pass.Deletes || pass.Done || pass.Items != 5 || pass.Left != (domain.RollbackLeft{Nodes: 3, Pages: 1, Created: 1}) {
		t.Fatalf("the second pass = %+v, %v; want one planned, of four items and a version", pass, err)
	}
	done, err = f.runPass(w, spec, plan, [][2]int{{0, len(plan.Steps())}})
	if err != nil || len(store.bad) > 0 {
		t.Fatalf("the second pass = %+v, %v, %v", done, err, store.bad)
	}
	if done.PutBack != 3 || done.Deleted != 1 || done.Written != 1 {
		t.Errorf("the second pass did %+v, want 3 put back, Topics deleted, Guide written back", done)
	}
	for _, b := range reads.baseline {
		if n := f.store.nodes[b.NodeID]; !n.State().Same(b.State()) {
			t.Errorf("%s at %+v, want the baseline's %+v", b.Name, n.State(), b.State())
		}
	}
	if c := f.store.contents[guide.ID]; c.Content != "guide" || len(f.store.changesets) != 1 {
		t.Errorf("Guide %q, %d changesets; want its baseline's content, written in the job's one", c.Content, len(f.store.changesets))
	}

	_, pass, err = w.PlanRollback(f.asJob(), spec)
	if err != nil || !pass.Done || !pass.Left.Zero() {
		t.Fatalf("the third pass = %+v, %v; want Done, nothing left", pass, err)
	}

	t.Run("the upgrade gone", func(t *testing.T) {
		for _, tt := range []struct {
			name  string
			state func()
			spec  uuid.UUID
		}{
			{"another upgrade", func() { f.rollingBack(uuid.NewV7(), domain.UpgradeRollingBack) }, up},
			{"upgrading", func() { f.rollingBack(up, domain.UpgradeUpgrading) }, up},
			{"none", func() { delete(f.notebooks.states, f.eng) }, up},
			{"a job of no upgrade", func() { f.rollingBack(up, domain.UpgradeRollingBack) }, uuid.UUID{}},
		} {
			tt.state()
			spec := spec
			spec.UpgradeID = tt.spec
			f.rec.calls = nil
			if _, _, err := w.PlanRollback(f.asJob(), spec); !errors.Is(err, app.ErrRollbackGone) {
				t.Errorf("%s: PlanRollback() = %v, want ErrRollbackGone", tt.name, err)
			}
			if got := strings.Join(f.rec.calls, ", "); strings.Contains(got, "Rollback") {
				t.Errorf("%s: read %s past the state", tt.name, got)
			}
		}
		f.rollingBack(up, domain.UpgradeRollingBack)
	})
	t.Run("the starter no longer an admin, the notebook gone", func(t *testing.T) {
		f.auth.grants[changeMode], f.auth.forbidden[changeMode] = false, true
		var e *shared.Error
		if _, _, err := w.PlanRollback(f.asJob(), spec); !errors.As(err, &e) || e.Kind != shared.KindForbidden {
			t.Errorf("PlanRollback() of a starter no longer an admin = %v, want forbidden", err)
		}
		f.auth.grants[changeMode], f.auth.forbidden[changeMode] = true, false
		f.notebooks.gone[f.eng] = true
		if _, _, err := w.PlanRollback(f.asJob(), spec); !errors.Is(err, domain.ErrNotebookNotFound) {
			t.Errorf("PlanRollback() of a notebook gone = %v, want notebook.not_found", err)
		}
		delete(f.notebooks.gone, f.eng)
	})
	t.Run("past MaxRollbackItems", func(t *testing.T) {
		n := f.store.nodes[notes.ID]
		n.SortOrder = 9
		f.store.nodes[notes.ID] = n
		g := f.store.nodes[guide.ID]
		g.SortOrder = 9
		f.store.nodes[guide.ID] = g
		if _, _, err := f.rollbacks(store, reads, 0, 1).PlanRollback(f.asJob(), spec); !errors.Is(err, domain.ErrRevertTooLarge) {
			t.Errorf("two items past 1 = %v, want page.revert_too_large", err)
		}
	})
	t.Run("free subtrees past MaxUnitNodes", func(t *testing.T) {
		for i := range 5 {
			f.plant(uuid.NewV7(), domain.KindPage, at(nil, fmt.Sprintf("Loose %d", i), float64(10+i)), "l")
		}
		plan, pass, err := f.rollbacks(store, reads, 2, 0).PlanRollback(f.asJob(), spec)
		if err != nil || !pass.Deletes || len(plan.Steps()) != 3 {
			t.Fatalf("five loose pages, two a unit = %+v, %v, %d steps; want three", pass, err, len(plan.Steps()))
		}
		// Two packs in one batch are past MaxUnitNodes: to halve.
		if _, err := f.rollbacks(store, reads, 2, 0).RunBatch(f.asJob(), spec, plan, 0, 2); !errors.Is(err, domain.ErrJobBatchTooLarge) {
			t.Errorf("two packs of two in a batch = %v, want ErrJobBatchTooLarge", err)
		}
	})
	// A unit that deletes first packs MaxDeleteFirstNodes, below
	// MaxUnitNodes (M11 decision 243): the same five, two a pack of four
	// whole, are three steps, and two of them in a batch are past it.
	t.Run("free subtrees past MaxDeleteFirstNodes", func(t *testing.T) {
		w := f.checkedWrites(store, app.JobRollbacks{Reads: reads, Action: changeMode, MaxNodes: 4, MaxDeleteNodes: 2})
		plan, pass, err := w.PlanRollback(f.asJob(), spec)
		if err != nil || !pass.Deletes || len(plan.Steps()) != 3 {
			t.Fatalf("five loose pages, two a unit = %+v, %v, %d steps; want three", pass, err, len(plan.Steps()))
		}
		if _, err := w.RunBatch(f.asJob(), spec, plan, 0, 2); !errors.Is(err, domain.ErrJobBatchTooLarge) {
			t.Errorf("two packs of two in a batch = %v, want ErrJobBatchTooLarge", err)
		}
	})
}

// rbTrial is a rollback's trial over the fixture: a random baseline, the
// agent's random changes through the guardrails, each pass of the
// rollback run in batches cut at random.
type rbTrial struct {
	rng   *rand.Rand
	f     *fixture
	reads *rollbackReads
	names []string
	order float64
}

func (p *rbTrial) alive() []uuid.UUID {
	var out []uuid.UUID
	for id := range p.f.store.nodes {
		out = append(out, id)
	}
	slices.SortFunc(out, func(a, b uuid.UUID) int { return a.Compare(b) })
	return out
}

func (p *rbTrial) under(id, top uuid.UUID) bool {
	for at := &id; at != nil; at = p.f.store.nodes[*at].ParentID {
		if *at == top {
			return true
		}
	}
	return false
}

func (p *rbTrial) depth(id uuid.UUID) int {
	d := 0
	for at := &id; at != nil; at = p.f.store.nodes[*at].ParentID {
		d++
	}
	return d
}

func (p *rbTrial) free(parent *uuid.UUID, name string, except uuid.UUID) bool {
	key := shared.TitleKey(name)
	for id, n := range p.f.store.nodes {
		if id != except && domain.SameParent(n.ParentID, parent) && n.NameKey == key {
			return false
		}
	}
	return true
}

func (p *rbTrial) parentFor() *uuid.UUID {
	var pages []uuid.UUID
	for _, id := range p.alive() {
		if p.f.store.nodes[id].Kind == domain.KindPage && p.depth(id) < 3 {
			pages = append(pages, id)
		}
	}
	if len(pages) == 0 || p.rng.IntN(4) == 0 {
		return nil
	}
	return &pages[p.rng.IntN(len(pages))]
}

func (p *rbTrial) height(id uuid.UUID) int {
	h := 1
	for c, n := range p.f.store.nodes {
		if n.ParentID != nil && *n.ParentID == id {
			h = max(h, 1+p.height(c))
		}
	}
	return h
}

func (p *rbTrial) place(id uuid.UUID, to *uuid.UUID, name string, order float64) {
	n := p.f.store.nodes[id]
	n.ParentID, n.Name, n.NameKey, n.SortOrder = to, name, shared.TitleKey(name), order
	p.f.store.nodes[id] = n
}

func (p *rbTrial) holds(id uuid.UUID) bool {
	for _, b := range p.reads.baseline {
		if b.NodeID != id && p.under(b.NodeID, id) {
			if _, ok := p.f.store.nodes[b.NodeID]; ok {
				return true
			}
		}
	}
	return false
}

func newRbTrial(seed uint64) *rbTrial {
	f := newFixture()
	f.grant(changeMode)
	p := &rbTrial{rng: rand.New(rand.NewPCG(seed, 0x2545f4914f6cdd1d)), f: f, reads: &rollbackReads{f: f}}
	for i := range 10 {
		p.names = append(p.names, fmt.Sprintf("n%d", i))
	}
	p.names = append(p.names, "a.png")
	for range 4 + p.rng.IntN(16) {
		to, name := p.parentFor(), p.names[p.rng.IntN(len(p.names))]
		if !p.free(to, name, uuid.UUID{}) {
			continue
		}
		kind := domain.KindPage
		if p.rng.IntN(5) == 0 {
			kind = domain.KindAsset
		}
		p.order++
		id := f.plant(uuid.NewV7(), kind, domain.TreeState{ParentID: to, Name: name, SortOrder: p.order}, fmt.Sprintf("base %v", p.order)).ID
		if kind == domain.KindPage && p.rng.IntN(2) == 0 {
			f.rewrite(id, fmt.Sprintf("base %v, second", p.order))
		}
	}
	for _, id := range p.alive() {
		p.reads.baseline = append(p.reads.baseline, f.baselineOf(id))
	}
	return p
}

// agent changes the notebook as the initiator's agent may: moves across
// levels, renames, the names of siblings in a cycle, reorders, new pages
// (free, or holding baseline nodes once some move under them), writes,
// some back to the baseline's content, deletes of new subtrees holding no
// baseline node.
func (p *rbTrial) agent(ops int) {
	for range ops {
		live := p.alive()
		id := live[p.rng.IntN(len(live))]
		n := p.f.store.nodes[id]
		switch p.rng.IntN(8) {
		case 0, 1:
			to, name := p.parentFor(), n.Name
			if p.rng.IntN(3) == 0 {
				name = p.names[p.rng.IntN(len(p.names))]
			}
			if to != nil && (p.under(*to, id) || p.depth(*to)+p.height(id) > 4) || !p.free(to, name, id) {
				continue
			}
			p.place(id, to, name, float64(p.rng.IntN(8)))
		case 2:
			if name := p.names[p.rng.IntN(len(p.names))]; p.free(n.ParentID, name, id) {
				p.place(id, n.ParentID, name, n.SortOrder)
			}
		case 3:
			p.place(id, n.ParentID, n.Name, n.SortOrder+0.5)
		case 4:
			to, name := p.parentFor(), p.names[p.rng.IntN(len(p.names))]
			if p.free(to, name, uuid.UUID{}) {
				p.order++
				p.f.plant(uuid.NewV7(), domain.KindPage, domain.TreeState{ParentID: to, Name: name, SortOrder: p.order}, "new")
			}
		case 5:
			if n.Kind != domain.KindPage {
				continue
			}
			content := fmt.Sprintf("w%d", p.rng.IntN(100))
			if b, ok := p.reads.of(id); ok && p.rng.IntN(3) == 0 {
				content = p.f.log.versions[domain.VersionRef{NodeID: id, Revision: b.Revision}]
			}
			p.f.rewrite(id, content)
		case 6:
			var sibs []uuid.UUID
			for _, o := range live {
				if m := p.f.store.nodes[o]; domain.SameParent(m.ParentID, n.ParentID) && m.Kind == n.Kind {
					sibs = append(sibs, o)
				}
			}
			p.rng.Shuffle(len(sibs), func(i, j int) { sibs[i], sibs[j] = sibs[j], sibs[i] })
			if len(sibs) < 2 {
				continue
			}
			sibs = sibs[:min(len(sibs), 2+p.rng.IntN(2))]
			first := p.f.store.nodes[sibs[0]].Name
			for i := range len(sibs) - 1 {
				m := p.f.store.nodes[sibs[i+1]]
				p.place(sibs[i], m.ParentID, m.Name, p.f.store.nodes[sibs[i]].SortOrder)
			}
			last := p.f.store.nodes[sibs[len(sibs)-1]]
			p.place(sibs[len(sibs)-1], last.ParentID, first, last.SortOrder)
		case 7:
			if _, ok := p.reads.of(id); ok || p.holds(id) {
				continue
			}
			for _, c := range p.alive() {
				if p.under(c, id) {
					delete(p.f.store.nodes, c)
					delete(p.f.store.contents, c)
				}
			}
		}
	}
}

// The rollback's passes, each run in batches cut at random, put the
// notebook back as the upgrade started (M11/P1 design 3.18, after
// TestTheJobsStepsReplayToStar): the passes that delete first, then one
// pass planned, then Done; every write as a store checks it; at the end
// each baseline node where it was with its content, nothing else alive.
func TestTheRollbacksStepsReplayToTheBaseline(t *testing.T) {
	trials := 2000
	if testing.Short() {
		trials = 400
	}
	var deletes, planned int
	for seed := range trials {
		p := newRbTrial(uint64(seed))
		p.agent(5 + p.rng.IntN(30))
		up := uuid.NewV7()
		p.f.rollingBack(up, domain.UpgradeRollingBack)
		store := &checkedStore{fakeStore: p.f.store}
		whole := 1 + p.rng.IntN(4)
		w := p.f.checkedWrites(store, app.JobRollbacks{Reads: p.reads, Action: changeMode, MaxNodes: whole,
			MaxDeleteNodes: 1 + p.rng.IntN(whole)})
		spec := p.f.jobSpec()
		spec.UpgradeID = up
		plans := 0
		for pass := 0; ; pass++ {
			if pass > 30 {
				t.Fatalf("seed %d: no end", seed)
			}
			plan, rb, err := w.PlanRollback(p.f.asJob(), spec)
			if err != nil {
				t.Fatalf("seed %d, pass %d: %v", seed, pass, err)
			}
			if rb.Done {
				break
			}
			if rb.Deletes {
				if plans > 0 {
					t.Fatalf("seed %d, pass %d: deletes after a plan", seed, pass)
				}
				deletes++
			} else {
				if plans++; plans > 1 {
					t.Fatalf("seed %d, pass %d: a second plan, left %+v", seed, pass, rb.Left)
				}
				planned++
			}
			if _, err := p.f.runPass(w, spec, plan, cutsBy(p.rng, plan.Steps())); err != nil || len(store.bad) > 0 {
				t.Fatalf("seed %d, pass %d: %v, %v", seed, pass, err, store.bad)
			}
		}
		for _, b := range p.reads.baseline {
			n, ok := p.f.store.nodes[b.NodeID]
			if !ok || !n.State().Same(b.State()) {
				t.Fatalf("seed %d: %s at %+v, want the baseline's %+v", seed, b.NodeID, n.State(), b.State())
			}
			if c := p.f.store.contents[b.NodeID]; b.Kind == domain.KindPage && !bytes.Equal(c.Hash, b.Hash) {
				t.Fatalf("seed %d: %s of another content", seed, b.NodeID)
			}
		}
		if len(p.f.store.nodes) != len(p.reads.baseline) {
			t.Fatalf("seed %d: %d nodes alive, the baseline's %d", seed, len(p.f.store.nodes), len(p.reads.baseline))
		}
	}
	t.Logf("%d trials: %d passes deleting first, %d planned", trials, deletes, planned)
	if deletes < trials/2 || planned < trials/2 {
		t.Errorf("idle: %d passes deleting first, %d planned in %d trials", deletes, planned, trials)
	}
}
