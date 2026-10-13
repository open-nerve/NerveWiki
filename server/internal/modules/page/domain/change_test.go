package domain_test

import (
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestChangeMoves(t *testing.T) {
	p, q := uuid.NewV7(), uuid.NewV7()
	at := domain.TreeState{ParentID: &p, Name: "a", SortOrder: 1}
	for _, tt := range []struct {
		name          string
		before, after *domain.TreeState
		want          bool
	}{
		{"created", nil, &at, true},
		{"deleted", &at, nil, true},
		{"renamed", &at, &domain.TreeState{ParentID: &p, Name: "b", SortOrder: 1}, true},
		{"moved", &at, &domain.TreeState{ParentID: &q, Name: "a", SortOrder: 1}, true},
		{"to the root", &at, &domain.TreeState{Name: "a", SortOrder: 1}, true},
		{"reordered", &at, &domain.TreeState{ParentID: &p, Name: "a", SortOrder: 2}, true},
		{"content alone", &at, &domain.TreeState{ParentID: &p, Name: "a", SortOrder: 1}, false},
	} {
		if got := (domain.Change{Before: tt.before, After: tt.after}).Moves(); got != tt.want {
			t.Errorf("%s: Moves() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestChangeThen(t *testing.T) {
	id := uuid.NewV7()
	a, b, c := domain.TreeState{Name: "a"}, domain.TreeState{Name: "b"}, domain.TreeState{Name: "c"}
	got := domain.Change{NodeID: id, After: &a, Revision: 1, Facts: "one"}.Then(domain.Change{NodeID: id, Before: &a, After: &b})
	if got.Before != nil || got.After.Name != "b" || got.Revision != 1 || got.Facts != "one" {
		t.Errorf("created then renamed = %+v; want created, named b, at revision 1 with its facts", got)
	}
	got = domain.Change{NodeID: id, Before: &a, After: &b}.Then(domain.Change{NodeID: id, Before: &b, After: &c, Revision: 4, Facts: "four"})
	if got.Before.Name != "a" || got.After.Name != "c" || got.Revision != 4 || got.Facts != "four" {
		t.Errorf("renamed twice, then written = %+v; want a to c at revision 4 with its facts", got)
	}
	got = domain.Change{NodeID: id, Before: &a, After: &a, Revision: 2, Facts: "two"}.Then(
		domain.Change{NodeID: id, Before: &a, After: &a, Revision: 3, Facts: "three"})
	if got.Revision != 3 || got.Facts != "three" {
		t.Errorf("written twice = %+v; want revision 3 with its facts", got)
	}
}

// A restore is the first change's, as its before is: a later change of the
// node in the unit keeps it.
func TestChangeThenKeepsTheFirstRestore(t *testing.T) {
	id := uuid.NewV7()
	a, b := domain.TreeState{Name: "a"}, domain.TreeState{Name: "b"}
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	restored := domain.Change{NodeID: id, After: &a, Revision: 3, Restored: &domain.Restored{DeletedAt: at, Asset: true}}
	got := restored.Then(domain.Change{NodeID: id, Before: &a, After: &b})
	if got.Restored == nil || !got.Restored.DeletedAt.Equal(at) || !got.Restored.Asset || got.Before != nil || got.After.Name != "b" {
		t.Errorf("restored then renamed = %+v; want restored from %s, an attachment, named b", got, at)
	}
	got = domain.Change{NodeID: id, Before: &a, After: &a}.Then(restored)
	if got.Restored != nil {
		t.Errorf("written then restored = %+v; want the first change's restore, none", got)
	}
}
