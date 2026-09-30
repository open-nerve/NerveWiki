package app

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ListAPITokens lists the caller's tokens: GET /api/v0/me/api-tokens. The
// list is a small collection, not paged (v0.1 design 6.1).
type ListAPITokens struct {
	tokens APITokenLister
}

// NewListAPITokens returns the use case.
func NewListAPITokens(tokens APITokenLister) *ListAPITokens {
	return &ListAPITokens{tokens: tokens}
}

// Execute returns the caller's unrevoked tokens, expired ones too, newest
// first.
func (l *ListAPITokens) Execute(ctx context.Context) ([]domain.APIToken, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	return l.tokens.ListAPITokens(ctx, actor.UserID)
}
