package domain_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// stepNodes are a chain of nodes, from a root down, one a name: a page,
// or an attachment for a name with an extension that is not ".md".
func stepNodes(names ...string) []domain.Node {
	out := make([]domain.Node, len(names))
	for i, name := range names {
		out[i] = domain.Node{ID: uuid.NewV7(), Kind: domain.KindPage, Name: name}
		if strings.Contains(name, ".") && !strings.HasSuffix(strings.ToLower(name), ".md") {
			out[i].Kind = domain.KindAsset
		}
	}
	return out
}

// mustPath is s parsed as a path.
func mustPath(t *testing.T, s string) domain.Path {
	t.Helper()
	p, err := domain.ParsePath("path", s, false)
	if err != nil {
		t.Fatalf("ParsePath(%q): %v", s, err)
	}
	return p
}

// CreationOf, one rule a row (M9/P2 design 3.8): the leaf when Walk found
// it, a page; an attachment of its name taken; else the titles from the
// first missing segment, under the last found or the root; an attachment
// passed, a missing ancestor ending with .md in any case, and a path
// deeper than pages nest, refused before anything is written.
func TestCreationOf(t *testing.T) {
	deep := strings.Repeat("a/", 10) + "b.md"
	for _, tt := range []struct {
		name, path string
		found      []domain.Node
		titles     []string // the pages to create, when the leaf is not there
		want       string   // the problem, or "leaf" when the leaf is there
	}{
		{"the leaf there", "A/B.md", stepNodes("A", "B"), nil, "leaf"},
		{"an attachment holds the leaf's name", "A/x.png.md", stepNodes("A", "x.png"), nil, "taken A/x.png.md asset"},
		{"a root", "A.md", nil, []string{"A"}, ""},
		{"a page under one there", "A/B.md", stepNodes("A"), []string{"B"}, ""},
		{"two missing ancestors", "A/B/C/D.md", stepNodes("A"), []string{"B", "C", "D"}, ""},
		{"all missing", "A/B/C.md", nil, []string{"A", "B", "C"}, ""},
		{"an attachment passed", "A/x.png/C.md", stepNodes("A", "x.png"), nil, "validation_failed path not_allowed: segment 2 is an attachment, which holds no pages"},
		{"an attachment passed at the root", "x.png/C.md", stepNodes("x.png"), nil, "validation_failed path not_allowed: segment 1 is an attachment, which holds no pages"},
		{"a missing ancestor ending with .md", "A/B.md/C.md", stepNodes("A"), nil, "validation_failed path not_allowed: segment 2 ends with .md: an ancestor is written by its title, without .md"},
		{"a missing ancestor ending with .MD", "A.MD/C.md", nil, nil, "validation_failed path not_allowed: segment 1 ends with .md: an ancestor is written by its title, without .md"},
		{"a missing ancestor, then one ending with .md", "A/B/C.md/D.md", stepNodes("A"), nil, "validation_failed path not_allowed: segment 3 ends with .md: an ancestor is written by its title, without .md"},
		{"an ancestor titled with .md there", "A.md/C.md", stepNodes("A.md"), []string{"C"}, ""},
		{"a leaf titled with .md", "A/B.md.md", stepNodes("A"), []string{"B.md"}, ""},
		{"eleven levels", deep, stepNodes(strings.Split(strings.Repeat("a/", 10), "/")[:10]...), nil, "page.too_deep"},
		{"eleven levels, none there", deep, nil, nil, "page.too_deep"},
		{"ten levels", strings.Repeat("a/", 9) + "b.md", stepNodes("a", "a"), append(slices.Repeat([]string{"a"}, 7), "b"), ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := mustPath(t, tt.path)
			c, err := domain.CreationOf(domain.Args{}, path, tt.found)
			var taken *domain.PathTaken
			got := ""
			switch {
			case errors.As(err, &taken) && errors.Is(err, domain.ErrTitleTaken):
				got = "taken " + taken.Path
				if taken.Asset {
					got += " asset"
				}
			case errors.Is(err, domain.ErrTooDeep):
				got = "page.too_deep"
			case err != nil:
				got = said(err)
			case c.Leaf != nil:
				got = "leaf"
				if c.Leaf.ID != tt.found[len(tt.found)-1].ID || c.Titles != nil {
					t.Errorf("CreationOf = %+v, want the last found and nothing to create", c)
				}
			}
			if got != tt.want {
				t.Fatalf("CreationOf(%q) = %+v, %v; want %s", tt.path, c, err, tt.want)
			}
			if tt.want != "" {
				return
			}
			if !slices.Equal(c.Titles, tt.titles) {
				t.Errorf("CreationOf(%q) creates %q, want %q", tt.path, c.Titles, tt.titles)
			}
			switch {
			case len(tt.found) == 0 && c.Parent != nil:
				t.Errorf("CreationOf(%q) creates under %s, want the root", tt.path, *c.Parent)
			case len(tt.found) > 0 && (c.Parent == nil || *c.Parent != tt.found[len(tt.found)-1].ID):
				t.Errorf("CreationOf(%q) creates under %v, want the last found", tt.path, c.Parent)
			}
		})
	}
}

// said is a problem's code, field, field code and message.
func said(err error) string {
	var e *shared.Error
	if !errors.As(err, &e) || len(e.Fields) != 1 {
		return err.Error()
	}
	f := e.Fields[0]
	return e.Code + " " + f.Field + " " + f.Code + ": " + f.Message
}

// MoveKinds lets a page's path move to a page's and an attachment's to an
// attachment's, and refuses the two others on to.
func TestMoveKinds(t *testing.T) {
	const refused = "validation_failed to not_allowed: is not of from's kind: a page moves to a page's path, ending with .md, an attachment to an attachment's"
	for _, tt := range []struct{ from, to, want string }{
		{"A.md", "B/C.md", ""},
		{"A/x.png", "B/y.jpg", ""},
		{"A.md", "B/x.png", refused},
		{"A/x.png", "B/x.png.md", refused},
	} {
		err := domain.MoveKinds(domain.Args{}, mustPath(t, tt.from), mustPath(t, tt.to))
		if got := ""; err != nil {
			got = said(err)
			if got != tt.want {
				t.Errorf("MoveKinds(%q, %q) = %s, want %q", tt.from, tt.to, got, tt.want)
			}
		} else if tt.want != "" {
			t.Errorf("MoveKinds(%q, %q) = nil, want %q", tt.from, tt.to, tt.want)
		}
	}
}

// DeletionOf deletes a lone page or attachment, and a page with nodes
// under it only with recursive: *Recursive, counting them, otherwise.
func TestDeletionOf(t *testing.T) {
	sub := func(kinds ...domain.Kind) domain.Subtree {
		out := make(domain.Subtree, len(kinds))
		for i, k := range kinds {
			out[i] = domain.SubtreeNode{Node: domain.Node{ID: uuid.NewV7(), Kind: k}, Level: min(i+1, 2)}
		}
		return out
	}
	p, a := domain.KindPage, domain.KindAsset
	for _, tt := range []struct {
		name      string
		sub       domain.Subtree
		recursive bool
		pages     int
		assets    int // under it, when refused
		refused   bool
	}{
		{"a lone page", sub(p), false, 0, 0, false},
		{"a lone attachment", sub(a), false, 0, 0, false},
		{"a page with children", sub(p, p, a, p, a, p), false, 3, 2, true},
		{"a page with an attachment", sub(p, a), false, 0, 1, true},
		{"a page with children, recursive", sub(p, p, a), true, 0, 0, false},
	} {
		err := domain.DeletionOf(domain.Args{}, tt.sub, tt.recursive)
		var r *domain.Recursive
		switch {
		case !tt.refused && err != nil:
			t.Errorf("%s: DeletionOf = %v, want nil", tt.name, err)
		case tt.refused && (!errors.As(err, &r) || r.Pages != tt.pages || r.Assets != tt.assets):
			t.Errorf("%s: DeletionOf = %v, want *Recursive{%d, %d}", tt.name, err, tt.pages, tt.assets)
		}
	}
}

// ParentChainOf judges the pages Walk found towards move's parent (M9/P2
// design 3.8.3): the root, and a line of pages found whole, are the
// parent; a segment that is an attachment, where to goes on, is 422 on to,
// whether Walk stopped at it or found it last; a parent that names nothing
// is page.not_found at the parent's path, hinted that move makes no pages,
// unless a missing segment ends with .md, write's 422 on to.
func TestParentChainOf(t *testing.T) {
	a, b := stepNodes("A")[0], stepNodes("B")[0]
	img := stepNodes("x.png")[0]
	for _, tt := range []struct {
		name, to string
		chain    []domain.Node
		on       *domain.AttachmentOnPath
		want     string
	}{
		{"the root", "C.md", nil, nil, ""},
		{"a line found whole", "A/B/C.md", []domain.Node{a, b}, nil, ""},
		{"an attachment Walk stopped at", "A/x.png/C.md", []domain.Node{a, img}, &domain.AttachmentOnPath{Segment: 2},
			"validation_failed to not_allowed: segment 2 is an attachment, which holds no nodes"},
		{"an attachment found last", "x.png/C.md", []domain.Node{img}, nil,
			"validation_failed to not_allowed: segment 1 is an attachment, which holds no nodes"},
		{"a parent that names nothing", "A/B/C.md", []domain.Node{a}, nil, `not found "A/B.md", to's parent`},
		{"a missing segment ending with .md", "A/B.md/C.md", []domain.Node{a}, nil,
			"validation_failed to not_allowed: segment 2 ends with .md: an ancestor is written by its title, without .md"},
	} {
		err := domain.ParentChainOf(domain.Args{}, mustPath(t, tt.to), tt.chain, tt.on)
		got := ""
		var missing *domain.PathNotFound
		switch {
		case errors.As(err, &missing):
			got = `not found "` + missing.Path + `", ` + missing.ParentOf + "'s parent"
			if missing.Hint != domain.MoveParentHint || !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("%s: the hint %q of %v, want MoveParentHint's page.not_found", tt.name, missing.Hint, err)
			}
		case err != nil:
			got = said(err)
		}
		if got != tt.want {
			t.Errorf("%s: ParentChainOf = %s, want %s", tt.name, got, tt.want)
		}
	}
}

// PagePath lets write and edit take a page's path, ending with .md in any
// case, and refuses an attachment's on path in the tool's own words.
func TestPagePath(t *testing.T) {
	for _, tt := range []struct{ path, tool, want string }{
		{"A/B.md", "write", ""},
		{"A/B.MD", "edit", ""},
		{"A/x.png", "write",
			"validation_failed path not_allowed: is an attachment's path: write writes pages, whose paths end with .md; attachments are uploaded in the web"},
		{"x.png", "edit",
			"validation_failed path not_allowed: is an attachment's path: edit edits pages, whose paths end with .md; attachments are uploaded in the web"},
	} {
		got := ""
		if err := domain.PagePath(domain.Args{}, mustPath(t, tt.path), tt.tool); err != nil {
			got = said(err)
		}
		if got != tt.want {
			t.Errorf("PagePath(%q, %s) = %q, want %q", tt.path, tt.tool, got, tt.want)
		}
	}
}

// RenameTitle checks a new name by the node's kind: a page's by the title's
// rules, an attachment's by an attachment's, its extension kept and no
// page's .md.
func TestRenameTitle(t *testing.T) {
	page, img := stepNodes("Plan")[0], stepNodes("x.png")[0]
	for _, tt := range []struct {
		node       domain.Node
		name, want string
	}{
		{page, "Plan 2", "Plan 2"},
		{page, "x.md", "x.md"},
		{img, "y.jpg", "y.jpg"},
		{page, "a/b", `validation_failed name invalid_format: must not contain / \ : * ? " < > | # ^ [ ] or control characters`},
		{img, "y", "validation_failed name not_allowed: An attachment's new name must keep an extension: its type is told by it."},
		{img, "y.md", `validation_failed name not_allowed: An attachment's name must not end with ".md", which names a page's file.`},
	} {
		title, err := domain.RenameTitle("name", tt.node, tt.name)
		got := title.Name
		if err != nil {
			got = said(err)
		}
		if got != tt.want {
			t.Errorf("RenameTitle(%s, %q) = %q, want %q", tt.node.Name, tt.name, got, tt.want)
		}
	}
}

// RelocationOf answers a move's 409s in Move's order (M4/P1 design 3.6;
// M9/P2 design 3.5.1), which Unit.Relocate and a batch's plan share: a
// parent in the node's subtree first, then a sibling holding the name, of
// its kind, then a subtree too deep there, an attachment no level; the
// root is no parent in any subtree.
func TestRelocationOfIsMovesOrder(t *testing.T) {
	ids := stepNodes("N", "C", "img.png")
	n, c, img := ids[0], ids[1], ids[2]
	sub := domain.Subtree{{Node: n, Level: 1}, {Node: c, Level: 2}}
	withImage := domain.Subtree{{Node: n, Level: 1}, {Node: img, Level: 2}}
	line := func(depth int) []domain.Ancestor {
		out := make([]domain.Ancestor, depth)
		for i := range out {
			out[i] = domain.Ancestor{ID: uuid.NewV7(), Name: "L"}
		}
		return out
	}
	held, heldAsset := stepNodes("Plan")[0], stepNodes("Plan.png")[0]
	elsewhere := uuid.NewV7()
	for _, tt := range []struct {
		name   string
		sub    domain.Subtree
		parent *uuid.UUID
		line   []domain.Ancestor
		holder *domain.Node
		want   string
	}{
		{"free", sub, &elsewhere, line(2), nil, ""},
		{"to the root", sub, nil, nil, nil, ""},
		{"under itself", sub, &n.ID, line(1), nil, "page.cycle"},
		{"under its child, the name held, too deep", sub, &c.ID, line(9), &held, "page.cycle"},
		{"the name held by a page, too deep", sub, &elsewhere, line(9), &held, "taken page"},
		{"the name held by an attachment", sub, &elsewhere, line(1), &heldAsset, "taken attachment"},
		{"its children past the tenth level", sub, &elsewhere, line(9), nil, "page.too_deep"},
		{"its attachment no level", withImage, &elsewhere, line(9), nil, ""},
	} {
		err := domain.RelocationOf(tt.sub, tt.parent, tt.line, tt.holder)
		var taken *domain.PathTaken
		got := ""
		switch {
		case errors.As(err, &taken) && errors.Is(err, domain.ErrTitleTaken):
			got = map[bool]string{false: "taken page", true: "taken attachment"}[taken.Asset]
		case errors.Is(err, domain.ErrCycle):
			got = "page.cycle"
		case errors.Is(err, domain.ErrTooDeep):
			got = "page.too_deep"
		case err != nil:
			got = err.Error()
		}
		if got != tt.want {
			t.Errorf("%s: RelocationOf = %q, want %q", tt.name, got, tt.want)
		}
	}
}
