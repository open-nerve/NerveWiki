package domain_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// tree builds nodes from lines "path kind [empty]": a path's last name is
// the node's, its folders its ancestors, in the order given (their sort
// order). kind is page or asset; a page has content unless empty.
type tree struct {
	nodes []domain.Node
	ids   map[string]uuid.UUID
}

func newTree(lines ...string) *tree {
	t := &tree{ids: map[string]uuid.UUID{}}
	for i, line := range lines {
		fields := strings.Fields(line)
		path, kind := fields[0], fields[1]
		path = strings.ReplaceAll(path, "_", " ")
		id := uuid.NewV7()
		t.ids[path] = id
		n := domain.Node{ID: id, Asset: kind == "asset", Name: path[strings.LastIndex(path, "/")+1:], SortOrder: float64(i),
			Modified: time.Unix(0, 0)}
		if !n.Asset && (len(fields) < 3 || fields[2] != "empty") {
			n.Bytes = 1
		}
		if i := strings.LastIndex(path, "/"); i >= 0 {
			parent := t.ids[path[:i]]
			n.ParentID = &parent
		}
		t.nodes = append(t.nodes, n)
	}
	return t
}

func (t *tree) linked(paths ...string) func(uuid.UUID) bool {
	return func(id uuid.UUID) bool {
		for _, p := range paths {
			if t.ids[p] == id {
				return true
			}
		}
		return false
	}
}

func nothingLinked(uuid.UUID) bool { return false }

// entries are a plan's entries as "path file", file "-" for none.
func entries(p *domain.Plan) []string {
	var out []string
	for _, e := range p.Entries {
		file := e.File
		if file == "" {
			file = "-"
		}
		out = append(out, e.Path+" "+file)
	}
	return out
}

// notebook is the notebook of the plans.
func notebook() domain.Named {
	return domain.Named{ID: uuid.MustParse("0199a2b4-0000-7000-8000-0000000000aa"), Name: "Notes"}
}

// The mapping of v0.1 design 3.5, rule by rule.
func TestThePlanMapsTheTree(t *testing.T) {
	cases := []struct {
		name   string
		tree   *tree
		linked []string
		want   []string
	}{
		{"a page with content is its file", newTree("A page"), nil, []string{"A.md A.md"}},
		{"a page without content and children is an empty file", newTree("A page empty"), nil, []string{"A.md A.md"}},
		{"a page with content and children is its file and its folder",
			newTree("A page", "A/B page"), nil, []string{"A.md A.md", "A/B.md A/B.md"}},
		{"a page without content with children is only its folder",
			newTree("A page empty", "A/B page"), nil, []string{"A/ -", "A/B.md A/B.md"}},
		{"unless a link leads to it", newTree("A page empty", "A/B page", "C page"), []string{"A"},
			[]string{"A.md A.md", "A/B.md A/B.md", "C.md C.md"}},
		{"an attachment is a file of its name in its page's folder",
			newTree("A page", "A/x.png asset"), nil, []string{"A.md A.md", "A/x.png A/x.png"}},
		{"an attachment at the root is in the vault's root", newTree("x.png asset", "A page"), nil,
			[]string{"x.png x.png", "A.md A.md"}},
		{"an attachment makes its page a folder", newTree("A page empty", "A/x.png asset"), nil,
			[]string{"A/ -", "A/x.png A/x.png"}},
		{"deep", newTree("A page empty", "A/B page empty", "A/B/C page", "A/B/C/x.pdf asset"), nil,
			[]string{"A/ -", "A/B/ -", "A/B/C.md A/B/C.md", "A/B/C/x.pdf A/B/C/x.pdf"}},
		{"a page named N.md with children is renamed beside the page N",
			newTree("N page", "N.md page empty", "N.md/c page"), nil,
			[]string{"N.md N.md", "N.md 2/ -", "N.md 2/c.md N.md 2/c.md"}},
		{"its file too", newTree("N page", "N.md page", "N.md/c page"), nil,
			[]string{"N.md N.md", "N.md 2.md N.md 2.md", "N.md 2/c.md N.md 2/c.md"}},
		{"by title key", newTree("n page", "N.MD page empty", "N.MD/c page"), nil,
			[]string{"n.md n.md", "N.MD 2/ -", "N.MD 2/c.md N.MD 2/c.md"}},
		{"by full case folding", newTree("ß page", "SS.md page empty", "SS.md/c page"), nil,
			[]string{"ß.md ß.md", "SS.md 2/ -", "SS.md 2/c.md SS.md 2/c.md"}},
		{"the next number free", newTree("N page", "N.md_2 page", "N.md page empty", "N.md/c page"), nil,
			[]string{"N.md N.md", "N.md 2.md N.md 2.md", "N.md 3/ -", "N.md 3/c.md N.md 3/c.md"}},
		{"not where a renamed one's file would be a folder", newTree("N page", "N.md page", "N.md/c page", "N.md_2.md page empty", "N.md_2.md/d page"), nil,
			[]string{"N.md N.md", "N.md 3.md N.md 3.md", "N.md 3/c.md N.md 3/c.md", "N.md 2.md/ -", "N.md 2.md/d.md N.md 2.md/d.md"}},
		{"a folder alone may be where a file is not", newTree("N page", "N.md page empty", "N.md/c page", "N.md_2.md page empty", "N.md_2.md/d page"), nil,
			[]string{"N.md N.md", "N.md 2/ -", "N.md 2/c.md N.md 2/c.md", "N.md 2.md/ -", "N.md 2.md/d.md N.md 2.md/d.md"}},
		{"a page without children has no folder to clash", newTree("N page", "N.md page"), nil,
			[]string{"N.md N.md", "N.md.md N.md.md"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := domain.NewPlan(notebook(), nil, c.tree.nodes, c.tree.linked(c.linked...))
			if err != nil {
				t.Fatal(err)
			}
			if got := entries(p); !slices.Equal(got, c.want) {
				t.Errorf("entries =\n%q\nwant\n%q", got, c.want)
			}
			if p.Root != "Notes" {
				t.Errorf("root = %q, want the notebook's name", p.Root)
			}
		})
	}
}

// A renamed node is reported with where it would have been and where it
// is: a page's file, or its folder when it has none; in a renamed folder,
// where it would have been had none been renamed. A folder's renames come
// before its subfolders'.
func TestThePlanReportsTheRenamed(t *testing.T) {
	tr := newTree("N page", "N.md page empty", "N.md/c page", "N.md/c.md page empty", "N.md/c.md/d page", "M page", "M.md page", "M.md/c page")
	p, err := domain.NewPlan(notebook(), nil, tr.nodes, nothingLinked)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.Problem{
		{Path: "N.md/", Code: domain.ProblemRenamed, To: "N.md 2/"},
		{Path: "M.md.md", Code: domain.ProblemRenamed, To: "M.md 2.md"},
		{Path: "N.md/c.md/", Code: domain.ProblemRenamed, To: "N.md 2/c.md 2/"},
	}
	if !slices.Equal(p.Renamed, want) {
		t.Errorf("renamed = %+v, want %+v", p.Renamed, want)
	}
}

// A page whose file's name would pass 255 bytes is renamed, its name cut
// at a character so that the number and ".md" fit; an attachment's name
// fits as it is, and so does a page's folder.
func TestThePlanRenamesAPageWhoseFileIsTooLong(t *testing.T) {
	long := strings.Repeat("名", 84) // 252 bytes: its file is 255
	longer := strings.Repeat("名", 85)
	tr := newTree(long+" page", longer+" page", longer+"/c page", strings.Repeat("a", 251)+".png asset")
	p, err := domain.NewPlan(notebook(), nil, tr.nodes, nothingLinked)
	if err != nil {
		t.Fatal(err)
	}
	cut := strings.Repeat("名", 83) // 249 bytes, then " 2" and ".md": 254
	want := []string{long + ".md " + long + ".md", cut + " 2.md " + cut + " 2.md", cut + " 2/c.md " + cut + " 2/c.md",
		strings.Repeat("a", 251) + ".png " + strings.Repeat("a", 251) + ".png"}
	if got := entries(p); !slices.Equal(got, want) {
		t.Errorf("entries =\n%q\nwant\n%q", got, want)
	}
	for _, e := range p.Entries {
		for _, name := range strings.Split(e.File, "/") {
			if len(name) > domain.MaxSegment {
				t.Errorf("%q has a name of %d bytes", e.File, len(name))
			}
		}
	}
}

// Siblings come by their order, then id, whatever order the nodes come in;
// each folder's entries follow its page.
func TestThePlanOrdersSiblings(t *testing.T) {
	tr := newTree("A page", "A/a page", "B page", "C page")
	tr.nodes[0].SortOrder, tr.nodes[2].SortOrder, tr.nodes[3].SortOrder = 5, 1, 1
	reversed := slices.Clone(tr.nodes)
	slices.Reverse(reversed)
	p, err := domain.NewPlan(notebook(), nil, reversed, nothingLinked)
	if err != nil {
		t.Fatal(err)
	}
	// B and C tie: B's id is the lower (UUIDv7, made first).
	want := []string{"B.md B.md", "C.md C.md", "A.md A.md", "A/a.md A/a.md"}
	if got := entries(p); !slices.Equal(got, want) {
		t.Errorf("entries = %q, want %q", got, want)
	}
}

// A subtree's archive is named after its page, which is a page at the
// vault's root: its file and its folder there. Nodes outside are left.
func TestThePlanOfASubtree(t *testing.T) {
	tr := newTree("A page", "A/B page", "A/B/x.png asset", "A/B/C page empty", "D page")
	root := domain.Named{ID: tr.ids["A/B"], Name: "B"}
	want := []string{"B.md B.md", "B/x.png B/x.png", "B/C.md B/C.md"}
	for name, nodes := range map[string][]domain.Node{"its nodes": tr.nodes[1:4], "every node": tr.nodes} {
		p, err := domain.NewPlan(notebook(), &root, nodes, nothingLinked)
		if err != nil {
			t.Fatal(err)
		}
		if got := entries(p); p.Root != "B" || !slices.Equal(got, want) {
			t.Errorf("%s: root %q, entries %q; want B, %q", name, p.Root, got, want)
		}
	}
	if _, err := domain.NewPlan(notebook(), &root, tr.nodes[2:4], nothingLinked); err == nil {
		t.Error("NewPlan() without its root = nil error")
	}
	asset := domain.Named{ID: tr.ids["A/B/x.png"], Name: "x.png"}
	if _, err := domain.NewPlan(notebook(), &asset, tr.nodes[2:3], nothingLinked); err == nil {
		t.Error("NewPlan() of an attachment = nil error")
	}
}

// Two siblings of one title key break the tree's rule: no archive with
// two entries of one name.
func TestThePlanRefusesSiblingsOfOneKey(t *testing.T) {
	tr := newTree("A page", "a page")
	if _, err := domain.NewPlan(notebook(), nil, tr.nodes, nothingLinked); err == nil {
		t.Error("NewPlan() = nil error, want the duplicate refused")
	}
}

// A contributor's file goes where nothing of the export is.
func TestContributedFilesGoWhereNothingIs(t *testing.T) {
	tr := newTree("A page", "A/B page", "x asset", "N page empty", "N/c page")
	for _, c := range []struct {
		path string
		ok   bool
	}{
		{"index.md", true},
		{"A/index.md", true},
		{"New/deep/log.md", true},
		{"A.md", false},      // a page's file
		{"a.MD", false},      // by title key
		{"A", false},         // a page's folder
		{"N", false},         // a folder-only page's
		{"x/y.md", false},    // in a file
		{"A.md/y.md", false}, // in a page's file
		{".nerve/x.md", false},
		{"A//b.md", false},
		{"../b.md", false},
		{"a:b.md", false},
		{" index.md", false},
		{"", false},
	} {
		p, err := domain.NewPlan(notebook(), nil, tr.nodes, nothingLinked)
		if err != nil {
			t.Fatal(err)
		}
		err = p.Contribute(c.path)
		if (err == nil) != c.ok || err != nil && !errors.Is(err, domain.ErrContributorConflict) {
			t.Errorf("Contribute(%q) = %v, want ok %v", c.path, err, c.ok)
		}
	}
	p, _ := domain.NewPlan(notebook(), nil, tr.nodes, nothingLinked)
	for _, path := range []string{"New/log.md", "new/LOG.md", "New", "New/log.md/x"} {
		err := p.Contribute(path)
		if (err == nil) != (path == "New/log.md") {
			t.Errorf("after New/log.md, Contribute(%q) = %v", path, err)
		}
	}
	if got := p.Contributed(); !slices.Equal(got, []string{"New/log.md"}) {
		t.Errorf("Contributed() = %q", got)
	}
}
