package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
)

// batchDeleter answers each call with the next of counts, or err once
// counts run out; it records the arguments of every call.
type batchDeleter struct {
	counts []int
	err    error
	calls  []time.Time
	limits []int
}

func (d *batchDeleter) DeleteExpiredSessions(_ context.Context, now time.Time, limit int) (int, error) {
	d.calls, d.limits = append(d.calls, now), append(d.limits, limit)
	if len(d.counts) == 0 {
		return 0, d.err
	}
	n := d.counts[0]
	d.counts = d.counts[1:]
	return n, nil
}

// The cleanup deletes batch after batch at one instant, until a batch comes
// back short, and logs how many it deleted.
func TestCleanupSessionsDeletesInBatches(t *testing.T) {
	d := &batchDeleter{counts: []int{1000, 1000, 3}}
	var logs bytes.Buffer
	uc := app.NewCleanupSessions(d, fixedClock(testNow()), slog.New(slog.NewJSONHandler(&logs, nil)))

	n, err := uc.Execute(context.Background())

	if n != 2003 || err != nil || len(d.calls) != 3 {
		t.Errorf("Execute() = %d, %v after %d batches; want 2003 after 3", n, err, len(d.calls))
	}
	for i := range d.calls {
		if !d.calls[i].Equal(testNow()) || d.limits[i] != 1000 {
			t.Errorf("batch %d at %v of %d, want the clock's now and 1000", i, d.calls[i], d.limits[i])
		}
	}
	if out := logs.String(); !strings.Contains(out, `"msg":"expired sessions deleted"`) || !strings.Contains(out, `"deleted":2003`) {
		t.Errorf("logs = %s, want the count deleted", out)
	}
}

// A failed batch ends the cleanup with its error and the count of the
// batches before it; a run that deletes nothing logs nothing.
func TestCleanupSessionsStops(t *testing.T) {
	boom := errors.New("connection reset")
	d := &batchDeleter{counts: []int{1000}, err: boom}
	uc := app.NewCleanupSessions(d, fixedClock(testNow()), slog.New(slog.DiscardHandler))
	if n, err := uc.Execute(context.Background()); n != 1000 || !errors.Is(err, boom) {
		t.Errorf("Execute() = %d, %v; want 1000 and the batch's error", n, err)
	}

	var logs bytes.Buffer
	uc = app.NewCleanupSessions(&batchDeleter{counts: []int{0}}, fixedClock(testNow()), slog.New(slog.NewJSONHandler(&logs, nil)))
	if n, err := uc.Execute(context.Background()); n != 0 || err != nil || logs.Len() != 0 {
		t.Errorf("Execute() = %d, %v with logs %q; want 0, nil and no log", n, err, logs.String())
	}
}
