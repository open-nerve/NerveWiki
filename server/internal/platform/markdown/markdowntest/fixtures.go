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

// casesDir, resolveDir and renameDir are the fixture set's directories,
// from the repository's root.
const (
	casesDir   = "tools/md-fixtures/cases"
	resolveDir = "tools/md-fixtures/resolve"
	renameDir  = "tools/md-fixtures/rename"
)

// Fixtures reads the fixture set, looking for it from the test's directory
// up to the repository's root.
func Fixtures(tb testing.TB) []Fixture {
	tb.Helper()
	names, err := filepath.Glob(filepath.Join(fixtureDir(tb, casesDir), "*.md"))
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

// ResolveCase is one case of the fixture set's link resolution (M6/P3
// design 3.7): its name and JSON.
type ResolveCase struct {
	Name string
	JSON []byte
}

// ResolveCases reads the fixture set's resolution cases, looking for them
// as Fixtures does.
func ResolveCases(tb testing.TB) []ResolveCase {
	tb.Helper()
	names, err := filepath.Glob(filepath.Join(fixtureDir(tb, resolveDir), "*.json"))
	if err != nil || len(names) == 0 {
		tb.Fatalf("resolution cases: %d, %v", len(names), err)
	}
	out := make([]ResolveCase, 0, len(names))
	for _, name := range names {
		c := ResolveCase{Name: filepath.Base(name)}
		if c.JSON, err = os.ReadFile(name); err != nil {
			tb.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

// RenameCase is one case of the fixture set's rewriting of links on a rename
// or move (M6/P4 design 7): its name, its JSON, and its page's content
// before and after.
type RenameCase struct {
	Name          string
	JSON          []byte
	Content, Want []byte
}

// RenameCases reads the fixture set's rename cases, looking for them as
// Fixtures does.
func RenameCases(tb testing.TB) []RenameCase {
	tb.Helper()
	names, err := filepath.Glob(filepath.Join(fixtureDir(tb, renameDir), "*.json"))
	if err != nil || len(names) == 0 {
		tb.Fatalf("rename cases: %d, %v", len(names), err)
	}
	out := make([]RenameCase, 0, len(names))
	for _, name := range names {
		base := strings.TrimSuffix(name, ".json")
		c := RenameCase{Name: filepath.Base(base)}
		for path, into := range map[string]*[]byte{name: &c.JSON, base + ".md": &c.Content, base + ".out.md": &c.Want} {
			if *into, err = os.ReadFile(path); err != nil {
				tb.Fatal(err)
			}
		}
		out = append(out, c)
	}
	return out
}

// fixtureDir is dir, from the repository's root, found from the test's
// directory up.
func fixtureDir(tb testing.TB, dir string) string {
	tb.Helper()
	at, err := os.Getwd()
	if err != nil {
		tb.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(at, filepath.FromSlash(dir))); err == nil {
			return filepath.Join(at, filepath.FromSlash(dir))
		}
		parent := filepath.Dir(at)
		if parent == at {
			tb.Fatalf("no %s above the test's directory", dir)
		}
		at = parent
	}
}
