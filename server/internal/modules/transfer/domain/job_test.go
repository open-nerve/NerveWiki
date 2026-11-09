package domain_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// Kinds names every kind of job job.go declares, each once: the rescue
// goes over Kinds (M11/P1 design 3.19), and a kind left out of it would be
// a queued job River dropped that stays queued, its notebook's writes
// held by it for good. Writes is the writing key's: the import, the
// restore, the revert and the rollback.
func TestKindsAreEveryKind(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "job.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var declared []domain.Kind
	for _, d := range file.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, spec := range g.Specs {
			v := spec.(*ast.ValueSpec)
			if id, ok := v.Type.(*ast.Ident); !ok || id.Name != "Kind" {
				continue
			}
			for _, value := range v.Values {
				lit := value.(*ast.BasicLit)
				declared = append(declared, domain.Kind(lit.Value[1:len(lit.Value)-1]))
			}
		}
	}
	kinds := domain.Kinds()
	if len(declared) != 5 || len(kinds) != len(declared) {
		t.Fatalf("job.go declares %v, Kinds() = %v; want the five, each once", declared, kinds)
	}
	for _, k := range declared {
		if !slices.Contains(kinds, k) {
			t.Errorf("Kinds() = %v lacks %q", kinds, k)
		}
	}
	var writing []domain.Kind
	for _, k := range kinds {
		if k.Writes() {
			writing = append(writing, k)
		}
	}
	if !slices.Equal(writing, []domain.Kind{domain.KindImport, domain.KindTrashRestore, domain.KindRevert, domain.KindRollback}) {
		t.Errorf("the kinds that write = %v, want the import, the restore, the revert and the rollback", writing)
	}
}
