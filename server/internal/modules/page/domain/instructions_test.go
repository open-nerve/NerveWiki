package domain_test

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// member counts a string member of a structured answer as its text block
// writes it, closely enough for the tests: a quote four bytes, a line
// break three, any other its own.
func member(s string) int {
	return len(s) + 3*strings.Count(s, `"`) + 2*strings.Count(s, "\n") + 2*strings.Count(s, "\r")
}

// frontmatterEnd is the end of the content's frontmatter as the parse
// reads it, for the tests: a first line of "---", after a byte order mark
// maybe, up to the next.
func frontmatterEnd(content string) int {
	start := 0
	if strings.HasPrefix(content, "\xef\xbb\xbf") {
		start = 3
	}
	lines := strings.SplitAfter(content[start:], "\n")
	if strings.TrimSuffix(lines[0], "\r\n") != "---" && strings.TrimSuffix(lines[0], "\n") != "---" || !strings.HasSuffix(lines[0], "\n") {
		return 0
	}
	end := start + len(lines[0])
	for _, l := range lines[1:] {
		end += len(l)
		if strings.TrimSuffix(strings.TrimSuffix(l, "\n"), "\r") == "---" {
			return end
		}
	}
	return 0
}

// An AGENTS page's body is its content past its frontmatter and a byte
// order mark, without the white space around it; cut at its last line
// break within MaxInstructionsBytes as the measure given counts it, else
// at a character's boundary (M9/P1 design 3.15.3).
func TestAgentsBody(t *testing.T) {
	const max = domain.MaxInstructionsBytes
	long := strings.Repeat("a", max-20) + "\nsecond line\nthird line goes on"
	tests := []struct {
		name, content string
		measure       func(string) int
		body          string
		cut           bool
	}{
		{"a frontmatter", "---\ntags: [rules]\n---\n\n# Conventions\nWrite in English.\n", wire, "# Conventions\nWrite in English.", false},
		{"a frontmatter after a byte order mark, CRLF", "\xef\xbb\xbf---\r\na: 1\r\n---\r\n\r\n# Rules\r\nOne.\r\n", wire, "# Rules\r\nOne.", false},
		{"a byte order mark without a frontmatter", "\xef\xbb\xbf # Rules\n", wire, "# Rules", false},
		{"no closing line: no frontmatter", "---\na: 1\nbody\n", wire, "---\na: 1\nbody", false},
		{"a frontmatter alone", "---\na: 1\n---\n\n \n", wire, "", false},
		{"white space alone", " \n\t\r\n", wire, "", false},
		{"exactly the bound", strings.Repeat("x", max), wire, strings.Repeat("x", max), false},
		{"a byte more, without a line break", strings.Repeat("x", max+1), wire, strings.Repeat("x", max), true},
		{"a byte more, cut at its last line break", long, wire, strings.Repeat("a", max-20) + "\nsecond line", true},
		{"a line break the bound holds, white space before it", strings.Repeat("a", max-6) + " \t\n" + strings.Repeat("b", 9), wire,
			strings.Repeat("a", max-6), true},
		{"a character across the bound", strings.Repeat("a", max-1) + "漢", wire, strings.Repeat("a", max-1), true},
		{"CRLF counts its \\r two bytes", strings.Repeat("a\r\n", max/5) + "tail" + strings.Repeat("z", 10), wire,
			strings.TrimSpace(strings.Repeat("a\r\n", max/5)), true},
		{"quotes and line breaks, as the measure counts", strings.Repeat("\"\n", max/4+1), wire,
			strings.TrimSpace(strings.Repeat("\"\n", max/4)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, cut := domain.AgentsBody(tt.content, frontmatterEnd(tt.content), tt.measure)
			if body != tt.body || cut != tt.cut {
				t.Errorf("AgentsBody = %.80q… (%d bytes), %v; want %.80q… (%d bytes), %v", body, len(body), cut, tt.body, len(tt.body), tt.cut)
			}
			if tt.measure(body) > max {
				t.Errorf("the body measures %d, over %d", tt.measure(body), max)
			}
		})
	}
}

// fitsWithin is a fits of the handshake's kind: the text measures at most
// n.
func fitsWithin(measure func(string) int, n int) func(string) bool {
	return func(s string) bool { return measure(s) <= n }
}

// The instructions are the body when it was not cut and fits; else its
// longest prefix at a line break that fits with the note, which names the
// page's title now, and the note; at a character's boundary when no line
// break does.
func TestAgentsText(t *testing.T) {
	note := func(title string) string {
		return "\n\n(The rest of " + title + ".md is cut here: read it whole with read, path \"" + title + ".md\".)"
	}
	body := "# Rules\nOne.\nTwo."
	bytes := func(s string) int { return len(s) }
	tests := []struct {
		name string
		a    domain.Agents
		fits func(string) bool
		want string
	}{
		{"not cut, fitting", domain.Agents{Body: body, Title: "AGENTS"}, fitsWithin(wire, 1000), body},
		{"cut, fitting with the note", domain.Agents{Body: body, Cut: true, Title: "AGENTS"}, fitsWithin(wire, 1000), body + note("AGENTS")},
		{"the title now, after a rename of its case", domain.Agents{Body: body, Cut: true, Title: "agents"}, fitsWithin(wire, 1000),
			body + note("agents")},
		{"not cut, not fitting", domain.Agents{Body: "# Rules\nOne.\n" + strings.Repeat("x", 200), Title: "AGENTS"}, fitsWithin(bytes, len("# Rules\nOne.")+len(note("AGENTS"))+2),
			"# Rules\nOne." + note("AGENTS")},
		{"cut, the note taking a line", domain.Agents{Body: body, Cut: true, Title: "AGENTS"},
			fitsWithin(bytes, len("# Rules\nOne.")+len(note("AGENTS"))+3), "# Rules\nOne." + note("AGENTS")},
		{"no line break fits: a character's boundary", domain.Agents{Body: "漢字漢字漢字\nmore", Cut: true, Title: "A"},
			fitsWithin(bytes, 7+len(note("A"))), "漢字" + note("A")},
		{"nothing fits but the note", domain.Agents{Body: "abcdef", Cut: true, Title: "A"}, fitsWithin(wire, len(note("A"))-2),
			strings.TrimPrefix(note("A"), "\n\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.AgentsText(tt.a, tt.fits); got != tt.want {
				t.Errorf("AgentsText = %q, want %q", got, tt.want)
			}
		})
	}
}

// agentsTextBackwards is AgentsText measured one line break at a time
// from the end, then one character: what the binary search must find.
func agentsTextBackwards(a domain.Agents, fits func(string) bool) string {
	if !a.Cut && fits(a.Body) {
		return a.Body
	}
	note := "(The rest of " + a.Title + ".md is cut here: read it whole with read, path \"" + a.Title + ".md\".)"
	with := func(end int) string { return strings.TrimSpace(a.Body[:end]) + "\n\n" + note }
	if fits(with(len(a.Body))) {
		return with(len(a.Body))
	}
	for end := strings.LastIndexByte(a.Body, '\n'); end >= 0; end = strings.LastIndexByte(a.Body[:end], '\n') {
		if fits(with(end)) {
			return with(end)
		}
	}
	for end := len(a.Body) - 1; end > 0; end-- {
		if utf8.RuneStart(a.Body[end]) && fits(with(end)) {
			return with(end)
		}
	}
	return note
}

// The binary search finds what measuring back one line break at a time
// does, on random bodies, cut or not, and both kinds of fits: the
// handshake's, and open_notebook's, which also bounds the member.
func TestAgentsTextIsTheLastLineBreakThatFits(t *testing.T) {
	r := rand.New(rand.NewPCG(31, 15))
	pieces := []string{"a", "word ", "\n", "\r\n", "\"", "\\", "é", "漢", "😀", " ", "\n\n", strings.Repeat("x", 40)}
	for i := range 3000 {
		var b strings.Builder
		for range r.IntN(400) {
			b.WriteString(pieces[r.IntN(len(pieces))])
		}
		body := strings.TrimSpace(b.String())
		if body == "" {
			continue
		}
		a := domain.Agents{Body: body, Cut: r.IntN(2) == 0, Title: []string{"AGENTS", "agents", "A&B"}[r.IntN(3)]}
		n := r.IntN(wire(body) + 200)
		budget := r.IntN(member(body) + 300)
		for name, fits := range map[string]func(string) bool{
			"the handshake's": fitsWithin(wire, n),
			"open_notebook's": func(s string) bool { return wire(s) <= n && member(s) <= budget },
		} {
			got, want := domain.AgentsText(a, fits), agentsTextBackwards(a, fits)
			if got != want {
				t.Fatalf("#%d, %s fits: AgentsText(%q) = %q, want %q", i, name, a.Body, got, want)
			}
		}
	}
}
