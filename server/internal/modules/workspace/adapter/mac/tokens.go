// Package macadapter makes and checks the invitations' tokens (M2/P3
// design 3.2): the token of an invitation is a MAC of its id, so it is
// never stored, and an admin can copy the link at any time. The key is
// derived from the instance's signing key; it never leaves this package
// and is never logged.
package macadapter

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"uuid"
)

// prefix marks an invitation's token, as nwk_pat_ marks a personal access
// token: a secret scanner, or a reader, can tell what it is.
const prefix = "nwk_inv_"

// Tokens implements app.InvitationTokens.
type Tokens struct {
	key []byte
}

// New returns the tokens of key.
func New(key []byte) Tokens {
	return Tokens{key: key}
}

// Token returns the token of the invitation id: nwk_inv_ and the first 16
// bytes of HMAC-SHA256(key, id), base64url without padding.
func (t Tokens) Token(id uuid.UUID) string {
	return prefix + base64.RawURLEncoding.EncodeToString(t.tag(id))
}

// Valid reports whether token is the token of id. It decodes strictly, so
// that a token has one spelling: the last character's unused bits must be
// zero. It compares in constant time: the time of a comparison would tell a
// forger how much of a tag is right.
func (t Tokens) Valid(id uuid.UUID, token string) bool {
	encoded, ok := strings.CutPrefix(token, prefix)
	if !ok {
		return false
	}
	tag, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	return err == nil && hmac.Equal(tag, t.tag(id))
}

func (t Tokens) tag(id uuid.UUID) []byte {
	h := hmac.New(sha256.New, t.key)
	h.Write(id[:])
	return h.Sum(nil)[:16]
}
