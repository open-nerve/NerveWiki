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

// A deletion of nodes or notebooks deletes their rows not deleted, at its
// time, and leaves a row deleted before as it was; the activity counts
// the rows not deleted; the infos and the known blobs are those with a
// row, deleted or not.
func TestRowsFollowTheirNodesAndNotebooks(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	store := postgresadapter.New(f.pool)
	photo, report := f.blob(f.photo, f.eng, 0, 0), f.blob(f.report, f.ops, 0, 0)
	report.Bytes = 40
	for _, b := range []domain.Blob{photo, report} {
		if err := store.CreateBlob(ctx, b); err != nil {
			t.Fatal(err)
		}
	}
	activity, err := store.NotebookActivities(ctx, []uuid.UUID{f.eng, f.ops, uuid.NewV7()})
	if err != nil || len(activity) != 2 || activity[f.eng].Bytes != 3 || activity[f.ops].Bytes != 40 {
		t.Errorf("NotebookActivities() = %+v, %v; want eng's 3 bytes and ops' 40", activity, err)
	}
	first := time.Date(2026, 10, 8, 11, 0, 0, 0, time.UTC)
	if err := store.DeleteBlobsOfNodes(ctx, []uuid.UUID{f.photo, uuid.NewV7()}, first); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteBlobsOfNotebooks(ctx, []uuid.UUID{f.eng, f.ops}, first.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	// A deletion that reaches a row deleted already passes it by.
	if err := store.DeleteBlobsOfNodes(ctx, []uuid.UUID{f.photo}, first.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	deletedAt := func(id uuid.UUID) time.Time {
		var at time.Time
		if err := f.pool.QueryRow(ctx, "SELECT deleted_at FROM asset_blobs WHERE id = $1", id).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at
	}
	if !deletedAt(photo.ID).Equal(first) || !deletedAt(report.ID).Equal(first.Add(time.Hour)) {
		t.Errorf("deleted at %v and %v, want photo's by its node's at 11:00 alone, report's by its notebook's at 12:00",
			deletedAt(photo.ID), deletedAt(report.ID))
	}
	if activity, err := store.NotebookActivities(ctx, []uuid.UUID{f.eng, f.ops}); err != nil || len(activity) != 0 {
		t.Errorf("NotebookActivities() after the deletions = %+v, %v; want none", activity, err)
	}
	infos, err := store.AttachmentInfos(ctx, []uuid.UUID{f.photo, f.report, uuid.NewV7()})
	if err != nil || len(infos) != 2 || infos[f.photo].MIME != photo.MIME || infos[f.photo].Bytes != 3 || infos[f.report].Bytes != 40 {
		t.Errorf("AttachmentInfos() of the rows deleted = %+v, %v; want the photo's and the report's type and size", infos, err)
	}
	other := uuid.NewV7()
	known, err := store.KnownBlobs(ctx, []uuid.UUID{photo.ID, report.ID, other})
	slices.SortFunc(known, uuid.UUID.Compare)
	want := []uuid.UUID{photo.ID, report.ID}
	slices.SortFunc(want, uuid.UUID.Compare)
	if err != nil || !slices.Equal(known, want) {
		t.Errorf("KnownBlobs() = %v, %v; want photo's and report's, deleted", known, err)
	}
}

// A restore brings back the rows of its nodes deleted at its time, and
// tells how many: not a row deleted at another time, nor one not deleted.
func TestRestoreBlobsOfNodesTakesItsTimeAlone(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	store := postgresadapter.New(f.pool)
	photo, report := f.blob(f.photo, f.eng, 0, 0), f.blob(f.report, f.ops, 0, 0)
	for _, b := range []domain.Blob{photo, report} {
		if err := store.CreateBlob(ctx, b); err != nil {
			t.Fatal(err)
		}
	}
	at := time.Date(2026, 10, 8, 11, 0, 0, 0, time.UTC)
	if err := store.DeleteBlobsOfNodes(ctx, []uuid.UUID{f.photo}, at); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteBlobsOfNodes(ctx, []uuid.UUID{f.report}, at.Add(time.Microsecond)); err != nil {
		t.Fatal(err)
	}
	if n, err := store.RestoreBlobsOfNodes(ctx, []uuid.UUID{f.photo, f.report, uuid.NewV7()}, at); err != nil || n != 1 {
		t.Errorf("RestoreBlobsOfNodes(at the photo's time) = %d, %v; want the photo's row", n, err)
	}
	var alive, gone int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE id = $1 AND deleted_at IS NULL),
		count(*) FILTER (WHERE id = $2 AND deleted_at IS NOT NULL) FROM asset_blobs`, photo.ID, report.ID).Scan(&alive, &gone); err != nil {
		t.Fatal(err)
	}
	if alive != 1 || gone != 1 {
		t.Errorf("the photo's row back %d, the report's still deleted %d; want each 1", alive, gone)
	}
	if n, err := store.RestoreBlobsOfNodes(ctx, []uuid.UUID{f.photo}, at); err != nil || n != 0 {
		t.Errorf("RestoreBlobsOfNodes(a row not deleted) = %d, %v; want none", n, err)
	}
}
