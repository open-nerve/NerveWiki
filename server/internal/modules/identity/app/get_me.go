package app

import (
	"context"
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetMe reads the caller's account: GET /api/v0/me.
type GetMe struct {
	users UserReader
}

// NewGetMe returns the use case.
func NewGetMe(users UserReader) *GetMe {
	return &GetMe{users: users}
}

// Execute returns the account of the request's actor.
func (g *GetMe) Execute(ctx context.Context) (domain.User, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return domain.User{}, err
	}
	u, err := g.users.GetUser(ctx, actor.UserID)
	if errors.Is(err, ErrNotFound) {
		// Authentication found the account a moment ago; it is gone now.
		return domain.User{}, shared.Unauthenticated()
	}
	return u, err
}
