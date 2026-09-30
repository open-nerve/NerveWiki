package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
)

// The whole-program tests (M0/P4 design 3.6): the wired app against the
// contract, operation by operation, so an operation added later is covered
// without a new test.

// The routes registered under /api/, but for the platform's fallback, are the
// contract's operations: a route the contract does not describe, or an
// operation no module serves, fails here.
func TestAPIRoutesAreTheContractsOperations(t *testing.T) {
	contract := apitest.Load(t)
	a := buildApp(t, testConfig(t, unreachableDB, false), sampleMigrations())

	var got []string
	for _, pattern := range a.router.Patterns() {
		path := pattern
		if _, rest, ok := strings.Cut(pattern, " "); ok {
			path = rest
		}
		if strings.HasPrefix(path, "/api/") && pattern != "/api/" {
			got = append(got, pattern)
		}
	}
	slices.Sort(got)
	var want []string
	for _, op := range contract.Operations() {
		want = append(want, op.Pattern())
	}
	if len(want) == 0 || !slices.Equal(got, want) {
		t.Errorf("routes under /api/ = %q, want the contract's operations %q", got, want)
	}
}

// Every operation's default response, the problem, declares the headers a
// problem may carry: Retry-After (shared.RateLimited, shared.ServerBusy) and
// WWW-Authenticate (every 401). Each module declares its own Problem
// response, so a module that leaves one out fails here.
func TestEveryProblemResponseDeclaresItsHeaders(t *testing.T) {
	contract := apitest.Load(t)

	for _, op := range contract.Operations() {
		for _, header := range []string{"Retry-After", "WWW-Authenticate"} {
			if !slices.Contains(op.ProblemHeaders, header) {
				t.Errorf("%s: the default response declares %q, want %s among them", op.Pattern(), op.ProblemHeaders, header)
			}
		}
	}
}

// The operations the modules declare public are exactly the contract's
// operations with security: [] (M1/P1 design 3.9), as the wired app answers
// them without a token: every other operation answers 401 with the Bearer
// challenge before its handler runs, a public one never does. A module list
// that misses an operation, or names one the contract protects, fails here.
func TestPublicOperationsAreTheContractsPublicOperations(t *testing.T) {
	contract := apitest.Load(t)
	a := buildApp(t, testConfig(t, unreachableDB, false), sampleMigrations())

	for _, op := range contract.Operations() {
		req := httptest.NewRequest(op.Method, op.Path, nil)
		rec := httptest.NewRecorder()

		a.router.ServeHTTP(rec, req)

		res := rec.Result()
		contract.CheckResponse(t, req, res)
		var p httpserver.Problem
		_ = json.Unmarshal(rec.Body.Bytes(), &p)
		// The detail is the authentication middleware's: a handler that
		// finds no caller answers 401 too, but only after it has run.
		turnedAway := res.StatusCode == http.StatusUnauthorized && res.Header.Get("WWW-Authenticate") == "Bearer" &&
			p.Detail == "This operation requires a bearer token."
		if turnedAway == op.Public {
			t.Errorf("%s without a token = %d %s, WWW-Authenticate %q; want the middleware's 401 exactly when the contract protects it (public: %v)",
				op.Pattern(), res.StatusCode, rec.Body, res.Header.Get("WWW-Authenticate"), op.Public)
		}
	}
}
