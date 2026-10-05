package obsidian

import (
	"bytes"
	"strconv"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// kindWikilink is the kind of a wikilink and of an embed.
var kindWikilink = ast.NewNodeKind("Wikilink") //nolint:gochecknoglobals // a kind is made once, as goldmark's are

// wikilink is [[target#anchor|display]], or the embed ![[…]] (rule 7). What
// it holds is not parsed: its one child is the text it shows, so that a
// heading's id and an image's text have it.
type wikilink struct {
	ast.BaseInline
	embed  bool
	inLink bool // in a Markdown link's text
	parts
}

// Kind implements ast.Node.
func (w *wikilink) Kind() ast.NodeKind { return kindWikilink }

// RendersLink implements markdown.Linker: a user's <a> around it is dropped.
func (w *wikilink) RendersLink() {}

// Dump implements ast.Node.
func (w *wikilink) Dump(source []byte, level int) {
	ast.DumpHelper(w, source, level, map[string]string{
		"Embed": strconv.FormatBool(w.embed), "Target": w.target, "Anchor": w.anchor, "Display": w.display,
	}, nil)
}

// parts are what a wikilink's brackets hold: its target, where the target
// is written, its anchor and its display text, each trimmed of spaces and
// tabs; an empty one is none.
type parts struct {
	target, anchor, display string
	at                      markdown.Span
}

// split reads the bytes between a wikilink's brackets: the first '|' or
// "\|" starts the display text, the first '#' before it the anchor. at is
// where the target is written in inner.
func split(inner []byte) parts {
	target, display := inner, []byte(nil)
	if k := bytes.IndexByte(inner, '|'); k >= 0 {
		target, display = inner[:k], inner[k+1:]
		if k > 0 && inner[k-1] == '\\' {
			target = inner[:k-1]
		}
	}
	var anchor []byte
	if k := bytes.IndexByte(target, '#'); k >= 0 {
		target, anchor = target[:k], target[k+1:]
	}
	lead := len(target) - len(bytes.TrimLeft(target, " \t"))
	target = bytes.Trim(target, " \t")
	return parts{
		target:  string(target),
		anchor:  string(bytes.Trim(anchor, " \t")),
		display: string(bytes.Trim(display, " \t")),
		at:      markdown.Span{Start: lead, Stop: lead + len(target)},
	}
}

// shown is the text the wikilink shows: its display text, or its target
// and anchor as Obsidian shows them. An embed's display text is a size.
func (w *wikilink) shown() string {
	switch {
	case w.display != "" && !w.embed:
		return w.display
	case w.target == "":
		return w.anchor
	case w.anchor == "":
		return w.target
	}
	return w.target + " > " + w.anchor
}

// wikilinkParser parses wikilinks and embeds.
type wikilinkParser struct{}

func (wikilinkParser) Trigger() []byte { return []byte{'[', '!'} }

// Parse makes the wikilink from "[[" to the first "]]" of the line, if no
// other bracket comes between: each scan stops at the next bracket, so a
// line's scans read it about twice. A wikilink with neither a target nor
// an anchor is text.
func (wikilinkParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, seg := block.PeekLine()
	open := 2
	if line[0] == '!' {
		open = 3
	}
	if len(line) < open || !bytes.HasPrefix(line[open-2:], []byte("[[")) {
		return nil
	}
	end := -1
scan:
	for i := open; i < len(line); i++ {
		switch line[i] {
		case '[', '\n', '\r':
			return nil
		case ']':
			if i+1 < len(line) && line[i+1] == ']' {
				end = i
				break scan
			}
			return nil
		}
	}
	if end < 0 {
		return nil
	}
	p := split(line[open:end])
	if p.target == "" && p.anchor == "" {
		return nil
	}
	// The line starts with the trigger, so it has no padding: line[i] is
	// the source's seg.Start+i.
	from := seg.Start + open
	p.at = markdown.Span{Start: from + p.at.Start, Stop: from + p.at.Stop}
	block.Advance(end + 2)
	w := &wikilink{embed: open == 3, parts: p}
	shown := ast.NewString([]byte(w.shown()))
	shown.SetRaw(true)
	w.AppendChild(w, shown)
	return w
}
