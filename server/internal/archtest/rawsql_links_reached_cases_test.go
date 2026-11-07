package archtest

import (
	"strings"
	"testing"
)

const rawSQLLinksReachedPath = "internal/modules/linking/adapter/postgres/links_reached.go"
const rawSQLLinksReachedSource = `package postgresadapter
import "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
func linksReached(ctx context.Context, q postgres.Querier, notebookID uuid.UUID, r domain.Reach) ([]app.Link, error) {
	rows, err := q.Query(ctx, gen.LinksReached, notebookID, r.Keys, r.Targets, r.Sources)
	if err != nil {
		return nil, err
	}
	var links linksReachedBuffer
	return links.read(rows)
}
`

func TestRawSQLLinksReachedOwnQueryPasses(t *testing.T) {
	rawSQLForwarderCheck(t, rawSQLLinksReachedPath, rawSQLLinksReachedSource)
	rawSQLForwarderCheck(t, rawSQLLinksReachedPath, strings.Replace(rawSQLLinksReachedSource, "import \"", "import gen \"", 1))
}

func TestRawSQLLinksReachedExceptionIsExact(t *testing.T) {
	const call = ": calls Query, which runs SQL outside sqlc's queries"
	edit := func(old, replacement string) string {
		t.Helper()
		if strings.Count(rawSQLLinksReachedSource, old) != 1 {
			t.Fatalf("fixture replacement %q is not unique", old)
		}
		return strings.Replace(rawSQLLinksReachedSource, old, replacement, 1)
	}
	for _, path := range []string{
		"internal/modules/linking/adapter/postgres/store.go",
		"internal/modules/linking/adapter/postgres/links_reached_other.go",
		"internal/modules/page/adapter/postgres/links_reached.go",
		"internal/modules/linking/app/links_reached.go",
	} {
		t.Run(path, func(t *testing.T) { rawSQLForwarderCheck(t, path, rawSQLLinksReachedSource, call) })
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{"wrong_package", "package postgresadapter", "package app"},
		{"foreign_gen", "/modules/linking/adapter/postgres/gen", "/modules/page/adapter/postgres/gen"},
		{"renamed_import", "import \"", "import other \""},
		{"dot_import", "import \"", "import . \""},
		{"missing_import", "import \"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen\"", ""},
		{"wrong_function", "func linksReached(", "func other("},
		{"receiver_method", "func linksReached(", "func (s *Store) linksReached("},
		{"type_parameters", "func linksReached(", "func linksReached[T any]("},
		{"wrong_context_type", "ctx context.Context", "ctx other.Context"},
		{"wrong_querier_type", "q postgres.Querier", "q *pgx.Conn"},
		{"wrong_notebook_type", "notebookID uuid.UUID", "notebookID string"},
		{"wrong_reach_type", "r domain.Reach", "r other.Reach"},
		{"renamed_parameter", "q postgres.Querier", "other postgres.Querier"},
		{"extra_parameter", "r domain.Reach)", "r domain.Reach, extra int)"},
		{"wrong_result", "([]app.Link, error)", "([]gen.LinksReachedRow, error)"},
		{"named_result", "([]app.Link, error)", "(out []app.Link, err error)"},
		{"array_result", "[]app.Link", "[1]app.Link"},
		{"wrong_sql_constant", "gen.LinksReached,", "gen.OtherQuery,"},
		{"local_sql_variable", "gen.LinksReached,", "statement,"},
		{"sql_concatenation", "gen.LinksReached,", "gen.LinksReached + suffix,"},
		{"sql_transform", "gen.LinksReached,", "rewrite(gen.LinksReached),"},
		{"wrong_querier", "q.Query(", "other.Query("},
		{"wrong_context_argument", "Query(ctx,", "Query(other,"},
		{"wrong_notebook_argument", "gen.LinksReached, notebookID,", "gen.LinksReached, other,"},
		{"changed_reach_argument", "r.Keys, r.Targets, r.Sources", "other.Keys, r.Targets, r.Sources"},
		{"swapped_reach_arguments", "r.Keys, r.Targets, r.Sources", "r.Keys, r.Sources, r.Targets"},
		{"missing_argument", "r.Keys, r.Targets, r.Sources", "r.Keys, r.Targets"},
		{"extra_argument", "r.Keys, r.Targets, r.Sources", "r.Keys, r.Targets, r.Sources, extra"},
		{"ellipsis", "r.Sources)", "r.Sources...)"},
		{"changed_assignment", "rows, err :=", "rows, err ="},
		{"query_inside_loop", "\trows, err :=", "\tfor {\n\trows, err :="},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := edit(tc.old, tc.replacement)
			if tc.name == "query_inside_loop" {
				source += "}\n"
			}
			rawSQLForwarderCheck(t, rawSQLLinksReachedPath, source, call)
		})
	}
	for _, name := range []string{"ctx", "q", "gen", "notebookID", "r"} {
		for _, body := range []string{
			name + " = other",
			"{ " + name + " := other; _ = " + name + " }",
			"{ var " + name + " = other; _ = " + name + " }",
			"mutate(&" + name + ")",
			"alias := " + name + "; _ = alias",
			"func(" + name + " any) {}(other)",
		} {
			t.Run(body, func(t *testing.T) {
				// Inject after the exact query, so the protected-use check,
				// rather than just the first-statement check, must reject it.
				source := edit("\tif err != nil {", "\t"+body+"\n\tif err != nil {")
				rawSQLForwarderCheck(t, rawSQLLinksReachedPath, source, call)
			})
		}
	}
	t.Run("duplicate_matching_call", func(t *testing.T) {
		source := edit("\tif err != nil {", "\tq.Query(ctx, gen.LinksReached, notebookID, r.Keys, r.Targets, r.Sources)\n\tif err != nil {")
		rawSQLForwarderCheck(t, rawSQLLinksReachedPath, source, call, call)
	})
	t.Run("matching_call_in_another_function", func(t *testing.T) {
		source := rawSQLLinksReachedSource + "func other() { q.Query(ctx, gen.LinksReached, notebookID, r.Keys, r.Targets, r.Sources) }\n"
		rawSQLForwarderCheck(t, rawSQLLinksReachedPath, source, call, call)
	})
	t.Run("duplicate_function", func(t *testing.T) {
		source := rawSQLLinksReachedSource + "func linksReached() {}\n"
		rawSQLForwarderCheck(t, rawSQLLinksReachedPath, source, call)
	})
	t.Run("duplicate_gen_binding", func(t *testing.T) {
		source := edit("func linksReached(", "import gen \"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen\"\nfunc linksReached(")
		rawSQLForwarderCheck(t, rawSQLLinksReachedPath, source, call)
	})
	t.Run("different_method", func(t *testing.T) {
		source := edit("q.Query(", "q.Exec(")
		rawSQLForwarderCheck(t, rawSQLLinksReachedPath, source, ": calls Exec, which runs SQL outside sqlc's queries")
	})
	t.Run("other_call_and_raw_sql_in_same_function", func(t *testing.T) {
		source := edit("\tif err != nil {", "\tother.Exec(otherContext, \"DELETE FROM users\")\n\tif err != nil {")
		rawSQLForwarderCheck(t, rawSQLLinksReachedPath, source,
			": calls Exec, which runs SQL outside sqlc's queries", ": holds SQL outside sqlc's queries")
	})
	t.Run("raw_sql_elsewhere_in_file", func(t *testing.T) {
		source := rawSQLLinksReachedSource + "const raw = \"SELECT id FROM users\"\n"
		rawSQLForwarderCheck(t, rawSQLLinksReachedPath, source, ": holds SQL outside sqlc's queries")
	})
}
