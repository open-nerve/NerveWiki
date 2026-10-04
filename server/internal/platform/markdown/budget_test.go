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
	six, err := b.Take(ctx, 6)
	if err != nil {
		t.Fatal(err)
	}
	if empty, err := b.Take(ctx, 0); err != nil {
		t.Fatalf("an empty content: %v", err)
	} else {
		empty.Release()
	}
	got := make(chan error, 1)
	go func() {
		all, err := b.Take(ctx, 20)
		if err == nil {
			all.Release()
		}
		got <- err
	}()
	select {
	case err := <-got:
		t.Fatalf("all of the budget taken while 6 bytes are held: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	six.Release()
	six.Release() // a second release gives nothing back
	select {
	case err := <-got:
		if err != nil {
			t.Fatalf("all of the budget after the release: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("all of the budget not taken 5s after the release")
	}
	all, err := b.Take(ctx, 10)
	if err != nil {
		t.Fatalf("the budget after both releases: %v", err)
	}
	all.Release()
}

// A content's facts keep a tenth of its bytes once its parse is done
// (FactsRatio of parseRatio, rounded up), until the release, and a share for
// each value of its frontmatter; a content larger than the budget keeps no
// more than all of it; a negative size holds and keeps nothing.
func TestAHoldKeepsItsFactsShare(t *testing.T) {
	b := markdown.NewBudget(100, 20*time.Millisecond, slog.New(slog.DiscardHandler))
	ctx := context.Background()
	h, err := b.Take(ctx, 91)
	if err != nil {
		t.Fatal(err)
	}
	h.KeepFacts(markdown.Facts{})
	h.KeepFacts(markdown.Facts{}) // keeping again gives nothing more back
	rest, err := b.Take(ctx, 90)
	if err != nil {
		t.Fatalf("the budget beside a kept tenth of 91 bytes: %v", err)
	}
	if _, err := b.Take(ctx, 1); !errors.Is(err, markdown.ErrBusy) {
		t.Errorf("a byte beside them = %v, want busy: the facts keep 10 bytes", err)
	}
	h.Release()
	if one, err := b.Take(ctx, 1); err != nil {
		t.Errorf("a byte once the facts are released: %v", err)
	} else {
		one.Release()
	}
	rest.Release()
	large, err := b.Take(ctx, 5000)
	if err != nil {
		t.Fatal(err)
	}
	large.KeepFacts(markdown.Facts{})
	if _, err := b.Take(ctx, 1); !errors.Is(err, markdown.ErrBusy) {
		t.Errorf("a byte beside the facts of a content ten times the budget = %v, want busy", err)
	}
	large.Release()

	m, err := markdown.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	// 21 bytes and four values: 30 times 21 and 400 times four, 2,230
	// bytes of facts, keep 8 bytes of the budget.
	content := "---\na: [x, x, x]\n---\n"
	fm, err := b.Take(ctx, len(content))
	if err != nil {
		t.Fatal(err)
	}
	fm.KeepFacts(m.Parse([]byte(content)).Facts())
	if rest, err := b.Take(ctx, 92); err != nil {
		t.Errorf("the budget beside the facts of a frontmatter of four values: %v", err)
	} else {
		rest.Release()
	}
	if _, err := b.Take(ctx, 93); !errors.Is(err, markdown.ErrBusy) {
		t.Errorf("a byte more = %v, want busy: the facts keep 8 bytes", err)
	}
	fm.Release()

	all, err := b.Take(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	negative, err := b.Take(ctx, -1000)
	if err != nil {
		t.Fatal(err)
	}
	negative.KeepFacts(markdown.Facts{})
	negative.Release()
	if _, err := b.Take(ctx, 1); !errors.Is(err, markdown.ErrBusy) {
		t.Errorf("a byte once a negative size kept and released = %v, want busy: it gave back what it never held", err)
	}
	all.Release()
}

// A take that does not get its bytes within the wait is busy, and logs it;
// a cancelled request, or one whose own deadline comes first, gets its
// context's error: the request ran out, the server is not busy.
func TestABudgetThatDoesNotFreeUpIsBusy(t *testing.T) {
	var logs bytes.Buffer
	b := markdown.NewBudget(10, 20*time.Millisecond, slog.New(slog.NewTextHandler(&logs, nil)))
	all, err := b.Take(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	defer all.Release()
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
// bound nothing, or refuse every wait (M4/P4 fix check, finding 2).
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
