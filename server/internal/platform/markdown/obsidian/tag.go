package obsidian

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// kindTag is the kind of a tag.
var kindTag = ast.NewNodeKind("Tag") //nolint:gochecknoglobals // a kind is made once, as goldmark's are

// tag is #name (rule 9). Its one child is its text, '#' and name.
type tag struct {
	ast.BaseInline
	name string
	seg  text.Segment // '#' and name
	// spaced: a space or the start of a line is before it, not an inline
	// node, whose own second look the tag waits for (tagsAfterText).
	spaced bool
	// cut is how many '_' the name ends with that the parse left to the
	// emphasis: they are the name's if they are text after it.
	cut    int
	inLink bool // in a Markdown link's text
}

// Kind implements ast.Node.
func (t *tag) Kind() ast.NodeKind { return kindTag }

// RendersLink implements markdown.Linker: a tag the tag pane counts
// renders as a link but in a Markdown link's text (M6/P6 design 3), and
// but for the tag "/" (#//), which an address's path cannot name: the
// server's router reads its %2F as a trailing slash (M6/P6 fix check).
func (t *tag) RendersLink() bool {
	counted, ok := CountedTag(t.name)
	return ok && counted != "/" && !t.inLink
}

// Dump implements ast.Node.
func (t *tag) Dump(source []byte, level int) {
	ast.DumpHelper(t, source, level, map[string]string{"Name": t.name}, nil)
}

// isTagRune tells whether r may be in a tag's name: a letter, a mark, a
// number, '_', '-' or '/'.
func isTagRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) || r == '_' || r == '-' || r == '/'
}

// tagParser parses tags.
type tagParser struct{}

func (tagParser) Trigger() []byte { return []byte{'#'} }

// Parse makes a tag where a text starts: after a space (a full-width one
// too), at the start of a line, or right after an inline node, whatever
// the character before. An ATX heading's '#' never comes here: the
// heading's parser takes it first.
func (tagParser) Parse(parent ast.Node, block text.Reader, _ parser.Context) ast.Node {
	spaced := unicode.IsSpace(block.PrecendingCharacter()) || atLineStart(parent, block)
	if !spaced {
		// The text before '#' is a text node already: goldmark adds it
		// before it calls a parser.
		if last := parent.LastChild(); last == nil || last.Kind() == ast.KindText || last.Kind() == ast.KindString {
			return nil
		}
	}
	line, seg := block.PeekLine()
	i := 1
	for i < len(line) {
		r, size := utf8.DecodeRune(line[i:])
		if !isTagRune(r) {
			break
		}
		i += size
	}
	if !isTagName(line[1:i]) {
		return nil
	}
	// A '_' that ends the name may close an emphasis, _#tag_, which is
	// known after the parse: the name leaves it to the emphasis's runs, all
	// but one if the name needs one.
	cut := 0
	for line[i-1-cut] == '_' {
		cut++
	}
	if cut > 0 && !isTagName(line[1:i-cut]) {
		cut--
	}
	block.Advance(i - cut)
	t := &tag{name: string(line[1 : i-cut]), seg: seg.WithStop(seg.Start + i - cut), spaced: spaced, cut: cut}
	t.AppendChild(t, ast.NewTextSegment(t.seg))
	return t
}

// CountedTag is the tag Obsidian's tag pane counts for name, a tag's name
// or a frontmatter's tag, without its '#' (its getTags, read from its
// code): name without one '/' it ends with, if that is not empty, not all
// ASCII digits, and has none of the characters the pane refuses, white
// space as JavaScript's \s takes it, the General and Supplemental
// Punctuation blocks, and ASCII's !"#$%&'()*+,.:;<=>?@[\]^`{|}~. A body's
// tag is one but for its last '/'.
func CountedTag(name string) (string, bool) {
	name = strings.TrimSuffix(name, "/")
	if strings.ContainsFunc(name, refused) || !isTagName([]byte(name)) {
		return "", false
	}
	return name, true
}

// refused tells whether Obsidian's tag pane refuses r in a tag.
func refused(r rune) bool {
	return r == '\uFEFF' || r != '\u0085' && unicode.IsSpace(r) ||
		'\u2000' <= r && r <= '\u206F' || '\u2E00' <= r && r <= '\u2E7F' ||
		r < utf8.RuneSelf && strings.ContainsRune(`!"#$%&'()*+,.:;<=>?@[\]^`+"`"+`{|}~`, r)
}

// isTagName tells whether name, of tag runes, is one: not empty, not all
// ASCII digits (other numbers, ½ or ١, make a name, as in Obsidian).
func isTagName(name []byte) bool {
	for _, c := range name {
		if c < '0' || c > '9' {
			return true
		}
	}
	return false
}

// atLineStart tells whether the reader is at the start of one of block's
// lines: after a container's marker, such as a block quote's '>'.
func atLineStart(block ast.Node, reader text.Reader) bool {
	l, pos := reader.Position()
	lines := block.Lines()
	return l < lines.Len() && lines.At(l).Start == pos.Start
}

// tagsAfterText takes a second look at the tags, once the runs of '*', '_',
// '~' and '=' are paired and those left over are text, as is a comment's
// '%%' left alone. The '_' a tag left to a run that is text are its name's
// again. A tag after such text, rather than after an inline node, is text,
// as one after any text is.
type tagsAfterText struct{}

func (tagsAfterText) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	var tags []*tag
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := n.(*tag); ok && entering {
			tags = append(tags, t)
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	// In the tree's order: a tag after one made text here is text too.
	for _, t := range tags {
		if k := leftOver(t, reader.Source()); k > 0 {
			next := t.NextSibling().(*ast.Text)
			t.name += strings.Repeat("_", k)
			t.seg = t.seg.WithStop(t.seg.Stop + k)
			t.FirstChild().(*ast.Text).Segment = t.seg
			next.Segment = next.Segment.WithStart(t.seg.Stop)
		}
		if t.spaced {
			continue
		}
		if prev := t.PreviousSibling(); prev != nil && (prev.Kind() == ast.KindText || prev.Kind() == ast.KindString) {
			ast.MergeOrReplaceTextSegment(t.Parent(), t, t.seg)
		}
	}
}

// leftOver is how many of the '_' t left to the runs are text right after
// it: none if a run took the first of them, fewer if it took the last.
func leftOver(t *tag, source []byte) int {
	next, ok := t.NextSibling().(*ast.Text)
	if !ok || t.cut == 0 || next.Segment.Start != t.seg.Stop || next.Segment.Padding != 0 {
		return 0
	}
	k := 0
	for k < t.cut && next.Segment.Start+k < next.Segment.Stop && source[next.Segment.Start+k] == '_' {
		k++
	}
	return k
}
