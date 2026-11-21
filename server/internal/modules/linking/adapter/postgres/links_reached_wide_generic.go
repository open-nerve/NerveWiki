package postgresadapter

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// linksReachedWideGeneric reads LinksReachedWideGeneric, LinksReachedWide
// under plan_cache_mode = force_generic_plan, through the planned querier
// supplied by Store.Links, with linksReached's collector (M12/P4/S7 design
// 3.1).
func linksReachedWideGeneric(ctx context.Context, q postgres.Querier, notebookID uuid.UUID, r domain.Reach) ([]app.Link, error) {
	rows, err := q.Query(ctx, gen.LinksReachedWideGeneric, notebookID, r.Keys, r.Targets, r.Sources)
	if err != nil {
		return nil, err
	}
	var links linksReachedBuffer
	return links.read(rows)
}
