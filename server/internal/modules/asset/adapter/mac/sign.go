// Package macadapter signs and checks the addresses of the attachments'
// contents (M7 design 4.5; M7/P2 design 3.6): a reader who may read an
// attachment gets its address, which any browser then opens without a
// token for an hour or two. The key is derived from the instance's signing
// key; it never leaves this package and is never logged (v0.1 design 13.1,
// item 25).
package macadapter

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
)

// Signer implements app.Signer with the content key.
type Signer struct {
	key []byte
}

// New returns the signer of key, the identity module's signing key's
// derivation for the contents (asset.ContentKeyInfo).
func New(key []byte) Signer {
	return Signer{key: key}
}

// Sign signs the address of node's file blob as of now: it expires at the
// end of the hour after now's, so an hour's addresses are the same and
// each lasts one to two hours.
func (s Signer) Sign(now time.Time, node, blob uuid.UUID) app.Signed {
	e := (now.Unix()/3600 + 2) * 3600
	return app.Signed{Expires: time.Unix(e, 0).UTC(), Inline: s.mac(node, blob, e, false), Download: s.mac(node, blob, e, true)}
}

// Valid reports whether sig signs node's file blob, expiring at e, shown
// or downloaded, and e is later than now.
func (s Signer) Valid(now time.Time, node, blob uuid.UUID, e int64, download bool, sig string) bool {
	return e > now.Unix() && hmac.Equal([]byte(s.mac(node, blob, e, download)), []byte(sig))
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
