package bootstrap

import (
	"log/slog"
	"os"
	"regexp"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/buildinfo"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// The MCP server's serverInfo is nervewiki and the build's version (00-M12 4.9, M12/P5/S1). make
// test-go-short stamps its test binaries with a version that is not the default and says which in
// NWIKI_TEST_VERSION: then a serverInfo written as the default, not read from buildinfo, differs.
func TestMCPServerInfoIsTheBuild(t *testing.T) {
	o := mcpOptions(config.Config{}, slog.New(slog.DiscardHandler), httpserver.APIErrors{}, nil, nil, nil, nil)
	if o.Name != "nervewiki" || o.Version != buildinfo.Get().Version {
		t.Errorf("serverInfo is %q %q, want nervewiki and the build's version %q", o.Name, o.Version, buildinfo.Get().Version)
	}
	stamped := os.Getenv("NWIKI_TEST_VERSION")
	if stamped == "" {
		t.Log("no version stamped (make test-go-short stamps one): a serverInfo written as the default would pass unseen")
		return
	}
	// buildinfo.go's default, read from its source as TestTheDefaultVersionIsOneEverywhere reads it
	source, err := os.ReadFile("../platform/buildinfo/buildinfo.go")
	if err != nil {
		t.Fatal(err)
	}
	found := regexp.MustCompile(`(?m)^var version = "([^"]+)"$`).FindAllSubmatch(source, -1)
	if len(found) != 1 {
		t.Fatalf("buildinfo.go: %d defaults of version, want one", len(found))
	}
	if got := buildinfo.Get().Version; got != stamped || got == string(found[0][1]) {
		t.Errorf("the test binary's version is %q, want the stamped %q, not buildinfo.go's default %q: the -ldflags of "+
			"make test-go-short did not reach it", got, stamped, found[0][1])
	}
}
