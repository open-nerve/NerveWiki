package httpadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
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

// UpdateMe serves PATCH /api/v0/me.
func (h handler) UpdateMe(ctx context.Context, req gen.UpdateMeRequestObject) (gen.UpdateMeResponseObject, error) {
	u, err := h.uc.UpdateMe.Execute(ctx, domain.UserPatch{DisplayName: req.Body.DisplayName})
	if err != nil {
		return nil, err
	}
	return gen.UpdateMe200JSONResponse(user(u)), nil
}

// RecordOnboardingStep serves POST /api/v0/me/onboarding-steps.
func (h handler) RecordOnboardingStep(ctx context.Context, req gen.RecordOnboardingStepRequestObject) (gen.RecordOnboardingStepResponseObject, error) {
	u, err := h.uc.RecordOnboardingStep.Execute(ctx, req.Body.Step)
	if err != nil {
		return nil, err
	}
	return gen.RecordOnboardingStep200JSONResponse(user(u)), nil
}

// ChangePassword serves POST /api/v0/me/change-password. password_user
// limits it per account: each attempt costs argon2 (M1/P3 design 3.4).
func (h handler) ChangePassword(ctx context.Context, req gen.ChangePasswordRequestObject) (gen.ChangePasswordResponseObject, error) {
	if err := h.limitPassword(ctx); err != nil {
		return nil, err
	}
	if err := h.uc.ChangePassword.Execute(ctx, app.ChangePasswordInput{Current: req.Body.CurrentPassword, New: req.Body.NewPassword}); err != nil {
		return nil, err
	}
	return gen.ChangePassword204Response{}, nil
}

func user(u domain.User) gen.User {
	steps := u.OnboardingSteps
	if steps == nil {
		steps = []string{} // the contract's array: never null
	}
	return gen.User{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, OnboardingSteps: steps}
}
