// Package markdown is the one parse of Markdown and its rendering (overall
// design 4.3, 4.6; M4 design 4 and 8): Parse turns a page's content into a
// Document, its frontmatter, its tree and what each extension takes from the
// tree; Render turns a Document into the HTML of a reading view; a write
// keeps a Document's Facts, which outlive its tree. Its parse is goldmark's,
// hardened (internal/harden), so it costs about the size of the content
// whatever the content is; a Budget bounds the content parsed at once.
//
// It imports goldmark, go.yaml.in/yaml, golang.org/x/net/html and
// golang.org/x/sync, and no other platform package.
package markdown

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/internal/harden"
)

// Extension extends the parse and the rendering (M5: task items' byte
// positions; M6: Obsidian's dialect; M7: embedded attachments). The
// composition root hands the registered ones to New.
type Extension struct {
	// Name keys what Extract takes from a Document; no two are the same.
	Name string
	// Parser adds goldmark's parsers, paragraph and tree transformers,
	// which must cost about the size of what they read, as the rest of the
	// parse does (markdowntest.CheckCosts). The parse they join is
	// hardened (internal/harden): a delimiter syntax must not use goldmark's
	// delimiter list, which nothing processes; goldmark's
	// Context.IsInLinkLabel is always false; and a link an extension makes
	// does not count for "a link may not contain a link".
	Parser []parser.Option
	// Extract takes the extension's result from the parse's tree. The
	// result outlives the tree, in the Document's Facts: it holds none of
	// the tree's nodes, each of which holds the whole tree
	// (markdowntest.CheckFacts), and with the frontmatter and the other
	// extensions' results at most Facts.Limit, as the budget counts them
	// (markdowntest.CheckCosts). It may be nil.
	Extract func(t Tree) any
	// Fetch gets the extension's data for one page before Render renders
	// it, from what Extract took: in the caller's read, holding no lock. Its
	// result goes to Links and Renderer alone. It may be nil.
	Fetch func(ctx context.Context, page Page, extracted any) (any, error)
	// Links is, given what Fetch got, how the Markdown links and images
	// the extension knows are written (M6: a link to a page of the
	// notebook, whose address the front end gives): by where a link's or
	// an image's destination starts in the content (Tree.Destination), the
	// attributes its <a> carries in place of its address, an image's inner
	// one too, and true; false leaves the address. Of the extensions that
	// answer true, the first registered is taken. They are written as
	// WriteAttrs writes them; Markup has the names. It may be nil.
	Links func(data any) func(start int) ([]Attr, bool)
	// Images is, given what Fetch got, how the Markdown images the
	// extension knows are written (M7: an attachment's): by where an
	// image's destination starts in the content, the Image that writes it,
	// and true; false leaves it to Links and the core, which write it as
	// a link. Of the extensions that answer true, the first registered is
	// taken. It may be nil.
	Images func(data any) func(start int) (Image, bool)
	// Expires is, given what Fetch got, when what the extension wrote
	// stops being valid (M7: the attachments' signed addresses); zero for
	// never. It may be nil.
	Expires func(data any) time.Time
	// Renderer is goldmark's node renderers of the extension, given what
	// Fetch got. Its addresses must go through SafeURL (WriteAttrs), and a
	// node it renders as a link is a Linker. It must render every kind of
	// node Parser makes: goldmark's renderer panics on a kind made after
	// every kind it renders. It may be nil.
	Renderer func(data any) []util.PrioritizedValue
	// Properties is how the property table writes the frontmatter's
	// strings written on one line, given what Fetch got (M6/P6 design 4):
	// each as a link, with its attributes, which WriteAttrs writes, and the
	// text it shows, which the table escapes; or as it is, false. Of the
	// extensions that write a string as a link, the first does. It may be
	// nil.
	Properties func(data any) func(s Scalar) ([]Attr, string, bool)
	// Markup is what Renderer and Properties write, for the test of the
	// final HTML (markdowntest.CheckHTML).
	Markup Markup
}

// Attr is an attribute of an element: its name, and its value, which the
// renderer escapes.
type Attr struct {
	Name, Value string
}

// Image writes a Markdown image an extension knows, the whole of it (M7/P3
// design 5.2): given the text it shows (ShownText) and whether it is in a
// link, which it must not write a link or a control in. What it writes is
// in its extension's Markup, its attributes written by WriteAttrs.
type Image func(w Writer, shown string, inLink bool)

// View is a page's reading view: its HTML, and when it stops being valid,
// the earliest of the extensions' Expires; zero for never.
type View struct {
	HTML    string
	Expires time.Time
}

// Hider is a node of an extension that hides what it holds from the
// reading view (M6: what a comment spans). A heading in it takes no id, and
// its text is no heading's or image's.
type Hider interface {
	Hides()
}

// Linker is a node of an extension that may render as a link (M6: a
// wikilink; P6: a tag): a user's <a> around one that does is dropped, as
// one around a Markdown link is. In a Markdown link's text it must render
// no link: the platform drops only a user's <a>.
type Linker interface {
	// RendersLink tells whether the node renders as a link, once the
	// parse's transformers have run.
	RendersLink() bool
}

// Markup is the HTML an extension's renderers write.
type Markup struct {
	// Elements are the elements, each with the attributes it may carry.
	Elements map[string][]string
	// URLs are the attributes that hold an address.
	URLs []string
	// Classes are the classes it gives.
	Classes []string
}

// Tree is what an extension's Extract reads of a parse (M6/P1 design 3.2).
type Tree struct {
	// Root is the parse's tree: its offsets are Content's.
	Root ast.Node
	// Content is the page's content, byte for byte.
	Content []byte
	// Frontmatter is the content's frontmatter, with its scalars.
	Frontmatter Frontmatter

	destinations func(ast.Node) (text.Segment, bool)
}

// Span is the bytes of the content from Start up to Stop.
type Span struct {
	Start, Stop int
}

// Destination tells where the destination of n, a Markdown link or image,
// is written in the content: its own, inside its angle brackets if it has
// them, or its reference definition's. An empty destination is nowhere.
func (t Tree) Destination(n ast.Node) (Span, bool) {
	if t.destinations == nil {
		return Span{}, false
	}
	s, ok := t.destinations(n)
	return Span{Start: s.Start, Stop: s.Stop}, ok
}

// Page is the page a Render is for, and the revision of its content
// rendered (M6: Fetch tells whether the link index is of it).
type Page struct {
	NotebookID uuid.UUID
	PageID     uuid.UUID
	Revision   int
}

// Markdown parses and renders. It is safe for concurrent use.
type Markdown struct {
	parser parser.Parser
	exts   []Extension
}

// New returns the Markdown with exts.
func New(exts []Extension) (*Markdown, error) {
	seen := map[string]bool{}
	var opts []parser.Option
	for _, e := range exts {
		switch {
		case e.Name == "":
			return nil, errors.New("markdown: an extension without a name")
		case seen[e.Name]:
			return nil, fmt.Errorf("markdown: two extensions named %q", e.Name)
		}
		seen[e.Name] = true
		opts = append(opts, e.Parser...)
	}
	opts = append(opts, parser.WithASTTransformers(util.Prioritized(headingIDs{}, 100)))
	return &Markdown{parser: harden.NewParser(opts...), exts: exts}, nil
}

// Document is a parse of a page's content. A Render changes its tree: it
// serves one request.
type Document struct {
	source []byte // the content as the parser read it: its frontmatter blank
	root   ast.Node
	facts  Facts
	// destinations is where the parse found the links' and images'
	// destinations written, for the extensions' Links.
	destinations func(ast.Node) (text.Segment, bool)
}

// Frontmatter is the document's frontmatter.
func (d *Document) Frontmatter() Frontmatter { return d.facts.frontmatter }

// Extracted is what the extension named name took from the document, nil
// for none.
func (d *Document) Extracted(name string) any { return d.facts.Extracted(name) }

// Facts is what the document found that outlives its tree.
func (d *Document) Facts() Facts { return d.facts }
