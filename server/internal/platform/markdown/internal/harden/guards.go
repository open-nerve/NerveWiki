package harden

import (
	"bytes"
	"sort"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// index is what the guards look up in the block being parsed: where the
// closing sequences are, so an opener without its closer is turned down at
// once instead of scanning the rest of the block for it, once per opener.
// Positions are in the source.
type index struct {
	block ast.Node
	// ticks: for each length, where the last backtick string of it starts.
	ticks map[int]int
	// Where the last of each ends a raw HTML comment, processing
	// instruction, declaration or CDATA section starts, -1 for none.
	comment, instruction, declaration, cdata int
	// angles: the '>' no backslash escapes, where a destination in angle
	// brackets may end, in order.
	angles []int
	// emails: the runs of characters an e-mail address's local part may
	// have, in order.
	emails []email
}

// email is a run of local-part characters, [start, end), and whether
// goldmark's linkify makes an address that starts in it a link.
type email struct {
	start, end int
	link       bool
}

var indexKey = parser.NewContextKey() //nolint:gochecknoglobals // a key is made once, as goldmark's are

// indexOf is the index of block, made the first time a guard needs it.
func indexOf(block ast.Node, source []byte, pc parser.Context) *index {
	if ix, ok := pc.Get(indexKey).(*index); ok && ix.block == block {
		return ix
	}
	ix := &index{block: block, ticks: map[int]int{}, comment: -1, instruction: -1, declaration: -1, cdata: -1}
	lines := block.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		ix.add(seg.Start, source[seg.Start:seg.Stop])
	}
	pc.Set(indexKey, ix)
	return ix
}

func (ix *index) add(at int, line []byte) {
	last := func(sep string) int {
		if k := bytes.LastIndex(line, []byte(sep)); k >= 0 {
			return at + k
		}
		return -1
	}
	ix.comment = max(ix.comment, last("-->"))
	ix.instruction = max(ix.instruction, last("?>"))
	ix.declaration = max(ix.declaration, last(">"))
	ix.cdata = max(ix.cdata, last("]]>"))
	for _, r := range runsOf(line, func(c byte) bool { return c == '`' }) {
		ix.ticks[r[1]-r[0]] = at + r[0]
	}
	for _, r := range runsOf(line, isEmailChar) {
		ix.emails = append(ix.emails, email{at + r[0], at + r[1], emailLink(line[r[0]:])})
	}
	backslashes := 0
	for j, c := range line {
		switch {
		case c == '\\':
			backslashes++
			continue
		case c == '>' && backslashes%2 == 0:
			ix.angles = append(ix.angles, at+j)
		}
		backslashes = 0
	}
}

// runsOf is the maximal runs of bytes of line that in holds, as [start,
// end).
func runsOf(line []byte, in func(c byte) bool) [][2]int {
	var runs [][2]int
	for j := 0; j < len(line); j++ {
		if !in(line[j]) {
			continue
		}
		k := j
		for k < len(line) && in(line[k]) {
			k++
		}
		runs = append(runs, [2]int{j, k})
		j = k
	}
	return runs
}

// isEmailChar is goldmark's util.FindEmailIndex test for the local part:
// letters, digits and .!#$%&'*+/=?^_`{|}~-.
func isEmailChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte(".!#$%&'*+/=?^_`{|}~-", c) >= 0
}

// emailLink tells whether goldmark's linkify makes the e-mail address at
// the head of line a link, wherever in its local part it starts: the
// address is found, has a '.' before its last character and no '-' or '_'
// after it.
func emailLink(line []byte) bool {
	stop := util.FindEmailIndex(line)
	if stop < 0 {
		return false
	}
	at := bytes.IndexByte(line, '@')
	if bytes.IndexByte(line[at:stop-1], '.') < 0 {
		return false
	}
	if line[stop-1] == '.' {
		stop--
	}
	return stop >= len(line) || line[stop] != '-' && line[stop] != '_'
}

// textOf consumes n bytes of the reader as text.
func textOf(block text.Reader, seg text.Segment, n int) ast.Node {
	block.Advance(n)
	return ast.NewTextSegment(seg.WithStop(seg.Start + n))
}

// codeSpanGuard goes before goldmark's code span parser: a backtick string
// with no string of its length after it in the block is text, which is
// what the parser would make of it after scanning to the end of the block.
type codeSpanGuard struct{}

func (codeSpanGuard) Trigger() []byte { return []byte{'`'} }

func (codeSpanGuard) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, seg := block.PeekLine()
	n := 0
	for n < len(line) && line[n] == '`' {
		n++
	}
	if last, ok := indexOf(parent, block.Source(), pc).ticks[n]; ok && last >= seg.Start+n {
		return nil
	}
	return textOf(block, seg, n)
}

// rawHTMLGuard goes before goldmark's raw HTML parser and after its
// autolinks: a comment, processing instruction, declaration or CDATA section
// with no end after it in the block is text. goldmark looks for the end from
// the '<' (the comment's after "<!--").
type rawHTMLGuard struct{}

func (rawHTMLGuard) Trigger() []byte { return []byte{'<'} }

func (rawHTMLGuard) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, seg := block.PeekLine()
	var last, from int
	switch {
	case bytes.HasPrefix(line, []byte("<!-->")), bytes.HasPrefix(line, []byte("<!--->")):
		return nil
	case bytes.HasPrefix(line, []byte("<!--")):
		last, from = indexOf(parent, block.Source(), pc).comment, seg.Start+4
	case bytes.HasPrefix(line, []byte("<?")):
		last, from = indexOf(parent, block.Source(), pc).instruction, seg.Start
	case bytes.HasPrefix(line, []byte("<![CDATA[")):
		last, from = indexOf(parent, block.Source(), pc).cdata, seg.Start
	case len(line) > 2 && line[1] == '!' && line[2] >= 'A' && line[2] <= 'Z':
		last, from = indexOf(parent, block.Source(), pc).declaration, seg.Start
	default:
		return nil
	}
	if last >= from {
		return nil
	}
	return textOf(block, seg, 1)
}

// linkify calls goldmark's GFM autolink parser only where it may make a
// link. After every inline node the parser reads the e-mail address it
// might start there, to the end of its run of local-part characters and
// past its '@'; reading a run that is no address once per node in it is
// quadratic. Whether a run is one is known once, in the index. A "www."
// that goldmark's pattern does not take is read as an address too.
type linkify struct{ inner parser.InlineParser }

func (l linkify) Trigger() []byte { return l.inner.Trigger() }

func (l linkify) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	if inLinkLabel(pc) {
		return nil
	}
	line, seg := block.PeekLine()
	at := seg.Start
	switch line[0] {
	case ' ', '*', '_', '~', '(':
		line = line[1:]
		at++
	}
	for _, p := range []string{"http:", "https:", "ftp:"} {
		if bytes.HasPrefix(line, []byte(p)) {
			return l.inner.Parse(parent, block, pc)
		}
	}
	if isWWW(line) {
		return l.inner.Parse(parent, block, pc)
	}
	emails := indexOf(parent, block.Source(), pc).emails
	k := sort.Search(len(emails), func(i int) bool { return emails[i].end > at })
	if k < len(emails) && emails[k].start <= at && !emails[k].link {
		return nil
	}
	return l.inner.Parse(parent, block, pc)
}

// isWWW tells whether goldmark's pattern of a "www." link takes the head
// of line: `^www\.[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-z]+`, a path maybe
// after it. The pattern's 256 repetitions make it slow to fail, and it
// fails at every node of a long run.
func isWWW(line []byte) bool {
	if !bytes.HasPrefix(line, []byte("www.")) {
		return false
	}
	for j := 5; j <= 260 && j+1 < len(line); j++ {
		if !isDomainChar(line[j-1]) {
			return false
		}
		if line[j] == '.' && line[j+1] >= 'a' && line[j+1] <= 'z' {
			return true
		}
	}
	return false
}

// isDomainChar is the pattern's [-a-zA-Z0-9@:%._\+~#=].
func isDomainChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("-@:%._+~#=", c) >= 0
}

// nested refuses to open a block quote, list, list item or footnote
// definition inside MaxNesting block quotes, list items and footnote
// definitions: the line is text instead.
type nested struct{ parser.BlockParser }

func (b nested) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	depth := 0
	for p := parent; p != nil; p = p.Parent() {
		if k := p.Kind(); k == ast.KindBlockquote || k == ast.KindListItem || k == east.KindFootnote {
			depth++
			if depth >= MaxNesting {
				return nil, parser.NoChildren
			}
		}
	}
	return b.BlockParser.Open(parent, reader, pc)
}
