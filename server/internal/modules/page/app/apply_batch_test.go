package app_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fakeLinks is the BatchLinks of a batch's tests: a content's links are
// its [[...]], each leading by its text's key to the first node of the
// plan's tree so keyed; a rewrite writes each as subs tells. Each content
// it resolves or rewrites goes through the plan's parse, a rewrite's read
// back too. The renames and moves before a write reach its page when
// reached says so.
type fakeLinks struct {
	*recorder
	subs       map[string]string
	notIndexed bool
	reached    bool
	frozen     map[string][]uuid.UUID               // the frozen nodes Reached and Rewrite were last given
	created    map[string]map[uuid.UUID][]uuid.UUID // the nodes created after moves they were last given, by call and page
	reads      []*app.TreeReads                     // the reads Reached and Rewrite were given, in their order
}

func (l *fakeLinks) Indexed(ctx context.Context, _, _ uuid.UUID, _ int, _ domain.PlanTree) (domain.LinksBefore, bool, error) {
	l.record(ctx, "Indexed")
	return domain.LinksBefore{Leads: map[int]domain.Lead{}, Targets: map[uuid.UUID][]domain.PathStep{}}, !l.notIndexed, nil
}

func (l *fakeLinks) Reached(ctx context.Context, _, id uuid.UUID, _ int, _ []domain.Change, frozen []uuid.UUID,
	created map[uuid.UUID][]uuid.UUID, _ domain.PlanTree, reads *app.TreeReads,
) (bool, error) {
	l.kept("Reached", reads)
	l.record(ctx, "Reached")
	l.saw("Reached", frozen)
	l.made("Reached", id, created)
	return l.reached && !l.notIndexed, nil
}

func (l *fakeLinks) Resolve(ctx context.Context, q app.LinksOn) (domain.LinksBefore, error) {
	l.record(ctx, "Resolve")
	if _, err := q.Parse(ctx, q.Content); err != nil {
		return domain.LinksBefore{}, err
	}
	return leadsOn(q.Tree, q.Content), nil
}

func (l *fakeLinks) Rewrite(ctx context.Context, q app.LinksMoved) (string, error) {
	l.record(ctx, "Rewrite")
	l.saw("Rewrite", q.Frozen)
	l.made("Rewrite", q.PageID, q.CreatedAfter)
	l.kept("Rewrite", q.Reads)
	if _, err := q.Parse(ctx, q.Content); err != nil {
		return "", err
	}
	out := q.Content
	for from, to := range l.subs {
		out = strings.ReplaceAll(out, from, to)
	}
	if out != q.Content {
		if _, err := q.Parse(ctx, out); err != nil { // the read back
			return "", err
		}
	}
	return out, nil
}

// kept keeps the reads call was given.
func (l *fakeLinks) kept(_ string, reads *app.TreeReads) {
	l.reads = append(l.reads, reads)
}

// saw keeps the frozen nodes call was given.
func (l *fakeLinks) saw(call string, frozen []uuid.UUID) {
	if l.frozen == nil {
		l.frozen = map[string][]uuid.UUID{}
	}
	l.frozen[call] = frozen
}

// made keeps the nodes created after moves call was given for the page
// id.
func (l *fakeLinks) made(call string, id uuid.UUID, created map[uuid.UUID][]uuid.UUID) {
	if l.created == nil {
		l.created = map[string]map[uuid.UUID][]uuid.UUID{}
	}
	l.created[call+" "+id.String()] = created
}

// leadsOn is where content's [[...]] lead on tree, by where each starts.
func leadsOn(tree domain.PlanTree, content string) domain.LinksBefore {
	out := domain.LinksBefore{Leads: map[int]domain.Lead{}, Targets: map[uuid.UUID][]domain.PathStep{}}
	for start := 0; ; {
		i := strings.Index(content[start:], "[[")
		if i < 0 {
			return out
		}
		at := start + i + 2
		end := strings.Index(content[at:], "]]")
		if end < 0 {
			return out
		}
		out.Leads[at] = domain.Lead{}
		if nodes := tree.ByKeys([]string{shared.TitleKey(content[at : at+end])}); len(nodes) > 0 {
			out.Leads[at] = domain.Lead{ID: nodes[0].ID}
			out.Targets[nodes[0].ID] = nodes[0].Path
		}
		start = at + end
	}
}

// batcher is batch's use case over the fixture, with links.
func (f *fixture) batcher(links app.BatchLinks) *app.ApplyBatch {
	return app.NewApplyBatch(f.writer(), f.parser(), f.store, f.store, f.store, links, app.NewLockHolders(f.store, f.names),
		f.notebooks, f.auth, 0, 0, 0, f.clock, f.logger())
}

// applyBatch runs batch in eng as alice from an agent, page.batch
// granted, its links links.
func (f *fixture) applyBatch(links app.BatchLinks, ops ...domain.BatchArgs) (app.Batched, error) {
	f.grant(domain.ActionBatch)
	return f.batcher(links).Execute(f.asAlice(), f.eng, app.PathBatch{Ops: ops, Message: "a batch"}, agent)
}

// The ops of a batch as an agent gives them.
func writeOp(path, content string) domain.BatchArgs {
	return domain.BatchArgs{Op: "write", Path: &path, Content: &content}
}

func writeOn(path, content string, base *string) domain.BatchArgs {
	return domain.BatchArgs{Op: "write", Path: &path, Content: &content, Base: base}
}

func editOp(path, old, new string) domain.BatchArgs {
	return domain.BatchArgs{Op: "edit", Path: &path, Old: &old, New: &new}
}

func moveOp(from, to string) domain.BatchArgs {
	return domain.BatchArgs{Op: "move", From: &from, To: &to}
}

func keepOp(from, to string) domain.BatchArgs {
	return domain.BatchArgs{Op: "move", From: &from, To: &to, UpdateLinks: new(false)}
}

func deleteOp(path string, recursive bool) domain.BatchArgs {
	return domain.BatchArgs{Op: "delete", Path: &path, Recursive: &recursive}
}

// opOf is err's op and its code, "-1 code" for an error at no op.
func opOf(err error) string {
	var f *domain.OpFailed
	if !errors.As(err, &f) {
		return "-1 " + codeOf(err)
	}
	return fmt.Sprintf("%d %s", f.Op, codeOf(err))
}

// A batch's unit creates its pages with the ids its plan gave them
// (PageDraft.ID): every id its participants get, the pages written, their
// links' leads and targets, the changes, is one the store created or read
// (M9/P3 review C-I2).
func TestABatchCreatesThePagesItPlanned(t *testing.T) {
	f := newFixture()
	a := f.page("A", nil, 1)
	p := &participant{recorder: f.rec}
	f.partakers = []app.Participant{p}
	links := &fakeLinks{recorder: f.rec}
	out, err := f.applyBatch(links, writeOp("F/N.md", "see [[A]]"), moveOp("A.md", "F/B.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.steps) != 1 || p.steps[0].Batch == nil {
		t.Fatalf("the participants followed %+v, want the batch's one step", p.steps)
	}
	known := func(id uuid.UUID) bool { _, ok := f.store.nodes[id]; return ok }
	b := p.steps[0].Batch
	for id, w := range b.Written {
		if !known(id) {
			t.Errorf("a page written, %s, is none the store holds", id)
		}
		for _, l := range w.Before.Leads {
			if l.ID != (uuid.UUID{}) && !known(l.ID) {
				t.Errorf("a lead of %s, to %s, is none the store holds", id, l.ID)
			}
		}
		for target, path := range w.Before.Targets {
			for _, s := range path {
				if !known(s.ID) {
					t.Errorf("a step of the path of %s, %s, is none the store holds", target, s.ID)
				}
			}
		}
	}
	for _, c := range p.steps[0].Changes {
		if !known(c.NodeID) {
			t.Errorf("a change of %s, none the store holds", c.NodeID)
		}
	}
	created := out.Ops[0].Created
	if len(created) != 2 || created[0].Path != "F.md" || created[1].Path != "F/N.md" || !known(created[1].Node.ID) ||
		b.Written[created[1].Node.ID].Facts != "see [[A]]" || out.Ops[1].Path != "F/B.md" || out.Ops[1].Node.ID != a.ID {
		t.Errorf("the ops' outcomes = %+v; want F.md and F/N.md created, A at F/B.md", out.Ops)
	}
}

// A write after a move that reaches its page by the index is judged on the
// page as the move leaves it, as separate calls are (M9/P3 review A-I1):
// the page is read whole, and the write of the bytes the move leaves is
// unchanged. A write that no move reaches reads the page's head alone, as
// before: its bytes are not the page's.
func TestABatchsWriteSeesItsPageAsTheMovesBeforeItLeaveIt(t *testing.T) {
	for _, reached := range []bool{true, false} {
		f := newFixture()
		f.page("A", nil, 1)
		p := f.page("P", nil, 2)
		f.content(p.ID, "[[A]] p", 3)
		links := &fakeLinks{recorder: f.rec, subs: map[string]string{"[[A]]": "[[B]]"}, reached: reached}
		out, err := f.applyBatch(links, moveOp("A.md", "B.md"), writeOp("P.md", "[[B]] p"))
		switch {
		case reached && (err != nil || out.Ops[1].Changed || f.store.contents[p.ID].Content != "[[B]] p" || f.count("Reached") != 1 ||
			f.count("PageContentWithin") != 1):
			t.Errorf("reached: %v, %+v, P %q, Reached asked %d times, %d contents read; want done, unchanged, P read whole", err,
				out.Ops, f.store.contents[p.ID].Content, f.count("Reached"), f.count("PageContentWithin"))
		case !reached && (opOf(err) != "1 page.title_taken" || f.count("PageContentWithin") != 0):
			t.Errorf("not reached: %v, %d contents read; want ops[1]'s PathTaken on P's head", err, f.count("PageContentWithin"))
		}
	}
}

// A batch's content ops of one page are one write (M9/P3 design 3.5.2): at
// its first content op, of its last content, through the guards once, one
// version; the later ops find the page alone. A write without a token of
// the bytes the page holds is the page's content op too: it locks the
// page's content and compares its revision, so that another's write since
// the plan is a redo, whose plan answers that write's PathTaken (review
// A-M4). Once compared under the lock, a mismatch of the write is a
// defect, never a redo.
func TestABatchWritesAPageOnceAtItsFirstContentOp(t *testing.T) {
	f := newFixture()
	p, x := f.page("P", nil, 1), f.page("X", nil, 2)
	f.content(p.ID, "a", 3)
	g := &guard{recorder: f.rec}
	f.guards = []app.WriteGuard{g}
	out, err := f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("P.md", "a", "b"), keepOp("X.md", "Y.md"), editOp("P.md", "b", "c"),
		writeOn("P.md", "final", tokenOf(p.ID, 3)))
	if err != nil {
		t.Fatal(err)
	}
	var ops []string
	for _, s := range g.steps {
		ops = append(ops, string(s.Operation))
	}
	if !slices.Equal(ops, []string{string(domain.OpContent), string(domain.OpRename)}) || g.steps[0].Changes[0].Facts != "final" ||
		len(f.store.revisions) != 1 || f.store.contents[p.ID].Content != "final" || f.store.contents[p.ID].Revision != 4 {
		t.Errorf("the guards saw %v, %d versions, P %+v; want P's last content written once, first", ops, len(f.store.revisions),
			f.store.contents[p.ID])
	}
	for i, want := range []int{4, 0, 4, 4} {
		if i != 1 && out.Ops[i].Version != (domain.VersionToken{Node: p.ID, Revision: want}) {
			t.Errorf("ops[%d]'s version = %v, want P's last, %d", i, out.Ops[i].Version, want)
		}
	}
	if out.Ops[1].Node.ID != x.ID || out.Ops[1].Path != "Y.md" {
		t.Errorf("ops[1] = %+v, want X at Y.md", out.Ops[1])
	}
	if logs := f.logs.String(); !strings.Contains(logs, `msg="batch applied"`) || !strings.Contains(logs, "unit_ms=") ||
		!strings.Contains(logs, "pages_written=1") || strings.Contains(logs, "final") || strings.Contains(logs, ".md") ||
		strings.Contains(logs, "a batch") {
		t.Errorf("the batch's log = %s; want its ids, counts and time, no path, content or message", logs)
	}

	f = newFixture()
	p = f.page("P", nil, 1)
	f.content(p.ID, "p", 2)
	f.rec.before = map[string]func(){"LockContent in tx": func() { f.writeAs(p.ID, "other") }}
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, writeOp("P.md", "p")); opOf(err) != "0 page.title_taken" ||
		f.count("ListNodes") != 2 {
		t.Errorf("a write of the bytes there after another's write = %v after %v; want ops[0]'s PathTaken, planned again",
			err, f.rec.calls)
	}

	f = newFixture()
	p = f.page("P", nil, 1)
	f.content(p.ID, "a", 2)
	f.rec.before = map[string]func(){"LockContent in tx": func() {
		f.rec.before["LockContent in tx"] = func() { f.writeAs(p.ID, "under the lock") }
	}}
	_, err = f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("P.md", "a", "b"))
	var problem *shared.Error
	if err == nil || errors.As(err, &problem) || f.count("ListNodes") != 1 {
		t.Errorf("a write that meets another's under its lock = %v after %v; want an internal error, not planned again", err,
			f.rec.calls)
	}
}

// A batch's participants follow it once, after all its ops (M9/P3 design
// 3.7.1): its renames and moves merged node by node, of no revision, the
// nodes a move carried as they are, a node created and then moved from
// where its creation put it, none a move kept the links of; each page its
// content ops decided, alive after it, with its own changes, those after
// its last content op of the nodes they rename or move alone, its links
// then and its final content's facts, one whose edits put back the bytes
// it held among them, told unwritten (Codex review R1), one edited and
// then deleted not; the former parents' paths, the nodes gone, the nodes
// it froze with their moves merged (M9/P3 review B1-N1), the bounds, of
// pages, links and bytes, and the first op that moves.
func TestABatchsParticipantsFollowItOnceItsOpsDone(t *testing.T) {
	f := newFixture()
	a, folder, g, h := f.page("A", nil, 1), f.page("F", nil, 2), f.page("G", nil, 3), f.page("H", nil, 4)
	x := f.page("X", &folder.ID, 1)
	e := f.page("E", nil, 5)
	q := f.page("Q", &e.ID, 1)
	d := f.page("D", nil, 6)
	w := f.page("W", nil, 7)
	f.content(w.ID, "see [[A]] and [[Y]]", 2)
	back, edited := f.page("U", nil, 8), f.page("Z", nil, 9)
	f.content(back.ID, "u [[A]]", 2)
	f.content(edited.ID, "z", 2)
	p := &participant{recorder: f.rec}
	f.partakers = []app.Participant{p}
	links := &fakeLinks{recorder: f.rec}
	out, err := f.applyBatch(links, writeOp("N.md", "n"), moveOp("N.md", "G/N2.md"), editOp("W.md", "see", "look"),
		editOp("U.md", "u", "v"), editOp("U.md", "v", "u"), editOp("Z.md", "z", "y"), moveOp("A.md", "B.md"), moveOp("F.md", "G/F.md"),
		moveOp("E/Q.md", "Q.md"), keepOp("H.md", "H2.md"), deleteOp("D.md", false), deleteOp("Z.md", false))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.steps) != 1 || slices.Index(f.rec.calls, "Participate move in tx") < slices.Index(f.rec.calls, "DeleteNodes in tx") {
		t.Fatalf("the participants followed %d steps, at %v; want one, after the last op", len(p.steps), f.rec.calls)
	}
	n := out.Ops[0].Node.ID
	s := p.steps[0]
	var ids []uuid.UUID
	for _, c := range s.Changes {
		ids = append(ids, c.NodeID)
		if c.Revision != 0 || c.Facts != nil {
			t.Errorf("the change of %s carries revision %d, facts %v; want none", c.NodeID, c.Revision, c.Facts)
		}
	}
	if want := []uuid.UUID{n, a.ID, folder.ID, x.ID, q.ID}; !slices.Equal(ids, want) || slices.Contains(ids, h.ID) {
		t.Errorf("the changes followed are of %v, want N, A, F and X it carried, Q; not H, kept: %v", ids, want)
	}
	if c := s.Changes[0]; c.Before == nil || c.Before.ParentID != nil || c.Before.Name != "N" || *c.After.ParentID != g.ID {
		t.Errorf("N's change = %+v, want from where its creation put it", c)
	}
	b := s.Batch
	if b.MaxPages != domain.MaxBatchRewrites || b.WrittenWeight != domain.WrittenRewriteWeight || b.MaxLinks != domain.MaxBatchRewriteLinks ||
		b.MaxBytes != domain.MaxBatchRewriteBytes || b.FirstMove != 1 ||
		!slices.Equal(b.Gone, []uuid.UUID{d.ID, edited.ID}) || len(b.Parents) != 1 || len(b.Parents[e.ID]) != 1 ||
		b.Parents[e.ID][0].Name != "E" ||
		!slices.Equal(b.Frozen, []uuid.UUID{h.ID}) || len(b.FrozenMoves) != 1 || b.FrozenMoves[0].NodeID != h.ID ||
		b.FrozenMoves[0].Before.Name != "H" || b.FrozenMoves[0].After.Name != "H2" {
		t.Errorf("the batch's rewrite = max %d, weight %d, links %d, bytes %d, first %d, gone %v, parents %v, frozen %v, %+v",
			b.MaxPages, b.WrittenWeight, b.MaxLinks, b.MaxBytes, b.FirstMove, b.Gone, b.Parents, b.Frozen, b.FrozenMoves)
	}
	if len(b.Written) != 3 || b.Written[w.ID].Unwritten || b.Written[n].Unwritten || !b.Written[back.ID].Unwritten {
		t.Fatalf("the pages decided = %v, want W and N written, U not; Z, deleted, not among them", b.Written)
	}
	for _, id := range []uuid.UUID{w.ID, back.ID} {
		written := b.Written[id]
		var own []uuid.UUID
		for _, c := range written.Changes {
			own = append(own, c.NodeID)
		}
		facts := map[uuid.UUID]app.Facts{w.ID: "look [[A]] and [[Y]]", back.ID: "u [[A]]"}[id]
		leads := map[uuid.UUID]int{w.ID: 2, back.ID: 1}[id]
		if !slices.Equal(own, []uuid.UUID{a.ID, folder.ID, q.ID}) || written.Facts != facts || len(written.Before.Leads) != leads ||
			written.Before.Targets[a.ID] == nil || !slices.Equal(written.Frozen, []uuid.UUID{h.ID}) {
			t.Errorf("the rewrite of %s = changes of %v, facts %v, before %+v, frozen %v; want A, F and Q, not X under F nor H, "+
				"its last content, H frozen", f.store.nodes[id].Name, own, written.Facts, written.Before, written.Frozen)
		}
	}
	if created := b.Written[n]; len(created.Changes) != 4 || created.Facts != "n" || !slices.Equal(created.Frozen, []uuid.UUID{h.ID}) {
		t.Errorf("N's rewrite = %+v, want the moves after its creation, its facts, H frozen", created)
	}
}

// A batch's unit queues the nodes it changes for the search in bulk, its
// outcome Building (M9/P3 design 3.10.1: its step 0's measure); one that
// queues none, a move under another parent, is not building.
func TestABatchQueuesForTheSearchInBulk(t *testing.T) {
	f := newFixture()
	p := f.page("P", nil, 1)
	f.content(p.ID, "p", 1)
	folder := f.page("F", nil, 2)
	out, err := f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("P.md", "p", "q"), writeOp("N.md", "n"))
	if err != nil || len(f.store.queued) != 1 || !f.store.queued[0].bulk || len(f.store.queued[0].ids) != 2 || !out.Outcome.Building {
		t.Errorf("a batch's queue = %+v, building %t, %v; want P and N in bulk, building", f.store.queued, out.Outcome.Building, err)
	}
	f.store.queued = nil
	out, err = f.applyBatch(&fakeLinks{recorder: f.rec}, moveOp("P.md", "F/P.md"))
	if err != nil || len(f.store.queued) != 0 || out.Outcome.Building || f.store.nodes[p.ID].ParentID == nil ||
		*f.store.nodes[p.ID].ParentID != folder.ID {
		t.Errorf("a batch of a move = %+v, building %t, %v; want none queued, not building", f.store.queued, out.Outcome.Building, err)
	}
}

// The plan's rewrites of the content ops between two ops that change the
// tree share one of the links' reads of it (M9/P3 review B-N2): P's and
// Q's after A's move, R's after B's move another.
func TestABatchsPlanReadsATreeOnce(t *testing.T) {
	f := newFixture()
	f.page("A", nil, 1)
	for i, name := range []string{"P", "Q", "R"} {
		f.content(f.page(name, nil, float64(i+2)).ID, "[[A]] x", 1)
	}
	links := &fakeLinks{recorder: f.rec}
	if _, err := f.applyBatch(links, moveOp("A.md", "B.md"), editOp("P.md", "x", "p"), editOp("Q.md", "x", "q"), moveOp("B.md", "C.md"),
		editOp("R.md", "x", "r")); err != nil {
		t.Fatal(err)
	}
	if r := links.reads; len(r) != 3 || r[0] == nil || r[1] != r[0] || r[2] == r[0] || r[2] == nil {
		t.Errorf("the rewrites were given the reads %v; want P's and Q's one, R's another", r)
	}
}

// The plan holds one tree's reads of the links at a time (M9/P3 fix round
// 3: one kept for each tree held a renamed folder's subtree and reach for
// each rename): the reads' check asks for the trees after A's move and
// after B's, then the contents' pass for them again, each given anew once
// the plan has passed it, the ops between two moves sharing one: P's and
// R's reach, each its own; P's write and Q's edit, R's write and S's edit,
// one each, none the check's.
func TestABatchsPlanHoldsOneTreesReads(t *testing.T) {
	f := newFixture()
	f.page("A", nil, 1)
	for i, name := range []string{"P", "Q", "R", "S"} {
		f.content(f.page(name, nil, float64(i+2)).ID, "[[A]] x", 1)
	}
	links := &fakeLinks{recorder: f.rec, reached: true}
	if _, err := f.applyBatch(links, moveOp("A.md", "B.md"), writeOp("P.md", "[[A]] x"), editOp("Q.md", "x", "q"),
		moveOp("B.md", "C.md"), writeOp("R.md", "[[A]] x"), editOp("S.md", "x", "s")); err != nil {
		t.Fatal(err)
	}
	r := links.reads
	if len(r) != 6 || slices.Contains(r, nil) || r[1] == r[0] || r[3] != r[2] || r[5] != r[4] || r[4] == r[2] ||
		slices.Contains(r[:2], r[2]) || slices.Contains(r[:2], r[4]) {
		t.Errorf("Reached and Rewrite were given the reads %v; want P's reach and R's, then P's and Q's rewrites one, R's and "+
			"S's another, each new", r)
	}
}

// A delete between two content ops renews the plan's reads of the tree,
// as a move does (M9/P3 fix check 3, A's F3-1): a delete inside a renamed
// folder takes a node out of its subtree, which a reach made before still
// held, and a write after it read its page whole for a link the delete
// leaves dangling. F renamed G, then P edited, G/X deleted, Q edited: P's
// rewrite and Q's are given a read each.
func TestABatchsPlanReadsATreeAgainAfterADelete(t *testing.T) {
	f := newFixture()
	folder := f.page("F", nil, 1)
	f.page("X", &folder.ID, 1)
	for i, name := range []string{"P", "Q"} {
		f.content(f.page(name, nil, float64(i+2)).ID, "[[F/X]] x", 1)
	}
	links := &fakeLinks{recorder: f.rec}
	if _, err := f.applyBatch(links, moveOp("F.md", "G.md"), editOp("P.md", "x", "p"), deleteOp("G/X.md", false),
		editOp("Q.md", "x", "q")); err != nil {
		t.Fatal(err)
	}
	if r := links.reads; len(r) != 2 || r[0] == nil || r[1] == nil || r[1] == r[0] {
		t.Errorf("the rewrites were given the reads %v; want P's one, Q's another", r)
	}
}

// The plan's links are given the nodes the ops before frozen, moved with
// update_links false, whose links they leave (M9/P3 review B-M1): the
// reach of a page written after the moves, and the rewrite of one edited
// after them.
func TestABatchsPlanLeavesTheLinksToTheNodesItFroze(t *testing.T) {
	f := newFixture()
	f.page("A", nil, 1)
	x, p, q := f.page("X", nil, 2), f.page("P", nil, 3), f.page("Q", nil, 4)
	f.content(p.ID, "[[A]] p", 3)
	f.content(q.ID, "[[A]] q", 3)
	links := &fakeLinks{recorder: f.rec, subs: map[string]string{"[[A]]": "[[B]]"}, reached: true}
	if _, err := f.applyBatch(links, keepOp("X.md", "Y.md"), moveOp("A.md", "B.md"), writeOp("P.md", "[[B]] p"),
		editOp("Q.md", "q", "r")); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(links.frozen["Reached"], []uuid.UUID{x.ID}) || !slices.Equal(links.frozen["Rewrite"], []uuid.UUID{x.ID}) {
		t.Errorf("the plan's links were given frozen %v; want X for Reached and Rewrite", links.frozen)
	}
}

// The links are given, for each node a rename or move with update_links
// moves, the nodes the batch created under it after its last such op, in
// the window they follow a page's links in (M9 design 4.11; M9/P3 review
// B1-M1): the plan's reach of R, written after A's move and N's creation
// under it, and rewrite of Q, edited after them, N under A's; the
// participant's follow of P, edited before the move, N, M and K under A's;
// of Q, edited after it, none; the index's pages, those of the whole
// batch. Z, created elsewhere, is none of them (M9/P3 fix check 2, A's
// F2-1).
func TestABatchsLinksLeaveOutTheNodesItCreatedAfterAMove(t *testing.T) {
	f := newFixture()
	a := f.page("A", nil, 1)
	p, q, r := f.page("P", nil, 2), f.page("Q", nil, 3), f.page("R", nil, 4)
	f.content(p.ID, "[[A]] p", 1)
	f.content(q.ID, "[[A]] q", 1)
	f.content(r.ID, "[[A]] r", 1)
	part := &participant{recorder: f.rec}
	f.partakers = []app.Participant{part}
	links := &fakeLinks{recorder: f.rec, reached: true}
	out, err := f.applyBatch(links, editOp("P.md", "p", "p2"), moveOp("A.md", "B.md"), writeOp("Z.md", "z"), writeOp("B/N.md", "n"),
		editOp("Q.md", "q", "q2"), writeOn("R.md", "[[A]] r", tokenOf(r.ID, 1)), writeOp("B/M/K.md", "k"))
	if err != nil {
		t.Fatal(err)
	}
	n := out.Ops[3].Node.ID
	var nmk []uuid.UUID
	for _, c := range out.Ops[6].Created {
		nmk = append(nmk, c.Node.ID)
	}
	nmk = slices.SortedFunc(slices.Values(append(nmk, n)), uuid.UUID.Compare)
	eq := func(got map[uuid.UUID][]uuid.UUID, want map[uuid.UUID][]uuid.UUID) bool {
		return maps.EqualFunc(got, want, slices.Equal)
	}
	if got := links.created["Reached "+r.ID.String()]; !eq(got, map[uuid.UUID][]uuid.UUID{a.ID: {n}}) {
		t.Errorf("R's reach was given %v, want N after A", got)
	}
	if got := links.created["Rewrite "+q.ID.String()]; !eq(got, map[uuid.UUID][]uuid.UUID{a.ID: {n}}) {
		t.Errorf("Q's rewrite was given %v, want N after A", got)
	}
	if len(part.steps) != 1 || len(nmk) != 3 {
		t.Fatalf("the participants followed %d steps, created %v", len(part.steps), nmk)
	}
	b := part.steps[0].Batch
	if !eq(b.CreatedAfter, map[uuid.UUID][]uuid.UUID{a.ID: nmk}) || !eq(b.Written[p.ID].CreatedAfter, map[uuid.UUID][]uuid.UUID{a.ID: nmk}) ||
		!eq(b.Written[q.ID].CreatedAfter, map[uuid.UUID][]uuid.UUID{}) {
		t.Errorf("the participants were given %v, P's %v, Q's %v; want N, M and K after A, P's those, Q's none", b.CreatedAfter,
			b.Written[p.ID].CreatedAfter, b.Written[q.ID].CreatedAfter)
	}
}

// The plan's links share one read of its tree across the creations
// between the content ops after a rename (M9/P3 fix check 2, A's F2-1: a
// creation changes nothing those reads hold; 49 of them made the reads,
// and the reach, anew for each write after the rename of a folder of
// 100,000, 3.75 s and 15.2 GB): A renamed, then N1, P written, N2, Q
// edited, N3 and R written, Ns created at the root: the reads' check and
// the contents' pass are given one read, and none of the nodes created
// elsewhere, so that one reach serves them.
func TestABatchsPlanReadsATreeAcrossItsCreations(t *testing.T) {
	f := newFixture()
	a := f.page("A", nil, 1)
	for i, name := range []string{"P", "Q", "R"} {
		f.content(f.page(name, nil, float64(i+2)).ID, "[[A]] x", 1)
	}
	links := &fakeLinks{recorder: f.rec, reached: true}
	if _, err := f.applyBatch(links, moveOp("A.md", "B.md"), writeOp("N1.md", "n"), writeOp("P.md", "[[A]] x"), writeOp("N2.md", "n"),
		editOp("Q.md", "x", "q"), writeOp("N3.md", "n"), writeOp("R.md", "[[A]] x")); err != nil {
		t.Fatal(err)
	}
	r := links.reads
	if len(r) != 5 || r[0] == nil || slices.ContainsFunc(r, func(x *app.TreeReads) bool { return x != r[0] }) {
		t.Errorf("Reached and Rewrite were given the reads %v; want P's and R's reach, then P's, Q's and R's rewrites one", r)
	}
	for call, created := range links.created {
		if len(created) != 0 {
			t.Errorf("%s was given %v created after %s's rename; want none", call, created, a.ID)
		}
	}
}

// A batch's unit that finds the notebook other than its plan had it is
// made again once, from the read of the tree (M9/P3 design 3.6.2): a page
// written since, a node at a path that is another since, two names
// traded, and each op's compare of what it found: a content op's page, a
// delete's subtree, a creation's parent and a move's destination parent,
// each another since; a page gone between the plan's reads; then
// page.revision_mismatch at the first op that
// differs, a content op's with the version there and why as its edit's, a
// tree op's the notebook changed twice. A guard's refusal is answered as
// it is.
func TestABatchIsPlannedAgainOnce(t *testing.T) {
	f := newFixture()
	p := f.page("P", nil, 1)
	f.content(p.ID, "one two", 2)
	f.rec.before = map[string]func(){"LockContent in tx": func() { f.writeAs(p.ID, "one two three") }}
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("P.md", "two", "2")); err != nil ||
		f.store.contents[p.ID].Content != "one 2 three" || f.count("ListNodes") != 2 ||
		!strings.Contains(f.logs.String(), `msg="batch planned again"`) {
		t.Errorf("a batch after a write in between = %v, P %q after %v; want it made again on the new content", err,
			f.store.contents[p.ID].Content, f.rec.calls)
	}

	f = newFixture()
	p = f.page("P", nil, 1)
	f.content(p.ID, "one two", 2)
	f.rec.before = map[string]func(){"LockContent in tx": func() {
		f.writeAs(p.ID, "one two three")
		f.rec.before["LockContent in tx"] = func() { f.writeAs(p.ID, "one two three four") }
	}}
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("P.md", "two", "2")); opOf(err) != "0 page.revision_mismatch" ||
		mismatchOf(err) != "P.md kept changing "+p.ID.String()+":4" || f.count("ListNodes") != 2 {
		t.Errorf("a batch after two writes in between = %v (%s) after %v; want ops[0]'s 409 kept changing, planned twice",
			mismatchOf(err), opOf(err), f.rec.calls)
	}

	f = newFixture()
	a := f.page("A", nil, 1)
	p = f.page("P", nil, 2)
	f.content(p.ID, "one two", 2)
	f.rec.before = map[string]func(){"RootByKey in tx": func() {
		delete(f.store.nodes, a.ID)
		f.page("A", nil, 1) // another A since the plan
		f.rec.before["LockContent in tx"] = func() { f.writeAs(p.ID, "one two three") }
	}}
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, moveOp("A.md", "B.md"), writeOn("P.md", "new", tokenOf(p.ID, 2))); opOf(err) !=
		"1 page.revision_mismatch" || mismatchOf(err) != "P.md changed "+p.ID.String()+":3" || f.count("ListNodes") != 2 {
		t.Errorf("a batch whose page changed in its second unit = %v (%s) after %v; want ops[1]'s 409 changed", mismatchOf(err),
			opOf(err), f.rec.calls)
	}

	f = newFixture()
	a, b := f.page("A", nil, 1), f.page("B", nil, 2)
	swap := func() {
		x, y := f.store.nodes[a.ID], f.store.nodes[b.ID]
		x.Name, x.NameKey, y.Name, y.NameKey = y.Name, y.NameKey, x.Name, x.NameKey
		f.store.nodes[a.ID], f.store.nodes[b.ID] = x, y
	}
	f.rec.before = map[string]func(){"RootByKey in tx": func() {
		swap()
		f.rec.before["RootByKey in tx"] = swap
	}}
	_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, moveOp("A.md", "C.md"))
	var problem *shared.Error
	if opOf(err) != "0 page.revision_mismatch" || !errors.As(err, &problem) || !strings.Contains(problem.Detail, "twice") ||
		f.store.nodes[a.ID].Name != "A" || f.store.nodes[b.ID].Name != "B" || f.count("ListNodes") != 2 {
		t.Errorf("a batch whose names were traded twice = %v after %v; want ops[0]'s notebook changed twice, nothing moved", err,
			f.rec.calls)
	}

	// Each op's compare of the nodes it found (M9/P3 review C-I1), the
	// notebook changed at the unit's first lookup: a content op's page,
	// another at the revision the plan read.
	f = newFixture()
	a = f.page("A", nil, 1)
	f.content(a.ID, "one two", 1)
	var other domain.Node
	f.rec.before = map[string]func(){"RootByKey in tx": func() {
		delete(f.store.nodes, a.ID)
		delete(f.store.contents, a.ID)
		other = f.page("A", nil, 1)
		f.content(other.ID, "one two three", 1)
		f.rec.before["RootByKey in tx"] = func() {}
	}}
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("A.md", "two", "2")); err != nil ||
		f.store.contents[other.ID].Content != "one 2 three" || f.count("ListNodes") != 2 {
		t.Errorf("an edit whose page is another since the plan, at the same revision = %v, that page %q after %v; want the edit "+
			"planned again and done on it", err, f.store.contents[other.ID].Content, f.rec.calls)
	}

	// A delete's subtree, a child since the plan: refused as the delete
	// alone refuses it.
	f = newFixture()
	folder := f.page("F", nil, 1)
	var child domain.Node
	f.rec.before = map[string]func(){"RootByKey in tx": func() {
		child = f.page("Y", &folder.ID, 1)
		f.rec.before["RootByKey in tx"] = func() {}
	}}
	_, err = f.applyBatch(&fakeLinks{recorder: f.rec}, deleteOp("F.md", false))
	if _, trashed := f.store.trash[child.ID]; opOf(err) != "0 validation_failed ops[0].recursive" || trashed ||
		len(f.store.deletions) != 0 || f.count("ListNodes") != 2 {
		t.Errorf("a delete whose page has a child since the plan = %v (%s) after %v; want it planned again, ops[0]'s recursive "+
			"refused, nothing deleted", err, opOf(err), f.rec.calls)
	}

	// A creation's parent: F renamed G and another F made, the path F
	// another page's.
	f = newFixture()
	old := f.page("F", nil, 1)
	var now domain.Node
	replace := func() {
		g := f.store.nodes[old.ID]
		g.Name, g.NameKey = "G", "g"
		f.store.nodes[old.ID] = g
		now = f.page("F", nil, 9)
		f.rec.before["RootByKey in tx"] = func() {}
	}
	f.rec.before = map[string]func(){"RootByKey in tx": replace}
	out, err := f.applyBatch(&fakeLinks{recorder: f.rec}, writeOp("F/X.md", "x"))
	if err != nil || len(out.Ops) != 1 || f.store.nodes[out.Ops[0].Node.ID].ParentID == nil ||
		*f.store.nodes[out.Ops[0].Node.ID].ParentID != now.ID || f.count("ListNodes") != 2 {
		t.Errorf("a write under a parent another since the plan = %+v, %v after %v; want it planned again, X under the new F %s",
			out.Ops, err, f.rec.calls, now.ID)
	}

	// A move's destination parent, F replaced so: the move done under the
	// new F, and the rewrite of the page written before it given the move
	// to the new F, not to G (M9/P3 review C-S3).
	f = newFixture()
	a, old = f.page("A", nil, 1), f.page("F", nil, 2)
	w := f.page("W", nil, 3)
	f.content(w.ID, "see [[A]]", 2)
	seen := f.store.contents[w.ID]
	rewrite := &participant{recorder: f.rec}
	f.partakers = []app.Participant{rewrite}
	f.rec.before = map[string]func(){"RootByKey in tx": func() {
		replace()
		// The first unit's write of W undone by the second plan, as the
		// unit's rollback undoes it: the fakes keep it.
		f.rec.before["ListNodes"] = func() { f.store.contents[w.ID] = seen }
	}}
	_, err = f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("W.md", "see", "look"), moveOp("A.md", "F/A.md"))
	if err != nil || f.store.nodes[a.ID].ParentID == nil || *f.store.nodes[a.ID].ParentID != now.ID || f.count("ListNodes") != 2 ||
		len(rewrite.steps) != 1 {
		t.Fatalf("a move to a parent another since the plan = %v, A under %v after %v; want it planned again, A under the new F %s",
			err, f.store.nodes[a.ID].ParentID, f.rec.calls, now.ID)
	}
	for _, changes := range [][]domain.Change{rewrite.steps[0].Changes, rewrite.steps[0].Batch.Written[w.ID].Changes} {
		if len(changes) != 1 || changes[0].NodeID != a.ID || changes[0].After == nil || changes[0].After.ParentID == nil ||
			*changes[0].After.ParentID != now.ID {
			t.Errorf("the changes followed = %+v; want A's, to the new F %s, not to G %s", changes, now.ID, old.ID)
		}
	}

	// A page gone between the plan's reads of the tree and of its head or
	// its content is such a difference (M9/P3 review A-M2): the second
	// plan answers at the op its absence refuses, with its path.
	for _, read := range []string{"PageStates 1", "PageContentWithin"} {
		f = newFixture()
		p = f.page("P", nil, 1)
		f.content(p.ID, "p", 2)
		f.rec.before = map[string]func(){read: func() {
			delete(f.store.nodes, p.ID)
			delete(f.store.contents, p.ID)
		}}
		_, err = f.applyBatch(&fakeLinks{recorder: f.rec}, moveOp("P.md", "R.md"), editOp("R.md", "p", "q"))
		var missing *domain.PathNotFound
		if opOf(err) != "0 page.not_found" || !errors.As(err, &missing) || missing.Path != "P.md" || f.count("ListNodes") != 2 {
			t.Errorf("a page gone before %s = %v (%s) after %v; want it planned again, ops[0]'s P.md not found", read, err,
				opOf(err), f.rec.calls)
		}
	}

	f = newFixture()
	p = f.page("P", nil, 1)
	f.content(p.ID, "one two", 2)
	f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: domain.ErrLocked}}
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, moveOp("P.md", "Q.md")); opOf(err) != "0 page.locked" ||
		f.count("ListNodes") != 1 {
		t.Errorf("a batch a guard refuses = %s after %v; want ops[0]'s refusal as it is, planned once", opOf(err), f.rec.calls)
	}
}

// refuseOn refuses the steps of op with err, and lets the others by.
type refuseOn struct {
	op  domain.Operation
	err error
}

func (g refuseOn) GuardWrite(_ context.Context, s app.Step) error {
	if s.Operation == g.op {
		return g.err
	}
	return nil
}

// A refusal is answered at its op, whatever its code (M10/P1 design
// 3.22): a guard's page.title_taken, a code the plan checks, marked with
// its node, is ops[1]'s, the batch planned once and rolled back. The
// reverse: the unit's own page.title_taken, unmarked, a name taken since
// the plan, has the batch planned again, whose tree's pass answers it (a
// lone move: the fakes keep a rolled back unit's writes).
func TestABatchAnswersARefusalAtItsOperation(t *testing.T) {
	f := newFixture()
	p, q := f.page("P", nil, 1), f.page("Q", nil, 2)
	f.content(p.ID, "a", 2)
	f.guards = []app.WriteGuard{refuseOn{op: domain.OpRename, err: domain.Refuse(domain.ErrTitleTaken, q.ID)}}
	_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("P.md", "a", "b"), moveOp("Q.md", "R.md"))
	var r *domain.Refused
	if opOf(err) != "1 page.title_taken" || !errors.As(err, &r) || r.NodeID != q.ID || f.count("ListNodes") != 1 || !f.tx.rolledBack {
		t.Errorf("a batch a guard refuses at ops[1] = %s (%v) after %v; want ops[1]'s refusal of Q, planned once, rolled back",
			opOf(err), err, f.rec.calls)
	}

	f = newFixture()
	f.page("Q", nil, 2)
	f.rec.before = map[string]func(){"LockNotebook in tx": func() { f.page("R", nil, 3) }}
	_, err = f.applyBatch(&fakeLinks{recorder: f.rec}, moveOp("Q.md", "R.md"))
	var taken *domain.PathTaken
	if opOf(err) != "0 page.title_taken" || errors.As(err, &r) || !errors.As(err, &taken) || f.count("ListNodes") != 2 {
		t.Errorf("a batch whose move's name another took since its plan = %s (%v) after %v; want it planned again, ops[0]'s "+
			"PathTaken", opOf(err), err, f.rec.calls)
	}
}

// sized gives the page id a content of n bytes by its row alone, as its
// head tells it: none of its bytes is read here.
func (f *fixture) sized(id uuid.UUID, n int) {
	c := f.store.contents[id]
	c.ByteSize = n
	f.store.contents[id] = c
}

// A batch's plan reads the contents whose sizes it bounded, in their
// statements, within their sizes then (M9 Codex review D2): a page grown
// past its head since is read none of, a difference, planned again, as a
// page gone is; the second plan reads it at its size now. P grown from 1
// byte to 1,001 before its read: none read, then 1,001, the edit done on
// it. P, Q and R grown to 5 MiB each, 15 MiB: none read, then the reads'
// bound at the sizes now, page.batch_too_large at ops[2], no content read.
func TestABatchsReadsHoldTheirBoundOnWhatTheyRead(t *testing.T) {
	grow := func(f *fixture, to string, ids ...uuid.UUID) {
		grown := false
		f.rec.before = map[string]func(){"PageContentWithin": func() {
			if !grown {
				for _, id := range ids {
					f.writeAs(id, to)
				}
				grown = true
			}
		}}
	}
	f := newFixture()
	p := f.page("P", nil, 1)
	f.content(p.ID, "a", 2)
	grow(f, "a"+strings.Repeat("x", 1000), p.ID)
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("P.md", "a", "b")); err != nil || f.count("ListNodes") != 2 ||
		!slices.Equal(f.store.read, []int{0, 1001}) || f.store.contents[p.ID].Content != "b"+strings.Repeat("x", 1000) {
		t.Errorf("P grown before its read = %v, planned %d times, read %v, P %.10q; want none read, then all, the edit on it", err,
			f.count("ListNodes"), f.store.read, f.store.contents[p.ID].Content)
	}

	f = newFixture()
	var ids []uuid.UUID
	var ops []domain.BatchArgs
	for i, name := range []string{"P", "Q", "R"} {
		n := f.page(name, nil, float64(i))
		f.content(n.ID, "a", 2)
		ids, ops = append(ids, n.ID), append(ops, editOp(name+".md", "a", "b"))
	}
	grow(f, "a"+strings.Repeat("x", domain.MaxContentBytes-1), ids...)
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, ops...); opOf(err) != "2 page.batch_too_large" || f.count("ListNodes") != 2 ||
		!slices.Equal(f.store.read, []int{0}) {
		t.Errorf("P, Q and R grown to 15 MiB = %s, planned %d times, read %v; want the reads' bound at ops[2], none read",
			opOf(err), f.count("ListNodes"), f.store.read)
	}

	// P grown before each read, a page there each time: the edit's
	// mismatch with P's version then, why its page kept changing, as the
	// unit's second difference answers a page there (fix round 1, B-N1);
	// the log's difference its version, nothing read.
	f = newFixture()
	p = f.page("P", nil, 1)
	f.content(p.ID, "a", 2)
	var grown func() // once each read: the recorder's hook runs once
	grown = func() {
		f.writeAs(p.ID, f.store.contents[p.ID].Content+strings.Repeat("x", 1000))
		f.rec.before["PageContentWithin"] = grown
	}
	f.rec.before = map[string]func(){"PageContentWithin": grown}
	_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("P.md", "a", "b"))
	var mismatch *domain.VersionMismatch
	if !errors.As(err, &mismatch) || opOf(err) != "0 page.revision_mismatch" || mismatch.Why != domain.KeptChanging ||
		mismatch.Current != (domain.VersionToken{Node: p.ID, Revision: 4}) || f.count("ListNodes") != 2 || !slices.Equal(f.store.read, []int{0, 0}) ||
		!strings.Contains(f.logs.String(), "differs=version") {
		t.Errorf("P grown before each read = %v (%+v), planned %d times, read %v; want ops[0] page.revision_mismatch at P:4, kept "+
			"changing, none read, logged as its version", err, mismatch, f.count("ListNodes"), f.store.read)
	}
}

// The edit locks' precheck answers at the first op, in their order, that
// writes or deletes a page being edited (M9/P3 design 3.4.5; review
// A-M1): a delete's subtree, the deletions merged into it among them, in
// the delete's words, every page of it held named; a content op's, its
// page. Before any parse, the unit not begun.
func TestABatchsLockPrecheckIsInItsOpsOrder(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ops   []domain.BatchArgs
		want  string // the op and its code
		locks []string
	}{
		{"a delete, then an edit", []domain.BatchArgs{deleteOp("F/Q.md", false), editOp("P.md", "p", "q")}, "0 page.locked",
			[]string{"Q"}},
		{"a delete of a folder with a page held", []domain.BatchArgs{deleteOp("F.md", true), editOp("P.md", "p", "q")},
			"0 page.locked", []string{"Q", "R"}},
		{"a deletion merged into a later delete", []domain.BatchArgs{deleteOp("F/Q.md", false), deleteOp("F.md", true),
			editOp("P.md", "p", "q")}, "1 page.locked", []string{"Q", "R"}},
		{"an edit, then a delete", []domain.BatchArgs{editOp("P.md", "p", "q"), deleteOp("F/Q.md", false)}, "0 page.locked",
			[]string{"P"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			folder := f.page("F", nil, 1)
			q, r := f.page("Q", &folder.ID, 1), f.page("R", &folder.ID, 2)
			p := f.page("P", nil, 2)
			f.content(p.ID, "p", 1)
			bob := uuid.NewV7()
			f.names[f.alice], f.names[bob] = "Alice", "Bob"
			f.session(q.ID, bob, now().Add(time.Minute))
			f.session(r.ID, f.alice, now().Add(time.Minute))
			f.session(p.ID, f.alice, now().Add(time.Minute))
			_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, tt.ops...)
			byID := map[uuid.UUID]string{q.ID: "Q", r.ID: "R", p.ID: "P"}
			var named []string
			var e *shared.Error
			if errors.As(err, &e) && e.Lock != nil {
				for _, h := range append([]shared.LockHolder{*e.Lock}, e.Locks...) {
					named = append(named, byID[h.PageID])
				}
				named = slices.Compact(named)
			}
			if opOf(err) != tt.want || !slices.Equal(named, tt.locks) || f.took() || f.called("LockNotebook in tx") {
				t.Errorf("the batch = %s naming %v after %v; want %s naming %v, before any parse and the unit", opOf(err), named,
					f.rec.calls, tt.want, tt.locks)
			}
		})
	}
}

// A batch's bounds (M9/P3 design 3.8): its items in the tree's pass, past
// them nothing read; the contents its edits read, by their sizes before
// any is read; the heads alone of the pages its writes replace; the edit
// locks before any parse; the unit's pages and bytes by page and hash, a
// lone page's copies one page; the links of its final contents.
func TestABatchsBounds(t *testing.T) {
	f := newFixture()
	p := f.page("P", nil, 1)
	f.content(p.ID, "p", 1)
	d := f.page("D", nil, 2)
	for i := range domain.MaxUnitNodes {
		f.page(fmt.Sprintf("C%d", i), &d.ID, float64(i))
	}
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("P.md", "missing", "x"), deleteOp("D.md", true)); opOf(err) !=
		"1 page.batch_too_large" || f.called("PageStates 1") || f.called("PageContentWithin") {
		t.Errorf("a batch past its items = %s after the reads %v; want ops[1]'s 422 in the tree's pass, nothing read", opOf(err),
			f.called("PageStates 1"))
	}

	for _, tt := range []struct {
		name  string
		bytes int
		want  string
	}{
		{"at the reads' bound", domain.MaxBatchReadBytes / 2, "0 validation_failed ops[0].old_string"},
		{"a byte past it", domain.MaxBatchReadBytes/2 + 1, "1 page.batch_too_large"},
	} {
		f := newFixture()
		p, q := f.page("P", nil, 1), f.page("Q", nil, 2)
		f.sized(p.ID, domain.MaxBatchReadBytes/2)
		f.sized(q.ID, tt.bytes)
		_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("P.md", "a", "b"), editOp("Q.md", "a", "b"))
		if opOf(err) != tt.want || f.called("PageContentWithin") != (tt.want != "1 page.batch_too_large") {
			t.Errorf("%s: %s, the contents read %v; want %s", tt.name, opOf(err), f.called("PageContentWithin"), tt.want)
		}
	}

	f = newFixture()
	var ops []domain.BatchArgs
	small := strings.Repeat("s", 1<<10)
	for i := range 50 {
		big, same := f.page(fmt.Sprintf("B%d", i), nil, float64(i)), f.page(fmt.Sprintf("S%d", i), nil, float64(50+i))
		f.sized(big.ID, domain.MaxContentBytes)
		f.content(same.ID, small, 1)
		ops = append(ops, writeOn(big.Name+".md", "small", tokenOf(big.ID, 1)), writeOp(same.Name+".md", small))
	}
	if out, err := f.applyBatch(&fakeLinks{recorder: f.rec}, ops...); err != nil || f.called("PageContentWithin") || len(out.Ops) != 100 {
		t.Errorf("100 writes of 50 pages of 5 MiB and 50 of their bytes = %v, the contents read %v; want them done on their heads",
			err, f.called("PageContentWithin"))
	}

	f = newFixture()
	ops = nil
	for i := range 3 {
		n := f.page(fmt.Sprintf("P%d", i), nil, float64(i))
		f.sized(n.ID, domain.MaxContentBytes)
		ops = append(ops, writeOp(n.Name+".md", "other bytes"))
	}
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, ops...); opOf(err) != "0 page.title_taken" {
		t.Errorf("three writes over 5 MiB pages of other bytes = %s, want ops[0]'s PathTaken (review A3-N1)", opOf(err))
	}

	f = newFixture()
	f.page("X", nil, 0)
	p = f.page("P", nil, 1)
	f.content(p.ID, "one two", 1)
	f.names[f.alice] = "Alice"
	f.session(p.ID, f.alice, now().Add(time.Minute))
	_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, deleteOp("X.md", false), editOp("P.md", "two", "2"))
	var paths *domain.LockPaths
	if page, holder, locked := lockOf(err, f.names); opOf(err) != "1 page.locked" || !locked || page != p.ID || holder != "Alice" ||
		f.took() || !errors.As(err, &paths) || paths.Paths[p.ID] != "P.md" {
		t.Errorf("a batch writing a page in an edit session = %v after %v; want ops[1]'s page.locked with its path, nothing parsed",
			err, f.rec.calls)
	}

	f = newFixture()
	a := f.page("A", nil, 1)
	p = f.page("P", nil, 2)
	big := "[[A]]" + strings.Repeat("x", domain.MaxContentBytes-6) + "a"
	f.content(p.ID, big, 1)
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec, subs: map[string]string{"[[A]]": "[[B]]"}}, moveOp("A.md", "B.md"),
		editOp("P.md", "xa", "xb")); err != nil || !strings.HasPrefix(f.store.contents[p.ID].Content, "[[B]]") ||
		f.store.nodes[a.ID].Name != "B" {
		t.Errorf("a lone page of 5 MiB rewritten and edited = %v; want it done, its three copies one page", err)
	}

	f = newFixture()
	f.page("A", nil, 1)
	p = f.page("P", nil, 2)
	f.content(p.ID, big, 1)
	links := &fakeLinks{recorder: f.rec, subs: map[string]string{"[[A]]": "[[B]]"}}
	takes := func() int {
		return len(slices.DeleteFunc(slices.Clone(f.rec.calls), func(c string) bool { return !strings.HasPrefix(c, "Take ") }))
	}
	if _, err := f.applyBatch(links, moveOp("A.md", "B.md"), editOp("P.md", "xa", "xb"), moveOp("B.md", "C.md"),
		editOp("P.md", "xb", "xc")); opOf(err) != "3 page.batch_too_large" || takes() != 2 {
		t.Errorf("a plan past its parses' bound = %s after %d takes; want ops[3]'s 422, the parse past it not done", opOf(err),
			takes())
	}

	for _, tt := range []struct {
		name  string
		links int
		want  string
	}{
		{"at the unit's links", domain.MaxUnitLinks, ""},
		{"a link past them", domain.MaxUnitLinks + 1, "-1 page.batch_too_large"},
	} {
		f := newFixture()
		p, q := f.page("P", nil, 1), f.page("Q", nil, 2)
		_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, writeOn("P.md", strings.Repeat("[[x]]", domain.MaxUnitLinks/2), tokenOf(p.ID, 1)),
			writeOn("Q.md", strings.Repeat("[[x]]", tt.links-domain.MaxUnitLinks/2), tokenOf(q.ID, 1)))
		if got := opOf(err); err == nil && tt.want != "" || err != nil && got != tt.want {
			t.Errorf("%s: %v, want %q", tt.name, err, tt.want)
		}
	}
}

// What a batch answers (M9/P3 design 3.6.3): a content op of a page a
// later op deleted, the version its composed write left and its path at
// the op; each op changed as its tool would at its step, though the batch
// changed nothing; a batch that moved a page and moved it back drops its
// item and its changeset.
func TestABatchAnswersEachOpAtItsStep(t *testing.T) {
	f := newFixture()
	p := f.page("P", nil, 1)
	f.content(p.ID, "a", 2)
	out, err := f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("P.md", "a", "b"), deleteOp("P.md", false))
	if err != nil || out.Ops[0].Version != (domain.VersionToken{Node: p.ID, Revision: 3}) || out.Ops[0].Path != "P.md" ||
		out.Ops[1].Path != "P.md" || out.Ops[1].Pages != 1 || out.Outcome.ChangesetID == (uuid.UUID{}) {
		t.Errorf("an edit, then a delete, of P = %+v, %v; want the edit's version 3 at P.md", out.Ops, err)
	}

	f = newFixture()
	p = f.page("P", nil, 1)
	f.content(p.ID, "a", 2)
	out, err = f.applyBatch(&fakeLinks{recorder: f.rec}, editOp("P.md", "a", "b"), editOp("P.md", "b", "a"))
	if err != nil || !out.Ops[0].Changed || !out.Ops[1].Changed || out.Outcome.ChangesetID != (uuid.UUID{}) ||
		out.Ops[1].Version != (domain.VersionToken{Node: p.ID, Revision: 2}) || len(f.store.changesets) != 0 {
		t.Errorf("an edit and its undoing = %+v, %v; want both changed at their step, no changeset", out, err)
	}

	f = newFixture()
	p = f.page("P", nil, 1)
	out, err = f.applyBatch(&fakeLinks{recorder: f.rec}, moveOp("P.md", "Q.md"), moveOp("Q.md", "P.md"))
	if err != nil || !out.Ops[0].Changed || !out.Ops[1].Changed || out.Outcome.ChangesetID != (uuid.UUID{}) ||
		!f.called("DropUnchanged in tx") || len(f.store.changesets) != 0 || len(f.store.recorded) != 0 || out.Ops[1].Path != "P.md" {
		t.Errorf("a rename and its undoing = %+v, %v after %v; want both changed, no item, no changeset", out, err, f.rec.calls)
	}
}

// refusing is a participant that refuses with err.
type refusing struct{ err error }

func (r refusing) Participate(context.Context, app.Step, app.Appender) error { return r.err }

// Each error of an op is an OpFailed at it, of its tool; one of the batch
// as a whole is at none (M9/P3 design 3.9.2); the deferred rewrite's at the
// first op that renames or moves with update_links, a move that changes
// nothing not one (M9/P3 review C-N3).
func TestABatchsErrorsAreAtTheirOps(t *testing.T) {
	for _, tt := range []struct {
		name string
		ops  []domain.BatchArgs
		want string
		tool domain.OpKind
	}{
		{"a page not found", []domain.BatchArgs{moveOp("P.md", "Q.md"), editOp("Missing.md", "a", "b")}, "1 page.not_found",
			domain.OpKindEdit},
		{"a name taken", []domain.BatchArgs{writeOp("New.md", ""), moveOp("New.md", "P.md")}, "1 page.title_taken", domain.OpKindMove},
		{"a delete of what it created", []domain.BatchArgs{writeOp("N.md", ""), deleteOp("N.md", false)},
			"1 validation_failed ops[1]", domain.OpKindDelete},
		{"an old_string not there", []domain.BatchArgs{writeOp("P.md", "p"), editOp("P.md", "x", "y")},
			"1 validation_failed ops[1].old_string", domain.OpKindEdit},
		{"no op", nil, "-1 validation_failed ops", ""},
	} {
		f := newFixture()
		p := f.page("P", nil, 1)
		f.content(p.ID, "p", 1)
		_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, tt.ops...)
		var failed *domain.OpFailed
		if opOf(err) != tt.want || errors.As(err, &failed) && failed.Tool != tt.tool {
			t.Errorf("%s: %s, %v; want %s of %s", tt.name, opOf(err), err, tt.want, tt.tool)
		}
	}
	f := newFixture()
	f.page("A", nil, 1)
	f.page("B", nil, 2)
	f.partakers = []app.Participant{refusing{shared.NewError(shared.KindConflict, "linking.pages_locked", "")}}
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, moveOp("A.md", "A.md"), moveOp("B.md", "C.md")); opOf(err) !=
		"1 linking.pages_locked" {
		t.Errorf("a move to its own path, then the rewrite's refusal = %s, want it at ops[1], the move that changes", opOf(err))
	}

	f = newFixture()
	f.grant(domain.ActionRead)
	f.auth.forbidden[domain.ActionBatch] = true
	if _, err := f.batcher(&fakeLinks{recorder: f.rec}).Execute(f.asAlice(), f.eng, app.PathBatch{Ops: []domain.BatchArgs{
		writeOp("N.md", "n"),
	}, Message: "m"}, agent); opOf(err) != "-1 forbidden" {
		t.Errorf("a batch of a reader = %s, want 403 at no op", opOf(err))
	}
}

// A batch's base_version of another page than its path's answers the
// version of the page there, read once the plan chose the error (Codex
// review R2): the tree's pass holds no revision. A write's and an edit's
// alike, A at its third revision; nothing written, no unit run. A page
// deleted between the tree's read and that version's is a difference:
// planned again, the path not found at ops[0].
func TestABatchsMismatchOfAnotherPageTellsItsVersion(t *testing.T) {
	edit := editOp("A.md", "a", "b")
	for _, op := range []domain.BatchArgs{writeOn("A.md", "x", nil), edit} {
		f := newFixture()
		a, b := f.page("A", nil, 1), f.page("B", nil, 2)
		f.content(a.ID, "a", 3)
		op.Base = tokenOf(b.ID, 1)
		_, err := f.applyBatch(&fakeLinks{recorder: f.rec}, op)
		if got, want := mismatchOf(err), "A.md other node "+a.ID.String()+":3"; got != want || opOf(err) != "0 page.revision_mismatch" ||
			f.store.contents[a.ID].Content != "a" || f.called("LockNotebook in tx") {
			t.Errorf("a %s on B's token = %s, %s; want %s at ops[0], nothing written", op.Op, opOf(err), got, want)
		}
	}
	f := newFixture()
	a, b := f.page("A", nil, 1), f.page("B", nil, 2)
	f.rec.before = map[string]func(){"PageStates 1": func() {
		delete(f.store.contents, a.ID)
		delete(f.store.nodes, a.ID)
	}}
	if _, err := f.applyBatch(&fakeLinks{recorder: f.rec}, writeOn("A.md", "x", tokenOf(b.ID, 1))); opOf(err) != "0 page.not_found" ||
		f.count("ListNodes") != 2 {
		t.Errorf("A deleted before its version's read = %s after %v; want ops[0]'s page.not_found, planned again", opOf(err),
			f.rec.calls)
	}
}

// tenthBudget is a parse budget of capacity bytes whose holds keep a tenth
// of their bytes once they keep facts, as the server's does: a take past
// what is left waits for its context's end.
type tenthBudget struct {
	capacity, held int
}

func (b *tenthBudget) Take(ctx context.Context, n int) (app.BudgetHold, error) {
	if b.held+n > b.capacity {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	b.held += n
	return &tenthHold{budget: b, n: n}, nil
}

type tenthHold struct {
	budget *tenthBudget
	n      int
	done   bool
}

func (h *tenthHold) KeepFacts(app.Facts) {
	kept := h.n / 10
	h.budget.held -= h.n - kept
	h.n = kept
}

func (b *tenthBudget) Takes(n int) (int, int) { return n, b.capacity }

func (h *tenthHold) Held() int {
	if h.done {
		return 0
	}
	return h.n
}

func (h *tenthHold) Release() {
	if !h.done {
		h.done = true
		h.budget.held -= h.n
	}
}

// reader is a participant that reads back a content of n bytes, as the
// deferred rewrite does, taking them from budget without waiting.
type reader struct {
	budget app.ParseBudget
	n      int
	took   bool
}

func (r *reader) Participate(ctx context.Context, _ app.Step, _ app.Appender) error {
	waited, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	hold, err := r.budget.Take(waited, r.n)
	if err != nil {
		return err
	}
	hold.Release()
	r.took = true
	return nil
}

// A batch's parses on a budget of a page's bytes (M9/P3 design 3.4.5,
// review A2-M2): the plan's rewrite of a 5 MiB page reads it back with the
// whole budget, its own facts' shares given back first; the deferred
// rewrite reads back a 5 MiB page with the shares of the final contents
// given back, all of them.
func TestABatchsParsesFindTheBudget(t *testing.T) {
	big := "[[A]]" + strings.Repeat("x", domain.MaxContentBytes-6) + "a"
	f := newFixture()
	f.page("A", nil, 1)
	p := f.page("P", nil, 2)
	f.content(p.ID, big, 1)
	budget := &tenthBudget{capacity: domain.MaxContentBytes}
	f.grant(domain.ActionBatch)
	parser := app.NewContentParser(f.writer(), f.md, budget, 50*time.Millisecond)
	batch := app.NewApplyBatch(f.writer(), parser, f.store, f.store, f.store, &fakeLinks{recorder: f.rec,
		subs: map[string]string{"[[A]]": "[[B]]"}}, app.NewLockHolders(f.store, f.names), f.notebooks, f.auth, 0, 0, 0, f.clock, f.logger())
	ops := []domain.BatchArgs{moveOp("A.md", "B.md"), editOp("P.md", "xa", "xb")}
	if _, err := batch.Execute(f.asAlice(), f.eng, app.PathBatch{Ops: ops, Message: "m"}, agent); err != nil || budget.held != 0 {
		t.Errorf("a 5 MiB page rewritten and edited on a 5 MiB budget = %v, %d held after; want it done, nothing held", err,
			budget.held)
	}

	f = newFixture()
	f.page("A", nil, 1)
	p, q := f.page("P", nil, 2), f.page("Q", nil, 3)
	f.content(p.ID, "see [[A]]", 1)
	f.content(q.ID, "q", 1)
	budget = &tenthBudget{capacity: domain.MaxContentBytes}
	r := &reader{budget: budget, n: domain.MaxContentBytes}
	f.partakers = []app.Participant{r}
	f.grant(domain.ActionBatch)
	parser = app.NewContentParser(f.writer(), f.md, budget, 50*time.Millisecond)
	batch = app.NewApplyBatch(f.writer(), parser, f.store, f.store, f.store, &fakeLinks{recorder: f.rec},
		app.NewLockHolders(f.store, f.names), f.notebooks, f.auth, 0, 0, 0, f.clock, f.logger())
	ops = []domain.BatchArgs{writeOn("P.md", "see [[A]] "+strings.Repeat("p", 3<<20), tokenOf(p.ID, 1)),
		writeOn("Q.md", strings.Repeat("q", 1<<20), tokenOf(q.ID, 1)), moveOp("A.md", "B.md")}
	if _, err := batch.Execute(f.asAlice(), f.eng, app.PathBatch{Ops: ops, Message: "m"}, agent); err != nil || !r.took {
		t.Errorf("a rewrite's read back of 5 MiB after a batch wrote 3 and 1 MiB = %v; want the budget whole for it", err)
	}
}

// A batch's plan judges the contract before its reads and its lock (M10/P1
// design 3.22): a writer's [move raw, delete AGENTS] is refused at ops[0],
// page.source_admin_only of raw, as separate calls would be first; an
// admin's at ops[1], page.protected of AGENTS; AGENTS held by Bob's
// session changes neither, the plan before the edit locks' precheck, which
// a writer's [move raw, edit AGENTS] would meet at ops[1]; the
// unit not begun, page.manage_source asked once by the plan. A writer's
// [move raw, edit Big], Big past MaxBatchReadBytes, is ops[0]'s 403 with
// no content read; Big gone after the plan, ops[0]'s still, not planned
// again. The authorizer's ErrNotVisible is the notebook's 404; its
// database error is as it is, at no op. An admin's lone move asks the plan
// and the guard in the unit, once each.
func TestABatchsPlanRefusesBeforeItsLock(t *testing.T) {
	type wiki struct {
		f                *fixture
		raw, agents, big domain.Node
	}
	newWiki := func(held bool) wiki {
		f := newFixture()
		w := wiki{f: f, raw: f.page("raw", nil, 1), agents: f.page("AGENTS", nil, 2), big: f.page("Big", nil, 4)}
		f.page("Notes", nil, 3)
		f.content(w.agents.ID, "# AGENTS\n", 1)
		f.sized(w.big.ID, domain.MaxBatchReadBytes+1)
		f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: w.raw.ID}}
		f.guards = []app.WriteGuard{app.NewModeGuard(f.auth, f.store)}
		if held {
			f.session(w.agents.ID, uuid.NewV7(), now().Add(time.Minute))
		}
		return w
	}
	writer := func(f *fixture) { f.auth.forbidden[domain.ActionManageSource] = true }
	admin := func(f *fixture) { f.grant(domain.ActionManageSource) }
	refused := func(f *fixture) []domain.BatchArgs {
		return []domain.BatchArgs{moveOp("raw.md", "Notes/raw.md"), deleteOp("AGENTS.md", false)}
	}
	reading := func(f *fixture) []domain.BatchArgs {
		return []domain.BatchArgs{moveOp("raw.md", "Notes/raw.md"), editOp("Big.md", "a", "b")}
	}
	editing := func(f *fixture) []domain.BatchArgs {
		return []domain.BatchArgs{moveOp("raw.md", "Notes/raw.md"), editOp("AGENTS.md", "AGENTS", "Agents")}
	}
	dbErr := errors.New("db down")
	failing := func(f *fixture) { f.auth.failing = map[shared.Action]error{domain.ActionManageSource: dbErr} }
	for _, tt := range []struct {
		name  string
		held  bool
		role  func(*fixture)
		ops   func(*fixture) []domain.BatchArgs
		after func(wiki) // after the plan, before its reads
		want  string
		of    func(wiki) uuid.UUID // the node the refusal is about, nil for no refusal
	}{
		{"a writer's", false, writer, refused, nil, "0 page.source_admin_only", func(w wiki) uuid.UUID { return w.raw.ID }},
		{"an admin's", false, admin, refused, nil, "1 page.protected", func(w wiki) uuid.UUID { return w.agents.ID }},
		{"a writer's, AGENTS held", true, writer, refused, nil, "0 page.source_admin_only", func(w wiki) uuid.UUID { return w.raw.ID }},
		{"an admin's, AGENTS held", true, admin, refused, nil, "1 page.protected", func(w wiki) uuid.UUID { return w.agents.ID }},
		{"a writer's, AGENTS edited and held", true, writer, editing, nil, "0 page.source_admin_only",
			func(w wiki) uuid.UUID { return w.raw.ID }},
		{"a writer's, Big past the reads' bound", false, writer, reading, nil, "0 page.source_admin_only",
			func(w wiki) uuid.UUID { return w.raw.ID }},
		{"a writer's, Big gone after the plan", false, writer, reading, func(w wiki) {
			delete(w.f.store.nodes, w.big.ID)
			delete(w.f.store.contents, w.big.ID)
		}, "0 page.source_admin_only", func(w wiki) uuid.UUID { return w.raw.ID }},
		{"one who lost the notebook", false, func(*fixture) {}, refused, nil, "-1 notebook.not_found", nil},
		{"the authorizer's database error", false, failing, refused, nil, "-1 db down", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWiki(tt.held)
			tt.role(w.f)
			if tt.after != nil {
				w.f.rec.before = map[string]func(){"Authorize page.manage_source": func() { tt.after(w) }}
			}
			_, err := w.f.applyBatch(&fakeLinks{recorder: w.f.rec}, tt.ops(w.f)...)
			var r *domain.Refused
			var op *domain.OpFailed
			switch {
			case opOf(err) != tt.want:
				t.Errorf("the batch = %s (%v), want %s", opOf(err), err, tt.want)
			case tt.of != nil && (!errors.As(err, &r) || r.NodeID != tt.of(w)):
				t.Errorf("the batch's refusal = %#v, want one of %s", err, tt.of(w))
			case tt.want == "-1 db down" && (!errors.Is(err, dbErr) || errors.As(err, &op)):
				t.Errorf("the batch = %v, want the database's error at no op", err)
			}
			if w.f.called("LockNotebook in tx") || w.f.called("PageContentWithin") || w.f.count("Authorize page.manage_source") != 1 ||
				w.f.count("ListNodes") != 1 {
				t.Errorf("the batch called %v; want page.manage_source asked once, planned once, no content read, no unit", w.f.rec.calls)
			}
		})
	}

	w := newWiki(false)
	admin(w.f)
	_, err := w.f.applyBatch(&fakeLinks{recorder: w.f.rec}, moveOp("raw.md", "Notes/raw.md"))
	calls := w.f.rec.calls
	if at := slices.Index(calls, "ListNodes"); err != nil || at < 0 || at+1 >= len(calls) || calls[at+1] != "StateOf" ||
		w.f.count("Authorize page.manage_source") != 1 || w.f.count("Authorize page.manage_source in tx") != 1 {
		t.Errorf("an admin's move of raw = %v after %v; want it written, the state read unlocked after the tree, asked by the "+
			"plan and by the guard in the unit", err, calls)
	}
}
