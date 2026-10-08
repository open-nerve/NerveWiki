package app

import "time"

// Signed is an attachment's signed address (M7 design 4.5; M7/P2 design
// 3.6): when it expires, and the signatures of its content, shown and
// downloaded (d=1).
type Signed struct {
	Expires  time.Time
	Inline   string
	Download string
}
