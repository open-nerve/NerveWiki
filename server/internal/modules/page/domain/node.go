// Package domain holds the page module's rules (v0.1 design 3.5, 3.8; M4
// design): the tree of a notebook's nodes, their titles and order, and the
// changes a write makes.
package domain

import (
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Kind is what a node is: a page, or from M7 an attachment.
type Kind string

// The two kinds.
const (
	KindPage  Kind = "page"
	KindAsset Kind = "asset"
)

// MaxDepth is how deep pages nest: a root is at depth 1 (v0.1 design 3.5).
const MaxDepth = 10

// Node is a page or an attachment in a notebook's tree. A root has no
// parent; a parent is in the same notebook.
type Node struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	ParentID   *uuid.UUID
	Kind       Kind
	Name       string
	NameKey    string
	SortOrder  float64
	CreatedBy  uuid.UUID
	UpdatedBy  uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// State is where the node is in the tree.
func (n Node) State() TreeState {
	return TreeState{ParentID: n.ParentID, Name: n.Name, SortOrder: n.SortOrder}
}

// Title is a page's title, or an attachment's file name, with the key it
// compares by.
type Title struct {
	Name string
	Key  string
}

// CheckTitle checks s as field: the title rules of shared.CheckTitle, and
// the key of shared.TitleKey. A title that breaks a rule is 422 on field.
func CheckTitle(field, s string) (Title, error) {
	name, f := shared.CheckTitle(field, s)
	if f != nil {
		return Title{}, shared.Invalid(*f)
	}
	return Title{Name: name, Key: shared.TitleKey(name)}, nil
}

// Ancestor is a node on the way from a node up to its notebook's root.
type Ancestor struct {
	ID   uuid.UUID
	Name string
}
