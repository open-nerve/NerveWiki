package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetLinkLanding reads where a page made for a link's target would go, so
// that the link leads to it: GET /api/v0/pages/{page_id}/link-landing
// (M6/P6 design 2). The page is made by the page module's createPage.
type GetLinkLanding struct {
	Access Access
	Pages  PageTree
	Reads  Reads
	// MaxDepth is how deep pages nest, a root at depth 1.
	MaxDepth int
}

// Execute returns the landing of target, a link's target as written in the
// page id, after the decision, a writer's. Then the target: absent or
// longer than domain.MaxLandingTarget bytes is 422 on target; one that
// cuts into no segments, holds a NUL or is not valid UTF-8 has no landing,
// domain.TargetInvalid, without a read. The aliases are read only for a
// target that may lead by one. A page deleted since the decision is
// page.not_found, and an aliased page deleted since its alias was read is
// none. A read takes no lock and opens no transaction.
func (g GetLinkLanding) Execute(ctx context.Context, id uuid.UUID, target *string) (domain.Landing, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return domain.Landing{}, err
	}
	notebookID, err := g.Access.page(ctx, actor, id, domain.ActionReadLinkLanding)
	if err != nil {
		return domain.Landing{}, err
	}
	switch {
	case target == nil:
		return domain.Landing{}, shared.Invalid(shared.FieldError{Field: "target", Code: shared.FieldRequired, Message: "is required"})
	case len(*target) > domain.MaxLandingTarget:
		return domain.Landing{}, shared.Invalid(shared.FieldError{Field: "target", Code: shared.FieldTooLong, Message: fmt.Sprintf("must be at most %d bytes", domain.MaxLandingTarget)})
	}
	t, ok := domain.ParseTarget(*target)
	if !ok || !utf8.ValidString(*target) || strings.ContainsRune(*target, 0) {
		return domain.Landing{Reason: domain.TargetInvalid}, nil
	}
	last := t.LastKeys()
	var lead string
	keys := last
	if n := len(t.Keys); n > 1 {
		lead = t.Keys[n-2]
		keys = append(slices.Clip(keys), lead)
	}
	nodes, err := g.Pages.ByKeys(ctx, notebookID, keys)
	if err != nil {
		return domain.Landing{}, err
	}
	var candidates, parents []domain.Node
	for _, n := range nodes {
		key := n.Path[len(n.Path)-1].Key
		if slices.Contains(last, key) {
			candidates = append(candidates, n)
		}
		if lead != "" && key == lead {
			parents = append(parents, n)
		}
	}
	var aliases []Alias
	if t.ByAlias() {
		if aliases, err = g.Reads.Aliases(ctx, notebookID, last); err != nil {
			return domain.Landing{}, err
		}
	}
	ids := []uuid.UUID{id}
	for _, a := range aliases {
		ids = append(ids, a.PageID)
	}
	slices.SortFunc(ids, uuid.UUID.Compare)
	paths, err := g.Pages.Paths(ctx, notebookID, slices.Compact(ids))
	if err != nil {
		return domain.Landing{}, err
	}
	byID := make(map[uuid.UUID]domain.Node, len(paths))
	for _, n := range paths {
		byID[n.ID] = n
	}
	from, ok := byID[id]
	if !ok {
		return domain.Landing{}, domain.ErrPageNotFound
	}
	aliased := make(map[string][]domain.Node)
	for _, a := range aliases {
		if n, ok := byID[a.PageID]; ok {
			aliased[a.Key] = append(aliased[a.Key], n)
		}
	}
	return domain.Land(t, from.Path, candidates, parents, aliased, g.MaxDepth), nil
}
