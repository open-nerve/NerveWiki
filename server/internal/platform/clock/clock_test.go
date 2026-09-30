package clock_test

import (
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
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
