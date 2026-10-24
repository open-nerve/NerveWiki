package shared

import (
	"context"
	"strings"
	"unicode/utf8"
	"uuid"

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

// ClientOf is where the request ctx carries came from, as a write's
// changeset records it (v0.1 design 13.1 item 2): a personal access
// token's request is the API's, "api"; a sign-in session's access token's
// the web's, "web". It reads the request's Actor: 401 unauthorized when
// there is none, as RequireActor, which with the deny-by-default
// authentication only happens on a route wired wrongly. Each module's
// adapter converts the answer to its own type where it has one.
func ClientOf(ctx context.Context) (string, error) {
	actor, err := RequireActor(ctx)
	switch {
	case err != nil:
		return "", err
	case actor.APITokenID != uuid.UUID{}:
		return "api", nil
	}
	return "web", nil
}
