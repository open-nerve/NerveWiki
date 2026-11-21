package postgresadapter

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// linksReachedWide reads LinksReachedWide through the planned querier
// supplied by Store.Links, with linksReached's collector (M12/P4/S7 design
// 3.1).
func linksReachedWide(ctx context.Context, q postgres.Querier, notebookID uuid.UUID, r domain.Reach) ([]app.Link, error) {
	rows, err := q.Query(ctx, gen.LinksReachedWide, notebookID, r.Keys, r.Targets, r.Sources)
	if err != nil {
		return nil, err
	}
	var links linksReachedBuffer
	return links.read(rows)
}
