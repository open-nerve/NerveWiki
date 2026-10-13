package domain_test

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// A report keeps its first 1,000 problems (the interface's number, not
// the constant's: Codex M7 review 6.1), in order, and marks the rest left
// out; each path is cut to 1,024 bytes at a character's boundary.
func TestAReportKeepsItsFirstProblems(t *testing.T) {
	var r domain.Report
	long := strings.Repeat("名", 400) // 1,200 bytes
	r.Add(domain.Problem{Path: long, Code: domain.ProblemRenamed, To: long + "x"})
	for i := 1; i < 1000; i++ {
		r.Add(domain.Problem{Path: strconv.Itoa(i), Code: domain.ProblemFileMissing})
	}
	if r.Truncated {
		t.Fatal("truncated at 1,000 problems, want only past them")
	}
	r.Add(domain.Problem{Path: "last", Code: domain.ProblemFileMissing})
	if len(r.Problems) != 1000 || !r.Truncated || r.Problems[1].Path != "1" || r.Problems[999].Path != "999" {
		t.Errorf("%d problems, truncated %v; want 1,000 in order, true", len(r.Problems), r.Truncated)
	}
	p := r.Problems[0]
	if len(p.Path) != 1023 || len(p.To) != 1023 || !utf8.ValidString(p.Path) || !utf8.ValidString(p.To) {
		t.Errorf("paths of %d and %d bytes, want 1,023 (341 characters of three bytes), valid", len(p.Path), len(p.To))
	}
	r = domain.Report{}
	r.Add(domain.Problem{Path: strings.Repeat("a", 1024), Code: domain.ProblemFileMissing})
	if len(r.Problems[0].Path) != 1024 {
		t.Errorf("a path of 1,024 bytes cut to %d", len(r.Problems[0].Path))
	}
}

// The files of formats compressed already go in as they are.
func TestStoredFormats(t *testing.T) {
	for name, want := range map[string]bool{
		"a.png": true, "a.JPG": true, "a.mp4": true, "a.webm": true, "a.zip": true, "a.docx": true, "a.flac": true,
		"a.md": false, "a.txt": false, "a.svg": false, "a.pdf": false, "a.bmp": false, "a.wav": false, "a": false, "png": false,
	} {
		if got := domain.Stored(name); got != want {
			t.Errorf("Stored(%q) = %v, want %v", name, got, want)
		}
	}
}
