package domain

import (
	"strings"
	"unicode/utf8"
)

// MaxContext is the most bytes of its line a backlink's context holds
// (M6 design 4.11).
const MaxContext = 240

// contextLead is about how many bytes of a long line a backlink's context
// holds before the link (M6/P5 design 3).
const contextLead = 80

// ellipsis marks where a context cuts its line.
const ellipsis = "…"

// Context is the context of a link whose target is content's bytes from
// start to end (M6/P5 design 3): the line where it starts, which ends at
// "\n", "\r" or both, as CommonMark's lines do. A line of at most
// MaxContext bytes is the whole of it; of a longer one, at most MaxContext
// bytes with the link's start, cut on characters' boundaries, with an
// ellipsis where it cuts: from the line's start when the link ends within
// its first MaxContext bytes; otherwise from about contextLead bytes before
// the link, though no later than MaxContext bytes before the line's end.
// It holds no bytes of content's memory, which a list of contexts would
// keep whole.
func Context(content string, start, end int) string {
	from := strings.LastIndexAny(content[:start], "\r\n") + 1
	to := len(content)
	if i := strings.IndexAny(content[start:], "\r\n"); i >= 0 {
		to = start + i
	}
	if to-from <= MaxContext {
		return strings.Clone(content[from:to])
	}
	s := from
	if end > from+MaxContext {
		s = max(from, min(start-contextLead, to-MaxContext))
	}
	for s < start && !utf8.RuneStart(content[s]) {
		s++
	}
	e := min(to, s+MaxContext)
	for e < to && !utf8.RuneStart(content[e]) {
		e--
	}
	var b strings.Builder
	if s > from {
		b.WriteString(ellipsis)
	}
	b.WriteString(content[s:e])
	if e < to {
		b.WriteString(ellipsis)
	}
	return b.String()
}
