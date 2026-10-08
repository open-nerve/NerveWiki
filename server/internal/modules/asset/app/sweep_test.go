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

// storedFiles are files by key with their times, listed in the order
// they were added; undeletable's Delete fails.
type storedFiles struct {
	keys        []string
	at          map[string]time.Time
	deleted     []string
	undeletable string
	before      time.Time
}

func (s *storedFiles) add(key string, at time.Time) {
	s.keys = append(s.keys, key)
	s.at[key] = at
}

func (s *storedFiles) List(_ context.Context, area string, before time.Time, each func(string) error) error {
	s.before = before
	for _, k := range s.keys {
		if strings.HasPrefix(k, area+"/") && s.at[k].Before(before) {
			if err := each(k); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *storedFiles) Delete(_ context.Context, key string) error {
	if key == s.undeletable {
		return errors.New("remove " + key + ": permission denied")
	}
	s.deleted = append(s.deleted, key)
	return nil
}

// knownRows are the blobs with a row; reads counts its calls.
type knownRows struct {
	ids   map[uuid.UUID]bool
	reads int
	err   error
}

func (k *knownRows) KnownBlobs(_ context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	k.reads++
	var out []uuid.UUID
	for _, id := range ids {
		if k.ids[id] {
			out = append(out, id)
		}
	}
	return out, k.err
}

// The sweep deletes the files older than a day that no row holds, deleted
// or not, 500 at a read of the rows; it leaves a newer file, and a file of
// the area that is no blob's, logged as a warning.
func TestSweepDeletesTheOldFilesNoRowHolds(t *testing.T) {
	files, rows := &storedFiles{at: map[string]time.Time{}}, &knownRows{ids: map[uuid.UUID]bool{}}
	old := now().Add(-app.SweepAge - time.Minute)
	var orphans []string
	for i := range 1001 {
		id := uuid.NewV7()
		files.add(domain.Key(id), old)
		if i%2 == 0 {
			rows.ids[id] = true // a row, deleted or not
		} else {
			orphans = append(orphans, domain.Key(id))
		}
	}
	files.add(domain.Key(uuid.NewV7()), now().Add(-app.SweepAge+time.Minute))
	files.add("blobs/notes.txt", old)
	var logs bytes.Buffer
	n, err := app.NewSweep(files, rows, fixedClock{now()}, slog.New(slog.NewTextHandler(&logs, nil))).Run(context.Background())
	if err != nil || n != len(orphans) || !slices.Equal(files.deleted, orphans) || rows.reads != 3 {
		t.Errorf("Run() = %d, %v after %d reads, deleted %d files; want the %d orphans, in 3 reads", n, err, rows.reads, len(files.deleted),
			len(orphans))
	}
	if !files.before.Equal(now().Add(-app.SweepAge)) {
		t.Errorf("listed before %v, want a day ago", files.before)
	}
	if l := logs.String(); !strings.Contains(l, "files=500") || !strings.Contains(l, "level=WARN") || !strings.Contains(l, "notes.txt") {
		t.Errorf("logs %q, want the count and a warning of notes.txt", l)
	}
}

// A file the sweep cannot delete is logged as a warning, with its blob's
// id; the sweep deletes the rest, then fails, counting it.
func TestSweepGoesOnPastAFileItCannotDelete(t *testing.T) {
	files := &storedFiles{at: map[string]time.Time{}}
	var keys []string
	for range 3 {
		keys = append(keys, domain.Key(uuid.NewV7()))
		files.add(keys[len(keys)-1], now().Add(-48*time.Hour))
	}
	files.undeletable = keys[1]
	var logs bytes.Buffer
	n, err := app.NewSweep(files, &knownRows{}, fixedClock{now()}, slog.New(slog.NewTextHandler(&logs, nil))).Run(context.Background())
	if n != 2 || err == nil || !strings.Contains(err.Error(), "1 orphan attachment files not deleted") ||
		!slices.Equal(files.deleted, []string{keys[0], keys[2]}) {
		t.Errorf("Run() = %d, %v, deleted %q; want the other two, then the failure", n, err, files.deleted)
	}
	if l := logs.String(); !strings.Contains(l, "level=WARN") || !strings.Contains(l, "blob_id="+strings.TrimPrefix(keys[1], domain.Area+"/")) {
		t.Errorf("logs %q, want the failure as a warning, with the blob's id", l)
	}
}

// The rows' failure stops the sweep, no file deleted after.
func TestSweepStopsAtTheRowsFailure(t *testing.T) {
	files := &storedFiles{at: map[string]time.Time{}}
	files.add(domain.Key(uuid.NewV7()), now().Add(-48*time.Hour))
	broken := errors.New("connection reset")
	if _, err := app.NewSweep(files, &knownRows{err: broken}, fixedClock{now()}, slog.New(slog.DiscardHandler)).Run(context.Background()); !errors.Is(err, broken) || len(files.deleted) != 0 {
		t.Errorf("Run() = %v, deleted %q; want the rows' failure, nothing deleted", err, files.deleted)
	}
}
