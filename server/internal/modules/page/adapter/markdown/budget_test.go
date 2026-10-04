package markdownadapter_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The platform's budget, busy, is 503 server_busy with its Retry-After; a
// take that gets its bytes holds them in the platform's budget, and a
// request that ran out gets its context's error.
func TestTheAdapterAnswersABusyBudget(t *testing.T) {
	b := markdown.NewBudget(10, 20*time.Millisecond, slog.New(slog.DiscardHandler))
	adapter := markdownadapter.NewBudget(b)
	all, err := adapter.Take(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Take(context.Background(), 1)
	var se *shared.Error
	if !errors.As(err, &se) || se.Code != shared.CodeServerBusy || se.RetryDelay != time.Second {
		t.Errorf("Take while the budget is held = %v, want server_busy after a second", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := adapter.Take(ctx, 1); !errors.Is(err, context.Canceled) || errors.As(err, &se) {
		t.Errorf("Take with a cancelled context = %v, want its error", err)
	}
	all.Release()
	if all, err = adapter.Take(context.Background(), 10); err != nil {
		t.Errorf("Take after the release = %v", err)
	} else {
		all.Release()
	}
}

// A hold the adapter gives keeps the share of the facts it is handed, the
// platform's: with its frontmatter's values; facts of elsewhere keep the
// content's share alone.
func TestTheAdapterKeepsTheFactsShare(t *testing.T) {
	m, err := markdown.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	// 21 bytes and four values keep 8 bytes; 21 bytes alone keep 3.
	content := "---\na: [x, x, x]\n---\n"
	for _, tt := range []struct {
		name  string
		facts app.Facts
		kept  int
	}{{"the platform's facts", m.Parse([]byte(content)).Facts(), 8}, {"facts of elsewhere", content, 3}} {
		t.Run(tt.name, func(t *testing.T) {
			b := markdown.NewBudget(100, 20*time.Millisecond, slog.New(slog.DiscardHandler))
			hold, err := markdownadapter.NewBudget(b).Take(context.Background(), len(content))
			if err != nil {
				t.Fatal(err)
			}
			hold.KeepFacts(tt.facts)
			rest, err := b.Take(context.Background(), 100-tt.kept)
			if err != nil {
				t.Fatalf("the budget beside the facts' %d bytes: %v", tt.kept, err)
			}
			if _, err := b.Take(context.Background(), 1); !errors.Is(err, markdown.ErrBusy) {
				t.Errorf("a byte more = %v, want busy: the facts keep %d bytes", err, tt.kept)
			}
			rest.Release()
			hold.Release()
		})
	}
}

// A module wired without the server's budget fails at its wiring, not at
// its first write (M6/P2 review L2).
func TestTheAdapterRefusesNoBudget(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewBudget(nil) did not panic")
		}
	}()
	markdownadapter.NewBudget(nil)
}
