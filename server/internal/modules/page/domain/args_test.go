package domain_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fieldRule is a refusal of a rule a batch's op goes through: the rule, as
// the source names its function, the call that refuses given the op's
// Args, and the fields it names with the zero Args, a tool's own call's.
type fieldRule struct {
	rule   string
	refuse func(a domain.Args) error
	fields []string
}

// fieldRules are each refusal of each rule of a write's step, one a row
// (M9/P3 design 3.2): the path's, the content's, the token's, the tree's
// pass's and the edit's, and the new names a move checks through
// RenameTitle. A rule that takes its field as a string (ParsePath,
// CheckContent, ParseVersionToken, MissingAncestors) is given a.Field, as
// CheckBatch and the plan give it.
func fieldRules(t *testing.T) []fieldRule {
	page := func(name string) domain.Node { return domain.Node{ID: uuid.NewV7(), Kind: domain.KindPage, Name: name} }
	asset := func(name string) domain.Node {
		return domain.Node{ID: uuid.NewV7(), Kind: domain.KindAsset, Name: name}
	}
	path := func(a domain.Args, s string) error {
		_, err := domain.ParsePath(a.Field("path"), s, false)
		return err
	}
	creation := func(s string, found ...domain.Node) func(domain.Args) error {
		return func(a domain.Args) error {
			_, err := domain.CreationOf(a, mustPath(t, s), found)
			return err
		}
	}
	chain := func(to string, on *domain.AttachmentOnPath, found ...domain.Node) func(domain.Args) error {
		return func(a domain.Args) error { return domain.ParentChainOf(a, mustPath(t, to), found, on) }
	}
	edit := func(old, new string) func(domain.Args) error {
		return func(a domain.Args) error { return domain.CheckEdit(a, old, new) }
	}
	replace := func(content, old, new string) func(domain.Args) error {
		return func(a domain.Args) error {
			_, err := domain.Replace(a, content, old, new, false)
			return err
		}
	}
	rename := func(n domain.Node, name string) func(domain.Args) error {
		return func(a domain.Args) error {
			_, err := domain.RenameTitle(a.Field("to"), n, name)
			return err
		}
	}
	return []fieldRule{
		{"ParsePath", func(a domain.Args) error { return path(a, strings.Repeat("a", domain.MaxPathBytes+1)) }, []string{"path"}},
		{"ParsePath", func(a domain.Args) error { return path(a, "A/") }, []string{"path"}},
		{"ParsePath", func(a domain.Args) error { return path(a, "/") }, []string{"path"}},
		{"ParsePath", func(a domain.Args) error { return path(a, strings.Repeat("a/", domain.MaxPathSegments)+"b.md") },
			[]string{"path"}},
		{"ParsePath", func(a domain.Args) error { return path(a, "A/../B.md") }, []string{"path"}},
		{"ParsePath", func(a domain.Args) error { return path(a, "A/ b.md") }, []string{"path"}},
		{"ParsePath", func(a domain.Args) error { return path(a, "A/b|c.md") }, []string{"path"}},
		{"PagePath", func(a domain.Args) error { return domain.PagePath(a, mustPath(t, "A/x.png"), "write") }, []string{"path"}},
		{"CheckContent", func(a domain.Args) error {
			return domain.CheckContent(a.Field("content"), strings.Repeat("a", domain.MaxContentBytes+1))
		}, []string{"content"}},
		{"CheckContent", func(a domain.Args) error { return domain.CheckContent(a.Field("content"), "a\x00") }, []string{"content"}},
		{"ParseVersionToken", func(a domain.Args) error {
			_, err := domain.ParseVersionToken(a.Field("base_version"), "x")
			return err
		}, []string{"base_version"}},
		{"CreationOf", creation("A/x.png/C.md", page("A"), asset("x.png")), []string{"path"}},
		{"CreationOf", creation("A/B.md/C.md", page("A")), []string{"path"}},
		{"MissingAncestors", func(a domain.Args) error {
			return domain.MissingAncestors(a.Field("to"), mustPath(t, "A.md/B.md"), 0)
		}, []string{"to"}},
		{"MoveKinds", func(a domain.Args) error { return domain.MoveKinds(a, mustPath(t, "A.md"), mustPath(t, "B/x.png")) },
			[]string{"to"}},
		{"ParentChainOf", chain("A/x.png/C.md", &domain.AttachmentOnPath{Segment: 2}, page("A"), asset("x.png")), []string{"to"}},
		{"ParentChainOf", chain("x.png/C.md", nil, asset("x.png")), []string{"to"}},
		{"ParentChainOf", chain("A/B/C.md", nil, page("A")), []string{"to"}},
		{"ParentChainOf", chain("A/B.md/C.md", nil, page("A")), []string{"to"}},
		{"CheckTitle", rename(page("Plan"), "a/b"), []string{"to"}},
		{"CheckAssetRename", rename(asset("x.png"), "x"), []string{"to"}},
		{"CheckAssetName", rename(asset("x.png"), "x.md"), []string{"to"}},
		{"DeletionOf", func(a domain.Args) error {
			sub := domain.Subtree{{Node: page("A"), Level: 1}, {Node: page("B"), Level: 2}}
			return domain.DeletionOf(a, sub, false)
		}, []string{"recursive"}},
		{"CheckEdit", edit("", "b"), []string{"old_string"}},
		{"CheckEdit", edit(strings.Repeat("a", domain.MaxEditOldBytes+1), "b"), []string{"old_string"}},
		{"CheckEdit", edit("a\x00", "b"), []string{"old_string"}},
		{"CheckEdit", edit("a", strings.Repeat("b", domain.MaxContentBytes+1)), []string{"new_string"}},
		{"CheckEdit", edit("a", "b\x00"), []string{"new_string"}},
		{"CheckEdit", edit("a", "a"), []string{"new_string"}},
		{"Replace", replace("hello", "world", "there"), []string{"old_string"}},
		{"Replace", replace("aa", "a", "b"), []string{"old_string"}},
		{"Replace", replace(strings.Repeat("x", domain.MaxContentBytes-1)+"a", "a", "bbb"), []string{"new_string"}},
	}
}

// namedFields is the fields err names, each once in its order: its problem's,
// a *Recursive's and a *PathNotFound's parent's, which an MCP answer
// writes into its words.
func namedFields(err error) []string {
	var out []string
	var e *shared.Error
	if errors.As(err, &e) {
		for _, f := range e.Fields {
			out = append(out, f.Field)
		}
	}
	var r *domain.Recursive
	if errors.As(err, &r) {
		out = append(out, r.Field)
	}
	var missing *domain.PathNotFound
	if errors.As(err, &missing) && missing.ParentOf != "" {
		out = append(out, missing.ParentOf)
	}
	var once []string
	for _, f := range out {
		if !slices.Contains(once, f) {
			once = append(once, f)
		}
	}
	return once
}

// notCounted are the functions of page/domain that answer a 422 but are no
// rule of a batch's op, each with why (M9/P3 design 3.2): the other tools'
// arguments, which a batch never takes, the batch's own message, of no op,
// and the batch's own refusals, which no tool's own call answers.
func notCounted() map[string]string {
	return map[string]string{
		"ParseActor":          "log's actor",
		"clientFilterInvalid": "log's client filter",
		"NotATask":            "toggle_task's offset",
		"TaskWouldGo":         "toggle_task's offset",
		"ParseGlob":           "glob's pattern",
		"CheckListLimit":      "list's limit",
		"invalidPattern":      "grep's pattern (ParseGrepPattern)",
		"CheckGrepLimit":      "grep's limit",
		"CheckWindow":         "read's window",
		"columnOutOfRange":    "read's column",
		"ParseSearchQuery":    "search's query",
		"SearchLimit":         "search's limit",
		"CheckRevision":       "versions' revision",
		"CheckVersionsArgs":   "versions' two calls",
		"CheckMessage":        "batch's message, of no op: CheckBatch checks it as message",
		// The batch's own refusals, which only a batch's op answers, at
		// ops[i] by construction: TestCheckBatch and TestPlanBatch check
		// their fields.
		"CheckBatch":                        "batch's ops, too_short at ops",
		"checkOp":                           "batch's ops[i].op",
		"checkMembers":                      "batch's ops[i] members a tool requires or takes not",
		"(*treePass).content":               "batch's ops[i].base_version for a page the batch created",
		"(*BatchPlan).deletesWhatItChanged": "batch's ops[i], B1 and B3",
	}
}

// constructors are the functions of page/domain that build a 422 and
// decide nothing: each is counted by the rules that call it, whose rows
// are its rows.
func constructors() map[string]string {
	return map[string]string{
		"NotAllowed":          "CreationOf, MoveKinds, ParentChainOf, PagePath, CheckEdit, Replace and the others",
		"invalidPath":         "ParsePath",
		"notInPage":           "Replace",
		"(*Recursive).Unwrap": "DeletionOf",
		"NotAVersion":         "ParseVersionToken",
	}
}

// refusing are the functions of the package's non-test files in dir that
// answer a 422 of their own: those that call NotAllowed, shared.Invalid or
// invalidPath, or write a *Recursive or a *PathNotFound with a ParentOf,
// which carry a field (a page.not_found without one names a path, no
// field); a method named "(*T).M" or "T.M" by its receiver.
func refusing(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	out := map[string]string{} // function: its file
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, dir+"/"+e.Name(), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			found := false
			ast.Inspect(fd, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					switch fn := x.Fun.(type) {
					case *ast.Ident:
						found = found || fn.Name == "NotAllowed" || fn.Name == "invalidPath"
					case *ast.SelectorExpr:
						id, ok := fn.X.(*ast.Ident)
						found = found || ok && id.Name == "shared" && fn.Sel.Name == "Invalid"
					}
				case *ast.CompositeLit:
					id, ok := x.Type.(*ast.Ident)
					found = found || ok && (id.Name == "Recursive" || id.Name == "PathNotFound" && keyed(x, "ParentOf"))
				}
				return true
			})
			if found {
				out[methodName(fd)] = e.Name()
			}
		}
	}
	return out
}

// keyed reports whether the composite literal lit sets the field key.
func keyed(lit *ast.CompositeLit, key string) bool {
	return slices.ContainsFunc(lit.Elts, func(e ast.Expr) bool {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			return false
		}
		id, ok := kv.Key.(*ast.Ident)
		return ok && id.Name == key
	})
}

// methodName is fd's name: a method's with its receiver, "(*T).M" or "T.M".
func methodName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	switch r := fd.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := r.X.(*ast.Ident); ok {
			return "(*" + id.Name + ")." + fd.Name.Name
		}
	case *ast.Ident:
		return r.Name + "." + fd.Name.Name
	}
	return fd.Name.Name
}

// Each rule of a step names the fields it is given (M9/P3 design 3.2): at a
// batch's op 7 every field of each refusal starts with "ops[7]."; with the
// zero Args, a tool's own call, the fields are today's. And the table is
// whole: every function of page/domain's non-test files that answers a 422
// of its own is a row's rule, or listed with why as no rule of an op or as
// a constructor its rules call; each listed one is still there.
func TestEveryRuleNamesTheFieldsItIsGiven(t *testing.T) {
	rules := fieldRules(t)
	for i, r := range rules {
		at := r.refuse(domain.OpArgs(7))
		own := r.refuse(domain.Args{})
		switch fields := namedFields(at); {
		case at == nil || own == nil:
			t.Errorf("row %d, %s: refuses %v at an op and %v alone; want a refusal both ways", i, r.rule, at, own)
		case len(fields) == 0 || slices.ContainsFunc(fields, func(f string) bool { return !strings.HasPrefix(f, "ops[7].") }):
			t.Errorf("row %d, %s: names %q at op 7, want each field at ops[7].", i, r.rule, fields)
		case !slices.Equal(namedFields(own), r.fields):
			t.Errorf("row %d, %s: names %q in a tool's own call, want %q", i, r.rule, namedFields(own), r.fields)
		}
	}
	found := refusing(t, ".")
	for _, f := range []string{"CreationOf", "CheckEdit", "(*Recursive).Unwrap"} { // a wrong directory must not pass
		if found[f] == "" {
			t.Fatalf("%s is not found: the check reads %d functions", f, len(found))
		}
	}
	rowed := map[string]bool{}
	for _, r := range rules {
		rowed[r.rule] = true
	}
	skip, built := notCounted(), constructors()
	for _, fn := range slices.Sorted(maps.Keys(found)) {
		if !rowed[fn] && skip[fn] == "" && built[fn] == "" {
			t.Errorf("%s in %s answers a 422 that no row refuses: add its refusals to fieldRules, naming its fields through "+
				"the Args it is given, or list it with why in notCounted or constructors", fn, found[fn])
		}
	}
	for _, listed := range []map[string]string{skip, built} {
		for _, fn := range slices.Sorted(maps.Keys(listed)) {
			if found[fn] == "" {
				t.Errorf("%s is listed, but no function of that name answers a 422 any more", fn)
			}
		}
	}
}
