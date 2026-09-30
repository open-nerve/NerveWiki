package httpadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// GetMe serves GET /api/v0/me.
func (h handler) GetMe(ctx context.Context, _ gen.GetMeRequestObject) (gen.GetMeResponseObject, error) {
	u, err := h.uc.GetMe.Execute(ctx)
	if err != nil {
		return nil, err
	}
	return gen.GetMe200JSONResponse(user(u)), nil
}

func user(u domain.User) gen.User {
	steps := u.OnboardingSteps
	if steps == nil {
		steps = []string{} // the contract's array: never null
	}
	return gen.User{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, OnboardingSteps: steps}
}
