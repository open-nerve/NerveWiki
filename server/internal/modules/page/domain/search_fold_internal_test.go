package domain

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// searchFoldBefore is searchFold as page/domain/search.go wrote it at
// 194ca350, before M12/P3/S1 deleted it, on shared.TitleKey as
// shared/title_key.go wrote it there: copied, not called, so that the
// comparison judges the key the search folds by now against the one it
// folded by then. Calling shared.TitleKey here would compare the new key
// with itself.
func searchFoldBefore(s string) string {
	key := strings.ToLower(s)
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			key = norm.NFC.String(cases.Fold().String(norm.NFC.String(s)))
			break
		}
	}
	return strings.Map(func(r rune) rune {
		switch {
		case 0xAB70 <= r && r <= 0xABBF:
			return r - 0xAB70 + 0x13A0
		case 0x13F8 <= r && r <= 0x13FD:
			return r - 0x13F8 + 0x13F0
		}
		return r
	}, key)
}

// The search folds its lexemes and its queries' keys by shared.TitleKey
// now, which takes Cherokee's step, where it took searchFold, TitleKey and
// the step after it (M12 design 4.6.3, 25.1): the two are byte for byte
// the same, so the tokenizer's version stays and no row is rebuilt. Every
// code point alone; each Cherokee letter with its other case, before and
// after it; words of Cherokee mixed with ASCII; and the names and queries
// of search_test.go: Greek's sigmas, Turkish İ, German ß, a ligature, a
// decomposed accent, CJK. The inputs are UTF-8, as every name (CheckTitle),
// content (CheckContent) and query (ParseSearchQuery) the search folds is:
// on a byte that is no UTF-8 the two differ, searchFold's strings.Map
// writing U+FFFD where the key keeps the byte.
func TestSearchFoldIsTheTitleKeyNow(t *testing.T) {
	corpus := []string{
		"\U000013A0\U0000AB71\U000013F8x", "Project \U0000ABB3\U0000AB83\U0000AB79 MIX", "\U0000AB70 Straße \U000013F8",
		"\U000013E3\U000013B3\U000013A9 \U000013A6\U000013EC\U000013C2\U000013AF\U000013CD\U000013D7 \U000013F0\U000013F4",
		"ΣΊΣΥΦΟΣ", "σίσυφος", "İstanbul", "STRASSE straße", "Straße", "ﬃ ß ǅ", "ﬁle", "Cafe\U00000301", "中国历史 数据库",
		"Go语言 学习go", "Kubernetes", "node.js", "a\U00000345\U00000300",
	}
	corpus = append(corpus, strings.Map(unicode.ToLower, corpus[3]))
	cherokee := 0
	for _, block := range [][2]rune{{0x13A0, 0x13FD}, {0xAB70, 0xABBF}} {
		for r := block[0]; r <= block[1]; r++ {
			other := unicode.SimpleFold(r)
			corpus = append(corpus, string(r)+string(other), string(other)+string(r))
			cherokee++
		}
	}
	if cherokee != 94+80 {
		t.Fatalf("%d Cherokee code points paired, want the 94 of U+13A0-13FD and the 80 of U+AB70-ABBF", cherokee)
	}
	for _, s := range corpus {
		if got, want := shared.TitleKey(s), searchFoldBefore(s); got != want {
			t.Errorf("TitleKey(%q) = %q, searchFold's was %q", s, got, want)
		}
	}
	for r := range unicode.MaxRune + 1 {
		if 0xD800 <= r && r <= 0xDFFF {
			continue // a surrogate is no UTF-8
		}
		if got, want := shared.TitleKey(string(r)), searchFoldBefore(string(r)); got != want {
			t.Errorf("TitleKey(%U) = %q, searchFold's was %q", r, got, want)
		}
	}
}
