package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
	"uuid"
)

func sampleToken() RefreshToken {
	t := RefreshToken{SessionID: uuid.MustParse("0199a2b4-7c3e-7d2a-9f10-2b3c4d5e6f70"), Generation: 0x01020304}
	for i := range t.Secret {
		t.Secret[i] = byte(0x40 + i)
	}
	for i := range t.Tag {
		t.Tag[i] = byte(0xa0 + i)
	}
	return t
}

// The 68 bytes: session id 0–15, generation 16–19 big endian, secret 20–51,
// tag 52–67 (M1/P1 design 3.4).
func TestRefreshTokenLayout(t *testing.T) {
	tok := sampleToken()
	s := tok.String()

	// A pattern a secret scanner can match.
	if !regexp.MustCompile(`^nwk_rt_[A-Za-z0-9_-]{91}$`).MatchString(s) || len(s) != refreshTokenTextLen {
		t.Fatalf("token %q does not match nwk_rt_[A-Za-z0-9_-]{91}", s)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(s, RefreshTokenPrefix))
	if err != nil || len(raw) != 68 {
		t.Fatalf("decoded %d bytes, %v; want 68", len(raw), err)
	}
	if !bytes.Equal(raw[0:16], tok.SessionID[:]) || !bytes.Equal(raw[16:20], []byte{1, 2, 3, 4}) ||
		!bytes.Equal(raw[20:52], tok.Secret[:]) || !bytes.Equal(raw[52:68], tok.Tag[:]) {
		t.Errorf("layout = % x", raw)
	}
	if !bytes.Equal(tok.MACMessage(), raw[:52]) {
		t.Errorf("MACMessage() = % x, want the first 52 bytes", tok.MACMessage())
	}
}

func TestRefreshTokenSecretHash(t *testing.T) {
	tok := sampleToken()
	want := sha256.Sum256(tok.Secret[:])
	if got := tok.SecretHash(); !bytes.Equal(got, want[:]) || len(got) != 32 {
		t.Errorf("SecretHash() = % x, want the SHA-256 of the secret", got)
	}
}

func TestParseRefreshTokenReadsWhatStringWrites(t *testing.T) {
	for _, generation := range []uint32{0, 0x01020304, math.MaxInt32} {
		tok := sampleToken()
		tok.Generation = generation

		got, ok := ParseRefreshToken(tok.String())

		if !ok || got != tok {
			t.Errorf("ParseRefreshToken(String()) at generation %d = %+v, %v; want the token back", generation, got, ok)
		}
	}
}

func TestParseRefreshTokenRejects(t *testing.T) {
	good := sampleToken().String()
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := strings.IndexByte(alphabet, good[len(good)-1])
	beyond := sampleToken()
	beyond.Generation = math.MaxInt32 + 1
	tests := []struct{ name, token string }{
		{"empty", ""},
		{"the prefix alone", RefreshTokenPrefix},
		{"another prefix", "nwk_xx_" + good[len(RefreshTokenPrefix):]},
		{"a personal access token", "nwk_pat_" + strings.Repeat("A", 43)},
		{"one character short", good[:len(good)-1]},
		{"one character more", good + "A"},
		{"padded", good[:len(good)-1] + "="},
		{"standard base64", good[:20] + "+" + good[21:]},
		{"a newline inside", good[:50] + "\n" + good[51:]},
		{"a space in front", " " + good[:len(good)-1]},
		{"a space behind", good[:len(good)-1] + " "},
		{"a newline behind", good + "\n"},
		// 91 characters carry 546 bits for 544: the last two must be zero,
		// so that one token has one spelling.
		{"unused bits set", good[:len(good)-1] + string(alphabet[last|1])},
		{"a generation beyond integer", beyond.String()},
	}
	for _, tt := range tests {
		if got, ok := ParseRefreshToken(tt.token); ok {
			t.Errorf("%s: ParseRefreshToken(%q) = %+v, want false", tt.name, tt.token, got)
		}
	}
}

func TestSanitizeUserAgent(t *testing.T) {
	long := strings.Repeat("é", 600)
	tests := []struct{ in, want string }{
		{"Mozilla/5.0", "Mozilla/5.0"},
		{"agent\x00/1", "agent/1"},
		{"bad \xff byte", "bad " + string(utf8.RuneError) + " byte"},
		{long, strings.Repeat("é", 512)},
	}
	for _, tt := range tests {
		got := SanitizeUserAgent(tt.in)
		if got != tt.want || !utf8.ValidString(got) {
			t.Errorf("SanitizeUserAgent(%.20q) = %.20q (%d runes), want %.20q", tt.in, got, utf8.RuneCountInString(got), tt.want)
		}
	}
}

// M1/P2 design 3.5's table, row by row.
func TestJudgeRefresh(t *testing.T) {
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	current := sampleToken()
	current.Generation = 5
	live := SessionState{Generation: 5, TokenHash: current.SecretHash(), ExpiresAt: now.Add(time.Hour)}
	older, newer, otherSecret := current, current, current
	older.Generation, newer.Generation = 4, 6
	otherSecret.Secret[0] ^= 1
	revoked, expired, lastInstant := live, live, live
	revoked.Revoked = true
	expired.ExpiresAt = now
	lastInstant.ExpiresAt = now.Add(time.Microsecond)
	tests := []struct {
		name     string
		state    SessionState
		token    RefreshToken
		tagValid bool
		want     Verdict
	}{
		{"current generation", live, current, false, Rotate},
		{"current generation, tag valid too", live, current, true, Rotate},
		{"current generation, another secret", live, otherSecret, true, Reject},
		{"older generation the session issued", live, older, true, Reuse},
		{"older generation, forged", live, older, false, Reject},
		{"newer generation", live, newer, true, Reject},
		{"revoked, current generation", revoked, current, false, Reject},
		{"revoked, older generation the session issued", revoked, older, true, Reject},
		{"expired at this instant", expired, current, false, Reject},
		{"expired, older generation the session issued", expired, older, true, Reject},
		{"a microsecond before expiry", lastInstant, current, false, Rotate},
	}
	for _, tt := range tests {
		if got := JudgeRefresh(tt.state, tt.token, tt.tagValid, now); got != tt.want {
			t.Errorf("%s: JudgeRefresh() = %d, want %d", tt.name, got, tt.want)
		}
	}
}
