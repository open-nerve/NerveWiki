package shared

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// maxTitleBytes is a title's longest, in bytes of UTF-8: a file name's
// limit on the common file systems.
const maxTitleBytes = 255

// CheckTitle checks a title: a notebook's name (M3/P1 design 3.2), and from
// M4 a page's title, which an export writes as the name of a file or a
// folder (v0.1 design 3.5). It returns s without its surrounding white
// space, in NFC, when that is 1–255 bytes, holds none of / \ : * ? " < > |
// # ^ [ ] nor a character unshowable in a name (see unshowable), neither
// starts nor ends with a dot, and is no name Windows reserves; otherwise a
// problem on field. The bytes are counted in NFC, as they are stored.
func CheckTitle(field, s string) (string, *FieldError) {
	s = norm.NFC.String(strings.TrimSpace(s))
	switch {
	case s == "":
		return "", &FieldError{Field: field, Code: FieldRequired, Message: "is required"}
	case len(s) > maxTitleBytes:
		return "", &FieldError{Field: field, Code: FieldTooLong, Message: "must be at most 255 bytes"}
	case strings.ContainsAny(s, `/\:*?"<>|#^[]`) || strings.ContainsFunc(s, unshowable):
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
func windowsReserved(s string) bool {
	base, _, _ := strings.Cut(s, ".")
	base = strings.ToUpper(base)
	switch {
	case base == "CON" || base == "PRN" || base == "AUX" || base == "NUL":
		return true
	case len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")):
		return base[3] >= '1' && base[3] <= '9'
	}
	return false
}
