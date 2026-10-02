package archtest

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

// reachesInfrastructure marks what the pure layers (domain, app,
// internal/shared) must not depend on, even indirectly: net/http,
// database/sql and any module other than this one, but what the pure
// libraries bring along.
func reachesInfrastructure(_, path string) bool {
	_, inModule := local(path)
	return !inModule && isInfrastructure(path) && !isPureLibraryDependency(path)
}

func isPure(path string) bool {
	_, layer, ok := moduleOf(path)
	return ok && (layer == "domain" || layer == "app") || inModuleDir(path, "internal/shared")
}

// Rules 2 and 10 judge direct imports only, and some standard library
// packages import net/http themselves (expvar, net/rpc): domain -> expvar
// passes both rules yet links net/http. This walks every dependency instead.
func TestPureLayersReachNoInfrastructure(t *testing.T) {
	g := loadDeps(t, "./...")
	pure := 0
	for _, pkg := range slices.Sorted(maps.Keys(g)) {
		if !isPure(pkg) {
			continue
		}
		pure++
		for _, b := range bannedImports(g, pkg, reachesInfrastructure) {
			t.Errorf("%s must not depend on %s, imported via %s", rel(pkg), b.banned(), b.via())
		}
	}
	// A loader problem must not pass as "nothing reached".
	if pure == 0 {
		t.Fatalf("dependency graph holds no domain, app or internal/shared package; loaded %d packages", len(g))
	}
}

// The standard library is pure as a whole, including the golang.org/x code it
// vendors (net, net/mail, crypto/x509, ...): the pure layers may use it, so
// the walk must not report it. A synthetic graph cannot show this; it takes
// the real loader.
func TestStandardLibraryReachesNoInfrastructure(t *testing.T) {
	for _, pkg := range []string{"net/mail", "crypto/x509"} {
		g := loadDeps(t, pkg)
		if _, ok := g[pkg]; !ok {
			t.Fatalf("dependency graph lacks %s; loaded %d packages", pkg, len(g))
		}
		for _, b := range bannedImports(g, pkg, reachesInfrastructure) {
			t.Errorf("%s reaches %s via %s, want the standard library to count as pure", pkg, b.banned(), b.via())
		}
	}
}

// Infrastructure counts however it is reached: through the standard library
// or through this module's own packages, which are not infrastructure
// themselves.
func TestReachesInfrastructure(t *testing.T) {
	domain := m("internal/modules/page/domain")
	g := graph{
		domain:                  {"expvar", "fmt", m("internal/shared/id")},
		"expvar":                {"net/http"},
		"fmt":                   {"strconv"},
		m("internal/shared/id"): {"github.com/jackc/pgx/v5/pgtype", "golang.org/x/text/cases"},
		// Case folding brings language along: reached, not imported.
		"golang.org/x/text/cases": {"golang.org/x/text/language", "golang.org/x/text/internal"},
	}
	var got []string
	for _, b := range bannedImports(g, domain, reachesInfrastructure) {
		got = append(got, b.via())
	}
	want := []string{
		"internal/modules/page/domain → internal/shared/id → github.com/jackc/pgx/v5/pgtype",
		"internal/modules/page/domain → expvar → net/http",
	}
	if !slices.Equal(got, want) {
		t.Errorf("bannedImports() =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
