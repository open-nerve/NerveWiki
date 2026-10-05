package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ListTags lists a notebook's tags: GET
// /api/v0/notebooks/{notebook_id}/tags (M6/P5 design 5).
type ListTags struct {
	Access Access
	Reads  Reads
}

// Execute returns the tags of the notebook id, by key, after the decision.
func (l ListTags) Execute(ctx context.Context, id uuid.UUID) ([]Tag, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := l.Access.notebook(ctx, actor, id, domain.ActionListTags); err != nil {
		return nil, err
	}
	return l.Reads.Tags(ctx, id)
}

// GetTag lists the pages of a notebook with a tag: GET
// /api/v0/notebooks/{notebook_id}/tags/{tag} (M6/P5 design 5).
type GetTag struct {
	Access Access
	Reads  Reads
	// TagKey is the key a tag's name is kept by; false for a name no
	// page's tag has.
	TagKey func(name string) (string, bool)
}

// Execute returns the pages of the notebook id with the tag name or one
// under it, name/…, by id, after the decision. A name no page's tag has,
// one not valid UTF-8 among them, has none, without a query.
func (g GetTag) Execute(ctx context.Context, id uuid.UUID, name string) ([]uuid.UUID, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := g.Access.notebook(ctx, actor, id, domain.ActionReadTag); err != nil {
		return nil, err
	}
	key, ok := g.TagKey(name)
	if !ok {
		return []uuid.UUID{}, nil
	}
	return g.Reads.TagPages(ctx, id, key)
}
