package domain

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// DefaultReadLines is read's limit when none is given.
const DefaultReadLines = 2000

// Window is what of a page read asks for: from the line Offset (from 1),
// its byte Column (from 0), Limit lines at most.
type Window struct{ Offset, Column, Limit int }

// CheckWindow checks offset (from 1), column (from 0) and limit (from 1),
// nil for the defaults: out_of_range on each, before any read.
func CheckWindow(offset, column, limit *int) (Window, error) {
	w := Window{Offset: 1, Column: 0, Limit: DefaultReadLines}
	var fields []shared.FieldError
	if offset != nil {
		if w.Offset = *offset; w.Offset < 1 {
			fields = append(fields, shared.FieldError{Field: "offset", Code: shared.FieldOutOfRange, Message: "must be at least 1"})
		}
	}
	if column != nil {
		if w.Column = *column; w.Column < 0 {
			fields = append(fields, shared.FieldError{Field: "column", Code: shared.FieldOutOfRange, Message: "must be at least 0"})
		}
	}
	if limit != nil {
		if w.Limit = *limit; w.Limit < 1 {
			fields = append(fields, shared.FieldError{Field: "limit", Code: shared.FieldOutOfRange, Message: "must be at least 1"})
		}
	}
	if len(fields) > 0 {
		return Window{}, shared.Invalid(fields...)
	}
	return w, nil
}

// Slice is the part of a content read gives.
type Slice struct {
	Text                   string // the content's bytes from the window's start, exactly
	Lines, TotalLines      int
	NextOffset, NextColumn int // 0: none
}

// ErrNoRoom is a budget that holds no character: Cut never answers an
// empty slice where the content goes on.
var ErrNoRoom = errors.New("page: a read's budget holds no character")

// Cut cuts content for w within budget, as measure counts a text block
// (mcpserver.TextBytes, which page's adapter/mcp passes: page counts no
// bytes of its own), each character on its own: column out_of_range when
// w.Column is not on a character's boundary of its line or past it, or
// when the line is past the end and the column is not 0.
//
// Lines end at "\n", which ends the last line without starting another;
// "\r" stays in its line. From the window's start the lines are added
// whole, each with its "\n", while they fit; a first line that does not
// fit is cut at the longest start of it that does, NextColumn telling
// where. Reading on from NextOffset and NextColumn, the slices joined are
// the content.
func Cut(content string, w Window, budget int, measure func(string) int) (Slice, error) {
	total := strings.Count(content, "\n")
	if content != "" && !strings.HasSuffix(content, "\n") {
		total++
	}
	s := Slice{TotalLines: total}
	if w.Offset > total {
		if w.Column != 0 {
			return Slice{}, columnOutOfRange("must be 0 past the content's last line")
		}
		return s, nil
	}
	start := lineStart(content, w.Offset)
	line, _, _ := strings.Cut(content[start:], "\n")
	switch {
	case w.Column > len(line):
		return Slice{}, columnOutOfRange("must be at most the line's length in bytes")
	case w.Column < len(line) && !utf8.RuneStart(line[w.Column]):
		return Slice{}, columnOutOfRange("must be on a character's boundary")
	}
	start += w.Column
	end, used := start, 0
	for s.Lines < w.Limit && end < len(content) {
		next := len(content)
		if i := strings.IndexByte(content[end:], '\n'); i >= 0 {
			next = end + i + 1
		}
		n := measure(content[end:next])
		if used+n > budget {
			if s.Lines > 0 {
				break
			}
			// The first line does not fit: its longest start that does.
			cut := prefix(content[end:next], budget-used, measure)
			if cut == 0 {
				return Slice{}, ErrNoRoom
			}
			s.Text, s.Lines = content[start:end+cut], 1
			s.NextOffset, s.NextColumn = w.Offset, w.Column+cut
			return s, nil
		}
		end, used = next, used+n
		s.Lines++
	}
	s.Text = content[start:end]
	if end < len(content) {
		s.NextOffset = w.Offset + s.Lines
	}
	return s, nil
}

// lineStart is the byte where line n (from 1) of content starts; content
// has n lines at least.
func lineStart(content string, n int) int {
	start := 0
	for range n - 1 {
		start += strings.IndexByte(content[start:], '\n') + 1
	}
	return start
}

// prefix is the bytes of the longest start of s at a character's boundary
// that budget holds, as measure counts it.
func prefix(s string, budget int, measure func(string) int) int {
	n, used := 0, 0
	for n < len(s) {
		_, size := utf8.DecodeRuneInString(s[n:])
		k := measure(s[n : n+size])
		if used+k > budget {
			break
		}
		n, used = n+size, used+k
	}
	return n
}

// columnOutOfRange is 422 out_of_range on column.
func columnOutOfRange(message string) error {
	return shared.Invalid(shared.FieldError{Field: "column", Code: shared.FieldOutOfRange, Message: message})
}

// MaxInlineImageBytes and MaxInlineImageSide bound an image read gives
// whole (M9 design 4.9): a model's client takes about 5 MB of base64 and
// 8,000 pixels a side.
const (
	MaxInlineImageBytes = 3 << 20
	MaxInlineImageSide  = 8000
)

// InlineImage reports whether read gives an attachment's bytes as an
// image (M9 design 4.9): PNG, JPEG, GIF or WebP, at most
// MaxInlineImageBytes, both sides known and at most MaxInlineImageSide.
// An SVG is XML, which a model does not read as an image.
func InlineImage(mime string, bytes int64, width, height int) bool {
	return InlineType(mime) && bytes <= MaxInlineImageBytes &&
		width >= 1 && width <= MaxInlineImageSide && height >= 1 && height <= MaxInlineImageSide
}

// InlineType reports whether read may give an attachment of type mime as
// an image.
func InlineType(mime string) bool {
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	}
	return false
}
