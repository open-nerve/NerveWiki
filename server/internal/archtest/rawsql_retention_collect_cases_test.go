package archtest

import (
	"strings"
	"testing"
)

const rawSQLRetentionPath = "internal/modules/linking/adapter/postgres/retention_collect.go"
const rawSQLRetentionSource = `package postgresadapter
import "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
func inspectPageLinksForRetention(ctx context.Context, q postgres.Querier, arg gen.InspectPageLinksForRetentionParams, links *gen.InsertLinksParams) (retentionPlan, error) {
	rows, err := q.Query(ctx, gen.InspectPageLinksForRetention, arg.SourceID, arg.NotebookID, arg.Extractor)
	if err != nil {
		return retentionPlan{}, err
	}
	return collectPageLinksForRetention(rows, links)
}
`

func TestRawSQLRetentionOwnQueryPasses(t *testing.T) {
	rawSQLForwarderCheck(t, rawSQLRetentionPath, rawSQLRetentionSource)
	rawSQLForwarderCheck(t, rawSQLRetentionPath, strings.Replace(rawSQLRetentionSource, "import \"", "import gen \"", 1))
}

func TestRawSQLRetentionExceptionIsExact(t *testing.T) {
	const call = ": calls Query, which runs SQL outside sqlc's queries"
	edit := func(old, replacement string) string {
		t.Helper()
		if strings.Count(rawSQLRetentionSource, old) != 1 {
			t.Fatalf("fixture replacement %q is not unique", old)
		}
		return strings.Replace(rawSQLRetentionSource, old, replacement, 1)
	}
	for _, path := range []string{
		"internal/modules/linking/adapter/postgres/store.go",
		"internal/modules/linking/adapter/postgres/retention_other.go",
		"internal/modules/linking/adapter/postgres/links_reached.go",
		"internal/modules/page/adapter/postgres/retention_collect.go",
		"internal/modules/linking/app/retention_collect.go",
	} {
		t.Run(path, func(t *testing.T) { rawSQLForwarderCheck(t, path, rawSQLRetentionSource, call) })
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{"wrong_package", "package postgresadapter", "package app"},
		{"foreign_gen", "/modules/linking/adapter/postgres/gen", "/modules/page/adapter/postgres/gen"},
		{"renamed_import", "import \"", "import other \""},
		{"dot_import", "import \"", "import . \""},
		{"missing_import", "import \"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen\"", ""},
		{"wrong_function", "func inspectPageLinksForRetention(", "func other("},
		{"receiver_method", "func inspectPageLinksForRetention(", "func (s *Store) inspectPageLinksForRetention("},
		{"type_parameters", "func inspectPageLinksForRetention(", "func inspectPageLinksForRetention[T any]("},
		{"wrong_context_type", "ctx context.Context", "ctx other.Context"},
		{"wrong_querier_type", "q postgres.Querier", "q *pgx.Conn"},
		{"wrong_arg_type", "arg gen.InspectPageLinksForRetentionParams", "arg other.InspectPageLinksForRetentionParams"},
		{"wrong_links_type", "links *gen.InsertLinksParams", "links *other.InsertLinksParams"},
		{"value_links_type", "links *gen.InsertLinksParams", "links gen.InsertLinksParams"},
		{"renamed_parameter", "q postgres.Querier", "other postgres.Querier"},
		{"extra_parameter", "links *gen.InsertLinksParams)", "links *gen.InsertLinksParams, extra int)"},
		{"wrong_result", "(retentionPlan, error)", "(other.Plan, error)"},
		{"named_result", "(retentionPlan, error)", "(out retentionPlan, err error)"},
		{"slice_result", "(retentionPlan, error)", "([]retentionPlan, error)"},
		{"wrong_sql_constant", "gen.InspectPageLinksForRetention,", "gen.LinksReached,"},
		{"local_sql_variable", "gen.InspectPageLinksForRetention,", "statement,"},
		{"sql_concatenation", "gen.InspectPageLinksForRetention,", "gen.InspectPageLinksForRetention + suffix,"},
		{"sql_transform", "gen.InspectPageLinksForRetention,", "rewrite(gen.InspectPageLinksForRetention),"},
		{"wrong_querier", "q.Query(", "other.Query("},
		{"wrong_context_argument", "Query(ctx,", "Query(other,"},
		{"changed_source_argument", "arg.SourceID,", "other.SourceID,"},
		{"swapped_arguments", "arg.SourceID, arg.NotebookID", "arg.NotebookID, arg.SourceID"},
		{"wrong_extractor", "arg.Extractor)", "domain.Extractor)"},
		{"missing_argument", "arg.NotebookID, arg.Extractor", "arg.NotebookID"},
		{"extra_argument", "arg.Extractor)", "arg.Extractor, extra)"},
		{"ellipsis", "arg.Extractor)", "arg.Extractor...)"},
		{"changed_assignment", "rows, err :=", "rows, err ="},
		{"query_inside_loop", "\trows, err :=", "\tfor {\n\trows, err :="},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := edit(tc.old, tc.replacement)
			if tc.name == "query_inside_loop" {
				source += "}\n"
			}
			rawSQLForwarderCheck(t, rawSQLRetentionPath, source, call)
		})
	}
	for _, name := range []string{"ctx", "q", "gen", "arg"} {
		for _, body := range []string{
			name + " = other",
			"{ " + name + " := other; _ = " + name + " }",
			"{ var " + name + " = other; _ = " + name + " }",
			"mutate(&" + name + ")",
			"alias := " + name + "; _ = alias",
			"func(" + name + " any) {}(other)",
		} {
			t.Run(body, func(t *testing.T) {
				source := edit("\tif err != nil {", "\t"+body+"\n\tif err != nil {")
				rawSQLForwarderCheck(t, rawSQLRetentionPath, source, call)
			})
		}
	}
	for _, body := range []string{"arg.SourceID = other", "mutate(&arg.NotebookID)", "arg.Extractor++"} {
		t.Run(body, func(t *testing.T) {
			rawSQLForwarderCheck(t, rawSQLRetentionPath, edit("\tif err != nil {", "\t"+body+"\n\tif err != nil {"), call)
		})
	}
	t.Run("duplicate_matching_call", func(t *testing.T) {
		source := edit("\tif err != nil {", "\tq.Query(ctx, gen.InspectPageLinksForRetention, arg.SourceID, arg.NotebookID, arg.Extractor)\n\tif err != nil {")
		rawSQLForwarderCheck(t, rawSQLRetentionPath, source, call, call)
	})
	t.Run("matching_call_in_another_function", func(t *testing.T) {
		source := rawSQLRetentionSource + "func other() { q.Query(ctx, gen.InspectPageLinksForRetention, arg.SourceID, arg.NotebookID, arg.Extractor) }\n"
		rawSQLForwarderCheck(t, rawSQLRetentionPath, source, call, call)
	})
	t.Run("duplicate_function", func(t *testing.T) {
		rawSQLForwarderCheck(t, rawSQLRetentionPath, rawSQLRetentionSource+"func inspectPageLinksForRetention() {}\n", call)
	})
	t.Run("duplicate_gen_binding", func(t *testing.T) {
		source := edit("func inspectPageLinksForRetention(", "import gen \"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen\"\nfunc inspectPageLinksForRetention(")
		rawSQLForwarderCheck(t, rawSQLRetentionPath, source, call)
	})
	t.Run("different_method", func(t *testing.T) {
		rawSQLForwarderCheck(t, rawSQLRetentionPath, edit("q.Query(", "q.Exec("), ": calls Exec, which runs SQL outside sqlc's queries")
	})
	t.Run("other_call_and_raw_sql_in_same_function", func(t *testing.T) {
		source := edit("\tif err != nil {", "\tother.Exec(otherContext, \"DELETE FROM users\")\n\tif err != nil {")
		rawSQLForwarderCheck(t, rawSQLRetentionPath, source,
			": calls Exec, which runs SQL outside sqlc's queries", ": holds SQL outside sqlc's queries")
	})
	t.Run("raw_sql_elsewhere_in_file", func(t *testing.T) {
		rawSQLForwarderCheck(t, rawSQLRetentionPath, rawSQLRetentionSource+"const raw = \"SELECT id FROM users\"\n", ": holds SQL outside sqlc's queries")
	})
}
