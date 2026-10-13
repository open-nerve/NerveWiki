package bootstrap

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// restoringAssets is the asset module's observer as assetObserver calls
// it: it keeps what it is asked, and answers rows for a restore.
type restoringAssets struct {
	deleted  []uuid.UUID
	restored map[time.Time][]uuid.UUID
	rows     func(ids []uuid.UUID) int
}

func (a *restoringAssets) NodesDeleted(_ context.Context, ids []uuid.UUID, _ time.Time) error {
	a.deleted = append(a.deleted, ids...)
	return nil
}

func (a *restoringAssets) NodesRestored(_ context.Context, ids []uuid.UUID, deletedAt time.Time) (int, error) {
	a.restored[deletedAt] = append(a.restored[deletedAt], ids...)
	return a.rows(ids), nil
}

// A unit's restored attachments come back by the time they were deleted
// at, a restored page asking nothing; a row the purge took, fewer rows
// than attachments, is a trash item changed (M8/P2 design 3.6).
func TestTheAssetObserverRestoresTheAttachmentsByTheirTime(t *testing.T) {
	first, second := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC), time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)
	gone, pageID, a, b, c := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	state := page.TreeState{Name: "x"}
	e := page.Event{At: second.Add(time.Hour), Changes: []page.Change{
		{NodeID: gone, Before: &state},
		{NodeID: pageID, After: &state, Restored: &page.Restored{DeletedAt: first}},
		{NodeID: a, After: &state, Restored: &page.Restored{DeletedAt: first, Asset: true}},
		{NodeID: b, After: &state, Restored: &page.Restored{DeletedAt: second.In(time.FixedZone("x", 3600)), Asset: true}},
		{NodeID: c, After: &state, Restored: &page.Restored{DeletedAt: first, Asset: true}},
	}}
	assets := &restoringAssets{restored: map[time.Time][]uuid.UUID{}, rows: func(ids []uuid.UUID) int { return len(ids) }}
	if err := (assetObserver{assets}).PagesChanged(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(assets.deleted, []uuid.UUID{gone}) || len(assets.restored) != 2 ||
		!slices.Equal(assets.restored[first], []uuid.UUID{a, c}) || !slices.Equal(assets.restored[second], []uuid.UUID{b}) {
		t.Errorf("deleted %v, restored %v; want the deleted node, the attachments by their time", assets.deleted, assets.restored)
	}
	assets = &restoringAssets{restored: map[time.Time][]uuid.UUID{}, rows: func(ids []uuid.UUID) int { return len(ids) - 1 }}
	if err := (assetObserver{assets}).PagesChanged(context.Background(), e); !errors.Is(err, page.ErrTrashChanged) {
		t.Errorf("PagesChanged(a row the purge took) = %v, want page.trash_changed", err)
	}
}
