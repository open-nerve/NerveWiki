package domain

import (
	"slices"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// MaxSlugLength is the longest slug.
const MaxSlugLength = 48

// Why a slug cannot be taken, as checkWorkspaceSlug answers it.
const (
	SlugInvalid  = "invalid"
	SlugReserved = "reserved"
	SlugTaken    = "taken"
)

// slugPattern reports whether s is spelled as a slug: 1–48 of a–z, 0–9, _
// and - (v0.1 design 3.2). There is no folding: an upper-case or padded slug
// is refused, not quietly changed into another address.
func slugPattern(s string) bool {
	if s == "" || len(s) > MaxSlugLength {
		return false
	}
	for _, c := range []byte(s) {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

// SlugProblem is why slug cannot name a new workspace, before the
// database is asked whether one has it: SlugInvalid, SlugReserved, or ""
// when it can.
func SlugProblem(slug string) string {
	switch {
	case !slugPattern(slug):
		return SlugInvalid
	case slices.Contains(Reserved().All(), slug):
		return SlugReserved
	}
	return ""
}

// checkSlug is SlugProblem as a field problem of the slug field.
func checkSlug(slug string) *shared.FieldError {
	switch SlugProblem(slug) {
	case SlugInvalid:
		return &shared.FieldError{Field: "slug", Code: shared.FieldInvalidFormat,
			Message: "must be 1–48 lower-case letters, digits, _ or -"}
	case SlugReserved:
		return &shared.FieldError{Field: "slug", Code: shared.FieldNotAllowed, Message: "is reserved"}
	}
	return nil
}
