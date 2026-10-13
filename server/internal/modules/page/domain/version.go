package domain

import (
	"math"
	"time"
	"uuid"
)

// MaxRevision is the largest revision a page's content may have: its
// column's integer. No version holds one beyond it.
const MaxRevision = math.MaxInt32

// Version is a page's version (M8 design 4.3): a row of its versions,
// which holds its content as a changeset left it. It is addressed by the
// revision it wrote; an edit session's saves move that on, so it holds
// every revision in (Base, Revision]. Base is nil for the version that
// created the page. Editing marks the page's newest version while the edit
// session whose changeset it is is alive: its revision may move on.
type Version struct {
	ID        uuid.UUID
	Base      *int
	Revision  int
	ByteSize  int
	CreatedAt time.Time
	UpdatedAt time.Time
	Editing   bool
	Changeset VersionChangeset
}

// Holds reports whether the version holds revision: no version holds one
// below 1.
func (v Version) Holds(revision int) bool {
	return revision >= 1 && revision <= v.Revision && (v.Base == nil || *v.Base < revision)
}

// VersionChangeset is the changeset a version is of: its kind, its client,
// who made it, with their display name, which the reads fill, and when.
type VersionChangeset struct {
	ID        uuid.UUID
	Kind      ChangesetKind
	Client    Client
	By        uuid.UUID
	ByName    string
	CreatedAt time.Time
}

// VersionCursor is a page of a page's versions' cursor: the next page
// lists those below the revision Below, the last one listed.
type VersionCursor struct {
	Below int `json:"below"`
}
