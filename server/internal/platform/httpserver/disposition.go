package httpserver

import (
	"encoding/hex"
	"strings"
)

// Disposition is the Content-Disposition of a file named name (RFC 6266):
// inline, shown, or attachment, downloaded; its name in ASCII, every other
// character, a quote and a backslash an underscore, and in UTF-8 (RFC
// 8187).
func Disposition(inline bool, name string) string {
	kind := "attachment"
	if inline {
		kind = "inline"
	}
	var ascii, utf strings.Builder
	for _, r := range name {
		if r < 0x20 || r >= 0x7f || r == '"' || r == '\\' {
			ascii.WriteByte('_')
		} else {
			ascii.WriteRune(r)
		}
	}
	for _, c := range []byte(name) {
		if attrChar(c) {
			utf.WriteByte(c)
		} else {
			utf.WriteString("%" + strings.ToUpper(hex.EncodeToString([]byte{c})))
		}
	}
	return kind + `; filename="` + ascii.String() + `"; filename*=UTF-8''` + utf.String()
}

// attrChar reports whether c is written as itself in an RFC 8187 value.
func attrChar(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || strings.IndexByte("!#$&+-.^_`|~", c) >= 0
}
