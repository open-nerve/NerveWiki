package domain_test

import (
	"regexp"
	"regexp/syntax"
	"slices"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// TestGrepKeys checks GrepKeys (M9/P4 design 3.5) on 3.5.3's examples, a
// row for each kind of node and each trim, and a row for each fix of
// 3.5.4, its counterexample, which the fix's mutant fails (6.3, 137-142):
// the keys, and, where the row has a line, that the pattern matches it in
// NFC and every key is among the lexemes of a page holding it.
func TestGrepKeys(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, pattern, line string
		want                []string
	}{
		// 3.5.3's examples.
		{"a literal", `foo`, "a foo b", []string{"foo"}},
		{"without case", `(?i)FOO`, "x Foo y", []string{"foo"}},
		{"classes between", `func\s+\w+\(`, "func main() {", []string{"fun", "unc"}},
		{"branches with nothing in common", `TODO|FIXME`, "TODO: x", nil},
		{"CJK", `数据库设计`, "关于数据库设计的", []string{"数据", "据库", "库设", "设计"}},
		{"a short word between \\b", `\bai\b`, "the ai way", nil},
		{"a short word between others", ` ai `, "the ai way", []string{"ai"}},
		{"a short word between a line's ends", `^ai$`, "ai", []string{"ai"}},
		{"a heading", `^# Title$`, "# Title", []string{"tit", "itl", "tle"}},
		{"ß folds", `Straße`, "Straße", []string{"str", "tra", "ras", "ass", "sse"}},
		{"ß's fold", `(?i)STRASSE`, "Strasse", []string{"str", "tra", "ras", "ass", "sse"}},
		{"a repeat", `a{3}`, "baaab", []string{"aaa"}},
		{"a plus then a literal", `(abc)+def`, "abcabcdef", []string{"abc", "bcd", "cde", "def"}},
		{"a branch's prefix", `(?:foo|foobar)baz`, "foobarbaz", []string{"foo", "baz"}},
		{"an extension", `\.md$`, "notes.md", []string{"md"}},
		{"capitals", `README`, "README.md", []string{"rea", "ead", "adm", "dme"}},
		// Each kind of node.
		{"a class of others", `foo[ -]bar`, "foo bar", []string{"foo", "bar"}},
		{"a class of others between short words", `x[ -]y[ -]z`, "x-y z", []string{"y"}},
		{"a class not of others", `foo[a-z]bar`, "fooxbar", []string{"foo", "bar"}},
		{"line anchors", `(?m)^ab$`, "ab", []string{"ab"}},
		{"text anchors", `\Aab\z`, "ab", []string{"ab"}},
		{"\\B takes nothing", `x\Byz`, "xyz", []string{"xyz"}},
		{"an empty group", `ab(?:)c`, "abc", []string{"abc"}},
		{"a capture", `(abc)`, "abc", []string{"abc"}},
		{"branches of one text", `(?:abc|abc)d`, "abcd", []string{"abc", "bcd"}},
		{"branches' common suffix", `(?:1+xyz|2+xyz)`, "11xyz", []string{"xyz"}},
		{"a plus", `(?:abc)+`, "abcabc", []string{"abc"}},
		{"a repeat of an exact text", `(?:ab){2}`, "abab", []string{"aba", "bab"}},
		{"a repeat past 1,024 atoms", `(?:abc-){300}`, repeat("abc-", 300), []string{"abc"}},
		{"a repeat from 0", `abc(?:def){0,2}ghi`, "abcghi", []string{"abc", "ghi"}},
		{"a star", `abc(?:def)*ghi`, "abcdefghi", []string{"abc", "ghi"}},
		{"a quest", `abc(?:def)?ghi`, "abcghi", []string{"abc", "ghi"}},
		{"any character", `(?s)abc.ghi`, "abc-ghi", []string{"abc", "ghi"}},
		{"any character but a newline", `abc.ghi`, "abcxghi", []string{"abc", "ghi"}},
		{"no match", `abc[^\x00-\x{10FFFF}]`, "", []string{"abc"}},
		{"a run past 64 ASCII", `[a-z]0123456789012345678901234567890123456789012345678901234567890123456789`, "", []string{
			"012", "123", "234", "345", "456", "567", "678", "789", "890", "901"}},
		// Each trim.
		{"a leading mark", "\u0301abc", "x\u0301abc", []string{"abc"}},
		{"a leading iota", `(?i)ιabc`, "ιabc", []string{"abc"}},
		{"H and U+0331", `abcH`, "abcH\u0331", []string{"abc"}},
		{"H before an edge", `abcH$`, "abcH", []string{"abc", "bch"}},
		{"a fold of a mark", `abcİ`, "abcİ", []string{"abc"}},
		// Each fix of 3.5.4, its counterexample (137-141).
		{"#137 the tail's sequence", "ab\u017f\u0301", "ab\u017f\u0301\u0307", nil},
		{"#138 the tail's orbit", `(?i)abſ`, "ab\u017f\u0301", nil},
		{"#139 the atoms after the tail", "(?i)ab\u0389\u0345", "ab\u0389\u0399\u0342", nil},
		{"#140 folded atom by atom", "(?i)\u1fbc\u0342\u1e22\uff21\uac00\ufb06",
			"\u1fbc\u0342\u1e22\uff21\uac00\ufb05\u030a", []string{"\u03b1\u1fd6\u1e23", "\u1fd6\u1e23\uff41"}},
		{"#141 \\b is no edge", `文(e)\b`, "B文eς", nil},
		// MaxGrepKeys, in the order of the segments.
		{"more than 32 keys", `abcdefghijklmnopqrstuvwxyz0123456789`, "", []string{"abc", "bcd", "cde", "def", "efg", "fgh", "ghi",
			"hij", "ijk", "jkl", "klm", "lmn", "mno", "nop", "opq", "pqr", "qrs", "rst", "stu", "tuv", "uvw", "vwx", "wxy",
			"xyz", "yz0", "z01", "012", "123", "234", "345", "456", "567"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			pattern := norm.NFC.String(row.pattern)
			re, err := syntax.Parse(pattern, syntax.Perl)
			if err != nil {
				t.Fatal(err)
			}
			got := domain.GrepKeys(re)
			if !slices.Equal(got, row.want) {
				t.Errorf("GrepKeys(%q) = %q, want %q", pattern, got, row.want)
			}
			if row.line == "" {
				return
			}
			if !regexp.MustCompile(pattern).MatchString(norm.NFC.String(row.line)) {
				t.Fatalf("%q does not match %q", pattern, row.line)
			}
			content := "x\n" + row.line + "\ny"
			_, body, partial := domain.SearchTokens("n", &content)
			if partial {
				t.Fatalf("the page of %q is partial", row.line)
			}
			for _, k := range got {
				if !slices.Contains(body, k) {
					t.Errorf("GrepKeys(%q)'s %q is not among the lexemes of a page holding %q: %q", pattern, k, row.line, body)
				}
			}
		})
	}
}

func repeat(s string, n int) string {
	out := ""
	for range n {
		out += s
	}
	return out
}
