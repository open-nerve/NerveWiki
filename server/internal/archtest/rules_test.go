// Package archtest enforces the architecture rules (M0/P3 design 3.6, v0.1
// design 8.1) as tests. Each rule is a pure predicate over one import edge,
// so rules are unit-tested on synthetic edges and then applied to the real
// import graph of the module. Two more tests walk transitive dependencies:
// the nervewiki binary's, for modules it must not link, and those of domain,
// app and internal/shared, for infrastructure.
package archtest

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

const modulePath = "github.com/open-nerve/NerveWiki/server"

// graph maps each package of the module to the import paths it imports.
// Test files are not part of it.
type graph map[string][]string

type rule struct {
	name      string
	forbidden func(from, to string) bool
}

type violation struct {
	from, to, rule string
}

func (v violation) String() string {
	return fmt.Sprintf("%s imports %s: %s", rel(v.from), rel(v.to), v.rule)
}

func rules() []rule {
	return []rule{
		{"module layers point inward: adapter -> app -> domain", layersPointInward},
		{"domain and app import only the standard library (not net/http or database/sql), Unicode normalization and case folding, their own module's inner layers and internal/shared", innerLayersArePure},
		{"modules do not import each other", modulesAreIsolated},
		{"platform does not import modules, bootstrap or internal/shared", platformIsBusinessFree},
		{"only bootstrap imports modules", onlyBootstrapImportsModules},
		{"generated code is imported only by its own adapter", generatedCodeStaysInAdapter},
		{"platform packages do not import each other, except config", platformPackagesAreIndependent},
		{"test helpers (pgtest, apitest, httpservertest, clocktest, markdowntest) are imported only by tests", testHelpersOnlyInTests},
		{"module packages live in domain, app or adapter, or at the module root", moduleLayoutIsKnown},
		{"internal/shared imports only the standard library (not net/http or database/sql), Unicode normalization and case folding, and internal/shared", sharedKernelIsPure},
		{"River is imported only by platform/jobs and a module's adapter/river", riverStaysInJobs},
		{"goldmark, golang.org/x/net/html and go.yaml.in/yaml are imported only by platform/markdown", markdownLibrariesStayInMarkdown},
		{"bootstrap imports only a module's root", bootstrapImportsModuleRoots},
		{"a module's adapters do not import each other", adaptersAreIndependent},
	}
}

// check applies every rule to every edge of g, in a stable order.
func check(g graph) []violation {
	var found []violation
	for _, from := range slices.Sorted(maps.Keys(g)) {
		for _, to := range g[from] {
			for _, r := range rules() {
				if r.forbidden(from, to) {
					found = append(found, violation{from, to, r.name})
				}
			}
		}
	}
	return found
}

// local returns the module-relative path of a package of this module.
func local(path string) (string, bool) {
	return strings.CutPrefix(path, modulePath+"/")
}

func rel(path string) string {
	if r, ok := local(path); ok {
		return r
	}
	return path
}

// within reports whether path is dir or inside it.
func within(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, dir+"/")
}

// moduleOf splits internal/modules/<name>/<layer>/... of this module.
func moduleOf(path string) (name, layer string, ok bool) {
	r, ok := local(path)
	if !ok {
		return "", "", false
	}
	rest, ok := strings.CutPrefix(r, "internal/modules/")
	if !ok {
		return "", "", false
	}
	name, sub, _ := strings.Cut(rest, "/")
	layer, _, _ = strings.Cut(sub, "/")
	return name, layer, true
}

// platformOf returns the platform package a path belongs to, e.g. "postgres"
// for internal/platform/postgres/pgtest.
func platformOf(path string) (string, bool) {
	r, ok := local(path)
	if !ok {
		return "", false
	}
	rest, ok := strings.CutPrefix(r, "internal/platform/")
	if !ok {
		return "", false
	}
	name, _, _ := strings.Cut(rest, "/")
	return name, true
}

func isStdlib(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

// isPureLibrary reports the third-party packages the pure layers may
// import: Unicode normalization (and what it imports), which the password
// rules need to judge the form the hasher hashes (identity's
// CanonicalPassword), and case folding, which a page title's key needs
// (shared.TitleKey, M4/P1 design 3.2).
func isPureLibrary(path string) bool {
	switch path {
	case "golang.org/x/text/unicode/norm", "golang.org/x/text/transform", "golang.org/x/text/cases":
		return true
	}
	return false
}

// isPureLibraryDependency reports what the pure libraries may bring along:
// they and what they import, case folding golang.org/x/text/language and
// its internal packages, all under golang.org/x/text/, which the pure
// layers may reach but not import (isPureLibrary).
func isPureLibraryDependency(path string) bool {
	return strings.HasPrefix(path, "golang.org/x/text/")
}

// isInfrastructure reports whether an import outside this module is
// technology the pure layers must not see: any third-party module but the
// pure libraries, net/http or database/sql.
func isInfrastructure(path string) bool {
	return !isStdlib(path) && !isPureLibrary(path) || within(path, "net/http") || within(path, "database/sql")
}

func inModuleDir(path, dir string) bool {
	r, ok := local(path)
	return ok && within(r, dir)
}

// layerRank orders the layers of a module from the inside out; "" is the
// module root (module.go).
func layerRank(layer string) (int, bool) {
	switch layer {
	case "domain":
		return 0, true
	case "app":
		return 1, true
	case "adapter":
		return 2, true
	case "":
		return 3, true
	}
	return 0, false
}

func layersPointInward(from, to string) bool {
	fm, fl, ok := moduleOf(from)
	if !ok {
		return false
	}
	tm, tl, ok := moduleOf(to)
	if !ok || fm != tm {
		return false
	}
	fromRank, fromKnown := layerRank(fl)
	toRank, toKnown := layerRank(tl)
	return fromKnown && toKnown && toRank > fromRank
}

// moduleLayoutIsKnown keeps every module package in a layer the other rules
// know. A package elsewhere in a module would escape them: the layer rule
// cannot rank it and the purity rule leaves in-module imports to the layer
// rule, so domain -> page/transport -> net/http would pass.
func moduleLayoutIsKnown(from, to string) bool {
	return inUnknownLayer(from) || inUnknownLayer(to)
}

func inUnknownLayer(path string) bool {
	if _, layer, ok := moduleOf(path); ok {
		_, known := layerRank(layer)
		return !known
	}
	return false
}

// innerLayersArePure keeps domain and app free of infrastructure: no
// third-party module, no platform package, no net/http or database/sql.
// Imports of modules are judged by the layer, layout and isolation rules,
// which together restrict them to the own module's same or inner layers.
func innerLayersArePure(from, to string) bool {
	if _, layer, ok := moduleOf(from); !ok || (layer != "domain" && layer != "app") {
		return false
	}
	if r, ok := local(to); ok {
		_, _, inModule := moduleOf(to)
		return !inModule && !within(r, "internal/shared")
	}
	return isInfrastructure(to)
}

// sharedKernelIsPure holds internal/shared to the purity of the domain and
// app layers that may import it; otherwise it would carry infrastructure
// into them, as in domain -> shared/x -> net/http.
func sharedKernelIsPure(from, to string) bool {
	if !inModuleDir(from, "internal/shared") {
		return false
	}
	if r, ok := local(to); ok {
		return !within(r, "internal/shared")
	}
	return isInfrastructure(to)
}

func modulesAreIsolated(from, to string) bool {
	fm, _, ok := moduleOf(from)
	if !ok {
		return false
	}
	tm, _, ok := moduleOf(to)
	return ok && fm != tm
}

// platformIsBusinessFree keeps the platform free of business code. That
// includes internal/shared: the platform declares the small interfaces it
// needs (such as ProblemError) and shared's types satisfy them by structure.
func platformIsBusinessFree(from, to string) bool {
	return inModuleDir(from, "internal/platform") &&
		(inModuleDir(to, "internal/modules") || inModuleDir(to, "internal/bootstrap") || inModuleDir(to, "internal/shared"))
}

func onlyBootstrapImportsModules(from, to string) bool {
	return inModuleDir(to, "internal/modules") &&
		!inModuleDir(from, "internal/modules") && !inModuleDir(from, "internal/bootstrap")
}

// bootstrapImportsModuleRoots keeps the composition root at the modules'
// entry points: what it needs of a module, the module's root offers
// (v0.1 design 13.1 rule 11), and the module's layers stay its own.
func bootstrapImportsModuleRoots(from, to string) bool {
	_, layer, ok := moduleOf(to)
	return ok && layer != "" && inModuleDir(from, "internal/bootstrap")
}

// adaptersAreIndependent keeps the adapters of a module apart: they meet
// in the app layer's ports, wired at the module root, so that one adapter's
// technology does not leak into another. An adapter may import its own
// subpackages, such as its generated code.
func adaptersAreIndependent(from, to string) bool {
	fm, fa, ok := adapterOf(from)
	if !ok {
		return false
	}
	tm, ta, ok := adapterOf(to)
	return ok && fm == tm && fa != ta
}

// adapterOf splits internal/modules/<module>/adapter/<adapter>/... of this
// module.
func adapterOf(path string) (module, adapter string, ok bool) {
	module, layer, ok := moduleOf(path)
	if !ok || layer != "adapter" {
		return "", "", false
	}
	r, _ := local(path)
	parts := strings.Split(r, "/")
	if len(parts) < 5 {
		return "", "", false
	}
	return module, parts[4], true
}

// generatedCodeStaysInAdapter lets only an adapter import its own generated
// code: modules/<m>/adapter/<a>/gen (oapi-codegen for http, sqlc for
// postgres) is imported by modules/<m>/adapter/<a> and its own subpackages.
func generatedCodeStaysInAdapter(from, to string) bool {
	adapter, ok := generatedCodeOwner(to)
	if !ok {
		return false
	}
	r, _ := local(from)
	return r != adapter && !within(r, adapter+"/gen")
}

// generatedCodeOwner returns the adapter that owns path when path is inside
// internal/modules/<m>/adapter/<a>/gen.
func generatedCodeOwner(path string) (string, bool) {
	r, ok := local(path)
	if !ok {
		return "", false
	}
	parts := strings.Split(r, "/")
	// internal/modules/<m>/adapter/<a>/gen[/...]
	if len(parts) < 6 || parts[0] != "internal" || parts[1] != "modules" || parts[3] != "adapter" || parts[5] != "gen" {
		return "", false
	}
	return strings.Join(parts[:5], "/"), true
}

func platformPackagesAreIndependent(from, to string) bool {
	fp, ok := platformOf(from)
	if !ok {
		return false
	}
	tp, ok := platformOf(to)
	return ok && tp != fp && tp != "config"
}

// riverStaysInJobs keeps the job queue behind two doors (M1/P4 design 3.2):
// platform/jobs runs River, and a module's adapter/river turns its use cases
// into River's workers and periodic jobs. The rest of a module sees jobs
// through them, as it sees SQL through adapter/postgres.
func riverStaysInJobs(from, to string) bool {
	if !within(to, "github.com/riverqueue/river") || inModuleDir(from, "internal/platform/jobs") {
		return false
	}
	r, _ := local(from)
	parts := strings.Split(r, "/")
	// internal/modules/<m>/adapter/river[/...]
	return len(parts) < 5 || parts[0] != "internal" || parts[1] != "modules" || parts[3] != "adapter" || parts[4] != "river"
}

// markdownLibrariesStayInMarkdown keeps the Markdown libraries behind
// platform/markdown (M4/P3 design 3.11): the one parse and its rendering,
// hardened, and the one reading of a frontmatter. platform/config reads YAML
// through koanf, which is no direct import. M6's dialect is within it
// (platform/markdown/obsidian), so the rule held.
func markdownLibrariesStayInMarkdown(from, to string) bool {
	library := within(to, "github.com/yuin/goldmark") || within(to, "golang.org/x/net/html") ||
		strings.HasPrefix(to, "go.yaml.in/yaml/")
	return library && !inModuleDir(from, "internal/platform/markdown")
}

func testHelpersOnlyInTests(_, to string) bool {
	// The graph holds no test files, so any importer is production code.
	return inModuleDir(to, "internal/platform/postgres/pgtest") ||
		inModuleDir(to, "internal/platform/httpserver/apitest") ||
		inModuleDir(to, "internal/platform/httpserver/httpservertest") ||
		inModuleDir(to, "internal/platform/clock/clocktest") ||
		inModuleDir(to, "internal/platform/markdown/markdowntest")
}
