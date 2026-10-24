package shared_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"
	"uuid"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A client's name is cleaned in its order, and cut to 128 characters at a
// normalization boundary keeping all it can (M9/P1 design 2.1 item 2,
// 3.16.1): a character dropped before NFC leaves nothing to combine on a
// second clean; 128 characters stay when the 129th starts anew, one
// starter fewer when it combines.
func TestCleanClientName(t *testing.T) {
	x128 := strings.Repeat("x", 128)
	for _, tt := range []struct{ name, in, want string }{
		{"a plain name", "claude-code", "claude-code"},
		{"white space around", "  Codex CLI \t\n", "Codex CLI"},
		{"control characters", "a\x00b\tc\x7fd" + "\u0085e", "abcde"},
		{"line and paragraph separators, bidirectional controls", "a\u2028b\u2029c\u202ed\u2066e\u200ff", "abcdef"},
		{"a mark between a letter and its accent: dropped, then composed", "e\u200e\u0301", "\u00e9"},
		{"decomposed, composed", "Cafe\u0301", "Caf\u00e9"},
		{"a byte order mark stays: it is no control", "\ufeffname", "\ufeffname"},
		{"bytes that are no UTF-8", "a\xffb\xc3", "ab"},
		{"white space alone", " \t\n\u3000", ""},
		{"unshowable alone", "\u202e\x00", ""},
		{"exactly 128", x128, x128},
		{"129 ASCII keep 128", x128 + "y", x128},
		{"128 x and a Han character keep 128", x128 + "中", x128},
		{"129 Han characters keep 128", strings.Repeat("中", 129), strings.Repeat("中", 128)},
		{"140 dashes keep 128", strings.Repeat("-", 140), strings.Repeat("-", 128)},
		{"140 emoji keep 128", strings.Repeat("\U0001F600", 140), strings.Repeat("\U0001F600", 128)},
		{"127 a, an e and its accent, an x keep 128", strings.Repeat("a", 127) + "e\u0301x", strings.Repeat("a", 127) + "\u00e9"},
		{"cut between combining characters: their starter goes", strings.Repeat("a", 127) + "e\u0301\u0302", strings.Repeat("a", 127)},
		{"white space at the cut goes", strings.Repeat("a", 127) + " b", strings.Repeat("a", 127)},
	} {
		got := shared.CleanClientName(tt.in)
		if got != tt.want {
			t.Errorf("%s: CleanClientName(%q) = %q (%d characters), want %q", tt.name, tt.in, got, utf8.RuneCountInString(got), tt.want)
		}
		if again := shared.CleanClientName(got); again != got {
			t.Errorf("%s: cleaned again %q, not %q", tt.name, again, got)
		}
	}
}

// A clean of random code points is idempotent, at most 128 characters,
// NFC, without an unshowable character or white space around it.
func TestCleanClientNameIsIdempotent(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	ranges := [][2]rune{{0, 0x7f}, {0x80, 0x24f}, {0x300, 0x36f}, {0x200b, 0x206f}, {0x3000, 0x30ff}, {0x4e00, 0x4e3f},
		{0x1100, 0x11ff}, {0xac00, 0xac3f}, {0xfe00, 0xfeff}, {0x1f600, 0x1f64f}}
	for i := range 100000 {
		var b strings.Builder
		for range r.IntN(200) {
			rg := ranges[r.IntN(len(ranges))]
			b.WriteRune(rg[0] + rune(r.IntN(int(rg[1]-rg[0]+1))))
		}
		if r.IntN(10) == 0 {
			b.WriteByte(0xff)
		}
		s := shared.CleanClientName(b.String())
		switch {
		case shared.CleanClientName(s) != s:
			t.Fatalf("#%d: %q cleans to %q, then %q", i, b.String(), s, shared.CleanClientName(s))
		case utf8.RuneCountInString(s) > shared.MaxClientNameChars:
			t.Fatalf("#%d: %q is %d characters", i, s, utf8.RuneCountInString(s))
		case !norm.NFC.IsNormalString(s):
			t.Fatalf("#%d: %q is not NFC", i, s)
		case strings.ContainsFunc(s, shared.Unshowable):
			t.Fatalf("#%d: %q holds an unshowable character", i, s)
		case strings.TrimSpace(s) != s:
			t.Fatalf("#%d: %q has white space around it", i, s)
		}
	}
}

// A long name of starters alone, letters, Han, Hangul, digits, a dash, a
// dot and emoji, keeps exactly 128 characters: the cut takes none the
// bound holds (M9/P1 design review A-M5, its check A N-1).
func TestCleanClientNameKeepsAllItCan(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	starters := []rune{'a', 'Z', 0x4e2d, 0xac00, 0x1f600, '-', '.', '1', 0xe9, 0x3042}
	for i := range 20000 {
		var b strings.Builder
		for range 129 + r.IntN(100) {
			b.WriteRune(starters[r.IntN(len(starters))])
		}
		name := norm.NFC.String(b.String())
		if utf8.RuneCountInString(strings.TrimSpace(name)) < 129 {
			continue
		}
		if got := utf8.RuneCountInString(shared.CleanClientName(name)); got != shared.MaxClientNameChars {
			t.Fatalf("#%d: %q keeps %d characters, want 128", i, name, got)
		}
	}
}

// The client is the credential's (M10/P1 design 3.7): a sign-in session's
// request is the web's, a personal access token's the API's; a request
// without an actor is 401, as RequireActor answers.
func TestClientOfIsTheCredential(t *testing.T) {
	user := uuid.NewV7()
	for _, tt := range []struct {
		name  string
		actor shared.Actor
		want  string
	}{
		{"a sign-in session", shared.Actor{UserID: user, SessionID: uuid.NewV7()}, "web"},
		{"a personal access token", shared.Actor{UserID: user, APITokenID: uuid.NewV7()}, "api"},
	} {
		if got, err := shared.ClientOf(shared.WithActor(context.Background(), tt.actor)); err != nil || got != tt.want {
			t.Errorf("%s: ClientOf() = %q, %v; want %q", tt.name, got, err, tt.want)
		}
	}
	if got, err := shared.ClientOf(context.Background()); !errors.Is(err, shared.Unauthenticated()) || got != "" {
		t.Errorf("ClientOf() without an actor = %q, %v; want 401", got, err)
	}
}
