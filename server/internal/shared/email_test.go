package shared_test

import (
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestNormalizeEmail(t *testing.T) {
	for in, want := range map[string]string{
		"  Alice@Corp.COM\t\n": "alice@corp.com",
		"ÉLODIE@EXÄMPLE.COM":   "élodie@exämple.com",
		"\xc2\xa0bob@corp.com": "bob@corp.com", // U+00A0: TrimSpace takes it too
	} {
		if got := shared.NormalizeEmail(in); got != want {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

// The cases follow Django 5.2's own tests of EmailValidator
// (tests/validators/tests.py) where they apply to normalized addresses.
func TestValidEmail(t *testing.T) {
	validEmails := []string{
		"email@here.com",
		"weirder-email@here.and.there.com",
		"email@[127.0.0.1]",
		"email@[2001:dB8::1]",
		"email@[2001:db8:0:0:0:0:0:1]",
		"email@[::fffF:127.0.0.1]",
		"example@valid-----hyphens.com",
		"example@valid-with-hyphens.com",
		"test@domain.with.idn.tld.उदाहरण.परीक्षा",
		"email@localhost",
		`"test@test"@example.com`,
		"example@atm." + strings.Repeat("a", 63),
		"example@" + strings.Repeat("a", 63) + ".atm",
		"example@" + strings.Repeat("a", 63) + "." + strings.Repeat("b", 10) + ".atm",
		"a.b+c@sub.example.co.uk",
		"x@xn--80ak6aa92e.xn--p1ai",
		"elodie@exämple.com",
		"ıſ@example.com", // Python's IGNORECASE matches ı and ſ with [A-Z]
	}
	invalidEmails := []string{
		"",
		"abc",
		"abc@",
		"@abc.com",
		"a @x.cz",
		"abc@.com",
		"something@@somewhere.com",
		"email@127.0.0.1",
		"email@[127.0.0.256]",
		"email@[2001:db8::12345]",
		"email@[2001:db8:0:0:0:0:1]",
		"email@[::ffff:127.0.0.256]",
		"email@[2001:dg8::1]",
		"email@[2001:dG8:0:0:0:0:0:1]",
		"email@[::fTzF:127.0.0.1]",
		"example@invalid-.com",
		"example@-invalid.com",
		"example@invalid.com-",
		"example@inv-.alid-.com",
		"example@inv-.-alid.com",
		"test@example.com\n\n<script src=\"x.js\">",
		"\"\\\t\"@here.com", // an escaped tab: Django accepts it, control characters are refused first
		"trailingdot@shouldfail.com.",
		"a@b.com\n",
		"a\n@b.com",
		`"test@test"\n@example.com`,
		"a@[127.0.0.1]\n",
		"example@atm." + strings.Repeat("a", 64),
		"example@" + strings.Repeat("b", 64) + ".atm.localhost",
		"example@atm." + strings.Repeat("a", 59) + "xn--", // a TLD ends in a letter
		"a@b",
		"a@b.c",
		"a@b.c0m",
		"a..b@c.com",
		".a@c.com",
		"a.@c.com",
		"élodie@exämple.com", // Django's local part is ASCII
		"é@x.com",
		"a@😀.com",
		`"a b"@example.com`,
		"a@ex\xe3\x80\x80ample.com", // U+3000: Django accepts it in a domain; white space is refused first
		// Format characters, which Django accepts in a domain: bidirectional
		// controls, zero-width characters, the soft hyphen.
		"alice@exa\u202emple.com",
		"alice@example.c\u202eom",
		"alice@ex\u2066ample.com",
		"alice@exa\u200bmple.com",
		"alice@exa\u200dmple.com",
		"alice@exa\u00admple.com",
		strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 60) + ".com",
	}
	for _, e := range validEmails {
		if !shared.ValidEmail(e) {
			t.Errorf("ValidEmail(%q) = false, want true", e)
		}
	}
	for _, e := range invalidEmails {
		if shared.ValidEmail(e) {
			t.Errorf("ValidEmail(%q) = true, want false", e)
		}
	}
}

func TestValidEmailLengthLimit(t *testing.T) {
	address := func(last int) string {
		return strings.Repeat("a", 64) + "@" + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", last) + ".com"
	}
	if at255 := address(58); len(at255) != 255 || !shared.ValidEmail(at255) {
		t.Errorf("a %d-character address: valid = %v, want true", len(at255), shared.ValidEmail(at255))
	}
	// Longer than users.email holds, though Django allows 320.
	if at256 := address(59); len(at256) != 256 || shared.ValidEmail(at256) {
		t.Errorf("a %d-character address is valid, want invalid", len(at256))
	}
}

func TestCheckEmail(t *testing.T) {
	tests := []struct {
		email string
		want  *shared.FieldError
	}{
		{"bob@corp.com", nil},
		{"", &shared.FieldError{Field: "to", Code: shared.FieldRequired, Message: "is required"}},
		{strings.Repeat("a", 250) + "@c.com", &shared.FieldError{Field: "to", Code: shared.FieldTooLong, Message: "must be at most 255 characters"}},
		{"bob", &shared.FieldError{Field: "to", Code: shared.FieldInvalidFormat, Message: "is not a valid e-mail address"}},
	}
	for _, tt := range tests {
		got := shared.CheckEmail("to", tt.email)
		if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
			t.Errorf("CheckEmail(%q) = %+v, want %+v", tt.email, got, tt.want)
		}
	}
}
