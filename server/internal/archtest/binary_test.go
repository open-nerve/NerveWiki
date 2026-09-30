package archtest

import (
	"slices"
	"strings"
	"testing"
)

// bannedFromBinary lists the import path prefixes that must not reach the
// nervewiki binary: testcontainers and docker (pgtest), and google/uuid,
// which the standard library's uuid replaces. Rule 8 keeps the test helpers
// themselves out, but any other import could still pull these in, and
// depguard sees direct imports only.
func bannedFromBinary() []string {
	return []string{
		"github.com/testcontainers/",
		"github.com/google/uuid",
		"github.com/docker/",
	}
}

func isBannedFromBinary(_, path string) bool {
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
func TestBannedImports(t *testing.T) {
	root := m("cmd/nervewiki")
	g := graph{
		root:                                   {"github.com/spf13/cobra", m("internal/bootstrap")},
		"github.com/spf13/cobra":               {"github.com/spf13/pflag"},
		m("internal/bootstrap"):                {m("internal/platform/httpserver"), m("internal/platform/postgres")},
		m("internal/platform/httpserver"):      {"github.com/google/uuid", "net/http"},
		m("internal/platform/postgres"):        {"github.com/jackc/pgx/v5", m("internal/platform/postgres/pgtest")},
		m("internal/platform/postgres/pgtest"): {"github.com/testcontainers/testcontainers-go"},
		// Not reachable from the root.
		m("internal/archtest"): {"github.com/docker/docker/client"},
	}
	var got []string
	for _, b := range bannedImports(g, root, isBannedFromBinary) {
		got = append(got, b.via())
	}
	want := []string{
		"cmd/nervewiki → internal/bootstrap → internal/platform/httpserver → github.com/google/uuid",
		"cmd/nervewiki → internal/bootstrap → internal/platform/postgres → internal/platform/postgres/pgtest → github.com/testcontainers/testcontainers-go",
	}
	if !slices.Equal(got, want) {
		t.Errorf("bannedImports() =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
