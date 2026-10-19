package domain

import (
	"math"
	"strconv"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// MaxRevision is the largest revision a page's content may have: its
// column's integer. No version holds one beyond it.
const MaxRevision = math.MaxInt32

// CheckRevision checks the revision of a version MCP's versions reads
// (M9/P2 design 3.16.1): 1 to MaxRevision, out_of_range on revision
// otherwise, before any read, as no page's version can hold it (REST's
// getPageVersion answers page.version_not_found for it, M8's).
func CheckRevision(revision int) error {
	if revision < 1 || revision > MaxRevision {
		return shared.Invalid(shared.FieldError{Field: "revision", Code: shared.FieldOutOfRange,
			Message: "must be from 1 to " + strconv.Itoa(MaxRevision)})
	}
	return nil
}

// CheckVersionsArgs checks that versions' arguments are those of one of
// its two calls (M9/P2 design 3.16.1): with revision, a version's read,
// cursor is the list's; without it, the list, offset and column are a
// read's. Each argument of the other call is 422 not_allowed on it, one
// field each: the call's mode is judged before anything else, so a call
// with such an argument reads none of its own, its cursor neither.
func CheckVersionsArgs(revision, offset, column *int, cursor *string) error {
	var fields []shared.FieldError
	misused := func(name string, set bool, message string) {
		if set {
			fields = append(fields, shared.FieldError{Field: name, Code: shared.FieldNotAllowed, Message: message})
		}
	}
	if revision != nil {
		misused("cursor", cursor != nil, "is for the list of versions, without revision")
	} else {
		misused("offset", offset != nil, "is for reading a version, with revision")
		misused("column", column != nil, "is for reading a version, with revision")
	}
	if len(fields) > 0 {
		return shared.Invalid(fields...)
	}
	return nil
}

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
