// Package domain holds the workspace module's rules: the slug and the name
// of a workspace, the reserved slugs, the module's actions and errors.
package domain

import (
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// MaxNameLength is the longest workspace name, in characters.
const MaxNameLength = 80

// Workspace is a workspace as the API shows it.
type Workspace struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CheckName checks a workspace's new name (422), by CheckDraft's rule for
// it, and returns it trimmed.
func CheckName(name string) (string, error) {
	name, f := shared.CheckName("name", name, MaxNameLength)
	if f != nil {
		return "", shared.Invalid(*f)
	}
	return name, nil
}

// Draft is a new workspace's checked values.
type Draft struct {
	Name string
	Slug string
}

// CheckDraft checks a new workspace's name and slug, every problem at
// once (422): the name trimmed, 1–80 characters, none that changes how
// the text around it reads; the slug spelled as one and not reserved.
func CheckDraft(name, slug string) (Draft, error) {
	var problems []shared.FieldError
	name, f := shared.CheckName("name", name, MaxNameLength)
	if f != nil {
		problems = append(problems, *f)
	}
	if f := checkSlug(slug); f != nil {
		problems = append(problems, *f)
	}
	if problems != nil {
		return Draft{}, shared.Invalid(problems...)
	}
	return Draft{Name: name, Slug: slug}, nil
}
