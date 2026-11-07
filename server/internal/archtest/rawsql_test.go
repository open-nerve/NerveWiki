package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestModulesRunSQLOnlyThroughSQLC holds all of a module's SQL to v0.1
// design 7.1, not only the queries sqlc sees:
// outside the gen packages that sqlc writes, a module's production code
// neither runs a statement of its own nor holds one. TestSQLCSchemaScope
// sees sqlc's queries only, so a statement run through pgx directly could
// read another module's tables, e.g. a member list that joins users instead
// of asking MemberProfiles. Tests are left out: they seed fixtures with SQL.
func TestModulesRunSQLOnlyThroughSQLC(t *testing.T) {
	registerSources(t)
	files := map[string]string{}
	root := filepath.Join(moduleRoot, "internal", "modules")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// A walk that finds no store must not pass as "no raw SQL".
	if !slices.ContainsFunc(slices.Collect(maps.Keys(files)), func(p string) bool {
		return strings.HasSuffix(p, "/adapter/postgres/store.go")
	}) {
		t.Fatalf("read %d files under internal/modules and no adapter/postgres/store.go: the rule checks nothing", len(files))
	}
	for _, v := range rawSQLViolations(files) {
		t.Error(v)
	}
}

// isStatementMethod reports the pgx methods that run SQL they are given: pgx's
// Conn, Tx and Batch results, pgxpool's Pool and Conn, and
// platform/postgres's Querier have the first six, and pgconn's PgConn,
// which a pgx.Conn's PgConn() returns, the last four. The call rule counts
// only calls with at least two arguments, a context and the SQL (or the
// batch, the table or the statement's name): a method of one of these names
// that takes fewer, such as url.URL.Query, runs no statement.
func isStatementMethod(name string) bool {
	switch name {
	case "Exec", "Query", "QueryRow", "SendBatch", "CopyFrom", "Prepare",
		"ExecParams", "ExecPrepared", "CopyTo", "ExecBatch":
		return true
	}
	return false
}

// sqlText matches a string literal that holds a statement: upper-case SQL,
// as every query of this repository is written. Lower-case prose, such as a
// message that tells a user to "select a workspace from the list", is not
// matched.
var sqlText = regexp.MustCompile(`\b(?:SELECT\b[\s\S]*\bFROM|INSERT\s+INTO|UPDATE\s+\S+\s+SET|DELETE\s+FROM|TRUNCATE|MERGE\s+INTO)\b`)

// rawSQLViolations checks the Go files of internal/modules, by path
// relative to server/: in a file that is neither a test nor generated (in
// the gen directory right under adapter/<kind>/, where sqlc and oapi-codegen
// write), no call of an isStatementMethod method with two arguments or more,
// except checked forwarding calls and named own-gen calls below, and no
// string literal that sqlText matches. A file that does not parse is
// reported too, so that it cannot hide either.
//
// The rule finds raw SQL written by accident, not an evasion. Out of its
// reach are: a statement that sqlText does not match, one without FROM
// (SELECT 1, SELECT pg_advisory_xact_lock($1)), a CTE (WITH ...), DDL or LOCK
// TABLE, when a wrapper of the pgx methods or a method with fewer than two
// arguments runs it; a pgx.Batch's Queue(sql) with a variable argument; a
// method value (run := tx.Query); SQL built from lower-case pieces. A port
// method of an app file named like an isStatementMethod method and called
// with two arguments or more is reported: name it after what it does
// (MemberByID, not Query).
func rawSQLViolations(files map[string]string) []string {
	var found []string
	for _, path := range slices.Sorted(maps.Keys(files)) {
		if strings.HasSuffix(path, "_test.go") || generated(path) {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, files[path], parser.SkipObjectResolution)
		if err != nil {
			found = append(found, fmt.Sprintf("%s does not parse: %v", path, err))
			continue
		}
		forwarded := rawSQLForwardedCalls(path, f)
		collectors := rawSQLIngestOldBodyCalls(path, f)
		if collector := rawSQLCollectorCall(path, f); collector != nil {
			collectors[collector] = true
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok && isStatementMethod(sel.Sel.Name) && len(n.Args) >= 2 && !forwarded[n] && !collectors[n] {
					found = append(found, fmt.Sprintf("%s: calls %s, which runs SQL outside sqlc's queries", fset.Position(n.Pos()), sel.Sel.Name))
				}
			case *ast.BasicLit:
				if n.Kind != token.STRING {
					return true
				}
				if s, err := strconv.Unquote(n.Value); err == nil && sqlText.MatchString(s) {
					found = append(found, fmt.Sprintf("%s: holds SQL outside sqlc's queries", fset.Position(n.Pos())))
				}
			}
			return true
		})
	}
	return found
}

// rawSQLCollectorCall permits only the named uses of linking's own sqlc
// constants. Protected identifiers may occur only at the checked argument
// nodes: writes, aliases, shadows and address escape invalidate the call.
func rawSQLCollectorCall(path string, f *ast.File) *ast.CallExpr {
	if f.Name.Name != "postgresadapter" {
		return nil
	}
	var name string
	var params [][3]string
	var protected []string
	var query func(*ast.CallExpr) bool
	switch path {
	case "internal/modules/linking/adapter/postgres/links_reached.go":
		name, query = "linksReached", rawSQLLinksReachedQuery
		params = [][3]string{{"ctx", "context", "Context"}, {"q", "postgres", "Querier"}, {"notebookID", "uuid", "UUID"}, {"r", "domain", "Reach"}}
		protected = []string{"ctx", "q", "gen", "notebookID", "r"}
	case "internal/modules/linking/adapter/postgres/retention_collect.go":
		name, query = "inspectPageLinksForRetention", rawSQLRetentionQuery
		params = [][3]string{{"ctx", "context", "Context"}, {"q", "postgres", "Querier"}, {"arg", "gen", "InspectPageLinksForRetentionParams"}, {"links", "gen", "InsertLinksParams"}}
		protected = []string{"ctx", "q", "gen", "arg"}
	case "internal/modules/linking/adapter/postgres/prepared_source_collect.go":
		name, query = "inspectPageLinksForPreparation", rawSQLPreparationQuery
		params = [][3]string{{"ctx", "context", "Context"}, {"q", "postgres", "Querier"}, {"arg", "gen", "InspectPageLinksForPreparationParams"}, {"links", "", "preparedLinkFacts"}}
		protected = []string{"ctx", "q", "gen", "arg"}
	case "internal/modules/linking/adapter/postgres/links_reached_except.go":
		name, query = "linksReachedExcept", rawSQLLinksExceptQuery
		params = [][3]string{{"ctx", "context", "Context"}, {"q", "postgres", "Querier"}, {"arg", "gen", "LinksReachedExceptParams"}}
		protected = []string{"ctx", "q", "gen", "arg"}
	default:
		return nil
	}
	imports := 0
	for _, imp := range f.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil
		}
		name := value[strings.LastIndex(value, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "gen" {
			if value != "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen" {
				return nil
			}
			imports++
		}
	}
	if imports != 1 {
		return nil
	}
	var fn *ast.FuncDecl
	for _, decl := range f.Decls {
		if candidate, ok := decl.(*ast.FuncDecl); ok && candidate.Name.Name == name {
			if fn != nil {
				return nil
			}
			fn = candidate
		}
	}
	if fn == nil || fn.Recv != nil || fn.Body == nil || fn.Type.TypeParams != nil ||
		fn.Type.Params == nil || len(fn.Type.Params.List) != len(params) || fn.Type.Results == nil || len(fn.Type.Results.List) != 2 {
		return nil
	}
	for i, want := range params {
		field := fn.Type.Params.List[i]
		typ := field.Type
		if (name == "inspectPageLinksForRetention" || name == "inspectPageLinksForPreparation") && i == 3 {
			pointer, ok := typ.(*ast.StarExpr)
			if !ok {
				return nil
			}
			typ = pointer.X
		}
		validType := rawSQLSelector(typ, want[1], want[2])
		if name == "inspectPageLinksForPreparation" && i == 3 {
			validType = rawSQLName(typ, "preparedLinkFacts")
		}
		if len(field.Names) != 1 || !rawSQLName(field.Names[0], want[0]) || !validType {
			return nil
		}
	}
	results := fn.Type.Results.List
	validResult := rawSQLName(results[0].Type, "retentionPlan")
	if name == "inspectPageLinksForPreparation" {
		validResult = rawSQLName(results[0].Type, "preparedLinkRead")
	}
	if name == "linksReached" || name == "linksReachedExcept" {
		out, ok := results[0].Type.(*ast.ArrayType)
		validResult = ok && out.Len == nil && rawSQLSelector(out.Elt, "app", "Link")
	}
	if !validResult || !rawSQLName(results[1].Type, "error") || len(results[0].Names) != 0 || len(results[1].Names) != 0 || len(fn.Body.List) == 0 {
		return nil
	}
	// A direct first statement cannot be a closure, deferred call or loop.
	assign, ok := fn.Body.List[0].(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE || len(assign.Lhs) != 2 || !rawSQLName(assign.Lhs[0], "rows") || !rawSQLName(assign.Lhs[1], "err") || len(assign.Rhs) != 1 {
		return nil
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || !query(call) {
		return nil
	}
	uses := map[*ast.Ident]bool{call.Fun.(*ast.SelectorExpr).X.(*ast.Ident): true}
	for _, arg := range call.Args {
		if sel, ok := arg.(*ast.SelectorExpr); ok {
			uses[sel.X.(*ast.Ident)] = true
		} else {
			uses[arg.(*ast.Ident)] = true
		}
	}
	unchanged, matches := true, 0
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && query(call) {
			matches++
		}
		return true
	})
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			unchanged = false
		case *ast.Ident:
			if slices.Contains(protected, n.Name) {
				unchanged = unchanged && uses[n]
			}
		}
		return true
	})
	if unchanged && matches == 1 {
		return call
	}
	return nil
}

func rawSQLLinksReachedQuery(call *ast.CallExpr) bool {
	return rawSQLSelector(call.Fun, "q", "Query") && len(call.Args) == 6 && !call.Ellipsis.IsValid() &&
		rawSQLName(call.Args[0], "ctx") && rawSQLSelector(call.Args[1], "gen", "LinksReached") &&
		rawSQLName(call.Args[2], "notebookID") && rawSQLSelector(call.Args[3], "r", "Keys") &&
		rawSQLSelector(call.Args[4], "r", "Targets") && rawSQLSelector(call.Args[5], "r", "Sources")
}

func rawSQLRetentionQuery(call *ast.CallExpr) bool {
	return rawSQLSelector(call.Fun, "q", "Query") && len(call.Args) == 5 && !call.Ellipsis.IsValid() &&
		rawSQLName(call.Args[0], "ctx") && rawSQLSelector(call.Args[1], "gen", "InspectPageLinksForRetention") &&
		rawSQLSelector(call.Args[2], "arg", "SourceID") && rawSQLSelector(call.Args[3], "arg", "NotebookID") &&
		rawSQLSelector(call.Args[4], "arg", "Extractor")
}

// rawSQLForwardedCalls permits only the named adapters that pass sqlc's SQL
// through unchanged while selecting an execution mode or argument encoder.
// The exception belongs to exact return calls, never to a file or type.
func rawSQLForwardedCalls(path string, f *ast.File) map[*ast.CallExpr]bool {
	var receiver, method string
	switch path {
	case "internal/modules/linking/adapter/postgres/lint.go":
		receiver, method = "lintBodyQuerier", "Query"
	case "internal/modules/linking/adapter/postgres/retention_array_encoding.go":
		receiver, method = "retentionArrayQuerier", "Exec"
	case "internal/modules/linking/adapter/postgres/set_resolutions_mode.go":
		receiver, method = "setResolutionBatchQuerier", "Exec"
	default:
		return nil
	}
	allowed := map[*ast.CallExpr]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !rawSQLForwarderSignature(fn, receiver, method) {
			continue
		}
		calls := map[*ast.CallExpr]bool{}
		sqlReads := map[*ast.Ident]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if ret, ok := n.(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
				if call, ok := ret.Results[0].(*ast.CallExpr); ok && rawSQLForwardingCall(call, method, receiver == "lintBodyQuerier" || receiver == "setResolutionBatchQuerier") {
					calls[call] = true
					sqlReads[call.Args[1].(*ast.Ident)] = true
				}
			}
			// The encoder's budget inspection reads the string by value.
			if call, ok := n.(*ast.CallExpr); ok && receiver == "retentionArrayQuerier" && rawSQLName(call.Fun, "retentionArrayBudget") &&
				len(call.Args) == 2 && !call.Ellipsis.IsValid() && rawSQLName(call.Args[0], "sql") && rawSQLName(call.Args[1], "args") {
				sqlReads[call.Args[0].(*ast.Ident)] = true
			}
			return true
		})
		unchanged := true
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				// Every use is accounted for by node identity, so assignment,
				// aliasing, concatenation, shadowing and address escape fail.
				if n.Name == "sql" && !sqlReads[n] {
					unchanged = false
				}
			case *ast.FuncLit:
				// These wrappers have no nested functions. Do not attribute
				// a closure's parameters or calls to the outer signature.
				unchanged = false
			case *ast.AssignStmt:
				for _, lhs := range n.Lhs {
					unchanged = unchanged && !rawSQLBindingMention(lhs)
				}
			case *ast.RangeStmt:
				unchanged = unchanged && !rawSQLBindingMention(n.Key) && !rawSQLBindingMention(n.Value)
			case *ast.ValueSpec:
				for _, name := range n.Names {
					unchanged = unchanged && !rawSQLBindingMention(name)
				}
			case *ast.TypeSpec:
				unchanged = unchanged && !rawSQLBindingMention(n.Name)
			case *ast.Field:
				for _, name := range n.Names {
					unchanged = unchanged && !rawSQLBindingMention(name)
				}
			case *ast.IncDecStmt:
				unchanged = unchanged && !rawSQLBindingMention(n.X)
			case *ast.UnaryExpr:
				if n.Op == token.AND {
					unchanged = unchanged && !rawSQLBindingMention(n.X)
				}
			}
			return true
		})
		if unchanged {
			maps.Copy(allowed, calls)
		}
	}
	return allowed
}

func rawSQLForwarderSignature(fn *ast.FuncDecl, receiver, method string) bool {
	if fn.Name.Name != method || fn.Body == nil || fn.Recv == nil || len(fn.Recv.List) != 1 ||
		len(fn.Recv.List[0].Names) != 1 || !rawSQLName(fn.Recv.List[0].Names[0], "q") || !rawSQLName(fn.Recv.List[0].Type, receiver) ||
		fn.Type.TypeParams != nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 3 {
		return false
	}
	params := fn.Type.Params.List
	for i, name := range []string{"ctx", "sql", "args"} {
		if len(params[i].Names) != 1 || !rawSQLName(params[i].Names[0], name) {
			return false
		}
	}
	variadic, ok := params[2].Type.(*ast.Ellipsis)
	if !rawSQLSelector(params[0].Type, "context", "Context") || !rawSQLName(params[1].Type, "string") || !ok || !rawSQLName(variadic.Elt, "any") {
		return false
	}
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 2 {
		return false
	}
	results := fn.Type.Results.List
	resultPackage, resultType := "pgx", "Rows"
	if method == "Exec" {
		resultPackage, resultType = "pgconn", "CommandTag"
	}
	return len(results[0].Names) == 0 && len(results[1].Names) == 0 &&
		rawSQLSelector(results[0].Type, resultPackage, resultType) && rawSQLName(results[1].Type, "error")
}

func rawSQLForwardingCall(call *ast.CallExpr, method string, modePrefix bool) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != method || !rawSQLSelector(sel.X, "q", "Querier") || len(call.Args) != 3 ||
		!call.Ellipsis.IsValid() || !rawSQLName(call.Args[0], "ctx") || !rawSQLName(call.Args[1], "sql") {
		return false
	}
	if rawSQLName(call.Args[2], "args") {
		return true
	}
	// Only the exact mode forwarders prepend one pgx mode to original args.
	prepend, ok := call.Args[2].(*ast.CallExpr)
	if !modePrefix || !ok || !rawSQLName(prepend.Fun, "append") || len(prepend.Args) != 2 ||
		!prepend.Ellipsis.IsValid() || !rawSQLName(prepend.Args[1], "args") {
		return false
	}
	mode, ok := prepend.Args[0].(*ast.CompositeLit)
	if !ok || len(mode.Elts) != 1 || !rawSQLName(mode.Elts[0], "mode") {
		return false
	}
	slice, ok := mode.Type.(*ast.ArrayType)
	return ok && slice.Len == nil && rawSQLName(slice.Elt, "any")
}

func rawSQLName(n ast.Node, name string) bool {
	id, ok := n.(*ast.Ident)
	return ok && id.Name == name
}

func rawSQLSelector(n ast.Node, base, name string) bool {
	sel, ok := n.(*ast.SelectorExpr)
	return ok && rawSQLName(sel.X, base) && sel.Sel.Name == name
}

// Reject writes, declarations or addresses involving any forwarding binding.
// This deliberately rejects harmless local shadowing too: these small
// wrappers need none, and name equality alone must never justify a forward.
func rawSQLBindingMention(n ast.Node) bool {
	if n == nil {
		return false
	}
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			switch id.Name {
			case "q", "ctx", "sql", "args":
				found = true
			}
		}
		return !found
	})
	return found
}

// generated reports whether path, relative to server/, is in the gen
// directory of a module's adapter (internal/modules/<module>/adapter/<kind>/gen/),
// where sqlc and oapi-codegen write; a gen directory anywhere else is
// checked like any other.
func generated(path string) bool {
	parts := strings.Split(path, "/")
	return len(parts) > 6 && parts[0] == "internal" && parts[1] == "modules" && parts[3] == "adapter" && parts[5] == "gen"
}

func rawSQLPreparationQuery(call *ast.CallExpr) bool {
	return rawSQLSelector(call.Fun, "q", "Query") && len(call.Args) == 5 && !call.Ellipsis.IsValid() &&
		rawSQLName(call.Args[0], "ctx") && rawSQLSelector(call.Args[1], "gen", "InspectPageLinksForPreparation") &&
		rawSQLSelector(call.Args[2], "arg", "SourceID") && rawSQLSelector(call.Args[3], "arg", "NotebookID") &&
		rawSQLSelector(call.Args[4], "arg", "Extractor")
}

func rawSQLLinksExceptQuery(call *ast.CallExpr) bool {
	return rawSQLSelector(call.Fun, "q", "Query") && len(call.Args) == 7 && !call.Ellipsis.IsValid() &&
		rawSQLName(call.Args[0], "ctx") && rawSQLSelector(call.Args[1], "gen", "LinksReachedExcept") &&
		rawSQLSelector(call.Args[2], "arg", "ExcludedSources") && rawSQLSelector(call.Args[3], "arg", "Keys") &&
		rawSQLSelector(call.Args[4], "arg", "NotebookID") && rawSQLSelector(call.Args[5], "arg", "Targets") &&
		rawSQLSelector(call.Args[6], "arg", "Sources")
}
