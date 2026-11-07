package archtest

import (
	"strings"
	"testing"
)

const rawSQLIngestOldFPath = "internal/modules/linking/adapter/postgres/ingest_old_f_collect.go"
const rawSQLIngestOldFPreamble = `package postgresadapter
import "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
`
const rawSQLIngestOldFFirst = `func ingestOldBodyReferencesFirst(ctx context.Context, q postgres.Querier, arg gen.IngestOldBodyReferencesFirstParams) (pgx.Rows, error) {
	return q.Query(ctx, gen.IngestOldBodyReferencesFirst, arg.NotebookID, arg.FirstSource, arg.LastSource, arg.Take, arg.WikiPageIds)
}
`
const rawSQLIngestOldFAfter = `func ingestOldBodyReferencesAfter(ctx context.Context, q postgres.Querier, arg gen.IngestOldBodyReferencesAfterParams) (pgx.Rows, error) {
	return q.Query(ctx, gen.IngestOldBodyReferencesAfter, arg.NotebookID, arg.FirstSource, arg.LastSource, arg.AfterSource, arg.AfterStart, arg.Take, arg.WikiPageIds)
}
`

func TestRawSQLIngestOldBodyOwnQueriesPass(t *testing.T) {
	for _, body := range []string{rawSQLIngestOldFFirst, rawSQLIngestOldFAfter, rawSQLIngestOldFFirst + rawSQLIngestOldFAfter} {
		source := rawSQLIngestOldFPreamble + body
		rawSQLForwarderCheck(t, rawSQLIngestOldFPath, source)
		rawSQLForwarderCheck(t, rawSQLIngestOldFPath, strings.Replace(source, "import \"", "import gen \"", 1))
	}
}

func TestRawSQLIngestOldBodyExceptionIsExact(t *testing.T) {
	const call = ": calls Query, which runs SQL outside sqlc's queries"
	for _, part := range []struct{ name, body string }{{"First", rawSQLIngestOldFFirst}, {"After", rawSQLIngestOldFAfter}} {
		t.Run(part.name, func(t *testing.T) {
			source := rawSQLIngestOldFPreamble + part.body
			edit := func(old, replacement string) string {
				t.Helper()
				if strings.Count(source, old) != 1 {
					t.Fatalf("non-unique fixture edit %q", old)
				}
				return strings.Replace(source, old, replacement, 1)
			}
			for _, path := range []string{"internal/modules/linking/adapter/postgres/other.go", "internal/modules/page/adapter/postgres/ingest_old_f_collect.go", "internal/modules/linking/app/ingest_old_f_collect.go"} {
				t.Run("path/"+path, func(t *testing.T) { rawSQLForwarderCheck(t, path, source, call) })
			}
			for _, tc := range []struct{ name, old, replacement string }{
				{"package", "package postgresadapter", "package app"},
				{"foreign_gen", "/modules/linking/adapter/postgres/gen", "/modules/page/adapter/postgres/gen"},
				{"renamed_gen", "import \"", "import other \""},
				{"dot_gen", "import \"", "import . \""},
				{"function", "func ingestOldBodyReferences" + part.name + "(", "func other("},
				{"receiver", "func ingestOldBodyReferences", "func (s *Store) ingestOldBodyReferences"},
				{"generic", part.name + "(ctx", part.name + "[T any](ctx"},
				{"context_type", "ctx context.Context", "ctx other.Context"},
				{"querier_type", "q postgres.Querier", "q *pgx.Conn"},
				{"parameter_type", "arg gen.IngestOldBodyReferences" + part.name + "Params", "arg other.Params"},
				{"parameter_name", "q postgres.Querier", "other postgres.Querier"},
				{"extra_parameter", "Params) (pgx.Rows", "Params, extra int) (pgx.Rows"},
				{"result_type", "(pgx.Rows, error)", "(pgx.Row, error)"},
				{"named_results", "(pgx.Rows, error)", "(rows pgx.Rows, err error)"},
				{"context_argument", "q.Query(ctx,", "q.Query(context.Background(),"},
				{"query_constant", "gen.IngestOldBodyReferences" + part.name + ",", "gen.LinksReached,"},
				{"notebook", "arg.NotebookID", "arg.FirstSource"},
				{"source_order", "arg.FirstSource, arg.LastSource", "arg.LastSource, arg.FirstSource"},
				{"fixed_take", "arg.Take", "int32(512)"},
				{"variadic", "arg.WikiPageIds)", "arg.WikiPageIds...)"},
				{"mutation", "\treturn q.Query", "\targ.Take++\n\treturn q.Query"},
				{"alias", "\treturn q.Query", "\tother := q\n\treturn other.Query"},
			} {
				t.Run(tc.name, func(t *testing.T) { rawSQLForwarderCheck(t, rawSQLIngestOldFPath, edit(tc.old, tc.replacement), call) })
			}
			if part.name == "After" {
				rawSQLForwarderCheck(t, rawSQLIngestOldFPath, edit("arg.AfterSource, arg.AfterStart", "arg.FirstSource, arg.AfterStart"), call)
			}
			rawSQLForwarderCheck(t, rawSQLIngestOldFPath, edit("q.Query(", "q.Exec("), ": calls Exec, which runs SQL outside sqlc's queries")
			rawSQLForwarderCheck(t, rawSQLIngestOldFPath, source+part.body, call, call)
			other := strings.Replace(part.body, "ingestOldBodyReferences"+part.name, "unapproved", 1)
			rawSQLForwarderCheck(t, rawSQLIngestOldFPath, source+other, call)
		})
	}
}
