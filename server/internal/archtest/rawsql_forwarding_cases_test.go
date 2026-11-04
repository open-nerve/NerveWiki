package archtest

import (
	"strings"
	"testing"
)

type rawSQLForwarderFixture struct {
	path, receiver, method, result string
}

func rawSQLForwarders() []rawSQLForwarderFixture {
	return []rawSQLForwarderFixture{
		{"internal/modules/linking/adapter/postgres/lint.go", "lintBodyQuerier", "Query", "pgx.Rows"},
		{"internal/modules/linking/adapter/postgres/retention_array_encoding.go", "retentionArrayQuerier", "Exec", "pgconn.CommandTag"},
	}
}

func (f rawSQLForwarderFixture) source(body string) string {
	return "package postgresadapter\n\nfunc (q " + f.receiver + ") " + f.method +
		"(ctx context.Context, sql string, args ...any) (" + f.result + ", error) {\n" +
		strings.ReplaceAll(body, "METHOD", f.method) + "\n}\n"
}

// Keep all base-layout files in every check: an exception for a forwarder
// must not change the independent rules for the rest of the module tree.
func rawSQLForwarderCheck(t *testing.T, path, source string, want ...string) {
	t.Helper()
	files := rawSQLBase()
	files[path] = source
	got := rawSQLViolations(files)
	if len(got) != len(want) {
		t.Fatalf("violations=%q want suffixes=%q", got, want)
	}
	for i := range want {
		if !strings.HasPrefix(got[i], path+":") || !strings.HasSuffix(got[i], want[i]) {
			t.Fatalf("violation[%d]=%q want path=%q suffix=%q", i, got[i], path, want[i])
		}
	}
}

func TestRawSQLOnlyKnownPureForwardersPass(t *testing.T) {
	for _, f := range rawSQLForwarders() {
		t.Run(f.method, func(t *testing.T) {
			rawSQLForwarderCheck(t, f.path, f.source("\treturn q.Querier.METHOD(ctx, sql, args...)"))
		})
	}
	lint, retention := rawSQLForwarders()[0], rawSQLForwarders()[1]
	rawSQLForwarderCheck(t, lint.path, lint.source(`	var conn *pgx.Conn
	switch db := q.Querier.(type) {
	case pgx.Tx:
		conn = db.Conn()
	case *pgxpool.Conn:
		conn = db.Conn()
	}
	mode := pgx.QueryExecModeDescribeExec
	if conn != nil && conn.Config().DescriptionCacheCapacity > 0 {
		mode = pgx.QueryExecModeCacheDescribe
	}
	return q.Querier.Query(ctx, sql, append([]any{mode}, args...)...)`))
	rawSQLForwarderCheck(t, retention.path, retention.source(`	tx, ok := postgres.TxFrom(ctx)
	actual, actualOK := q.Querier.(pgx.Tx)
	if !ok || !actualOK || actual.Conn() != tx.Conn() || ctx.Err() != nil {
		return q.Querier.Exec(ctx, sql, args...)
	}
	reserve, ok := retentionArrayBudget(sql, args)
	if !ok {
		return q.Querier.Exec(ctx, sql, args...)
	}
	restore := retentionArrayEncoding(tx.Conn().TypeMap(), reserve)
	if restore != nil {
		defer restore()
	}
	return q.Querier.Exec(ctx, sql, args...)`))
}

func TestRawSQLForwarderChangesAreReported(t *testing.T) {
	for _, f := range rawSQLForwarders() {
		t.Run(f.method, func(t *testing.T) {
			call := ": calls " + f.method + ", which runs SQL outside sqlc's queries"
			for _, tc := range []struct {
				name, body string
				calls      int
			}{
				{"concatenation", `return q.Querier.METHOD(ctx, sql + suffix, args...)`, 1},
				{"transformed_sql", `return q.Querier.METHOD(ctx, strings.TrimSpace(sql), args...)`, 1},
				{"alias_variable", `statement := sql; return q.Querier.METHOD(ctx, statement, args...)`, 1},
				{"replacement_variable", `return q.Querier.METHOD(ctx, replacement, args...)`, 1},
				{"reassignment", `sql = replacement; return q.Querier.METHOD(ctx, sql, args...)`, 1},
				{"compound_assignment", `sql += suffix; return q.Querier.METHOD(ctx, sql, args...)`, 1},
				{"short_declaration_shadow", `{ sql := replacement; return q.Querier.METHOD(ctx, sql, args...) }`, 1},
				{"var_shadow", `{ var sql = replacement; return q.Querier.METHOD(ctx, sql, args...) }`, 1},
				{"address_escape", `rewrite(&sql); return q.Querier.METHOD(ctx, sql, args...)`, 1},
				{"string_data_escape", `rewrite(unsafe.StringData(sql)); return q.Querier.METHOD(ctx, sql, args...)`, 1},
				{"receiver_shadow", `{ q := other; return q.Querier.METHOD(ctx, sql, args...) }`, 1},
				{"context_shadow", `{ ctx := other; return q.Querier.METHOD(ctx, sql, args...) }`, 1},
				{"arguments_shadow", `{ args := other; return q.Querier.METHOD(ctx, sql, args...) }`, 1},
				{"receiver_field_write", `q.Querier = other; return q.Querier.METHOD(ctx, sql, args...)`, 1},
				{"context_address_escape", `replace(&ctx); return q.Querier.METHOD(ctx, sql, args...)`, 1},
				{"range_shadow", `for _, sql := range statements { return q.Querier.METHOD(ctx, sql, args...) }`, 1},
				{"range_context_shadow", `for ctx := range contexts { return q.Querier.METHOD(ctx, sql, args...) }`, 1},
				{"lambda_shadow", `func(sql string) { q.Querier.METHOD(ctx, sql, args...) }(replacement); return q.Querier.METHOD(ctx, sql, args...)`, 2},
				{"lambda_reassignment", `func() { sql = replacement }(); return q.Querier.METHOD(ctx, sql, args...)`, 1},
				{"lambda_return", "return func() (" + f.result + ", error) { return q.Querier.METHOD(ctx, sql, args...) }()", 1},
				{"wrong_context_argument", `return q.Querier.METHOD(other, sql, args...)`, 1},
				{"wrong_arguments", `return q.Querier.METHOD(ctx, sql, other...)`, 1},
				{"missing_variadic_forward", `return q.Querier.METHOD(ctx, sql, args)`, 1},
				{"wrong_receiver_field", `return q.Other.METHOD(ctx, sql, args...)`, 1},
				{"non_return_call", `q.Querier.METHOD(ctx, sql, args...); return nil, nil`, 1},
				{"additional_call", `q.Querier.METHOD(ctx, sql, args...); return q.Querier.METHOD(ctx, sql, args...)`, 2},
				{"deferred_call", `defer q.Querier.METHOD(ctx, sql, args...); return q.Querier.METHOD(ctx, sql, args...)`, 2},
				{"unapproved_generated_constant", `return q.Querier.METHOD(ctx, gen.UnapprovedStatement, args...)`, 1},
			} {
				t.Run(tc.name, func(t *testing.T) {
					want := make([]string, tc.calls)
					for i := range want {
						want[i] = call
					}
					rawSQLForwarderCheck(t, f.path, f.source(tc.body), want...)
				})
			}
		})
	}
}

func TestRawSQLForwarderIdentityAndSignatureAreExact(t *testing.T) {
	for _, f := range rawSQLForwarders() {
		t.Run(f.method, func(t *testing.T) {
			body := "\treturn q.Querier.METHOD(ctx, sql, args...)"
			source := f.source(body)
			call := ": calls " + f.method + ", which runs SQL outside sqlc's queries"
			for _, path := range []string{
				strings.Replace(f.path, "linking", "workspace", 1),
				strings.TrimSuffix(f.path, ".go") + "_other.go",
				rawSQLForwarders()[1].path,
				rawSQLForwarders()[0].path,
			} {
				if path != f.path {
					rawSQLForwarderCheck(t, path, source, call)
				}
			}
			for _, change := range []struct{ from, to string }{
				{"(q " + f.receiver + ")", "(q OtherQuerier)"},
				{"(q " + f.receiver + ")", "(q *" + f.receiver + ")"},
				{") " + f.method + "(", ") Forward("},
				{"ctx context.Context", "ctx OtherContext"},
				{"sql string", "sql Statement"},
				{"args ...any", "args []any"},
				{"args ...any", "args ...Value"},
				{"args ...any", "extra int, args ...any"},
				{"(" + f.result + ", error)", "(result " + f.result + ", err error)"},
			} {
				rawSQLForwarderCheck(t, f.path, strings.Replace(source, change.from, change.to, 1), call)
			}
			renamed := strings.ReplaceAll(source, "sql", "statement")
			rawSQLForwarderCheck(t, f.path, renamed, call)
			rawSQLForwarderCheck(t, f.path, f.source("return q.Querier.QueryRow(ctx, sql)"),
				": calls QueryRow, which runs SQL outside sqlc's queries")
		})
	}
}

func TestRawSQLForwardersStillReportOtherCallsAndSQLLiterals(t *testing.T) {
	for _, f := range rawSQLForwarders() {
		t.Run(f.method, func(t *testing.T) {
			call := ": calls " + f.method + ", which runs SQL outside sqlc's queries"
			const held = ": holds SQL outside sqlc's queries"
			// A valid forwarding call does not suppress the independent literal check.
			rawSQLForwarderCheck(t, f.path, f.source(`const statement = "SELECT id FROM users"; log(statement); return q.Querier.METHOD(ctx, sql, args...)`), held)
			rawSQLForwarderCheck(t, f.path, f.source(`return q.Querier.METHOD(ctx, "SELECT id FROM users", args...)`), call, held)
			// SELECT 1 is outside sqlText's pattern; its actual call still reports.
			rawSQLForwarderCheck(t, f.path, f.source(`other.Exec(ctx, "SELECT 1"); return q.Querier.METHOD(ctx, sql, args...)`),
				": calls Exec, which runs SQL outside sqlc's queries")
			other := "\nfunc (q " + f.receiver + ") Other(ctx context.Context, sql string, args ...any) { q.Querier." + f.method + "(ctx, sql, args...) }\n"
			rawSQLForwarderCheck(t, f.path, f.source("return q.Querier.METHOD(ctx, sql, args...)")+other, call)
		})
	}
}
