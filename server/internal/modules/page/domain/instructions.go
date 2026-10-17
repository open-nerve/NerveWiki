package domain

import (
	"sort"
	"strings"
)

// AgentsTitle is the title of a notebook's root page whose content is its
// instructions for MCP's agents (M9 design 4.6): found by its title key,
// so that "agents" is it too.
const AgentsTitle = "AGENTS"

// MaxInstructionsBytes bounds the initialize instructions, as
// mcpserver.TextBytes counts them, the note included (M9 design 4.6).
const MaxInstructionsBytes = 32 << 10

// Agents is an AGENTS page's text as cached: Body without the frontmatter
// and cut to MaxInstructionsBytes, Cut when it was; Title is the page's
// title now, for the note.
type Agents struct {
	Body  string
	Cut   bool
	Title string
}

// bom is the byte order mark a content may start with.
const bom = "\ufeff"

// AgentsBody is the body of a content whose frontmatter ends at end: the
// content after it and after a byte order mark, without the white space
// around it; cut, when measure counts more than MaxInstructionsBytes, at
// its last line break within it, else at a character's boundary. "" when
// nothing is left.
func AgentsBody(content string, end int, measure func(string) int) (body string, cut bool) {
	body = strings.TrimSpace(strings.TrimPrefix(content[end:], bom))
	n := prefix(body, MaxInstructionsBytes, measure)
	if n == len(body) {
		return body, false
	}
	if i := strings.LastIndexByte(body[:n], '\n'); i >= 0 {
		n = i
	}
	return strings.TrimSpace(body[:n]), true
}

// AgentsText is a's instructions within fits: the body when it was not cut
// and fits; else its longest prefix that ends at a line break (else at a
// character's boundary) and fits with the note, which names a.Title, and
// the note. fits must be monotone in the prefix, as both measures are
// (they add up per character), so the prefix is found by a binary search
// over the line breaks: about 15 measures of at most 32 KiB.
func AgentsText(a Agents, fits func(string) bool) string {
	if !a.Cut && fits(a.Body) {
		return a.Body
	}
	note := agentsNote(a.Title)
	with := func(end int) string { return strings.TrimSpace(a.Body[:end]) + "\n\n" + note }
	// The body's end is a line break of the content it was cut from.
	breaks := append(lineBreaks(a.Body), len(a.Body))
	if k := longest(len(breaks), func(k int) bool { return fits(with(breaks[k])) }); k >= 0 {
		return with(breaks[k])
	}
	var starts []int // the characters' boundaries but the first
	for i := range a.Body {
		if i > 0 {
			starts = append(starts, i)
		}
	}
	if k := longest(len(starts), func(k int) bool { return fits(with(starts[k])) }); k >= 0 {
		return with(starts[k])
	}
	return note
}

// agentsNote is the line that ends an AGENTS page's instructions cut
// short: title is the page's title now.
func agentsNote(title string) string {
	return "(The rest of " + title + `.md is cut here: read it whole with read, path "` + title + `.md".)`
}

// lineBreaks are the byte offsets of s's "\n", in order.
func lineBreaks(s string) []int {
	var out []int
	for i := 0; i < len(s); {
		j := strings.IndexByte(s[i:], '\n')
		if j < 0 {
			break
		}
		out = append(out, i+j)
		i += j + 1
	}
	return out
}

// longest is the largest k below n that ok holds for, ok being true up to
// some k and false after it; -1 when it holds for none.
func longest(n int, ok func(k int) bool) int {
	return sort.Search(n, func(k int) bool { return !ok(k) }) - 1
}
