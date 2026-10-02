package domain_test

import (
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func TestPreOrder(t *testing.T) {
	ids := make([]uuid.UUID, 7)
	for i := range ids {
		ids[i] = uuid.NewV7()
	}
	child := func(i int, parent *uuid.UUID, order float64) domain.Node {
		return domain.Node{ID: ids[i], ParentID: parent, SortOrder: order, Name: string(rune('a' + i))}
	}
	a, b := ids[0], ids[1]
	nodes := []domain.Node{
		child(3, &a, 1), // a's second child
		child(1, nil, 2),
		child(2, &a, 0), // a's first child
		child(0, nil, 1),
		child(4, &ids[2], 0), // a grandchild
		child(5, &b, 0),
		child(6, &ids[6], 0), // its own parent: unreachable
	}
	var got []string
	for _, n := range domain.PreOrder(nodes) {
		got = append(got, n.Name)
	}
	if want := []string{"a", "c", "e", "d", "b", "f"}; !slices.Equal(got, want) {
		t.Errorf("PreOrder() = %v, want %v", got, want)
	}
}

// Siblings of the same order fall back on their ids.
func TestPreOrderBreaksTiesByID(t *testing.T) {
	first, second := uuid.NewV7(), uuid.NewV7()
	got := domain.PreOrder([]domain.Node{{ID: second}, {ID: first}})
	if got[0].ID != first || got[1].ID != second {
		t.Errorf("PreOrder() = %s, %s; want the lower id first", got[0].ID, got[1].ID)
	}
}

func TestDepth(t *testing.T) {
	if got := domain.Depth(nil); got != 1 {
		t.Errorf("Depth(no ancestors) = %d, want 1", got)
	}
	if got := domain.Depth(make([]domain.Ancestor, domain.MaxDepth-1)); got != domain.MaxDepth {
		t.Errorf("Depth(%d ancestors) = %d, want %d", domain.MaxDepth-1, got, domain.MaxDepth)
	}
}
