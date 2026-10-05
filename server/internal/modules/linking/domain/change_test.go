package domain_test

import (
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
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
