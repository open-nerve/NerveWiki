package domain_test

import (
	"bytes"
	"cmp"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// search is a call of the search family whose patterns are not all short
// literals, or a function of the family taken as a value: the file it is
// in, under the page module's directory, the function, and its source.
type search struct{ file, fn, source string }

// allowedSearch is a search the check lets be, and why it may.
type allowedSearch struct {
	search
	why string
}

// allowedSearches are the searches of the packages old_string passes
// through whose patterns no agent gives (M9/P2 design 3.9). An entry that
// matches no search fails the check as a search no entry matches does:
// the table holds no entry gone stale.
func allowedSearches() []allowedSearch {
	return []allowedSearch{
		{search{"domain/edit.go", "matcher.index", "strings.IndexByte(s[i:], p[0])"},
			"the matcher skips the bytes that cannot start its pattern by one byte of it: linear"},
		{search{"domain/client.go", "Client.Valid", "strings.CutPrefix(string(c), mcpPrefix)"}, "a constant of 4 bytes"},
		{search{"domain/client.go", "Client.Valid", "strings.ContainsFunc(name, shared.Unshowable)"}, "a function"},
		{search{"domain/glob.go", "ParseGlob",
			"strings.ContainsFunc(seg, func(r rune) bool { return r != '*' && r != '?' && !shared.TitleHolds(r) })"}, "a function"},
		{search{"domain/search.go", "lexemes.scan", "strings.IndexFunc(s[i:], func(r rune) bool { return !inDataURL(r) })"},
			"a function"},
		{search{"domain/search.go", "inDataURL", "strings.ContainsRune(" + strconv.Quote(")\"'<>]`") + ", r)"},
			"a character, in a constant of seven bytes"},
		{search{"domain/search.go", "parseTerm", "regexp.MustCompile(t.pattern(syntaxRE2, text))"},
			"a search's term, of a query of at most MaxSearchQueryBytes"},
		{search{"domain/search.go", "SearchQuery.InContents", "regexp.MustCompile(t.pattern(syntaxRE2, t.content))"},
			"a search's term in NFC, of a query of at most MaxSearchQueryBytes"},
		{search{"domain/grep.go", "GrepPattern.Compile", "regexp.Compile(p.expr)"},
			"grep's pattern, of at most MaxGrepPatternBytes and MaxGrepInsts, its lines matched within MaxGrepWork (M9/P4 design 3.6)"},
	}
}

// inSearchFamily reports whether pkg's function name finds, cuts, splits
// or replaces by a pattern (M9/P2 design 3.9): of strings and bytes, the
// functions whose names start with Index, LastIndex, Contains, Count, Cut,
// Split or Replace, NewReplacer, and Trim, TrimLeft and TrimRight, which
// look a cutset up for each byte; of regexp, the compilers and matchers,
// whose pattern is the program.
func inSearchFamily(pkg, name string) bool {
	switch pkg {
	case "strings", "bytes":
		switch name {
		case "NewReplacer", "Trim", "TrimLeft", "TrimRight":
			return true
		}
		return slices.ContainsFunc([]string{"Index", "LastIndex", "Contains", "Count", "Cut", "Split", "Replace"},
			func(prefix string) bool { return strings.HasPrefix(name, prefix) })
	case "regexp":
		switch name {
		case "Compile", "MustCompile", "CompilePOSIX", "MustCompilePOSIX", "Match", "MatchString", "MatchReader":
			return true
		}
	}
	return false
}

// patternsOf are the arguments of a call of pkg's name that are patterns:
// regexp's first, each old string of NewReplacer, the second of the
// others: Index's, Replace's old, Trim's cutset.
func patternsOf(pkg, name string, args []ast.Expr) []ast.Expr {
	switch {
	case pkg == "regexp":
		return args[:min(len(args), 1)]
	case name == "NewReplacer":
		var olds []ast.Expr
		for i := 0; i < len(args); i += 2 {
			olds = append(olds, args[i])
		}
		return olds
	case len(args) > 1:
		return args[1:2]
	}
	return nil
}

// shortLiteral reports whether e is a literal of at most 32 bytes, a
// string, a character or a number, or []byte of one: a search compares a
// byte it scans with such a pattern 32 times at most.
func shortLiteral(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.ParenExpr:
		return shortLiteral(x.X)
	case *ast.BasicLit:
		s, err := strconv.Unquote(x.Value)
		return x.Kind != token.STRING || err == nil && len(s) <= 32
	case *ast.CallExpr:
		at, ok := x.Fun.(*ast.ArrayType)
		if !ok || at.Len != nil || len(x.Args) != 1 {
			return false
		}
		elt, ok := at.Elt.(*ast.Ident)
		return ok && elt.Name == "byte" && shortLiteral(x.Args[0])
	}
	return false
}

// funcName is the name of the function d declares, a method's as its
// receiver's type and its name; "" for a declaration of no function.
func funcName(d ast.Decl) string {
	f, ok := d.(*ast.FuncDecl)
	if !ok {
		return ""
	}
	if f.Recv == nil || len(f.Recv.List) == 0 {
		return f.Name.Name
	}
	recv := f.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	switch x := recv.(type) {
	case *ast.IndexExpr:
		recv = x.X
	case *ast.IndexListExpr:
		recv = x.X
	}
	if id, ok := recv.(*ast.Ident); ok {
		return id.Name + "." + f.Name.Name
	}
	return f.Name.Name
}

// searchesIn are the searches of files, by their names, in the order of
// the names and of the source: a call of the family whose patterns are not
// all short literals, or a function of the family not called, a value. A
// file's imports name its packages, under an alias too; in a file that
// imports one with a dot, any name of its family counts, called or not.
func searchesIn(fset *token.FileSet, files map[string]*ast.File) []search {
	var out []search
	for _, file := range slices.Sorted(maps.Keys(files)) {
		f := files[file]
		named, dotted := map[string]string{}, []string{}
		for _, im := range f.Imports {
			path, err := strconv.Unquote(im.Path.Value)
			if err != nil || path != "strings" && path != "bytes" && path != "regexp" {
				continue
			}
			switch {
			case im.Name == nil:
				named[path] = path
			case im.Name.Name == ".":
				dotted = append(dotted, path)
			case im.Name.Name != "_":
				named[im.Name.Name] = path
			}
		}
		// family is the package and the name of the family's function e
		// names, if it names one.
		family := func(e ast.Expr) (string, string, bool) {
			switch x := e.(type) {
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && inSearchFamily(named[id.Name], x.Sel.Name) {
					return named[id.Name], x.Sel.Name, true
				}
			case *ast.Ident:
				for _, pkg := range dotted {
					if inSearchFamily(pkg, x.Name) {
						return pkg, x.Name, true
					}
				}
			}
			return "", "", false
		}
		for _, d := range f.Decls {
			fn := funcName(d)
			seen := map[ast.Node]bool{} // a call's function, a selector's name
			ast.Inspect(d, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					pkg, name, ok := family(x.Fun)
					if !ok {
						return true
					}
					seen[x.Fun] = true
					if !slices.ContainsFunc(patternsOf(pkg, name, x.Args), func(e ast.Expr) bool { return !shortLiteral(e) }) {
						return true
					}
					out = append(out, search{file, fn, sourceOf(fset, x)})
				case *ast.SelectorExpr:
					seen[x.Sel] = true
					if _, _, ok := family(x); ok && !seen[x] {
						out = append(out, search{file, fn, sourceOf(fset, x)})
					}
				case *ast.Ident:
					if _, _, ok := family(x); ok && !seen[x] {
						out = append(out, search{file, fn, sourceOf(fset, x)})
					}
				}
				return true
			})
		}
	}
	return out
}

// sourceOf is n's source as gofmt writes it.
func sourceOf(fset *token.FileSet, n ast.Node) string {
	var b bytes.Buffer
	if err := printer.Fprint(&b, fset, n); err != nil {
		return err.Error()
	}
	return b.String()
}

// unlistedSearches are the searches no entry of allowed lists, and the
// entries that list none.
func unlistedSearches(found []search, allowed []allowedSearch) ([]search, []allowedSearch) {
	var unlisted []search
	for _, s := range found {
		if !slices.ContainsFunc(allowed, func(a allowedSearch) bool { return a.search == s }) {
			unlisted = append(unlisted, s)
		}
	}
	var stale []allowedSearch
	for _, a := range allowed {
		if !slices.Contains(found, a.search) {
			stale = append(stale, a)
		}
	}
	return unlisted, stale
}

// Only the matcher takes an agent's string as a pattern (M9/P2 design 3.9;
// review B, BC-M1): every non-test file of the three packages old_string
// passes through, page's adapter/mcp, page/app and page/domain, searches
// with the strings, bytes and regexp family only by a literal of at most
// 32 bytes, or as allowedSearches lists, with why; and each entry of it
// is still in the source. strings.Index is Rabin–Karp past a few bytes,
// quadratic for a crafted pattern, and regexp's program is the pattern:
// TestReplaceStaysWithinItsTime times what a search there would cost.
func TestReplaceSearchesOnlyWithItsMatcher(t *testing.T) {
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, dir := range []string{"adapter/mcp", "app", "domain"} {
		entries, err := os.ReadDir(filepath.Join("..", dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, filepath.Join("..", dir, e.Name()), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			files[dir+"/"+e.Name()] = f
		}
	}
	// A wrong directory must not pass as one without searches.
	for _, f := range []string{"adapter/mcp/tools.go", "app/unit.go", "domain/edit.go"} {
		if files[f] == nil {
			t.Fatalf("%s is not read: the check reads %d files", f, len(files))
		}
	}
	unlisted, stale := unlistedSearches(searchesIn(fset, files), allowedSearches())
	for _, s := range unlisted {
		t.Errorf("%s: %s in %s searches by a pattern an agent may give: search with Replace's matcher, or list it in allowedSearches with why it may",
			s.file, s.source, s.fn)
	}
	for _, a := range stale {
		t.Errorf("allowedSearches lists %s in %s of %s, which is there no more", a.source, a.fn, a.file)
	}
}

// The check finds each shape a search by an agent's string may take, in
// a synthetic source: a helper in another file, other functions of the
// family, regexp compiling old_string, a package under an alias or
// imported with a dot, a function taken as a value, a literal one byte
// past 32; and lets the listed and the short literals be. An entry that
// lists nothing is reported (M9/P2 design 3.9, its reverse control).
func TestTheSearchCheckFindsEachShape(t *testing.T) {
	sources := map[string]string{
		"app/helper.go": `package app

import (
	"bytes"
	"regexp"
	"strings"
)

func lastOf(content, old string) int { return strings.LastIndex(content, old) }

func head(b, sep []byte) [][]byte { return bytes.SplitN(b, sep, 2) }

func swap(s, old, new string) string { return strings.NewReplacer(old, new).Replace(s) }

func rest(s, p string) (string, bool) { return strings.CutPrefix(s, p) }

func compiled(old string) *regexp.Regexp { return regexp.MustCompile(regexp.QuoteMeta(old)) }

func long(s string) int { return strings.Index(s, "abcdefghijklmnopqrstuvwxyz0123456") }

func short(s string) int {
	return strings.Index(s, "abcdefghijklmnopqrstuvwxyz012345") + strings.IndexByte(s, '\n') + bytes.Count([]byte(s), []byte("\n"))
}

func value() func(string, string) int {
	f := strings.Index
	return f
}

func listed(s string, b byte) int { return strings.IndexByte(s, b) }
`,
		"domain/alias.go": `package domain

import s "strings"

func find(content, old string) int { return s.Index(content, old) }
`,
		"adapter/mcp/dot.go": `package mcpadapter

import . "bytes"

func find(b, sep []byte) int { return Index(b, sep) }
`,
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for name, src := range sources {
		f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = f
	}
	gone := allowedSearch{search{"domain/gone.go", "gone", "strings.Index(s, p)"}, "a search taken out since"}
	unlisted, stale := unlistedSearches(searchesIn(fset, files),
		[]allowedSearch{{search{"app/helper.go", "listed", "strings.IndexByte(s, b)"}, "listed"}, gone})
	want := []search{
		{"adapter/mcp/dot.go", "find", "Index(b, sep)"},
		{"app/helper.go", "compiled", "regexp.MustCompile(regexp.QuoteMeta(old))"},
		{"app/helper.go", "head", "bytes.SplitN(b, sep, 2)"},
		{"app/helper.go", "lastOf", "strings.LastIndex(content, old)"},
		{"app/helper.go", "long", `strings.Index(s, "abcdefghijklmnopqrstuvwxyz0123456")`},
		{"app/helper.go", "rest", "strings.CutPrefix(s, p)"},
		{"app/helper.go", "swap", "strings.NewReplacer(old, new)"},
		{"app/helper.go", "value", "strings.Index"},
		{"domain/alias.go", "find", "s.Index(content, old)"},
	}
	byPlace := func(a, b search) int { return cmp.Or(cmp.Compare(a.file, b.file), cmp.Compare(a.fn, b.fn)) }
	slices.SortFunc(unlisted, byPlace)
	if !slices.Equal(unlisted, want) {
		t.Errorf("the check finds %q,\nwant %q", unlisted, want)
	}
	if len(stale) != 1 || stale[0] != gone {
		t.Errorf("the check finds the entries %q stale, want the one of gone.go", stale)
	}
}
