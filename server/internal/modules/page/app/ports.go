// Package app holds the page module's use cases and the ports they need
// (M4 design 4; M4/P1 design 3.6): the write unit every page write runs in,
// and the reads.
package app

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// ErrNotFound is what a repository returns for a missing row.
var ErrNotFound = errors.New("not found")

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// Content is a page's content as a write leaves it.
type Content struct {
	NodeID   uuid.UUID
	Content  string
	Revision int
	Hash     []byte // SHA-256 of Content
	ByteSize int
	By       uuid.UUID
	At       time.Time
}

// ContentMeta is what a page tells of its content besides the content.
type ContentMeta struct {
	Revision  int
	ByteSize  int
	UpdatedBy uuid.UUID
	UpdatedAt time.Time
}

// Changeset is a write's entry in its notebook's log.
type Changeset struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	Kind       string
	Client     domain.Client
	Message    *string
	By         uuid.UUID
	At         time.Time
}

// Item is a node's change in a changeset; Revision is the content's version
// in it.
type Item struct {
	ID          uuid.UUID
	ChangesetID uuid.UUID
	Change      domain.Change
	At          time.Time
}

// Revision is a page's content as a changeset leaves it.
type Revision struct {
	ID          uuid.UUID
	ChangesetID uuid.UUID
	NodeID      uuid.UUID
	Base        *int // nil: the changeset created the page
	Revision    int
	Content     string
	Hash        []byte
	ByteSize    int
	At          time.Time
}

// Nodes reads a notebook's nodes: unlocked for a read, under the write
// unit's locks for a write. ErrNotFound for a missing or deleted node.
type Nodes interface {
	FindNode(ctx context.Context, id uuid.UUID) (domain.Node, error)
	// FindNodeIn is FindNode within the notebook.
	FindNodeIn(ctx context.Context, notebookID, id uuid.UUID) (domain.Node, error)
	ListNodes(ctx context.Context, notebookID uuid.UUID) ([]domain.Node, error)
	// Children are a parent's children not deleted, the root's when
	// parentID is nil, in order.
	Children(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID) ([]domain.Node, error)
	// Ancestors are a node's ancestors from the root down to its parent.
	Ancestors(ctx context.Context, id uuid.UUID) ([]domain.Ancestor, error)
	ContentMeta(ctx context.Context, id uuid.UUID) (ContentMeta, error)
}

// NodeWriter writes nodes in the write unit's transaction. A name its
// siblings hold already is domain.ErrTitleTaken.
type NodeWriter interface {
	CreateNode(ctx context.Context, n domain.Node) error
	CreateContent(ctx context.Context, c Content) error
	RenameNode(ctx context.Context, n domain.Node) error
	SetSortOrder(ctx context.Context, id uuid.UUID, order float64) error
}

// ChangesetWriter records a write's changeset, its items and its versions.
type ChangesetWriter interface {
	CreateChangeset(ctx context.Context, c Changeset) error
	// RecordItem inserts the item, or moves the after of the node's item in
	// the changeset on, keeping its before.
	RecordItem(ctx context.Context, it Item) error
	// RecordRevision inserts the version, or updates the page's in the
	// changeset, keeping its base.
	RecordRevision(ctx context.Context, r Revision) error
}

// NotebookPages deletes the pages of deleted notebooks.
type NotebookPages interface {
	DeleteNotebooksPages(ctx context.Context, ids []uuid.UUID, by uuid.UUID, at time.Time) error
}
