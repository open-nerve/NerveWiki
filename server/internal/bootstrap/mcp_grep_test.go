package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math/rand/v2"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// mcpGrepHit is a hit of grep's answer, as these tests read it.
type mcpGrepHit struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Text   string `json:"text"`
	Column *int   `json:"column"`
	Cut    bool   `json:"cut"`
}

// mcpGrepLine is a partial line of grep's answer.
type mcpGrepLine struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

// mcpGrepAnswer is grep's answer.
type mcpGrepAnswer struct {
	Hits         []mcpGrepHit  `json:"hits"`
	PartialLines []mcpGrepLine `json:"partial_lines"`
	NextCursor   *string       `json:"next_cursor"`
	Building     bool          `json:"building"`
	Incomplete   bool          `json:"incomplete"`
}

// at is the hit as "path:line".
func (h mcpGrepHit) at() string { return fmt.Sprintf("%s:%d", h.Path, h.Line) }

// productionDeadlines gives an app production's deadlines
// (configs/config.yaml: request_timeout 15s, write_timeout 60s) in place of
// the tests' 4 s and 10 s. grep starts no more lines once its call is
// within GrepSoftMargin (3 s) of its deadline: at the tests' 4 s that is a
// second into the call, which a slow runner under -race can pass before
// the call's first hit (bootstrap-3 of CI run 39709536904). Such a stop
// answers incomplete with a cursor, rightly, but the exact answers of the
// tests that count grep's pages and stops do not allow for it: at 15 s
// they count the stops they inject, and those alone.
func productionDeadlines(c *config.Config) {
	c.Server.WriteTimeout, c.Server.RequestTimeout = 60*time.Second, 15*time.Second
}

// startGrepApp is startReadsAppWith of w at production's deadlines.
func startGrepApp(t *testing.T, w wiring) readsApp {
	t.Helper()
	return startReadsAppOf(t, publicOrigin, w, productionDeadlines)
}

// grep is m's grep of args in its notebook, which must answer.
func (m *mcpApp) grep(t *testing.T, args map[string]any) mcpGrepAnswer {
	t.Helper()
	return answerOf[mcpGrepAnswer](t, m.use(t, "grep", args))
}

// grepThrough is m's grep of args, page after page while next_cursor is
// set: each page's answer, 200 at most.
func (m *mcpApp) grepThrough(t *testing.T, args map[string]any) []mcpGrepAnswer {
	t.Helper()
	var out []mcpGrepAnswer
	args = maps.Clone(args)
	for range 200 {
		a := m.grep(t, args)
		out = append(out, a)
		if a.NextCursor == nil {
			return out
		}
		args["cursor"] = *a.NextCursor
	}
	t.Fatalf("grep %v went on past 200 pages", args)
	return nil
}

// hitsOf are the pages' hits as "path:line", in their order.
func hitsOf(pages []mcpGrepAnswer) []string {
	var out []string
	for _, p := range pages {
		for _, h := range p.Hits {
			out = append(out, h.at())
		}
	}
	return out
}

// mcpWrite writes content at path in m's notebook through MCP's write and
// answers the page's id.
func (m *mcpApp) mcpWrite(t *testing.T, path, content string) string {
	t.Helper()
	return answerOf[mcpWritten](t, m.use(t, "write", map[string]any{"path": path, "content": content})).NodeID
}

// grepWalked is a "grep walked" line of the log, as these tests read it.
type grepWalked struct {
	Msg        string `json:"msg"`
	Keys       int    `json:"keys"`
	Candidates int    `json:"candidates"`
	Pages      int    `json:"pages"`
	Work       int64  `json:"work"`
	Lines      int    `json:"lines"`
	Stop       string `json:"stop"`
}

// grepWalkLines are the log's "grep walked" lines, in turn.
func grepWalkLines(t *testing.T, logs *syncBuffer) []grepWalked {
	t.Helper()
	var out []grepWalked
	for line := range strings.SplitSeq(logs.String(), "\n") {
		var l grepWalked
		if json.Unmarshal([]byte(line), &l) == nil && l.Msg == "grep walked" {
			out = append(out, l)
		}
	}
	return out
}

// grepWalks are the stops of the log's "grep walked" lines, in turn.
func grepWalks(t *testing.T, logs *syncBuffer) []string {
	t.Helper()
	var out []string
	for _, l := range grepWalkLines(t, logs) {
		out = append(out, l.Stop)
	}
	return out
}

// TestTheMCPGrepFindsWhatItsKeysFind: grep of each kind of pattern, a
// literal, (?i), CJK, a whole word (" go ", whose key is the word, review
// C-m1) and one of no keys, finds the pages MCP wrote once the job has
// built their rows (M9/P4 design 6.1), each call's "grep walked" line
// telling keys for the keyed patterns and none, nor candidates read, for
// the others, so that a pattern that loses its keys shows; a page
// queued exact, the job not run, is a candidate and found; a page a batch
// wrote, queued in bulk, is not found by a keyed grep, the answer building,
// until the job has built it; a grep of no keys walks it at once, not
// building.
func TestTheMCPGrepFindsWhatItsKeysFind(t *testing.T) {
	r := startGrepApp(t, wiring{})
	for path, content := range map[string]string{
		"Literal.md": "a needle here\n", "Folded.md": "THE NEEDLE\n", "数据.md": "设计数据库的说明\n", "Word.md": "we go now\n",
		"Dates.md": "x\n2026-10-21\n", "Plain.md": "nothing here\n",
	} {
		r.mcpWrite(t, path, content)
	}
	drainSearch(t, r.pool)
	finds := func(pattern string, keyed bool, want ...string) {
		t.Helper()
		before := len(grepWalkLines(t, r.logs))
		pages := r.grepThrough(t, map[string]any{"pattern": pattern})
		got := hitsOf(pages)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) || len(pages) != 1 || pages[0].Building || pages[0].Incomplete {
			t.Errorf("grep %q = %+v, want %q, not building", pattern, pages, want)
		}
		walks := grepWalkLines(t, r.logs)[before:]
		if len(walks) != 1 || (walks[0].Keys > 0) != keyed || (walks[0].Candidates >= 0) != keyed {
			t.Errorf("grep %q walked %+v, want one walk, keyed %v", pattern, walks, keyed)
		}
	}
	finds("needle", true, "Literal.md:1")
	finds("(?i)needle", true, "Literal.md:1", "Folded.md:1")
	finds("数据库", true, "数据.md:1")
	finds(" go ", true, "Word.md:1")
	finds(`\d{4}-\d{2}-\d{2}`, false, "Dates.md:2")
	r.mcpWrite(t, "Fresh.md", "a fresh needle\n")
	finds("needle", true, "Literal.md:1", "Fresh.md:1")
	answerOf[map[string]any](t, r.use(t, "batch", map[string]any{"message": "batched",
		"ops": []any{map[string]any{"op": "write", "path": "Batched.md", "content": "a batched needle\n"}}}))
	if a := r.grep(t, map[string]any{"pattern": "needle"}); !a.Building || slices.Contains(hitsOf([]mcpGrepAnswer{a}), "Batched.md:1") {
		t.Errorf("a keyed grep after the batch = %+v, want building, the batch's page not found", a)
	}
	if a := r.grep(t, map[string]any{"pattern": "needle|zqzqz"}); a.Building || !slices.Contains(hitsOf([]mcpGrepAnswer{a}), "Batched.md:1") {
		t.Errorf("a grep of no keys after the batch = %+v, want the batch's page, not building", a)
	}
	drainSearch(t, r.pool)
	finds("needle", true, "Literal.md:1", "Fresh.md:1", "Batched.md:1")
}

// grepCutContents are a content of each cut of the search's tokens (M9/P4
// design 3.3), needle where each leaves text out: past
// MaxSearchLexemeBytes, after 240 KiB of random CJK; inside a run of
// ASCII letters and digits; inside a data: string.
func grepCutContents(rng *rand.Rand) map[string]string {
	var cjk strings.Builder
	for range 240 << 10 / 3 {
		cjk.WriteRune(rune(0x4E00 + rng.IntN(0x9FFF-0x4E00)))
	}
	return map[string]string{
		"Bound": "x\n" + cjk.String() + " needle\n",
		"Long":  "a run x" + strings.Repeat("ab", 30) + "needle" + strings.Repeat("cd", 10) + "\n",
		"Data":  "![a](data:image/png;base64," + strings.Repeat("Qk", 30) + "needle" + strings.Repeat("Zx", 20) + ")\n",
	}
}

// TestTheMCPGrepWalksEveryPartialPage: a page whose search row leaves
// out the text a match is in, of each cut, two each, is found by a keyed
// grep (M9/P4 design 6.1): before the grep the test reads that each row
// is partial and lacks needle's keys; the walk injected 5 pages, the six
// answer incomplete and a cursor, then the rest, each found once.
func TestTheMCPGrepWalksEveryPartialPage(t *testing.T) {
	r := startGrepApp(t, wiring{page: func(d *page.Deps) { d.GrepWalk = 5 }})
	rng := rand.New(rand.NewPCG(4, 6))
	ids, want := map[string]string{}, []string{}
	for i := range 2 {
		for kind, content := range grepCutContents(rng) {
			path := fmt.Sprintf("%s %d.md", kind, i)
			ids[path] = r.mcpWrite(t, path, content)
			want = append(want, fmt.Sprintf("%s:%d", path, strings.Count(content, "\n")))
		}
	}
	r.mcpWrite(t, "Plain.md", "no match here\n")
	drainSearch(t, r.pool)
	re, err := syntax.Parse("needle", syntax.Perl)
	if err != nil {
		t.Fatal(err)
	}
	keys := domain.GrepKeys(re)
	for path, id := range ids {
		var partial bool
		var keyed int
		if err := r.pool.QueryRow(context.Background(), `SELECT partial FROM page_search WHERE node_id = $1`, id).Scan(&partial); err != nil {
			t.Fatal(err)
		}
		if err := r.pool.QueryRow(context.Background(), `SELECT count(*) FROM page_search_hits(ARRAY[$1::uuid], $2::text[], NULL::text[]) h (id)
			WHERE h.id = $3`, r.notebook, keys, id).Scan(&keyed); err != nil {
			t.Fatal(err)
		}
		if !partial || keyed != 0 {
			t.Errorf("%s's row: partial %v, holding needle's keys %q %v; want partial, without them", path, partial, keys, keyed != 0)
		}
	}
	pages := r.grepThrough(t, map[string]any{"pattern": "needle"})
	got := hitsOf(pages)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) || len(pages) != 2 || !pages[0].Incomplete || pages[1].Incomplete || len(pages[0].Hits) != 5 {
		t.Errorf("grep needle = %+v, want %q, five then one, the first incomplete", pages, want)
	}
}

// TestTheMCPGrepStopsAtItsBounds: each of grep's bounds, injected small,
// stops a call once, incomplete with a cursor, and the calls that go on
// from it find every hit once (M9/P4 design 6.1): the walk's 500 pages as
// 5, the read's 32 MiB as 1 MiB, the call's work; the "grep walked" lines
// tell each stop. A line's work cut small leaves a line whose match is
// past the cut in partial_lines, one before it a hit. The limit's stop is
// not incomplete; an answer cut at 48 KiB, its walk incomplete, is not
// either, and its cursor goes on after its last line (mutant 170).
func TestTheMCPGrepStopsAtItsBounds(t *testing.T) {
	insts := func(pattern string) int64 {
		t.Helper()
		p, err := domain.ParseGrepPattern(pattern, false)
		if err != nil {
			t.Fatal(err)
		}
		return p.Insts
	}
	// once checks that pages hold every hit of want once, and the logs
	// the stop why, then done.
	once := func(t *testing.T, r readsApp, pages []mcpGrepAnswer, want []string, why string) {
		t.Helper()
		got := hitsOf(pages)
		slices.Sort(got)
		slices.Sort(want)
		walks := grepWalks(t, r.logs)
		if !slices.Equal(got, want) || len(pages) < 2 || !slices.Contains(walks, why) || walks[len(walks)-1] != "done" {
			t.Errorf("%d pages of hits %q, the walks %q; want %q once each, a stop at %s, then done", len(pages), got, walks, want, why)
		}
	}
	t.Run("the walk's pages, the limit and the answer's bound", func(t *testing.T) {
		r := startGrepApp(t, wiring{page: func(d *page.Deps) { d.GrepWalk = 5 }})
		var want []string
		for i := range 12 {
			var b strings.Builder
			for j := range 12 {
				fmt.Fprintf(&b, "%s a hit %d\n", strings.Repeat("y", 1200), j)
				want = append(want, fmt.Sprintf("P%02d.md:%d", i, j+1))
			}
			r.mcpWrite(t, fmt.Sprintf("P%02d.md", i), b.String())
		}
		first := r.grep(t, map[string]any{"pattern": "hit", "limit": 500})
		walks := grepWalks(t, r.logs)
		if n := len(first.Hits); n == 0 || n >= 60 || first.Incomplete || first.NextCursor == nil || walks[0] != "pages" ||
			!first.Hits[n-1].Cut {
			t.Errorf("the first answer: %d hits, incomplete %v, next %v, the walk %q; want fewer than its 60, its walk's pages stop, "+
				"not incomplete", n, first.Incomplete, first.NextCursor, walks)
		}
		pages := slices.Concat([]mcpGrepAnswer{first}, r.grepThrough(t, map[string]any{"pattern": "hit", "limit": 500,
			"cursor": *first.NextCursor}))
		once(t, r, pages, want, "pages")
		limited := r.grep(t, map[string]any{"pattern": "hit", "limit": 2})
		if len(limited.Hits) != 2 || limited.Incomplete || limited.NextCursor == nil || grepWalks(t, r.logs)[len(grepWalks(t, r.logs))-1] != "limit" {
			t.Errorf("limit 2 = %+v, want two hits, a cursor, not incomplete", limited)
		}
	})
	t.Run("the read's bytes", func(t *testing.T) {
		r := startGrepApp(t, wiring{page: func(d *page.Deps) { d.MaxGrepRead = 1 << 20 }})
		var want []string
		line := strings.Repeat("x", 1023) + "\n"
		for i := range 5 {
			path := fmt.Sprintf("B%d.md", i)
			r.mcpWrite(t, path, strings.Repeat(line, 1023)+"a hit\n")
			want = append(want, path+":1024")
		}
		once(t, r, r.grepThrough(t, map[string]any{"pattern": "hit"}), want, "bytes")
	})
	t.Run("the call's work", func(t *testing.T) {
		const line = "a line of the page\n"
		content := strings.Repeat(line, 49) + "a hit\n" + strings.Repeat(line, 50)
		work := 7 * domain.GrepWork(len(line)-1, insts("hit")) * 100 / 2 // three pages and a half
		r := startGrepApp(t, wiring{page: func(d *page.Deps) { d.MaxGrepWork = work }})
		var want []string
		for i := range 10 {
			path := fmt.Sprintf("W%d.md", i)
			r.mcpWrite(t, path, content)
			want = append(want, path+":50")
		}
		once(t, r, r.grepThrough(t, map[string]any{"pattern": "hit"}), want, "work")
	})
	t.Run("a line's work", func(t *testing.T) {
		lineWork := 102 * insts("hit") // GrepLineCut 100
		r := startGrepApp(t, wiring{page: func(d *page.Deps) { d.MaxGrepLineWork = lineWork }})
		r.mcpWrite(t, "Long.md", "hit "+strings.Repeat("y", 300)+"\n"+strings.Repeat("y", 300)+" hit\nplain\n")
		a := r.grep(t, map[string]any{"pattern": "hit"})
		if len(a.Hits) != 1 || a.Hits[0].at() != "Long.md:1" || len(a.PartialLines) != 1 || a.PartialLines[0] != (mcpGrepLine{"Long.md", 2}) ||
			a.Incomplete || a.NextCursor != nil {
			t.Errorf("grep with a line's work of 100 bytes = %+v, want line 1 a hit, line 2 partial", a)
		}
	})
}

// grepPieces are the lines TestTheMCPGrepLinesAreReads builds contents
// of, as page/app's TestGrepLinesAreReads does: empty ones, matches, a
// line not in NFC, a lone CR inside, and long ones.
func grepPieces(rng *rand.Rand) string {
	decomposed := "e\U00000301"
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
		return "\U000000e9t\U000000e9"
	case 5:
		return strings.Repeat(decomposed, 400+rng.IntN(400)) + " needle " + strings.Repeat("z", rng.IntN(1500))
	case 6:
		return strings.Repeat("y", 1200+rng.IntN(800)) + "needle"
	case 7:
		return "needle at the end y"
	}
	return "nothing"
}

// grepContent is a random content of grepPieces' lines ended by "\n" or
// "\r\n", maybe a BOM before, maybe no last newline.
func grepContent(rng *rand.Rand) string {
	var b strings.Builder
	if rng.IntN(4) == 0 {
		b.WriteString("\U0000FEFF")
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

// oracleLines are content's lines split apart from grep: by "\n", none
// after a last "\n", each without one trailing "\r", the first without its
// BOM.
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
	lines[0] = strings.TrimPrefix(lines[0], "\U0000FEFF")
	return lines
}

// TestTheMCPGrepLinesAreReads: page/app's property through the whole
// program (M9/P4 design 6.1): contents MCP wrote, of CRLF, a lone CR, a
// BOM, empty lines, no last newline, long lines and lines not in NFC; the
// lines grep finds are, as a set, those of an oracle that splits them
// apart and matches their NFC; each hit's line, read by MCP's read at its
// number, is the hit's text with its line end, and a cut hit's text is
// where read reads from its column.
func TestTheMCPGrepLinesAreReads(t *testing.T) {
	r := startReadsApp(t, publicOrigin)
	rng := rand.New(rand.NewPCG(7, 9))
	contents := map[string]string{}
	for i := range 40 {
		path := fmt.Sprintf("p%02d.md", i)
		contents[path] = grepContent(rng)
		r.mcpWrite(t, path, contents[path])
	}
	for _, pattern := range []string{`needle`, `^$`, `y$`, `\rn`} {
		var got []string
		for _, p := range r.grepThrough(t, map[string]any{"pattern": pattern, "limit": 500}) {
			for _, h := range p.Hits {
				got = append(got, h.at())
				line := oracleLines(contents[h.Path])[h.Line-1]
				args := map[string]any{"path": h.Path, "offset": h.Line, "limit": 1}
				if h.Column != nil {
					args["column"] = *h.Column
				}
				_, text, code := readBack(t, r.use(t, "read", args))
				whole := strings.TrimSuffix(strings.TrimSuffix(text, "\n"), "\r")
				if h.Line == 1 && h.Column == nil {
					whole = strings.TrimPrefix(whole, "\U0000FEFF")
				}
				switch {
				case code != "":
					t.Errorf("%q: read %s:%d = %s", pattern, h.Path, h.Line, code)
				case !h.Cut && (whole != line || h.Text != line):
					t.Errorf("%q: %s:%d's hit %q, read %q, the oracle's line %q", pattern, h.Path, h.Line, h.Text, whole, line)
				case h.Cut && !strings.HasPrefix(text, h.Text):
					t.Errorf("%q: %s:%d's cut hit %.40q, read from %d %.40q", pattern, h.Path, h.Line, h.Text, *h.Column, text)
				}
			}
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
			t.Errorf("%q: grep found %q, the oracle %q", pattern, got, want)
		}
	}
}
