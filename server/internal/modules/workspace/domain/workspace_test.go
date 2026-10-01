package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestCheckDraftTrimsTheName(t *testing.T) {
	d, err := CheckDraft("  Acme 研发 \n", "acme")
	if err != nil || d != (Draft{Name: "Acme 研发", Slug: "acme"}) {
		t.Errorf("CheckDraft() = %+v, %v", d, err)
	}
	if _, err := CheckDraft(strings.Repeat("名", 80), "acme"); err != nil {
		t.Errorf("CheckDraft(80 characters) = %v", err)
	}
}

func TestCheckDraftListsEveryProblem(t *testing.T) {
	tests := []struct {
		name, wsName, slug string
		want               []shared.FieldError
	}{
		{"both wrong", " ", "Acme", []shared.FieldError{
			{Field: "name", Code: shared.FieldRequired, Message: "is required"},
			{Field: "slug", Code: shared.FieldInvalidFormat, Message: "must be 1–48 lower-case letters, digits, _ or -"},
		}},
		{"name too long", strings.Repeat("a", 81), "acme", []shared.FieldError{
			{Field: "name", Code: shared.FieldTooLong, Message: "must be at most 80 characters"},
		}},
		{"a bidirectional control", "Acme\u202e", "acme", []shared.FieldError{
			{Field: "name", Code: shared.FieldInvalidFormat, Message: "must not contain control characters"},
		}},
		{"reserved", "Acme", "settings", []shared.FieldError{
			{Field: "slug", Code: shared.FieldNotAllowed, Message: "is reserved"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CheckDraft(tt.wsName, tt.slug)
			var se *shared.Error
			if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed || !slices.Equal(se.Fields, tt.want) {
				t.Errorf("CheckDraft() = %+v, want validation_failed with %+v", err, tt.want)
			}
		})
	}
}

func TestCheckNameIsTheDraftsRule(t *testing.T) {
	if got, err := CheckName("  Acme 研发 \n"); err != nil || got != "Acme 研发" {
		t.Errorf("CheckName() = %q, %v; want it trimmed", got, err)
	}
	_, err := CheckName(strings.Repeat("a", 81))
	var se *shared.Error
	want := []shared.FieldError{{Field: "name", Code: shared.FieldTooLong, Message: "must be at most 80 characters"}}
	if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed || !slices.Equal(se.Fields, want) {
		t.Errorf("CheckName(81 characters) = %+v, want validation_failed with %+v", err, want)
	}
}
