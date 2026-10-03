package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetEditLock reads a page's edit lock: GET
// /api/v0/pages/{page_id}/edit-lock (M5 design 4.2).
type GetEditLock struct {
	notebooks Notebooks
	nodes     Nodes
	sessions  AliveSessions
	names     Names
	auth      shared.Authorizer
	clock     Clock
}

// NewGetEditLock returns the use case.
func NewGetEditLock(notebooks Notebooks, nodes Nodes, sessions AliveSessions, names Names, auth shared.Authorizer,
	clock Clock,
) *GetEditLock {
	return &GetEditLock{notebooks: notebooks, nodes: nodes, sessions: sessions, names: names, auth: auth, clock: clock}
}

// LockView is a page's edit lock as its read gives it: who holds it and
// for how many seconds its lease lasts, rounded up; no holder for a page
// no session holds. A client's clock may be wrong, so the read gives a
// span, not the time the lease ends.
type LockView struct {
	Holder    *LockHolder
	ExpiresIn int
}

// LockHolder is the account whose alive session holds a page's lock.
type LockHolder struct {
	UserID      uuid.UUID
	DisplayName string
}

// Execute reads the lock of the page id, without a transaction or a lock,
// as a read of the page does: whoever may read the page may read it. A
// page that does not exist, is deleted, or whose notebook the caller has
// no role in is page.not_found.
func (g *GetEditLock) Execute(ctx context.Context, id uuid.UUID) (LockView, error) {
	if _, err := readable(ctx, g.notebooks, g.nodes, g.auth, id); err != nil {
		return LockView{}, err
	}
	now := g.clock.Now()
	alive, err := g.sessions.AliveSessionsOf(ctx, []uuid.UUID{id}, now)
	if err != nil || len(alive) == 0 {
		return LockView{}, err
	}
	s := alive[0]
	names, err := g.names.DisplayNames(ctx, []uuid.UUID{s.UserID})
	if err != nil {
		return LockView{}, err
	}
	left := s.ExpiresAt.Sub(now)
	return LockView{
		Holder:    &LockHolder{UserID: s.UserID, DisplayName: names[s.UserID]},
		ExpiresIn: int((left + time.Second - 1) / time.Second),
	}, nil
}
