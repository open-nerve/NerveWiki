package domain_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"uuid"

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

func TestNumbered(t *testing.T) {
	long := strings.Repeat("a", 260)
	for _, tt := range []struct {
		name  string
		n     int
		asset bool
		want  string
	}{
		{"Notes", 2, false, "Notes 2"},
		{"v1.2 notes", 3, false, "v1.2 notes 3"},
		{"photo.png", 2, true, "photo 2.png"},
		{"archive.tar.gz", 10, true, "archive.tar 10.gz"},
		{"README", 2, true, "README 2"},
		{"photo.png", 2, false, "photo.png 2"},
		{long[:255], 2, false, long[:253] + " 2"},
		{long[:251] + ".png", 12, true, long[:248] + " 12.png"},
		{strings.Repeat("名", 85), 2, false, strings.Repeat("名", 84) + " 2"},
		// An extension that leaves no room: numbered as a page's name.
		{"a." + long[:253], 2, true, ("a." + long[:253])[:253] + " 2"},
	} {
		got := domain.Numbered(tt.name, tt.n, tt.asset)
		if got != tt.want {
			t.Errorf("Numbered(%q, %d, %t) = %q, want %q", tt.name, tt.n, tt.asset, got, tt.want)
		}
		if _, f := shared.CheckTitle("name", got); f != nil {
			t.Errorf("Numbered(%q, %d, %t) = %q, not a title: %+v", tt.name, tt.n, tt.asset, got, f)
		}
		// Its family: the number's digits '#'s.
		number := strconv.Itoa(tt.n)
		want := strings.Replace(tt.want, " "+number, " "+strings.Repeat("#", len(number)), 1)
		if got := domain.Family(tt.name, tt.n, tt.asset); got != want {
			t.Errorf("Family(%q, %d, %t) = %q, want %q", tt.name, tt.n, tt.asset, got, want)
		}
	}
}

// A step aside's name (M9/P3 design 3.17): " ~" and the id's last 8 hex
// digits; all 32 when those are taken, by key, then -2 and on; built as
// Numbered builds a name, cut at a character within 255 bytes, an
// attachment's extension kept; always a title.
func TestAsideName(t *testing.T) {
	id := uuid.MustParse("0192f3a4-5b6c-7d8e-9fa0-b1c2d3e4f5a6")
	const last, all = "d3e4f5a6", "0192f3a45b6c7d8e9fa0b1c2d3e4f5a6"
	long := strings.Repeat("a", 260)
	for _, tt := range []struct {
		name  string
		asset bool
		taken []string
		want  string
	}{
		{"Plan", false, nil, "Plan ~" + last},
		{"Plan", false, []string{"Plan ~" + last}, "Plan ~" + all},
		// A sibling of another case holds the key.
		{"Plan", false, []string{"PLAN ~" + strings.ToUpper(last)}, "Plan ~" + all},
		// Someone named a page so: the next number.
		{"Plan", false, []string{"Plan ~" + last, "Plan ~" + all}, "Plan ~" + all + "-2"},
		{"Plan", false, []string{"Plan ~" + last, "Plan ~" + all, "Plan ~" + all + "-2"}, "Plan ~" + all + "-3"},
		{long[:245], false, nil, long[:245] + " ~" + last},
		{long[:246], false, nil, long[:245] + " ~" + last},
		{long[:254], false, nil, long[:245] + " ~" + last},
		{long[:255], false, nil, long[:245] + " ~" + last},
		{long[:255], false, []string{long[:245] + " ~" + last}, long[:221] + " ~" + all},
		{strings.Repeat("名", 85), false, nil, strings.Repeat("名", 81) + " ~" + last},
		{"photo.png", true, nil, "photo ~" + last + ".png"},
		{"photo.png", true, []string{"photo ~" + last + ".png"}, "photo ~" + all + ".png"},
		{"photo.png", false, nil, "photo.png ~" + last},
		{"README", true, nil, "README ~" + last},
		{long[:251] + ".png", true, nil, long[:241] + " ~" + last + ".png"},
		// An extension that leaves no room: as a page's name.
		{"a." + long[:253], true, nil, ("a." + long[:253])[:245] + " ~" + last},
	} {
		taken := map[string]bool{}
		for _, n := range tt.taken {
			taken[shared.TitleKey(n)] = true
		}
		got := domain.AsideName(tt.name, id, tt.asset, func(key string) bool { return taken[key] })
		if got != tt.want {
			t.Errorf("AsideName(%q, %t, taken %q) = %q, want %q", tt.name, tt.asset, tt.taken, got, tt.want)
		}
		if title, f := shared.CheckTitle("name", got); f != nil || title != got {
			t.Errorf("AsideName(%q, %t) = %q, not a title: %+v", tt.name, tt.asset, got, f)
		}
	}
}
