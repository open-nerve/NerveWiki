package buildinfo

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The image builds with the Go the modules ask for (00-M12 4.3, M12/P1/S2). The golang image sets
// GOTOOLCHAIN=local, so a toolchain line that moves on while deploy/Dockerfile stays behind builds the
// release with the older Go, silently: the image's tag and the version its server stage asserts must both
// be the two modules' toolchain, and so must the version README.md tells contributors.
func TestTheImageBuildsWithTheModulesToolchain(t *testing.T) {
	read := func(file string) string {
		t.Helper()
		content, err := os.ReadFile("../../../../" + file)
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	// The one line of file that re matches, by its first group; "" and an error when there are none or several.
	only := func(file string, re *regexp.Regexp) string {
		t.Helper()
		found := re.FindAllStringSubmatch(read(file), -1)
		if len(found) != 1 {
			t.Errorf("%s: %d lines match %s, want one", file, len(found), re)
			return ""
		}
		return found[0][1]
	}
	toolchain := regexp.MustCompile(`(?m)^toolchain (go\S+)$`)

	want := only("server/go.mod", toolchain)
	if want == "" {
		t.FailNow()
	}
	if got := only("server/tools/go.mod", toolchain); got != "" && got != want {
		t.Errorf("server/tools/go.mod: toolchain %s, want %s as in server/go.mod", got, want)
	}
	tag := only("deploy/Dockerfile", regexp.MustCompile(`(?m)^FROM golang:(\S+)-trixie@sha256:[0-9a-f]{64} AS server$`))
	if tag != "" && "go"+tag != want {
		t.Errorf("deploy/Dockerfile: the server stage is golang:%s-trixie, want the modules' toolchain %s", tag, want)
	}
	asserted := only("deploy/Dockerfile", regexp.MustCompile(`(?m)^RUN test "\$\(go env GOVERSION\)" = (go\S+) \|\|`))
	if asserted != "" && asserted != want {
		t.Errorf("deploy/Dockerfile: the server stage asserts %s, want the modules' toolchain %s", asserted, want)
	}
	told := "指定的 Go " + strings.TrimPrefix(want, "go") + "。"
	if n := strings.Count(read("README.md"), told); n != 1 {
		t.Errorf("README.md: %q %d times, want once: it tells contributors the modules' toolchain %s", told, n, want)
	}
}
