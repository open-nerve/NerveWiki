package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

// runtimeTypes is the package whose UUID type oapi-codegen uses for format:
// uuid when a module's type-mapping lacks the standard library's uuid.
const runtimeTypes = "github.com/oapi-codegen/runtime/types"

// No file that oapi-codegen generates, for the shared components or for a
// module's HTTP adapter, uses the runtime's UUID type: the program has one
// uuid type, and a module whose type-mapping lacks it fails here (M0/P4
// design 3.3). The binary ban on google/uuid guards the same today, but the
// first operation with parameters brings in oapi-codegen/runtime, which
// links google/uuid, and that ban then needs an exception for it (M1).
func TestGeneratedCodeUsesTheStandardUUID(t *testing.T) {
	registerSources(t)
	var files []string
	for _, pattern := range []string{"internal/platform/httpserver/apigen/*.gen.go", "internal/modules/*/adapter/http/gen/*.gen.go"} {
		matches, err := filepath.Glob(filepath.Join(moduleRoot, pattern))
		// A wrong pattern must not pass as "nothing uses it".
		if err != nil || len(matches) == 0 {
			t.Fatalf("glob %s = %q, %v; want generated files", pattern, matches, err)
		}
		files = append(files, matches...)
	}
	fset := token.NewFileSet()
	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, at := range runtimeUUIDUses(fset, f) {
			t.Errorf("%s uses %s.UUID; map format uuid to the standard library's uuid.UUID in the oapi-codegen config", at, runtimeTypes)
		}
	}
}

// runtimeUUIDUses returns the positions where f refers to UUID of
// runtimeTypes, under whatever name f imports it.
func runtimeUUIDUses(fset *token.FileSet, f *ast.File) []string {
	var names []string
	for _, imp := range f.Imports {
		if path, err := strconv.Unquote(imp.Path.Value); err != nil || path != runtimeTypes {
			continue
		}
		name := "types"
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names = append(names, name)
	}
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "UUID" {
			if id, ok := sel.X.(*ast.Ident); ok && slices.Contains(names, id.Name) {
				found = append(found, fset.Position(sel.Pos()).String())
			}
		}
		return true
	})
	return found
}

func TestRuntimeUUIDUses(t *testing.T) {
	tests := []struct {
		name, src string
		want      []string
	}{
		{"the generated name", `package gen
import openapi_types "github.com/oapi-codegen/runtime/types"
type Thing struct{ ID openapi_types.UUID }`, []string{"gen.go:3:23"}},
		{"another name", `package gen
import rt "github.com/oapi-codegen/runtime/types"
func f(id *rt.UUID) {}`, []string{"gen.go:3:12"}},
		{"the package's own name", `package gen
import "github.com/oapi-codegen/runtime/types"
var ids []types.UUID`, []string{"gen.go:3:11"}},
		{"another type of the runtime", `package gen
import openapi_types "github.com/oapi-codegen/runtime/types"
var day openapi_types.Date`, nil},
		{"the standard library's uuid", `package gen
import "uuid"
var id uuid.UUID`, nil},
		{"another package named types", `package gen
import "example.com/types"
var id types.UUID`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "gen.go", tt.src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if got := runtimeUUIDUses(fset, f); !slices.Equal(got, tt.want) {
				t.Errorf("runtimeUUIDUses() = %q, want %q", got, tt.want)
			}
		})
	}
}
