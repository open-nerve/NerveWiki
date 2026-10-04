package markdown_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// A take within the budget's free bytes holds them at once; one beyond
// them waits for a release; nothing is taken for an empty content, and a
// content larger than the budget takes all of it.
func TestABudgetHoldsWhatIsTaken(t *testing.T) {
	const size = 10 * markdown.MinTake
	b := markdown.NewBudget(size, time.Minute, slog.New(slog.DiscardHandler))
	ctx := context.Background()
	six, err := b.Take(ctx, 6*markdown.MinTake)
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
		all, err := b.Take(ctx, 2*size)
		if err == nil {
			all.Release()
		}
		got <- err
	}()
	select {
	case err := <-got:
		t.Fatalf("all of the budget taken while six tenths are held: %v", err)
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
	all, err := b.Take(ctx, size)
	if err != nil {
		t.Fatalf("the budget after both releases: %v", err)
	}
	all.Release()
}

// A short content takes MinTake, however short (M6/P2 fix check 3 L1); a
// longer one its size.
func TestAShortContentTakesAtLeastMinTake(t *testing.T) {
	b := markdown.NewBudget(3*markdown.MinTake, 20*time.Millisecond, slog.New(slog.DiscardHandler))
	ctx := context.Background()
	var holds []*markdown.Hold
	for range 3 {
		h, err := b.Take(ctx, 1)
		if err != nil {
			t.Fatalf("a byte beside %d others: %v", len(holds), err)
		}
		holds = append(holds, h)
	}
	if _, err := b.Take(ctx, 1); !errors.Is(err, markdown.ErrBusy) {
		t.Errorf("a fourth byte = %v, want busy: each takes MinTake", err)
	}
	holds[0].Release()
	if _, err := b.Take(ctx, markdown.MinTake+1); !errors.Is(err, markdown.ErrBusy) {
		t.Errorf("a byte more than MinTake beside two of them = %v, want busy: it takes its size", err)
	}
	if h, err := b.Take(ctx, 1); err != nil {
		t.Errorf("a byte once one is released: %v", err)
	} else {
		h.Release()
	}
	for _, h := range holds[1:] {
		h.Release()
	}
}

// A content's facts keep their Limit's share of the budget once its parse is
// done (a parseRatio-th, rounded up: about a tenth of its bytes, and a share
// for each value of its frontmatter and its paths' bytes), until the
// release; but no more than the content took: all of the budget for a
// content larger than it, math.MaxInt bytes too, the limit not wrapping
// around; MinTake for a frontmatter whose aliases' share is more. A
// negative size holds and keeps nothing.
func TestAHoldKeepsItsFactsShare(t *testing.T) {
	const size = 20 * markdown.MinTake
	b := markdown.NewBudget(size, 20*time.Millisecond, slog.New(slog.DiscardHandler))
	ctx := context.Background()
	share := func(limit int) int { return (limit + markdown.ParseRatio - 1) / markdown.ParseRatio }
	m, err := markdown.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	frontmatter := "---\na: [x, x, x]\n---\n" + strings.Repeat("b", 179)
	aliases := "---\na: &a [x, x, x, x, x, x, x, x, x, x]\nb: [" + strings.Repeat("*a, ", 300) + "]\n---\n"
	aliased := m.Parse([]byte(aliases)).Facts()
	if s := share(aliased.Limit(len(aliases))); s <= markdown.MinTake {
		t.Fatalf("the aliases' share is %d, not more than MinTake", s)
	}
	for _, tt := range []struct {
		name  string
		n     int
		facts markdown.Facts
		kept  int
	}{
		// FactsBase and 30 times 9,100, 277,096 bytes of facts, keep 924.
		{"a content of 9,100 bytes", 9100, markdown.Facts{}, share(markdown.FactsBase + markdown.FactsRatio*9100)},
		// 200 bytes, four values and the paths "a.0", "a.1", "a.2": FactsBase,
		// 30 times 200, 400 times four and 9, 11,705 bytes of facts, keep 40.
		{"a frontmatter of four values", len(frontmatter), m.Parse([]byte(frontmatter)).Facts(),
			share(markdown.FactsBase + markdown.FactsRatio*200 + 400*4 + 9)},
		{"a frontmatter's aliases", len(aliases), aliased, markdown.MinTake},
		{"a content fifty times the budget", 50 * size, markdown.Facts{}, size},
		{"a content of math.MaxInt bytes", math.MaxInt, markdown.Facts{}, size},
	} {
		h, err := b.Take(ctx, tt.n)
		if err != nil {
			t.Fatal(err)
		}
		h.KeepFacts(tt.facts)
		h.KeepFacts(tt.facts) // keeping again gives nothing more back
		if tt.kept < size {
			if rest, err := b.Take(ctx, size-tt.kept); err != nil {
				t.Errorf("the budget beside the facts of %s: %v", tt.name, err)
			} else {
				rest.Release()
			}
		}
		if _, err := b.Take(ctx, size-tt.kept+1); !errors.Is(err, markdown.ErrBusy) {
			t.Errorf("a byte more beside %s = %v, want busy: the facts keep %d bytes", tt.name, err, tt.kept)
		}
		h.Release()
	}

	all, err := b.Take(ctx, size)
	if err != nil {
		t.Fatalf("the budget once every facts' share is released: %v", err)
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
