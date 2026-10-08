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

// oneTx runs fn as a transaction: its error rolls it back.
type oneTx struct{ rolledBack bool }

func (t *oneTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := fn(ctx); err != nil {
		t.rolledBack = true
		return err
	}
	return nil
}

// expiredRows holds the ids of the rows past the retention; DeleteBlobs
// keeps what it was given, and the files deleted by then.
type expiredRows struct {
	ids     []uuid.UUID
	before  time.Time
	batch   int
	deleted []uuid.UUID
	files   *memFiles
	gone    int
	err     error
}

func (r *expiredRows) ExpiredBlobs(_ context.Context, before time.Time, batch int) ([]uuid.UUID, error) {
	r.before, r.batch = before, batch
	return r.ids, r.err
}

func (r *expiredRows) DeleteBlobs(_ context.Context, ids []uuid.UUID) (int, error) {
	r.deleted = slices.Clone(ids)
	if r.files != nil {
		r.gone = len(r.files.deleted)
	}
	return len(ids), nil
}

// A batch deletes the expired rows' files, then the rows, and tells how
// many; a file already gone is no failure.
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
	if !rows.before.Equal(before) || rows.batch != 10 || !slices.Equal(rows.deleted, rows.ids) || rows.gone != 2 {
		t.Errorf("asked for %d before %v, deleted %v after %d files; want 10 before %v, both after both files",
			rows.batch, rows.before, rows.deleted, rows.gone, before)
	}
	if want := []string{domain.Key(a), domain.Key(b)}; !slices.Equal(files.deleted, want) || len(files.keys()) != 0 {
		t.Errorf("files deleted %q, left %q; want %q, none", files.deleted, files.keys(), want)
	}
}

// A file that is not deleted fails the batch, logged with its blob: the
// transaction rolls back, and no row is deleted.
func TestPurgeStopsAtAFileNotDeleted(t *testing.T) {
	files, tx := newFiles(), &oneTx{}
	a, b := uuid.NewV7(), uuid.NewV7()
	files.undeletable = domain.Key(b)
	rows := &expiredRows{ids: []uuid.UUID{a, b}}
	var logs bytes.Buffer
	n, err := app.NewPurge(tx, rows, files, slog.New(slog.NewTextHandler(&logs, nil))).Batch(context.Background(), now(), 10)
	if err == nil || n != 0 || !tx.rolledBack || rows.deleted != nil {
		t.Errorf("Batch() = %d, %v, rolled back %v, rows deleted %v; want the failure, rolled back, none", n, err, tx.rolledBack, rows.deleted)
	}
	if l := logs.String(); !strings.Contains(l, "level=ERROR") || !strings.Contains(l, "blob_id="+b.String()) {
		t.Errorf("logs %q, want an error naming blob %s", l, b)
	}
}

// The rows' failure is the batch's.
func TestPurgeFailsWithItsRows(t *testing.T) {
	broken := errors.New("connection reset")
	rows, tx := &expiredRows{err: broken}, &oneTx{}
	if n, err := app.NewPurge(tx, rows, newFiles(), slog.New(slog.DiscardHandler)).Batch(context.Background(), now(), 10); !errors.Is(err, broken) || n != 0 {
		t.Errorf("Batch() = %d, %v; want the rows' failure", n, err)
	}
}
