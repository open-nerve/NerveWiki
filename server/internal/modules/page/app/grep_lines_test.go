package app_test

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// grepLinesSeed seeds TestGrepLinesAreReads; its expectations hold on any
// seed (1 to 10 checked when it was written).
const grepLinesSeed = 1

// grepPieces are the lines TestGrepLinesAreReads builds contents of: empty
// ones, matches, a line not in NFC (e and U+0301, which é in NFC
// matches), a lone CR inside, and long ones, the match after a part not in
// NFC, which a hit cuts to 1,000 bytes about it.
func grepPieces(rng *rand.Rand) string {
	decomposed := "e\u0301"
	switch rng.IntN(9) {
	case 0:
		return ""
	case 1:
		return "a needle here"
	case 2:
		return "x" + decomposed + "t" + decomposed + " needle"
	case 3:
		return "one\rneedle"
	case 4:
		return "\u00e9t\u00e9"
	case 5:
		return strings.Repeat(decomposed, 400+rng.IntN(400)) + " needle " + strings.Repeat("z", rng.IntN(1500))
	case 6:
		return strings.Repeat("y", 1200+rng.IntN(800)) + "needle"
	case 7:
		return "needle at the end y"
	}
	return "nothing"
}

// grepContent is a random content: lines of grepPieces ended by "\n" or
// "\r\n", maybe a BOM before, maybe no last newline; sometimes empty.
func grepContent(rng *rand.Rand) string {
	if rng.IntN(20) == 0 {
		return ""
	}
	var b strings.Builder
	if rng.IntN(4) == 0 {
		b.WriteString("\ufeff")
	}
	n := 1 + rng.IntN(8)
	for i := range n {
		b.WriteString(grepPieces(rng))
		if i < n-1 || rng.IntN(2) == 0 {
			b.WriteString([]string{"\n", "\r\n"}[rng.IntN(2)])
		}
	}
	return b.String()
}

// oracleLines are content's lines split here, apart from GrepLines: by
// "\n", none after a last "\n", each without one trailing "\r", the first
// without its BOM.
func oracleLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if strings.HasSuffix(content, "\n") {
		lines = lines[:len(lines)-1]
	}
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	lines[0] = strings.TrimPrefix(lines[0], "\ufeff")
	return lines
}

// TestGrepLinesAreReads is grep's numbering against read's (M9/P4 design
// 3.7.4, review C-M9): over random contents (CRLF, a lone CR, a BOM, empty
// lines, no last newline, an empty content, long lines, lines not in NFC),
// the lines grep finds are, as a set, those of an oracle that splits the
// lines apart and matches their NFC; each hit's line, read by ReadPath at
// its number without its "\r" and BOM, is the hit's text, and a cut hit's
// text is where ReadPath reads from its column. Patterns ^$ and ^ among
// them. Its floor: a hit cut on a long line, after a part not in NFC, one
// in a hundred hits at least.
func TestGrepLinesAreReads(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	reads := newFakeGrep(f.store)
	rng := rand.New(rand.NewPCG(grepLinesSeed, 9))
	contents := map[string]string{}
	for i := range 300 {
		name := fmt.Sprintf("p%03d", i)
		content := grepContent(rng)
		f.content(f.page(name, nil, float64(i)).ID, content, 1)
		contents[name+".md"] = content
	}
	reads.buildAll()
	g := f.grepper(reads, nil)
	read := app.NewReadPath(f.notebooks, f.store, f.store, f.auth)
	limit := domain.MaxGrepLimit
	cutAfterNFC, hits := 0, 0
	for _, pattern := range []string{`needle`, `^$`, `^`, `(?i)NEEDLE`, "\u00e9t", `y$`, `\rn`} {
		var got []string
		cursor := (*string)(nil)
		for range 100 {
			page, err := g.Execute(f.asAlice(), f.eng, app.GrepRequest{Pattern: pattern, Limit: &limit, Cursor: cursor})
			if err != nil {
				t.Fatal(err)
			}
			for _, found := range page.Found {
				if found.Hit == nil {
					t.Errorf("%q: %s:%d matched in part", pattern, found.Path, found.Line)
					continue
				}
				hits++
				got = append(got, fmt.Sprintf("%s:%d", found.Path, found.Line))
				line := oracleLines(contents[found.Path])[found.Line-1]
				r, err := read.Execute(f.asAlice(), f.eng, found.Path, &found.Line, nil, ptr(1), textMeasure(1<<30))
				if err != nil {
					t.Fatal(err)
				}
				whole := strings.TrimSuffix(strings.TrimSuffix(r.Page.Slice.Text, "\n"), "\r")
				if found.Line == 1 {
					whole = strings.TrimPrefix(whole, "\ufeff")
				}
				if whole != line {
					t.Errorf("%q: %s:%d reads %q, the oracle's line %q", pattern, found.Path, found.Line, shortText(whole), shortText(line))
				}
				if !found.Hit.Cut {
					if found.Hit.Text != line || found.Hit.Column != columnOf(found.Line, contents[found.Path]) {
						t.Errorf("%q: %s:%d's hit %q at %d, the line %q", pattern, found.Path, found.Line, shortText(found.Hit.Text),
							found.Hit.Column, shortText(line))
					}
					continue
				}
				from, err := read.Execute(f.asAlice(), f.eng, found.Path, &found.Line, &found.Hit.Column, ptr(1), textMeasure(1<<30))
				if err != nil {
					t.Fatalf("%s:%d at %d: %v", found.Path, found.Line, found.Hit.Column, err)
				}
				if !strings.HasPrefix(from.Page.Slice.Text, found.Hit.Text) || len(found.Hit.Text) > domain.MaxGrepLineText {
					t.Errorf("%q: %s:%d's cut hit %q, read from %d %q", pattern, found.Path, found.Line, shortText(found.Hit.Text),
						found.Hit.Column, shortText(from.Page.Slice.Text))
				}
				if !norm.NFC.IsNormalString(line[:found.Hit.Column-columnOf(found.Line, contents[found.Path])]) {
					cutAfterNFC++
				}
			}
			if page.Next == "" {
				break
			}
			cursor = &page.Next
		}
		re := regexp.MustCompile(pattern)
		var want []string
		for _, path := range slices.Sorted(maps.Keys(contents)) {
			for i, line := range oracleLines(contents[path]) {
				if re.MatchString(norm.NFC.String(line)) {
					want = append(want, fmt.Sprintf("%s:%d", path, i+1))
				}
			}
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%q: grep found %d lines, the oracle %d: only grep's %q, only the oracle's %q", pattern, len(got), len(want),
				minus(got, want), minus(want, got))
		}
	}
	t.Logf("%d hits, %d of them cut after a part not in NFC", hits, cutAfterNFC)
	if cutAfterNFC*100 < hits {
		t.Errorf("%d hits cut after a part not in NFC of %d, want 1%% at least", cutAfterNFC, hits)
	}
}

// columnOf is the column a hit's text starts at on a line of no cut: 3 on
// line 1 after a BOM, else 0.
func columnOf(line int, content string) int {
	if line == 1 && strings.HasPrefix(content, "\ufeff") {
		return 3
	}
	return 0
}

// minus is a's lines not in b, ten at most.
func minus(a, b []string) []string {
	var out []string
	for _, s := range a {
		if !slices.Contains(b, s) && len(out) < 10 {
			out = append(out, s)
		}
	}
	return out
}

func shortText(s string) string {
	if len(s) > 40 {
		return s[:20] + "…" + s[len(s)-20:]
	}
	return s
}
