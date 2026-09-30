package archtest

import (
	"slices"
	"strings"
	"testing"
)

// bannedFromBinary lists the import path prefixes that must not reach the
// nervewiki binary: the test-only kin-openapi (apitest), testcontainers and
// docker (pgtest), and google/uuid, which the standard library's uuid
// replaces. Rule 8 keeps the test helpers themselves out, but any other
// import could still pull these in, and depguard sees direct imports only.
func bannedFromBinary() []string {
	return []string{
		"github.com/getkin/kin-openapi",
		"github.com/testcontainers/",
		"github.com/google/uuid",
		"github.com/docker/",
	}
}

// oapiRuntime is the module that generated code imports to bind path
// parameters. It imports google/uuid itself (runtime/types/uuid.go and the
// runtime package's styleparam.go, v1.7.0), so its packages, and only they,
// may import google/uuid (M1/P3 design 3.8). That generated code uses the
// standard library's uuid is checked directly, in generated_test.go.
const oapiRuntime = "github.com/oapi-codegen/runtime"

// isBannedFromBinary judges the import of path by importer.
func isBannedFromBinary(importer, path string) bool {
	if strings.HasPrefix(path, "github.com/google/uuid") && (importer == oapiRuntime || strings.HasPrefix(importer, oapiRuntime+"/")) {
		return false
	}
	return slices.ContainsFunc(bannedFromBinary(), func(prefix string) bool { return strings.HasPrefix(path, prefix) })
}

func TestBinaryLinksNoBannedModule(t *testing.T) {
	root := m("cmd/nervewiki")
	g := loadDeps(t, "./cmd/nervewiki")
	// A loader problem must not pass as "nothing banned".
	for _, want := range []string{root, m("internal/bootstrap"), "net/http"} {
		if _, ok := g[want]; !ok {
			t.Fatalf("dependency graph lacks %s; loaded %d packages", want, len(g))
		}
	}
	for _, b := range bannedImports(g, root, isBannedFromBinary) {
		t.Errorf("the nervewiki binary must not link %s, imported via %s", b.banned(), b.via())
	}
}

// Each banned import is reported with the shortest chain to it; the test
// helpers stay out of the walk unless production code imports them.
// google/uuid is reached first through oapi-codegen/runtime, which may
// import it; every other import of it is still reported, each on its own,
// runtime-extra's too: the exception is the runtime module, not a prefix.
func TestBannedImports(t *testing.T) {
	root := m("cmd/nervewiki")
	gen := m("internal/modules/identity/adapter/http/gen")
	g := graph{
		root:                                        {"github.com/spf13/cobra", m("internal/bootstrap")},
		"github.com/spf13/cobra":                    {"github.com/spf13/pflag"},
		m("internal/bootstrap"):                     {gen, m("internal/platform/httpserver"), m("internal/platform/postgres")},
		gen:                                         {"github.com/oapi-codegen/runtime", "github.com/oapi-codegen/runtime-extra", "net/http"},
		"github.com/oapi-codegen/runtime":           {"github.com/google/uuid", "github.com/oapi-codegen/runtime/types"},
		"github.com/oapi-codegen/runtime/types":     {"github.com/google/uuid"},
		"github.com/oapi-codegen/runtime-extra":     {"github.com/google/uuid"},
		m("internal/platform/httpserver"):           {"github.com/getkin/kin-openapi/openapi3", m("internal/platform/httpserver/bodyshape"), "net/http"},
		"github.com/getkin/kin-openapi/openapi3":    {"github.com/google/uuid"},
		m("internal/platform/httpserver/bodyshape"): {"github.com/google/uuid"},
		m("internal/platform/postgres"):             {"github.com/jackc/pgx/v5", m("internal/platform/postgres/pgtest")},
		m("internal/platform/postgres/pgtest"):      {"github.com/testcontainers/testcontainers-go"},
		// Not reachable from the root.
		m("internal/archtest"): {"github.com/docker/docker/client"},
	}
	var got []string
	for _, b := range bannedImports(g, root, isBannedFromBinary) {
		got = append(got, b.via())
	}
	want := []string{
		"cmd/nervewiki → internal/bootstrap → internal/platform/httpserver → github.com/getkin/kin-openapi/openapi3",
		"cmd/nervewiki → internal/bootstrap → internal/modules/identity/adapter/http/gen → github.com/oapi-codegen/runtime-extra → github.com/google/uuid",
		"cmd/nervewiki → internal/bootstrap → internal/platform/httpserver → internal/platform/httpserver/bodyshape → github.com/google/uuid",
		"cmd/nervewiki → internal/bootstrap → internal/platform/postgres → internal/platform/postgres/pgtest → github.com/testcontainers/testcontainers-go",
	}
	if !slices.Equal(got, want) {
		t.Errorf("bannedImports() =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
