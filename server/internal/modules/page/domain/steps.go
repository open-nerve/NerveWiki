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
// the segments to create, from the first missing one. 422 on field: a
// found node before the last segment that is an attachment, or a missing
// ancestor ending with .md; 409 page.title_taken (*PathTaken) for an
// attachment that holds the leaf's name, and page.too_deep for a path of
// more than MaxDepth pages.
func CreationOf(field string, path Path, found []Node) (Creation, error) {
	last := len(path.Keys) - 1
	for i, n := range found {
		if i < last && n.Kind == KindAsset {
			return Creation{}, NotAllowed(field, fmt.Sprintf("segment %d is an attachment, which holds no pages", i+1))
		}
	}
	if len(found) == len(path.Keys) {
		leaf := found[last]
		if leaf.Kind == KindAsset {
			return Creation{}, &PathTaken{Path: path.Written, Asset: true}
		}
		return Creation{Leaf: &leaf}, nil
	}
	for i := len(found); i < last; i++ {
		// A page titled A.md is found by its title: it is no missing one.
		if t := path.Titles[i]; len(t) >= len(pageSuffix) && strings.EqualFold(t[len(t)-len(pageSuffix):], pageSuffix) {
			return Creation{}, NotAllowed(field,
				fmt.Sprintf("segment %d ends with .md: an ancestor is written by its title, without .md", i+1))
		}
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

// MoveKinds is 422 on to when from and to are not of one kind: a page's
// path to a page's, an attachment's to an attachment's.
func MoveKinds(from, to Path) error {
	if from.Kind != to.Kind {
		return NotAllowed("to", "is not of from's kind: a page moves to a page's path, ending with .md, an attachment to an attachment's")
	}
	return nil
}

// DeletionOf is 422 on recursive (*Recursive) when sub has nodes under its
// root and recursive is false.
func DeletionOf(sub Subtree, recursive bool) error {
	if len(sub) > 1 && !recursive {
		pages, assets := sub[1:].Counts()
		return &Recursive{Pages: pages, Assets: assets}
	}
	return nil
}
