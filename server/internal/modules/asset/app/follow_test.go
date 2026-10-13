package app_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
)

// deletions keeps what it is asked to delete and restore, and when; a
// restore answers rows.
type deletions struct {
	nodes, notebooks, restored []uuid.UUID
	at                         []time.Time
	rows                       int
}

func (d *deletions) DeleteBlobsOfNodes(_ context.Context, ids []uuid.UUID, at time.Time) error {
	d.nodes, d.at = append(d.nodes, ids...), append(d.at, at)
	return nil
}

func (d *deletions) RestoreBlobsOfNodes(_ context.Context, ids []uuid.UUID, deletedAt time.Time) (int, error) {
	d.restored, d.at = append(d.restored, ids...), append(d.at, deletedAt)
	return d.rows, nil
}

func (d *deletions) DeleteBlobsOfNotebooks(_ context.Context, ids []uuid.UUID, at time.Time) error {
	d.notebooks, d.at = append(d.notebooks, ids...), append(d.at, at)
	return nil
}

// A deletion of nodes or notebooks deletes their rows at its time;
// nothing deleted asks nothing.
func TestFollowDeletesTheRowsOfWhatWasDeleted(t *testing.T) {
	d := &deletions{}
	f := app.NewFollow(d)
	nodes, notebooks := []uuid.UUID{uuid.NewV7(), uuid.NewV7()}, []uuid.UUID{uuid.NewV7()}
	ctx := context.Background()
	for _, err := range []error{f.NodesDeleted(ctx, nodes, now()), f.NotebooksDeleted(ctx, notebooks, now().Add(time.Hour)),
		f.NodesDeleted(ctx, nil, now()), f.NotebooksDeleted(ctx, nil, now())} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(d.nodes, nodes) || !slices.Equal(d.notebooks, notebooks) || len(d.at) != 2 || !d.at[0].Equal(now()) ||
		!d.at[1].Equal(now().Add(time.Hour)) {
		t.Errorf("deleted nodes %v, notebooks %v at %v; want each once at its time", d.nodes, d.notebooks, d.at)
	}
}

// A restore of nodes brings back their rows deleted at their time and
// tells how many, as many as the store found: one of two here; nothing
// restored asks nothing.
func TestFollowRestoresTheRowsOfWhatWasRestored(t *testing.T) {
	d := &deletions{rows: 1}
	f := app.NewFollow(d)
	nodes := []uuid.UUID{uuid.NewV7(), uuid.NewV7()}
	ctx := context.Background()
	if n, err := f.NodesRestored(ctx, nodes, now()); err != nil || n != 1 {
		t.Errorf("NodesRestored() = %d, %v; want the rows' 1", n, err)
	}
	if n, err := f.NodesRestored(ctx, nil, now()); err != nil || n != 0 {
		t.Errorf("NodesRestored(none) = %d, %v; want 0", n, err)
	}
	if !slices.Equal(d.restored, nodes) || len(d.at) != 1 || !d.at[0].Equal(now()) {
		t.Errorf("restored %v at %v; want the nodes once at their time", d.restored, d.at)
	}
}
