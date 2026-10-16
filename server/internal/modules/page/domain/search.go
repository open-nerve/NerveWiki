package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Full-text search (M8 design 4.12; M8/P5 design 3.3): a node's lexemes,
// which the search's job stores in page_search, and a query's keys, which
// find its candidates there, and patterns, which check them as they are.
// Words are compared folded, as titles are (shared.TitleKey); a CJK run of
// characters keys by its bigrams, the title also by its characters; a run
// of letters, digits and marks by its trigrams, a short one of one or two
// whole. The keys of a query are among the lexemes of every text its
// patterns match, but where the lexemes leave part of a text out (see
// SearchTokens).

// The search's bounds (M8/P5 design 3.3–3.5, 3.9).
const (
	// MaxSearchQueryBytes is how long a query is at most, in bytes, once
	// its surrounding white space goes; MaxSearchTerms how many terms it
	// holds at most, once those of one key go.
	MaxSearchQueryBytes = 256
	MaxSearchTerms      = 8
	// DefaultSearchLimit and MaxSearchLimit are a page of hits'.
	DefaultSearchLimit = 20
	MaxSearchLimit     = 50
	// MaxSearchWalk is how many pages' contents a request checks at most,
	// beyond the names: measured (M8/P5 design 3.9, the largest step whose
	// walk to its end keeps a page within 300 ms on 300,000 pages; at 1,000
	// "A股" took 364).
	MaxSearchWalk = 500
	// MaxSearchCount is the count's largest value: one past it is "at
	// least"; MaxSearchCountScan how many candidates of the body the count
	// counts at most; SearchCountSample how many it checks for its rate
	// when the first page checked none (M8/P5 design 3.5);
	// SearchCountPasses how many of those it checked must pass for a rate,
	// when it did not check them all (M8/P5A review A-M5).
	MaxSearchCount     = 1000
	MaxSearchCountScan = 20_000
	SearchCountSample  = 100
	SearchCountPasses  = 2
	// MaxSearchLexemeBytes bounds a row's lexemes, their bytes summed: a
	// tsvector holds at most 1,048,575 and refuses more.
	MaxSearchLexemeBytes = 1_000_000
	// MaxSearchWordRunes is the longest run of ASCII letters and digits
	// indexed, folded: a longer one, base64, hex and the like, is not, nor
	// is a query's. A longer run of another script's letters and marks,
	// Thai's, Lao's, Khmer's or Burmese's, which part no words by spaces, is
	// indexed as any other (M8/P5A review A-I1).
	MaxSearchWordRunes = 64
	// MinSearchRebuild is how soon after the job built a node's row it
	// builds it again: a page being edited is saved every few seconds.
	MinSearchRebuild = 30 * time.Second
	// The job's batch (M8/P5 design 3.4): SearchBatchNodes queued nodes,
	// the contents it reads of them SearchBatchBytes at most, an import's
	// batch's; it analyzes page_search each SearchAnalyzeRows rows it
	// writes, and starts no batch after SearchRunBudget.
	SearchBatchNodes  = 100
	SearchBatchBytes  = 8 << 20
	SearchAnalyzeRows = 10_000
	SearchRunBudget   = time.Minute
)

// searchTokenizerVersion is the version of the rules below: a change of
// them is a new one, whose rows nervewiki reindex queues. 2: the cut of
// long runs to ASCII's, the end of a data: string, Cherokee's case (M8/P5A
// review A-I1, A-M3, A-M1).
const searchTokenizerVersion = "2"

// SearchTokenizer names the lexemes a row holds: the rules' version, Go's
// Unicode version, whose tables classify the characters, and x/text's,
// whose fold and NFC key them (shared.TitleKey): an upgrade of either may
// change a row's lexemes.
const SearchTokenizer = searchTokenizerVersion + "/" + unicode.Version + "/" + norm.Version

// searchClasses are the characters' classes (M8/P5 design 3.3) as tables,
// from Go's: CJK, with ー, ｰ and 〆, of no script but inside Japanese
// words; the letters, digits and marks of the other scripts, words; the
// rest part them. A rune's class here, and the boundaries of the patterns.
//
//nolint:gochecknoglobals // made once, read only
var searchClasses = sync.OnceValues(func() (cjk, word *unicode.RangeTable) {
	cjkRanges := union(rangesOf(unicode.Han), rangesOf(unicode.Hiragana), rangesOf(unicode.Katakana),
		rangesOf(unicode.Hangul), rangesOf(unicode.Bopomofo), []runeRange{{'〆', '〆'}, {'ー', 'ー'}, {'ｰ', 'ｰ'}})
	wordRanges := minus(union(rangesOf(unicode.L), rangesOf(unicode.N), rangesOf(unicode.Mn), rangesOf(unicode.Mc),
		rangesOf(unicode.Me)), cjkRanges)
	return tableOf(cjkRanges), tableOf(wordRanges)
})

// runeRange is the code points lo to hi.
type runeRange struct{ lo, hi rune }

// rangesOf is t's code points as ranges, in order.
func rangesOf(t *unicode.RangeTable) []runeRange {
	var out []runeRange
	add := func(lo, hi, stride rune) {
		for r := lo; r <= hi; r += stride {
			end := r
			if stride == 1 {
				end = hi
			}
			out = append(out, runeRange{r, end})
			if stride == 1 {
				break
			}
		}
	}
	for _, g := range t.R16 {
		add(rune(g.Lo), rune(g.Hi), rune(g.Stride))
	}
	for _, g := range t.R32 {
		add(rune(g.Lo), rune(g.Hi), rune(g.Stride))
	}
	return out
}

// union is the code points of any of sets, as ranges in order, adjacent
// ones joined.
func union(sets ...[]runeRange) []runeRange {
	var all []runeRange
	for _, s := range sets {
		all = append(all, s...)
	}
	slices.SortFunc(all, func(a, b runeRange) int { return int(a.lo - b.lo) })
	var out []runeRange
	for _, g := range all {
		if n := len(out); n > 0 && g.lo <= out[n-1].hi+1 {
			out[n-1].hi = max(out[n-1].hi, g.hi)
			continue
		}
		out = append(out, g)
	}
	return out
}

// minus is the code points of a not in b, both as union returns them.
func minus(a, b []runeRange) []runeRange {
	var out []runeRange
	for _, g := range a {
		lo := g.lo
		for _, h := range b {
			if h.hi < lo || h.lo > g.hi {
				continue
			}
			if h.lo > lo {
				out = append(out, runeRange{lo, h.lo - 1})
			}
			lo = h.hi + 1
		}
		if lo <= g.hi {
			out = append(out, runeRange{lo, g.hi})
		}
	}
	return out
}

// tableOf is ranges as a table unicode.Is reads.
func tableOf(ranges []runeRange) *unicode.RangeTable {
	t := &unicode.RangeTable{}
	for _, g := range ranges {
		switch {
		case g.hi <= 0xFFFF:
			t.R16 = append(t.R16, unicode.Range16{Lo: uint16(g.lo), Hi: uint16(g.hi), Stride: 1})
			if g.hi <= unicode.MaxLatin1 {
				t.LatinOffset++
			}
		case g.lo > 0xFFFF:
			t.R32 = append(t.R32, unicode.Range32{Lo: uint32(g.lo), Hi: uint32(g.hi), Stride: 1})
		default:
			t.R16 = append(t.R16, unicode.Range16{Lo: uint16(g.lo), Hi: 0xFFFF, Stride: 1})
			t.R32 = append(t.R32, unicode.Range32{Lo: 0x10000, Hi: uint32(g.hi), Stride: 1})
		}
	}
	return t
}

// searchClass is a rune's class.
type searchClass int

const (
	classOther searchClass = iota
	classCJK
	classWord
)

func classOf(r rune) searchClass {
	if r < utf8.RuneSelf {
		if 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' {
			return classWord
		}
		return classOther
	}
	cjk, word := searchClasses()
	switch {
	case unicode.Is(cjk, r):
		return classCJK
	case unicode.Is(word, r):
		return classWord
	}
	return classOther
}

// searchFold is how lexemes and keys compare: shared.TitleKey, Cherokee's
// small letters then its capitals. x/text's full folding toggles Cherokee's
// case, capitals to small letters and small letters to capitals, where
// Unicode's folds both to the capitals: a query in one case would key the
// other's lexemes, found while its page waits in the queue and lost once
// the job built its row (M8/P5A review A-M1). The pattern's orbit class
// matches either case; the key is now one.
func searchFold(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case 0xAB70 <= r && r <= 0xABBF: // ꭰ–ꮿ: Ꭰ–Ꮿ
			return r - 0xAB70 + 0x13A0
		case 0x13F8 <= r && r <= 0x13FD: // ᏸ–ᏽ: Ᏸ–Ᏽ
			return r - 0x13F8 + 0x13F0
		}
		return r
	}, shared.TitleKey(s))
}

// SearchTokens are a node's lexemes (M8/P5 design 3.3), in the order they
// first occur: its name's, the title's, each CJK character and bigram; and
// a page's name and content's, the body's, nil for an attachment (content
// nil) alone, each CJK bigram. Both hold each run of letters' trigrams, or the
// run when it is of one or two. A run of ASCII letters and digits longer
// than MaxSearchWordRunes, a data: string longer than 64 characters (an
// image in the Markdown), up to the first character no data: URL holds,
// and what follows MaxSearchLexemeBytes are left out: a query finds them
// only while the node waits in the queue.
func SearchTokens(name string, content *string) (title, body []string) {
	t := lexemes{out: []string{}}
	t.scan(searchFold(name), true)
	if content == nil {
		return t.out, nil
	}
	b := lexemes{out: []string{}}
	b.scan(searchFold(name+"\n"+*content), false)
	return t.out, b.out
}

// lexemes collects a row's lexemes, each once, up to MaxSearchLexemeBytes.
type lexemes struct {
	seen  map[string]struct{}
	out   []string
	bytes int
	full  bool
}

func (l *lexemes) add(s string) {
	if l.full {
		return
	}
	if _, ok := l.seen[s]; ok {
		return
	}
	if l.bytes+len(s) > MaxSearchLexemeBytes {
		l.full = true
		return
	}
	if l.seen == nil {
		l.seen = map[string]struct{}{}
	}
	l.seen[s] = struct{}{}
	l.out = append(l.out, s)
	l.bytes += len(s)
}

// scan adds the lexemes of s, folded: its CJK runs' bigrams, and their
// characters when unigrams is set; its word runs'.
func (l *lexemes) scan(s string, unigrams bool) {
	prev := -1 // the offset of the CJK run's last character, -1 outside one
	word := -1 // the offset of the word run's first character, -1 outside one
	words := 0 // its characters
	for i := 0; i < len(s) && !l.full; {
		r, size := utf8.DecodeRuneInString(s[i:])
		if word < 0 && r == 'd' && strings.HasPrefix(s[i:], "data:") {
			// A data: URL as a word starts: one longer than 64 characters
			// is left out up to its end, the first character no data: URL
			// holds, so that the text right after it, a CJK paragraph's
			// after an image's ")", is not (M8/P5A review A-M3).
			end := strings.IndexFunc(s[i:], func(r rune) bool { return !inDataURL(r) })
			if end < 0 {
				end = len(s) - i
			}
			if end > 64 { // bytes: the URL's characters are ASCII
				prev = -1
				i += end
				continue
			}
		}
		switch classOf(r) {
		case classCJK:
			l.word(s, word, i, words)
			word, words = -1, 0
			if unigrams {
				l.add(s[i : i+size])
			}
			if prev >= 0 {
				l.add(s[prev : i+size])
			}
			prev = i
		case classWord:
			prev = -1
			if word < 0 {
				word = i
			}
			words++
		default:
			prev = -1
			l.word(s, word, i, words)
			word, words = -1, 0
		}
		i += size
	}
	l.word(s, word, len(s), words)
}

// inDataURL reports whether a data: URL holds r: ASCII but white space and
// what ends it in Markdown and HTML, ")", quotes, "<", ">", "]" and the
// backtick that ends inline code (M8/P5A fix check N-7).
func inDataURL(r rune) bool {
	return r < utf8.RuneSelf && !unicode.IsSpace(r) && !strings.ContainsRune(")\"'<>]`", r)
}

// longASCII reports whether the word run of n characters in b bytes is
// one MaxSearchWordRunes leaves out: longer, and of ASCII letters and
// digits alone, a byte each.
func longASCII(n, b int) bool {
	return n > MaxSearchWordRunes && b == n
}

// word adds the lexemes of the word run s[start:end] of n characters: none
// for no run or one of ASCII past MaxSearchWordRunes; itself for one of one
// or two; its trigrams otherwise.
func (l *lexemes) word(s string, start, end, n int) {
	switch {
	case start < 0 || longASCII(n, end-start):
		return
	case n <= 2:
		l.add(s[start:end])
		return
	}
	var at [3]int // the offsets of the last three characters
	k := 0
	for i, r := range s[start:end] {
		at[k%3] = start + i
		k++
		if k >= 3 {
			l.add(s[at[k%3] : start+i+utf8.RuneLen(r)])
		}
	}
}

// SearchQuery is a parsed query (M8/P5 design 3.3): its terms, each found
// in a node's name or a page's content, all of them in each hit.
type SearchQuery struct {
	Terms []SearchTerm
}

// SearchTerm is a query's term: its text in NFC; its keys, the lexemes of
// the title and the body every text it matches holds, and its single CJK
// characters, keys of the title alone; and whether it is found in names
// alone: one whose one keyed run is a single CJK character ("锅", "锅！"),
// and, in a query no term of which has a key of the body, one of single
// CJK characters alone ("中·国"), whose keys the names alone hold (M8/P5
// design 3.3; M8/P5A review A-M2).
//
// Text is Go's NFC, x/text's, as a name is (shared.Title) and as the keys
// are. That is the Stream-Safe Text Format: a CGJ (U+034F) after 30
// non-starters in a row, which PostgreSQL's normalize, a content's NFC as
// the search reads it, does not insert, and the run's marks are ordered
// on each side of it alone. A term whose NFC so differs from what it was
// written is matched in contents by PostgreSQL's NFC of it, which the use
// case reads (Written, InContents); in names by Text (M8 Codex review R2).
type SearchTerm struct {
	Text       string
	TitlesOnly bool
	keys       []string
	unigrams   []string
	// left and right: the term's ends are word runs that must start and end
	// there in the text.
	left, right bool
	re          *regexp.Regexp
	// written is the term as the query wrote it when Text holds a CGJ it
	// did not, "" otherwise; content is the text it is matched with in
	// contents, Text until InContents gives PostgreSQL's NFC of written,
	// and contentRE its pattern for Go.
	written   string
	content   string
	contentRE *regexp.Regexp
}

// cgj is U+034F COMBINING GRAPHEME JOINER, which Go's NFC inserts after 30
// non-starters in a row.
const cgj = "\u034f"

// ParseSearchQuery parses a search's q (M8/P5 design 3.3): 422 on q when
// it is missing or blank (required), not UTF-8 or holding NUL
// (invalid_format), longer than MaxSearchQueryBytes, of more terms than
// MaxSearchTerms or of a run of ASCII letters and digits longer than
// MaxSearchWordRunes, which no lexeme holds (too_long), or of no key at
// all: punctuation alone (not_allowed). Its terms are parted by white
// space, U+3000 too, and those of one folded form are one.
func ParseSearchQuery(q *string) (SearchQuery, error) {
	invalid := func(code, message string) (SearchQuery, error) {
		return SearchQuery{}, shared.Invalid(shared.FieldError{Field: "q", Code: code, Message: message})
	}
	if q == nil {
		return invalid(shared.FieldRequired, "is required")
	}
	if !utf8.ValidString(*q) || strings.IndexByte(*q, 0) >= 0 {
		return invalid(shared.FieldInvalidFormat, "must be UTF-8 text without NUL characters")
	}
	s := strings.TrimFunc(*q, unicode.IsSpace)
	switch {
	case s == "":
		return invalid(shared.FieldRequired, "is required")
	case len(s) > MaxSearchQueryBytes:
		return invalid(shared.FieldTooLong, fmt.Sprintf("must be at most %d bytes", MaxSearchQueryBytes))
	}
	var out SearchQuery
	seen := map[string]bool{}
	keyed := false
	for _, f := range strings.FieldsFunc(s, unicode.IsSpace) {
		text := norm.NFC.String(f)
		folded := searchFold(text)
		// A term whose NFC holds a CGJ it did not is one of its own: matched
		// in contents by PostgreSQL's NFC of it, which another term of the
		// same folded NFC, written with the CGJ, is not.
		streamSafe := strings.Count(text, cgj) > strings.Count(f, cgj)
		seenAs := folded
		if streamSafe {
			seenAs += "\x00" + f
		}
		if seen[seenAs] {
			continue
		}
		seen[seenAs] = true
		if len(out.Terms) == MaxSearchTerms {
			return invalid(shared.FieldTooLong, fmt.Sprintf("must hold at most %d terms", MaxSearchTerms))
		}
		t, ok := parseTerm(text, folded)
		if !ok {
			return invalid(shared.FieldTooLong, fmt.Sprintf("must hold at most %d ASCII letters and digits in a row", MaxSearchWordRunes))
		}
		if streamSafe {
			t.written = f
		}
		keyed = keyed || len(t.keys) > 0 || len(t.unigrams) > 0
		out.Terms = append(out.Terms, t)
	}
	if !keyed {
		return invalid(shared.FieldNotAllowed, "must hold a letter, a digit, a mark or a CJK character")
	}
	if !slices.ContainsFunc(out.Terms, func(t SearchTerm) bool { return len(t.keys) > 0 }) {
		// No key of the body: the single CJK characters are the body tier's
		// keys, of the names.
		for i := range out.Terms {
			out.Terms[i].TitlesOnly = len(out.Terms[i].unigrams) > 0
		}
	}
	return out, nil
}

// parseTerm is the term text, folded: false when a word run of it is of
// ASCII letters and digits longer than MaxSearchWordRunes.
func parseTerm(text, folded string) (SearchTerm, bool) {
	t := SearchTerm{Text: text}
	type run struct {
		class      searchClass
		start, end int
		n          int
	}
	var runs []run
	for i, r := range folded {
		c := classOf(r)
		if n := len(runs); n > 0 && runs[n-1].class == c && c != classOther {
			runs[n-1].end = i + utf8.RuneLen(r)
			runs[n-1].n++
			continue
		}
		runs = append(runs, run{class: c, start: i, end: i + utf8.RuneLen(r), n: 1})
	}
	keys := lexemes{}
	unigrams := lexemes{}
	keyed := 0 // the runs of keys
	for _, g := range runs {
		if g.class != classOther {
			keyed++
		}
		switch g.class {
		case classCJK:
			if g.n == 1 {
				unigrams.add(tsqueryOperand(folded[g.start:g.end]))
				continue
			}
			bigrams := lexemes{}
			bigrams.scan(folded[g.start:g.end], false)
			for _, x := range bigrams.out {
				keys.add(tsqueryOperand(x))
			}
		case classWord:
			if longASCII(g.n, g.end-g.start) {
				return SearchTerm{}, false
			}
			w := lexemes{}
			w.word(folded, g.start, g.end, g.n)
			for _, x := range w.out {
				keys.add(tsqueryOperand(x))
			}
		}
	}
	t.unigrams = unigrams.out
	t.keys = keys.out
	t.TitlesOnly = keyed == 1 && len(t.keys) == 0 && len(t.unigrams) == 1
	if n := len(runs); n > 0 {
		first, last := runs[0], runs[n-1]
		t.left = first.class == classWord && first.n <= 2
		t.right = last.class == classWord && last.n <= 2
	}
	t.re = regexp.MustCompile(t.pattern(syntaxRE2, text))
	t.content, t.contentRE = text, t.re
	return t, true
}

// Written are the terms, as the query wrote them, whose NFC by Go holds a
// CGJ they did not (SearchTerm), in their order: none for most queries.
// The use case reads PostgreSQL's NFC of them, in one statement, for
// InContents.
func (q SearchQuery) Written() []string {
	var out []string
	for _, t := range q.Terms {
		if t.written != "" {
			out = append(out, t.written)
		}
	}
	return out
}

// InContents is q whose terms of Written are matched in contents by nfc,
// PostgreSQL's NFC of each, in Written's order: their patterns of contents
// and their hits in excerpts. Their names' patterns stay Text's.
func (q SearchQuery) InContents(nfc []string) (SearchQuery, error) {
	out := SearchQuery{Terms: slices.Clone(q.Terms)}
	k := 0
	for i, t := range out.Terms {
		if t.written == "" {
			continue
		}
		if k == len(nfc) {
			break
		}
		t.content = nfc[k]
		t.contentRE = regexp.MustCompile(t.pattern(syntaxRE2, t.content))
		out.Terms[i] = t
		k++
	}
	if k != len(nfc) || k != len(q.Written()) {
		return SearchQuery{}, fmt.Errorf("search: %d texts in NFC for %d terms written", len(nfc), len(q.Written()))
	}
	return out, nil
}

// tsqueryOperand is the lexeme x as a tsquery's operand: quoted, though it
// holds letters, digits and marks alone.
func tsqueryOperand(x string) string {
	return "'" + x + "'"
}

// TitleKeys is the title's tsquery: each term's keys and characters.
func (q SearchQuery) TitleKeys() string {
	var ops []string
	for _, t := range q.Terms {
		ops = append(ops, t.keys...)
		ops = append(ops, t.unigrams...)
	}
	return andOf(ops)
}

// Bodied reports whether the query has a body tier: a term found in
// contents too.
func (q SearchQuery) Bodied() bool {
	return slices.ContainsFunc(q.Terms, func(t SearchTerm) bool { return !t.TitlesOnly })
}

// BodyKeys is the body tier's tsquery of the body, the keys of its terms
// found in contents too; nil when they have none.
func (q SearchQuery) BodyKeys() *string {
	var ops []string
	for _, t := range q.Terms {
		if !t.TitlesOnly {
			ops = append(ops, t.keys...)
		}
	}
	return orNil(andOf(ops))
}

// TitledKeys is the body tier's tsquery of the title, the characters of
// its terms found in names alone; nil when it has none.
func (q SearchQuery) TitledKeys() *string {
	var ops []string
	for _, t := range q.Terms {
		if t.TitlesOnly {
			ops = append(ops, t.unigrams...)
		}
	}
	return orNil(andOf(ops))
}

// Patterns are the terms' patterns of names, PostgreSQL's (see
// SearchTerm.Pattern); TitledPatterns those of the terms found in names
// alone, BodiedPatterns the others'. ContentPatterns are the terms'
// patterns of contents (SearchTerm.ContentPattern), BodiedContentPatterns
// those of BodiedPatterns' terms, in their order.
func (q SearchQuery) Patterns() []string {
	return q.patterns(func(SearchTerm) bool { return true }, SearchTerm.Pattern)
}

func (q SearchQuery) TitledPatterns() []string {
	return q.patterns(func(t SearchTerm) bool { return t.TitlesOnly }, SearchTerm.Pattern)
}

func (q SearchQuery) BodiedPatterns() []string {
	return q.patterns(func(t SearchTerm) bool { return !t.TitlesOnly }, SearchTerm.Pattern)
}

func (q SearchQuery) ContentPatterns() []string {
	return q.patterns(func(SearchTerm) bool { return true }, SearchTerm.ContentPattern)
}

func (q SearchQuery) BodiedContentPatterns() []string {
	return q.patterns(func(t SearchTerm) bool { return !t.TitlesOnly }, SearchTerm.ContentPattern)
}

func (q SearchQuery) patterns(keep func(SearchTerm) bool, pattern func(SearchTerm) string) []string {
	out := []string{}
	for _, t := range q.Terms {
		if keep(t) {
			out = append(out, pattern(t))
		}
	}
	return out
}

// andOf is the tsquery of ops all, each once.
func andOf(ops []string) string {
	seen := map[string]bool{}
	var out []string
	for _, o := range ops {
		if !seen[o] {
			seen[o] = true
			out = append(out, o)
		}
	}
	return strings.Join(out, " & ")
}

func orNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Pattern is the term's pattern for PostgreSQL's ~ and regexp_instr, an
// ARE matched with case (M8/P5 design 3.3): the term's text, each
// character with case as the class of its simple folding's orbit, the
// rest escaped, in one group; a boundary before it when it starts with a
// word run that must start there, after it when it ends with one that must
// end there. A boundary is the text's start or end, or a character of no
// word. Go's patterns, which give the hits, are built alike. It matches
// names; ContentPattern contents.
func (t SearchTerm) Pattern() string {
	return t.pattern(syntaxARE, t.Text)
}

// ContentPattern is the term's pattern of contents: Pattern's of the text
// it is matched with in contents, Text's unless InContents gave another.
func (t SearchTerm) ContentPattern() string {
	return t.pattern(syntaxARE, t.content)
}

// patternSyntax is a regular expressions' syntax.
type patternSyntax int

const (
	// syntaxARE is PostgreSQL's advanced regular expressions.
	syntaxARE patternSyntax = iota
	// syntaxRE2 is Go's, run on the text between two NULs, which no name
	// or content holds, rather than with ^ and $: a search from a hit's end
	// then sees the character before it.
	syntaxRE2
)

// pattern is the term's pattern for syntax of text, Text or content.
func (t SearchTerm) pattern(syntax patternSyntax, text string) string {
	var b strings.Builder
	if t.left {
		if syntax == syntaxARE {
			b.WriteString("(?:^|" + boundaryClass(syntax) + ")")
		} else {
			b.WriteString(boundaryClass(syntax))
		}
	}
	b.WriteByte('(')
	for _, r := range text {
		orbit := foldOrbit(r)
		if len(orbit) == 1 {
			writeEscaped(&b, syntax, r)
			continue
		}
		b.WriteByte('[')
		for _, o := range orbit {
			writeEscaped(&b, syntax, o)
		}
		b.WriteByte(']')
	}
	b.WriteByte(')')
	if t.right {
		if syntax == syntaxARE {
			b.WriteString("(?:$|" + boundaryClass(syntax) + ")")
		} else {
			b.WriteString(boundaryClass(syntax))
		}
	}
	return b.String()
}

// foldOrbit is r's orbit under simple case folding, from r: the characters
// a pattern matches for it.
func foldOrbit(r rune) []rune {
	out := []rune{r}
	for o := unicode.SimpleFold(r); o != r; o = unicode.SimpleFold(o) {
		out = append(out, o)
	}
	return out
}

// writeEscaped writes r for syntax: an ASCII letter or digit as itself,
// any other character by its code point.
func writeEscaped(b *strings.Builder, syntax patternSyntax, r rune) {
	switch {
	case r < utf8.RuneSelf && ('a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9'):
		b.WriteRune(r)
	case syntax == syntaxRE2:
		fmt.Fprintf(b, `\x{%X}`, r)
	case r <= 0xFFFF:
		fmt.Fprintf(b, `\u%04X`, r)
	default:
		fmt.Fprintf(b, `\U%08X`, r)
	}
}

// boundaryClass is the class of the characters of no word, for syntax.
func boundaryClass(syntax patternSyntax) string {
	return boundaryClasses()[syntax]
}

// boundaryClasses are boundaryClass's, by syntax.
//
//nolint:gochecknoglobals // made once, read only
var boundaryClasses = sync.OnceValue(func() [2]string {
	_, word := searchClasses()
	var out [2]string
	for _, syntax := range []patternSyntax{syntaxARE, syntaxRE2} {
		var b strings.Builder
		b.WriteString("[^")
		write := func(lo, hi rune) {
			writeEscaped(&b, syntax, lo)
			if hi > lo {
				b.WriteByte('-')
				writeEscaped(&b, syntax, hi)
			}
		}
		for _, g := range word.R16 {
			write(rune(g.Lo), rune(g.Hi))
		}
		for _, g := range word.R32 {
			write(rune(g.Lo), rune(g.Hi))
		}
		b.WriteByte(']')
		out[syntax] = b.String()
	}
	return out
})

// SearchLimit is the page size limit asks for, DefaultSearchLimit when it
// is nil: 422 on limit outside 1–MaxSearchLimit.
func SearchLimit(limit *int) (int, error) {
	switch {
	case limit == nil:
		return DefaultSearchLimit, nil
	case *limit < 1 || *limit > MaxSearchLimit:
		return 0, shared.Invalid(shared.FieldError{Field: "limit", Code: shared.FieldOutOfRange,
			Message: fmt.Sprintf("must be between 1 and %d", MaxSearchLimit)})
	}
	return *limit, nil
}
