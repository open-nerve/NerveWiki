package app_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
)

// deletions keeps what it is asked to delete, and when.
type deletions struct {
	nodes, notebooks []uuid.UUID
	at               []time.Time
}

func (d *deletions) DeleteBlobsOfNodes(_ context.Context, ids []uuid.UUID, at time.Time) error {
	d.nodes, d.at = append(d.nodes, ids...), append(d.at, at)
	return nil
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
