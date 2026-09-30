package httpadapter

import (
	"context"
	"time"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// ListAPITokens serves GET /api/v0/me/api-tokens.
func (h handler) ListAPITokens(ctx context.Context, _ gen.ListAPITokensRequestObject) (gen.ListAPITokensResponseObject, error) {
	tokens, err := h.uc.ListAPITokens.Execute(ctx)
	if err != nil {
		return nil, err
	}
	out := gen.ListAPITokens200JSONResponse{Data: make([]gen.APIToken, len(tokens))}
	for i, t := range tokens {
		out.Data[i] = gen.APIToken{
			ID: t.ID, Name: t.Name, ExpiresAt: nullableTime(t.ExpiresAt), LastUsedAt: nullableTime(t.LastUsedAt), CreatedAt: t.CreatedAt,
		}
	}
	return out, nil
}

// CreateAPIToken serves POST /api/v0/me/api-tokens. password_user limits
// it per account: each attempt costs argon2 (M1/P3 design 3.4).
func (h handler) CreateAPIToken(ctx context.Context, req gen.CreateAPITokenRequestObject) (gen.CreateAPITokenResponseObject, error) {
	if err := h.limitPassword(ctx); err != nil {
		return nil, err
	}
	t, err := h.uc.CreateAPIToken.Execute(ctx, app.CreateAPITokenInput{
		Spec:            domain.APITokenSpec{Name: req.Body.Name, ExpiresAt: req.Body.ExpiresAt},
		CurrentPassword: req.Body.CurrentPassword,
	})
	if err != nil {
		return nil, err
	}
	return gen.CreateAPIToken201JSONResponse{
		ID: t.ID, Name: t.Name, ExpiresAt: nullableTime(t.ExpiresAt), LastUsedAt: nullableTime(t.LastUsedAt), CreatedAt: t.CreatedAt,
		Token: t.Token,
	}, nil
}

// RevokeAPIToken serves DELETE /api/v0/api-tokens/{token_id}.
func (h handler) RevokeAPIToken(ctx context.Context, req gen.RevokeAPITokenRequestObject) (gen.RevokeAPITokenResponseObject, error) {
	if err := h.uc.RevokeAPIToken.Execute(ctx, req.TokenID); err != nil {
		return nil, err
	}
	return gen.RevokeAPIToken204Response{}, nil
}

// nullableTime is t as a required field that may be null: the zero
// Nullable is "unspecified" and would marshal as the zero time, so nil is
// set to null explicitly.
func nullableTime(t *time.Time) nullable.Nullable[time.Time] {
	if t == nil {
		return nullable.NewNullNullable[time.Time]()
	}
	return nullable.NewNullableWithValue(*t)
}
