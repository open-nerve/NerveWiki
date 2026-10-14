package domain_test

import (
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// shape is a group read as TrashShape reads it: the root, then each level
// by parent, each parent's children in order. ids names the nodes by
// their positions in it.
type shape struct {
	root  domain.ShapeNode
	below []domain.ShapeNode
	names map[uuid.UUID]string
}

func newShape(name string) *shape {
	s := &shape{root: domain.ShapeNode{ID: uuid.NewV7()}, names: map[uuid.UUID]string{}}
	s.names[s.root.ID] = name
	return s
}

// add adds a node named name under parent, after its siblings added so
// far, and answers its id.
func (s *shape) add(parent uuid.UUID, name string, asset bool) uuid.UUID {
	n := domain.ShapeNode{ID: uuid.NewV7(), ParentID: parent, Asset: asset, Bytes: len(name)}
	s.below, s.names[n.ID] = append(s.below, n), name
	return n.ID
}

func (s *shape) order() ([]domain.GroupNode, []string) {
	nodes := domain.GroupPreOrder(s.root, s.below)
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = s.names[n.ID]
	}
	return nodes, names
}

// MaxJobItems is the size measured at the full load, 20 links a page
// (M8/P4 design 3.11, 3.12; its review's fix round): a new value is
// measured again (BenchmarkBackgroundSteps) and the guards' limits set
// anew (TestStartRevertAtMaxJobItems, TestARevertJobOfAnImportAtMaxJobItems).
func TestMaxJobItemsIsTheMeasuredSize(t *testing.T) {
	if domain.MaxJobItems != 60_000 {
		t.Errorf("MaxJobItems = %d, measured 60,000", domain.MaxJobItems)
	}
}

// A group's pre-order: each parent before its children, its subtree whole
// before the next sibling, the siblings in the order the level read them
// (by order and id, as TrashGroup reads each level), not by id; each
// node's parent by its position, the root's -1; a page's bytes and an
// attachment's kind kept.
func TestGroupPreOrder(t *testing.T) {
	s := newShape("r")
	b := s.add(s.root.ID, "b", false) // the root's first child, of a later id than none
	a := s.add(s.root.ID, "a", false)
	s.add(s.root.ID, "z.png", true)
	s.add(a, "a1", false)
	s.add(b, "b1", false)
	b2 := s.add(b, "b2", false)
	s.add(b2, "b2x", false)
	nodes, names := s.order()
	if want := []string{"r", "b", "b1", "b2", "b2x", "a", "a1", "z.png"}; !slices.Equal(names, want) {
		t.Fatalf("GroupPreOrder() = %v, want %v", names, want)
	}
	if want := []int32{-1, 0, 1, 1, 3, 0, 5, 0}; !slices.Equal(parents(nodes), want) {
		t.Errorf("parents %v, want %v", parents(nodes), want)
	}
	if !nodes[7].Asset || nodes[6].Asset || nodes[6].Bytes != 2 {
		t.Errorf("nodes %+v, %+v; want the attachment's kind and the page's bytes kept", nodes[6], nodes[7])
	}
}

func parents(nodes []domain.GroupNode) []int32 {
	out := make([]int32, len(nodes))
	for i, n := range nodes {
		out[i] = n.Parent
	}
	return out
}

// The counts of each node's subtree within the group, itself included: a
// page's levels are one more than its highest child's, an attachment is no
// level; across three levels and a wide one.
func TestGroupCounts(t *testing.T) {
	s := newShape("r")
	a := s.add(s.root.ID, "a", false)
	for range 5 {
		s.add(a, "w", false) // a wide level
	}
	s.add(a, "a.png", true)
	b := s.add(s.root.ID, "b", false)
	c := s.add(b, "c", false)
	s.add(c, "c.png", true)
	s.add(s.root.ID, "r.png", true)
	nodes, names := s.order()
	domain.GroupCounts(nodes)
	type counts struct{ pages, assets, height int }
	want := map[string]counts{"r": {9, 3, 3}, "a": {6, 1, 2}, "w": {1, 0, 1}, "a.png": {0, 1, 0}, "b": {2, 1, 2}, "c": {1, 1, 1},
		"c.png": {0, 1, 0}, "r.png": {0, 1, 0}}
	for i, n := range nodes {
		if got := (counts{int(n.Pages), int(n.Assets), int(n.Height)}); got != want[names[i]] {
			t.Errorf("%s counts %+v, want %+v", names[i], got, want[names[i]])
		}
		if n.Size() != int(n.Pages+n.Assets) {
			t.Errorf("%s size %d", names[i], n.Size())
		}
	}
	// Counted again, as a plan of the same group is, the same.
	again := slices.Clone(nodes)
	domain.GroupCounts(again)
	if !slices.Equal(again, nodes) {
		t.Errorf("counted twice %+v, want %+v", again, nodes)
	}
}

// A batch's frontier: the children of the nodes it restored that come
// after it, those of a node it did not restore left alone, the batch's end
// in the middle of a parent's children.
func TestFrontier(t *testing.T) {
	s := newShape("r")
	a := s.add(s.root.ID, "a", false)
	s.add(a, "a1", false)
	a2 := s.add(a, "a2", false)
	s.add(a2, "a2x", false)
	s.add(a, "a3", false)
	b := s.add(s.root.ID, "b", false)
	s.add(b, "b1", false)
	s.add(s.root.ID, "c", false)
	nodes, names := s.order()
	// r a a1 a2 a2x a3 b b1 c
	domain.GroupCounts(nodes)
	all := func(int) bool { return true }
	named := func(at []int) []string {
		out := make([]string, len(at))
		for i, n := range at {
			out[i] = names[n]
		}
		return out
	}
	for _, c := range []struct {
		name     string
		from, to int
		restored func(int) bool
		want     []string
	}{
		{"the first batch, ending among a's children", 0, 3, all, []string{"a2", "a3", "b", "c"}},
		{"the next, its parent restored before it", 3, 6, all, []string{}},
		{"one ending under a2", 3, 4, all, []string{"a2x"}},
		{"its node not restored: someone else did", 3, 4, func(i int) bool { return i != 3 }, []string{}},
		{"the whole group", 0, len(nodes), all, []string{}},
		{"the last batch", 6, 9, all, []string{}},
	} {
		got := named(domain.Frontier(nodes, c.from, c.to, c.restored))
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: Frontier(%d, %d) = %v, want %v", c.name, c.from, c.to, got, c.want)
		}
	}
}

// A group restored a batch at a time in its pre-order leaves at most
// about levels times fan-out roots after each batch, and every node of the
// group restored or a root once at most: 50 folders of 1,000 pages each,
// the first batch's 951 the 49 folders after it and the 902 pages of the
// first left; level by level, the first batch would set 49,951 (M8/P4
// design 3.4, review A-I2).
func TestTheFrontierOfAGroupInPreOrder(t *testing.T) {
	s := newShape("r")
	for range 50 {
		folder := s.add(s.root.ID, "f", false)
		for range 1000 {
			s.add(folder, "p", false)
		}
	}
	nodes, _ := s.order()
	domain.GroupCounts(nodes)
	const batch = 100
	marked, most := map[int]bool{}, 0
	for from := 0; from < len(nodes); from += batch {
		to := min(from+batch, len(nodes))
		frontier := domain.Frontier(nodes, from, to, func(int) bool { return true })
		if from == 0 && len(frontier) != 951 {
			t.Errorf("the first batch's frontier holds %d, want 951", len(frontier))
		}
		most = max(most, len(frontier))
		for _, n := range frontier {
			if n < to || marked[n] {
				t.Fatalf("batch %d-%d marks %d: in it, or marked before", from, to, n)
			}
			marked[n] = true
		}
	}
	if most > 2*1000 {
		t.Errorf("a batch's frontier held %d, want about levels times fan-out at most", most)
	}
}
