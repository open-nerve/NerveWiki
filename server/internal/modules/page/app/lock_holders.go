package app

import (
	"context"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// LockHolders reads the edit locks of pages for another module: M6's
// rewrite refuses a rename or move whose pages are being edited, naming
// each page and its editor (M6/P4 design 4.1). Its read takes no lock: the
// unit that runs it holds its notebook's row FOR NO KEY UPDATE, which an
// opening, a take-over and an unlock wait behind (M6 design 4.6).
type LockHolders struct {
	sessions AliveSessions
	names    Names
}

// NewLockHolders returns the read.
func NewLockHolders(sessions AliveSessions, names Names) *LockHolders {
	return &LockHolders{sessions: sessions, names: names}
}

// Of is the edit locks of ids alive at now, one a page, its session the
// first opened, by page id; the caller's own among them.
func (h *LockHolders) Of(ctx context.Context, ids []uuid.UUID, now time.Time) ([]shared.LockHolder, error) {
	alive, err := h.sessions.AliveSessionsOf(ctx, ids, now)
	if err != nil || len(alive) == 0 {
		return nil, err
	}
	var locks []shared.LockHolder
	var users []uuid.UUID
	for _, s := range alive {
		if !slices.ContainsFunc(locks, func(l shared.LockHolder) bool { return l.PageID == s.NodeID }) {
			locks = append(locks, shared.LockHolder{PageID: s.NodeID, UserID: s.UserID})
			users = append(users, s.UserID)
		}
	}
	names, err := h.names.DisplayNames(ctx, users)
	if err != nil {
		return nil, err
	}
	for i := range locks {
		locks[i].DisplayName = names[locks[i].UserID]
	}
	slices.SortFunc(locks, func(a, b shared.LockHolder) int { return a.PageID.Compare(b.PageID) })
	return locks, nil
}
