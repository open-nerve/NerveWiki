package markdown_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// A take within the budget's free bytes holds them at once; one beyond
// them waits for a release; nothing is taken for an empty content, and a
// content larger than the budget takes all of it.
func TestABudgetHoldsWhatIsTaken(t *testing.T) {
	b := markdown.NewBudget(10, time.Minute, slog.New(slog.DiscardHandler))
	ctx := context.Background()
	release6, err := b.Take(ctx, 6)
	if err != nil {
		t.Fatal(err)
	}
	if release, err := b.Take(ctx, 0); err != nil {
		t.Fatalf("an empty content: %v", err)
	} else {
		release()
	}
	got := make(chan error, 1)
	go func() {
		release, err := b.Take(ctx, 20)
		if err == nil {
			release()
		}
		got <- err
	}()
	select {
	case err := <-got:
		t.Fatalf("all of the budget taken while 6 bytes are held: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	release6()
	release6() // a second release gives nothing back
	select {
	case err := <-got:
		if err != nil {
			t.Fatalf("all of the budget after the release: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("all of the budget not taken 5s after the release")
	}
	release, err := b.Take(ctx, 10)
	if err != nil {
		t.Fatalf("the budget after both releases: %v", err)
	}
	release()
}

// A take that does not get its bytes within the wait is busy, and logs it;
// a cancelled request, or one whose own deadline comes first, gets its
// context's error: the request ran out, the server is not busy.
func TestABudgetThatDoesNotFreeUpIsBusy(t *testing.T) {
	var logs bytes.Buffer
	b := markdown.NewBudget(10, 20*time.Millisecond, slog.New(slog.NewTextHandler(&logs, nil)))
	release, err := b.Take(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := b.Take(context.Background(), 1); !errors.Is(err, markdown.ErrBusy) ||
		!strings.Contains(logs.String(), "content parsing is saturated") {
		t.Errorf("Take = %v, logs %q; want busy, logged", err, logs.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.Take(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Errorf("Take with a cancelled context = %v, want its error", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, err := b.Take(ctx, 1); !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, markdown.ErrBusy) {
		t.Errorf("Take with a deadline before the wait's = %v, want the context's error", err)
	}
}

// A budget of nothing, or no wait, is a fault of the caller's: it would
// bound nothing, or refuse every wait (P4 fix check, finding 2).
func TestABudgetOfNothingIsRefused(t *testing.T) {
	for _, tt := range []struct {
		name string
		size int
		wait time.Duration
	}{{"no bytes", 0, time.Second}, {"no wait", 1, 0}} {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("NewBudget(%d, %s) did not panic", tt.size, tt.wait)
				}
			}()
			markdown.NewBudget(tt.size, tt.wait, slog.New(slog.DiscardHandler))
		})
	}
}
