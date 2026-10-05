package markdown

import "testing"

// SafeURL's allowlist (M4/P3 design 3.8).
func TestSafeURLLetsThroughWebMailAndThisSite(t *testing.T) {
	tests := []struct {
		raw, want string
		ok        bool
	}{
		{"https://x.example/a?b#c", "https://x.example/a?b#c", true},
		{"HTTP://x.example", "HTTP://x.example", true},
		{"http:\\\\x.example", "http:\\\\x.example", true},
		{"mailto:a@b.example", "mailto:a@b.example", true},
		{"MailTo:a@b.example", "MailTo:a@b.example", true},
		{"/a/b", "/a/b", true},
		{"../a", "../a", true},
		{"a/b:c", "a/b:c", true},
		{"?a", "?a", true},
		{"#a", "#a", true},
		{"", "", true},
		{" \x01/a\x1f ", "/a", true},
		{"/a\tb\nc\rd", "/abcd", true},
		{"https:x.example", "", false},
		{"https:/x.example", "", false},
		{"https://", "", false},
		{"javascript:alert(1)", "", false},
		{"JavaScript:alert(1)", "", false},
		{" javascript:alert(1)", "", false},
		{"java\tscript:alert(1)", "", false},
		{"java\nscript:alert(1)", "", false},
		{"\x00javascript:alert(1)", "", false},
		{"vbscript:x", "", false},
		{"data:text/html,x", "", false},
		{"file:///etc/passwd", "", false},
		{"a:b", "", false},
		{"//x.example", "", false},
		{"/\\x.example", "", false},
		{"\\\\x.example", "", false},
		{"\\/x.example", "", false},
		{" //x.example", "", false},
		{"/\t/x.example", "", false},
		{"/a\x01b", "", false},
		{"/a\x7fb", "", false},
		{"%zz", "", false},
		{"mailto:a\x01b@c.example", "", false},
	}
	for _, tt := range tests {
		got, ok := SafeURL(tt.raw)
		if got != tt.want || ok != tt.ok {
			t.Errorf("SafeURL(%q) = %q, %v; want %q, %v", tt.raw, got, ok, tt.want, tt.ok)
		}
	}
}

// DecodeURI is JavaScript's decodeURI (the fixtures' rule 8).
func TestDecodeURIIsJavaScripts(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"a.md", "a.md"},
		{"c%20d.md", "c d.md"},
		{"%E4%B8%AD", "中"},
		{"%e4%b8%ad%2F", "中%2F"},
		{"a%2Fb%3F%23%25", "a%2Fb%3F%23%"},
		{"%41%3b", "A%3b"},
		{"%F0%9F%98%80", "😀"},
		{"%", "%"},
		{"%4", "%4"},
		{"%ZZ x", "%ZZ x"},
		{"%E4%B8 x", "%E4%B8 x"},               // a character cut short
		{"%C0%80", "%C0%80"},                   // an overlong form
		{"%ED%A0%80", "%ED%A0%80"},             // a surrogate
		{"%80", "%80"},                         // a continuation byte alone
		{"ok %20 then %FF", "ok %20 then %FF"}, // one bad escape keeps all
		{"%ZZ%20", "%ZZ%20"},
	} {
		if got := DecodeURI(tt.in); got != tt.want {
			t.Errorf("DecodeURI(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
