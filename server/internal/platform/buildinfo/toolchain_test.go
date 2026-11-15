package buildinfo

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The image builds with the Go the modules ask for (00-M12 4.3, M12/P1/S2). The golang image sets
// GOTOOLCHAIN=local, so a toolchain line that moves on while deploy/Dockerfile stays behind builds the
// release with the older Go, silently: the image's tag, the version its server stage asserts first thing
// and the version that assertion's message names must all be the two modules' toolchain, and so must the
// version README.md tells contributors.
func TestTheImageBuildsWithTheModulesToolchain(t *testing.T) {
	read := func(file string) string {
		t.Helper()
		content, err := os.ReadFile("../../../../" + file)
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	// The groups of the one match of re in file. With none or several it fails and returns nil, naming
	// the groups of each match, or with none the file's lines that begin with one of hints.
	only := func(file string, re *regexp.Regexp, hints ...string) []string {
		t.Helper()
		content := read(file)
		found := re.FindAllStringSubmatch(content, -1)
		if len(found) == 1 {
			return found[0][1:]
		}
		seen := make([]string, 0, len(found))
		for _, groups := range found {
			seen = append(seen, strings.Join(groups[1:], " "))
		}
		for i, line := range strings.Split(content, "\n") {
			for _, hint := range hints {
				if len(found) == 0 && strings.HasPrefix(line, hint) {
					seen = append(seen, fmt.Sprintf("line %d: %s", i+1, line))
					break
				}
			}
		}
		t.Errorf("%s: %d matches of %s, want one; found %q", file, len(found), re, seen)
		return nil
	}
	toolchain := regexp.MustCompile(`(?m)^toolchain (go\S+)$`)

	modules := only("server/go.mod", toolchain, "toolchain")
	if modules == nil {
		t.FailNow()
	}
	want := modules[0]
	if tools := only("server/tools/go.mod", toolchain, "toolchain"); tools != nil && tools[0] != want {
		t.Errorf("server/tools/go.mod: toolchain %s, want %s as in server/go.mod", tools[0], want)
	}
	// The server stage's FROM, then comments only, then the assertion as the stage's first instruction
	// (decision 348), the whole line: the version it compares and the one its message names.
	stage := only("deploy/Dockerfile", regexp.MustCompile(
		`(?m)^FROM golang:(\S+)-trixie@sha256:[0-9a-f]{64} AS server\n(?:#.*\n)*`+
			`RUN test "\$\(go env GOVERSION\)" = (go\S+) \|\| `+
			`\{ echo "golang 镜像是 \$\(go env GOVERSION\)，go\.mod 要 (go[^：\s]+)：换摘要" >&2; exit 1; \}$`),
		"FROM golang:", "RUN test ")
	if stage != nil {
		if tag := stage[0]; "go"+tag != want {
			t.Errorf("deploy/Dockerfile: the server stage is golang:%s-trixie, want the modules' toolchain %s", tag, want)
		}
		if asserted := stage[1]; asserted != want {
			t.Errorf("deploy/Dockerfile: the server stage asserts %s, want the modules' toolchain %s", asserted, want)
		}
		if named := stage[2]; named != want {
			t.Errorf("deploy/Dockerfile: the assertion's message says go.mod asks for %s, want the modules' toolchain %s", named, want)
		}
	}
	asserting := regexp.MustCompile(`(?m)^RUN test "\$\(go env GOVERSION\)".*$`).FindAllString(read("deploy/Dockerfile"), -1)
	if len(asserting) != 1 {
		t.Errorf("deploy/Dockerfile: %d lines assert the Go version, want the server stage's one: %q", len(asserting), asserting)
	}
	if told := only("README.md", regexp.MustCompile(`指定的 Go (\S+?)。`)); told != nil && "go"+told[0] != want {
		t.Errorf("README.md: tells contributors Go %s, want the modules' toolchain %s", told[0], want)
	}
}
