package obsidian

import (
	"regexp"
	"strconv"
	"strings"
)

// sizeShape is a size as an embed's display text or an image's caption
// ends with it: a width, or a width 'x' a height, in decimal digits.
var sizeShape = regexp.MustCompile(`^([0-9]+)(?:x([0-9]+))?$`)

// maxSide is the largest width or height a size gives.
const maxSide = 10_000

// size is the width and height an embed is written with, in pixels; 0 for
// one not given.
type size struct {
	width, height int
}

// sized reads an embed's display text or a Markdown image's caption as
// Obsidian 1.12.7 does (M7/P3 design 5.5): when what follows its last '|',
// or the whole of it, has a size's shape, that is the size and what comes
// before the '|' its caption; else it is all caption ("300x" and "x200"
// are captions). The caption is without the white space around it (an
// image's caption may be written of a line break, &#10;). A
// number outside 1–10,000 is not given, though the shape is still a
// size's (nerve-defined: Obsidian writes 0 and 20000 as they are).
func sized(s string) (string, size) {
	caption, last := "", s
	if k := strings.LastIndexByte(s, '|'); k >= 0 {
		caption, last = s[:k], s[k+1:]
	}
	m := sizeShape.FindStringSubmatch(strings.Trim(last, " \t"))
	if m == nil {
		return strings.Trim(s, blank), size{}
	}
	return strings.Trim(caption, blank), size{width: side(m[1]), height: side(m[2])}
}

// blank is the white space around a caption: ASCII's, which HTML's is.
const blank = " \t\n\f\r"

// side is a width's or a height's digits as a number of pixels, 0 for
// none or one outside 1–10,000.
func side(digits string) int {
	n, err := strconv.Atoi(digits)
	if err != nil || n < 1 || n > maxSide {
		return 0
	}
	return n
}
