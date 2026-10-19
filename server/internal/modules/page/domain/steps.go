package domain

import (
	"fmt"
	"strings"
	"uuid"
)

// The rules of each step an agent's write takes, which need no store: a
// function of a path and the nodes found for it, which the write tools'
// use cases call in their units, and a batch's plan on the tree it
// simulates (M9/P2 design 3.8, 3.20), so that each rule is written once.
// Each names its fields through the Args it is given (M9/P3 design 3.2).

// Creation is what writing a page at a path creates (M9 design 4.10):
// nothing when Leaf, the page the path names, is there; else a page of
// each of Titles, each under the one before, the first under Parent (nil
// for the notebook's root), the last the path's page.
type Creation struct {
	Leaf   *Node
	Parent *uuid.UUID
	Titles []string
}

// CreationOf is what writing a page at path creates, given the nodes Walk
// found from the root (M9 design 4.10): the leaf when it is there, else
// the segments to create, from the first missing one. 422 on a's path: a
// found node before the last segment that is an attachment, or a missing
// ancestor ending with .md (MissingAncestors); 409 page.title_taken
// (*PathTaken) for an attachment that holds the leaf's name, and
// page.too_deep for a path of more than MaxDepth pages.
func CreationOf(a Args, path Path, found []Node) (Creation, error) {
	last := len(path.Keys) - 1
	for i, n := range found {
		if i < last && n.Kind == KindAsset {
			return Creation{}, NotAllowed(a.Field("path"), fmt.Sprintf("segment %d is an attachment, which holds no pages", i+1))
		}
	}
	if len(found) == len(path.Keys) {
		leaf := found[last]
		if leaf.Kind == KindAsset {
			return Creation{}, &PathTaken{Path: path.Written, Asset: true}
		}
		return Creation{Leaf: &leaf}, nil
	}
	if err := MissingAncestors(a.Field("path"), path, len(found)); err != nil {
		return Creation{}, err
	}
	if len(path.Keys) > MaxDepth {
		return Creation{}, ErrTooDeep
	}
	c := Creation{Titles: path.Titles[len(found):]}
	if len(found) > 0 {
		parent := found[len(found)-1].ID
		c.Parent = &parent
	}
	return c, nil
}

// MissingAncestors is 422 on field when an ancestor of path that Walk did
// not find, from the found-th segment on, ends with .md: an ancestor is
// written by its title, and a page titled A.md is found by its title, so
// is no missing one. write's creation and move's parent both ask it, so
// that the mistake of a parent written as a page's path gets one answer,
// and not move's 404 with its hint to write the parent first (M9/P2 S1
// review B-N2).
func MissingAncestors(field string, path Path, found int) error {
	for i := found; i < len(path.Keys)-1; i++ {
		if t := path.Titles[i]; len(t) >= len(pageSuffix) && strings.EqualFold(t[len(t)-len(pageSuffix):], pageSuffix) {
			return NotAllowed(field, fmt.Sprintf("segment %d ends with .md: an ancestor is written by its title, without .md", i+1))
		}
	}
	return nil
}

// MoveKinds is 422 on a's to when from and to are not of one kind: a
// page's path to a page's, an attachment's to an attachment's.
func MoveKinds(a Args, from, to Path) error {
	if from.Kind != to.Kind {
		return NotAllowed(a.Field("to"),
			"is not of from's kind: a page moves to a page's path, ending with .md, an attachment to an attachment's")
	}
	return nil
}

// ParentChainOf is whether chain, the pages Walk found from the root
// towards the parent to asks for (on, the segment it found an attachment
// at, nil for none), is that parent's whole line, none for the root: the
// first segment that fails decides (M9/P2 design 3.8.3). One that names an
// attachment is 422 on a's to, an attachment holds no nodes; one that
// names nothing is page.not_found at to's parent's path, a move creating
// no parent pages, unless a segment that names nothing ends with .md:
// then write's 422 on a's to, the parent written as a page's path
// (MissingAncestors).
func ParentChainOf(a Args, to Path, chain []Node, on *AttachmentOnPath) error {
	keys := len(to.Keys) - 1
	holdsNoNodes := func(segment int) error {
		return NotAllowed(a.Field("to"), fmt.Sprintf("segment %d is an attachment, which holds no nodes", segment))
	}
	switch {
	case on != nil:
		return holdsNoNodes(on.Segment)
	case len(chain) < keys:
		if err := MissingAncestors(a.Field("to"), to, len(chain)); err != nil {
			return err
		}
		return &PathNotFound{Path: PathOfNames(to.Titles[:keys], KindPage), Hint: MoveParentHint, ParentOf: a.Field("to")}
	case len(chain) > 0 && chain[len(chain)-1].Kind != KindPage:
		return holdsNoNodes(len(chain))
	}
	return nil
}

// RenameTitle checks name as field, n's new name, by n's kind: a page's
// title (CheckTitle), an attachment's name, its extension kept
// (CheckAssetRename). Unit.Relocate, Unit.Rename and a batch's plan check
// a new name through it.
func RenameTitle(field string, n Node, name string) (Title, error) {
	if n.Kind == KindAsset {
		return CheckAssetRename(field, n.Name, name)
	}
	return CheckTitle(field, name)
}

// RelocationOf checks a node's move under another parent, as a unit and a
// batch's plan check it, in Move's order (M4/P1 design 3.6; M9/P2 design
// 3.5.1); a rename under its parent reads no subtree and is not one:
// parent in the node's subtree, page.cycle; holder, the sibling there
// that holds the new name's key, *PathTaken of its kind; the subtree's
// deepest page past MaxDepth under line, the parent's line from the root,
// page.too_deep.
func RelocationOf(sub Subtree, parent *uuid.UUID, line []Ancestor, holder *Node) error {
	if parent != nil && sub.Holds(*parent) {
		return ErrCycle
	}
	if holder != nil {
		return &PathTaken{Asset: holder.Kind == KindAsset}
	}
	// The subtree's deepest node lands Height - 1 levels below the node.
	if Depth(line)+sub.Height()-1 > MaxDepth {
		return ErrTooDeep
	}
	return nil
}

// DeletionOf is 422 on a's recursive (*Recursive) when sub has nodes under
// its root and recursive is false.
func DeletionOf(a Args, sub Subtree, recursive bool) error {
	if len(sub) > 1 && !recursive {
		pages, assets := sub[1:].Counts()
		return &Recursive{Field: a.Field("recursive"), Pages: pages, Assets: assets}
	}
	return nil
}
