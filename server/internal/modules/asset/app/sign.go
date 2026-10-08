package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"time"
	"uuid"
)

// The signed addresses of the attachments' contents (M7 design 4.5;
// M7/P2 design 3.6): a reader who may read an attachment gets its address,
// which any browser then opens without a token for an hour or two.

// Signed is an attachment's signed address: when it expires, and the
// signatures of its content, shown and downloaded (d=1).
type Signed struct {
	Expires  time.Time
	Inline   string
	Download string
}

// Signer signs and checks the addresses with the content key.
type Signer struct {
	key   []byte
	clock Clock
}

// NewSigner returns the signer of key, the identity module's signing
// key's derivation for the content (ContentKeyInfo).
func NewSigner(key []byte, clock Clock) Signer {
	return Signer{key: key, clock: clock}
}

// Sign signs the address of node's file blob: it expires at the end of the
// hour after this one, so an hour's addresses are the same and each lasts
// one to two hours.
func (s Signer) Sign(node, blob uuid.UUID) Signed {
	e := (s.clock.Now().Unix()/3600 + 2) * 3600
	return Signed{Expires: time.Unix(e, 0).UTC(), Inline: s.mac(node, blob, e, false), Download: s.mac(node, blob, e, true)}
}

// Valid reports whether sig signs node's file blob, expiring at e, shown
// or downloaded, and e is later than now.
func (s Signer) Valid(node, blob uuid.UUID, e int64, download bool, sig string) bool {
	return e > s.clock.Now().Unix() && hmac.Equal([]byte(s.mac(node, blob, e, download)), []byte(sig))
}

// mac is the first 16 bytes of HMAC-SHA256 over "asset-content", the
// node's and the blob's 16 bytes, e's 8 bytes big-endian and download's
// byte, in base64url without padding: 22 characters.
func (s Signer) mac(node, blob uuid.UUID, e int64, download bool) string {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte("asset-content"))
	m.Write(node[:])
	m.Write(blob[:])
	m.Write(binary.BigEndian.AppendUint64(nil, uint64(e))) //nolint:gosec // a time after 1970
	d := byte(0)
	if download {
		d = 1
	}
	m.Write([]byte{d})
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil)[:16])
}
