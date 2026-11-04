package markdown

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"testing"
	"time"
)

func TestReservationCannotPassAnActualQueuedWaiter(t *testing.T) {
	b := NewBudget(16384, time.Minute, slog.New(slog.DiscardHandler))
	other, err := b.TakeNow(t.Context(), 8192)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Release()
	r, ok := b.TryReserve(4096)
	if !ok {
		t.Fatal("initial reservation")
	}
	defer r.Close()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	joined := false
	defer func() {
		cancel()
		if !joined {
			<-done
		}
	}()
	go func() {
		h, e := b.Take(ctx, 8192)
		if h != nil {
			h.Release()
		}
		done <- e
	}()
	// Weighted.TryAcquire(0) succeeds iff its waiter queue is empty. With
	// 4096 free and an 8192 waiter, false observes the real queued waiter,
	// not Budget.waiting's earlier increment. No timer guesses its arrival.
	deadline := time.Now().Add(2 * time.Second)
	for b.bytes.TryAcquire(0) {
		b.bytes.Release(0)
		if time.Now().After(deadline) {
			t.Fatal("waiter did not enter original Weighted queue")
		}
		runtime.Gosched()
	}
	if got, ok := b.TryReserve(4096); ok || got != nil {
		if got != nil {
			got.Close()
		}
		t.Fatal("RESERVATION_WAITER: optional reserve passed the queued take")
	}
	if r.TryGrow(8192) {
		t.Fatal("RESERVATION_WAITER: positive growth passed the queued take")
	}
	empty, ok := b.TryReserve(0)
	if !ok || !r.TryGrow(4096) {
		t.Fatal("zero/new-no-credit admission changed original zero semantics")
	}
	empty.Close()
	cancel()
	if e := <-done; !errors.Is(e, context.Canceled) {
		joined = true
		t.Fatalf("original queued cancellation: %v", e)
	}
	joined = true
	if !r.TryGrow(8192) {
		t.Fatal("cancelled waiter still blocked available growth")
	}
	r.Close()
	other.Release()
	reservationFree(t, b, 16384)
}
