package app

import (
	"context"
	"crypto/rand"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// CreateAPITokenDeps are CreateAPIToken's collaborators.
type CreateAPITokenDeps struct {
	Password CurrentPassword
	Tokens   APITokenCreator
	Clock    Clock
	Logger   *slog.Logger
}

// CreateAPIToken creates a personal access token for the caller:
// POST /api/v0/me/api-tokens. Any credential may, a token too; the current
// password is asked for, so that a refresh token that leaks cannot become a
// token that never expires (M1 design 4).
type CreateAPIToken struct {
	d CreateAPITokenDeps
}

// NewCreateAPIToken returns the use case.
func NewCreateAPIToken(d CreateAPITokenDeps) *CreateAPIToken {
	return &CreateAPIToken{d: d}
}

// CreateAPITokenInput is the token asked for and the current password.
type CreateAPITokenInput struct {
	Spec            domain.APITokenSpec
	CurrentPassword string
}

// CreatedAPIToken is a new token and, this once, the token itself.
type CreatedAPIToken struct {
	domain.APIToken
	Token string
}

// Execute checks the spec (422), then confirms the current password and,
// under the credential lock, inserts the token (M1/P3 design 3.2, 3.4): a
// token made with a credential that a concurrent change revoked is never
// inserted. The row and the answer hold the spec as the check returns it.
func (c *CreateAPIToken) Execute(ctx context.Context, in CreateAPITokenInput) (CreatedAPIToken, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return CreatedAPIToken{}, err
	}
	now := c.d.Clock.Now()
	spec, err := domain.CheckAPIToken(in.Spec, now)
	if err != nil {
		return CreatedAPIToken{}, err
	}
	account, err := c.d.Password.Account(ctx, actor)
	if err != nil {
		return CreatedAPIToken{}, err
	}
	var pat domain.PAT
	_, _ = rand.Read(pat[:]) // never fails since Go 1.24
	n := NewAPIToken{ID: uuid.NewV7(), UserID: actor.UserID, TokenHash: pat.Hash(), Name: spec.Name, ExpiresAt: spec.ExpiresAt, Now: now}
	err = c.d.Password.Confirm(ctx, actor, in.CurrentPassword, account.PasswordHash, now, nil, func(ctx context.Context) error {
		return c.d.Tokens.CreateAPIToken(ctx, n)
	})
	if err != nil {
		return CreatedAPIToken{}, err
	}
	c.d.Logger.InfoContext(ctx, "API token created", slog.String("user_id", actor.UserID.String()), slog.String("token_id", n.ID.String()))
	return CreatedAPIToken{
		APIToken: domain.APIToken{ID: n.ID, Name: n.Name, ExpiresAt: n.ExpiresAt, CreatedAt: now},
		Token:    pat.String(),
	}, nil
}
