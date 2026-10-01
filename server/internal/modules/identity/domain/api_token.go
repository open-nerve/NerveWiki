package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// PATPrefix starts every personal access token (v0.1 design 2.3).
const PATPrefix = "nwk_pat_"

// PAT is the random part of a personal access token: 32 bytes. The server
// stores only Hash.
type PAT [32]byte

// patTextLen is the token as the client holds it: the prefix and 43
// characters of unpadded base64url, 51.
const patTextLen = len(PATPrefix) + (32*8+5)/6

// String is the token the client holds.
func (p PAT) String() string {
	return PATPrefix + base64.RawURLEncoding.EncodeToString(p[:])
}

// Hash is what api_tokens.token_hash holds: the SHA-256 of the whole token.
// The token is 256 random bits: no slow hash is needed (M1/P3 design 3.2).
func (p PAT) Hash() []byte {
	sum := sha256.Sum256([]byte(p.String()))
	return sum[:]
}

// ParsePAT reads a token that String wrote, without looking anything up. It
// accepts only that spelling: the prefix, then 43 characters of unpadded
// base64url whose unused last bits are zero, nothing around them.
func ParsePAT(s string) (PAT, bool) {
	if len(s) != patTextLen || !strings.HasPrefix(s, PATPrefix) {
		return PAT{}, false
	}
	// A decoder skips \r and \n: with them inside, fewer than 32 bytes come out.
	raw, err := base64.RawURLEncoding.Strict().DecodeString(s[len(PATPrefix):])
	if err != nil || len(raw) != len(PAT{}) {
		return PAT{}, false
	}
	return PAT(raw), true
}

// APIToken is a personal access token as the API lists it: never the token
// itself.
type APIToken struct {
	ID         uuid.UUID
	Name       string
	ExpiresAt  *time.Time // nil: never expires
	LastUsedAt *time.Time // nil: never used
	CreatedAt  time.Time
}

// APITokenSpec is what the caller asks for when creating a token.
type APITokenSpec struct {
	Name      string
	ExpiresAt *time.Time // nil: never expires
}

// MaxAPITokenNameLength is api_tokens.name's varchar(100), in characters.
const MaxAPITokenNameLength = 100

// CheckAPIToken checks spec at now (M1/P3 design 3.2) and returns it as the
// database stores it: the name trimmed, the expiry in UTC and truncated to
// the microsecond. The check, the row and the answer all use that expiry,
// so the time a token is created with reads back unchanged. Every problem
// is reported at once, as one 422 validation_failed.
func CheckAPIToken(spec APITokenSpec, now time.Time) (APITokenSpec, error) {
	var fields []shared.FieldError
	name, f := shared.CheckName("name", spec.Name, MaxAPITokenNameLength)
	if f != nil {
		fields = append(fields, *f)
	}
	var expires *time.Time
	if spec.ExpiresAt != nil {
		at := spec.ExpiresAt.UTC().Truncate(time.Microsecond)
		expires = &at
		if !at.After(now) {
			fields = append(fields, shared.FieldError{Field: "expires_at", Code: shared.FieldOutOfRange, Message: "must be in the future"})
		}
	}
	if len(fields) > 0 {
		return APITokenSpec{}, shared.Invalid(fields...)
	}
	return APITokenSpec{Name: name, ExpiresAt: expires}, nil
}
