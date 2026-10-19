package shared

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// MaxClientNameChars bounds a client's name, in characters (M9 design 4.2).
const MaxClientNameChars = 128

// CleanClientName is name as a changeset's client and a token's
// client_name hold it (M9 design 4.5): its bytes that are not UTF-8 and its
// characters unshowable in a name dropped, in NFC, without the white space
// around it, cut to MaxClientNameChars at a normalization boundary, and
// trimmed again; "" when nothing is left. It is idempotent: P2's
// domain.MCPClient cleans a recorded name again.
//
// The order makes it so (M9/P1 design 2.1 item 2): NFC after dropping, so
// that no character dropped leaves two to combine on a second clean. The
// cut keeps all it can and stays NFC: after the 128th character when the
// 129th starts anew, combining with nothing before it; else at the last
// boundary of the first 128, before the starter the 129th would combine
// with.
func CleanClientName(name string) string {
	s := strings.Map(func(r rune) rune {
		if Unshowable(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(name, ""))
	s = strings.TrimSpace(norm.NFC.String(s))
	if utf8.RuneCountInString(s) <= MaxClientNameChars {
		return s
	}
	r := []rune(s)
	cut := string(r[:MaxClientNameChars])
	if !norm.NFC.PropertiesString(string(r[MaxClientNameChars])).BoundaryBefore() {
		if b := norm.NFC.LastBoundary([]byte(cut)); b >= 0 {
			cut = cut[:b]
		}
	}
	return strings.TrimSpace(cut)
}
