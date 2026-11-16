package shared_test

import (
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestTitleKeysMatchAcrossCaseAndNormalization(t *testing.T) {
	for _, tt := range []struct{ a, b string }{
		{"Engineering", "ENGINEERING"},
		{"Straße", "STRASSE"},  // full folding: ß is ss
		{"ﬁle", "FILE"},        // the ligature ﬁ is fi
		{"ΣΊΣΥΦΟΣ", "σίσυφος"}, // final sigma and sigma alike
		{"Café", "Café"},      // NFC and NFD
		{"ǰ̣", "J̣̌"},          // folding ǰ gives j and a caron before the dot below: NFC again orders them
		{"会议纪要", "会议纪要"},
		// Cherokee: its capitals and its small letters, which x/text's fold
		// swaps (M12 design 4.6.3, 25.1).
		{"\U000013E3\U000013B3\U000013A9", "\U0000ABB3\U0000AB83\U0000AB79"},
		// Folding the ypogegrammeni of an unordered sequence gives an iota that NFC
		// composes with the grave before it: NFC first orders them, as in à and
		// the ypogegrammeni.
		{"a\u0345\u0300", "\u00e0\u0345"},
	} {
		if ka, kb := shared.TitleKey(tt.a), shared.TitleKey(tt.b); ka != kb {
			t.Errorf("TitleKey(%q) = %q, TitleKey(%q) = %q; want them equal", tt.a, ka, tt.b, kb)
		}
	}
}

func TestTitleKeysKeepWhatFoldingDoesNot(t *testing.T) {
	for _, tt := range []struct{ a, b string }{
		{"Ａ", "A"}, // full width stays apart: folding is no compatibility mapping
		{"İ", "i"}, // language-independent: İ is i and a dot above
		{"a b", "ab"},
		{"Notes", "Notes 2"},
	} {
		if ka, kb := shared.TitleKey(tt.a), shared.TitleKey(tt.b); ka == kb {
			t.Errorf("TitleKey(%q) = TitleKey(%q) = %q; want them apart", tt.a, tt.b, ka)
		}
	}
}

// The key is itself NFC and folded: keying it again changes nothing.
// Cherokee too, each of its letters, a capital or small, of both blocks of
// small letters, and mixed with ASCII (M12 design 4.6.3, 25.1): x/text's
// fold alone swaps its case, so that a key's key was the other case's.
func TestTitleKeyIsStable(t *testing.T) {
	samples := []string{"Straße", "ǰ̣", "ΣΊΣΥΦΟΣ", "Café", "İstanbul",
		"\U000013A0", "\U0000AB70", "\U000013F0", "\U000013F8", "\U000013A0\U0000AB71\U000013F8x"}
	for _, block := range [][2]rune{{0x13A0, 0x13FD}, {0xAB70, 0xABBF}} {
		for r := block[0]; r <= block[1]; r++ {
			samples = append(samples, string(r))
		}
	}
	for _, s := range samples {
		k := shared.TitleKey(s)
		if again := shared.TitleKey(k); again != k {
			t.Errorf("TitleKey(TitleKey(%q)) = %q, want %q", s, again, k)
		}
	}
}
