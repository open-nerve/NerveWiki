package domain

import (
	"bytes"
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Node is a node of an export's scope, as its snapshot reads it.
type Node struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
	// Asset tells an attachment from a page.
	Asset     bool
	Name      string
	SortOrder float64
	// Empty tells a page without content.
	Empty bool
	// Modified is when it was last written: its entry's time in the
	// archive.
	Modified time.Time
}

// Named is a notebook or a page, by id and name.
type Named struct {
	ID   uuid.UUID
	Name string
}

// Entry is where an export puts a node of its scope, relative to the
// vault, the archive's root folder.
type Entry struct {
	Node Node
	// Path is the node in the vault: a page's file, or its folder (ending in
	// "/") when it has none; an attachment's file.
	Path string
	// File is the file written for it: a page's .md or an attachment's
	// file; "" for a page that is only its folder.
	File string
}

// MaxSegment is the most bytes of one name of a path most file systems
// take: a page's file, its name and ".md", must fit.
const MaxSegment = 255

// ErrContributorConflict is a contributor's file where a node, another of
// the contributors' files or the export's own is, or not named as a node
// is named (M7/P5 design 3.10).
var ErrContributorConflict = errors.New("transfer: a contributed file conflicts with the export's")

// Plan is where an export puts each node of its scope (v0.1 design 3.5;
// M7/P5 design 3.9): the archive's root folder, named after the notebook
// or the page exported with its subtree, holds the vault. A page E in the
// folder D is the file D/E.md, its children are in D/E/; an attachment
// is a file of its name in its page's folder. A page without content that
// has children is only its folder, unless a link of the scope leads to
// it: Obsidian then finds it as Nerve does, an empty file; a page without
// content and without children is an empty file. Siblings come by their
// order, then id, as the tree lists them; a folder's entries depth first.
//
// A page whose folder would be a sibling's file (the page "N.md" with
// children beside the page N), or whose file's name would be longer than
// MaxSegment bytes, is exported under the first free of "<name> 2",
// "<name> 3"…, the name cut at a character so that its file fits: free by
// title key among the folder's names, files and folders, as file systems
// may not tell case. Renamed tells them.
type Plan struct {
	// Root is the name of the archive's root folder.
	Root    string
	Entries []Entry
	Renamed []Problem
	// files and folders are the vault's paths taken, by their keys: the
	// nodes', and the contributors' as they add theirs.
	files, folders map[string]bool
	contributed    []string
}

// NewPlan maps nodes, the scope's not deleted, of notebook, or of the page
// root and its subtree when root is set: root is then among nodes, a page.
// linked tells the pages a link of the scope leads to. A node whose parent
// is not among nodes is out of the scope.
func NewPlan(notebook Named, root *Named, nodes []Node, linked func(id uuid.UUID) bool) (*Plan, error) {
	children := map[uuid.UUID][]Node{}
	var tops []Node
	for _, n := range nodes {
		switch {
		case root != nil && n.ID == root.ID:
			if n.Asset {
				return nil, errors.New("transfer: the root exported is an attachment")
			}
			tops = append(tops, n)
		case root == nil && n.ParentID == nil:
			tops = append(tops, n)
		case n.ParentID != nil:
			children[*n.ParentID] = append(children[*n.ParentID], n)
		}
	}
	p := &Plan{Root: notebook.Name, files: map[string]bool{}, folders: map[string]bool{"": true}}
	if root != nil {
		if len(tops) != 1 {
			return nil, errors.New("transfer: the root exported is not among the nodes")
		}
		p.Root = root.Name
	}
	for _, kids := range children {
		slices.SortFunc(kids, bySiblingOrder)
	}
	slices.SortFunc(tops, bySiblingOrder)
	if err := p.place("", tops, children, linked); err != nil {
		return nil, err
	}
	return p, nil
}

// bySiblingOrder orders siblings as the tree does: by sort order, then id.
func bySiblingOrder(a, b Node) int {
	return cmp.Or(cmp.Compare(a.SortOrder, b.SortOrder), bytes.Compare(a.ID[:], b.ID[:]))
}

// placed is a sibling as its folder places it: the name it is exported
// under, and whether it is written as a file and holds a folder.
type placed struct {
	node         Node
	name         string
	file, folder bool
}

// fileName is the name of x's file: a page's is its name and ".md".
func (x placed) fileName() string {
	if x.node.Asset {
		return x.name
	}
	return x.name + ".md"
}

// path is x's entry's path in folder: its file, or its folder.
func (x placed) path(folder string) string {
	if x.file {
		return folder + x.fileName()
	}
	return folder + x.name + "/"
}

// place adds the entries of siblings, in folder ("" or ending in "/"), and
// of their subtrees. Siblings share a namespace: two of one title key
// break the tree's rule, and the export with it.
func (p *Plan) place(folder string, siblings []Node, children map[uuid.UUID][]Node, linked func(uuid.UUID) bool) error {
	names := map[string]bool{}
	xs := make([]placed, len(siblings))
	for i, s := range siblings {
		k := shared.TitleKey(s.Name)
		if names[k] {
			return errors.New("transfer: two siblings of one title key")
		}
		names[k] = true
		kids := len(children[s.ID]) > 0
		xs[i] = placed{node: s, name: s.Name, folder: !s.Asset && kids, file: s.Asset || !s.Empty || !kids || linked(s.ID)}
	}
	files, folders := map[string]bool{}, map[string]bool{}
	for _, x := range xs {
		if x.file {
			files[shared.TitleKey(x.fileName())] = true
		}
		if x.folder {
			folders[shared.TitleKey(x.name)] = true
		}
	}
	for i, x := range xs {
		long := !x.node.Asset && x.file && len(x.fileName()) > MaxSegment
		clash := x.folder && files[shared.TitleKey(x.name)]
		if !long && !clash {
			continue
		}
		from := x.path(folder)
		if x.file {
			delete(files, shared.TitleKey(x.fileName()))
		}
		if x.folder {
			delete(folders, shared.TitleKey(x.name))
		}
		x.name = freeName(x, names, files, folders)
		names[shared.TitleKey(x.name)] = true
		if x.file {
			files[shared.TitleKey(x.fileName())] = true
		}
		if x.folder {
			folders[shared.TitleKey(x.name)] = true
		}
		xs[i] = x
		p.Renamed = append(p.Renamed, Problem{Path: from, Code: ProblemRenamed, To: x.path(folder)})
	}
	for _, x := range xs {
		e := Entry{Node: x.node, Path: x.path(folder)}
		if x.file {
			e.File = folder + x.fileName()
			p.files[pathKey(e.File)] = true
		}
		p.Entries = append(p.Entries, e)
		if x.folder {
			sub := folder + x.name + "/"
			p.folders[pathKey(strings.TrimSuffix(sub, "/"))] = true
			if err := p.place(sub, children[x.node.ID], children, linked); err != nil {
				return err
			}
		}
	}
	return nil
}

// freeName is the first of "<name> 2", "<name> 3"… for the page x, the
// name cut so that its file fits MaxSegment bytes, whose key is no name,
// file or folder of its folder, nor its file's.
func freeName(x placed, names, files, folders map[string]bool) string {
	taken := func(s string) bool {
		k := shared.TitleKey(s)
		return names[k] || files[k] || folders[k]
	}
	for n := 2; ; n++ {
		suffix := " " + strconv.Itoa(n)
		limit := MaxSegment - len(suffix)
		if x.file {
			limit -= len(".md")
		}
		name := cutAt(x.node.Name, limit) + suffix
		if taken(name) || x.file && taken(name+".md") {
			continue
		}
		return name
	}
}

// cutAt is s cut to at most n bytes at a character's boundary.
func cutAt(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// pathKey is a path of the vault by the title keys of its names, as file
// systems that do not tell case compare it.
func pathKey(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = shared.TitleKey(part)
	}
	return strings.Join(parts, "/")
}

// Contribute takes path, a contributor's file in the vault, for the
// archive: names of a node's rules joined by "/", none of them the
// export's own (.nerve, which no node's name begins with a dot), not where
// a node's or another contributed file is, not in a folder that is a file,
// not a folder itself. Otherwise it returns ErrContributorConflict.
func (p *Plan) Contribute(path string) error {
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if checked, err := shared.CheckTitle("path", part); err != nil || checked != part {
			return ErrContributorConflict
		}
	}
	k := pathKey(path)
	if p.files[k] || p.folders[k] {
		return ErrContributorConflict
	}
	for i := 1; i < len(parts); i++ {
		if p.files[pathKey(strings.Join(parts[:i], "/"))] {
			return ErrContributorConflict
		}
	}
	p.files[k] = true
	for i := 1; i < len(parts); i++ {
		p.folders[pathKey(strings.Join(parts[:i], "/"))] = true
	}
	p.contributed = append(p.contributed, path)
	return nil
}

// Contributed are the contributors' files, in the order they were added.
func (p *Plan) Contributed() []string {
	return slices.Clone(p.contributed)
}
