package bootstrap

import (
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
)

// The whole-program tests (M0/P4 design 3.6): the wired app against the
// contract, operation by operation, so an operation added later is covered
// without a new test.

// The routes registered under /api/v0 are the contract's operations: a route
// the contract does not describe, or an operation no module serves, fails
// here.
func TestAPIRoutesAreTheContractsOperations(t *testing.T) {
	contract := apitest.Load(t)
	a := buildApp(t, testConfig(t, unreachableDB, false), sampleMigrations())

	var got []string
	for _, pattern := range a.router.Patterns() {
		path := pattern
		if _, rest, ok := strings.Cut(pattern, " "); ok {
			path = rest
		}
		if strings.HasPrefix(path, "/api/v0/") {
			got = append(got, pattern)
		}
	}
	slices.Sort(got)
	var want []string
	for _, op := range contract.Operations() {
		want = append(want, op.Pattern())
	}
	if len(want) == 0 || !slices.Equal(got, want) {
		t.Errorf("routes under /api/v0 = %q, want the contract's operations %q", got, want)
	}
}

// Every operation's default response, the problem, declares the header a
// problem may carry: Retry-After (shared.RateLimited, shared.ServerBusy).
// Each module declares its own Problem response, so a module that leaves it
// out fails here.
func TestEveryProblemResponseDeclaresItsHeaders(t *testing.T) {
	contract := apitest.Load(t)

	for _, op := range contract.Operations() {
		if !slices.Contains(op.ProblemHeaders, "Retry-After") {
			t.Errorf("%s: the default response declares %q, want Retry-After among them", op.Pattern(), op.ProblemHeaders)
		}
	}
}
