package app

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// CheckSlug tells whether a slug can name a new workspace:
// GET /api/v0/workspace-slugs/{slug}. Any signed-in account may ask; like
// the creation's 409, the answer tells whether a slug is taken, which
// choosing one needs.
type CheckSlug struct {
	slugs SlugChecker
}

// NewCheckSlug returns the use case.
func NewCheckSlug(slugs SlugChecker) *CheckSlug {
	return &CheckSlug{slugs: slugs}
}

// Execute returns why slug cannot name a new workspace:
// domain.SlugInvalid, domain.SlugReserved or domain.SlugTaken, or "" when
// it can.
func (c *CheckSlug) Execute(ctx context.Context, slug string) (string, error) {
	if _, err := shared.RequireActor(ctx); err != nil {
		return "", err
	}
	if problem := domain.SlugProblem(slug); problem != "" {
		return problem, nil
	}
	taken, err := c.slugs.SlugTaken(ctx, slug)
	if err != nil || !taken {
		return "", err
	}
	return domain.SlugTaken, nil
}
