package domain

import (
	"strings"
	"testing"
)

func TestSlugProblem(t *testing.T) {
	for slug, want := range map[string]string{
		"acme":                  "",
		"a":                     "",
		"acme_2-x":              "",
		"0":                     "",
		strings.Repeat("a", 48): "",
		"":                      SlugInvalid,
		strings.Repeat("a", 49): SlugInvalid,
		"Acme":                  SlugInvalid,
		" acme":                 SlugInvalid,
		"acme ":                 SlugInvalid,
		"acme corp":             SlugInvalid,
		"acme.corp":             SlugInvalid,
		"acme/corp":             SlugInvalid,
		"café":                  SlugInvalid,
		"ａｃｍｅ":                  SlugInvalid, // full-width letters
		"api":                   SlugReserved,
		"sign-in":               SlugReserved,
		"invitations":           SlugReserved,
		"mcp":                   SlugReserved,
		"apis":                  "",
		"settings-of-acme":      "",
	} {
		if got := SlugProblem(slug); got != want {
			t.Errorf("SlugProblem(%q) = %q, want %q", slug, got, want)
		}
	}
}

func TestTheReservedListParses(t *testing.T) {
	r := Reserved()
	if len(r.App) == 0 || len(r.Server) == 0 || len(r.Reserved) == 0 {
		t.Errorf("Reserved() = %+v, want names in every section", r)
	}
}

func TestParseReservedRefusesAMalformedList(t *testing.T) {
	for _, text := range []string{
		"[apps]\nfoo",               // unknown section
		"foo\n[app]\nbar",           // a name before any section
		"[app]\nFoo",                // not spelled as a slug
		"[app]\nfoo bar",            // not spelled as a slug
		"[app]\nfoo\n[server]\nfoo", // listed twice
	} {
		if r, err := parseReserved(text); err == nil {
			t.Errorf("parseReserved(%q) = %+v, want an error", text, r)
		}
	}
	r, err := parseReserved("# a comment\n\n[server]\n  api  \n[reserved]\n# held\nmcp\n")
	if err != nil || len(r.App) != 0 || len(r.Server) != 1 || r.Server[0] != "api" || len(r.Reserved) != 1 || r.Reserved[0] != "mcp" {
		t.Errorf("parseReserved() = %+v, %v", r, err)
	}
}
