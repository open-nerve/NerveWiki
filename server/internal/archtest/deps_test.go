package archtest

import (
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// loadDeps reads the import graph of the packages matching pattern and every
// package they depend on, test files excluded. Edges point at the resolved
// package paths, the same keys as the nodes: an import as written can differ,
// e.g. the standard library's golang.org/x/net/... is the package
// vendor/golang.org/x/net/..., which belongs to the standard library.
func loadDeps(t *testing.T, pattern string) graph {
	t.Helper()
	registerSources(t)
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedImports | packages.NeedDeps, Dir: moduleRoot}
	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		t.Fatalf("load packages: %v", err)
	}
	g := graph{}
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			t.Errorf("load %s: %v", p.PkgPath, e)
		}
		deps := make([]string, 0, len(p.Imports))
		for _, imp := range p.Imports {
			deps = append(deps, imp.PkgPath)
		}
		slices.Sort(deps)
		g[p.PkgPath] = deps
	})
	return g
}

// bannedImport is a banned package and the import chain that reaches it.
type bannedImport struct {
	chain []string // from the root to the banned package
}

func (b bannedImport) banned() string {
	return b.chain[len(b.chain)-1]
}

// via renders the chain as "a → b → c", this module's paths shortened.
func (b bannedImport) via() string {
	hops := make([]string, len(b.chain))
	for i, p := range b.chain {
		hops[i] = rel(p)
	}
	return strings.Join(hops, " → ")
}

// bannedImports walks g from root and returns every import of a banned
// package by a package that root reaches, each with the shortest chain to the
// importer. isBanned judges one import edge, so a package can be banned for
// some importers only. The imports of a banned package are not followed: it
// has to go anyway.
func bannedImports(g graph, root string, isBanned func(importer, path string) bool) []bannedImport {
	importer := map[string]string{root: ""}
	var found []bannedImport
	for queue := []string{root}; len(queue) > 0; queue = queue[1:] {
		pkg := queue[0]
		for _, dep := range g[pkg] {
			if isBanned(pkg, dep) {
				var chain []string
				for p := pkg; p != ""; p = importer[p] {
					chain = append(chain, p)
				}
				slices.Reverse(chain)
				found = append(found, bannedImport{append(chain, dep)})
				continue
			}
			if _, seen := importer[dep]; !seen {
				importer[dep] = pkg
				queue = append(queue, dep)
			}
		}
	}
	slices.SortStableFunc(found, func(a, b bannedImport) int {
		return strings.Compare(a.banned(), b.banned())
	})
	return found
}
