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
func TestTitleKeyIsStable(t *testing.T) {
	for _, s := range []string{"Straße", "ǰ̣", "ΣΊΣΥΦΟΣ", "Café", "İstanbul"} {
		k := shared.TitleKey(s)
		if again := shared.TitleKey(k); again != k {
			t.Errorf("TitleKey(TitleKey(%q)) = %q, want %q", s, again, k)
		}
	}
}
