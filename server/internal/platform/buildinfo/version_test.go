package buildinfo

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// matches reads file, from the repository's root, and returns the first group of each match of re, which
// must match exactly n times. When it does not, the test fails naming the file and each match with its
// line, and matches returns nil.
func matches(t *testing.T, file, re string, n int) []string {
	t.Helper()
	content, err := os.ReadFile("../../../../" + file)
	if err != nil {
		t.Fatal(err)
	}
	found := regexp.MustCompile(re).FindAllSubmatchIndex(content, -1)
	groups := make([]string, 0, len(found))
	for _, m := range found {
		groups = append(groups, string(content[m[2]:m[3]]))
	}
	if len(found) == n {
		return groups
	}
	seen := make([]string, 0, len(found))
	for i, m := range found {
		seen = append(seen, fmt.Sprintf("line %d: %s", strings.Count(string(content[:m[0]]), "\n")+1, groups[i]))
	}
	t.Errorf("%s: %d matches of %s, want %d; found %q", file, len(found), re, n, seen)
	return nil
}

// The version a build gets when nobody stamps one is the same everywhere it is written (00-M12 4.9,
// M12/P5/S1): Makefile's default, buildinfo's, both of deploy/Dockerfile's ARGs, and the one README
// tells contributors. A direct `docker build` takes the Dockerfile's ARG; make passes its own; a test
// binary has buildinfo's. The source is read, not the variable: make test-go-short stamps the variable.
func TestTheDefaultVersionIsOneEverywhere(t *testing.T) {
	source := matches(t, "server/internal/platform/buildinfo/buildinfo.go", `(?m)^var version = "([^"]+)"$`, 1)
	if source == nil {
		t.FailNow()
	}
	want := source[0]
	for _, c := range []struct {
		file, re, what string
		n              int
	}{
		// TEST_VERSION := … is no default: the pattern wants "VERSION ?=" at the line's start
		{"Makefile", `(?m)^VERSION \?= (\S+)$`, "the default of VERSION", 1},
		{"deploy/Dockerfile", `(?m)^ARG VERSION=(\S+)$`, "an ARG VERSION (the server and the runtime stages)", 2},
		{"README.md", "版本号取 `VERSION`，默认 `([^`]+)`", "the default README tells contributors", 1},
	} {
		for _, got := range matches(t, c.file, c.re, c.n) {
			if got != want {
				t.Errorf("%s: %s is %s, want buildinfo.go's %s", c.file, c.what, got, want)
			}
		}
	}
}
