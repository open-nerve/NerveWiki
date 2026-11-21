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

// rawSQLLinksRead is one of the three reads of a reach's links, each a
// forwarder of its own file with its own gen constant (M12/P4/S7 design
// 3.1): the rule's exception is the same for each, and holds each to its
// file and its constant.
type rawSQLLinksRead struct {
	path, function, statement string
}

func rawSQLLinksReads() []rawSQLLinksRead {
	return []rawSQLLinksRead{
		{rawSQLLinksReachedPath, "linksReached", "LinksReached"},
		{"internal/modules/linking/adapter/postgres/links_reached_wide.go", "linksReachedWide", "LinksReachedWide"},
		{"internal/modules/linking/adapter/postgres/links_reached_wide_generic.go", "linksReachedWideGeneric", "LinksReachedWideGeneric"},
	}
}

// source is rawSQLLinksReachedSource written for r: its function and its
// constant.
func (r rawSQLLinksRead) source() string {
	s := strings.Replace(rawSQLLinksReachedSource, "func linksReached(", "func "+r.function+"(", 1)
	return strings.Replace(s, "gen.LinksReached,", "gen."+r.statement+",", 1)
}

func TestRawSQLLinksReachedOwnQueryPasses(t *testing.T) {
	for _, read := range rawSQLLinksReads() {
		t.Run(read.function, func(t *testing.T) {
			rawSQLForwarderCheck(t, read.path, read.source())
			rawSQLForwarderCheck(t, read.path, strings.Replace(read.source(), "import \"", "import gen \"", 1))
		})
	}
}

func TestRawSQLLinksReachedExceptionIsExact(t *testing.T) {
	for _, read := range rawSQLLinksReads() {
		t.Run(read.function, func(t *testing.T) { rawSQLLinksReadExceptionIsExact(t, read) })
	}
}

func rawSQLLinksReadExceptionIsExact(t *testing.T, read rawSQLLinksRead) {
	const call = ": calls Query, which runs SQL outside sqlc's queries"
	rawSQLLinksReachedPath, rawSQLLinksReachedSource := read.path, read.source()
	own := "gen." + read.statement + ","
	// ours writes a case's text, written for LinksReached, for read: its
	// constant and its function's name.
	ours := func(text string) string {
		text = strings.ReplaceAll(text, "gen.LinksReached", "gen."+read.statement)
		text = strings.ReplaceAll(text, "func linksReached", "func "+read.function)
		return strings.ReplaceAll(text, "func (s *Store) linksReached", "func (s *Store) "+read.function)
	}
	edit := func(old, replacement string) string {
		t.Helper()
		old, replacement = ours(old), ours(replacement)
		if strings.Count(rawSQLLinksReachedSource, old) != 1 {
			t.Fatalf("fixture replacement %q is not unique", old)
		}
		return strings.Replace(rawSQLLinksReachedSource, old, replacement, 1)
	}
	paths := []string{
		"internal/modules/linking/adapter/postgres/store.go",
		"internal/modules/linking/adapter/postgres/links_reached_other.go",
		"internal/modules/page/adapter/postgres/links_reached.go",
		"internal/modules/linking/app/links_reached.go",
	}
	for _, other := range rawSQLLinksReads() {
		if other.path != read.path {
			paths = append(paths, other.path) // each read is its own file's alone
		}
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) { rawSQLForwarderCheck(t, path, rawSQLLinksReachedSource, call) })
	}
	for _, other := range rawSQLLinksReads() {
		if other.statement == read.statement {
			continue
		}
		// Another read's constant in this read's file: the rule matches
		// each file's own constant alone.
		t.Run("other_read_constant_"+other.statement, func(t *testing.T) {
			rawSQLForwarderCheck(t, rawSQLLinksReachedPath, strings.Replace(rawSQLLinksReachedSource, own, "gen."+other.statement+",", 1), call)
		})
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
		source := rawSQLLinksReachedSource + ours("func other() { q.Query(ctx, gen.LinksReached, notebookID, r.Keys, r.Targets, r.Sources) }\n")
		rawSQLForwarderCheck(t, rawSQLLinksReachedPath, source, call, call)
	})
	t.Run("duplicate_function", func(t *testing.T) {
		source := rawSQLLinksReachedSource + ours("func linksReached() {}\n")
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
