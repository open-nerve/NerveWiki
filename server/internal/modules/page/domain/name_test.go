package domain_test

import (
	"errors"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestCheckAssetName(t *testing.T) {
	for _, tt := range []struct {
		name, want string // want: the name, or the problem
	}{
		{" photo.png ", "photo.png"},
		{"notes.md.png", "notes.md.png"},
		{"README", "README"},
		{"amd", "amd"},
		{"ab", "ab"},
		{"a", "a"},
		{"notes.md", "validation_failed name not_allowed"},
		{"notes.MD", "validation_failed name not_allowed"},
		{"notes.mD ", "validation_failed name not_allowed"},
		{".md", "validation_failed name invalid_format"},
		{"a/b.png", "validation_failed name invalid_format"},
		{"", "validation_failed name required"},
	} {
		got, err := domain.CheckAssetName("name", tt.name)
		if s := nameOrProblem(got, err); s != tt.want {
			t.Errorf("CheckAssetName(%q) = %q, want %q", tt.name, s, tt.want)
		}
	}
}

func TestCheckAssetRename(t *testing.T) {
	for _, tt := range []struct {
		old, name, want string // want: the name, or the problem
	}{
		{"photo.png", "cover.png", "cover.png"},
		{"photo.png", " cover.png ", "cover.png"},
		{"photo.png", "b.c", "b.c"},
		{"photo.png", "photo.x", "photo.x"},
		{"a.png", "a", "validation_failed name not_allowed"},
		{"photo.x", "photo", "validation_failed name not_allowed"},
		{"photo.png", "photo.jpeg", "photo.jpeg"},
		{"photo.png", "photo.tar.gz", "photo.tar.gz"},
		{"photo.png", "photo", "validation_failed name not_allowed"},
		{"photo.png", "photo.md", "validation_failed name not_allowed"},
		{"README", "LICENSE", "LICENSE"},
		{"README", "README.txt", "README.txt"},
		{"README", "readme.md", "validation_failed name not_allowed"},
		{"photo.png", "a:b.png", "validation_failed name invalid_format"},
	} {
		got, err := domain.CheckAssetRename("name", tt.old, tt.name)
		if s := nameOrProblem(got, err); s != tt.want {
			t.Errorf("CheckAssetRename(%q, %q) = %q, want %q", tt.old, tt.name, s, tt.want)
		}
	}
}

// nameOrProblem is title's name, or err's code, field and field code.
func nameOrProblem(title domain.Title, err error) string {
	if err == nil {
		return title.Name
	}
	return problemOf(err)
}

// problemOf is err's code, its one field and the field's code.
func problemOf(err error) string {
	var e *shared.Error
	if !errors.As(err, &e) || len(e.Fields) != 1 {
		return err.Error()
	}
	return e.Code + " " + e.Fields[0].Field + " " + e.Fields[0].Code
}
