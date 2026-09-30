package archtest

import (
	"io/fs"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/tools/go/packages"
)

// moduleRoot is server/, relative to this package: go test runs a test in its
// package directory.
const moduleRoot = "../.."

// registerSources makes the module's source tree an input of the calling
// test. go list runs in a subprocess, so the go test result cache would not
// see edited or new source files and could replay a stale pass. Walking the
// tree makes every directory listing (with file sizes and times) an input.
func registerSources(t *testing.T) {
	t.Helper()
	if err := filepath.WalkDir(moduleRoot, func(_ string, _ fs.DirEntry, err error) error { return err }); err != nil {
		t.Fatalf("walk %s: %v", moduleRoot, err)
	}
}

// loadGraph reads the import graph of every non-test package of the module.
func loadGraph(t *testing.T) graph {
	t.Helper()
	registerSources(t)
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedImports, Dir: moduleRoot}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("load packages: %v", err)
	}
	g := graph{}
	for _, p := range pkgs {
		for _, e := range p.Errors {
			t.Errorf("load %s: %v", p.PkgPath, e)
		}
		imports := make([]string, 0, len(p.Imports))
		for path := range p.Imports {
			imports = append(imports, path)
		}
		slices.Sort(imports)
		g[p.PkgPath] = imports
	}
	return g
}

func TestRepositoryFollowsArchitectureRules(t *testing.T) {
	g := loadGraph(t)
	// A loader problem must not pass as "no violations".
	for _, want := range []string{"cmd/nervewiki", "internal/bootstrap", "internal/platform/config"} {
		if _, ok := g[m(want)]; !ok {
			t.Fatalf("import graph lacks %s; loaded %d packages", want, len(g))
		}
	}
	for _, v := range check(g) {
		t.Error(v)
	}
}
