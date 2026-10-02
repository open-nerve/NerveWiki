package archtest

import (
	"slices"
	"testing"
)

// m turns a module-relative path into a full import path.
func m(rel string) string { return modulePath + "/" + rel }

// TestRules proves every rule both fires and stays quiet, on synthetic edges:
// M0 has no module yet, so the real graph cannot exercise most rules.
func TestRules(t *testing.T) {
	const (
		inward   = "module layers point inward: adapter -> app -> domain"
		layout   = "module packages live in domain, app or adapter, or at the module root"
		pure     = "domain and app import only the standard library (not net/http or database/sql), Unicode normalization and case folding, their own module's inner layers and internal/shared"
		kernel   = "internal/shared imports only the standard library (not net/http or database/sql), Unicode normalization and case folding, and internal/shared"
		isolated = "modules do not import each other"
		business = "platform does not import modules, bootstrap or internal/shared"
		entry    = "only bootstrap imports modules"
		roots    = "bootstrap imports only a module's root"
		adapters = "a module's adapters do not import each other"
		gen      = "generated code is imported only by its own adapter"
		platform = "platform packages do not import each other, except config"
		testOnly = "test helpers (pgtest, apitest, httpservertest, clocktest) are imported only by tests"
		river    = "River is imported only by platform/jobs and a module's adapter/river"
	)
	tests := []struct {
		from, to string
		want     []string // violated rules; empty means allowed
	}{
		// Layers inside one module.
		{m("internal/modules/page/adapter/http"), m("internal/modules/page/app"), nil},
		{m("internal/modules/page/app"), m("internal/modules/page/domain"), nil},
		{m("internal/modules/page"), m("internal/modules/page/adapter/postgres"), nil},
		{m("internal/modules/page/domain"), m("internal/modules/page/app"), []string{inward}},
		{m("internal/modules/page/app"), m("internal/modules/page/adapter/http"), []string{inward}},
		{m("internal/modules/page/adapter/http"), m("internal/modules/page"), []string{inward}},
		{m("internal/modules/page/domain/state"), m("internal/modules/page/domain"), nil},

		// A package outside the known layers would escape the layer and purity
		// rules, e.g. domain -> page/transport -> net/http.
		{m("internal/modules/page/domain"), m("internal/modules/page/transport"), []string{layout}},
		{m("internal/modules/page/transport"), "net/http", []string{layout}},
		{m("internal/modules/page/adapter/http"), m("internal/modules/page/helper"), []string{layout}},

		// Purity of domain and app (app -> own domain is allowed, see above).
		{m("internal/modules/page/domain"), "time", nil},
		{m("internal/modules/page/domain"), m("internal/shared/id"), nil},
		{m("internal/modules/page/domain"), "net/http", []string{pure}},
		{m("internal/modules/page/domain"), "database/sql/driver", []string{pure}},
		{m("internal/modules/page/domain"), "github.com/jackc/pgx/v5", []string{pure}},
		{m("internal/modules/page/domain"), m("internal/platform/config"), []string{pure}},
		{m("internal/modules/page/domain"), "golang.org/x/text/unicode/norm", nil},
		{m("internal/modules/page/domain"), "golang.org/x/text/language", []string{pure}},
		{m("internal/modules/page/domain"), "golang.org/x/text/cases", nil},
		{m("internal/modules/page/app"), "context", nil},
		{m("internal/modules/page/app"), m("internal/shared/id"), nil},
		{m("internal/modules/page/app"), "net/http", []string{pure}},
		{m("internal/modules/page/app"), "github.com/jackc/pgx/v5", []string{pure}},
		{m("internal/modules/page/app"), m("internal/platform/postgres"), []string{pure}},

		// The shared kernel is as pure as the layers that import it, or it would
		// carry infrastructure into them: domain -> shared/x -> net/http.
		{m("internal/shared/id"), "time", nil},
		{m("internal/shared/tx"), m("internal/shared/id"), nil},
		{m("internal/shared/id"), "net/http", []string{kernel}},
		{m("internal/shared/id"), "github.com/jackc/pgx/v5", []string{kernel}},
		{m("internal/shared/id"), m("internal/platform/config"), []string{kernel}},
		{m("internal/shared/id"), "golang.org/x/text/cases", nil},
		{m("internal/shared/id"), "golang.org/x/text/language", []string{kernel}},
		{m("internal/shared/id"), m("internal/modules/page/domain"), []string{entry, kernel}},

		// The platform declares the interfaces shared satisfies; it never imports it.
		{m("internal/platform/postgres"), m("internal/shared"), []string{business}},
		{m("internal/platform/httpserver"), m("internal/shared"), []string{business}},

		// Module isolation and the composition root.
		{m("internal/modules/page/app"), m("internal/modules/notebook/domain"), []string{isolated}},
		{m("internal/bootstrap"), m("internal/modules/page"), nil},
		{m("cmd/nervewiki"), m("internal/modules/page"), []string{entry}},

		// The composition root reaches a module through its root alone, and a
		// module's adapters meet only in its app layer.
		{m("internal/bootstrap"), m("internal/modules/page"), nil},
		{m("internal/bootstrap"), m("internal/modules/page/adapter/postgres"), []string{roots}},
		{m("internal/bootstrap"), m("internal/modules/page/domain"), []string{roots}},
		{m("internal/modules/page/adapter/http"), m("internal/modules/page/adapter/postgres"), []string{adapters}},
		{m("internal/modules/page/adapter/postgres"), m("internal/modules/page/adapter/postgres/gen"), nil},
		{m("internal/modules/page/adapter/river"), m("internal/modules/page/app"), nil},
		{m("internal/platform/httpserver"), m("internal/modules/page"), []string{business, entry}},
		{m("internal/platform/httpserver"), m("internal/bootstrap"), []string{business}},

		// Generated code.
		{m("internal/modules/page/adapter/http"), m("internal/modules/page/adapter/http/gen"), nil},
		{m("internal/modules/page/adapter/http/gen"), m("internal/platform/httpserver/apigen"), nil},
		{m("internal/modules/page/app"), m("internal/modules/page/adapter/http/gen"), []string{inward, gen}},
		{m("internal/modules/page/adapter/postgres"), m("internal/modules/page/adapter/http/gen"), []string{gen, adapters}},
		{m("internal/modules/notebook/adapter/http"), m("internal/modules/page/adapter/http/gen"), []string{isolated, gen}},
		// sqlc's code under adapter/postgres/gen follows the same rule.
		{m("internal/modules/page/adapter/postgres"), m("internal/modules/page/adapter/postgres/gen"), nil},

		// River stays in platform/jobs and the modules' river adapters.
		{m("internal/platform/jobs"), "github.com/riverqueue/river", nil},
		{m("internal/platform/jobs"), "github.com/riverqueue/river/riverdriver/riverpgxv5", nil},
		{m("internal/modules/page/adapter/river"), "github.com/riverqueue/river", nil},
		{m("internal/modules/page/adapter/http"), "github.com/riverqueue/river", []string{river}},
		{m("internal/modules/page"), "github.com/riverqueue/river", []string{river}},
		{m("internal/modules/page/app"), "github.com/riverqueue/river", []string{pure, river}},
		{m("internal/bootstrap"), "github.com/riverqueue/river/rivertype", []string{river}},
		{m("internal/modules/page/adapter/postgres"), "github.com/riverqueue/riverfake", nil},
		{m("internal/modules/page/adapter/postgres/gen"), "github.com/jackc/pgx/v5", nil},
		{m("internal/modules/page/adapter/http"), m("internal/modules/page/adapter/postgres/gen"), []string{gen, adapters}},
		{m("internal/modules/page"), m("internal/modules/page/adapter/postgres/gen"), []string{gen}},

		// Platform packages.
		{m("internal/platform/logging"), m("internal/platform/config"), nil},
		{m("internal/platform/postgres/pgtest"), m("internal/platform/postgres"), nil},
		{m("internal/platform/httpserver"), m("internal/platform/postgres"), []string{platform}},

		// Test helpers.
		{m("internal/bootstrap"), m("internal/platform/postgres/pgtest"), []string{testOnly}},
		{m("internal/platform/httpserver"), m("internal/platform/postgres/pgtest"), []string{platform, testOnly}},
		{m("internal/modules/instance/adapter/http"), m("internal/platform/httpserver/apitest"), []string{testOnly}},
		{m("internal/platform/httpserver"), m("internal/platform/httpserver/apitest"), []string{testOnly}},
		{m("internal/modules/identity/adapter/http"), m("internal/platform/httpserver/httpservertest"), []string{testOnly}},
		{m("internal/bootstrap"), m("internal/platform/clock/clocktest"), []string{testOnly}},
		{m("internal/platform/clock/clocktest"), "time", nil},
	}
	fired := map[string]bool{}
	for _, tt := range tests {
		var got []string
		for _, v := range check(graph{tt.from: {tt.to}}) {
			got = append(got, v.rule)
			fired[v.rule] = true
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s -> %s: violated %q, want %q", rel(tt.from), rel(tt.to), got, tt.want)
		}
	}
	for _, r := range rules() {
		if !fired[r.name] {
			t.Errorf("rule %q has no violating case above", r.name)
		}
	}
}
