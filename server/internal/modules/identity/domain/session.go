package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"math"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"
)

// RefreshTokenPrefix starts every refresh token (M1/P1 design 3.4).
const RefreshTokenPrefix = "nwk_rt_"

// The layout of a refresh token's 68 bytes (M1/P1 design 3.4): session id,
// generation (big endian), secret, and the MAC tag of the first 52 bytes.
const (
	refreshTokenLen = 16 + 4 + 32 + 16
	macMessageLen   = 16 + 4 + 32
	// refreshTokenTextLen is the token as the client holds it: the prefix
	// and the 68 bytes in unpadded base64, 91 characters.
	refreshTokenTextLen = len(RefreshTokenPrefix) + (refreshTokenLen*8+5)/6
)

// RefreshToken is the content of a refresh token. The server stores only
// SecretHash of the current generation; Tag proves that the server issued a
// generation without storing it.
type RefreshToken struct {
	SessionID  uuid.UUID
	Generation uint32
	Secret     [32]byte
	Tag        [16]byte
}

// MACMessage is what the tag authenticates: session id ‖ generation ‖ secret.
func (t RefreshToken) MACMessage() []byte {
	return t.bytes()[:macMessageLen]
}

// SecretHash is the SHA-256 of the secret, what auth_sessions.token_hash holds.
func (t RefreshToken) SecretHash() []byte {
	h := sha256.Sum256(t.Secret[:])
	return h[:]
}

// String is the token the client holds: nwk_rt_ and the 68 bytes in
// unpadded base64url, 91 characters.
func (t RefreshToken) String() string {
	return RefreshTokenPrefix + base64.RawURLEncoding.EncodeToString(t.bytes())
}

func (t RefreshToken) bytes() []byte {
	b := make([]byte, 0, refreshTokenLen)
	b = append(b, t.SessionID[:]...)
	b = binary.BigEndian.AppendUint32(b, t.Generation)
	b = append(b, t.Secret[:]...)
	return append(b, t.Tag[:]...)
}

// ParseRefreshToken reads a token that String wrote, without looking
// anything up. It accepts only that spelling: the prefix, then 91
// characters of unpadded base64url whose unused last bits are zero, nothing
// around them. A generation beyond auth_sessions.generation's integer is not
// one the server issued.
func ParseRefreshToken(s string) (RefreshToken, bool) {
	if len(s) != refreshTokenTextLen || !strings.HasPrefix(s, RefreshTokenPrefix) {
		return RefreshToken{}, false
	}
	// A decoder skips \r and \n: with them inside, fewer than 68 bytes come out.
	raw, err := base64.RawURLEncoding.Strict().DecodeString(s[len(RefreshTokenPrefix):])
	if err != nil || len(raw) != refreshTokenLen {
		return RefreshToken{}, false
	}
	var t RefreshToken
	copy(t.SessionID[:], raw[0:16])
	t.Generation = binary.BigEndian.Uint32(raw[16:20])
	copy(t.Secret[:], raw[20:52])
	copy(t.Tag[:], raw[52:68])
	if t.Generation > math.MaxInt32 {
		return RefreshToken{}, false
	}
	return t, true
}

// SessionState is what a refresh judges a token against: the session's row.
type SessionState struct {
	Generation uint32
	TokenHash  []byte // SHA-256 of the current generation's secret
	Revoked    bool
	ExpiresAt  time.Time
}

// Verdict is what a refresh does with the token it is given.
type Verdict int

// The verdicts of M1/P2 design 3.5's table.
const (
	// Rotate: the current generation with its secret, of a live session.
	Rotate Verdict = iota + 1
	// Reuse: an older generation that the session did issue (its tag
	// holds), of a live session: revoke the session.
	Reuse
	// Reject: anything else; the session stays as it is.
	Reject
)

// JudgeRefresh applies M1/P2 design 3.5's table to token and the state of
// its session at now. tagValid is whether token's MAC tag holds; only an
// older generation needs it: the stored hash proves the current one, which
// keeps working across a change of the signing key. A session that does not
// exist is the caller's Reject.
func JudgeRefresh(s SessionState, token RefreshToken, tagValid bool, now time.Time) Verdict {
	switch {
	case s.Revoked || !now.Before(s.ExpiresAt):
		return Reject
	case token.Generation == s.Generation && bytes.Equal(token.SecretHash(), s.TokenHash):
		return Rotate
	case token.Generation < s.Generation && tagValid:
		return Reuse
	}
	return Reject
}

// MaxUserAgentLength bounds auth_sessions.user_agent, in characters.
const MaxUserAgentLength = 512

// SanitizeUserAgent is the User-Agent a session records: valid UTF-8, no
// NUL (PostgreSQL text cannot hold it), at most 512 characters.
func SanitizeUserAgent(ua string) string {
	ua = strings.ReplaceAll(strings.ToValidUTF8(ua, string(utf8.RuneError)), "\x00", "")
	if utf8.RuneCountInString(ua) <= MaxUserAgentLength {
		return ua
	}
	return string([]rune(ua)[:MaxUserAgentLength])
}
