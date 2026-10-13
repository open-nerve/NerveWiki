package app

import (
	"context"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// A step changes each node once, so that its items are one statement that
// reaches each row once (M8/P2 design 3.3): a step that changes a node
// twice is its caller's defect, refused before anything is written.
func TestAStepChangesEachNodeOnce(t *testing.T) {
	u := &Unit{w: &Writer{}, merged: map[uuid.UUID]int{}}
	id := uuid.NewV7()
	a, b := domain.TreeState{Name: "a"}, domain.TreeState{Name: "b"}
	if err := u.recordItems(context.Background(), []domain.Change{{NodeID: id, Before: &a, After: &b}, {NodeID: id, Before: &b}}); err == nil {
		t.Error("recordItems(a node twice) = nil, want the caller's defect")
	}
}
