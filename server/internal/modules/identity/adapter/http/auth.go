package httpadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// Register serves POST /api/v0/auth/register.
func (h handler) Register(ctx context.Context, req gen.RegisterRequestObject) (gen.RegisterResponseObject, error) {
	meta := httpserver.RequestMetaFrom(ctx)
	tokens, err := h.uc.Register.Execute(ctx, app.RegisterInput{
		Email:     req.Body.Email,
		Password:  req.Body.Password,
		UserAgent: meta.UserAgent,
		IP:        meta.ClientIP,
	})
	if err != nil {
		return nil, err
	}
	return gen.Register201JSONResponse(authTokens(tokens)), nil
}

func authTokens(t app.Tokens) gen.AuthTokens {
	return gen.AuthTokens{
		TokenType:             gen.AuthTokensTokenTypeBearer,
		AccessToken:           t.AccessToken,
		AccessTokenExpiresIn:  int(t.AccessExpiresIn.Seconds()),
		RefreshToken:          t.RefreshToken,
		RefreshTokenExpiresAt: t.RefreshExpiresAt,
	}
}
