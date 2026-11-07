package archtest

import (
	"strings"
	"testing"
)

func TestRawSQLPreparedQueriesAreExact(t *testing.T) {
	for _, tc := range []struct{ path, name, source string }{
		{"internal/modules/linking/adapter/postgres/prepared_source_collect.go", "inspectPageLinksForPreparation", `package postgresadapter
import "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
func inspectPageLinksForPreparation(ctx context.Context, q postgres.Querier, arg gen.InspectPageLinksForPreparationParams, links *preparedLinkFacts) (preparedLinkRead, error) {
 rows, err := q.Query(ctx, gen.InspectPageLinksForPreparation, arg.SourceID, arg.NotebookID, arg.Extractor)
 if err != nil { return preparedLinkRead{}, err }
 return collectPreparedSource(rows, links)
}
`},
		{"internal/modules/linking/adapter/postgres/links_reached_except.go", "linksReachedExcept", `package postgresadapter
import "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
func linksReachedExcept(ctx context.Context, q postgres.Querier, arg gen.LinksReachedExceptParams) ([]app.Link, error) {
 rows, err := q.Query(ctx, gen.LinksReachedExcept, arg.ExcludedSources, arg.Keys, arg.NotebookID, arg.Targets, arg.Sources)
 if err != nil { return nil, err }
 var links linksReachedBuffer
 return links.read(rows)
}
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const call = ": calls Query, which runs SQL outside sqlc's queries"
			rawSQLForwarderCheck(t, tc.path, tc.source)
			for _, edit := range []struct{ name, old, replacement string }{
				{"carrier_value", "*preparedLinkFacts", "preparedLinkFacts"},
				{"carrier_other", "*preparedLinkFacts", "*otherPreparedLinkFacts"},
				{"carrier_qualified", "*preparedLinkFacts", "*other.preparedLinkFacts"},
				{"carrier_old", "*preparedLinkFacts", "*gen.InsertLinksParams"},
				{"foreign", "/modules/linking/", "/modules/page/"},
				{"function", "func " + tc.name, "func other"},
				{"query", "q.Query(ctx, gen.", "q.Query(ctx, other."},
				{"sql_copy", "q.Query(ctx, gen.", "q.Query(ctx, copyOf."},
				{"swapped", "arg.SourceID, arg.NotebookID", "arg.NotebookID, arg.SourceID"},
				{"excluded_changed", "arg.ExcludedSources,", "other.ExcludedSources,"},
				{"scope", " if err != nil", " arg = other; if err != nil"},
				{"escape", " if err != nil", " mutate(&arg); if err != nil"},
				{"shadow", " if err != nil", " { arg := other; _ = arg }; if err != nil"},
				{"lambda", " if err != nil", " func(arg any) {}(other); if err != nil"},
			} {
				if !strings.Contains(tc.source, edit.old) {
					continue
				}
				t.Run(edit.name, func(t *testing.T) {
					rawSQLForwarderCheck(t, tc.path, strings.Replace(tc.source, edit.old, edit.replacement, 1), call)
				})
			}
			t.Run("path", func(t *testing.T) {
				rawSQLForwarderCheck(t, "internal/modules/linking/adapter/postgres/other.go", tc.source, call)
			})
			t.Run("extra_call", func(t *testing.T) {
				rawSQLForwarderCheck(t, tc.path, strings.Replace(tc.source, " if err != nil", " other.Exec(ctx2, statement); if err != nil", 1), ": calls Exec, which runs SQL outside sqlc's queries")
			})
			t.Run("raw_sql", func(t *testing.T) {
				rawSQLForwarderCheck(t, tc.path, tc.source+`const bad = "SELECT id FROM users"`, ": holds SQL outside sqlc's queries")
			})
		})
	}
}
