package archtest

import (
	"slices"
	"strings"
	"testing"
)

// rawSQLBase is a module as the rule wants it: a store that runs sqlc's
// queries, a handler that reads a URL's query and words its messages in
// lower case, sqlc's generated code and a test that seed with SQL.
func rawSQLBase() map[string]string {
	return map[string]string{
		"internal/modules/workspace/adapter/postgres/store.go": `package postgresadapter

func (s *Store) Members(ctx context.Context, id uuid.UUID) ([]gen.ListMembersRow, error) {
	// SELECT m.id FROM workspace_members m: a comment is not SQL
	return s.queries(ctx).ListMembers(ctx, id)
}
`,
		"internal/modules/workspace/adapter/http/members.go": `package httpadapter

const hint = "select a workspace from the list, then update your settings"

func page(r *http.Request) string { return r.URL.Query().Get("page") }
`,
		"internal/modules/workspace/adapter/postgres/gen/members.sql.go": "package gen\n\nconst listMembers = `-- name: ListMembers :many\n" +
			"SELECT id FROM workspace_members WHERE workspace_id = $1`\n\n" +
			"func (q *Queries) ListMembers(ctx context.Context, id uuid.UUID) (pgx.Rows, error) { return q.db.Query(ctx, listMembers, id) }\n",
		"internal/modules/workspace/adapter/postgres/store_test.go": "package postgresadapter_test\n\n" +
			"func seed(pool *pgxpool.Pool) { pool.Exec(context.Background(), \"INSERT INTO users (id) VALUES ($1)\", 1) }\n",
	}
}

func TestRawSQLOfTheBaseLayoutPasses(t *testing.T) {
	if got := rawSQLViolations(rawSQLBase()); len(got) != 0 {
		t.Errorf("violations = %q, want none", got)
	}
}

// Each way a module could run or hold SQL of its own is reported, at its
// line.
func TestRawSQLViolationsAreReported(t *testing.T) {
	const file = "internal/modules/workspace/adapter/postgres/members.go"
	tests := []struct {
		name, source string
		want         []string
	}{
		{"a member list that joins users, run on the context's transaction", "package postgresadapter\n\n" +
			"func (s *Store) Members(ctx context.Context, id uuid.UUID) (pgx.Rows, error) {\n" +
			"\treturn postgres.DB(ctx, s.pool).Query(ctx, `SELECT m.id, u.email\n\t\tFROM workspace_members m JOIN users u ON u.id = m.member_id\n" +
			"\t\tWHERE m.workspace_id = $1`, id)\n}\n",
			[]string{file + ":4:9: calls Query, which runs SQL outside sqlc's queries", file + ":4:45: holds SQL outside sqlc's queries"}},
		{"a statement in a constant, run on the pool", "package postgresadapter\n\n" +
			"const deactivate = \"UPDATE users SET is_active = false WHERE id = $1\"\n\n" +
			"func (s *Store) Off(ctx context.Context, id uuid.UUID) error {\n\t_, err := s.pool.Exec(ctx, deactivate, id)\n\treturn err\n}\n",
			[]string{file + ":3:20: holds SQL outside sqlc's queries", file + ":6:12: calls Exec, which runs SQL outside sqlc's queries"}},
		{"a row read on a transaction, the SQL built elsewhere", "package postgresadapter\n\n" +
			"func (s *Store) Email(ctx context.Context, tx pgx.Tx, q string) error {\n\treturn tx.QueryRow(ctx, q).Scan(nil)\n}\n",
			[]string{file + ":4:9: calls QueryRow, which runs SQL outside sqlc's queries"}},
		{"a batch", "package postgresadapter\n\n" +
			"func (s *Store) Many(ctx context.Context, b *pgx.Batch) {\n\tb.Queue(\"DELETE FROM workspace_members WHERE id = $1\", 1)\n" +
			"\ts.pool.SendBatch(ctx, b).Close()\n}\n",
			[]string{file + ":4:10: holds SQL outside sqlc's queries", file + ":5:2: calls SendBatch, which runs SQL outside sqlc's queries"}},
		{"a copy", "package postgresadapter\n\n" +
			"func (s *Store) Load(ctx context.Context, c *pgx.Conn, rows pgx.CopyFromSource) {\n" +
			"\tc.CopyFrom(ctx, pgx.Identifier{\"users\"}, []string{\"id\"}, rows)\n}\n",
			[]string{file + ":4:2: calls CopyFrom, which runs SQL outside sqlc's queries"}},
		{"a prepared statement", "package postgresadapter\n\n" +
			"func (s *Store) Ready(ctx context.Context, c *pgx.Conn, q string) { c.Prepare(ctx, \"members\", q) }\n",
			[]string{file + ":3:69: calls Prepare, which runs SQL outside sqlc's queries"}},
		{"statements on the connection under pgx", "package postgresadapter\n\n" +
			"func (s *Store) Raw(ctx context.Context, c *pgx.Conn, q string, w io.Writer) {\n" +
			"\tc.PgConn().ExecParams(ctx, q, nil, nil, nil, nil)\n\tc.PgConn().ExecPrepared(ctx, \"members\", nil, nil, nil)\n" +
			"\tc.PgConn().CopyTo(ctx, w, q)\n}\n",
			[]string{
				file + ":4:2: calls ExecParams, which runs SQL outside sqlc's queries",
				file + ":5:2: calls ExecPrepared, which runs SQL outside sqlc's queries",
				file + ":6:2: calls CopyTo, which runs SQL outside sqlc's queries",
			}},
		{"a table emptied and a merge, held for later", "package postgresadapter\n\n" +
			"const (\n\twipe  = \"TRUNCATE workspace_members\"\n" +
			"\tmerge = \"MERGE INTO users u USING workspace_members m ON u.id = m.member_id WHEN MATCHED THEN DO NOTHING\"\n)\n",
			[]string{file + ":4:10: holds SQL outside sqlc's queries", file + ":5:10: holds SQL outside sqlc's queries"}},
		{"a batch on the connection under pgx", "package postgresadapter\n\n" +
			"func (s *Store) Many(ctx context.Context, c *pgx.Conn, b *pgconn.Batch) { c.PgConn().ExecBatch(ctx, b) }\n",
			[]string{file + ":3:75: calls ExecBatch, which runs SQL outside sqlc's queries"}},
	}
	for _, tt := range tests {
		files := rawSQLBase()
		files[file] = tt.source
		if got := rawSQLViolations(files); !slices.Equal(got, tt.want) {
			t.Errorf("%s: violations =\n%q\nwant\n%q", tt.name, got, tt.want)
		}
	}
}

// SQL outside the store is reported wherever it sits in a module, not only
// under adapter/postgres: a literal and a call, in the app layer.
func TestRawSQLInTheAppLayerIsReported(t *testing.T) {
	const file = "internal/modules/workspace/app/preferences_sql.go"
	files := rawSQLBase()
	files[file] = "package app\n\nvar insert = `INSERT INTO workspace_user_properties (id) VALUES ($1)`\n\n" +
		"func run(ctx context.Context, tx pgx.Tx, q string) { tx.Exec(ctx, q) }\n"
	want := []string{
		file + ":3:14: holds SQL outside sqlc's queries",
		file + ":5:54: calls Exec, which runs SQL outside sqlc's queries",
	}
	if got := rawSQLViolations(files); !slices.Equal(got, want) {
		t.Errorf("violations =\n%q\nwant\n%q", got, want)
	}
}

// Only the gen directory right under a module's adapter/<kind>/ is sqlc's
// and oapi-codegen's: a file whose path merely contains "gen", and a gen
// directory anywhere else, are checked like any other.
func TestRawSQLInAPathContainingGenIsReported(t *testing.T) {
	const purge = "const purge = \"DELETE FROM workspace_members WHERE workspace_id = $1\"\n"
	for _, file := range []string{"internal/modules/workspace/app/generate.go", "internal/modules/workspace/app/gen/x.go",
		"internal/modules/workspace/adapter/gen/x.go"} {
		files := rawSQLBase()
		files[file] = "package app\n\n" + purge
		want := []string{file + ":3:15: holds SQL outside sqlc's queries"}
		if got := rawSQLViolations(files); !slices.Equal(got, want) {
			t.Errorf("violations =\n%q\nwant\n%q", got, want)
		}
	}
}

// A file that does not parse is reported, at the position go/parser gives,
// whatever its message: it cannot hide raw SQL.
func TestAFileThatDoesNotParseIsReported(t *testing.T) {
	const file = "internal/modules/workspace/adapter/postgres/members.go"
	files := rawSQLBase()
	files[file] = "package postgresadapter\n\nfunc (\n"
	got := rawSQLViolations(files)
	if prefix := file + " does not parse: " + file + ":3:8: "; len(got) != 1 || !strings.HasPrefix(got[0], prefix) {
		t.Errorf("violations = %q, want one that starts %q", got, prefix)
	}
}
