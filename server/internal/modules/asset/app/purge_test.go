package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
)

// txKey marks the context of oneTx's transaction.
type txKey struct{}

// oneTx runs fn as a transaction, in a context of its own: fn's error
// rolls it back; the commit of the rest fails with commitErr.
type oneTx struct {
	rolledBack bool
	commitErr  error
}

func (t *oneTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := fn(context.WithValue(ctx, txKey{}, true)); err != nil {
		t.rolledBack = true
		return err
	}
	return t.commitErr
}

// expiredRows holds the ids of the rows past the retention; DeleteBlobs
// keeps what it was given, and the files deleted by then, and counts its
// calls. err fails ExpiredBlobs, deleteErr DeleteBlobs; outside names the
// calls made outside the transaction.
type expiredRows struct {
	ids       []uuid.UUID
	before    time.Time
	batch     int
	deleted   []uuid.UUID
	deletes   int
	files     *memFiles
	gone      int
	err       error
	deleteErr error
	outside   []string
}

func (r *expiredRows) ExpiredBlobs(ctx context.Context, before time.Time, batch int) ([]uuid.UUID, error) {
	if ctx.Value(txKey{}) == nil {
		r.outside = append(r.outside, "ExpiredBlobs")
	}
	r.before, r.batch = before, batch
	return r.ids, r.err
}

func (r *expiredRows) DeleteBlobs(ctx context.Context, ids []uuid.UUID) (int, error) {
	if ctx.Value(txKey{}) == nil {
		r.outside = append(r.outside, "DeleteBlobs")
	}
	r.deletes++
	r.deleted = slices.Clone(ids)
	if r.files != nil {
		r.gone = len(r.files.deleted)
	}
	if r.deleteErr != nil {
		return 0, r.deleteErr
	}
	return len(ids), nil
}

// A batch deletes the expired rows' files, then the rows, in its
// transaction, and tells how many; a file already gone is no failure.
func TestPurgeDeletesTheFilesThenTheRows(t *testing.T) {
	files, tx := newFiles(), &oneTx{}
	a, b := uuid.NewV7(), uuid.NewV7()
	files.files[domain.Key(a)] = []byte("a")
	rows := &expiredRows{ids: []uuid.UUID{a, b}, files: files}
	before := now().Add(-24 * time.Hour)
	n, err := app.NewPurge(tx, rows, files, slog.New(slog.DiscardHandler)).Batch(context.Background(), before, 10)
	if err != nil || n != 2 || tx.rolledBack {
		t.Fatalf("Batch() = %d, %v, rolled back %v; want 2", n, err, tx.rolledBack)
	}
	if !rows.before.Equal(before) || rows.batch != 10 || !slices.Equal(rows.deleted, rows.ids) || rows.gone != 2 || len(rows.outside) != 0 {
		t.Errorf("asked for %d before %v, deleted %v after %d files, outside the transaction %q; want 10 before %v, both after both files, "+
			"none outside", rows.batch, rows.before, rows.deleted, rows.gone, rows.outside, before)
	}
	if want := []string{domain.Key(a), domain.Key(b)}; !slices.Equal(files.deleted, want) || len(files.keys()) != 0 {
		t.Errorf("files deleted %q, left %q; want %q, none", files.deleted, files.keys(), want)
	}
}

// A batch with no row past the retention deletes nothing, rows included.
func TestPurgeOfNoRowsDeletesNothing(t *testing.T) {
	rows := &expiredRows{}
	if n, err := app.NewPurge(&oneTx{}, rows, newFiles(), slog.New(slog.DiscardHandler)).Batch(context.Background(), now(), 10); err != nil ||
		n != 0 || rows.deletes != 0 {
		t.Errorf("Batch() = %d, %v after %d deletions of rows; want 0, none", n, err, rows.deletes)
	}
}

// A file that is not deleted fails the batch with its failure, logged
// with its blob: the transaction rolls back, and no row is deleted.
func TestPurgeStopsAtAFileNotDeleted(t *testing.T) {
	files, tx := newFiles(), &oneTx{}
	a, b := uuid.NewV7(), uuid.NewV7()
	files.undeletable = domain.Key(b)
	rows := &expiredRows{ids: []uuid.UUID{a, b}}
	var logs bytes.Buffer
	n, err := app.NewPurge(tx, rows, files, slog.New(slog.NewTextHandler(&logs, nil))).Batch(context.Background(), now(), 10)
	if !errors.Is(err, errDenied) || n != 0 || !tx.rolledBack || rows.deleted != nil {
		t.Errorf("Batch() = %d, %v, rolled back %v, rows deleted %v; want the failure, rolled back, none", n, err, tx.rolledBack, rows.deleted)
	}
	if l := logs.String(); !strings.Contains(l, "level=ERROR") || !strings.Contains(l, "blob_id="+b.String()) {
		t.Errorf("logs %q, want an error naming blob %s", l, b)
	}
}

// The rows' read failing, their deletion's and the commit's are the
// batch's failure, of no rows deleted.
func TestPurgeFailsWithItsRowsAndItsCommit(t *testing.T) {
	for _, tt := range []struct {
		name string
		rows *expiredRows
		tx   *oneTx
	}{
		{"the rows' read", &expiredRows{err: errPort}, &oneTx{}},
		{"the rows' deletion", &expiredRows{ids: []uuid.UUID{uuid.NewV7()}, deleteErr: errPort}, &oneTx{}},
		{"the commit", &expiredRows{ids: []uuid.UUID{uuid.NewV7()}}, &oneTx{commitErr: errPort}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if n, err := app.NewPurge(tt.tx, tt.rows, newFiles(), slog.New(slog.DiscardHandler)).Batch(context.Background(), now(), 10); !errors.Is(err, errPort) || n != 0 {
				t.Errorf("Batch() = %d, %v; want 0 and the failure", n, err)
			}
		})
	}
}
