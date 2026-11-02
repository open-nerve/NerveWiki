package domain

import (
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// DefaultListLimit and MaxListLimit are MCP's list's page (M9 design 4.9).
const (
	DefaultListLimit = 200
	MaxListLimit     = 1000
)

// CheckListLimit is the page size limit asks for: DefaultListLimit when it
// is nil; 422 out_of_range on limit outside 1–MaxListLimit.
func CheckListLimit(limit *int) (int, error) {
	switch {
	case limit == nil:
		return DefaultListLimit, nil
	case *limit < 1 || *limit > MaxListLimit:
		return 0, shared.Invalid(shared.FieldError{Field: "limit", Code: shared.FieldOutOfRange,
			Message: fmt.Sprintf("must be between 1 and %d", MaxListLimit)})
	}
	return *limit, nil
}

// ChildrenCursor is where a list of children goes on (M9 design 4.9):
// after the real item of sort order O and id I, or generated root item V
// (1 for index, 2 for log). Zero V retains the original cursor bytes.
type ChildrenCursor struct {
	O float64   `json:"o"`
	I uuid.UUID `json:"i"`
	V int       `json:"v,omitempty"`
}

// GlobCursor is where a glob's list goes on: after the path key K. Neither
// binds the pattern: an item moved by a concurrent write is neither
// repeated nor skipped among the others.
type GlobCursor struct {
	K string `json:"k"`
}

// Check keeps virtual progress separate from the real sort/id domain.
func (c ChildrenCursor) Check() error {
	if c.V < 0 || c.V > 2 || c.V != 0 && (c.O != 0 || c.I != (uuid.UUID{})) {
		return shared.InvalidCursor()
	}
	return nil
}
