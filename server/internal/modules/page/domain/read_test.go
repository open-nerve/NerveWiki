package domain_test

import (
	"errors"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// wire counts a text block's bytes as JSON writes a string, closely enough
// for the tests: an escaped character two bytes, any other its own.
func wire(s string) int {
	return len(s) + strings.Count(s, "\n") + strings.Count(s, "\r") + strings.Count(s, `"`) + strings.Count(s, `\`)
}

func TestCheckWindow(t *testing.T) {
	one, zero, minus, two := 1, 0, -1, 2
	if w, err := domain.CheckWindow(nil, nil, nil); err != nil || w != (domain.Window{Offset: 1, Column: 0, Limit: domain.DefaultReadLines}) {
		t.Errorf("the defaults are %+v, %v", w, err)
	}
	if w, err := domain.CheckWindow(&two, &zero, &one); err != nil || w != (domain.Window{Offset: 2, Column: 0, Limit: 1}) {
		t.Errorf("the window is %+v, %v", w, err)
	}
	_, err := domain.CheckWindow(&zero, &minus, &zero)
	var e *shared.Error
	if !errors.As(err, &e) || len(e.Fields) != 3 {
		t.Fatalf("CheckWindow(0, -1, 0) = %v", err)
	}
	for i, field := range []string{"offset", "column", "limit"} {
		if f := e.Fields[i]; f.Field != field || f.Code != shared.FieldOutOfRange {
			t.Errorf("field %d is %+v, want %s out_of_range", i, f, field)
		}
	}
}

// sliceOf is s's slice: its text, lines, total, next offset and column.
type sliceOf struct {
	text                   string
	lines, total           int
	nextOffset, nextColumn int
}

func TestCut(t *testing.T) {
	const bom = "\xef\xbb\xbf"
	for _, tt := range []struct {
		name    string
		content string
		w       domain.Window
		budget  int
		want    sliceOf
	}{
		{"all", "a\nb\nc\n", domain.Window{Offset: 1, Limit: 10}, 100, sliceOf{"a\nb\nc\n", 3, 3, 0, 0}},
		{"no last newline", "a\nb", domain.Window{Offset: 1, Limit: 10}, 100, sliceOf{"a\nb", 2, 2, 0, 0}},
		{"empty", "", domain.Window{Offset: 1, Limit: 10}, 100, sliceOf{"", 0, 0, 0, 0}},
		{"a newline", "\n", domain.Window{Offset: 1, Limit: 10}, 100, sliceOf{"\n", 1, 1, 0, 0}},
		{"CR stays in its line", bom + "a\r\nb\r\n", domain.Window{Offset: 1, Limit: 1}, 100, sliceOf{bom + "a\r\n", 1, 2, 2, 0}},
		{"from the second line", "a\nb\nc\n", domain.Window{Offset: 2, Limit: 1}, 100, sliceOf{"b\n", 1, 3, 3, 0}},
		{"to the limit", "a\nb\nc\n", domain.Window{Offset: 1, Limit: 2}, 100, sliceOf{"a\nb\n", 2, 3, 3, 0}},
		{"the limit at the end", "a\nb\n", domain.Window{Offset: 1, Limit: 2}, 100, sliceOf{"a\nb\n", 2, 2, 0, 0}},
		{"to the budget", "aa\nbb\ncc\n", domain.Window{Offset: 1, Limit: 10}, 8, sliceOf{"aa\nbb\n", 2, 3, 3, 0}},
		{"the budget exactly", "aa\nbb\n", domain.Window{Offset: 1, Limit: 10}, 8, sliceOf{"aa\nbb\n", 2, 2, 0, 0}},
		{"a long first line", strings.Repeat("x", 10) + "\nend\n", domain.Window{Offset: 1, Limit: 10}, 4, sliceOf{"xxxx", 1, 2, 1, 4}},
		{"on from its column", strings.Repeat("x", 10) + "\nend\n", domain.Window{Offset: 1, Column: 4, Limit: 10}, 4, sliceOf{"xxxx", 1, 2, 1, 8}},
		{"its end", strings.Repeat("x", 10) + "\nend\n", domain.Window{Offset: 1, Column: 8, Limit: 10}, 3, sliceOf{"xx", 1, 2, 1, 10}},
		{"its newline", strings.Repeat("x", 10) + "\nend\n", domain.Window{Offset: 1, Column: 10, Limit: 10}, 7, sliceOf{"\nend\n", 2, 2, 0, 0}},
		{"cut at a character", "ééé\n", domain.Window{Offset: 1, Limit: 10}, 5, sliceOf{"éé", 1, 1, 1, 4}},
		{"an escape counted", "\"\"\"\n", domain.Window{Offset: 1, Limit: 10}, 5, sliceOf{"\"\"", 1, 1, 1, 2}},
		{"past the end", "a\n", domain.Window{Offset: 2, Limit: 10}, 100, sliceOf{"", 0, 1, 0, 0}},
		{"far past the end", "a\n", domain.Window{Offset: 9, Limit: 10}, 100, sliceOf{"", 0, 1, 0, 0}},
	} {
		s, err := domain.Cut(tt.content, tt.w, tt.budget, wire)
		got := sliceOf{s.Text, s.Lines, s.TotalLines, s.NextOffset, s.NextColumn}
		if err != nil || got != tt.want {
			t.Errorf("%s: Cut = %+v, %v; want %+v", tt.name, got, err, tt.want)
		}
	}
	for _, tt := range []struct {
		name    string
		content string
		w       domain.Window
	}{
		{"inside a character", "é\n", domain.Window{Offset: 1, Column: 1, Limit: 1}},
		{"past the line", "ab\ncd\n", domain.Window{Offset: 1, Column: 3, Limit: 1}},
		{"past the end, a column", "a\n", domain.Window{Offset: 2, Column: 1, Limit: 1}},
		{"an empty content, a column", "", domain.Window{Offset: 1, Column: 1, Limit: 1}},
	} {
		if _, err := domain.Cut(tt.content, tt.w, 100, wire); err == nil || problemOf(err) != "validation_failed column out_of_range" {
			t.Errorf("%s: Cut = %v", tt.name, err)
		}
	}
	if _, err := domain.Cut("\"\n", domain.Window{Offset: 1, Limit: 1}, 1, wire); !errors.Is(err, domain.ErrNoRoom) {
		t.Errorf("a budget that holds no character: %v", err)
	}
}

// Read on from each answer's next offset and column, a content's slices
// joined are the content, byte for byte, each within its budget: lines of
// CRLF and LF, a BOM, long lines and characters of several bytes, for
// any window and budget (M9/P1 design 3.11.1, 3.19).
func TestCutRebuildsTheContent(t *testing.T) {
	rnd := rand.New(rand.NewPCG(5, 7))
	pieces := []string{"a", "é", "漢", "\n", "\r\n", "\"", "\t", "\xef\xbb\xbf", strings.Repeat("x", 40)}
	for range 5000 {
		var b strings.Builder
		for range rnd.IntN(60) {
			b.WriteString(pieces[rnd.IntN(len(pieces))])
		}
		content := b.String()
		budget := 6 + rnd.IntN(80)
		limit := 1 + rnd.IntN(5)
		var got strings.Builder
		w := domain.Window{Offset: 1, Limit: limit}
		for calls := 0; ; calls++ {
			if calls > 10000 {
				t.Fatalf("%q: no end", content)
			}
			s, err := domain.Cut(content, w, budget, wire)
			if err != nil {
				t.Fatalf("%q from %+v: %v", content, w, err)
			}
			if wire(s.Text) > budget || !utf8.ValidString(s.Text) || s.Lines > limit {
				t.Fatalf("%q from %+v: %+v is over its budget %d or limit %d", content, w, s, budget, limit)
			}
			got.WriteString(s.Text)
			if s.NextOffset == 0 {
				break
			}
			w.Offset, w.Column = s.NextOffset, s.NextColumn
		}
		if got.String() != content {
			t.Fatalf("the slices of %q are %q", content, got.String())
		}
	}
}

func TestInlineImage(t *testing.T) {
	for _, tt := range []struct {
		mime          string
		bytes         int64
		width, height int
		want          bool
	}{
		{"image/png", 100, 640, 480, true},
		{"image/jpeg", 100, 1, 1, true},
		{"image/gif", 100, 8000, 8000, true},
		{"image/webp", domain.MaxInlineImageBytes, 10, 10, true},
		{"image/webp", domain.MaxInlineImageBytes + 1, 10, 10, false},
		{"image/png", 100, 8001, 10, false},
		{"image/png", 100, 10, 8001, false},
		{"image/png", 100, 0, 0, false}, // no size known
		{"image/svg+xml", 100, 10, 10, false},
		{"image/avif", 100, 10, 10, false},
		{"application/pdf", 100, 10, 10, false},
	} {
		if got := domain.InlineImage(tt.mime, tt.bytes, tt.width, tt.height); got != tt.want {
			t.Errorf("InlineImage(%s, %d, %d×%d) = %v", tt.mime, tt.bytes, tt.width, tt.height, got)
		}
	}
}

func TestCheckListLimit(t *testing.T) {
	zero, one, most, over := 0, 1, domain.MaxListLimit, domain.MaxListLimit+1
	for _, tt := range []struct {
		limit *int
		want  string
	}{
		{nil, "200"}, {&one, "1"}, {&most, "1000"},
		{&zero, "validation_failed limit out_of_range"}, {&over, "validation_failed limit out_of_range"},
	} {
		n, err := domain.CheckListLimit(tt.limit)
		got := strconv.Itoa(n)
		if err != nil {
			got = problemOf(err)
		}
		if got != tt.want {
			t.Errorf("CheckListLimit = %q, want %q", got, tt.want)
		}
	}
}
