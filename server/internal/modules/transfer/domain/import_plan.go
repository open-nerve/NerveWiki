package domain

import (
	"cmp"
	"slices"
	"strings"
	"unicode"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// MaxDepth is how deep pages nest, a root page at 1 (v0.1 design 3.5): an
// import skips its pages deeper from where it goes, which the page
// module's units refuse.
const MaxDepth = 10

// Untitled is the name of a node whose name mends to nothing (M7 design
// 4.11): stored, so not in the reader's language.
const Untitled = "未命名"

// ImportName is an import's job's name: the uploaded file's, mended as an
// attachment's name is, keeping its extension; Untitled for none.
func ImportName(file string) string {
	return cmp.Or(shared.FixTitle(file, true), Untitled)
}

// ImportNode is a node an import creates (M7/P6 design 3.12): its parent
// among the plan's nodes, -1 for where the import goes; an attachment or
// a page; its name mended, and as the archive gives it (a page's file's
// without ".md"); its path in the vault, which the report tells (a page's
// file, a page that is only a folder ending in "/", an attachment's
// file); the entry holding its content or its file, -1 for a page that is
// only a folder; and its level, the vault's top 1.
type ImportNode struct {
	Parent   int
	Asset    bool
	Name     string
	Original string
	Path     string
	Entry    int
	Level    int
}

// ImportPlan is where an import puts the archive's entries: its nodes,
// level by level, each level's by their parents' order and each parent's
// in their order; and the problems of the nodes too deep.
type ImportPlan struct {
	Nodes   []ImportNode
	Skipped []Problem
}

// NewImportPlan maps entries, an archive's to import (M7/P6 design 3.12),
// for a place at depth (0: the notebook's root). A file X.md is the page
// X; a folder X/ holds X's children, X being the first page of its folder
// whose name has its key, before either is mended, or a page without
// content when there is none; any other file is an attachment of the page
// whose folder holds it. Names are mended (shared.FixTitle), an empty one
// Untitled. Siblings come by meta's order, then by name; a page deeper
// than MaxDepth from depth is skipped, and everything under it. meta's
// contributed files are left out.
func NewImportPlan(entries []ImportEntry, meta ImportMeta, depth int) ImportPlan {
	root := newFolder("", "")
	for _, e := range entries {
		if !e.Folder && meta.Contributed[e.Joined()] {
			continue
		}
		f := root
		names := e.Path
		if !e.Folder {
			names = e.Path[:len(e.Path)-1]
		}
		for _, name := range names {
			f = f.sub(name)
		}
		if !e.Folder {
			f.files = append(f.files, e)
		}
	}
	var p ImportPlan
	type level struct {
		parent int
		folder *folder
	}
	at := []level{{parent: -1, folder: root}}
	for n := 1; len(at) > 0; n++ {
		var next []level
		for _, l := range at {
			for _, c := range l.folder.children(meta) {
				node := c.node
				node.Parent, node.Level = l.parent, n
				if !node.Asset && depth+n > MaxDepth {
					p.skip(c)
					continue
				}
				p.Nodes = append(p.Nodes, node)
				if c.folder != nil {
					next = append(next, level{parent: len(p.Nodes) - 1, folder: c.folder})
				}
			}
		}
		at = next
	}
	return p
}

// skip skips c, too deep, and everything in its folder.
func (p *ImportPlan) skip(c child) {
	p.Skipped = append(p.Skipped, Problem{Path: c.node.Path, Code: ProblemTooDeep})
	if c.folder == nil {
		return
	}
	for _, sub := range c.folder.children(ImportMeta{}) {
		p.skip(sub)
	}
}

// folder is a folder of the vault: its name and path, its folders in the
// order they first appear, its files in the archive's order.
type folder struct {
	name, path string
	folders    []*folder
	byName     map[string]*folder
	files      []ImportEntry
}

func newFolder(name, path string) *folder {
	return &folder{name: name, path: path, byName: map[string]*folder{}}
}

// sub is f's folder named name, made when it is not yet.
func (f *folder) sub(name string) *folder {
	if g, ok := f.byName[name]; ok {
		return g
	}
	path := name
	if f.path != "" {
		path = f.path + "/" + name
	}
	g := newFolder(name, path)
	f.byName[name] = g
	f.folders = append(f.folders, g)
	return g
}

// child is a node a folder holds, and the folder it holds when it is a
// page with children.
type child struct {
	node   ImportNode
	folder *folder
}

// children are f's nodes in their order: its pages' files, each holding
// the first folder of its key not yet held; the folders no file holds, as
// pages without content; its attachments.
func (f *folder) children(meta ImportMeta) []child {
	var out []child
	pages := map[string][]int{}
	for _, e := range f.files {
		name := e.Path[len(e.Path)-1]
		c := child{node: ImportNode{Path: e.Joined(), Entry: e.Index}}
		if stem, ok := pageStem(name); ok {
			c.node.Original, c.node.Name = stem, mended(stem, false)
			k := shared.TitleKey(stem)
			pages[k] = append(pages[k], len(out))
		} else {
			c.node.Asset, c.node.Original, c.node.Name = true, name, mended(name, true)
		}
		out = append(out, c)
	}
	for _, g := range f.folders {
		k := shared.TitleKey(g.name)
		if held := pages[k]; len(held) > 0 {
			out[held[0]].folder = g
			pages[k] = held[1:]
			continue
		}
		out = append(out, child{node: ImportNode{Original: g.name, Name: mended(g.name, false), Path: g.path + "/", Entry: -1}, folder: g})
	}
	slices.SortStableFunc(out, func(a, b child) int {
		ao, aok := meta.Order[a.node.Path]
		bo, bok := meta.Order[b.node.Path]
		switch {
		case aok && bok:
			return cmp.Or(cmp.Compare(ao, bo), strings.Compare(a.node.Path, b.node.Path))
		case aok != bok:
			if aok {
				return -1
			}
			return 1
		}
		return cmp.Or(strings.Compare(shared.TitleKey(a.node.Name), shared.TitleKey(b.node.Name)), strings.Compare(a.node.Name, b.node.Name),
			strings.Compare(a.node.Path, b.node.Path))
	})
	return out
}

// pageStem is the title of a page's file named name: name without the
// white space and dots at its ends, then without ".md" in any case; false
// when name is no page's file.
func pageStem(name string) (string, bool) {
	trimmed := strings.TrimFunc(name, func(r rune) bool { return unicode.IsSpace(r) || r == '.' })
	if len(trimmed) <= len(".md") || !strings.EqualFold(trimmed[len(trimmed)-len(".md"):], ".md") {
		return "", false
	}
	return trimmed[:len(trimmed)-len(".md")], true
}

// mended is name mended into a page's title, or an attachment's name,
// which keeps its extension and never ends with ".md"; Untitled for none.
func mended(name string, asset bool) string {
	s := shared.FixTitle(name, asset)
	switch {
	case s == "":
		return Untitled
	case asset && len(s) >= len(".md") && strings.EqualFold(s[len(s)-len(".md"):], ".md"):
		return shared.FixTitle(s+"_", true)
	}
	return s
}
