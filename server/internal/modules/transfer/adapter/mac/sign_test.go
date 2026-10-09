package macadapter_test

import (
	"testing"
	"time"
	"uuid"

	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/mac"
)

// signKey is the bytes 0 to 31.
func signKey() []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func signJob() uuid.UUID   { return uuid.MustParse("0192b7c4-5e7a-7d2f-9b1e-3c4d5e6f7a8b") }
func signTime() time.Time  { return time.Date(2026, 10, 8, 10, 30, 0, 0, time.UTC) }
func signUntil() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }

// later is a bound on an address's expiry that the hour's comes before.
func later() time.Time { return signUntil().Add(24 * time.Hour) }

// The signature is HMAC-SHA256's first 16 bytes over "export-download",
// the job and e, in base64url without padding: a known answer, computed
// apart from this code (M7/P5 design 3.11; v0.1 design 13.1, item 25). e is
// the end of the hour after the signing's, in UTC whatever the zone.
func TestSignIsTheKnownAnswer(t *testing.T) {
	for _, at := range []time.Time{signTime(), signTime().In(time.FixedZone("CST", 8*3600))} {
		s := macadapter.New(signKey()).Sign(at, signJob(), later())
		if !s.Expires.Equal(signUntil()) || s.Expires.Location() != time.UTC || s.Signature != "T_4F1VYPnRYIUhzC0Qn4YQ" {
			t.Errorf("Sign(%v) = %+v, want until %v in UTC, T_4F1VYPnRYIUhzC0Qn4YQ", at, s, signUntil())
		}
	}
}

// An hour's addresses are the same; the next hour's expire an hour later.
func TestSignIsTheSameWithinTheHour(t *testing.T) {
	s := macadapter.New(signKey())
	first, last := s.Sign(time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC), signJob(), later()), s.Sign(time.Date(2026, 10, 8, 10, 59, 59, 0, time.UTC), signJob(), later())
	next := s.Sign(time.Date(2026, 10, 8, 11, 0, 0, 0, time.UTC), signJob(), later())
	if first != last || next.Expires.Sub(first.Expires) != time.Hour || next.Signature == first.Signature {
		t.Errorf("signed %+v, %+v, %+v; want the hour's the same, the next's an hour later", first, last, next)
	}
}

// A signature is valid for its job and expiry, until then; another job,
// expiry, key or signature is not.
func TestValid(t *testing.T) {
	s := macadapter.New(signKey())
	signed := s.Sign(signTime(), signJob(), later())
	e := signed.Expires.Unix()
	other := make([]byte, 32)
	for _, tt := range []struct {
		name  string
		ok    bool
		valid bool
	}{
		{"as signed", s.Valid(signTime(), signJob(), e, signed.Signature), true},
		{"a second before it expires", s.Valid(signed.Expires.Add(-time.Second), signJob(), e, signed.Signature), true},
		{"as it expires", s.Valid(signed.Expires, signJob(), e, signed.Signature), false},
		{"another job", s.Valid(signTime(), uuid.NewV7(), e, signed.Signature), false},
		{"another expiry", s.Valid(signTime(), signJob(), e+3600, signed.Signature), false},
		{"another key", macadapter.New(other).Valid(signTime(), signJob(), e, signed.Signature), false},
		{"another signature", s.Valid(signTime(), signJob(), e, "AAAAAAAAAAAAAAAAAAAAAA"), false},
	} {
		if tt.ok != tt.valid {
			t.Errorf("Valid(%s) = %v, want %v", tt.name, tt.ok, tt.valid)
		}
	}
}

// An address expires at until, in whole seconds, when that comes before
// the hour's end: its signature signs that expiry, valid until then.
func TestSignExpiresNoLaterThanUntil(t *testing.T) {
	s := macadapter.New(signKey())
	until := time.Date(2026, 10, 8, 11, 15, 30, 700_000_000, time.UTC)
	signed := s.Sign(signTime(), signJob(), until)
	e := signed.Expires.Unix()
	if !signed.Expires.Equal(until.Truncate(time.Second)) || signed.Signature == s.Sign(signTime(), signJob(), later()).Signature {
		t.Fatalf("Sign() = %+v, want expiring at %v, signed so", signed, until.Truncate(time.Second))
	}
	if !s.Valid(until.Add(-time.Second), signJob(), e, signed.Signature) || s.Valid(until, signJob(), e, signed.Signature) {
		t.Errorf("the address of %+v is not valid until %v alone", signed, until.Truncate(time.Second))
	}
}
