package bootstrap

import (
	"net/http"
	"strings"
	"testing"
)

// A route a CONNECT may take, of any method or of CONNECT, holds no
// wildcard: when the router redirects a CONNECT to the path and a slash, it
// sets the request's pattern to that path, which the logs then read as a
// route's (httpserver's loggedPath), and a path's text would be written as
// a wildcard's value (M6 Codex review, fix checks B4-Q1, B5-N2). Those
// routes are "/" and the API's subtree.
func TestTheRoutesOfAConnectHoldNoWildcard(t *testing.T) {
	a := buildApp(t, testConfig(t, unreachableDB, false), sampleMigrations())
	for _, pattern := range a.router.Patterns() {
		method, _, ok := strings.Cut(pattern, " ")
		if (!ok || method == http.MethodConnect) && strings.Contains(pattern, "{") {
			t.Errorf("the route %q, which a CONNECT may take, holds a wildcard", pattern)
		}
	}
}
