package macadapter_test

import (
	"testing"
	"time"
	"uuid"

	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/mac"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
)

// signKey is the bytes 0 to 31.
func signKey() []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func signNode() uuid.UUID  { return uuid.MustParse("0192b7c4-5e7a-7d2f-9b1e-3c4d5e6f7a8b") }
func signBlob() uuid.UUID  { return uuid.MustParse("0192b7c4-6f00-7000-8000-000000000001") }
func signTime() time.Time  { return time.Date(2026, 10, 8, 10, 30, 0, 0, time.UTC) }
func signUntil() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }

// The signature is HMAC-SHA256's first 16 bytes over "asset-content", the
// node, the blob, e and d, in base64url without padding: a known answer,
// computed apart from this code (M7/P2 design 3.6; v0.1 design 13.1, item
// 25). e is the end of the hour after the signing's, in UTC whatever the
// zone of the time signed at.
func TestSignIsTheKnownAnswer(t *testing.T) {
	for _, at := range []time.Time{signTime(), signTime().In(time.FixedZone("CST", 8*3600))} {
		s := macadapter.New(signKey()).Sign(at, signNode(), signBlob())
		if !s.Expires.Equal(signUntil()) || s.Expires.Location() != time.UTC || s.Inline != "gXD47aEsSDTtgsNi8TjgwA" ||
			s.Download != "dYc8qvppWC6RLPXNbH8z-Q" {
			t.Errorf("Sign(%v) = %+v, want until %v in UTC, gXD47aEsSDTtgsNi8TjgwA and dYc8qvppWC6RLPXNbH8z-Q", at, s, signUntil())
		}
	}
}

// An hour's addresses are the same; the next hour's expire an hour later.
func TestSignIsTheSameWithinTheHour(t *testing.T) {
	sign := func(at time.Time) app.Signed {
		return macadapter.New(signKey()).Sign(at, signNode(), signBlob())
	}
	first, last := sign(time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)), sign(time.Date(2026, 10, 8, 10, 59, 59, 0, time.UTC))
	next := sign(time.Date(2026, 10, 8, 11, 0, 0, 0, time.UTC))
	if first != last || next.Expires.Sub(first.Expires) != time.Hour || next.Inline == first.Inline {
		t.Errorf("signed %+v, %+v, %+v; want the hour's the same, the next's an hour later", first, last, next)
	}
}

// Valid takes the signature of each value it was made of, and before e
// only; a change of any value, of the key, or a time at e is invalid.
func TestValidChecksEveryValueAndTheExpiry(t *testing.T) {
	s := macadapter.New(signKey()).Sign(signTime(), signNode(), signBlob())
	e := s.Expires.Unix()
	otherKey := append([]byte{}, signKey()...)
	otherKey[0] ^= 1
	for _, tt := range []struct {
		name       string
		key        []byte
		at         time.Time
		node, blob uuid.UUID
		e          int64
		download   bool
		sig        string
		want       bool
	}{
		{"shown", signKey(), signTime(), signNode(), signBlob(), e, false, s.Inline, true},
		{"downloaded", signKey(), signTime(), signNode(), signBlob(), e, true, s.Download, true},
		{"a second before e", signKey(), signUntil().Add(-time.Second), signNode(), signBlob(), e, false, s.Inline, true},
		{"at e", signKey(), signUntil(), signNode(), signBlob(), e, false, s.Inline, false},
		{"another node", signKey(), signTime(), signBlob(), signBlob(), e, false, s.Inline, false},
		{"another blob", signKey(), signTime(), signNode(), signNode(), e, false, s.Inline, false},
		{"a later e", signKey(), signTime(), signNode(), signBlob(), e + 3600, false, s.Inline, false},
		{"the shown signature downloaded", signKey(), signTime(), signNode(), signBlob(), e, true, s.Inline, false},
		{"the download's shown", signKey(), signTime(), signNode(), signBlob(), e, false, s.Download, false},
		{"another key", otherKey, signTime(), signNode(), signBlob(), e, false, s.Inline, false},
		{"a signature cut short", signKey(), signTime(), signNode(), signBlob(), e, false, s.Inline[:21], false},
		{"no signature", signKey(), signTime(), signNode(), signBlob(), e, false, "", false},
	} {
		if got := macadapter.New(tt.key).Valid(tt.at, tt.node, tt.blob, tt.e, tt.download, tt.sig); got != tt.want {
			t.Errorf("%s: Valid() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
