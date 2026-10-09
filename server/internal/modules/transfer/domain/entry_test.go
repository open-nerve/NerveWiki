package domain_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// raw are entries of the names given, in order, each a regular file
// deflated, unless it ends with "/" (a folder); a name after "link:",
// "lock:" or "bz2:" is a symbolic link, encrypted, or of bzip2.
func raw(names ...string) []domain.RawEntry {
	out := make([]domain.RawEntry, len(names))
	for i, name := range names {
		e := domain.RawEntry{Index: i, Method: domain.MethodDeflate}
		switch {
		case strings.HasPrefix(name, "link:"):
			name, e.Special = strings.TrimPrefix(name, "link:"), true
		case strings.HasPrefix(name, "lock:"):
			name, e.Encrypted = strings.TrimPrefix(name, "lock:"), true
		case strings.HasPrefix(name, "bz2:"):
			name, e.Method = strings.TrimPrefix(name, "bz2:"), 12
		case strings.HasPrefix(name, "store:"):
			name, e.Method = strings.TrimPrefix(name, "store:"), domain.MethodStore
		}
		e.Name, e.Folder = name, strings.HasSuffix(name, "/")
		out[i] = e
	}
	return out
}

// shown is sorted as lines: each entry kept "index path[/]", each problem
// "code: path", and the meta's index.
func shown(s domain.Sorted) []string {
	var out []string
	for _, e := range s.Entries {
		line := fmt.Sprintf("%d %s", e.Index, e.Joined())
		if e.Folder {
			line += "/"
		}
		out = append(out, line)
	}
	for _, p := range s.Skipped {
		out = append(out, string(p.Code)+": "+p.Path)
	}
	if s.Meta >= 0 {
		out = append(out, fmt.Sprintf("meta %d", s.Meta))
	}
	return out
}

func TestClassify(t *testing.T) {
	for _, tt := range []struct {
		name  string
		names []string
		want  []string
	}{
		{"files and folders", []string{"a.md", "A/", "A/b.md", "x.png", "store:y.zip"},
			[]string{"0 a.md", "1 A/", "2 A/b.md", "3 x.png", "4 y.zip"}},
		{"backslashes, dots and empty names", []string{`A\b.md`, "./c//d.md", `.\e.md`},
			[]string{"0 A/b.md", "1 c/d.md", "2 e.md"}},
		{"NFC", []string{"Cafe\u0301.md"}, []string{"0 Café.md"}},
		{"not UTF-8", []string{"\xffa.md", "ok.md"}, []string{"1 ok.md", "name_not_utf8: \ufffda.md"}},
		{"leaving the root", []string{"../x.md", "a/../../x.md", "a/../b.md", "/etc/passwd", "C:/x.md", `c:\x.md`, `a\..\b.md`, "ok.md"},
			[]string{"7 ok.md", "unsafe_path: ../x.md", "unsafe_path: a/../../x.md", "unsafe_path: a/../b.md", "unsafe_path: /etc/passwd",
				"unsafe_path: C:/x.md", `unsafe_path: c:\x.md`, `unsafe_path: a\..\b.md`}},
		{"ignored", []string{".obsidian/app.json", "A/.DS_Store", "__MACOSX/._a.md", "A/__MACOSX/b", "Thumbs.db", "A/thumbs.DB", ".git/HEAD",
			".trash/x.md", "A/.nerve/meta.json", "a.md"}, []string{"9 a.md"}},
		{"the vault's meta", []string{".nerve/meta.json", "a.md", ".nerve/other.json"}, []string{"1 a.md", "meta 0"}},
		{"special files, encrypted, other methods", []string{"link:l.md", "lock:s.md", "bz2:b.md", "ok.md"},
			[]string{"3 ok.md", "special_file: l.md", "encrypted: s.md", "unsupported_method: b.md"}},
		{"duplicates", []string{"a.md", "a.md", `A\b.md`, "A/b.md", "Cafe\u0301.md", "Café.md", "A/", "A/", "a.md/"},
			[]string{"0 a.md", "2 A/b.md", "4 Café.md", "6 A/", "8 a.md/", "duplicate: a.md", "duplicate: A/b.md", "duplicate: Café.md"}},
		{"the vault's folder", []string{"V/", "V/.obsidian/app.json", "V/a.md", "V/B/c.png", "__MACOSX/V/._a.md", "V/.nerve/meta.json"},
			[]string{"2 a.md", "3 B/c.png", "meta 5"}},
		{"the vault's folder, marked by .nerve alone", []string{"V/.nerve/meta.json", "V/a.md"}, []string{"1 a.md", "meta 0"}},
		{"a folder without a mark", []string{"V/a.md", "V/b.md"}, []string{"0 V/a.md", "1 V/b.md"}},
		{"a folder beside another", []string{"V/.obsidian/app.json", "V/a.md", "W/b.md"}, []string{"1 V/a.md", "2 W/b.md"}},
		{"a folder beside a file", []string{"V/.obsidian/app.json", "V/a.md", "readme.md"}, []string{"1 V/a.md", "2 readme.md"}},
		{"nothing", nil, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := shown(domain.Classify(raw(tt.names...))); !slices.Equal(got, tt.want) {
				t.Errorf("Classify(%q) =\n%q\nwant\n%q", tt.names, got, tt.want)
			}
		})
	}
}
