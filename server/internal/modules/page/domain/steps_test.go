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
			c, err := domain.CreationOf("path", path, tt.found)
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
		err := domain.MoveKinds(mustPath(t, tt.from), mustPath(t, tt.to))
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
		err := domain.DeletionOf(tt.sub, tt.recursive)
		var r *domain.Recursive
		switch {
		case !tt.refused && err != nil:
			t.Errorf("%s: DeletionOf = %v, want nil", tt.name, err)
		case tt.refused && (!errors.As(err, &r) || r.Pages != tt.pages || r.Assets != tt.assets):
			t.Errorf("%s: DeletionOf = %v, want *Recursive{%d, %d}", tt.name, err, tt.pages, tt.assets)
		}
	}
}
