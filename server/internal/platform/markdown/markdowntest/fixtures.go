package markdowntest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fixture is one case of the fixture set, the Markdown's specification
// (overall design 13.3): a page's content and the JSON of what it holds.
type Fixture struct {
	Name    string
	Content []byte
	JSON    []byte
}

// casesDir is the fixture set's directory, from the repository's root.
const casesDir = "tools/md-fixtures/cases"

// Fixtures reads the fixture set, looking for it from the test's directory
// up to the repository's root.
func Fixtures(tb testing.TB) []Fixture {
	tb.Helper()
	dir, err := os.Getwd()
	if err != nil {
		tb.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(casesDir))); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			tb.Fatalf("no %s above the test's directory", casesDir)
		}
		dir = parent
	}
	names, err := filepath.Glob(filepath.Join(dir, filepath.FromSlash(casesDir), "*.md"))
	if err != nil || len(names) == 0 {
		tb.Fatalf("fixtures: %d, %v", len(names), err)
	}
	out := make([]Fixture, 0, len(names))
	for _, name := range names {
		f := Fixture{Name: filepath.Base(name)}
		if f.Content, err = os.ReadFile(name); err != nil {
			tb.Fatal(err)
		}
		if f.JSON, err = os.ReadFile(strings.TrimSuffix(name, ".md") + ".json"); err != nil {
			tb.Fatal(err)
		}
		out = append(out, f)
	}
	return out
}
