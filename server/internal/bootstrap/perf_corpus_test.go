//go:build !race

package bootstrap

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"maps"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A synthetic corpus for BenchmarkSearch (M12/P4 design 3.2): the three
// TSV files loadCorpus copies in, notebooks.tsv, nodes.tsv and
// contents.tsv, of 300,000 pages, 2% of them deleted, among 200 or 5,000
// notebooks by Zipf's law, written by hand,
//
//	NW_PERF_CORPUS_OUT=<dir> NW_PERF_CORPUS_NOTEBOOKS=200 NW_PERF_SEED=20261116 go test -count=1 -run '^TestPerfSearchCorpus$' -timeout 30m ./internal/bootstrap/
//
// NW_PERF_CORPUS_NOTEBOOKS is 200 when unset, NW_PERF_SEED 20261116,
// NW_PERF_CORPUS_PAGES 300,000; NW_PERF_CORPUS_SCHEMA=308eafc6 writes the
// notebooks' columns of that commit, M8's review's merge, which B1's
// comparison runs on (its nodes.tsv and contents.tsv are the same). The
// same seed and arguments write the same files.
//
// The words come from the repository's own text, two vocabularies in
// testdata that TestPerfVocabulary wrote once (NW_PERF_VOCAB_FROM names a
// clean checkout to read); a paragraph's words are drawn by their rank,
// by Zipf's law. The queries' words of searchBenchQueries are by class
// (perfQueryWords): a heavy one occurs as the words fall, and is planted
// in more pages when it falls short of its count; every other one never
// occurs but where it is planted, in exactly its count of live pages, a
// share of them in their names. The vocabularies hold no word that holds
// one, and a word that would spell one with the text before it is drawn
// again.

// perfQueryWord is a word of searchBenchQueries the corpus holds by
// design: its class, its documents among 300,000 pages (at least, for a
// heavy one), and the percentage of them that hold it in their names
// rather than in their contents.
type perfQueryWord struct {
	word  string
	class string
	pages int
	title int
}

// perfCorpusScale is the corpus's pages the table's counts are of.
const perfCorpusScale = 300_000

// perfQueryWords are the classes of the queries' words, against M8's
// queries (M8/P5 design 3.9, "查询" and "计数"): the counts M8 recorded on
// its real corpus where it did; heavy, 5% of the pages at least, for the
// commonest keys; medium, 300 to 3,000; light, 50 to 300; rare, 1 to 20,
// the words whose walks run out. A word of many names and few contents,
// pope, holds 40% of its documents in names; a single CJK character, which
// a query of it alone finds in names alone (domain.ParseSearchQuery's
// TitlesOnly), half of them or more. The queries of more words than one
// co-occur as their words fall, and are not planted.
func perfQueryWords() []perfQueryWord {
	heavy := func(w string) perfQueryWord { return perfQueryWord{w, "heavy", 15_000, 0} }
	return []perfQueryWord{
		heavy("的"), heavy("the"), heavy("of"), heavy("a"), heavy("ing"), heavy("中国"),
		heavy("数据库"), heavy("索引"), heavy("history"), heavy("api"), heavy("go"), heavy("c"), heavy("ai"),
		heavy("linux"), heavy("kernel"),
		{"A股", "M8", 406, 10}, {"latency", "M8", 225, 5}, {"Windows系统", "M8", 201, 10}, {"Go语言", "M8", 12, 10},
		{"锅", "medium", 1_000, 50}, {"共和国", "medium", 3_000, 15}, {"参考文献", "medium", 3_000, 0},
		{"轨道交通", "medium", 600, 20}, {"绩效", "medium", 400, 10}, {"pope", "medium", 1_500, 40},
		{"鉴权", "light", 60, 10}, {"元数据", "light", 150, 10}, {"微服务", "light", 80, 10}, {"空中加油", "light", 50, 20},
		{"全文检索", "light", 60, 10}, {"C语言", "light", 120, 15}, {"API接口", "light", 80, 10}, {"kubernetes", "light", 100, 10},
		{"veress", "rare", 3, 0}, {"ranters", "rare", 7, 0}, {"latere", "rare", 10, 0}, {"geass", "rare", 18, 20},
		{"犇", "rare", 5, 60}, {"囧", "rare", 12, 50},
	}
}

// perfForbidden are the folded spellings of the words that are planted,
// every class's but heavy's.
func perfForbidden() []string {
	var out []string
	for _, q := range perfQueryWords() {
		if q.class != "heavy" {
			out = append(out, shared.TitleKey(q.word))
		}
	}
	return out
}

// perfClean reports whether the folded text holds none of the words that
// are only planted.
func perfClean(forbidden []string, folded string) bool {
	for _, f := range forbidden {
		if strings.Contains(folded, f) {
			return false
		}
	}
	return true
}

// perfWordRune reports whether r is of a word, as the search's tokens
// class it: a letter, a digit or a mark, but a CJK character
// (domain.classOf).
func perfWordRune(r rune) bool {
	if r < utf8.RuneSelf {
		return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9'
	}
	return !unicode.Is(unicode.Han, r) && (unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r))
}

// perfBounded reports whether the folded word w starts and ends with a
// run of one or two characters of a word, which the search matches whole
// alone (domain.SearchTerm.Pattern).
func perfBounded(w string) (left, right bool) {
	run := func(rs []rune) int {
		n := 0
		for n < len(rs) && perfWordRune(rs[n]) {
			n++
		}
		return n
	}
	rs := []rune(w)
	first := run(rs)
	slices.Reverse(rs)
	last := run(rs)
	return first >= 1 && first <= 2, last >= 1 && last <= 2
}

// perfHolds reports whether the folded text holds the folded word w as
// the search matches it: where w's run of one or two characters of a word
// starts or ends it, with no such character beside it there.
func perfHolds(text, w string) bool {
	left, right := perfBounded(w)
	for i := 0; i < len(text); {
		j := strings.Index(text[i:], w)
		if j < 0 {
			return false
		}
		at, end := i+j, i+j+len(w)
		before, _ := utf8.DecodeLastRuneInString(text[:at])
		after, _ := utf8.DecodeRuneInString(text[end:])
		if (!left || at == 0 || !perfWordRune(before)) && (!right || end == len(text) || !perfWordRune(after)) {
			return true
		}
		_, size := utf8.DecodeRuneInString(text[at:])
		i = at + size
	}
	return false
}

// perfHan reports whether s holds a Han character: a word planted between
// full-width commas, rather than spaces.
func perfHan(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return unicode.Is(unicode.Han, r) })
}

// perfWords are a vocabulary's words, folded, by their rank, and the
// cumulative weights of Zipf's law with s = 1 over them: integers, so
// that a draw is an integer's and the same on every machine.
type perfWords struct {
	words []string
	cum   []uint64
}

func newPerfWords(words []string) perfWords {
	cum := make([]uint64, len(words))
	var total uint64
	for k := range words {
		total += (1 << 40) / uint64(k+1)
		cum[k] = total
	}
	return perfWords{words: words, cum: cum}
}

// rank is the rank drawn by r.
func (w perfWords) rank(r *rand.Rand) int {
	u := r.Uint64N(w.cum[len(w.cum)-1])
	return sort.Search(len(w.cum), func(i int) bool { return w.cum[i] > u })
}

// perfVocab are the corpus's two vocabularies.
type perfVocab struct{ zh, en perfWords }

// perfWordCount is a word of a vocabulary and how often the repository's
// text holds it.
type perfWordCount struct {
	word  string
	count int
}

// perfVocabFiles are the vocabularies' files in testdata.
func perfVocabFiles() (zh, en string) {
	return filepath.Join("testdata", "perf-vocab-zh.txt"), filepath.Join("testdata", "perf-vocab-en.txt")
}

// readPerfVocab reads a vocabulary's file: the commit its header names and
// its words in their order, each folded as it is written.
func readPerfVocab(path string) (string, []perfWordCount, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	var commit string
	var out []perfWordCount
	for i, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if rest, ok := strings.CutPrefix(line, "# "); ok {
			if c, ok := strings.CutPrefix(rest, "From commit "); ok {
				commit = strings.TrimSuffix(c, ".")
			}
			continue
		}
		word, n, ok := strings.Cut(line, "\t")
		count, err := strconv.Atoi(n)
		if !ok || err != nil || word == "" || shared.TitleKey(word) != word {
			return "", nil, fmt.Errorf("%s:%d: %q is no word and count", path, i+1, line)
		}
		out = append(out, perfWordCount{word, count})
	}
	return commit, out, nil
}

// perfVocabulary is the corpus's vocabularies, read from testdata.
func perfVocabulary(t testing.TB) perfVocab {
	t.Helper()
	zhFile, enFile := perfVocabFiles()
	var v perfVocab
	for _, f := range []struct {
		path string
		into *perfWords
	}{{zhFile, &v.zh}, {enFile, &v.en}} {
		_, words, err := readPerfVocab(f.path)
		if err != nil {
			t.Fatal(err)
		}
		var ws []string
		for _, w := range words {
			ws = append(ws, w.word)
		}
		*f.into = newPerfWords(ws)
	}
	return v
}

// perfCorpusSpec is a corpus to write: its pages, its notebooks, its seed,
// the commit whose notebooks' columns it writes ("" today's, or
// "308eafc6"), and its vocabularies.
type perfCorpusSpec struct {
	pages, notebooks int
	seed             uint64
	schema           string
	vocab            perfVocab
}

// perfCorpusSummary is what a corpus holds: its pages, live and deleted,
// its content's bytes, and the documents (live pages) whose contents and
// names hold each query's word.
type perfCorpusSummary struct {
	Pages, Live, Deleted, DeletedFolders int
	ContentBytes                         int64
	Body, Name, Documents                map[string]int
	SHA256                               map[string]string
}

// perfPage is a page of the corpus as its layout leaves it: its notebook,
// its place among the notebook's pages, its deletion's number (0 alive),
// and its name's words and what is planted in its name.
type perfPage struct {
	notebook, at, deleted int32
	words, planted        string
}

// perfPlant is a word planted in a page's content.
type perfPlant struct{ word string }

// perfCorpus is a corpus being written.
type perfCorpus struct {
	spec       perfCorpusSpec
	forbidden  []string
	inner      map[rune]bool
	origin     time.Time
	notebooks  []uuid.UUID
	pages      []perfPage
	byNotebook [][]int32
	plants     map[int32][]perfPlant
}

// The layout: under the notebook's root, a folder page, perfFolders folder
// pages under it, perfLeaves pages under each.
const (
	perfFolders = 10
	perfLeaves  = 100
	perfBlock   = 1 + perfFolders*(1+perfLeaves)
)

// level is the level of the notebook's page at, 1 to 3, and the place of
// its parent among the notebook's pages, -1 for none.
func perfLevel(at int) (level, parent int) {
	r := at % perfBlock
	if r == 0 {
		return 1, -1
	}
	t := (r - 1) % (1 + perfLeaves)
	if t == 0 {
		return 2, at - r
	}
	return 3, at - t
}

// perfID is an id derived from the seed, a kind and a number, with its
// time's milliseconds in front and the version and variant of a UUIDv7.
func perfID(seed uint64, kind byte, n int, at time.Time) uuid.UUID {
	var in [17]byte
	binary.BigEndian.PutUint64(in[:8], seed)
	in[8] = kind
	binary.BigEndian.PutUint64(in[9:], uint64(n))
	h := sha256.Sum256(in[:])
	var u uuid.UUID
	ms := uint64(at.UnixMilli())
	for i := range 6 {
		u[i] = byte(ms >> (40 - 8*i))
	}
	copy(u[6:], h[:10])
	u[6] = 0x70 | u[6]&0x0f
	u[8] = 0x80 | u[8]&0x3f
	return u
}

// writePerfCorpus writes the corpus of spec into dir and answers what it
// holds; it writes nothing when the counts of its queries' words are not
// the table's.
func writePerfCorpus(dir string, spec perfCorpusSpec) (perfCorpusSummary, error) {
	if spec.pages < 1 || spec.notebooks < 1 || spec.schema != "" && spec.schema != "308eafc6" ||
		len(spec.vocab.zh.words) == 0 || len(spec.vocab.en.words) == 0 {
		return perfCorpusSummary{}, fmt.Errorf("no corpus of %d pages in %d notebooks, schema %q", spec.pages, spec.notebooks, spec.schema)
	}
	c := &perfCorpus{spec: spec, forbidden: perfForbidden(), inner: map[rune]bool{},
		origin: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), plants: map[int32][]perfPlant{}}
	for _, f := range c.forbidden {
		for i, r := range f {
			if i > 0 {
				c.inner[r] = true
			}
		}
	}
	c.layout()
	if err := c.names(); err != nil {
		return perfCorpusSummary{}, err
	}
	deletedFolders := c.deletions()
	names := c.plant()
	natural, err := c.natural()
	if err != nil {
		return perfCorpusSummary{}, err
	}
	c.topUp(natural)
	s, err := c.write(dir, names)
	s.DeletedFolders = deletedFolders
	return s, err
}

// at is the time of the corpus's n-th second.
func (c *perfCorpus) at(n int) time.Time { return c.origin.Add(time.Duration(n) * time.Second) }

// layout draws each page's notebook by Zipf's law, s = 1: the largest
// notebook holds 1/H(N) of the pages.
func (c *perfCorpus) layout() {
	names := make([]string, c.spec.notebooks)
	for b := range c.spec.notebooks {
		c.notebooks = append(c.notebooks, perfID(c.spec.seed, 'b', b, c.origin))
		names[b] = fmt.Sprintf("Corpus %04d", b+1)
	}
	z := newPerfWords(names)
	r := rand.New(rand.NewPCG(c.spec.seed, 1))
	c.byNotebook = make([][]int32, c.spec.notebooks)
	c.pages = make([]perfPage, c.spec.pages)
	for i := range c.spec.pages {
		b := z.rank(r)
		c.pages[i] = perfPage{notebook: int32(b), at: int32(len(c.byNotebook[b]))}
		c.byNotebook[b] = append(c.byNotebook[b], int32(i))
	}
}

// names draws each page's name's words: two to four of one vocabulary.
func (c *perfCorpus) names() error {
	for i := range c.pages {
		r := rand.New(rand.NewPCG(c.spec.seed, 2<<32|uint64(i)))
		words, err := c.run(r, r.IntN(10) < 6, 2+r.IntN(3))
		if err != nil {
			return fmt.Errorf("the name of page %d: %w", i, err)
		}
		c.pages[i].words = words
	}
	return nil
}

// name is the page i's name: its words, what is planted in it, and a
// capital Latin letter, its number and Σς, so that it is one among its
// siblings and its key differs from it and from its lowercase.
func (c *perfCorpus) name(i int) string {
	p := c.pages[i]
	return fmt.Sprintf("%s%s %c%d Σς", p.words, p.planted, 'A'+i%26, i)
}

// deletions deletes 1% of the pages by whole folders of the second level,
// each with its pages at one time, then single pages of the third until
// 2% are; answers how many folders it deleted. Each deletion is numbered
// in its order; its time follows the pages' creations.
func (c *perfCorpus) deletions() int {
	r := rand.New(rand.NewPCG(c.spec.seed, 3))
	var folders, leaves []int32
	held := map[int32][]int32{}
	for i, p := range c.pages {
		switch level, parent := perfLevel(int(p.at)); level {
		case 2:
			folders = append(folders, int32(i))
		case 3:
			f := c.byNotebook[p.notebook][parent]
			held[f] = append(held[f], int32(i))
			leaves = append(leaves, int32(i))
		}
	}
	r.Shuffle(len(folders), func(i, j int) { folders[i], folders[j] = folders[j], folders[i] })
	r.Shuffle(len(leaves), func(i, j int) { leaves[i], leaves[j] = leaves[j], leaves[i] })
	deleted, n, k := 0, int32(0), 0
	for _, f := range folders {
		if deleted >= max(1, c.spec.pages/100) {
			break
		}
		if len(held[f]) == 0 {
			continue
		}
		n++
		k++
		for _, i := range append([]int32{f}, held[f]...) {
			c.pages[i].deleted = n
			deleted++
		}
	}
	for _, i := range leaves {
		if deleted >= max(2, c.spec.pages/50) {
			break
		}
		if c.pages[i].deleted == 0 {
			n++
			c.pages[i].deleted = n
			deleted++
		}
	}
	return k
}

// live are the live pages, in their order.
func (c *perfCorpus) live() []int32 {
	var out []int32
	for i, p := range c.pages {
		if p.deleted == 0 {
			out = append(out, int32(i))
		}
	}
	return out
}

// perfTarget is a word's documents among the live pages of a corpus of
// pages: its count of 300,000 pages in proportion, rounded, one at least;
// and of them those that hold it in their names.
func perfTarget(q perfQueryWord, pages int) (total, names int) {
	total = max(1, (q.pages*pages+perfCorpusScale/2)/perfCorpusScale)
	return total, (total*q.title + 50) / 100
}

// perfSeparated is w as it is planted: between full-width commas when it
// holds a Han character, else between spaces, so that it spells no other
// key with the text beside it.
func perfSeparated(w string) string {
	if perfHan(w) {
		return "，" + w + "，"
	}
	return " " + w + " "
}

// plant chooses, by the seed, each word's documents but the heavy ones'
// among the live pages, its share of them in their names, and answers
// how many names hold each.
func (c *perfCorpus) plant() map[string]int {
	live := c.live()
	names := map[string]int{}
	for k, q := range perfQueryWords() {
		if q.class == "heavy" {
			continue
		}
		total, inNames := perfTarget(q, len(c.pages))
		r := rand.New(rand.NewPCG(c.spec.seed, 1000+uint64(k)))
		chosen := perfChoose(r, live, total)
		for n, i := range chosen {
			if n < inNames {
				c.pages[i].planted += strings.TrimRight(perfSeparated(q.word), " ")
				continue
			}
			c.plants[i] = append(c.plants[i], perfPlant{q.word})
		}
		names[q.word] = inNames
	}
	return names
}

// perfChoose is n distinct pages of from, drawn by r, in the order drawn.
func perfChoose(r *rand.Rand, from []int32, n int) []int32 {
	n = min(n, len(from))
	taken := map[int32]bool{}
	var out []int32
	for len(out) < n {
		i := from[r.IntN(len(from))]
		if !taken[i] {
			taken[i] = true
			out = append(out, i)
		}
	}
	return out
}

// heavyWords are the heavy class's words, folded.
func perfHeavy() []string {
	var out []string
	for _, q := range perfQueryWords() {
		if q.class == "heavy" {
			out = append(out, shared.TitleKey(q.word))
		}
	}
	return out
}

// natural is, for each page, which heavy words its content and name hold
// as they fell, a bit for each.
func (c *perfCorpus) natural() ([]uint32, error) {
	heavy := perfHeavy()
	out := make([]uint32, len(c.pages))
	for i := range c.pages {
		content, err := c.content(i)
		if err != nil {
			return nil, err
		}
		key := shared.TitleKey(c.name(i))
		for k, h := range heavy {
			if perfHolds(content, h) || perfHolds(key, h) {
				out[i] |= 1 << k
			}
		}
	}
	return out, nil
}

// topUp plants each heavy word that fell short of its count in as many
// more live pages, by the seed.
func (c *perfCorpus) topUp(natural []uint32) {
	live := c.live()
	k := 0
	for n, q := range perfQueryWords() {
		if q.class != "heavy" {
			continue
		}
		var without []int32
		for _, i := range live {
			if natural[i]&(1<<k) == 0 {
				without = append(without, i)
			}
		}
		total, _ := perfTarget(q, len(c.pages))
		if short := total - (len(live) - len(without)); short > 0 {
			r := rand.New(rand.NewPCG(c.spec.seed, 1000+uint64(n)))
			for _, i := range perfChoose(r, without, short) {
				c.plants[i] = append(c.plants[i], perfPlant{q.word})
			}
		}
		k++
	}
}

// run is count words of one vocabulary, Chinese ones (zh) joined, English
// ones between spaces; a word that would spell a planted word with the
// text before it is drawn again.
func (c *perfCorpus) run(r *rand.Rand, zh bool, count int) (string, error) {
	words, sep := c.spec.vocab.en, " "
	if zh {
		words, sep = c.spec.vocab.zh, ""
	}
	var b strings.Builder
	tail := ""
	for n := range count {
		if n > 0 {
			b.WriteString(sep)
		}
		w, ok := c.word(r, words, tail)
		if !ok {
			return "", errors.New("every word drawn spells a planted word with the text before it")
		}
		b.WriteString(w)
		tail = perfTail(tail, w, zh)
	}
	if out := b.String(); perfClean(c.forbidden, out) {
		return out, nil
	}
	return "", errors.New("a run spells a planted word")
}

// word draws a word of words that spells no planted word with tail, the
// text before it since the last separator.
func (c *perfCorpus) word(r *rand.Rand, words perfWords, tail string) (string, bool) {
	for range 64 {
		w := words.words[words.rank(r)]
		if c.joins(tail, w) {
			continue
		}
		return w, true
	}
	return "", false
}

// joins reports whether w after tail spells a planted word across them:
// it must hold w's first character past its own first.
func (c *perfCorpus) joins(tail, w string) bool {
	if tail == "" {
		return false
	}
	first, _ := utf8.DecodeRuneInString(w)
	return c.inner[first] && !perfClean(c.forbidden, tail+w)
}

// perfTail is the text a word is checked against: the last characters
// before it in a Chinese run, which joins its words; none in an English
// one, whose words are apart.
func perfTail(tail, w string, zh bool) string {
	if !zh {
		return ""
	}
	t := tail + w
	for len(t) > 24 {
		_, n := utf8.DecodeRuneInString(t)
		t = t[n:]
	}
	return t
}

// perfContentSize is a content's size drawn by r: log-normal, its median
// 6 KiB and its mean about 8 KiB, and one page in a thousand 200 KiB.
func perfContentSize(r *rand.Rand) int {
	if r.IntN(1000) == 0 {
		return 200 << 10
	}
	x := math.Exp(math.Log(6<<10) + 0.758*r.NormFloat64())
	return int(min(max(x, 256), 4<<20))
}

// content is the page i's content as its words fall, before what is
// planted in it: paragraphs, six in ten Chinese, one in six a heading.
func (c *perfCorpus) content(i int) (string, error) {
	r := rand.New(rand.NewPCG(c.spec.seed, 1<<32|uint64(i)))
	size := perfContentSize(r)
	var b strings.Builder
	b.Grow(size + 256)
	for b.Len() < size {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		p, err := c.paragraph(r, size-b.Len())
		if err != nil {
			return "", fmt.Errorf("page %d: %w", i, err)
		}
		b.WriteString(p)
	}
	return b.String(), nil
}

// paragraph is a paragraph of room bytes at most, a heading's of two to
// five words; drawn again, as a whole, when it spells a planted word.
func (c *perfCorpus) paragraph(r *rand.Rand, room int) (string, error) {
	zh, heading := r.IntN(10) < 6, r.IntN(6) == 0
	for range 16 {
		if heading {
			words, err := c.run(r, zh, 2+r.IntN(4))
			if err != nil {
				return "", err
			}
			return "## " + words, nil
		}
		p, err := c.sentences(r, zh, min(room, 200+r.IntN(1001)))
		if err != nil {
			return "", err
		}
		if perfClean(c.forbidden, p) {
			return p, nil
		}
	}
	return "", errors.New("every paragraph drawn spells a planted word")
}

// sentences are a paragraph's words to size bytes, punctuated: in Chinese
// a comma after one word in ten and a full stop after one in twenty-five;
// in English a comma after one in fourteen and a full stop after one in
// sixteen.
func (c *perfCorpus) sentences(r *rand.Rand, zh bool, size int) (string, error) {
	words, sep, comma, stop, commas, stops := c.spec.vocab.en, " ", ",", ".", 7, 13
	if zh {
		words, sep, comma, stop, commas, stops = c.spec.vocab.zh, "", "，", "。", 10, 14
	}
	var b strings.Builder
	tail := ""
	for b.Len() < size {
		if b.Len() > 0 {
			b.WriteString(sep)
		}
		w, ok := c.word(r, words, tail)
		if !ok {
			// A comma parts the run: nothing spells across it.
			b.WriteString(comma)
			tail = ""
			continue
		}
		b.WriteString(w)
		tail = perfTail(tail, w, zh)
		switch u := r.IntN(100); {
		case u < commas:
			b.WriteString(comma)
			tail = ""
		case u < stops:
			b.WriteString(stop)
			tail = ""
		}
	}
	b.WriteString(stop)
	return b.String(), nil
}

// perfEscape is s in COPY's text format: backslash, tab, newline and
// carriage return escaped.
func perfEscape(s string) string {
	if !strings.ContainsAny(s, "\\\t\n\r") {
		return s
	}
	return strings.NewReplacer(`\`, `\\`, "\t", `\t`, "\n", `\n`, "\r", `\r`).Replace(s)
}

// perfTime is t as a COPY's timestamptz.
func perfTime(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05Z07:00") }

// write writes the three files under temporary names, counts each query's
// word in the live pages' contents and names, folded, and renames them
// into place when the counts are the table's: exactly what is planted for
// a word that is, at least its count for a heavy one; else it removes them.
func (c *perfCorpus) write(dir string, names map[string]int) (perfCorpusSummary, error) {
	s := perfCorpusSummary{Pages: len(c.pages), Body: map[string]int{}, Name: map[string]int{}, Documents: map[string]int{},
		SHA256: map[string]string{}}
	files := []string{"notebooks.tsv", "nodes.tsv", "contents.tsv"}
	tmp := func(f string) string { return filepath.Join(dir, "."+f+".tmp") }
	written := false
	defer func() {
		if !written {
			for _, f := range files {
				_ = os.Remove(tmp(f))
			}
		}
	}()
	const user, workspace = "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002"
	out := map[string]*bufio.Writer{}
	hashes := map[string]hash.Hash{}
	opened := map[string]*os.File{}
	defer func() {
		for _, f := range opened {
			_ = f.Close()
		}
	}()
	for _, f := range files {
		h := sha256.New()
		file, err := os.Create(tmp(f))
		if err != nil {
			return s, err
		}
		opened[f] = file
		out[f] = bufio.NewWriterSize(io.MultiWriter(file, h), 1<<20)
		hashes[f] = h
	}
	// A bufio.Writer keeps its first error, which Flush answers.
	row := func(f string, fields ...string) { _, _ = out[f].WriteString(strings.Join(fields, "\t") + "\n") }
	// Each notebook lets the workspace's members read it, so that alice,
	// its admin, sees every one (notebooks.sql's ListNotebooks).
	for b, id := range c.notebooks {
		fields := []string{id.String(), workspace, fmt.Sprintf("Corpus %04d", b+1), "viewer", `\N`, `\N`, user, user,
			perfTime(c.origin), perfTime(c.origin), `\N`}
		if c.spec.schema == "" {
			fields = append(fields, "standard", `\N`, `\N`, `\N`, `\N`, `\N`)
		}
		row("notebooks.tsv", fields...)
	}
	ids := make([]uuid.UUID, len(c.pages))
	deletedAt := func(p perfPage) string {
		if p.deleted == 0 {
			return `\N`
		}
		return perfTime(c.at(len(c.pages) + int(p.deleted)))
	}
	queries := perfQueryWords()
	for i, p := range c.pages {
		created := c.at(i + 1)
		ids[i] = perfID(c.spec.seed, 'n', i, created)
		parent := `\N`
		if _, at := perfLevel(int(p.at)); at >= 0 {
			parent = ids[c.byNotebook[p.notebook][at]].String()
		}
		name := c.name(i)
		key := shared.TitleKey(name)
		if len(name) > shared.MaxTitleBytes {
			return s, fmt.Errorf("page %d's name is %d bytes", i, len(name))
		}
		row("nodes.tsv", ids[i].String(), c.notebooks[p.notebook].String(), parent, "page", perfEscape(name), perfEscape(key),
			strconv.Itoa(int(p.at)), user, user, perfTime(created), perfTime(created), deletedAt(p))
		content, err := c.content(i)
		if err != nil {
			return s, err
		}
		for _, w := range c.plants[int32(i)] {
			content += "\n\n" + perfSeparated(w.word)
		}
		sum := sha256.Sum256([]byte(content))
		row("contents.tsv", ids[i].String(), perfEscape(content), "1", `\\x`+hex.EncodeToString(sum[:]), strconv.Itoa(len(content)),
			user, perfTime(created), deletedAt(p))
		if p.deleted != 0 {
			s.Deleted++
			continue
		}
		s.Live++
		s.ContentBytes += int64(len(content))
		folded := shared.TitleKey(content)
		for _, q := range queries {
			w := shared.TitleKey(q.word)
			inBody, inName := perfHolds(folded, w), perfHolds(key, w)
			if inBody {
				s.Body[q.word]++
			}
			if inName {
				s.Name[q.word]++
			}
			if inBody || inName {
				s.Documents[q.word]++
			}
		}
	}
	for _, f := range files {
		if err := out[f].Flush(); err != nil {
			return s, err
		}
		err := opened[f].Close()
		delete(opened, f)
		if err != nil {
			return s, err
		}
	}
	for _, q := range queries {
		total, _ := perfTarget(q, s.Pages)
		if q.class == "heavy" {
			if s.Documents[q.word] < total {
				return s, fmt.Errorf("%q is in %d live pages, want %d at least", q.word, s.Documents[q.word], total)
			}
			continue
		}
		if s.Body[q.word] != total-names[q.word] || s.Name[q.word] != names[q.word] {
			return s, fmt.Errorf("%q is in %d contents and %d names, want %d and %d", q.word, s.Body[q.word], s.Name[q.word],
				total-names[q.word], names[q.word])
		}
	}
	for _, f := range files {
		s.SHA256[f] = hex.EncodeToString(hashes[f].Sum(nil))
		if err := os.Rename(tmp(f), filepath.Join(dir, f)); err != nil {
			return s, err
		}
	}
	written = true
	return s, nil
}

// perfText is a file of the repository's text: Markdown, whose every word
// counts, or code, whose comments' do.
type perfText struct {
	path, text string
	markdown   bool
}

// perfMaxWords is how many words a vocabulary keeps at most.
const perfMaxWords = 20_000

// perfVocabularyOf counts the words of texts: of each run of Han
// characters in the Markdown, its two and three characters at each place;
// of the Markdown and the code's comments, each word of Latin letters
// alone, no digit, underscore or apostrophe beside it (M12's "M", it's
// "s"), lowercase, of 20 at most; the most frequent first, then by the word,
// perfMaxWords at most of each, without a word that holds a planted one.
func perfVocabularyOf(texts []perfText) (zh, en []perfWordCount) {
	forbidden := perfForbidden()
	han, latin := map[string]int{}, map[string]int{}
	tokens, letters := regexp.MustCompile(`[A-Za-z0-9_'’]+`), regexp.MustCompile(`^[A-Za-z]+$`)
	for _, t := range texts {
		english := t.text
		if t.markdown {
			var run []rune
			flush := func() {
				for n := 2; n <= 3; n++ {
					for i := 0; i+n <= len(run); i++ {
						han[string(run[i:i+n])]++
					}
				}
				run = run[:0]
			}
			for _, r := range t.text {
				if unicode.Is(unicode.Han, r) {
					run = append(run, r)
					continue
				}
				flush()
			}
			flush()
		} else {
			english = perfComments(t.text)
		}
		for _, w := range tokens.FindAllString(english, -1) {
			if len(w) <= 20 && letters.MatchString(w) {
				latin[strings.ToLower(w)]++
			}
		}
	}
	keep := func(counts map[string]int) []perfWordCount {
		var out []perfWordCount
		for w, n := range counts {
			if key := shared.TitleKey(w); key == w && perfClean(forbidden, key) {
				out = append(out, perfWordCount{w, n})
			}
		}
		slices.SortFunc(out, func(a, b perfWordCount) int {
			if a.count != b.count {
				return b.count - a.count
			}
			return strings.Compare(a.word, b.word)
		})
		return out[:min(len(out), perfMaxWords)]
	}
	return keep(han), keep(latin)
}

// perfComments are the comments of a Go or TypeScript source: each line's
// after //, and what /* */ holds; strings are not told apart.
func perfComments(src string) string {
	var b strings.Builder
	block := false
	for _, line := range strings.Split(src, "\n") {
		for line != "" {
			if block {
				end := strings.Index(line, "*/")
				if end < 0 {
					b.WriteString(line + "\n")
					line = ""
					continue
				}
				b.WriteString(line[:end] + "\n")
				line, block = line[end+2:], false
				continue
			}
			slash, star := strings.Index(line, "//"), strings.Index(line, "/*")
			switch {
			case slash >= 0 && (star < 0 || slash < star):
				b.WriteString(line[slash+2:] + "\n")
				line = ""
			case star >= 0:
				line, block = line[star+2:], true
			default:
				line = ""
			}
		}
	}
	return b.String()
}

// perfRepositoryText reads the text of the clean checkout dir: the
// Markdown under docs/, and the Go and TypeScript under server/ and web/,
// as git tracks them; answers its commit.
func perfRepositoryText(t *testing.T, dir string) (string, []perfText) {
	t.Helper()
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return string(out)
	}
	if status := git("status", "--porcelain"); status != "" {
		t.Fatalf("%s is not clean: %s", dir, status)
	}
	commit := strings.TrimSpace(git("rev-parse", "HEAD"))
	var texts []perfText
	for _, path := range strings.Split(strings.TrimRight(git("ls-files", "-z", "docs", "server", "web"), "\x00"), "\x00") {
		markdown := strings.HasPrefix(path, "docs/") && strings.HasSuffix(path, ".md")
		code := (strings.HasPrefix(path, "server/") || strings.HasPrefix(path, "web/")) &&
			slices.Contains([]string{".go", ".ts", ".tsx"}, filepath.Ext(path))
		if !markdown && !code {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, perfText{path: path, text: string(data), markdown: markdown})
	}
	return commit, texts
}

// writePerfVocab writes a vocabulary's file: its header, then a word and
// its count on each line.
func writePerfVocab(path, header string, words []perfWordCount) error {
	var b bytes.Buffer
	b.WriteString(header)
	for _, w := range words {
		fmt.Fprintf(&b, "%s\t%d\n", w.word, w.count)
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// TestPerfVocabulary writes the corpus's vocabularies into testdata from
// the clean checkout NW_PERF_VOCAB_FROM names, once (M12/P4 design 3.2):
// after it, a corpus reads them alone, so a seed writes the same corpus
// whatever the repository's text becomes.
func TestPerfVocabulary(t *testing.T) {
	dir := os.Getenv("NW_PERF_VOCAB_FROM")
	if dir == "" {
		t.Skip("NW_PERF_VOCAB_FROM names no checkout to read the words of")
	}
	commit, texts := perfRepositoryText(t, dir)
	zh, en := perfVocabularyOf(texts)
	zhFile, enFile := perfVocabFiles()
	if err := os.MkdirAll(filepath.Dir(zhFile), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct {
		path, what string
		words      []perfWordCount
	}{
		{zhFile, "Chinese: two and three characters at each place of each run of Han characters in docs/**/*.md", zh},
		{enFile, "English: each word of Latin letters alone, lowercase, of 20 at most, in docs/**/*.md and in the comments of server/'s and web/'s Go and TypeScript", en},
	} {
		header := fmt.Sprintf("# The search corpus's vocabulary (M12/P4 design 3.2), %s; a word and how often, most first,\n"+
			"# %d at most, without a word that holds a planted query's word. Written by TestPerfVocabulary.\n# From commit %s.\n",
			f.what, perfMaxWords, commit)
		if err := writePerfVocab(f.path, header, f.words); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d words of %d files of %s, the first %v", f.path, len(f.words), len(texts), commit, f.words[:min(5, len(f.words))])
	}
}

// perfEnv is the integer the environment's name gives, def when unset.
func perfEnv(t *testing.T, name string, def int) int {
	t.Helper()
	s := os.Getenv(name)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		t.Fatalf("%s=%q is no positive number", name, s)
	}
	return n
}

// TestPerfSearchCorpus writes the corpus into NW_PERF_CORPUS_OUT, by hand
// (M12/P4 design 3.2, batch B of S3): the arguments above the corpus's
// functions; it logs what the corpus holds and each file's SHA-256.
func TestPerfSearchCorpus(t *testing.T) {
	dir := os.Getenv("NW_PERF_CORPUS_OUT")
	if dir == "" {
		t.Skip("NW_PERF_CORPUS_OUT names no directory to write the corpus into")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := perfCorpusSpec{pages: perfEnv(t, "NW_PERF_CORPUS_PAGES", perfCorpusScale),
		notebooks: perfEnv(t, "NW_PERF_CORPUS_NOTEBOOKS", 200), seed: uint64(perfEnv(t, "NW_PERF_SEED", 20261116)),
		schema: os.Getenv("NW_PERF_CORPUS_SCHEMA"), vocab: perfVocabulary(t)}
	started := time.Now()
	s, err := writePerfCorpus(dir, spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d pages in %d notebooks, seed %d, schema %q: %d live, %d deleted (%d folders of the second level), %d bytes of live content, in %s",
		s.Pages, spec.notebooks, spec.seed, spec.schema, s.Live, s.Deleted, s.DeletedFolders, s.ContentBytes, time.Since(started).Round(time.Second))
	for _, q := range perfQueryWords() {
		t.Logf("%-12s %-6s %6d documents: %6d contents, %5d names", q.word, q.class, s.Documents[q.word], s.Body[q.word], s.Name[q.word])
	}
	for _, f := range slices.Sorted(maps.Keys(s.SHA256)) {
		t.Logf("sha256 %s  %s", s.SHA256[f], f)
	}
}

// perfCorpusIn writes the corpus of spec into a directory of the test's
// and answers it.
func perfCorpusIn(t *testing.T, spec perfCorpusSpec) (string, perfCorpusSummary) {
	t.Helper()
	dir := t.TempDir()
	s, err := writePerfCorpus(dir, spec)
	if err != nil {
		t.Fatal(err)
	}
	return dir, s
}

// perfFileSums are the SHA-256 of the corpus's three files in dir, read
// back.
func perfFileSums(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, f := range []string{"notebooks.tsv", "nodes.tsv", "contents.tsv"} {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		out[f] = hex.EncodeToString(sum[:])
	}
	return out
}

// perfRows are the rows of the TSV file f in dir, each its fields as
// written.
func perfRows(t *testing.T, dir, f string) [][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, f))
	if err != nil {
		t.Fatal(err)
	}
	var out [][]string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		out = append(out, strings.Split(line, "\t"))
	}
	return out
}

// perfUnescape is a field of COPY's text format as the server reads it.
func perfUnescape(s string) string {
	return strings.NewReplacer(`\\`, `\`, `\t`, "\t", `\n`, "\n", `\r`, "\r").Replace(s)
}

// The corpus is its seed's (M12/P4 design 3.2, 7.2's 1 and 2): 1,000
// pages written twice of one seed are the same three files, of another
// seed three others; of the same seed for 308eafc6, nodes.tsv and
// contents.tsv are the same and each notebook's row is today's but its
// last six columns, 00048's mode and source_root_id and 00055's upgrade's
// four.
func TestPerfCorpusIsItsSeed(t *testing.T) {
	vocab := perfVocabulary(t)
	spec := perfCorpusSpec{pages: 1000, notebooks: 200, seed: 20261116, vocab: vocab}
	first, _ := perfCorpusIn(t, spec)
	again, _ := perfCorpusIn(t, spec)
	sums, other := perfFileSums(t, first), perfFileSums(t, again)
	if !maps.Equal(sums, other) {
		t.Errorf("one seed wrote %v, then %v", sums, other)
	}
	spec.seed++
	another, _ := perfCorpusIn(t, spec)
	for f, sum := range perfFileSums(t, another) {
		if sum == sums[f] {
			t.Errorf("another seed wrote the same %s", f)
		}
	}
	spec.seed--
	spec.schema = "308eafc6"
	old, _ := perfCorpusIn(t, spec)
	oldSums := perfFileSums(t, old)
	for _, f := range []string{"nodes.tsv", "contents.tsv"} {
		if oldSums[f] != sums[f] {
			t.Errorf("308eafc6's %s differs from today's", f)
		}
	}
	today, then := perfRows(t, first, "notebooks.tsv"), perfRows(t, old, "notebooks.tsv")
	if len(today) != 200 || len(then) != len(today) {
		t.Fatalf("%d notebooks today, %d for 308eafc6, want 200", len(today), len(then))
	}
	for i := range today {
		if len(today[i]) != 17 || len(then[i]) != 11 || !slices.Equal(then[i], today[i][:11]) {
			t.Fatalf("notebook %d: today %q, for 308eafc6 %q; want 17 columns and their first 11", i, today[i], then[i])
		}
	}
}

// perfDocuments counts, in the corpus of dir as written, each query's
// word in the live pages' contents and names as the search's patterns
// match it (domain.SearchQuery.TitleHits), where its folded spelling is;
// a heavy word only up to atLeast documents.
func perfDocuments(t *testing.T, dir string, atLeast int) (live int, body, name map[string]int) {
	t.Helper()
	body, name = map[string]int{}, map[string]int{}
	type query struct {
		perfQueryWord
		folded string
		q      domain.SearchQuery
	}
	var queries []query
	for _, w := range perfQueryWords() {
		q, err := domain.ParseSearchQuery(&w.word)
		if err != nil || len(q.Terms) != 1 {
			t.Fatalf("%q parses as %+v, %v", w.word, q, err)
		}
		queries = append(queries, query{w, shared.TitleKey(w.word), q})
	}
	holds := func(q query, text, folded string) bool {
		return strings.Contains(folded, q.folded) && len(q.q.TitleHits(text)) > 0
	}
	nodes, contents := perfRows(t, dir, "nodes.tsv"), perfRows(t, dir, "contents.tsv")
	if len(nodes) != len(contents) {
		t.Fatalf("%d nodes, %d contents", len(nodes), len(contents))
	}
	for i, n := range nodes {
		c := contents[i]
		if c[0] != n[0] || c[7] != n[11] {
			t.Fatalf("content %d is %s's, deleted at %s; its node %s's, at %s", i, c[0], c[7], n[0], n[11])
		}
		content, nm := perfUnescape(c[1]), perfUnescape(n[4])
		folded, key := shared.TitleKey(content), shared.TitleKey(nm)
		if n[11] != `\N` {
			for _, q := range queries {
				if q.class != "heavy" && (strings.Contains(folded, q.folded) || strings.Contains(key, q.folded)) {
					t.Errorf("deleted page %s holds %q", n[0], q.word)
				}
			}
			continue
		}
		live++
		for _, q := range queries {
			if q.class == "heavy" && body[q.word]+name[q.word] >= atLeast {
				continue
			}
			if holds(q, content, folded) {
				body[q.word]++
			}
			if holds(q, nm, key) {
				name[q.word]++
			}
		}
	}
	return live, body, name
}

// The queries' words are where the table plants them (M12/P4 design 3.2,
// 7.2's 3 to 5): no word of either vocabulary holds one but a heavy one,
// nor does the vocabularies' counter keep one; on 3,000 pages, a hundredth
// of the table's 300,000, each word not heavy is in exactly its count of
// live pages' contents and names, in proportion, one at least, its share
// in names, and each heavy one in its count at least; on 100 pages of a
// vocabulary of two Chinese words that spell 共和国 when they meet, 共和国
// is still in its count alone.
func TestPerfCorpusPlantsTheQueries(t *testing.T) {
	forbidden := perfForbidden()
	for i, f := range forbidden {
		for j, g := range forbidden {
			if i != j && strings.Contains(f, g) {
				t.Fatalf("%q holds %q: a page of one counts for the other", f, g)
			}
		}
	}
	zhFile, enFile := perfVocabFiles()
	for _, path := range []string{zhFile, enFile} {
		commit, words, err := readPerfVocab(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(commit) != 40 || len(words) < 1000 || len(words) > perfMaxWords {
			t.Errorf("%s: of commit %q, %d words; want a commit and 1,000 to %d words", path, commit, len(words), perfMaxWords)
		}
		for _, w := range words {
			if !perfClean(forbidden, w.word) {
				t.Errorf("%s holds %q, which holds a planted word", path, w.word)
			}
		}
	}
	zh, en := perfVocabularyOf([]perfText{
		{path: "a.md", text: "元数据与共和国，参考文献，空中加油。A股 Windows系统 latency latencies pope popes 锅贴 囧", markdown: true},
		{path: "b.go", text: "x := 1 // kubernetes veress cluster geassed\n/* ranters latere\n*/ y := \"api\""},
	})
	if len(zh) == 0 || len(en) == 0 {
		t.Fatalf("the counter kept %v and %v of the sample", zh, en)
	}
	for _, w := range slices.Concat(zh, en) {
		if !perfClean(forbidden, w.word) {
			t.Errorf("the counter kept %q, which holds a planted word", w.word)
		}
	}

	check := func(t *testing.T, dir string, pages int) {
		t.Helper()
		live, body, name := perfDocuments(t, dir, max(1, (15_000*pages+150_000)/300_000))
		// A folder deleted whole may hold a hundred pages: 2% of 100 is
		// less than one.
		if pages >= 1000 && (live < pages*97/100 || live > pages*99/100) {
			t.Errorf("%d of %d pages live, want about 98%%", live, pages)
		}
		for _, q := range perfQueryWords() {
			total := max(1, (q.pages*pages+150_000)/300_000)
			names := (total*q.title + 50) / 100
			switch {
			case q.class == "heavy" && body[q.word]+name[q.word] < total:
				t.Errorf("%q is in %d contents and %d names, want %d documents at least", q.word, body[q.word], name[q.word], total)
			case q.class != "heavy" && (body[q.word] != total-names || name[q.word] != names):
				t.Errorf("%q is in %d contents and %d names, want %d and %d", q.word, body[q.word], name[q.word], total-names, names)
			}
		}
	}
	dir, _ := perfCorpusIn(t, perfCorpusSpec{pages: 3000, notebooks: 200, seed: 20261116, vocab: perfVocabulary(t)})
	check(t, dir, 3000)
	meeting := perfVocab{zh: newPerfWords([]string{"共和", "国度"}), en: newPerfWords([]string{"lorem", "ipsum"})}
	dir, _ = perfCorpusIn(t, perfCorpusSpec{pages: 100, notebooks: 5, seed: 7, vocab: meeting})
	check(t, dir, 100)
}

// The corpus loads into BenchmarkSearch's database and builds (M12/P4
// design 3.2, 7.2's 6 to 8): 1,000 pages through loadCorpus and
// buildSearch, as the benchmark loads them; 中国 is found; each node's key
// is shared.TitleKey of its name and not its name; under a deleted node
// every node is deleted at its time, and some deletions are of a folder
// with its pages, some of a page alone.
func TestPerfCorpusLoadsIntoTheBenchmark(t *testing.T) {
	bench := newTrashBench(t, 0, 0)
	dir, s := perfCorpusIn(t, perfCorpusSpec{pages: 1000, notebooks: 200, seed: 20261116, vocab: perfVocabulary(t)})
	slug := bench.loadCorpus(t, dir)
	bench.buildSearch(t)
	if n := count(t, bench.pool, "SELECT count(*) FROM nodes WHERE notebook_id <> $1", bench.nb); n != 1000 {
		t.Fatalf("%d nodes loaded, want 1,000", n)
	}
	status, answer := bench.serve(http.MethodGet, searchPath(slug, "中国", nil), "")
	var a searchAnswer
	if status == http.StatusOK {
		decodeAnswer(t, answer, &a)
	}
	if status != http.StatusOK || len(a.Data) == 0 {
		t.Errorf("the search for 中国 = %d %.300s, want hits of %d pages", status, answer, s.Documents["中国"])
	}
	rows, err := bench.pool.Query(t.Context(), "SELECT name, name_key FROM nodes WHERE notebook_id <> $1", bench.nb)
	if err != nil {
		t.Fatal(err)
	}
	keys := 0
	for rows.Next() {
		var name, key string
		if err := rows.Scan(&name, &key); err != nil {
			t.Fatal(err)
		}
		if key != shared.TitleKey(name) || key == name || key == strings.ToLower(name) {
			t.Errorf("%q's key is %q, want shared.TitleKey's, neither the name nor its lowercase", name, key)
		}
		keys++
	}
	if err := rows.Err(); err != nil || keys != 1000 {
		t.Fatalf("read %d keys, %v", keys, err)
	}
	if n := count(t, bench.pool, `SELECT count(*) FROM nodes c JOIN nodes p ON p.id = c.parent_id
		WHERE p.deleted_at IS NOT NULL AND c.deleted_at IS DISTINCT FROM p.deleted_at`); n != 0 {
		t.Errorf("%d nodes under a deleted one are not deleted at its time", n)
	}
	folders := count(t, bench.pool, `SELECT count(DISTINCT p.id) FROM nodes p JOIN nodes c ON c.parent_id = p.id
		WHERE p.deleted_at IS NOT NULL`)
	alone := count(t, bench.pool, `SELECT count(*) FROM nodes c LEFT JOIN nodes p ON p.id = c.parent_id
		WHERE c.deleted_at IS NOT NULL AND (p.id IS NULL OR p.deleted_at IS NULL)
			AND NOT EXISTS (SELECT 1 FROM nodes k WHERE k.parent_id = c.id)`)
	if folders != s.DeletedFolders || folders < 1 || alone < 1 {
		t.Errorf("%d deleted folders with their pages (%d written), %d pages deleted alone; want one of each at least", folders,
			s.DeletedFolders, alone)
	}
	t.Logf("%d pages, %d live, %d deleted: %d folders with their pages; %d bytes of live content; 中国 in %d pages, %d hits on the first page",
		s.Pages, s.Live, s.Deleted, s.DeletedFolders, s.ContentBytes, s.Documents["中国"], len(a.Data))
}
