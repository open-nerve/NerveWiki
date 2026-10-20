package domain_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// notebook is a tree for a plan's tests: its nodes, and their ids by path
// as written ("A/B.md" a page, "A/x.png" an attachment).
type notebook struct {
	nodes []domain.Node
	ids   map[string]uuid.UUID
}

// newNotebook is the tree of paths, each one's parents made as pages when
// not listed before it.
func newNotebook(paths ...string) *notebook {
	nb := &notebook{ids: map[string]uuid.UUID{}}
	for _, p := range paths {
		nb.add(p)
	}
	return nb
}

// add makes the node at path, and its parents, unless there.
func (nb *notebook) add(path string) uuid.UUID {
	if id, ok := nb.ids[path]; ok {
		return id
	}
	segments := strings.Split(path, "/")
	var parent *uuid.UUID
	if len(segments) > 1 {
		parent = new(nb.add(strings.Join(segments[:len(segments)-1], "/") + ".md"))
	}
	name, kind := segments[len(segments)-1], domain.KindAsset
	if strings.HasSuffix(name, ".md") {
		name, kind = strings.TrimSuffix(name, ".md"), domain.KindPage
	}
	n := domain.Node{ID: uuid.NewV7(), ParentID: parent, Kind: kind, Name: name, NameKey: shared.TitleKey(name),
		SortOrder: float64(len(nb.nodes) + 1)}
	nb.nodes, nb.ids[path] = append(nb.nodes, n), n.ID
	return n.ID
}

// id is the node at path.
func (nb *notebook) id(t *testing.T, path string) uuid.UUID {
	t.Helper()
	id, ok := nb.ids[path]
	if !ok {
		t.Fatalf("no node at %s", path)
	}
	return id
}

// planOf is ops planned on nb's tree, each write's ids made, a UUIDv7 a
// segment of its path.
func planOf(t *testing.T, nb *notebook, ops ...domain.BatchArgs) (*domain.BatchPlan, [][]uuid.UUID) {
	t.Helper()
	steps, err := domain.CheckBatch(ops, "m")
	if err != nil {
		t.Fatalf("CheckBatch: %v", err)
	}
	ids := make([][]uuid.UUID, len(steps))
	for i, s := range steps {
		if s.Kind == domain.OpKindWrite {
			for range s.Path.Keys {
				ids[i] = append(ids[i], uuid.NewV7())
			}
		}
	}
	return domain.PlanBatch(nb.nodes, steps, ids), ids
}

// failed is the plan's failure as a refusal, its code "" for none.
func failed(t *testing.T, p *domain.BatchPlan) refusal {
	t.Helper()
	if p.Failed == nil {
		return refusal{at: -1}
	}
	return refusalOf(t, p.Failed)
}

// tokenOf is a version token of id, as an op takes it.
func tokenOf(id uuid.UUID, revision int) *string {
	return str(domain.VersionToken{Node: id, Revision: revision}.String())
}

// The tree's pass refuses each of P2's steps as its tool does, at the op,
// its fields at ops[i] (M9/P3 design 3.4.2): a write's creation, a path
// that names nothing, a token, a move's parent, name and place, a
// delete's children.
func TestPlanBatch(t *testing.T) {
	deep := "D1/D2/D3/D4/D5/D6/D7/D8/D9/D10.md"
	nb := newNotebook("A.md", "A/B.md", "A/B/C.md", "A/x.png", "F.md", "F/G.md", "T.md.md", "H.md", "H/I.md", deep)
	ok := writeOp("Z.md", "z")
	for _, tt := range []struct {
		name string
		op   domain.BatchArgs
		want refusal
		is   func(error) bool // what else the error is, nil for nothing
	}{
		{"a write through an attachment", writeOp("A/x.png/y.md", "y"), refusal{"validation_failed", "ops[1].path", "not_allowed", 1}, nil},
		{"a write's missing ancestor ending with .md", writeOp("Q.md/R.md", "r"), refusal{"validation_failed", "ops[1].path", "not_allowed", 1}, nil},
		{"a write where an attachment has the name", writeOp("A/x.png.md", "x"), refusal{"page.title_taken", "", "", 1},
			func(err error) bool {
				var e *domain.PathTaken
				return errors.As(err, &e) && e.Asset && e.Path == "A/x.png.md"
			}},
		{"a write too deep", writeOp("E1/E2/E3/E4/E5/E6/E7/E8/E9/E10/E11.md", "e"), refusal{"page.too_deep", "", "", 1}, nil},
		{"an edit of nothing", editOp("Nope.md", "o", "n"), refusal{"page.not_found", "", "", 1}, nil},
		{"an edit of a page titled with .md", editOp("T.md", "o", "n"), refusal{"page.not_found", "", "", 1},
			func(err error) bool {
				var e *domain.PathNotFound
				return errors.As(err, &e) && e.Hint == domain.TitleHint("T.md.md")
			}},
		{"a delete of a page at an attachment's path", deleteOp("A/B"), refusal{"page.not_found", "", "", 1},
			func(err error) bool {
				var e *domain.PathNotFound
				return errors.As(err, &e) && e.Hint == domain.PageHint("A/B.md")
			}},
		{"a token of another page", with(editOp("A.md", "o", "n"), func(o *domain.BatchArgs) { o.Base = tokenOf(nb.id(t, "F.md"), 1) }),
			refusal{"page.revision_mismatch", "", "", 1},
			func(err error) bool {
				var e *domain.VersionMismatch
				return errors.As(err, &e) && e.Why == domain.OtherNode && e.Current.Node == nb.id(t, "A.md")
			}},
		{"a move of nothing", moveOp("Nope.md", "N.md"), refusal{"page.not_found", "", "", 1}, nil},
		{"a move under an attachment", moveOp("F.md", "A/x.png/F.md"), refusal{"validation_failed", "ops[1].to", "not_allowed", 1}, nil},
		{"a move under nothing", moveOp("F.md", "Nope/F.md"), refusal{"page.not_found", "", "", 1},
			func(err error) bool {
				var e *domain.PathNotFound
				return errors.As(err, &e) && e.ParentOf == "ops[1].to" && e.Path == "Nope.md" && e.Hint == domain.MoveParentHint
			}},
		{"a move under a parent written as a page's path", moveOp("F.md", "Nope.md/F.md"), refusal{"validation_failed", "ops[1].to", "not_allowed", 1}, nil},
		{"an attachment's extension lost", moveOp("A/x.png", "A/x"), refusal{"validation_failed", "ops[1].to", "not_allowed", 1}, nil},
		{"a rename to a name taken", moveOp("F.md", "A.md"), refusal{"page.title_taken", "", "", 1},
			func(err error) bool {
				var e *domain.PathTaken
				return errors.As(err, &e) && !e.Asset && e.Path == "A.md"
			}},
		{"a move under itself", moveOp("A.md", "A/B/A.md"), refusal{"page.cycle", "", "", 1}, nil},
		{"a move to a name taken", moveOp("F/G.md", "A/B.md"), refusal{"page.title_taken", "", "", 1},
			func(err error) bool { var e *domain.PathTaken; return errors.As(err, &e) && e.Path == "A/B.md" }},
		{"a move to an attachment's name", moveOp("F/G.md", "A/x.png.md"), refusal{"page.title_taken", "", "", 1},
			func(err error) bool {
				var e *domain.PathTaken
				return errors.As(err, &e) && e.Asset && e.Path == "A/x.png.md"
			}},
		{"a move too deep", moveOp("H.md", strings.TrimSuffix(deep, ".md")+"/H.md"), refusal{"page.too_deep", "", "", 1}, nil},
		{"a delete of nothing", deleteOp("Nope.md"), refusal{"page.not_found", "", "", 1}, nil},
		{"a delete of a page with children", deleteOp("A.md"), refusal{"validation_failed", "ops[1].recursive", "required", 1},
			func(err error) bool {
				var e *domain.Recursive
				return errors.As(err, &e) && e.Pages == 2 && e.Assets == 1
			}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := planOf(t, nb, ok, tt.op)
			if got := failed(t, p); got != tt.want {
				t.Fatalf("Failed = %+v (%v), want %+v", got, p.Failed, tt.want)
			}
			if p.Failed.Tool != domain.OpKind(tt.op.Op) {
				t.Errorf("Failed's tool = %q, want %q", p.Failed.Tool, tt.op.Op)
			}
			if tt.is != nil && !tt.is(p.Failed) {
				t.Errorf("Failed = %#v", p.Failed.Err)
			}
		})
	}
	// H and its child under D9 would reach depth 11; under D8, 10.
	if p, _ := planOf(t, nb, moveOp("H.md", "D1/D2/D3/D4/D5/D6/D7/D8/H.md")); p.Failed != nil {
		t.Errorf("a move to depth 10: %v", p.Failed)
	}
	if p, _ := planOf(t, nb, moveOp("H.md", "D1/D2/D3/D4/D5/D6/D7/D8/D9/H.md")); failed(t, p).code != "page.too_deep" {
		t.Errorf("a move to depth 11: %v", p.Failed)
	}
}

// Each op finds its paths on the tree the ops before it leave (M9/P3
// design 3.4.1): a page moved is at its new path, no longer at its old;
// a page created is there, under the parents created with it; a page
// deleted is not.
func TestPlanBatchFollowsEarlierOps(t *testing.T) {
	nb := newNotebook("A.md", "A/B.md", "F.md")
	p, _ := planOf(t, nb, moveOp("A.md", "M.md"), editOp("M/B.md", "o", "n"), editOp("A/B.md", "o", "n"))
	if got := failed(t, p); got != (refusal{"page.not_found", "", "", 2}) {
		t.Errorf("an edit at a moved page's old path = %+v, want page.not_found at 2", got)
	}
	if p.Steps[1].Node != nb.id(t, "A/B.md") || p.Steps[1].Path != "M/B.md" {
		t.Errorf("the edit at the new path = %+v", p.Steps[1])
	}
	p, ids := planOf(t, nb, writeOp("A/N/O.md", "o"), editOp("A/N/O.md", "o", "p"), deleteOp("F.md"), editOp("F.md", "o", "n"))
	if got := failed(t, p); got != (refusal{"page.not_found", "", "", 3}) {
		t.Errorf("an edit of a deleted page = %+v, want page.not_found at 3", got)
	}
	created := []domain.CreatedPage{{ID: ids[0][1], ParentID: new(nb.id(t, "A.md")), Title: "N"}, {ID: ids[0][2], ParentID: &ids[0][1], Title: "O"}}
	if s := p.Steps[0]; !slices.EqualFunc(s.Created, created, sameCreated) || s.Node != ids[0][2] || s.Path != "A/N/O.md" ||
		!slices.Equal(s.Found, []uuid.UUID{nb.id(t, "A.md")}) || !s.Changed {
		t.Errorf("the write = %+v, want it to create %+v with the ids given", s, created)
	}
	if s := p.Steps[1]; s.Node != ids[0][2] || s.Previous != 0 {
		t.Errorf("the edit of the created page = %+v", s)
	}
}

// sameCreated reports whether a and b are one created page.
func sameCreated(a, b domain.CreatedPage) bool {
	return a.ID == b.ID && a.Title == b.Title && domain.SameParent(a.ParentID, b.ParentID)
}

// Items are the nodes the batch creates, deletes, moves and renames, each
// once (M9/P3 design 3.4.3): a node created and moved, or moved twice, is
// one; a move to where the node is, so named, is none; content is none.
func TestPlanBatchCountsItsItems(t *testing.T) {
	nb := newNotebook("A.md", "A/B.md", "A/B/C.md", "A/x.png", "F.md")
	p, _ := planOf(t, nb,
		writeOp("N/O.md", "o"),              // 2
		moveOp("N/O.md", "N/P.md"),          // the created O: none
		moveOp("F.md", "G.md"),              // 1
		moveOp("G.md", "A/G.md"),            // F again: none
		moveOp("A.md", "A.md"),              // where it is: none
		editOp("A/B.md", "o", "n"),          // content: none
		with(deleteOp("A/B.md"), recursive), // B and C: 2
	)
	if p.Failed != nil {
		t.Fatalf("Failed = %v", p.Failed)
	}
	if p.Items != 5 {
		t.Errorf("Items = %d, want 5", p.Items)
	}
	if p.Steps[4].Changed || !p.Steps[3].Changed {
		t.Errorf("a move where the node is changed %v, one elsewhere %v", p.Steps[4].Changed, p.Steps[3].Changed)
	}
}

// recursive sets an op's recursive.
func recursive(o *domain.BatchArgs) { o.Recursive = flag(true) }

// A delete takes the deletions before it under its subtree into it (M9
// design 4.11's normalization; M9/P3 design 3.4.3): the earlier op merged
// into it, the later's subtree theirs and its own; a fold of two levels
// into the last; a delete elsewhere left alone; recursive by its subtree
// alive then, the deleted ones not counted.
func TestPlanBatchMergesDeletionsUnderLaterOnes(t *testing.T) {
	nb := newNotebook("A.md", "A/B.md", "A/B/C.md", "A/B/y.png", "X.md")
	ids := func(paths ...string) []uuid.UUID {
		out := make([]uuid.UUID, len(paths))
		for i, p := range paths {
			out[i] = nb.id(t, p)
		}
		slices.SortFunc(out, uuid.UUID.Compare)
		return out
	}
	sorted := func(s []uuid.UUID) []uuid.UUID {
		s = slices.Clone(s)
		slices.SortFunc(s, uuid.UUID.Compare)
		return s
	}
	p, _ := planOf(t, nb, deleteOp("A/B/C.md"), deleteOp("X.md"), with(deleteOp("A/B.md"), recursive), deleteOp("A.md"))
	if p.Failed != nil {
		t.Fatalf("Failed = %v", p.Failed)
	}
	for i, want := range []int{3, -1, 3, -1} {
		if got := p.Steps[i].MergedInto; got != want {
			t.Errorf("ops[%d].MergedInto = %d, want %d", i, got, want)
		}
	}
	if got := sorted(p.Steps[3].Subtree); !slices.Equal(got, ids("A.md", "A/B.md", "A/B/C.md", "A/B/y.png")) {
		t.Errorf("the last delete's subtree = %v", got)
	}
	if s := p.Steps[3]; s.Pages != 3 || s.Assets != 1 {
		t.Errorf("the last delete counts %d pages and %d attachments, want 3 and 1", s.Pages, s.Assets)
	}
	for _, i := range []int{0, 2} {
		if s := p.Steps[i]; s.Pages != 0 || s.Assets != 0 {
			t.Errorf("ops[%d], merged, counts %d pages and %d attachments", i, s.Pages, s.Assets)
		}
	}
	if s := p.Steps[1]; !slices.Equal(s.Subtree, ids("X.md")) || s.Pages != 1 {
		t.Errorf("the delete elsewhere = %+v", s)
	}
	// A, its children deleted before, needs no recursive: as separate
	// calls would find it.
	if p.Items != 5 {
		t.Errorf("Items = %d, want 5", p.Items)
	}
}

// A batch deletes nothing it created, moved or renamed, itself or under
// what it deletes (M9 design 4.11, B1 and B3): 422 at the delete; content
// written and then deleted is no change of the tree, and may.
func TestPlanBatchDeletesNothingItChanged(t *testing.T) {
	nb := newNotebook("A.md", "A/B.md", "A/x.png", "F.md", "G.md")
	for _, tt := range []struct {
		name string
		ops  []domain.BatchArgs
		want string // the message's start, "" for no refusal
	}{
		{"a page created", []domain.BatchArgs{writeOp("N.md", "n"), deleteOp("N.md")}, "deletes a page that ops[0] created"},
		{"under a page created", []domain.BatchArgs{writeOp("A/N.md", "n"), with(deleteOp("A.md"), recursive)}, "deletes a page that ops[0] created"},
		{"a page renamed", []domain.BatchArgs{moveOp("F.md", "F2.md"), deleteOp("F2.md")}, "deletes a page that ops[0] moved or renamed"},
		{"a page moved", []domain.BatchArgs{moveOp("F.md", "G/F.md"), with(deleteOp("G.md"), recursive)}, "deletes a page that ops[0] moved or renamed"},
		{"an attachment renamed", []domain.BatchArgs{moveOp("A/x.png", "A/z.png"), with(deleteOp("A.md"), recursive)},
			"deletes an attachment that ops[0] moved or renamed"},
		{"content written, then deleted", []domain.BatchArgs{editOp("F.md", "o", "n"), deleteOp("F.md")}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := planOf(t, nb, tt.ops...)
			if tt.want == "" {
				if p.Failed != nil {
					t.Fatalf("Failed = %v", p.Failed)
				}
				return
			}
			at := len(tt.ops) - 1
			if got := failed(t, p); got != (refusal{"validation_failed", "ops[" + string(rune('0'+at)) + "]", "not_allowed", at}) ||
				!strings.HasPrefix(fieldMessage(p.Failed), tt.want) {
				t.Errorf("Failed = %+v %q, want ops[%d] not_allowed %q", got, fieldMessage(p.Failed), at, tt.want)
			}
		})
	}
}

// A base_version refers to the notebook before the batch: on a page the
// batch created it is 422, even when its node is another page's, whose
// 409 never is answered first (M9/P3 review A-M12).
func TestPlanBatchRefusesATokenOfAPageItCreated(t *testing.T) {
	nb := newNotebook("A.md")
	p, _ := planOf(t, nb, writeOp("N.md", "n"),
		with(editOp("N.md", "n", "m"), func(o *domain.BatchArgs) { o.Base = tokenOf(nb.id(t, "A.md"), 1) }))
	if got := failed(t, p); got != (refusal{"validation_failed", "ops[1].base_version", "not_allowed", 1}) ||
		fieldMessage(p.Failed) != "refers to the notebook before the batch: ops[0] created this page" {
		t.Errorf("Failed = %+v %q", got, fieldMessage(p.Failed))
	}
}

// A rename under its parent reads no subtree, as the unit's rename reads
// none (M9/P3 review A-N11): a folder's rename is not as costly as its
// nodes; a move under another parent reads the node's.
func TestPlanBatchRenamesWithoutTheSubtree(t *testing.T) {
	nb := newNotebook("A.md", "A/B.md", "A/B/C.md", "F.md")
	p, _ := planOf(t, nb, moveOp("A.md", "A2.md"), moveOp("A2/B.md", "A2/B2.md"))
	if p.Failed != nil || domain.SubtreeReads(p) != 0 {
		t.Errorf("two renames read %d subtrees (%v)", domain.SubtreeReads(p), p.Failed)
	}
	p, _ = planOf(t, nb, moveOp("A.md", "F/A.md"))
	if p.Failed != nil || domain.SubtreeReads(p) != 1 {
		t.Errorf("a move read %d subtrees (%v), want 1", domain.SubtreeReads(p), p.Failed)
	}
}

// The items are answered in the tree's pass (M9/P3 review A-M11): the op
// past MaxUnitNodes, its contents' pass not run, though an earlier op
// would fail there; an op that fails before it in the tree's pass is the
// answer.
func TestPlanBatchAnswersItsItemsInTheTreesPass(t *testing.T) {
	nb := newNotebook("P.md", "Big.md")
	for i := range domain.MaxUnitNodes {
		nb.add("Big/n" + itoa(i) + ".png")
	}
	rw := &fakeRewriter{}
	p, _ := planOf(t, nb, editOp("P.md", "missing", "n"), with(deleteOp("Big.md"), recursive))
	if got := failed(t, p); got != (refusal{"page.batch_too_large", "", "", 1}) {
		t.Fatalf("Failed = %+v, want the items at 1", got)
	}
	if err := p.Contents(map[uuid.UUID]domain.PageText{nb.id(t, "P.md"): {Content: "text", Revision: 1}}, nil, rw); err != nil {
		t.Fatal(err)
	}
	if got := failed(t, p); got.at != 1 || got.code != "page.batch_too_large" {
		t.Errorf("after Contents, Failed = %+v: the contents' pass ran", got)
	}
	p, _ = planOf(t, nb, editOp("Nope.md", "o", "n"), with(deleteOp("Big.md"), recursive))
	if got := failed(t, p); got != (refusal{"page.not_found", "", "", 0}) {
		t.Errorf("Failed = %+v, want the 404 at 0", got)
	}
}

// The nodes a batch's moves to another parent carry, each move's subtree
// again, are bounded at MaxUnitNodes once there is more than one such
// move (M9/P3 review A-I2): F of 5,000 nodes and H of 5,000 moved are at
// the bound, H of 5,001 past it at its move, the contents' pass not run;
// one folder moved there and back, each move counted, at the bound with
// 5,000 nodes and past it with 5,001 (review A's N2); a lone move of a
// larger folder, and moves among siblings, are not. A move past it that
// its own checks refuse answers their refusal, as the items' bound waits
// for its op (review A's N4): H moved under its own child, a cycle's 409.
func TestPlanBatchBoundsWhatItsMovesCarry(t *testing.T) {
	half := domain.MaxUnitNodes / 2
	folders := func(f, h int) *notebook {
		nb := newNotebook("G.md", "F.md", "H.md")
		for i := range f - 1 {
			nb.add("F/f" + itoa(i) + ".png")
		}
		for i := range h - 1 {
			nb.add("H/h" + itoa(i) + ".png")
		}
		return nb
	}
	p, _ := planOf(t, folders(half, half), moveOp("F.md", "G/F.md"), moveOp("H.md", "G/H.md"))
	if p.Failed != nil {
		t.Errorf("at the bound: Failed = %v", p.Failed)
	}
	p, _ = planOf(t, folders(half, half+1), editOp("P.md", "o", "n"), moveOp("F.md", "G/F.md"), moveOp("H.md", "G/H.md"))
	if got := failed(t, p); got.code != "page.not_found" || got.at != 0 {
		t.Errorf("an op before: Failed = %+v, want its 404 at 0", got)
	}
	p, _ = planOf(t, folders(half, half+1), moveOp("F.md", "G/F.md"), moveOp("H.md", "G/H.md"), editOp("G/F.md", "o", "n"))
	if got := failed(t, p); got != (refusal{"page.batch_too_large", "", "", 1}) ||
		!strings.Contains(p.Failed.Error(), "10001 nodes the batch's moves carry, at most 10000: move each folder once, or in separate calls") {
		t.Fatalf("a node past: Failed = %+v (%v)", got, p.Failed)
	}
	if texts, heads := p.Reads(); len(texts)+len(heads) != 0 {
		t.Errorf("past the bound, Reads = %v, %v: the contents' pass would run", texts, heads)
	}
	p, _ = planOf(t, folders(half, 1), moveOp("F.md", "G/F.md"), moveOp("G/F.md", "F.md"))
	if p.Failed != nil {
		t.Errorf("a folder of %d there and back: Failed = %v", half, p.Failed)
	}
	p, _ = planOf(t, folders(half+1, 1), moveOp("F.md", "G/F.md"), moveOp("G/F.md", "F.md"))
	if got := failed(t, p); got != (refusal{"page.batch_too_large", "", "", 1}) ||
		!strings.Contains(p.Failed.Error(), "10002 nodes the batch's moves carry, at most 10000") {
		t.Errorf("a folder of %d there and back: Failed = %+v (%v), want the bound at its way back", half+1, got, p.Failed)
	}
	p, _ = planOf(t, folders(domain.MaxUnitNodes+1, 1), moveOp("F.md", "G/F.md"))
	if p.Failed != nil {
		t.Errorf("a lone move of %d nodes: Failed = %v", domain.MaxUnitNodes+1, p.Failed)
	}
	nb := folders(domain.MaxUnitNodes+1, 10)
	nb.add("H/K.md")
	p, _ = planOf(t, nb, moveOp("F.md", "G/F.md"), moveOp("H.md", "H/K/H.md"))
	if got := failed(t, p); got.code != "page.cycle" || got.at != 1 {
		t.Errorf("a cycle past the bound: Failed = %+v (%v), want its 409 at ops[1]", got, p.Failed)
	}
	p, _ = planOf(t, folders(domain.MaxUnitNodes, domain.MaxUnitNodes), moveOp("F.md", "F2.md"), moveOp("H.md", "H2.md"))
	if p.Failed != nil {
		t.Errorf("renames of two folders of %d nodes: Failed = %v", domain.MaxUnitNodes, p.Failed)
	}
}

// itoa is n in decimal.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// A write without base_version at a page there is that page's content op,
// its content compared in the contents' pass; and the first op that
// fails, of either pass, decides (M9/P3 design 3.4.4): one at 3 of the
// contents' before one at 5 of the tree's.
func TestPlanBatchFirstFailureDecides(t *testing.T) {
	nb := newNotebook("A.md", "B.md")
	a := nb.id(t, "A.md")
	p, _ := planOf(t, nb, writeOp("A.md", "same"), editOp("B.md", "b", "c"), writeOp("A.md", "same"), editOp("B.md", "missing", "x"),
		editOp("B.md", "c", "d"), editOp("Nope.md", "o", "n"))
	if got := failed(t, p); got != (refusal{"page.not_found", "", "", 5}) {
		t.Fatalf("the tree's pass failed %+v, want at 5", got)
	}
	if p.Steps[0].Node != a || p.Steps[0].Previous != -1 || p.Steps[2].Previous != 0 {
		t.Errorf("the writes at A = %+v, %+v", p.Steps[0], p.Steps[2])
	}
	heads := map[uuid.UUID]domain.PageHead{a: headOf("same", 2)}
	texts := map[uuid.UUID]domain.PageText{nb.id(t, "B.md"): {Content: "b", Revision: 1}}
	if err := p.Contents(texts, heads, &fakeRewriter{}); err != nil {
		t.Fatal(err)
	}
	if got := failed(t, p); got != (refusal{"validation_failed", "ops[3].old_string", "not_allowed", 3}) {
		t.Errorf("Failed = %+v, want the contents' at 3", got)
	}
	if p.Steps[0].Changed || p.Steps[2].Changed {
		t.Error("a write of the bytes there changed the page")
	}
}
