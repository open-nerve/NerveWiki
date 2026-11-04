package postgresadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

func linksReachedExcept(ctx context.Context, q postgres.Querier, arg gen.LinksReachedExceptParams) ([]app.Link, error) {
	rows, err := q.Query(ctx, gen.LinksReachedExcept, arg.ExcludedSources, arg.Keys, arg.NotebookID, arg.Targets, arg.Sources)
	if err != nil {
		return nil, err
	}
	var links linksReachedBuffer
	return links.read(rows)
}
