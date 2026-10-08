package postgresadapter_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
)

// The expired rows are those deleted before the time, the oldest deletions
// first, up to the batch, but those another transaction holds, which it
// does not wait for: a wait fails at the deadline. DeleteBlobs deletes the
// rows it is given.
func TestExpiredBlobsAndTheirDeletion(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store := postgresadapter.New(f.pool)
	photo, report := f.blob(f.photo, f.eng, 0, 0), f.blob(f.report, f.ops, 0, 0)
	for _, b := range []struct {
		blob domain.Blob
		ago  string
	}{{report, "2 days"}, {photo, "3 days"}} {
		if err := store.CreateBlob(ctx, b.blob); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Exec(ctx, "UPDATE asset_blobs SET deleted_at = now() - $2::interval WHERE id = $1", b.blob.ID, b.ago); err != nil {
			t.Fatal(err)
		}
	}
	day := func(n float64) time.Time { return time.Now().Add(-time.Duration(n * float64(24*time.Hour))) }
	for _, tt := range []struct {
		name   string
		before time.Time
		batch  int
		want   []uuid.UUID
	}{
		{"both, the oldest first", day(1), 10, []uuid.UUID{photo.ID, report.ID}},
		{"a batch of one", day(1), 1, []uuid.UUID{photo.ID}},
		{"one old enough", day(2.5), 10, []uuid.UUID{photo.ID}},
		{"none old enough", day(4), 10, nil},
	} {
		if got, err := store.ExpiredBlobs(ctx, tt.before, tt.batch); err != nil || !slices.Equal(got, tt.want) {
			t.Errorf("%s: ExpiredBlobs() = %v, %v; want %v", tt.name, got, err, tt.want)
		}
	}

	held, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Rollback(ctx) }()
	if _, err := held.Exec(ctx, "SELECT 1 FROM asset_blobs WHERE id = $1 FOR UPDATE", photo.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := store.ExpiredBlobs(ctx, day(1), 10); err != nil || !slices.Equal(got, []uuid.UUID{report.ID}) {
		t.Errorf("ExpiredBlobs() with photo's row held = %v, %v; want report's only", got, err)
	}
	_ = held.Rollback(ctx)

	if n, err := store.DeleteBlobs(ctx, []uuid.UUID{photo.ID}); err != nil || n != 1 {
		t.Errorf("DeleteBlobs() = %d, %v; want 1", n, err)
	}
	var left []uuid.UUID
	rows, err := f.pool.Query(ctx, "SELECT id FROM asset_blobs ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		left = append(left, id)
	}
	if rows.Err() != nil || !slices.Equal(left, []uuid.UUID{report.ID}) {
		t.Errorf("rows left %v, %v; want report's", left, rows.Err())
	}
}
