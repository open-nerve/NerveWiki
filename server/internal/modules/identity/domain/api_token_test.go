package domain

import (
	"crypto/sha256"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func samplePAT() PAT {
	var p PAT
	for i := range p {
		p[i] = byte(0x30 + i)
	}
	return p
}

// A token is nwk_pat_ and 43 characters; its hash is the SHA-256 of all 51.
func TestPATLayout(t *testing.T) {
	p := samplePAT()
	s := p.String()
	sum := sha256.Sum256([]byte(s))

	if len(s) != 51 || !strings.HasPrefix(s, PATPrefix) || !slices.Equal(p.Hash(), sum[:]) {
		t.Errorf("String() = %q (%d characters), Hash() = %x; want nwk_pat_ and 43 characters, hashed whole", s, len(s), p.Hash())
	}
}

func TestParsePATReadsWhatStringWrites(t *testing.T) {
	p := samplePAT()
	if got, ok := ParsePAT(p.String()); !ok || got != p {
		t.Errorf("ParsePAT(String()) = %x, %v; want the token back", got, ok)
	}
}

func TestParsePATRejects(t *testing.T) {
	good := samplePAT().String()
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := strings.IndexByte(alphabet, good[len(good)-1])
	tests := []struct{ name, token string }{
		{"empty", ""},
		{"the prefix alone", PATPrefix},
		{"a refresh token's prefix", "nwk_rt_" + good[len(PATPrefix):] + "A"},
		{"upper-case prefix", "NWK_PAT_" + good[len(PATPrefix):]},
		{"one character short", good[:len(good)-1]},
		{"one character more", good + "A"},
		{"padded", good[:len(good)-1] + "="},
		{"standard base64", good[:20] + "+" + good[21:]},
		{"a newline inside", good[:30] + "\n" + good[31:]},
		{"a space in front", " " + good[:len(good)-1]},
		{"a newline behind", good + "\n"},
		// 43 characters carry 258 bits for 256: the last two must be zero,
		// so that one token has one spelling.
		{"unused bits set", good[:len(good)-1] + string(alphabet[last|1])},
	}
	for _, tt := range tests {
		if got, ok := ParsePAT(tt.token); ok {
			t.Errorf("%s: ParsePAT(%q) = %x, want false", tt.name, tt.token, got)
		}
	}
}

func TestCheckAPITokenReturnsWhatIsStored(t *testing.T) {
	now := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	expires := time.Date(2026, 10, 30, 12, 0, 0, 123456789, time.FixedZone("CEST", 2*60*60))

	got, err := CheckAPIToken(APITokenSpec{Name: "  CI runner\t", ExpiresAt: &expires}, now)

	want := time.Date(2026, 10, 30, 10, 0, 0, 123456000, time.UTC)
	if err != nil || got.Name != "CI runner" || got.ExpiresAt == nil || !got.ExpiresAt.Equal(want) || got.ExpiresAt.Location() != time.UTC {
		t.Errorf("CheckAPIToken() = %+v, %v; want the name trimmed and the expiry %v in UTC, to the microsecond", got, err, want)
	}
	if never, err := CheckAPIToken(APITokenSpec{Name: "agent"}, now); err != nil || never.ExpiresAt != nil {
		t.Errorf("CheckAPIToken() without an expiry = %+v, %v; want one that never expires", never, err)
	}
}

// Every problem is reported at once, as one 422.
func TestCheckAPITokenReportsEveryField(t *testing.T) {
	now := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	// Truncated to the microsecond, this is now: not in the future.
	justAfter := now.Add(999 * time.Nanosecond)
	past := now.Add(-time.Hour)
	required := shared.FieldError{Field: "name", Code: "required", Message: "is required"}
	future := shared.FieldError{Field: "expires_at", Code: "out_of_range", Message: "must be in the future"}
	tests := []struct {
		name string
		spec APITokenSpec
		want []shared.FieldError
	}{
		{"empty name, past expiry", APITokenSpec{ExpiresAt: &past}, []shared.FieldError{required, future}},
		{"white space only", APITokenSpec{Name: " 　 "}, []shared.FieldError{required}},
		{"expiry within the microsecond of now", APITokenSpec{Name: "ci", ExpiresAt: &justAfter}, []shared.FieldError{future}},
		{"101 characters", APITokenSpec{Name: strings.Repeat("é", 101)}, []shared.FieldError{
			{Field: "name", Code: "too_long", Message: "must be at most 100 characters"},
		}},
		{"a newline inside", APITokenSpec{Name: "ci\nrunner"}, []shared.FieldError{
			{Field: "name", Code: "invalid_format", Message: "must not contain control characters"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CheckAPIToken(tt.spec, now)
			var se *shared.Error
			if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed || !slices.Equal(se.Fields, tt.want) {
				t.Errorf("CheckAPIToken() = %+v, want validation_failed with %+v", err, tt.want)
			}
		})
	}
}
