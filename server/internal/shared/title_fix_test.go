package shared_test

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestFixTitle(t *testing.T) {
	long := strings.Repeat("a", 300)
	for _, tt := range []struct {
		in        string
		extension bool
		want      string
	}{
		{"Engineering", false, "Engineering"},
		{"a/b\\c:d*e?f\"g<h>i|j#k^l[m]n", false, "a_b_c_d_e_f_g_h_i_j_k_l_m_n"},
		{"a\x00b\tc\u007fd\u2028e\u2029f\u202eg\u2066h", false, "a_b_c_d_e_f_g_h"},
		// Not UTF-8: a replacement character.
		{"a\xffb", false, "a\ufffdb"},
		// NFD in, NFC out.
		{"Cafe\u0301", false, "Café"},
		{"  . .notes. . ", false, "notes"},
		{"...", false, ""},
		{" \t", false, ""},
		{"", false, ""},
		{"con", false, "con_"},
		{"CON.txt", true, "CON_.txt"},
		{"nul.tar.gz", true, "nul_.tar.gz"},
		{"com¹", false, "com¹_"},
		{"console", false, "console"},
		// Too long: cut at a character, the extension kept.
		{long, false, long[:255]},
		{long + ".png", true, long[:251] + ".png"},
		{long + ".png", false, long[:255]},
		{strings.Repeat("名", 90) + ".png", true, strings.Repeat("名", 83) + ".png"},
		{strings.Repeat("名", 90), false, strings.Repeat("名", 85)},
		// An extension as long as the bytes: the whole is cut.
		{"a." + long, true, ("a." + long)[:255]},
		// A cut that leaves a dot or a blank at the end drops it.
		{strings.Repeat("a", 254) + ". b", false, strings.Repeat("a", 254)},
		// A stem whose first character does not fit beside the extension:
		// the whole is cut.
		{"名." + strings.Repeat("e", 253), true, ("名." + strings.Repeat("e", 253))[:255]},
		// A reserved name made longer than the bytes keeps what of it fits.
		{"con." + strings.Repeat("e", 252), true, "co." + strings.Repeat("e", 252)},
		// A cut that would leave a reserved name.
		{"conx." + strings.Repeat("e", 251), true, "con_." + strings.Repeat("e", 250)},
		{"\tnotes\t", false, "notes"},
	} {
		if got := shared.FixTitle(tt.in, tt.extension); got != tt.want {
			t.Errorf("FixTitle(%q, %t) = %q, want %q", tt.in, tt.extension, got, tt.want)
		}
	}
}

// Whatever it is given, FixTitle answers "" or a title CheckTitle passes
// unchanged; a title CheckTitle passes is its own answer.
func TestFixTitleAlwaysAnswersATitle(t *testing.T) {
	pieces := []string{
		"a", "Z", "名", "é", "e\u0301", "\u0301", " ", "\t", ".", "..", "/", "\\", ":", "*", "?", "\"", "<", ">", "|", "#", "^",
		"[", "]", "\x00", "\x7f", "\xff", "\xe5\x9b", "\u2028", "\u202e", "\u200d", "con", "CON", "aux", "LPT1", "com¹",
		".md", ".png", "\U0001F468", strings.Repeat("x", 120), strings.Repeat("名", 40), "ß", "\u0130",
	}
	r := rand.New(rand.NewPCG(7, 11))
	for range 20000 {
		var b strings.Builder
		for range r.IntN(12) {
			b.WriteString(pieces[r.IntN(len(pieces))])
		}
		in, extension := b.String(), r.IntN(2) == 0
		got := shared.FixTitle(in, extension)
		if got == "" {
			continue
		}
		checked, f := shared.CheckTitle("name", got)
		if f != nil || checked != got {
			t.Fatalf("FixTitle(%q, %t) = %q, which CheckTitle answers %q, %+v", in, extension, got, checked, f)
		}
		if again := shared.FixTitle(got, extension); again != got {
			t.Fatalf("FixTitle(%q, %t) = %q, a title, but FixTitle of it is %q", in, extension, got, again)
		}
	}
}
