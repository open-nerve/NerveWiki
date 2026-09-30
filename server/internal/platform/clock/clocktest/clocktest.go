// Package clocktest provides a clock that tests set and move by hand. Only
// test code may import it (enforced by internal/archtest).
package clocktest

import (
	"sync"
	"time"
)

// Fixed stands still until the test moves it. Like clock.System, its instants
// are in UTC and truncated to microseconds. It is safe for concurrent use: an
// HTTP test may move it while the server's goroutines read it.
type Fixed struct {
	mu  sync.Mutex
	now time.Time
}

// At returns a clock stopped at t.
func At(t time.Time) *Fixed {
	return &Fixed{now: normalize(t)}
}

// Now returns the instant the clock stands at.
func (f *Fixed) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance moves the clock forward by d.
func (f *Fixed) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = normalize(f.now.Add(d))
}

func normalize(t time.Time) time.Time {
	return t.UTC().Truncate(time.Microsecond)
}
