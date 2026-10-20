package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// CheckEvery is how many links resolutions resolves between two looks at
// its context.
const CheckEvery = checkEvery

// TreeOf is treeOf, what a rewrite reads of the tree, for the tests outside
// the package.
func TreeOf(ctx context.Context, pages Pages, m Moved, targets, sources []uuid.UUID) (
	domain.Tree, map[uuid.UUID][]domain.Step, error,
) {
	return treeOf(ctx, pages, m, targets, sources)
}
