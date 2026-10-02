// Package markdown is the one parse of Markdown and its rendering (overall
// design 4.3, 4.6; M4 design 4 and 8): Parse turns a page's content into a
// Document, its frontmatter, its tree and what each extension takes from the
// tree; Render turns a Document into the HTML of a reading view. Its parse
// is goldmark's, hardened (internal/harden), so it costs about the size of
// the content whatever the content is.
//
// It imports goldmark and go.yaml.in/yaml, and no other platform package.
package markdown

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/internal/harden"
)

// Extension extends the parse and the rendering (M5: task items' byte
// positions; M6: Obsidian's dialect; M7: embedded attachments). The
// composition root hands the registered ones to New.
type Extension struct {
	// Name keys what Extract takes from a Document; no two are the same.
	Name string
	// Parser adds goldmark's parsers, paragraph and tree transformers. A
	// delimiter syntax must not use goldmark's delimiter list: nothing
	// processes it (internal/harden).
	Parser []parser.Option
	// Extract takes the extension's result from the tree; content is the
	// page's, byte for byte. It may be nil.
	Extract func(root ast.Node, content []byte) any
	// Fetch gets the extension's data for one page before Render renders
	// it, from what Extract took: in the caller's read, holding no lock. Its
	// result goes to Renderer alone. It may be nil.
	Fetch func(ctx context.Context, page Page, extracted any) (any, error)
	// Renderer is goldmark's node renderers of the extension, given what
	// Fetch got. Its addresses must go through SafeURL. It may be nil.
	Renderer func(data any) []util.PrioritizedValue
	// Markup is what Renderer writes, for the test of the final HTML
	// (markdowntest.CheckHTML).
	Markup Markup
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

// Page is the page a Render is for.
type Page struct {
	NotebookID uuid.UUID
	PageID     uuid.UUID
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
	content     []byte
	source      []byte // content as the parser read it: its frontmatter blank
	root        ast.Node
	frontmatter Frontmatter
	extracted   map[string]any
}

// Frontmatter is the document's frontmatter.
func (d *Document) Frontmatter() Frontmatter { return d.frontmatter }

// Extracted is what the extension named name took from the document, nil
// for none.
func (d *Document) Extracted(name string) any { return d.extracted[name] }
