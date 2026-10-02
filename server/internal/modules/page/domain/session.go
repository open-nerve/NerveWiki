package domain

import "time"

// An edit session is a lease (v0.1 design 3.9; M4 design 4): it lives
// EditSessionLease after it opens or after its last heartbeat, and the
// editor beats every EditSessionHeartbeat, so that two beats may be lost
// before it ends. The web editor holds the same two numbers (M4/P6); each
// side's tests pin its own.
const (
	EditSessionLease     = 60 * time.Second
	EditSessionHeartbeat = 20 * time.Second
)

// EndReason is why an edit session ended, as its end's subscribers learn
// it. An expiry is no end: whether a session is alive is read from its
// lease, and the cleanup of the expired ones tells no one.
type EndReason string

// The reasons of M4; M5 adds the notebook admin's forced unlock.
const (
	// EndedByOwner: its owner ended it.
	EndedByOwner EndReason = "ended"
	// EndedWithPage: its page was deleted, alone, with a subtree, or with
	// its notebook.
	EndedWithPage EndReason = "page_deleted"
)
