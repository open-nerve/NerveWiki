package domain_test

import (
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// What a unit reaches: every node it changed, by its keys before and after
// and its id, sorted and each once; the nodes it renamed to another key,
// for the caller to add their subtrees. A unit that writes no content of a
// page it leaves and relocates no node reaches nothing: a move among
// siblings, a rename that keeps the key, a page created and deleted.
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
		{"a rename that keeps the key", []domain.Change{{NodeID: a, Before: at(nil, "Straße"), After: at(nil, "STRASSE")}}, domain.Reach{}, nil, false},
		{"created and deleted", []domain.Change{{NodeID: a, Revision: 1}}, domain.Reach{}, nil, false},
		{"a content written", []domain.Change{{NodeID: a, Before: at(nil, "A"), After: at(nil, "A"), Revision: 2}},
			domain.Reach{Keys: []string{"a"}, Targets: ids(a), Sources: ids(a)}, nil, true},
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

// A page's tags and aliases are one a title key, as first written; a
// tag's count is how often it is written.
func TestTagsAndAliasesAreOneAKey(t *testing.T) {
	tags := domain.TagsOf([]string{"ToDo", "x", "todo", "TODO", "Straße", "strasse"})
	if want := []domain.Tag{{Key: "todo", Name: "ToDo", Count: 3}, {Key: "x", Name: "x", Count: 1}, {Key: "strasse", Name: "Straße", Count: 2}}; !reflect.DeepEqual(tags, want) {
		t.Errorf("tags = %+v, want %+v", tags, want)
	}
	aliases := domain.AliasesOf([]string{"Al", "AL", "Straße", "STRASSE"})
	if want := []domain.Alias{{Key: "al", Name: "Al"}, {Key: "strasse", Name: "Straße"}}; !reflect.DeepEqual(aliases, want) {
		t.Errorf("aliases = %+v, want %+v", aliases, want)
	}
	if domain.TagsOf(nil) != nil || domain.AliasesOf(nil) != nil {
		t.Error("no names are some tags or aliases")
	}
}

// A link's keys are its target's last segment's, in the forms it may be
// written in; none for a target that resolves to nothing.
func TestALinksKeys(t *testing.T) {
	for target, want := range map[string][]string{
		"A/Note.md": {"note", "note.md"},
		"Note":      {"note"},
		"A//B":      nil,
	} {
		if got := (domain.Link{Target: target}).Keys(); !reflect.DeepEqual(got, want) {
			t.Errorf("the keys of %q = %v, want %v", target, got, want)
		}
	}
}
