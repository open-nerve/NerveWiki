package archiveadapter

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// The names' differential test (M12/P3 design 3.12.3; v0.1 design 13.4,
// item 11): random zips, archive/zip's, their names in valid UTF-8 past
// ASCII, in GBK (some of them valid UTF-8 by chance), in Shift-JIS or of
// any bytes, flagged UTF-8 or not, with a Unicode Path field or not, valid
// or of another name's CRC or of version 2. readNames is held to what the
// rule says of each name, judged from what the generator wrote and the GBK
// decoder alone, never from readNames:
//
//   - a flagged name is as written;
//   - a name with a valid Unicode Path field is the field's;
//   - in a zip with no candidate (a name not flagged, without a valid
//     field, not UTF-8) nothing else changes;
//   - an ASCII name changes by its field alone;
//   - a zip's candidates are all read as GBK or none, and none when one
//     reads with U+FFFD; then no name is read as GBK;
//   - in a zip whose candidates are read, every name not flagged, without
//     a valid field, past ASCII, that GBK reads with no U+FFFD, is read as
//     GBK, valid UTF-8 or not (decision 376);
//   - a name read as GBK is the decoder's reading of its bytes, with no
//     U+FFFD (not the round trip: two GBK pairs do not come back, review
//     C's M2); one read from its field is the field's, whose CRC is the
//     name's.
//
// The test runs seeds 1 to 3, 1,000 zips each. Each case a mutant would
// show in is counted, each count with a floor: two thirds, rounded down,
// of the fewest of 20 other groups of three seeds alike (3b+1 to 3b+3, b
// from 1 to 20; 05-lessons 1), measured on M12/P3/S3's generator. A change
// of how the generator draws moves every count: measure them all again.

// sample is an entry the generator writes: its name's bytes, flagged or
// not, and its Unicode Path field: none, valid, of another name's CRC, of
// version 2.
type sample struct {
	raw     string
	flagged bool
	field   int
	named   string // the field's name
}

const (
	noField = iota
	validField
	otherCRC
	version2
)

// The names the generator draws from, by kind. GBK's 目录 and 图片 are
// valid UTF-8 by chance; the rest of the Chinese ones are not.
func utf8Names() []string {
	return []string{"ü.md", "é", "中文.md", "naïve/x.md", "😀.png", "Ångström"}
}
func chanceNames() []string { return []string{"目录", "图片", "目录/图片", "图片/目录.md"} }
func gbkNames() []string {
	return []string{"测试资料.md", "截图.png", "文件/a.md", "资料/乗", "乗.md", "目录/文件.md", "中文", "€.md"}
}
func sjisNames() []string { return []string{"テスト資料.md", "ソース.md", "日本語/a.md"} }

// generator draws zips of samples from its seed.
type generator struct {
	r    *rand.Rand
	gbk  *encoding.Encoder
	sjis *encoding.Encoder
}

func (g generator) pick(names []string) string { return names[g.r.IntN(len(names))] }

func (g generator) encode(e *encoding.Encoder, s string) string {
	out, err := e.String(s)
	if err != nil {
		panic(err)
	}
	return out
}

// name draws a name of kind k: ASCII, UTF-8, GBK valid UTF-8 by chance,
// GBK, Shift-JIS, any bytes.
func (g generator) name(k int) string {
	switch k {
	case 0:
		return fmt.Sprintf("a%d.md", g.r.IntN(100))
	case 1:
		return g.pick(utf8Names())
	case 2:
		return g.encode(g.gbk, g.pick(chanceNames()))
	case 3:
		return g.encode(g.gbk, g.pick(gbkNames()))
	case 4:
		return g.encode(g.sjis, g.pick(sjisNames()))
	}
	b := []byte("b")
	for range 1 + g.r.IntN(4) {
		b = append(b, byte(0x80+g.r.IntN(0x80))) //nolint:gosec // under 0x100
	}
	return string(b) + ".md"
}

// zip draws a zip: one of no candidate (ASCII, UTF-8 and chance names
// alone, besides those flagged or with a valid field), of GBK and
// Shift-JIS names, or with names of any bytes too.
func (g generator) zip() []sample {
	kinds := [][]int{{0, 1, 2}, {0, 1, 2, 3, 4}, {0, 1, 2, 3, 5}}[g.r.IntN(3)]
	out := make([]sample, 1+g.r.IntN(6))
	for i := range out {
		s := sample{flagged: g.r.IntN(5) == 0}
		if f := g.r.IntN(10); f < 4 {
			s.field = []int{validField, validField, otherCRC, version2}[f]
			s.named = g.pick(utf8Names())
		}
		k := kinds[g.r.IntN(len(kinds))]
		if s.flagged || s.field == validField {
			k = g.r.IntN(6)
		}
		s.raw = g.name(k)
		out[i] = s
	}
	return out
}

// written is samples as archive/zip writes them, read back.
func written(t *testing.T, samples []sample) []*zip.File {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i, s := range samples {
		h := &zip.FileHeader{Name: s.raw, Method: zip.Store, NonUTF8: !s.flagged}
		if s.flagged {
			h.Flags = 0x800
		}
		if s.field != noField {
			version, of := byte(1), s.raw
			switch s.field {
			case otherCRC:
				of += "x"
			case version2:
				version = 2
			}
			h.Extra = binary.LittleEndian.AppendUint16(nil, unicodePathID)
			h.Extra = binary.LittleEndian.AppendUint16(h.Extra, uint16(5+len(s.named))) //nolint:gosec // short
			h.Extra = binary.LittleEndian.AppendUint32(append(h.Extra, version), crc32.ChecksumIEEE([]byte(of)))
			h.Extra = append(h.Extra, s.named...)
		}
		if _, err := w.CreateHeader(h); err != nil {
			t.Fatalf("entry %d %q: %v", i, s.raw, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range z.File {
		if f.Name != samples[i].raw || f.Flags&0x800 != 0 != samples[i].flagged {
			t.Fatalf("entry %d reads %q, flags %x; wrote %q, flagged %v", i, f.Name, f.Flags, samples[i].raw, samples[i].flagged)
		}
	}
	return z.File
}

// tally counts the cases a mutant would show in.
type tally struct {
	zips, gbkZips, plainZips, unreadZips               int
	flaggedReadable, flaggedNotUTF8, plainUTF8, chance int
	unreadCandidates, otherFields, fieldsRead, asciiUp int
	gbkRead, fieldsOfSameNames                         int
}

// isASCII reports whether s's bytes are all ASCII: the test's own, not
// readNames' ascii.
func isASCII(s string) bool {
	for _, b := range []byte(s) {
		if b >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// check holds readNames' names of samples to the rule, counting.
func check(t *testing.T, samples []sample, c *tally) {
	t.Helper()
	got := readNames(written(t, samples))
	dec := simplifiedchinese.GBK.NewDecoder()
	reading := func(s string) (string, bool) {
		out, err := dec.String(s)
		return out, err == nil && !strings.ContainsRune(out, utf8.RuneError)
	}
	// Candidates, and whether GBK reads them all.
	candidates, unread := 0, 0
	for _, s := range samples {
		if !s.flagged && s.field != validField && !utf8.ValidString(s.raw) {
			candidates++
			if _, ok := reading(s.raw); !ok {
				unread++
			}
		}
	}
	gbkZip := candidates > 0 && unread == 0
	c.zips++
	switch {
	case gbkZip:
		c.gbkZips++
	case candidates == 0:
		c.plainZips++
	default:
		c.unreadZips++
		if unread < candidates {
			c.unreadCandidates++
		}
	}
	readAny := false
	for i, s := range samples {
		g := got[i]
		gbkReading, readable := reading(s.raw)
		bad := func(why string) {
			t.Errorf("entry %d %q (flagged %v, field %d %q) in a zip of %d candidates, %d unread: read %q from %q; %s", i, s.raw, s.flagged,
				s.field, s.named, candidates, unread, g.name, g.from, why)
		}
		if g.from == domain.NameFromGBK {
			readAny = true
			if g.name != gbkReading || !readable {
				bad("a GBK reading is the decoder's, with no U+FFFD")
			}
		}
		switch {
		case s.flagged:
			if g.name != s.raw || g.from != domain.NameAsWritten {
				bad("a flagged name is as written")
			}
			if gbkZip && !isASCII(s.raw) && readable {
				c.flaggedReadable++
				if !utf8.ValidString(s.raw) {
					c.flaggedNotUTF8++
				}
			}
		case s.field == validField:
			if want := (readName{name: s.named, from: domain.NameFromUnicodePath}); s.named == s.raw {
				c.fieldsOfSameNames++
				if g != (readName{name: s.raw}) {
					bad("a field of the name as written changes nothing")
				}
			} else if g != want {
				bad("a valid field's name")
			}
			if g.from == domain.NameFromUnicodePath {
				c.fieldsRead++
				if isASCII(s.raw) {
					c.asciiUp++
				}
			}
		case isASCII(s.raw):
			if g != (readName{name: s.raw}) {
				bad("an ASCII name changes by its field alone")
			}
		case !gbkZip:
			if g != (readName{name: s.raw}) {
				bad("in a zip not read as GBK, nothing changes but by a field")
			}
			if candidates == 0 && utf8.ValidString(s.raw) {
				c.plainUTF8++
			}
		case readable:
			if g != (readName{name: gbkReading, from: domain.NameFromGBK}) {
				bad("in a zip read as GBK, a name GBK reads is read so")
			}
			c.gbkRead++
			if utf8.ValidString(s.raw) {
				c.chance++
			}
		default:
			if g != (readName{name: s.raw}) {
				bad("a valid UTF-8 name GBK cannot read is as written")
			}
		}
		if s.field == otherCRC || s.field == version2 {
			c.otherFields++
		}
	}
	if !gbkZip && readAny {
		t.Errorf("a zip of %d candidates, %d unread, reads a name as GBK", candidates, unread)
	}
}

func TestReadNamesReadsAsGBKOnlyAZipOfGBKNames(t *testing.T) {
	var c tally
	for _, seed := range []uint64{1, 2, 3} {
		g := generator{r: rand.New(rand.NewPCG(seed, 0x5eed)), gbk: simplifiedchinese.GBK.NewEncoder(), //nolint:gosec // a test's
			sjis: japanese.ShiftJIS.NewEncoder()}
		for range 1000 {
			check(t, g.zip(), &c)
			if t.Failed() {
				t.Fatalf("seed %d", seed)
			}
		}
	}
	t.Logf("%+v", c)
	for _, floor := range []struct {
		name      string
		got, want int
	}{
		{"zips read as GBK", c.gbkZips, 604},
		{"zips of no candidate", c.plainZips, 1162},
		{"zips with a candidate GBK cannot read", c.unreadZips, 133},
		{"of them, beside a candidate it reads (S3-6)", c.unreadCandidates, 54},
		{"names flagged GBK reads, in zips read as GBK (S3-1)", c.flaggedReadable, 266},
		{"of them, not UTF-8 (S3-2)", c.flaggedNotUTF8, 142},
		{"names valid UTF-8 past ASCII, not flagged, in zips of no candidate (S3-3)", c.plainUTF8, 1430},
		{"names valid UTF-8 by chance read as GBK (S3-4)", c.chance, 465},
		{"fields of another CRC or version (S3-7)", c.otherFields, 1334},
		{"names read from their fields", c.fieldsRead, 1065},
		{"of them, ASCII", c.asciiUp, 164},
		{"fields of the name as written", c.fieldsOfSameNames, 21},
		{"names read as GBK", c.gbkRead, 1338},
	} {
		if floor.got < floor.want {
			t.Errorf("%s: %d, below its floor %d: the generator no longer reaches it", floor.name, floor.got, floor.want)
		}
	}
}
