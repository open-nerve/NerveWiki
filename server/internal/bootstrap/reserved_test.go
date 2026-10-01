package bootstrap

import (
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/webui"
)

// The server's section of the reserved slugs (M2/P1 design 3.6) is the
// top-level paths the wired server answers itself, beside the web app's
// pages: each pattern of the root router but "/", by its first segment,
// and the frontend's directory of built assets. A path served at the top
// that the list leaves out would be a workspace's address that the server
// takes first; the web app's section is checked by its own test.
func TestTheReservedServerSlugsAreTheServersTopLevelPaths(t *testing.T) {
	a := buildApp(t, testConfig(t, unreachableDB, false), sampleMigrations())
	got := []string{webui.AssetsDir}
	for _, pattern := range a.router.Patterns() {
		path := pattern
		if _, rest, ok := strings.Cut(pattern, " "); ok {
			path = rest
		}
		first, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
		if first != "" && !slices.Contains(got, first) {
			got = append(got, first)
		}
	}
	slices.Sort(got)
	want := slices.Sorted(slices.Values(workspace.Reserved().Server))
	if !slices.Equal(got, want) {
		t.Errorf("the server's top-level paths = %q, want the reserved list's [server] %q", got, want)
	}
}
