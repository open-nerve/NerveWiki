package domain

import (
	"strings"
	"unicode/utf8"
)

// MaxContext is the most bytes of its line a backlink's context holds
// (M6 design 4.11).
const MaxContext = 240

// MaxContexts is the most links of a page whose contexts its backlink
// gives (M6/P5 design 3): a page may write a link to another a million
// times.
const MaxContexts = 10

// MaxCount is the most links of a page to another its backlink counts:
// one of MaxCount is as many or more. Counting a million links of each
// page of a backlinks page would pass a request's deadline (M6/P5 review
// r1-1, r2-M1).
const MaxCount = 1000

// MaxContentRead is about the most bytes of contents a page of backlinks
// reads for their contexts: past it, a page has none. A hundred pages of
// 5 MiB would read 500 MB for one request (M6/P5 review r2-L1).
const MaxContentRead = 32 << 20

// Range is where a link's target is written in a content, in bytes.
type Range struct {
	Start, End int
}

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
//
// The line's ends are looked for no further than MaxContext bytes and one
// from start: one further changes nothing, so a page of one line of 5 MiB
// is not read whole for each link (M6/P5 review r1-2, r2-L2). from is then
// at most start-MaxContext-1 and to at least start+MaxContext+1, which the
// rules above read alike.
func Context(content string, start, end int) string {
	lo := max(0, start-MaxContext-1)
	from := lo
	if i := strings.LastIndexAny(content[lo:start], "\r\n"); i >= 0 {
		from = lo + i + 1
	}
	hi := min(len(content), start+MaxContext+1)
	to := hi
	if i := strings.IndexAny(content[start:hi], "\r\n"); i >= 0 {
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
	for e > s && e < to && !utf8.RuneStart(content[e]) {
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

// Contexts is the contexts of links whose targets are content's bytes at
// ranges, by start (M6/P5 design 3): one a line, the first link's on it,
// told by no line's end between it and the link before. A range past
// content is none: the index's ranges are of the content of its revision,
// which the caller compares.
func Contexts(content string, ranges []Range) []string {
	out := []string{}
	last := -1
	for _, r := range ranges {
		if r.Start < 0 || r.End > len(content) || r.Start >= r.End {
			continue
		}
		if last >= 0 && last <= r.Start && !strings.ContainsAny(content[last:r.Start], "\r\n") {
			continue
		}
		last = r.Start
		out = append(out, Context(content, r.Start, r.End))
	}
	return out
}
