package domain

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func checkCode(t *testing.T, rules *PasswordRules, password, email string) string {
	t.Helper()
	if f := rules.Check("password", password, email); f != nil {
		return f.Code
	}
	return ""
}

// Only the length counts: no composition rules (M1 design 4).
func TestPasswordLength(t *testing.T) {
	rules := NewPasswordRules()
	tests := []struct{ name, password, want string }{
		{"empty", "", shared.FieldRequired},
		{"7 characters", "xq7vbnz", shared.FieldTooShort},
		{"8 characters", "xq7vbnzk", ""},
		{"lower-case letters alone", "vbnzkqxjwm", ""},
		{"a passphrase", "correct horse battery staple", ""},
		{"128 characters", strings.Repeat("xq7vbnzk", 16), ""},
		{"129 characters", strings.Repeat("xq7vbnzk", 16) + "w", shared.FieldTooLong},
		// Length is UTF-16 code units, like password.length in the web app.
		{"four emoji make 8 units", "😀🎉🔑🌊", ""},
		{"three emoji and a letter make 7 units", "😀🎉🔑w", shared.FieldTooShort},
		{"64 emoji are 128 units", strings.Repeat("😀🎉", 32), ""},
		{"64 emoji and a letter are 129 units", strings.Repeat("😀🎉", 32) + "w", shared.FieldTooLong},
		{"white space counts and stays", "   xq7 vbnz  ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkCode(t, rules, tt.password, "someone@example.com"); got != tt.want {
				t.Errorf("Check(%q) = %q, want %q (%d UTF-16 units)", tt.password, got, tt.want, len(utf16.Encode([]rune(tt.password))))
			}
		})
	}
}

// The list catches a common password as it is, in any case, and decorated
// with non-letters at its ends.
func TestCommonPasswords(t *testing.T) {
	rules := NewPasswordRules()
	for _, p := range []string{
		"password", "Password1!", "P@ssw0rd", "QWERTY123", "iloveyou2", "12345678", "Sunshine!",
		"Password1!~", "~Password1!", "Password1! ", "Football1!", "2024#Baseball#",
	} {
		if got := checkCode(t, rules, p, "someone@example.com"); got != shared.FieldCommonPassword {
			t.Errorf("Check(%q) = %q, want common_password", p, got)
		}
	}
	// A common word decorated with digits and symbols: the core of a
	// common password (tools/password-blocklist keeps the cores).
	for _, p := range []string{
		"Qwerty123!", "Summer2024!", "Welcome1!", "Letmein1!", "Admin123!", "Monkey123!", "Dragon2024!", "Michael1!",
	} {
		if got := checkCode(t, rules, p, "someone@example.com"); got != shared.FieldCommonPassword {
			t.Errorf("Check(%q) = %q, want common_password", p, got)
		}
	}
	for _, p := range []string{"Tr0ub4dor&3", "Correct-Horse-9", "correcthorsebatterystaple", "xq7vbnzk", "Love2026!!"} {
		if got := checkCode(t, rules, p, "someone@example.com"); got != "" {
			t.Errorf("Check(%q) = %q, want accepted", p, got)
		}
	}
}

// One character repeated, or blanks and control characters alone, are
// common whatever the list holds.
func TestTrivialPasswords(t *testing.T) {
	rules := NewPasswordRules()
	for _, p := range []string{
		"        ", "\t\t\t\t\t\t\t\t", " \t \n \r  ", strings.Repeat("\x00", 8), "\u3000\u3000\u3000\u3000\u3000\u3000\u3000\u3000",
		"ééééééééé", strings.Repeat("😀", 4), "zzzzzzzzzzzz",
	} {
		if got := checkCode(t, rules, p, "someone@example.com"); got != shared.FieldCommonPassword {
			t.Errorf("Check(%q) = %q, want common_password", p, got)
		}
	}
	for _, p := range []string{"zzzzzzzzzzzy", "        x", "\x00\x00\x00\x00\x00\x00\x00k"} {
		if got := checkCode(t, rules, p, "someone@example.com"); got != "" {
			t.Errorf("Check(%q) = %q, want accepted", p, got)
		}
	}
}

func TestPasswordWhoseCoreIsTheEmailsLocalPart(t *testing.T) {
	rules := NewPasswordRules()
	tests := []struct{ password, email, want string }{
		{"Liuwei123!", "liuwei@example.com", shared.FieldCommonPassword},
		{"Liuwei123!", "someone@example.com", ""},
		// Only the ends are trimmed: the dot inside stays in both cores.
		{"~Zhang.San9", "zhang.san@example.com", shared.FieldCommonPassword},
		{"2026liuwei!!", "9liuwei9@example.com", shared.FieldCommonPassword},
		// A quoted local part holds an @: the domain starts after the last.
		{"A@b-2026!", `"a@b"@example.com`, shared.FieldCommonPassword},
		{"a-2026!!", `"a@b"@example.com`, ""},
		{"12345678", "1234@example.com", shared.FieldCommonPassword}, // listed; no core to compare
		{"87654321x", "x@example.com", shared.FieldCommonPassword},
	}
	for _, tt := range tests {
		if got := checkCode(t, rules, tt.password, tt.email); got != tt.want {
			t.Errorf("Check(%q, %q) = %q, want %q", tt.password, tt.email, got, tt.want)
		}
	}
}

func TestCore(t *testing.T) {
	for in, want := range map[string]string{
		"Password1!~":  "password",
		"~Password1!":  "password",
		"Password1! ":  "password",
		"12!Pass-word": "pass-word",
		"Ünïcödé9!":    "ünïcödé",
		"2024!!":       "",
	} {
		if got := core(in); got != want {
			t.Errorf("core(%q) = %q, want %q", in, got, want)
		}
	}
}

// The list is what tools/password-blocklist/build.mjs writes: the header
// names the pinned source, its checksum and its licence; the entries are
// sorted by bytes, unique and lowercase, and each is 8–128 UTF-16 units
// long, the only ones a password can equal, or a core of 5 units or more.
func TestCommonPasswordList(t *testing.T) {
	header, _, _ := strings.Cut(commonPasswordsFile, "\n\n")
	for _, want := range []string{
		"SecLists/1a7bb9127eca9e6ff2fc0301c597fe6e16a0cb56/",
		"SHA-256 c2e5696882c603b76bb67a47ee970897e5a76fc4c3f5547abe3d0ca340c576e0",
		"Contains public sector information licensed under the Open Government Licence v3.0:",
		"https://www.nationalarchives.gov.uk/doc/open-government-licence/version/3/",
	} {
		if !strings.Contains(header, want) {
			t.Errorf("the list's header lacks %q", want)
		}
	}
	list := NewPasswordRules().common
	if len(list) != 67396 {
		t.Errorf("the list has %d entries, want 67396", len(list))
	}
	if !slices.IsSorted(list) || len(slices.Compact(slices.Clone(list))) != len(list) {
		t.Error("the list is not sorted by bytes without duplicates")
	}
	for _, e := range list {
		units := len(utf16.Encode([]rune(e)))
		password := units >= MinPasswordLength && units <= MaxPasswordLength
		if strings.ToLower(e) != e || (!password && (core(e) != e || units < 5)) {
			t.Errorf("entry %q is not lowercase, or neither 8-128 units long nor a core of 5 units or more", e)
		}
	}
}
