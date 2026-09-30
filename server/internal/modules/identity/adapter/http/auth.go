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
	if err := h.limitRegister(ctx); err != nil {
		return nil, err
	}
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

// Login serves POST /api/v0/auth/login.
func (h handler) Login(ctx context.Context, req gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	meta := httpserver.RequestMetaFrom(ctx)
	if err := h.limitLogin(ctx, req.Body.Email); err != nil {
		return nil, err
	}
	tokens, err := h.uc.Login.Execute(ctx, app.LoginInput{
		Email:     req.Body.Email,
		Password:  req.Body.Password,
		UserAgent: meta.UserAgent,
		IP:        meta.ClientIP,
	})
	if err != nil {
		return nil, err
	}
	return gen.Login200JSONResponse(authTokens(tokens)), nil
}

// RefreshTokens serves POST /api/v0/auth/refresh, within the refresh
// deadline (RequestTimeouts). Only the platform's anonymous bucket limits
// it.
func (h handler) RefreshTokens(ctx context.Context, req gen.RefreshTokensRequestObject) (gen.RefreshTokensResponseObject, error) {
	tokens, err := h.uc.Refresh.Execute(ctx, req.Body.RefreshToken, httpserver.RequestMetaFrom(ctx).ClientIP)
	if err != nil {
		return nil, err
	}
	return gen.RefreshTokens200JSONResponse(authTokens(tokens)), nil
}

// Logout serves POST /api/v0/auth/logout, within the refresh deadline
// (RequestTimeouts). Only the platform's anonymous bucket limits it.
func (h handler) Logout(ctx context.Context, req gen.LogoutRequestObject) (gen.LogoutResponseObject, error) {
	if err := h.uc.Logout.Execute(ctx, req.Body.RefreshToken); err != nil {
		return nil, err
	}
	return gen.Logout204Response{}, nil
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
