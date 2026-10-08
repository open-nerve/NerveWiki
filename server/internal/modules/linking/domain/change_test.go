package domain_test

import (
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// What a unit reaches: when it relocates a node, every node it changed, by
// its keys before and after and its id, sorted and each once; the nodes it
// renamed, to the same key of another length too, for the caller to add
// their subtrees; when it relocates none, the pages whose content it
// writes, as sources. A unit that writes no content of a page it leaves and
// relocates no node reaches nothing: a move among siblings, a rename to
// the same key and length, a page created and deleted.
func TestAffectedIsWhatAUnitReaches(t *testing.T) {
	a, b, c, root := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	at := func(parent *uuid.UUID, name string) *domain.Place { return &domain.Place{ParentID: parent, Name: name} }
	ids := func(ids ...uuid.UUID) []uuid.UUID {
		slices.SortFunc(ids, uuid.UUID.Compare)
		return ids
	}
	tests := []struct {
		name    string
		changes []domain.Change
		reach   domain.Reach
		renamed []uuid.UUID
		ok      bool
	}{
		{"a move among siblings, the pages under it", []domain.Change{
			{NodeID: a, Before: at(nil, "A"), After: at(nil, "A")},
			{NodeID: b, Before: at(&a, "B"), After: at(&a, "B")},
		}, domain.Reach{}, nil, false},
		{"a rename to the same key and length", []domain.Change{{NodeID: a, Before: at(nil, "Note"), After: at(nil, "NOTE")}}, domain.Reach{}, nil, false},
		{"created and deleted", []domain.Change{{NodeID: a, Revision: 1}}, domain.Reach{}, nil, false},
		{"a content written", []domain.Change{{NodeID: a, Before: at(nil, "A"), After: at(nil, "A"), Revision: 2}},
			domain.Reach{Sources: ids(a)}, nil, true},
		{"a content written, and a page moved", []domain.Change{
			{NodeID: a, Before: at(nil, "A"), After: at(nil, "A"), Revision: 2},
			{NodeID: b, Before: at(nil, "B"), After: at(&root, "B")},
		}, domain.Reach{Keys: []string{"a", "b"}, Targets: ids(a, b), Sources: ids(a, b)}, nil, true},
		{"a rename that keeps the key, to a name of another length", []domain.Change{{NodeID: a, Before: at(nil, "Straße"), After: at(nil, "STRASSE")}},
			domain.Reach{Keys: []string{"strasse"}, Targets: ids(a), Sources: ids(a)}, []uuid.UUID{a}, true},
		{"created", []domain.Change{{NodeID: a, After: at(nil, "New"), Revision: 1}},
			domain.Reach{Keys: []string{"new"}, Targets: ids(a), Sources: ids(a)}, nil, true},
		{"renamed", []domain.Change{{NodeID: a, Before: at(nil, "Old"), After: at(nil, "New")}},
			domain.Reach{Keys: []string{"new", "old"}, Targets: ids(a), Sources: ids(a)}, []uuid.UUID{a}, true},
		{"moved, with the page under it", []domain.Change{
			{NodeID: a, Before: at(nil, "A"), After: at(&root, "A")},
			{NodeID: b, Before: at(&a, "B"), After: at(&a, "B")},
		}, domain.Reach{Keys: []string{"a", "b"}, Targets: ids(a, b), Sources: ids(a, b)}, nil, true},
		{"deleted, with the page under it", []domain.Change{
			{NodeID: a, Before: at(nil, "A")},
			{NodeID: c, Before: at(&a, "C")},
		}, domain.Reach{Keys: []string{"a", "c"}, Targets: ids(a, c), Sources: ids(a, c)}, nil, true},
		{"one change that relocates, one that does not", []domain.Change{
			{NodeID: b, Before: at(nil, "B"), After: at(nil, "B")},
			{NodeID: a, Before: at(nil, "A")},
		}, domain.Reach{Keys: []string{"a", "b"}, Targets: ids(a, b), Sources: ids(a, b)}, nil, true},
	}
	for _, tt := range tests {
		reach, renamed, ok := domain.Affected(tt.changes)
		if !reflect.DeepEqual(reach, tt.reach) || !reflect.DeepEqual(renamed, tt.renamed) || ok != tt.ok {
			t.Errorf("%s: %+v, %v, %v; want %+v, %v, %v", tt.name, reach, renamed, ok, tt.reach, tt.renamed, tt.ok)
		}
	}
}

// Adding nodes reaches their keys, the links to them and those written in
// them; compacting sorts each set, each once.
func TestAReachAddsNodesAndCompacts(t *testing.T) {
	a, b := uuid.NewV7(), uuid.NewV7()
	r := domain.Reach{Keys: []string{"z"}}
	r.Add(domain.Step{ID: b, Key: "b"}, domain.Step{ID: a, Key: "a"}, domain.Step{ID: b, Key: "b"})
	r.Compact()
	want := domain.Reach{Keys: []string{"a", "b", "z"}, Targets: []uuid.UUID{a, b}, Sources: []uuid.UUID{a, b}}
	if !reflect.DeepEqual(r, want) {
		t.Errorf("reach = %+v, want %+v", r, want)
	}
}

// What a rewrite follows: a rename or a move that relocates a node, a
// rename of the case alone (with the length too: "ß" to "SS"), and nothing
// else: a move among siblings, a rename to itself, a creation, a deletion,
// a content written.
func TestRelocationIsWhatARewriteFollows(t *testing.T) {
	a, b, root := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	at := func(parent *uuid.UUID, name string) *domain.Place { return &domain.Place{ParentID: parent, Name: name} }
	for _, tt := range []struct {
		name      string
		changes   []domain.Change
		relocates bool
		recased   domain.Recased
	}{
		{"a rename", []domain.Change{{NodeID: a, Before: at(nil, "x"), After: at(nil, "y")}}, true, domain.Recased{}},
		{"a move with its subtree", []domain.Change{
			{NodeID: a, Before: at(nil, "x"), After: at(&root, "x")},
			{NodeID: b, Before: at(&a, "y"), After: at(&a, "y")},
		}, true, domain.Recased{}},
		{"the case alone", []domain.Change{{NodeID: a, Before: at(nil, "Old"), After: at(nil, "old")}}, false, domain.Recased{ID: a, Name: "old"}},
		{"the case alone, another length", []domain.Change{{NodeID: a, Before: at(nil, "ß"), After: at(nil, "SS")}}, true,
			domain.Recased{ID: a, Name: "SS"}},
		{"a move among siblings", []domain.Change{{NodeID: a, Before: at(&root, "x"), After: at(&root, "x")}}, false, domain.Recased{}},
		{"a creation", []domain.Change{{NodeID: a, After: at(nil, "x"), Revision: 1}}, false, domain.Recased{}},
		{"a deletion", []domain.Change{{NodeID: a, Before: at(nil, "x")}}, false, domain.Recased{}},
		{"a content", []domain.Change{{NodeID: a, Before: at(nil, "x"), After: at(nil, "x"), Revision: 2}}, false, domain.Recased{}},
		{"nothing", nil, false, domain.Recased{}},
	} {
		if relocates, recased := domain.Relocation(tt.changes); relocates != tt.relocates || recased != tt.recased {
			t.Errorf("%s: relocates %t, recased %v; want %t, %v", tt.name, relocates, recased, tt.relocates, tt.recased)
		}
	}
}

// A page's path before a rename or a move: the name before on the path of
// every page under the node renamed, its own too; the former parent's path
// before those under the node moved, the root's none; a page off them as
// it is; an attachment's the same, of its kind. The former parents are
// those of the nodes moved under another parent, each once.
func TestAPathBeforeARenameOrAMove(t *testing.T) {
	a, b, x, y, z := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	step := func(id uuid.UUID, name string) domain.Step {
		return domain.Step{ID: id, Key: shared.TitleKey(name), Name: name}
	}
	at := func(parent *uuid.UUID, name string) *domain.Place { return &domain.Place{ParentID: parent, Name: name} }
	node := func(steps ...domain.Step) domain.Node { return domain.Node{ID: steps[len(steps)-1].ID, Path: steps} }
	asset := func(steps ...domain.Step) domain.Node {
		n := node(steps...)
		n.Asset = true
		return n
	}
	renamed := []domain.Change{{NodeID: x, Before: at(&a, "Old"), After: at(&a, "New")}}
	moved := []domain.Change{
		{NodeID: x, Before: at(&a, "X"), After: at(&b, "X")},
		{NodeID: y, Before: at(&x, "Y"), After: at(&x, "Y")},
	}
	toRoot := []domain.Change{{NodeID: x, Before: at(&a, "X"), After: at(nil, "X")}}
	fromRoot := []domain.Change{{NodeID: x, Before: at(nil, "X"), After: at(&b, "X")}}
	parents := map[uuid.UUID][]domain.Step{a: {step(a, "A")}}
	for _, tt := range []struct {
		name    string
		n       domain.Node
		changes []domain.Change
		want    domain.Node
	}{
		{"the page renamed", node(step(a, "A"), step(x, "New")), renamed, node(step(a, "A"), step(x, "Old"))},
		{"a page under it", node(step(a, "A"), step(x, "New"), step(y, "Y")), renamed, node(step(a, "A"), step(x, "Old"), step(y, "Y"))},
		{"a page off it", node(step(a, "A"), step(z, "Z")), renamed, node(step(a, "A"), step(z, "Z"))},
		{"the page moved", node(step(b, "B"), step(x, "X")), moved, node(step(a, "A"), step(x, "X"))},
		{"a page under it", node(step(b, "B"), step(x, "X"), step(y, "Y")), moved, node(step(a, "A"), step(x, "X"), step(y, "Y"))},
		{"a page moved to the root", node(step(x, "X")), toRoot, node(step(a, "A"), step(x, "X"))},
		{"a page moved from the root", node(step(b, "B"), step(x, "X")), fromRoot, node(step(x, "X"))},
		{"an attachment under it", asset(step(a, "A"), step(x, "New"), step(z, "z.png")), renamed,
			asset(step(a, "A"), step(x, "Old"), step(z, "z.png"))},
		{"an attachment moved", asset(step(b, "B"), step(x, "X")), moved, asset(step(a, "A"), step(x, "X"))},
	} {
		if got := domain.PathBefore(tt.n, tt.changes, parents); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: %+v, want %+v", tt.name, got, tt.want)
		}
	}
	if got := domain.FormerParents(append(append(moved, toRoot...), fromRoot...)); !slices.Equal(got, []uuid.UUID{a}) {
		t.Errorf("former parents %v, want %v", got, []uuid.UUID{a})
	}
}
