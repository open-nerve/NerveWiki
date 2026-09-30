package signing

import (
	"crypto/hmac"
	"crypto/sha256"
)

// RefreshTokenMAC implements app.RefreshTokenMAC: the first 16 bytes of
// HMAC-SHA256 under K = HKDF-SHA256(the signing key's seed, info "nervewiki
// refresh-token mac v1") (M1/P1 design 3.4). The tag lets the server
// recognize an old generation it issued without storing it.
type RefreshTokenMAC struct {
	keys *Keys
}

// NewRefreshTokenMAC returns the MAC of keys.
func NewRefreshTokenMAC(keys *Keys) *RefreshTokenMAC {
	return &RefreshTokenMAC{keys: keys}
}

// Tag returns the tag of message.
func (m *RefreshTokenMAC) Tag(message []byte) [16]byte {
	h := hmac.New(sha256.New, m.keys.mac)
	h.Write(message)
	return [16]byte(h.Sum(nil)[:16])
}

// Verify reports whether tag is the tag of message. It compares in constant
// time: the time of a comparison would tell a forger how much of a tag is
// right.
func (m *RefreshTokenMAC) Verify(message []byte, tag [16]byte) bool {
	want := m.Tag(message)
	return hmac.Equal(want[:], tag[:])
}
