package domain

import "time"

// An edit session is a lease (v0.1 design 3.9; M4 design 4): it lives
// EditSessionLease after it opens or after its last heartbeat, and the
// editor beats every EditSessionHeartbeat. The lease is six beats (M5
// design 4.6): a browser wakes a tab hidden for over five minutes once a
// minute, so its beats may come a minute apart, and the lease is twice
// that. The web editor beats as often, by its own editSessionHeartbeat
// (web/apps/web/src/stores/page-editing.ts; the lease it does not need);
// each side's tests pin its own.
const (
	EditSessionLease     = 120 * time.Second
	EditSessionHeartbeat = 20 * time.Second
)

// EndReason is why an edit session ended, as its end's subscribers learn
// it. An expiry is no end: whether a session is alive is read from its
// lease, and the cleanup of the expired ones tells no one.
type EndReason string

// The reasons. A session taken over or unlocked keeps its row as a
// tombstone with its reason (M5 design 4.3), so that its tab learns why; the
// other ends delete the row.
const (
	// EndedByOwner: its owner ended it.
	EndedByOwner EndReason = "ended"
	// EndedWithPage: its page was deleted, alone, with a subtree, or with
	// its notebook.
	EndedWithPage EndReason = "page_deleted"
	// EndedTakenOver: its owner opened the page's session elsewhere and
	// took it over.
	EndedTakenOver EndReason = "taken_over"
	// EndedUnlocked: an admin of its notebook released the page's lock.
	EndedUnlocked EndReason = "unlocked"
)
