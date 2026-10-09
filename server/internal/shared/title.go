package shared

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// MaxTitleBytes is a title's longest, in bytes of UTF-8: a file name's
// limit on the common file systems.
const MaxTitleBytes = 255

// CheckTitle checks a title: a notebook's name (M3/P1 design 3.2), and from
// M4 a page's title, which an export writes as the name of a file or a
// folder (v0.1 design 3.5). It returns s without its surrounding white
// space, in NFC, when s is UTF-8 and that is 1–255 bytes, holds none of
// / \ : * ? " < > | # ^ [ ] nor a character unshowable in a name (see
// unshowable), neither starts nor ends with a dot, and is no name Windows
// reserves; otherwise a problem on field. The bytes are counted in NFC,
// as they are stored.
func CheckTitle(field, s string) (string, *FieldError) {
	if !utf8.ValidString(s) {
		// A JSON string is UTF-8 once decoded; a file name of a form's
		// part or of a zip's entry need not be (M7/P2).
		return "", &FieldError{Field: field, Code: FieldInvalidFormat, Message: "must be UTF-8"}
	}
	s = norm.NFC.String(strings.TrimSpace(s))
	switch {
	case s == "":
		return "", &FieldError{Field: field, Code: FieldRequired, Message: "is required"}
	case len(s) > MaxTitleBytes:
		return "", &FieldError{Field: field, Code: FieldTooLong, Message: "must be at most 255 bytes"}
	case strings.ContainsAny(s, forbidden) || strings.ContainsFunc(s, unshowable):
		return "", &FieldError{Field: field, Code: FieldInvalidFormat, Message: `must not contain / \ : * ? " < > | # ^ [ ] or control characters`}
	case strings.HasPrefix(s, ".") || strings.HasSuffix(s, "."):
		return "", &FieldError{Field: field, Code: FieldInvalidFormat, Message: "must not start or end with a dot"}
	case windowsReserved(s):
		return "", &FieldError{Field: field, Code: FieldNotAllowed, Message: "is a name Windows reserves"}
	}
	return s, nil
}

// windowsReserved reports whether Windows reserves s as a file name: a
// device name, in any case, alone or before an extension (con, CON.txt).
// The serial and parallel ports take the digits 1–9 and the superscripts
// ¹ ² ³, which Windows reads as digits too.
func windowsReserved(s string) bool {
	base, _, _ := strings.Cut(s, ".")
	base = strings.ToUpper(base)
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	port, ok := strings.CutPrefix(base, "COM")
	if !ok {
		port, ok = strings.CutPrefix(base, "LPT")
	}
	return ok && (len(port) == 1 && port[0] >= '1' && port[0] <= '9' || port == "¹" || port == "²" || port == "³")
}

// forbidden are the characters a title holds none of.
const forbidden = `/\:*?"<>|#^[]`

// FixTitle mends s into a title (M7/P6 design 3.4): an import's names,
// which a zip writes as it likes. Bytes that are not UTF-8 become U+FFFD;
// the rest is put in NFC; white space and dots go from both ends; each
// character left that CheckTitle refuses (see forbidden and unshowable)
// becomes "_", and the ends go again; a name Windows reserves takes "_"
// before its first dot (con is con_, con.txt con_.txt); and one longer
// than 255 bytes is cut at a character, keeping, when keepExtension is
// set, its extension: what follows its last dot. It answers "" when
// nothing is left; any other answer passes CheckTitle unchanged, and a
// title that passes it is its own answer.
func FixTitle(s string, keepExtension bool) string {
	s = trimEnds(norm.NFC.String(strings.ToValidUTF8(s, "\ufffd")))
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(forbidden, r) || unshowable(r) {
			return '_'
		}
		return r
	}, s)
	s = fit(unreserve(trimEnds(s)), keepExtension)
	if windowsReserved(s) {
		// A cut that kept little of the name may leave one reserved.
		s = fit(unreserve(s), false)
	}
	return s
}

// trimEnds is s without the white space and dots at its ends.
func trimEnds(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == '.' })
}

// unreserve is s, "_" after its first dot's left when Windows reserves
// it.
func unreserve(s string) string {
	if !windowsReserved(s) {
		return s
	}
	base, rest, dotted := strings.Cut(s, ".")
	if dotted {
		return base + "_." + rest
	}
	return base + "_"
}

// fit is s cut to MaxTitleBytes at a character, in NFC, without white
// space and dots at its ends: keeping its extension when keepExtension
// is set, it has one shorter than the bytes, and a character of the rest
// fits beside it.
func fit(s string, keepExtension bool) string {
	for len(s) > MaxTitleBytes {
		stem, ext := s, ""
		if i := strings.LastIndexByte(s, '.'); keepExtension && i > 0 && len(s)-i < MaxTitleBytes {
			stem, ext = s[:i], s[i:]
		}
		if cut := cutTo(stem, MaxTitleBytes-len(ext)); cut != "" {
			s = cut + ext
		} else {
			s = cutTo(s, MaxTitleBytes)
		}
		// A cut between a character and the marks after it may compose
		// otherwise in NFC.
		s = trimEnds(norm.NFC.String(s))
	}
	return trimEnds(s)
}

// cutTo is s cut to at most n bytes at a character's start.
func cutTo(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
