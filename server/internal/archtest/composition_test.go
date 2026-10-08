package archtest

import (
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// The command line's compositions, bootstrap.Users, bootstrap.Workspaces,
// bootstrap.Reindex, the migrations' and those to come, are a pool and the modules'
// administrator use cases (M1/P4 design 3.8): nothing they call builds a
// module's HTTP side (a module's New), the HTTP server, a rate limiter, a
// jobs client or the store of files, which only serve opens (M7/P1 design
// 3.5). The rule follows the static calls from each; the commands are
// func values it calls dynamically, so they are not followed: they only
// receive the composition. Reaching the module's NewAdmin shows the walk
// sees the composition at all. The registrants come from one place for serve
// and the command line alike (design 3.6; v0.1 design 13.1, item 21): serve
// and Users reach the deactivation's, and through them the workspace
// module's (M2/P2 review, Q2); Workspaces reaches the workspace module's.
// serve reaches the Markdown's extensions too (M4/P3 design 3.11), and so
// does Reindex, whose parse must be serve's (M6/P3 design 3.6).
func TestCommandsComposeNoServerAndNoJobs(t *testing.T) {
	registerSources(t)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedDeps |
			packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedTypesSizes,
		Dir: moduleRoot,
	}
	pkgs, err := packages.Load(cfg, "./internal/bootstrap")
	if err != nil {
		t.Fatalf("load packages: %v", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		t.Fatal("the packages have errors")
	}
	prog, built := ssautil.AllPackages(pkgs, 0)
	prog.Build()
	graph := static.CallGraph(prog)
	bootstrap := built[0]
	registrants := []string{m("internal/bootstrap") + ".deactivationRegistrants", m("internal/bootstrap") + ".workspaceRegistrants"}
	for _, c := range []struct {
		root  string
		reach []string
	}{
		{"Users", append([]string{m("internal/modules/identity") + ".NewAdmin"}, registrants...)},
		{"Workspaces", []string{m("internal/modules/workspace") + ".NewAdmin", m("internal/bootstrap") + ".workspaceRegistrants"}},
		{"Reindex", []string{m("internal/modules/linking") + ".NewAdmin", m("internal/bootstrap") + ".markdownExtensions"}},
		{"MigrateUp", []string{m("internal/platform/postgres") + ".NewMigrator"}},
		{"MigrateDown", []string{m("internal/platform/postgres") + ".NewMigrator"}},
		{"MigrateStatus", []string{m("internal/platform/postgres") + ".NewMigrator"}},
	} {
		root := bootstrap.Func(c.root)
		if root == nil {
			t.Fatalf("bootstrap.%s not found: the rule checks nothing", c.root)
		}
		reached, banned := walkCalls(graph, root, composesMore)
		assertReaches(t, "bootstrap."+c.root, reached, c.reach...)
		for _, chain := range banned {
			names := make([]string, len(chain))
			for i, f := range chain {
				names[i] = funcName(f)
			}
			t.Errorf("bootstrap.%s builds more than the pool and the administrator use cases: %s", c.root, strings.Join(names, " → "))
		}
	}

	serve := bootstrap.Func("newApp")
	if serve == nil {
		t.Fatal("bootstrap.newApp not found")
	}
	reached, _ := walkCalls(graph, serve, func(*ssa.Function) bool { return false })
	assertReaches(t, "bootstrap.newApp", reached,
		append(registrants, m("internal/bootstrap")+".markdownExtensions", m("internal/platform/storage")+".OpenLocal")...)
}

// assertReaches fails unless reached holds a chain to each of want. Not
// reaching the administrator use cases means the rule checks nothing.
func assertReaches(t *testing.T, root string, reached [][]*ssa.Function, want ...string) {
	t.Helper()
	for _, w := range want {
		if !slices.ContainsFunc(reached, func(chain []*ssa.Function) bool { return chain[len(chain)-1].String() == w }) {
			var names []string
			for _, chain := range reached {
				names = append(names, funcName(chain[len(chain)-1]))
			}
			t.Errorf("%s does not reach %s; it reaches:\n%s", root, w, strings.Join(names, "\n"))
		}
	}
}

// composesMore reports whether f builds what the command line must not: a
// module's HTTP side (New in a module's root package: identity.New,
// instance.New and those to come), the HTTP server (platform/httpserver), a
// rate limiter, a jobs client (platform/jobs, River) or the file store
// (platform/storage).
func composesMore(f *ssa.Function) bool {
	if f.Pkg == nil {
		return false
	}
	path := f.Pkg.Pkg.Path()
	if module, ok := strings.CutPrefix(path, m("internal/modules")+"/"); ok && !strings.Contains(module, "/") && f.Name() == "New" {
		return true
	}
	return slices.ContainsFunc([]string{
		m("internal/platform/httpserver"), m("internal/platform/ratelimit"), m("internal/platform/jobs"), "github.com/riverqueue/river",
		m("internal/platform/storage"),
	}, func(dir string) bool { return within(path, dir) })
}

// walkCalls follows the static calls from root, breadth first. It returns
// the call chain to every function reached, and the chain to each call of a
// function stop accepts, whose own calls it does not follow.
func walkCalls(graph *callgraph.Graph, root *ssa.Function, stop func(*ssa.Function) bool) (reached, stopped [][]*ssa.Function) {
	start := graph.Nodes[root]
	chains := map[*callgraph.Node][]*ssa.Function{start: {root}}
	for queue := []*callgraph.Node{start}; len(queue) > 0; queue = queue[1:] {
		caller := queue[0]
		for _, e := range caller.Out {
			chain := append(slices.Clip(chains[caller]), e.Callee.Func)
			if stop(e.Callee.Func) {
				stopped = append(stopped, chain)
				continue
			}
			if _, seen := chains[e.Callee]; !seen {
				chains[e.Callee] = chain
				reached = append(reached, chain)
				queue = append(queue, e.Callee)
			}
		}
	}
	return reached, stopped
}

// funcName is f's full name, this module's paths shortened.
func funcName(f *ssa.Function) string {
	return strings.ReplaceAll(f.String(), modulePath+"/", "")
}
