package archtest

import (
	"go/ast"
	"strconv"
	"strings"
)

// These two helpers can only return their own generated query unchanged.
// A single statement and exact parameter fields exclude mutation, shadowing,
// aliases, closures, deferred queries and additional work in either helper.
func rawSQLIngestOldBodyCalls(path string, f *ast.File) map[*ast.CallExpr]bool {
	allowed := map[*ast.CallExpr]bool{}
	if path != "internal/modules/linking/adapter/postgres/ingest_old_f_collect.go" || f.Name.Name != "postgresadapter" {
		return allowed
	}
	imports := 0
	for _, imp := range f.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return allowed
		}
		name := value[strings.LastIndex(value, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "gen" {
			if value != "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen" {
				return allowed
			}
			imports++
		}
	}
	if imports != 1 {
		return allowed
	}
	for _, spec := range []struct {
		name, query string
		fields      []string
	}{
		{"ingestOldBodyReferencesFirst", "IngestOldBodyReferencesFirst", []string{"NotebookID", "FirstSource", "LastSource", "Take", "WikiPageIds"}},
		{"ingestOldBodyReferencesAfter", "IngestOldBodyReferencesAfter", []string{"NotebookID", "FirstSource", "LastSource", "AfterSource", "AfterStart", "Take", "WikiPageIds"}},
	} {
		var fn *ast.FuncDecl
		count := 0
		for _, decl := range f.Decls {
			if candidate, ok := decl.(*ast.FuncDecl); ok && candidate.Name.Name == spec.name {
				fn = candidate
				count++
			}
		}
		if count != 1 || fn.Recv != nil || fn.Body == nil || len(fn.Body.List) != 1 || fn.Type.TypeParams != nil ||
			fn.Type.Params == nil || len(fn.Type.Params.List) != 3 || fn.Type.Results == nil || len(fn.Type.Results.List) != 2 {
			continue
		}
		params := [][3]string{{"ctx", "context", "Context"}, {"q", "postgres", "Querier"}, {"arg", "gen", spec.query + "Params"}}
		valid := true
		for i, want := range params {
			field := fn.Type.Params.List[i]
			if len(field.Names) != 1 || !rawSQLName(field.Names[0], want[0]) || !rawSQLSelector(field.Type, want[1], want[2]) {
				valid = false
			}
		}
		results := fn.Type.Results.List
		if !valid || len(results[0].Names) != 0 || len(results[1].Names) != 0 ||
			!rawSQLSelector(results[0].Type, "pgx", "Rows") || !rawSQLName(results[1].Type, "error") {
			continue
		}
		ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		call, ok := ret.Results[0].(*ast.CallExpr)
		if !ok || !rawSQLSelector(call.Fun, "q", "Query") || call.Ellipsis.IsValid() || len(call.Args) != len(spec.fields)+2 ||
			!rawSQLName(call.Args[0], "ctx") || !rawSQLSelector(call.Args[1], "gen", spec.query) {
			continue
		}
		for i, field := range spec.fields {
			if !rawSQLSelector(call.Args[i+2], "arg", field) {
				valid = false
			}
		}
		if valid {
			allowed[call] = true
		}
	}
	return allowed
}
