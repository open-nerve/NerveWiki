package bootstrap

import (
	"strings"
	"testing"
)

// A route of any method holds no wildcard: when the router redirects a
// CONNECT to the path and a slash, it sets the request's pattern to that
// path, which the logs then read as a route's (httpserver's loggedPath),
// and a path's text would be written as a wildcard's value (M6 Codex
// review, fix check B4-Q1). The routes of any method are "/" and the API's
// subtree.
func TestTheRoutesOfAnyMethodHoldNoWildcard(t *testing.T) {
	a := buildApp(t, testConfig(t, unreachableDB, false), sampleMigrations())
	for _, pattern := range a.router.Patterns() {
		if !strings.Contains(pattern, " ") && strings.Contains(pattern, "{") {
			t.Errorf("the route %q of any method holds a wildcard", pattern)
		}
	}
}
