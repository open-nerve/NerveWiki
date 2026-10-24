// Package domain holds the notebook module's rules: a notebook's name, its
// workspace access and its mode, the module's actions and errors.
package domain

import (
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Mode is a notebook's mode (M10 design 4.3): standard, or llm_wiki. An
// upgrade's states are M11's own, never a mode.
type Mode string

const (
	ModeStandard Mode = "standard"
	ModeLLMWiki  Mode = "llm_wiki"
)

// Locale is the language of an LLM Wiki's template (M10 design 4.4): the
// web sends its own; en when a request gives none.
type Locale string

const (
	LocaleEn   Locale = "en"
	LocaleZhCN Locale = "zh-CN"
)

// Notebook is a notebook not deleted.
type Notebook struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Name        string
	Access      shared.WorkspaceAccess
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// Ownerless is set while the notebook has no active admin (M3 design 4).
	Ownerless *Ownerless
	// Mode is the notebook's mode; SourceRootID an LLM Wiki's source root,
	// the zero id for a standard notebook (notebooks_source_root_check).
	Mode         Mode
	SourceRootID uuid.UUID
}

// Draft is a new notebook's checked values.
type Draft struct {
	Name   string
	Access shared.WorkspaceAccess
}

// CheckDraft checks a new notebook's name and workspace access, every
// problem at once (422): the name by shared.CheckTitle, the access one of
// the three; none when access is nil.
func CheckDraft(name string, access *string) (Draft, error) {
	var problems []shared.FieldError
	name, f := shared.CheckTitle("name", name)
	if f != nil {
		problems = append(problems, *f)
	}
	d := Draft{Name: name, Access: shared.AccessNone}
	if access != nil {
		if f := checkAccess(*access); f != nil {
			problems = append(problems, *f)
		}
		d.Access = shared.WorkspaceAccess(*access)
	}
	if problems != nil {
		return Draft{}, shared.Invalid(problems...)
	}
	return d, nil
}

// Change is an update's checked values: nil leaves the field as it is.
type Change struct {
	Name   *string
	Access *shared.WorkspaceAccess
}

// CheckChange checks an update's fields that are present, every problem at
// once (422), as CheckDraft does.
func CheckChange(name, access *string) (Change, error) {
	var (
		c        Change
		problems []shared.FieldError
	)
	if name != nil {
		n, f := shared.CheckTitle("name", *name)
		if f != nil {
			problems = append(problems, *f)
		}
		c.Name = &n
	}
	if access != nil {
		if f := checkAccess(*access); f != nil {
			problems = append(problems, *f)
		}
		a := shared.WorkspaceAccess(*access)
		c.Access = &a
	}
	if problems != nil {
		return Change{}, shared.Invalid(problems...)
	}
	return c, nil
}

// Apply returns n with c's fields, and whether that changes it.
func (n Notebook) Apply(c Change) (Notebook, bool) {
	changed := n
	if c.Name != nil {
		changed.Name = *c.Name
	}
	if c.Access != nil {
		changed.Access = *c.Access
	}
	return changed, changed != n
}

// checkAccess is an access's problem, or nil. The contract's enum is not
// checked before: a body's structure check leaves values to the domain.
func checkAccess(access string) *shared.FieldError {
	if !slices.Contains(shared.WorkspaceAccesses(), shared.WorkspaceAccess(access)) {
		return &shared.FieldError{Field: "workspace_access", Code: shared.FieldInvalidFormat, Message: "must be none, viewer or editor"}
	}
	return nil
}
