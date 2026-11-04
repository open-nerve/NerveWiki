package markdown

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// Inspect the actual original semaphore, without minTake rounding a free
// remainder. No second budget or fake counter can satisfy these bounds.
func reservationFree(t *testing.T, b *Budget, n int) {
	t.Helper()
	if b.bytes.TryAcquire(int64(n + 1)) {
		b.bytes.Release(int64(n + 1))
		t.Fatalf("RESERVATION_GLOBAL: more than %d free", n)
	}
	if !b.bytes.TryAcquire(int64(n)) {
		t.Fatalf("RESERVATION_GLOBAL: fewer than %d free", n)
	}
	b.bytes.Release(int64(n))
}

func TestReservationKeepsOneGlobalOwnerAcrossClose(t *testing.T) {
	for _, closeFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "keep_then_close", true: "close_then_keep"}[closeFirst], func(t *testing.T) {
			b := NewBudget(32768, time.Second, slog.New(slog.DiscardHandler))
			r, ok := b.TryReserve(16384)
			if !ok {
				t.Fatal("reserve")
			}
			h, ok := r.Take(9100)
			if !ok || h.Held() != 9100 {
				t.Fatal("actual take")
			}
			reservationFree(t, b, 16384)
			if closeFirst {
				r.Close()
				reservationFree(t, b, 23668)
			}
			h.KeepFacts(Facts{}) // Literal old formula: ceil((4096+30*9100)/300)=924.
			if h.Held() != 924 {
				t.Fatalf("RESERVATION_ACTUAL: kept %d, want 924", h.Held())
			}
			if !closeFirst {
				reservationFree(t, b, 16384)
				r.Close()
			}
			r.Close()
			reservationFree(t, b, 31844)
			h.KeepFacts(Facts{})
			h.Release()
			h.Release()
			r.Close()
			reservationFree(t, b, 32768)
		})
	}
}

func TestReservationGrowthDoesNotRetakeOldCredit(t *testing.T) {
	b := NewBudget(65536, time.Second, slog.New(slog.DiscardHandler))
	other, err := b.TakeNow(context.Background(), 16384)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := b.TryReserve(8192)
	if !ok {
		t.Fatal("reserve old")
	}
	old, ok := r.Take(8192)
	if !ok {
		t.Fatal("take old")
	}
	if !r.TryGrow(49152) {
		t.Fatal("RESERVATION_GROW: delta fits exactly, a full retake does not")
	}
	reservationFree(t, b, 0)
	if r.TryGrow(65536) {
		t.Fatal("growth ignored another request")
	}
	if !r.TryGrow(1) {
		t.Fatal("a smaller requested grant must keep the old one")
	}
	if h, ok := r.Take(40961); ok {
		h.Release()
		t.Fatal("borrowed beyond free parent credit")
	}
	old.Release()
	next, ok := r.Take(49152)
	if !ok || next.Held() != 49152 {
		t.Fatal("full grant not reusable after old release")
	}
	r.Close()
	r.Close()
	if _, ok := r.Take(0); ok || r.TryGrow(1) {
		t.Fatal("closed reservation reopened")
	}
	other.Release()
	next.Release()
	next.Release()
	reservationFree(t, b, 65536)
}

func TestReservationEmptyAndOptionalRefusal(t *testing.T) {
	b := NewBudget(8192, time.Hour, slog.New(slog.DiscardHandler))
	all, err := b.TakeNow(context.Background(), 8192)
	if err != nil {
		t.Fatal(err)
	}
	if r, ok := b.TryReserve(1); ok || r != nil {
		t.Fatal("optional refusal acquired occupied credit")
	}
	for _, n := range []int{-1, 8193} {
		if r, ok := b.TryReserve(n); ok || r != nil {
			t.Fatal("invalid reservation accepted")
		}
	}
	empty, ok := b.TryReserve(0)
	if !ok {
		t.Fatal("empty reservation consumed credit")
	}
	for _, n := range []int{0, -1} {
		h, ok := empty.Take(n)
		if !ok || h.Held() != 0 {
			t.Fatal("original empty/negative hold changed")
		}
		h.KeepFacts(Facts{})
		h.Release()
		h.Release()
	}
	if empty.TryGrow(4096) || empty.TryGrow(-1) {
		t.Fatal("failed grow changed ownership")
	}
	empty.Close()
	empty.Close()
	reservationFree(t, b, 0)
	all.Release()
	reservationFree(t, b, 8192)
}

func TestReservationCloseAndChildReleaseCanRace(t *testing.T) {
	for range 32 {
		b := NewBudget(32768, time.Second, slog.New(slog.DiscardHandler))
		r, ok := b.TryReserve(32768)
		if !ok {
			t.Fatal("reserve")
		}
		h, ok := r.Take(9100)
		if !ok {
			t.Fatal("take")
		}
		start := make(chan struct{})
		var workers sync.WaitGroup
		workers.Go(func() { <-start; h.KeepFacts(Facts{}); h.Release(); h.Release() })
		workers.Go(func() { <-start; r.Close(); r.Close() })
		close(start)
		workers.Wait()
		reservationFree(t, b, 32768)
	}
}
