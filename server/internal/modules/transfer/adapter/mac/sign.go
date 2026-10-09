// Package macadapter signs and checks the addresses of the exports'
// archives (M7/P5 design 3.11): a reader who may read the notebook gets
// the address, which any browser then opens without a token for an hour
// or two, or until the export expires. The key is derived from the
// instance's signing key; it never leaves this package and is never
// logged (v0.1 design 13.1, item 25).
package macadapter

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
)

// Signer implements app.Signer with the download key.
type Signer struct {
	key []byte
}

// New returns the signer of key, the identity module's signing key's
// derivation for the downloads (transfer.DownloadKeyInfo).
func New(key []byte) Signer {
	return Signer{key: key}
}

// Sign signs the address of the job id's archive as of now: it expires at
// the end of the hour after now's, so an hour's addresses are the same and
// each lasts one to two hours, or at until, in whole seconds, when that
// comes first. The signature signs the expiry the address carries.
func (s Signer) Sign(now time.Time, id uuid.UUID, until time.Time) app.Signed {
	e := min((now.Unix()/3600+2)*3600, until.Unix())
	return app.Signed{Expires: time.Unix(e, 0).UTC(), Signature: s.mac(id, e)}
}

// Valid reports whether sig signs the job id's archive, expiring at e, and
// e is later than now.
func (s Signer) Valid(now time.Time, id uuid.UUID, e int64, sig string) bool {
	return e > now.Unix() && hmac.Equal([]byte(s.mac(id, e)), []byte(sig))
}

// mac is the first 16 bytes of HMAC-SHA256 over "export-download", the
// job's 16 bytes and e's 8 bytes big-endian, in base64url without padding:
// 22 characters.
func (s Signer) mac(id uuid.UUID, e int64) string {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte("export-download"))
	m.Write(id[:])
	m.Write(binary.BigEndian.AppendUint64(nil, uint64(e))) //nolint:gosec // a time after 1970
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil)[:16])
}
