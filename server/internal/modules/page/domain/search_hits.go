package domain

import (
	"encoding/json"
	"math"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
	"uuid"
)

// A search's answer (M8/P5 design 3.5): where its terms hit a name and an
// excerpt of a page's content, the cursor of its pages, and the first
// page's count.

// TextRange is a hit in a text: from Start up to End, in UTF-16 code
// units, which a browser's strings count in.
type TextRange struct {
	Start, End int
}

// SearchExcerpt is a page's content around its first hit: the text, its
// white space collapsed, the hits in it, and whether it starts after the
// content's start and ends before its end.
type SearchExcerpt struct {
	Text     string
	Hits     []TextRange
	CutStart bool
	CutEnd   bool
}

// ExcerptBefore is how many characters an excerpt shows before the first
// hit, ExcerptChars how many it shows at most. The SQL cuts one more on
// each side, which tells the hits at the excerpt's ends whether a word
// goes on there (M8/P5 design 3.5).
const (
	ExcerptBefore = 40
	ExcerptChars  = 160
)

// hits are the term's hits in text, the byte ranges of its group: each
// search from the last hit's end on, which leaves the character after a
// hit to the next one's boundary.
func (t SearchTerm) hits(text string) [][2]int {
	padded := "\x00" + text + "\x00"
	var out [][2]int
	for pos := 1; pos < len(padded)-1; {
		from := pos
		if t.left {
			_, size := utf8.DecodeLastRuneInString(padded[:pos])
			from -= size
		}
		loc := t.re.FindStringSubmatchIndex(padded[from:])
		if loc == nil {
			break
		}
		out = append(out, [2]int{from + loc[2] - 1, from + loc[3] - 1})
		pos = from + loc[3]
	}
	return out
}

// queryHits are the query's terms' hits in text, sorted, those that
// overlap joined.
func (q SearchQuery) queryHits(text string) [][2]int {
	var hits [][2]int
	for _, t := range q.Terms {
		hits = append(hits, t.hits(text)...)
	}
	slices.SortFunc(hits, func(a, b [2]int) int { return a[0] - b[0] })
	var out [][2]int
	for _, h := range hits {
		if n := len(out); n > 0 && h[0] < out[n-1][1] {
			out[n-1][1] = max(out[n-1][1], h[1])
			continue
		}
		out = append(out, h)
	}
	return out
}

// TitleHits are the query's hits in a name.
func (q SearchQuery) TitleHits(name string) []TextRange {
	units := make(map[int]int, len(name)+1) // name's byte offsets to UTF-16 units
	n := 0
	for i, r := range name {
		units[i] = n
		n += utf16.RuneLen(r)
	}
	units[len(name)] = n
	return rangesAt(q.queryHits(name), units)
}

// rangesAt are hits, byte ranges, as the ranges of units, which maps their
// offsets: those left empty go.
func rangesAt(hits [][2]int, units map[int]int) []TextRange {
	out := []TextRange{}
	for _, h := range hits {
		if r := (TextRange{Start: units[h[0]], End: units[h[1]]}); r.End > r.Start {
			out = append(out, r)
		}
	}
	return out
}

// Excerpt is the excerpt of a page's content of chars characters, whose
// first hit is its character at (1 the first), from text, the SQL's cut:
// ExcerptBefore characters before at and one more when the content has
// them, then ExcerptChars and two more. The hits are found in all of
// text, then the characters around the excerpt go, and its white space
// collapses to one space, none at its ends.
func (q SearchQuery) Excerpt(text string, at, chars int) SearchExcerpt {
	first := max(at-ExcerptBefore, 1) // the content's character the excerpt starts at
	skip := 0                         // the characters of text before it
	if at-ExcerptBefore-1 >= 1 {
		skip = 1
	}
	var b strings.Builder
	units := make(map[int]int, len(text)+1) // text's byte offsets to the excerpt's UTF-16 units
	n, k, space := 0, 0, false
	for i, r := range text {
		units[i] = n
		switch {
		case k < skip || k >= skip+ExcerptChars:
		case unicode.IsSpace(r):
			space = true
		default:
			if space && n > 0 {
				b.WriteByte(' ')
				n++
				units[i] = n
			}
			space = false
			b.WriteRune(r)
			n += utf16.RuneLen(r)
		}
		k++
	}
	units[len(text)] = n
	return SearchExcerpt{
		Text: b.String(), Hits: rangesAt(q.queryHits(text), units),
		CutStart: first > 1, CutEnd: first+ExcerptChars-1 < chars,
	}
}

// The search's tiers (M8/P5 design 3.5): the nodes whose names hold every
// term, then the pages whose names hold some term alone.
const (
	SearchTierTitle = 0
	SearchTierBody  = 1
)

// SearchCursor is a search's page's cursor: the next page goes on in Tier
// with the nodes changed before At, or at At with an id below ID.
type SearchCursor struct {
	Tier int       `json:"tier"`
	At   time.Time `json:"at"`
	ID   uuid.UUID `json:"id"`
}

// MarshalJSON writes the cursor with At in UTC, so that each position has
// one spelling: shared.DecodeCursor refuses another.
func (c SearchCursor) MarshalJSON() ([]byte, error) {
	type plain SearchCursor
	return json.Marshal(plain{Tier: c.Tier, At: c.At.UTC(), ID: c.ID})
}

// Valid reports whether the cursor is in a tier.
func (c SearchCursor) Valid() bool {
	return c.Tier == SearchTierTitle || c.Tier == SearchTierBody
}

// SearchFrom is the cursor above every node of tier: the first page's
// with SearchTierTitle, and the body tier's first once the title tier
// ends.
func SearchFrom(tier int) SearchCursor {
	from := ChangesetFrom()
	return SearchCursor{Tier: tier, At: from.At, ID: from.ID}
}

// SearchCountKind is what a search's count is: the number of hits, about
// it, or as many or more.
type SearchCountKind string

// The kinds of count.
const (
	CountExact   SearchCountKind = "exact"
	CountAbout   SearchCountKind = "about"
	CountAtLeast SearchCountKind = "at_least"
)

// SearchCount is a search's count, at most MaxSearchCount.
type SearchCount struct {
	Value int
	Kind  SearchCountKind
}

// SearchTally is what the first page tells of a search with more pages
// (M8/P5 design 3.5): the title tier's hits, counted to MaxSearchCount +
// 1, and its pages among them; the body tier's candidates, counted to the
// search's bound, which BodiesCapped says they reached, and which hold
// those pages too; how many of its pages the walk, or the sample, checked
// from its top, how many passed, and whether they were all it holds
// (Ended); and the hits the page found.
type SearchTally struct {
	Titles, TitlePages int
	Bodies             int
	BodiesCapped       bool
	Checked, Passed    int
	Ended              bool
	Found              int
	// Bodied tells there is a body tier: a term found in contents too.
	Bodied bool
}

// EstimateCount is the count of a search with more pages than its first,
// tier by tier: the title tier's hits, and the body tier's candidates,
// the title tier's pages taken off, at the rate its walk passed; nil when
// none passed yet. A rate of fewer than SearchCountPasses passes, of a
// walk that left candidates unchecked, tells little: one pass gives a
// count within 2× about one time in two, its 95% interval two orders of
// magnitude wide, two passes about two times in three (M8/P5A review
// A-M5); the count is then of the hits seen, at least. A count past
// MaxSearchCount is MaxSearchCount at least, one of all the body's
// candidates counted about, to two significant digits; one of them
// counted to the bound at least, rounded down.
func EstimateCount(t SearchTally) *SearchCount {
	switch {
	case t.Titles > MaxSearchCount:
		return &SearchCount{Value: MaxSearchCount, Kind: CountAtLeast}
	case !t.Bodied:
		return &SearchCount{Value: t.Titles, Kind: CountExact}
	case t.Found == 0 && t.Titles == 0 && t.Passed == 0:
		return nil
	}
	n := float64(t.Titles)
	kind, round := CountAbout, math.Round
	switch {
	case !t.Ended && t.Passed < SearchCountPasses:
		n += float64(t.Passed)
		kind, round = CountAtLeast, math.Floor
	case t.Checked > 0:
		n += float64(max(t.Bodies-t.TitlePages, 0)) * float64(t.Passed) / float64(t.Checked)
	}
	if t.BodiesCapped {
		kind, round = CountAtLeast, math.Floor
	}
	n = max(n, float64(t.Found))
	v := significant(n, round)
	if v >= MaxSearchCount {
		return &SearchCount{Value: MaxSearchCount, Kind: CountAtLeast}
	}
	return &SearchCount{Value: v, Kind: kind}
}

// significant is n to two significant digits by round.
func significant(n float64, round func(float64) float64) int {
	if n < 100 {
		return int(round(n))
	}
	unit := math.Pow(10, math.Floor(math.Log10(n))-1)
	return int(round(n/unit) * unit)
}
