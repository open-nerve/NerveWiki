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
// they were added while the context lasts, then failing with listErr; the
// deletion of each key of failing fails with its error.
type storedFiles struct {
	keys    []string
	at      map[string]time.Time
	deleted []string
	failing map[string]error
	listErr error
	before  time.Time
}

func newStoredFiles() *storedFiles { return &storedFiles{at: map[string]time.Time{}} }

func (s *storedFiles) add(key string, at time.Time) {
	s.keys = append(s.keys, key)
	s.at[key] = at
}

// orphans adds n files of blobs two days old, and answers their keys.
func (s *storedFiles) orphans(n int) []string {
	var keys []string
	for range n {
		keys = append(keys, domain.Key(uuid.NewV7()))
		s.add(keys[len(keys)-1], now().Add(-48*time.Hour))
	}
	return keys
}

func (s *storedFiles) List(ctx context.Context, area string, before time.Time, each func(string) error) error {
	s.before = before
	for _, k := range s.keys {
		if err := ctx.Err(); err != nil {
			return err
		}
		if strings.HasPrefix(k, area+"/") && s.at[k].Before(before) {
			if err := each(k); err != nil {
				return err
			}
		}
	}
	return s.listErr
}

func (s *storedFiles) Delete(_ context.Context, key string) error {
	if err, ok := s.failing[key]; ok {
		return err
	}
	s.deleted = append(s.deleted, key)
	return nil
}

// knownRows are the blobs with a row; sizes are the counts of the ids of
// each read, the failAt-th of which, counted from 1, fails with err.
type knownRows struct {
	ids    map[uuid.UUID]bool
	sizes  []int
	failAt int
	err    error
}

func (k *knownRows) KnownBlobs(_ context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	k.sizes = append(k.sizes, len(ids))
	if len(k.sizes) == k.failAt {
		return nil, k.err
	}
	var out []uuid.UUID
	for _, id := range ids {
		if k.ids[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// The sweep deletes the files older than a day that no row holds, deleted
// or not, 500 at a read of the rows; it leaves a newer file, and a file of
// the area that is no blob's, logged as a warning. It logs how many it
// deleted.
func TestSweepDeletesTheOldFilesNoRowHolds(t *testing.T) {
	files, rows := newStoredFiles(), &knownRows{ids: map[uuid.UUID]bool{}}
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
	if err != nil || n != len(orphans) || !slices.Equal(files.deleted, orphans) || !slices.Equal(rows.sizes, []int{500, 500, 1}) {
		t.Errorf("Run() = %d, %v after reads of %v, deleted %d files; want the %d orphans, in reads of 500, 500, 1", n, err, rows.sizes,
			len(files.deleted), len(orphans))
	}
	if !files.before.Equal(now().Add(-app.SweepAge)) {
		t.Errorf("listed before %v, want a day ago", files.before)
	}
	if l := logs.String(); !strings.Contains(l, `level=INFO msg="orphan attachment files deleted" files=500`) ||
		!strings.Contains(l, "level=WARN") || !strings.Contains(l, "notes.txt") {
		t.Errorf("logs %q, want the count at INFO and a warning of notes.txt", l)
	}
}

// A sweep that deletes nothing logs nothing.
func TestSweepOfNothingLogsNothing(t *testing.T) {
	var logs bytes.Buffer
	files := newStoredFiles()
	files.add(domain.Key(uuid.NewV7()), now())
	if n, err := app.NewSweep(files, &knownRows{}, fixedClock{now()}, slog.New(slog.NewTextHandler(&logs, nil))).Run(context.Background()); err != nil ||
		n != 0 || logs.Len() != 0 {
		t.Errorf("Run() = %d, %v, logging %q; want nothing", n, err, logs.String())
	}
}

// A file the sweep cannot delete is logged as a warning, with its blob's
// id; the sweep deletes the rest, then fails with the first failure,
// counting them, and logs the count of the files deleted.
func TestSweepGoesOnPastAFileItCannotDelete(t *testing.T) {
	files := newStoredFiles()
	keys := files.orphans(4)
	first, second := errors.New("first: permission denied"), errors.New("second: permission denied")
	files.failing = map[string]error{keys[1]: first, keys[2]: second}
	var logs bytes.Buffer
	n, err := app.NewSweep(files, &knownRows{}, fixedClock{now()}, slog.New(slog.NewTextHandler(&logs, nil))).Run(context.Background())
	if n != 2 || !errors.Is(err, first) || errors.Is(err, second) || !strings.Contains(err.Error(), "2 orphan attachment files not deleted") ||
		!slices.Equal(files.deleted, []string{keys[0], keys[3]}) {
		t.Errorf("Run() = %d, %v, deleted %q; want the other two, then the first failure", n, err, files.deleted)
	}
	l := logs.String()
	for _, k := range keys[1:3] {
		if !strings.Contains(l, "level=WARN") || !strings.Contains(l, "blob_id="+strings.TrimPrefix(k, domain.Area+"/")) {
			t.Errorf("logs %q, want the failure of %s as a warning, with the blob's id", l, k)
		}
	}
	if !strings.Contains(l, "files=2") {
		t.Errorf("logs %q, want the two deleted counted", l)
	}
}

// The rows' failure, the walk's, and the end of the context stop the
// sweep, as the run's failure, whatever deletion failed before: no file
// is deleted after.
func TestSweepStopsAtItsFailures(t *testing.T) {
	for _, tt := range []struct {
		name    string
		setup   func(files *storedFiles, rows *knownRows)
		ctx     func() context.Context
		want    error
		deleted int
	}{
		{"the rows' read", func(f *storedFiles, r *knownRows) {
			f.orphans(1)
			r.failAt, r.err = 1, errPort
		}, context.Background, errPort, 0},
		{"the rows' read after a deletion failed", func(f *storedFiles, r *knownRows) {
			keys := f.orphans(501)
			f.failing = map[string]error{keys[0]: errDenied}
			r.failAt, r.err = 2, errPort
		}, context.Background, errPort, 499},
		{"the walk", func(f *storedFiles, _ *knownRows) {
			f.orphans(3)
			f.listErr = errPort
		}, context.Background, errPort, 0},
		{"the context", func(f *storedFiles, _ *knownRows) { f.orphans(3) }, func() context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}, context.Canceled, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files, rows := newStoredFiles(), &knownRows{}
			tt.setup(files, rows)
			if _, err := app.NewSweep(files, rows, fixedClock{now()}, slog.New(slog.DiscardHandler)).Run(tt.ctx()); !errors.Is(err, tt.want) ||
				len(files.deleted) != tt.deleted {
				t.Errorf("Run() = %v, deleted %d; want %v, %d deleted", err, len(files.deleted), tt.want, tt.deleted)
			}
		})
	}
}
