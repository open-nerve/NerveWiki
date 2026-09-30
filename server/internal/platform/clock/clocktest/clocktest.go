// Package clocktest provides a clock that tests set and move by hand. Only
// test code may import it (enforced by internal/archtest).
package clocktest

import "time"

// Fixed stands still until the test moves it. Like clock.System, its instants
// are in UTC and truncated to microseconds. It is not safe for concurrent use.
type Fixed struct {
	now time.Time
}

// At returns a clock stopped at t.
func At(t time.Time) *Fixed {
	return &Fixed{now: normalize(t)}
}

// Now returns the instant the clock stands at.
func (f *Fixed) Now() time.Time {
	return f.now
}

// Advance moves the clock forward by d.
func (f *Fixed) Advance(d time.Duration) {
	f.now = normalize(f.now.Add(d))
}

func normalize(t time.Time) time.Time {
	return t.UTC().Truncate(time.Microsecond)
}
