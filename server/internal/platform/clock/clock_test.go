package clock_test

import (
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock/clocktest"
)

type nower interface{ Now() time.Time }

// contract is what every Clock port implementation promises (Liskov
// substitution): UTC, no finer than timestamptz's microseconds, and
// never going backwards.
func contract(t *testing.T, c nower) {
	t.Helper()
	first := c.Now()
	second := c.Now()
	for _, now := range []time.Time{first, second} {
		if now.Location() != time.UTC {
			t.Errorf("Now() = %v, want UTC", now)
		}
		if now.Nanosecond()%1000 != 0 {
			t.Errorf("Now() = %v, want whole microseconds", now)
		}
		if now.IsZero() {
			t.Error("Now() is the zero time")
		}
	}
	if second.Before(first) {
		t.Errorf("Now() went backwards: %v then %v", first, second)
	}
}

func TestSystemFollowsTheClockContract(t *testing.T) {
	contract(t, clock.System{})
}

func TestFixedFollowsTheClockContract(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	contract(t, clocktest.At(time.Date(2026, 9, 25, 18, 0, 0, 123456789, loc)))
}

func TestFixedStandsStillUntilAdvanced(t *testing.T) {
	c := clocktest.At(time.Date(2026, 9, 25, 10, 0, 0, 999, time.UTC))
	start := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

	if got := c.Now(); !got.Equal(start) {
		t.Fatalf("Now() = %v, want %v", got, start)
	}
	c.Advance(15*time.Minute + time.Nanosecond)
	if got, want := c.Now(), start.Add(15*time.Minute); !got.Equal(want) {
		t.Errorf("Now() after Advance = %v, want %v", got, want)
	}
}
