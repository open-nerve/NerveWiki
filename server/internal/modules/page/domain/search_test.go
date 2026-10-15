package domain_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
	"uuid"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

func ptr[T any](v T) *T { return &v }

// A clause of Thai and one of Khmer, of 73 and 71 code points, letters and
// marks: their scripts part no words by spaces, each clause one run of
// word characters (M8/P5A review A-I1).
const (
	thaiClause  = "ภาษาไทยเป\u0e47นภาษาท\u0e35\u0e48ม\u0e35ระด\u0e31บเส\u0e35ยงของคำแน\u0e48นอนหร\u0e37อวรรณย\u0e38กต\u0e4cเช\u0e48นเด\u0e35ยวก\u0e31บภาษาจ\u0e35น"
	khmerClause = "ភ\u17b6ស\u17b6ខ\u17d2ម\u17c2រគ\u17baជ\u17b6ភ\u17b6ស\u17b6ក\u17c6ណ\u17beតរបស\u17cbជនជ\u17b6ត\u17b7ខ\u17d2ម\u17c2រន\u17b7ងជ\u17b6ភ\u17b6ស\u17b6ផ\u17d2ល\u17bcវក\u17b6ររបស\u17cbប\u17d2រទ\u17c1សកម\u17d2ព\u17bbជ\u17b6"
)

// trigramsOf are the trigrams of run, a run of word characters, in the
// order they first occur.
func trigramsOf(run string) []string {
	r := []rune(run)
	var out []string
	for i := 0; i+3 <= len(r); i++ {
		if x := string(r[i : i+3]); !slices.Contains(out, x) {
			out = append(out, x)
		}
	}
	return out
}

// The lexemes of a name and a page (M8/P5 design 3.3): the title holds a
// CJK run's characters and bigrams, the body its bigrams alone; both a
// word run's trigrams, or the run of one or two; the body the name's too.
func TestSearchTokens(t *testing.T) {
	for _, tt := range []struct {
		name, content string
		title, body   []string
	}{
		{"中国历史", "数据库", []string{"中", "国", "中国", "历", "国历", "史", "历史"}, []string{"中国", "国历", "历史", "数据", "据库"}},
		{"锅", "一口锅", []string{"锅"}, []string{"一口", "口锅"}},
		{"Kubernetes", "go ai", []string{"kub", "ube", "ber", "ern", "rne", "net", "ete", "tes"},
			[]string{"kub", "ube", "ber", "ern", "rne", "net", "ete", "tes", "go", "ai"}},
		{"x", "Go语言 学习go", []string{"x"}, []string{"x", "go", "语言", "学习"}},
		// Folded first: ß is two letters, ﬃ three; one with case keys as its fold.
		{"Straße", "ﬃ ß ǅ", []string{"str", "tra", "ras", "ass", "sse"}, []string{"str", "tra", "ras", "ass", "sse", "ffi", "ss", "ǆ"}},
		// The marks are of the word: Thai's above and below, Devanagari's
		// vowel signs and virama, which part none.
		{"สวัสดี", "हिन्दी", []string{"สวั", "วัส", "ัสด", "สดี"}, []string{"สวั", "วัส", "ัสด", "สดี", "हिन", "िन्", "न्द", "्दी"}},
		// The long mark and 〆 are of the Japanese word; Hangul is CJK.
		{"ラーメン", "〆切 한국", []string{"ラ", "ー", "ラー", "メ", "ーメ", "ン", "メン"},
			[]string{"ラー", "ーメ", "メン", "〆切", "한국"}},
		// Punctuation, symbols and emoji part words and runs alike.
		{"a-b", "c++ 😀d", []string{"a", "b"}, []string{"a", "b", "c", "d"}},
		// A clause of a script that parts no words by spaces, past
		// MaxSearchWordRunes, is indexed as any run (M8/P5A review A-I1).
		{thaiClause, "", trigramsOf(thaiClause), trigramsOf(thaiClause)},
		{"x", khmerClause, []string{"x"}, append([]string{"x"}, trigramsOf(khmerClause)...)},
	} {
		title, body := domain.SearchTokens(tt.name, &tt.content)
		if !slices.Equal(title, tt.title) || !slices.Equal(body, tt.body) {
			t.Errorf("SearchTokens(%q, %q) = %q, %q; want %q, %q", tt.name, tt.content, title, body, tt.title, tt.body)
		}
	}
}

// Numbers are words: their runs of three key by trigrams, shorter ones
// whole; a lone CJK character is no lexeme of the body.
func TestSearchTokensOfNumbers(t *testing.T) {
	_, body := domain.SearchTokens("x", ptr("2020年 v1.2 20"))
	if want := []string{"x", "202", "020", "v1", "2", "20"}; !slices.Equal(body, want) {
		t.Errorf("body = %q, want %q", body, want)
	}
}

// An attachment has a title alone.
func TestSearchTokensOfAnAttachment(t *testing.T) {
	title, body := domain.SearchTokens("报告.pdf", nil)
	if want := []string{"报", "告", "报告", "pdf"}; !slices.Equal(title, want) || body != nil {
		t.Errorf("SearchTokens(attachment) = %q, %q; want %q, nil", title, body, want)
	}
}

// A name and a page of no letter have no lexemes, which a row holds as
// empty tsvectors: the title's and a page's body are empty, not nil.
func TestSearchTokensOfNoLetter(t *testing.T) {
	title, body := domain.SearchTokens("#", ptr(""))
	if title == nil || len(title) != 0 || body == nil || len(body) != 0 {
		t.Errorf("SearchTokens(\"#\", \"\") = %#v, %#v; want empty and not nil", title, body)
	}
}

// A run of ASCII letters and digits past MaxSearchWordRunes, counted
// folded, and a data: string of more than 64 characters are left out;
// what surrounds them is not.
func TestSearchTokensLeaveOut(t *testing.T) {
	at := strings.Repeat("a", domain.MaxSearchWordRunes)
	past := strings.Repeat("b", domain.MaxSearchWordRunes+1)
	folded := strings.Repeat("ß", domain.MaxSearchWordRunes/2+1) // 66 letters folded
	image := "![x](data:image/png;base64," + strings.Repeat("QUJD", 20) + ")"
	short := "data:abc"
	edge, over := "data:"+strings.Repeat("k", 59), "data:"+strings.Repeat("m", 60) // 64 characters, 65
	_, body := domain.SearchTokens("x", ptr(strings.Join([]string{"aa", at, past, folded, image, short, edge, over, "zz"}, " ")))
	has := func(x string) bool { return slices.Contains(body, x) }
	switch {
	case !has("aaa"):
		t.Errorf("the run of %d letters is left out: %q", domain.MaxSearchWordRunes, body)
	case has("bbb"):
		t.Errorf("the run of %d letters is in: %q", domain.MaxSearchWordRunes+1, body)
	case has("sss"):
		t.Errorf("the run of 66 letters once folded is in: %q", body)
	case has("quj") || has("png") || has("ima") || has("mmm"):
		t.Errorf("a data: string of more than 64 characters is in: %q", body)
	case !has("kkk"):
		t.Errorf("the data: string of 64 characters is left out: %q", body)
	case !has("x") || !has("dat") || !has("abc") || !has("zz"):
		t.Errorf("what surrounds them, a short data: string, is left out: %q", body)
	}
}

// A data: string's left out run ends at the first character no data: URL
// holds: the text right after it is indexed, CJK right after an image's
// ")" (M8/P5A review A-M3), an HTML image's quote, inline code's backtick
// (fix check N-7), a non-ASCII character.
func TestSearchTokensLeaveADataURLOutToItsEnd(t *testing.T) {
	payload := "data:image/png;base64," + strings.Repeat("iVBORw0KGgo", 8)
	for _, tt := range []struct {
		content string
		after   []string
	}{
		{"点击![图](" + payload + ")按钮保存文档", []string{"按钮", "保存", "存文", "文档"}},
		{`<img src="` + payload + `">caption`, []string{"cap", "ion"}},
		{"<" + payload + ">tail", []string{"tai", "ail"}},
		{"[x](" + payload + "]note", []string{"not", "ote"}},
		{"'" + payload + "'quote", []string{"quo", "ote"}},
		{"`" + payload + "`word", []string{"wor", "ord"}},
		{payload + "数据库", []string{"数据", "据库"}},
		{payload + "\nnext", []string{"nex", "ext"}},
	} {
		_, body := domain.SearchTokens("x", &tt.content)
		for _, x := range tt.after {
			if !slices.Contains(body, x) {
				t.Errorf("SearchTokens(%q) = %q, want %q after the data: string", tt.content, body, x)
			}
		}
		for _, x := range []string{"ima", "png", "bas", "ivb", "vbo"} {
			if slices.Contains(body, x) {
				t.Errorf("SearchTokens(%q) = %q, holds %q of the data: string", tt.content, body, x)
			}
		}
	}
}

// Every member of every orbit of simple case folding keys as the orbit's
// others do, the title's lexemes of each alone, which the pattern's class
// of the orbit matches alike: Cherokee's small letters too, which x/text's
// fold makes capitals as it makes the capitals small letters (M8/P5A review
// A-M1).
func TestEachOrbitKeysAlike(t *testing.T) {
	orbits := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.SimpleFold(r) == r {
			continue
		}
		orbits++
		key, _ := domain.SearchTokens(string(r), nil)
		for o := unicode.SimpleFold(r); o != r; o = unicode.SimpleFold(o) {
			if other, _ := domain.SearchTokens(string(o), nil); !slices.Equal(other, key) {
				t.Errorf("%U %q keys %q, its orbit's %U %q keys %q", o, string(o), other, r, string(r), key)
			}
		}
	}
	if orbits < 2800 {
		t.Errorf("%d characters with case checked, want the some 2,900 of Unicode", orbits)
	}
	capitals := "ᏣᎳᎩ ᎦᏬᏂᎯᏍᏗ ᏰᏴ"
	small := strings.Map(unicode.ToLower, capitals)
	title, body := domain.SearchTokens(capitals, &small)
	if small == capitals || len(title) == 0 || !slices.Equal(body, title) {
		t.Errorf("Cherokee %q, its content in small letters %q: title %q, body %q; want the body the title's lexemes alone", capitals,
			small, title, body)
	}
}

// A row's lexemes stop at MaxSearchLexemeBytes, their bytes summed, in the
// order they first occur.
func TestSearchTokensStopAtTheBound(t *testing.T) {
	var b strings.Builder
	r := rand.New(rand.NewSource(1))
	for b.Len() < 4*domain.MaxSearchLexemeBytes/3 {
		b.WriteRune(rune(0x4E00 + r.Intn(0x5000)))
	}
	content := b.String()
	_, body := domain.SearchTokens("x", &content)
	sum := 0
	for _, x := range body {
		sum += len(x)
	}
	if sum > domain.MaxSearchLexemeBytes || sum < domain.MaxSearchLexemeBytes-6 {
		t.Fatalf("the lexemes hold %d bytes, want at most %d and within a bigram of it", sum, domain.MaxSearchLexemeBytes)
	}
	if first := string([]rune(content)[:2]); body[1] != first {
		t.Errorf("the first bigram is %q, want %q: the order of occurrence", body[1], first)
	}
}

// The lexemes stop at the first that does not fit: none after it, though
// shorter. The name's x and 166,666 bigrams of 6 bytes fill 999,997; the
// next bigram does not fit, nor then the q after it, which would.
func TestSearchTokensStopAtTheFirstPastTheBound(t *testing.T) {
	var b strings.Builder
	for k := range 166_670 {
		b.WriteRune(rune(0x4E00 + k/20_000))
		b.WriteRune(rune(0x4E00 + k%20_000))
		b.WriteByte(' ')
	}
	b.WriteString("q")
	content := b.String()
	_, body := domain.SearchTokens("x", &content)
	if len(body) != 1+166_666 || slices.Contains(body, "q") {
		t.Errorf("%d lexemes, the last %q; want the name's and 166,666 bigrams, none after", len(body), body[len(body)-1])
	}
}

// SearchTokenizer names the rules' version, Go's Unicode version and
// x/text's, whose fold and NFC are one Unicode version's.
func TestSearchTokenizer(t *testing.T) {
	if want := "2/" + unicode.Version + "/" + norm.Version; domain.SearchTokenizer != want {
		t.Errorf("SearchTokenizer = %q, want %s", domain.SearchTokenizer, want)
	}
	if cases.UnicodeVersion != norm.Version {
		t.Errorf("x/text's cases are of Unicode %s, its norm of %s: SearchTokenizer names one", cases.UnicodeVersion, norm.Version)
	}
}

// q's values (M8/P5 design 3.3): each refusal on q with its code.
func TestParseSearchQueryRefuses(t *testing.T) {
	letters := strings.Repeat("a", domain.MaxSearchWordRunes)
	for _, tt := range []struct {
		name string
		q    *string
		want string
	}{
		{"missing", nil, "required"},
		{"empty", ptr(""), "required"},
		{"blank, ideographic spaces too", ptr(" \t　 "), "required"},
		{"one byte past the bound", ptr(strings.Repeat("中", domain.MaxSearchQueryBytes/3) + "ab"), "too_long"},
		{"a term past the bound", ptr("a b c d e f g h i"), "too_long"},
		{"a run of letters past the bound", ptr(letters + "a"), "too_long"},
		{"a run past the bound once folded", ptr(strings.Repeat("ß", domain.MaxSearchWordRunes/2+1)), "too_long"},
		{"NUL", ptr("a\x00b"), "invalid_format"},
		{"not UTF-8", ptr("a\xffb"), "invalid_format"},
		{"no key at all", ptr("# ！ ++"), "not_allowed"},
	} {
		_, err := domain.ParseSearchQuery(tt.q)
		if err == nil {
			t.Errorf("%s: ParseSearchQuery() = nil, want validation_failed q %s", tt.name, tt.want)
			continue
		}
		if got := problemOf(err); got != "validation_failed q "+tt.want {
			t.Errorf("%s: ParseSearchQuery() = %v, want validation_failed q %s", tt.name, got, tt.want)
		}
	}
}

// What q may be at its bounds; its terms are parted by any white space
// and are one when they fold alike.
func TestParseSearchQueryTakes(t *testing.T) {
	for _, tt := range []struct {
		name  string
		q     string
		terms []string
	}{
		{"the bytes' bound, around blanks", "  " + strings.Repeat("中", domain.MaxSearchQueryBytes/3) + "a\n",
			[]string{strings.Repeat("中", domain.MaxSearchQueryBytes/3) + "a"}},
		{"the terms' bound", "a b c d e f g h", []string{"a", "b", "c", "d", "e", "f", "g", "h"}},
		{"terms that fold alike, more than the bound", "Go GO go gO a b c d e f g", []string{"Go", "a", "b", "c", "d", "e", "f", "g"}},
		{"the letters' bound", strings.Repeat("x", domain.MaxSearchWordRunes), []string{strings.Repeat("x", domain.MaxSearchWordRunes)}},
		{"an ideographic space", "中国　历史", []string{"中国", "历史"}},
		{"NFC", "é", []string{"é"}},
		{"a keyless term beside a keyed one", "锅 #", []string{"锅", "#"}},
		{"a run of Thai past 64 code points", thaiClause, []string{thaiClause}},
	} {
		q, err := domain.ParseSearchQuery(&tt.q)
		if err != nil {
			t.Errorf("%s: ParseSearchQuery(%q) = %v", tt.name, tt.q, err)
			continue
		}
		var terms []string
		for _, term := range q.Terms {
			terms = append(terms, term.Text)
		}
		if !slices.Equal(terms, tt.terms) {
			t.Errorf("%s: terms %q, want %q", tt.name, terms, tt.terms)
		}
	}
}

// Each kind of term's keys (M8/P5 design 3.3): a single CJK character is
// found in names alone, and is a key of the title alone in other terms;
// a term of such characters alone is found in names alone only in a query
// without keys of the body (M8/P5A review A-M2).
func TestSearchQueryKeys(t *testing.T) {
	str := func(s *string) string {
		if s == nil {
			return "-"
		}
		return *s
	}
	for _, tt := range []struct {
		q                   string
		title, body, titled string
		bodied              bool
	}{
		{"锅", "'锅'", "-", "'锅'", false},
		{"锅！", "'锅'", "-", "'锅'", false},
		{"锅#锅", "'锅'", "-", "'锅'", false},
		{"中·国", "'中' & '国'", "-", "'中' & '国'", false},
		{"锅 中·国", "'锅' & '中' & '国'", "-", "'锅' & '中' & '国'", false},
		{"中·国 #", "'中' & '国'", "-", "'中' & '国'", true},
		{"中·国 数据库", "'中' & '国' & '数据' & '据库'", "'数据' & '据库'", "-", true},
		{"锅#锅 数据库", "'锅' & '数据' & '据库'", "'数据' & '据库'", "-", true},
		{"タ・ネ api", "'タ' & 'ネ' & 'api'", "'api'", "-", true},
		{"锅 中·国 api", "'锅' & '中' & '国' & 'api'", "'api'", "'锅'", true},
		{"a·中", "'a' & '中'", "'a'", "-", true},
		{"数据库", "'数据' & '据库'", "'数据' & '据库'", "-", true},
		{"A股", "'a' & '股'", "'a'", "-", true},
		{"锅 #", "'锅'", "-", "'锅'", true},
		{"锅 中国", "'锅' & '中国'", "'中国'", "'锅'", true},
		{"Go语言", "'go' & '语言'", "'go' & '语言'", "-", true},
		{"C++", "'c'", "'c'", "-", true},
		{"node.js", "'nod' & 'ode' & 'js'", "'nod' & 'ode' & 'js'", "-", true},
		{"api", "'api'", "'api'", "-", true},
		{"20", "'20'", "'20'", "-", true},
		{"STRASSE straße", "'str' & 'tra' & 'ras' & 'ass' & 'sse'", "'str' & 'tra' & 'ras' & 'ass' & 'sse'", "-", true},
	} {
		q, err := domain.ParseSearchQuery(&tt.q)
		if err != nil {
			t.Errorf("ParseSearchQuery(%q) = %v", tt.q, err)
			continue
		}
		if got := [4]string{q.TitleKeys(), str(q.BodyKeys()), str(q.TitledKeys()), fmt.Sprint(q.Bodied())}; got != [4]string{tt.title, tt.body, tt.titled, fmt.Sprint(tt.bodied)} {
			t.Errorf("%q: title %s, body %s, titled %s, bodied %s; want %s, %s, %s, %v", tt.q, got[0], got[1], got[2], got[3],
				tt.title, tt.body, tt.titled, tt.bodied)
		}
	}
}

// Which terms are found in names alone, their patterns checked there, the
// others' in names or contents (M8/P5 design 3.3; M8/P5A review A-M2): one
// whose one keyed run is a single CJK character; in a query without keys of
// the body, one of single CJK characters alone too.
func TestSearchQueryTermsFoundInNamesAlone(t *testing.T) {
	for _, tt := range []struct {
		q              string
		titled, bodied int
	}{
		{"锅", 1, 0},
		{"锅！ 数据库", 1, 1},
		{"中·国", 1, 0},
		{"中·国 数据库", 0, 2},
		{"タ・ネ api", 0, 2},
		{"锅 中·国", 2, 0},
		{"锅 中·国 api", 1, 2},
		{"中·国 #", 1, 1},
		{"锅锅", 0, 1},
	} {
		q, err := domain.ParseSearchQuery(&tt.q)
		if err != nil {
			t.Errorf("ParseSearchQuery(%q) = %v", tt.q, err)
			continue
		}
		var titled int
		for _, term := range q.Terms {
			if term.TitlesOnly {
				titled++
			}
		}
		if got := [3]int{titled, len(q.TitledPatterns()), len(q.BodiedPatterns())}; got != [3]int{tt.titled, tt.titled, tt.bodied} {
			t.Errorf("%q: %d terms of names alone, %d patterns of names, %d of names or contents; want %d, %d, %d", tt.q,
				got[0], got[1], got[2], tt.titled, tt.titled, tt.bodied)
		}
	}
}

// A term's pattern (M8/P5 design 3.3): its text as a substring, with case,
// a word run of one or two letters or digits at an end whole.
func TestSearchQueryMatches(t *testing.T) {
	for _, tt := range []struct {
		q, text string
		want    bool
	}{
		{"数据库", "关系数据库的", true},
		{"api", "rapid", true},
		{"API", "an api", true},
		{"latency", "LATENCY.", true},
		{"ai", "AI", true},
		{"ai", "said", false},
		{"ai", "AI数据", true},
		{"ai", "aim", false},
		{"go", "Go语言", true},
		{"go", "gopher", false},
		{"Go语言", "学Go语言", true},
		{"Go语言", "DjanGo语言", false},
		{"C++", "用C++写", true},
		{"C++", "ObjC++", false},
		{"node.js", "xnode.js", true},
		{"node.js", "node.jsx", false},
		{"A股", "A股市场", true},
		{"A股", "BA股", false},
		{"20", "20 年", true},
		{"20", "2020年", false},
		{"c", "c", true},
		{"c", "abc", false},
		{"ΛΟΓΟΣ", "λογος", true},
		{"λογος", "ΛΟΓΟΣ", true},
		{"straße", "STRAẞE", true},
		{"strasse", "straße", false},
		{"KELVIN", "Kelvin", true},
		{"ǅx", "ǆX", true},
		{"ſ", "S", true},
		{"µ", "Μ", true},
		{"ς", "Σ", true},
		{"ﬃx", "ffix", false},
		{"İx", "ix", false},
		{"ı", "I", false},
		{"a%b", "a%b", true},
		{"a%b", "axb", false},
		{"a_b", "axb", false},
		{"a.b", "axb", false},
		{`a\b`, `a\b`, true},
		{"a[b", "a[b", true},
		{"(ab)", "x(ab)", true},
		{"锅", "一口锅", true},
	} {
		q, err := domain.ParseSearchQuery(&tt.q)
		if err != nil {
			t.Errorf("ParseSearchQuery(%q) = %v", tt.q, err)
			continue
		}
		if got := len(q.TitleHits(tt.text)) > 0; got != tt.want {
			t.Errorf("%q in %q = %v, want %v", tt.q, tt.text, got, tt.want)
		}
	}
}

// A character with case matches every one of its simple folding's orbit,
// each orbit of more than one member, both in NFC: a content's character
// that NFC changes (ά of oxia, U+1F71) matches its own spelling alone.
func TestSearchQueryMatchesEachOrbit(t *testing.T) {
	orbits := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.SimpleFold(r) == r || unicode.Is(unicode.Mn, r) {
			continue
		}
		q := "abc" + string(r)
		parsed, err := domain.ParseSearchQuery(&q)
		if err != nil {
			continue
		}
		orbits++
		for o := unicode.SimpleFold(r); ; o = unicode.SimpleFold(o) {
			if text := norm.NFC.String("ABC" + string(o)); len(parsed.TitleHits(text)) == 0 {
				t.Errorf("%q does not match %q, of the orbit of %U", q, text, r)
			}
			if o == r {
				break
			}
		}
	}
	if orbits < 2000 {
		t.Errorf("%d characters with case checked, want the some 2,800 of Unicode", orbits)
	}
}

// The search's property (M8/P5 design 3.3): a text a term matches holds
// the term's keys, of the body when it is a page's content, of the title
// when it is a name; but for what the lexemes leave out, which the texts
// here hold none of. The generator makes the characters whose folding
// differs from their simple folding's, and those of more than one fold;
// not a mark of a low combining class after İ (a virama), which NFC puts
// before the dot İ folds to: the text's fold then holds no lexeme of the
// term's, a case too rare to key.
func TestTheRecheckImpliesTheKeys(t *testing.T) {
	alphabet := []string{"a", "b", "c", "s", "k", "i", "A", "B", "S", "K", "I", "1", "2", "0", "中", "国", "数", "据", "库", "ー", "く",
		" ", "\n", "　", ".", "#", "+", "(", ")", "！", "ß", "ẞ", "İ", "ı", "ſ", "K", "µ", "μ", "ǅ", "Ǆ", "ǆ", "ς", "σ", "Σ", "ﬃ",
		"é", "É", "é", "ก", "ั", "न", "ि", "द", "ी", "😀", "-", "_", "%", "\\", "[", "Ꭰ", "ꭰ", "Ᏸ", "ᏸ"}
	text := func(r *rand.Rand, n int) string {
		var b strings.Builder
		for range n {
			b.WriteString(alphabet[r.Intn(len(alphabet))])
		}
		return norm.NFC.String(b.String())
	}
	r := rand.New(rand.NewSource(20261015))
	matched := 0
	for range 50_000 {
		s := text(r, 1+r.Intn(30))
		q := text(r, 1+r.Intn(6))
		if r.Intn(2) == 0 {
			runes := []rune(s)
			from := r.Intn(len(runes))
			q = string(runes[from : from+1+r.Intn(min(8, len(runes)-from))])
		}
		parsed, err := domain.ParseSearchQuery(&q)
		if err != nil {
			continue
		}
		title, _ := domain.SearchTokens(s, nil)
		_, body := domain.SearchTokens("-", &s)
		for _, term := range parsed.Terms {
			one, err := domain.ParseSearchQuery(&term.Text)
			if err != nil || len(one.TitleHits(s)) == 0 {
				continue
			}
			matched++
			if missing := missingKeys(one.TitleKeys(), title); missing != "" {
				t.Fatalf("%q matches the name %q, whose title %q lacks %s", term.Text, s, title, missing)
			}
			if keys := one.BodyKeys(); keys != nil {
				if missing := missingKeys(*keys, body); missing != "" {
					t.Fatalf("%q matches the content %q, whose body %q lacks %s", term.Text, s, body, missing)
				}
			}
		}
	}
	if matched < 10_000 {
		t.Fatalf("%d terms matched, want more for the property to say anything", matched)
	}
}

// The terms of a clause of Thai or Khmer, and the whole of it, past 64
// code points, find its text by their keys (M8/P5A review A-I1).
func TestTheKeysOfALongRunOfAnotherScript(t *testing.T) {
	for _, tt := range []struct{ text, q string }{
		{thaiClause, "ภาษาไทย"}, {thaiClause, "วรรณย\u0e38กต\u0e4c"}, {thaiClause, thaiClause},
		{khmerClause, "ភ\u17b6ស\u17b6ខ\u17d2ម\u17c2រ"}, {khmerClause, khmerClause},
	} {
		q, err := domain.ParseSearchQuery(&tt.q)
		if err != nil {
			t.Fatalf("ParseSearchQuery(%q) = %v", tt.q, err)
		}
		_, body := domain.SearchTokens("-", &tt.text)
		if len(q.TitleHits(tt.text)) == 0 || q.BodyKeys() == nil {
			t.Errorf("%q does not match %q, or has no keys", tt.q, tt.text)
		} else if missing := missingKeys(*q.BodyKeys(), body); missing != "" {
			t.Errorf("%q matches %q, whose body lacks %s", tt.q, tt.text, missing)
		}
	}
}

// The texts the lexemes leave out are the property's exceptions: a term
// matches them, its keys are not there.
func TestTheRecheckImpliesTheKeysButWhereLeftOut(t *testing.T) {
	for _, tt := range []struct{ name, q, text string }{
		{"a run past the bound", "abc", strings.Repeat("abc", 30)},
		{"a long data: string", "AAAA", "![](data:image/png;base64," + strings.Repeat("AAAA", 20) + ")"},
	} {
		q, err := domain.ParseSearchQuery(&tt.q)
		if err != nil {
			t.Fatal(err)
		}
		_, body := domain.SearchTokens("-", &tt.text)
		if len(q.TitleHits(tt.text)) == 0 || missingKeys(*q.BodyKeys(), body) == "" {
			t.Errorf("%s: %q matches %v and its keys are there: no exception", tt.name, tt.q, len(q.TitleHits(tt.text)) > 0)
		}
	}
}

// missingKeys are the operands of the tsquery keys, an AND, that lexemes
// lacks: "" when it holds them all.
func missingKeys(keys string, lexemes []string) string {
	var missing []string
	for _, op := range strings.Split(keys, " & ") {
		if !slices.Contains(lexemes, strings.Trim(op, "'")) {
			missing = append(missing, op)
		}
	}
	return strings.Join(missing, ", ")
}

// A name's hits are UTF-16 ranges, sorted, those that overlap joined.
func TestTitleHits(t *testing.T) {
	for _, tt := range []struct {
		q, name string
		want    []domain.TextRange
	}{
		{"go", "go go gogo Go语言", []domain.TextRange{{0, 2}, {3, 5}, {11, 13}}},
		{"中国", "😀中国😀中国", []domain.TextRange{{2, 4}, {6, 8}}},
		{"数据 据库", "数据库", []domain.TextRange{{0, 3}}},
		{"数据 库存", "数据库存", []domain.TextRange{{0, 2}, {2, 4}}},
		{"afé", "café", []domain.TextRange{{1, 4}}},
		{"ab", "ab abc", []domain.TextRange{{0, 2}}},
		{"x", "y", []domain.TextRange{}},
	} {
		q, err := domain.ParseSearchQuery(&tt.q)
		if err != nil {
			t.Fatal(err)
		}
		if got := q.TitleHits(tt.name); !slices.Equal(got, tt.want) {
			t.Errorf("%q in %q: %v, want %v", tt.q, tt.name, got, tt.want)
		}
	}
}

// An excerpt is the SQL's cut without its characters around, its white
// space collapsed; its hits see those characters, which tell a short
// word's ends.
func TestExcerpt(t *testing.T) {
	before := strings.Repeat("前", domain.ExcerptBefore)
	after := strings.Repeat("后", domain.ExcerptChars)
	for _, tt := range []struct {
		name       string
		q          string
		text       string // the SQL's cut
		at, chars  int
		excerpt    string
		hits       []domain.TextRange
		start, end bool
	}{
		{"at the content's start", "数据", "数据  \n 库" + after, 1, 300, "数据 库" + after[:3*153], []domain.TextRange{{0, 2}}, false, true},
		{"after the start, the character before cut", "数据", "x" + before + "数据" + after[:3*118], 42, 161, before + "数据" + after[:3*118],
			[]domain.TextRange{{40, 42}}, true, false},
		{"a short word whose letter before is cut", "数据 ai", "x" + "ai " + strings.Repeat("z", 37) + "数据" + after[:3*119], 42, 500,
			"ai " + strings.Repeat("z", 37) + "数据" + after[:3*118], []domain.TextRange{{40, 42}}, true, true},
		{"a short word after a cut space", "数据 ai", " " + "ai " + strings.Repeat("z", 37) + "数据" + after[:3*119], 42, 500,
			"ai " + strings.Repeat("z", 37) + "数据" + after[:3*118], []domain.TextRange{{0, 2}, {40, 42}}, true, true},
		{"a hit across the excerpt's end", "后后", "后" + strings.Repeat("x", 158) + "后后后", 1, 200, "后" + strings.Repeat("x", 158) + "后",
			[]domain.TextRange{{159, 160}}, false, true},
		{"surrogate pairs", "中国", "😀😀中国", 1, 4, "😀😀中国", []domain.TextRange{{4, 6}}, false, false},
		{"white space at the excerpt's start", "数据", "x  " + strings.Repeat("z", 38) + "数据" + after[:3*119], 42, 500,
			strings.Repeat("z", 38) + "数据" + after[:3*118], []domain.TextRange{{38, 40}}, true, true},
		{"the content's last character in", "数据", "数据" + strings.Repeat("x", 158), 1, 160, "数据" + strings.Repeat("x", 158),
			[]domain.TextRange{{0, 2}}, false, false},
		{"one character past the excerpt", "数据", "数据" + strings.Repeat("x", 159), 1, 161, "数据" + strings.Repeat("x", 158),
			[]domain.TextRange{{0, 2}}, false, true},
		// The content "zab yyy…": no word ab (M8/P5A review A-N2).
		{"a short word on the character before, cut", "数据 ab", "ab " + strings.Repeat("y", 38) + "数据" + after[:3*119], 43, 500,
			"b " + strings.Repeat("y", 38) + "数据" + after[:3*118], []domain.TextRange{{40, 42}}, true, true},
		// The content "… ab" then z: no word ab.
		{"a short word on the character after, cut", "数据 ab", "x" + strings.Repeat("y", 40) + "数据 " + strings.Repeat("y", 115) + " ab",
			42, 163, strings.Repeat("y", 40) + "数据 " + strings.Repeat("y", 115) + " a", []domain.TextRange{{40, 42}}, true, true},
		// The content ends with " ab": the word ab.
		{"a short word on the content's last character", "数据 ab", "x" + strings.Repeat("y", 40) + "数据 " + strings.Repeat("y", 115) + " ab",
			42, 162, strings.Repeat("y", 40) + "数据 " + strings.Repeat("y", 115) + " a", []domain.TextRange{{40, 42}, {159, 160}}, true, true},
		// A term of three letters needs no boundary: its hit on the character
		// before stays.
		{"a longer word on the character before", "数据 abc", "abc " + strings.Repeat("y", 37) + "数据" + after[:3*119], 43, 500,
			"bc " + strings.Repeat("y", 37) + "数据" + after[:3*118], []domain.TextRange{{0, 2}, {40, 42}}, true, true},
	} {
		q, err := domain.ParseSearchQuery(&tt.q)
		if err != nil {
			t.Fatal(err)
		}
		got := q.Excerpt(tt.text, tt.at, tt.chars)
		want := domain.SearchExcerpt{Text: tt.excerpt, Hits: tt.hits, CutStart: tt.start, CutEnd: tt.end}
		if got.Text != want.Text || !slices.Equal(got.Hits, want.Hits) || got.CutStart != want.CutStart || got.CutEnd != want.CutEnd {
			t.Errorf("%s: Excerpt() = %+v\nwant %+v", tt.name, got, want)
		}
		if n := utf8.RuneCountInString(got.Text); n > domain.ExcerptChars {
			t.Errorf("%s: the excerpt holds %d characters, past %d", tt.name, n, domain.ExcerptChars)
		}
	}
}

// The count by tiers (M8/P5 design 3.5); of the hits seen, at least, when
// too few passed of a walk that left candidates (M8/P5A review A-M5).
func TestEstimateCount(t *testing.T) {
	for _, tt := range []struct {
		name  string
		tally domain.SearchTally
		want  *domain.SearchCount
	}{
		{"names alone", domain.SearchTally{Titles: 340}, &domain.SearchCount{Value: 340, Kind: domain.CountExact}},
		{"names past the bound", domain.SearchTally{Titles: 1001, Bodied: true}, &domain.SearchCount{Value: 1000, Kind: domain.CountAtLeast}},
		{"names alone past the bound", domain.SearchTally{Titles: 1001}, &domain.SearchCount{Value: 1000, Kind: domain.CountAtLeast}},
		{"tier by tier", domain.SearchTally{Titles: 25, TitlePages: 20, Bodies: 10_020, Checked: 1000, Passed: 30, Found: 21, Bodied: true},
			&domain.SearchCount{Value: 330, Kind: domain.CountAbout}},
		{"two significant digits", domain.SearchTally{Titles: 3, Bodies: 1234, Checked: 100, Passed: 50, Found: 21, Bodied: true},
			&domain.SearchCount{Value: 620, Kind: domain.CountAbout}},
		{"no less than found", domain.SearchTally{Titles: 2, Bodies: 30, Checked: 30, Passed: 19, Found: 21, Bodied: true},
			&domain.SearchCount{Value: 21, Kind: domain.CountAbout}},
		{"the candidates counted to the bound", domain.SearchTally{Bodies: 20, BodiesCapped: true, Checked: 10, Passed: 3, Found: 3,
			Bodied: true}, &domain.SearchCount{Value: 6, Kind: domain.CountAtLeast}},
		{"the candidates counted below the bound", domain.SearchTally{Bodies: domain.MaxSearchCountScan, Checked: 1000, Passed: 3, Found: 3,
			Bodied: true}, &domain.SearchCount{Value: 60, Kind: domain.CountAbout}},
		{"none passed of a walk that left some", domain.SearchTally{Titles: 2, TitlePages: 2, Bodies: 5000, Checked: 500, Found: 2,
			Bodied: true}, &domain.SearchCount{Value: 2, Kind: domain.CountAtLeast}},
		{"one passed of a walk that left some", domain.SearchTally{Titles: 2, TitlePages: 2, Bodies: 5000, Checked: 500, Passed: 1, Found: 3,
			Bodied: true}, &domain.SearchCount{Value: 3, Kind: domain.CountAtLeast}},
		{"one passed of the sample alone", domain.SearchTally{Bodies: 5000, Checked: 100, Passed: 1, Bodied: true},
			&domain.SearchCount{Value: 1, Kind: domain.CountAtLeast}},
		{"the hits seen past the count's bound", domain.SearchTally{Titles: 999, TitlePages: 999, Bodies: 5000, Checked: 500, Passed: 1,
			Found: 21, Bodied: true}, &domain.SearchCount{Value: 1000, Kind: domain.CountAtLeast}},
		{"two passed of a walk that left some", domain.SearchTally{Titles: 2, TitlePages: 2, Bodies: 5000, Checked: 500, Passed: 2, Found: 4,
			Bodied: true}, &domain.SearchCount{Value: 22, Kind: domain.CountAbout}},
		// The rate of a walk the full page stopped leaves its last pass out
		// (M8/P5A fix check I-1): 2 of the 100 before the third pass, not 3
		// of 101, which would give 77.
		{"a walk the full page stopped", domain.SearchTally{Titles: 18, TitlePages: 18, Bodies: 2000, Checked: 101, Passed: 3, Full: true,
			Found: 21, Bodied: true}, &domain.SearchCount{Value: 58, Kind: domain.CountAbout}},
		// Stopped at its second pass, the full page's walk's rate rests on
		// one: at least the hits seen (M8/P5A fix check F-1), not about 59.
		{"a walk the full page stopped at its second pass", domain.SearchTally{Titles: 19, TitlePages: 19, Bodies: 2000, Checked: 51, Passed: 2,
			Full: true, Found: 21, Bodied: true}, &domain.SearchCount{Value: 21, Kind: domain.CountAtLeast}},
		// A walk of one candidate, the last, that filled the page: its own
		// pass rated, 1 of 1, not 0 of 0 (M8/P5A fix check N2-3).
		{"a walk of one candidate that filled the page", domain.SearchTally{Titles: 20, TitlePages: 20, Bodies: 21, Checked: 1, Passed: 1,
			Ended: true, Full: true, Found: 21, Bodied: true}, &domain.SearchCount{Value: 21, Kind: domain.CountAbout}},
		// A walk its bound stopped rates every pass: 10 of 500, not 9 of
		// 499, which would give 46.
		{"a walk its bound stopped", domain.SearchTally{Titles: 10, TitlePages: 10, Bodies: 2000, Checked: 500, Passed: 10, Found: 20,
			Bodied: true}, &domain.SearchCount{Value: 50, Kind: domain.CountAbout}},
		{"one passed of a walk that checked them all", domain.SearchTally{Titles: 2, TitlePages: 2, Bodies: 40, Checked: 38, Passed: 1,
			Ended: true, Found: 3, Bodied: true}, &domain.SearchCount{Value: 3, Kind: domain.CountAbout}},
		{"past the count's bound", domain.SearchTally{Titles: 10, Bodies: 5000, Checked: 100, Passed: 21, Found: 21, Bodied: true},
			&domain.SearchCount{Value: 1000, Kind: domain.CountAtLeast}},
		{"the sample of the body", domain.SearchTally{Titles: 21, TitlePages: 21, Bodies: 821, Checked: 100, Passed: 10, Found: 21, Bodied: true},
			&domain.SearchCount{Value: 100, Kind: domain.CountAbout}},
		{"none passed yet", domain.SearchTally{Bodies: 5000, Checked: 1000, Bodied: true}, nil},
		{"the title tier's pages not counted twice", domain.SearchTally{Titles: 40, TitlePages: 40, Bodies: 90, Checked: 50, Passed: 50,
			Found: 21, Bodied: true}, &domain.SearchCount{Value: 90, Kind: domain.CountAbout}},
		{"the count's bound, at least", domain.SearchTally{Bodies: 2000, Checked: 100, Passed: 50, Found: 21, Bodied: true},
			&domain.SearchCount{Value: 1000, Kind: domain.CountAtLeast}},
	} {
		got := domain.EstimateCount(tt.tally)
		if fmt.Sprint(got) != fmt.Sprint(tt.want) && (got == nil || tt.want == nil || *got != *tt.want) {
			t.Errorf("%s: EstimateCount() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// A cursor's position is in UTC, in a tier; the first page's is above
// every node.
func TestSearchCursor(t *testing.T) {
	from := domain.SearchFrom(domain.SearchTierBody)
	if from.Tier != domain.SearchTierBody || from.At.Year() != 9999 || !from.Valid() {
		t.Errorf("SearchFrom(body) = %+v", from)
	}
	if (domain.SearchCursor{Tier: 2}).Valid() {
		t.Error("a cursor of tier 2 is valid")
	}
	at := time.Date(2026, 10, 15, 8, 0, 0, 5, time.FixedZone("CST", 8*3600))
	b, err := json.Marshal(domain.SearchCursor{Tier: domain.SearchTierBody, At: at, ID: uuid.Nil()})
	if want := `{"tier":1,"at":"2026-10-15T00:00:00.000000005Z","id":"00000000-0000-0000-0000-000000000000"}`; err != nil || string(b) != want {
		t.Errorf("a cursor marshals to %s, %v; want %s, in UTC", b, err, want)
	}
}

// limit is 1–MaxSearchLimit, DefaultSearchLimit unset.
func TestSearchLimit(t *testing.T) {
	for _, tt := range []struct {
		limit *int
		want  string
	}{
		{nil, fmt.Sprint(domain.DefaultSearchLimit)},
		{ptr(1), "1"},
		{ptr(domain.MaxSearchLimit), fmt.Sprint(domain.MaxSearchLimit)},
		{ptr(0), "validation_failed limit out_of_range"},
		{ptr(domain.MaxSearchLimit + 1), "validation_failed limit out_of_range"},
	} {
		n, err := domain.SearchLimit(tt.limit)
		got := fmt.Sprint(n)
		if err != nil {
			got = problemOf(err)
		}
		if got != tt.want {
			t.Errorf("SearchLimit(%v) = %s, want %s", tt.limit, got, tt.want)
		}
	}
}
