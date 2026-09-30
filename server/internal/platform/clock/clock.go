// Package clock is the real time source. Each module declares its own Clock
// port (Now() time.Time) in its app layer; System satisfies it by structure.
package clock

import "time"

// System reads the system clock. Its instants are in UTC and truncated to
// microseconds, the precision of PostgreSQL's timestamptz, so a time a use
// case writes reads back from the database unchanged.
type System struct{}

// Now returns the current instant.
func (System) Now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}
