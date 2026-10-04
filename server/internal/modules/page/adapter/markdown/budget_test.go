package markdownadapter_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/markdown"
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
