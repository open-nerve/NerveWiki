package postgresadapter_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/unicode/rangetable"
)

// nfcGaps are the code points whose NFC Go's x/text and PostgreSQL write
// otherwise, by the pair of their Unicode versions, PostgreSQL's
// (unicode_version()) and x/text's (norm.Version), as ranges: reviewed and
// recorded. A term Go's NFC changes is matched in contents by
// PostgreSQL's NFC of it; one Go's NFC leaves as it is, typed in Go's
// order, misses a content typed otherwise (M8 Codex fix check 1, C1-1).
//
//nolint:gochecknoglobals // a table the test reads
var nfcGaps = map[string][][2]rune{
	// The 34 combining marks Unicode 17 added: x/text orders them by their
	// classes, PostgreSQL 18, of Unicode 16, takes them for starters.
	"16.0 17.0.0": {
		{0x1ACF, 0x1ADD}, {0x1AE0, 0x1AEB}, {0x10EFA, 0x10EFB}, {0x1E6E3, 0x1E6E3}, {0x1E6E6, 0x1E6E6},
		{0x1E6EE, 0x1E6EF}, {0x1E6F5, 0x1E6F5},
	},
}

// nfcProbe is a text whose NFC is compared, and the code point it is of.
type nfcProbe struct {
	r    rune
	text string
}

// pgNormalized is texts in form, NFC or NFD, as PostgreSQL puts them, in
// their order, in batches.
func (f fixture) pgNormalized(t *testing.T, form string, texts []string) []string {
	t.Helper()
	if form != "NFC" && form != "NFD" {
		t.Fatalf("no form %s", form)
	}
	const batch = 100_000
	out := make([]string, 0, len(texts))
	for from := 0; from < len(texts); from += batch {
		rows, err := f.pool.Query(context.Background(),
			`SELECT normalize(x.t, `+form+`) FROM unnest($1::text[]) WITH ORDINALITY AS x(t, i) ORDER BY x.i`,
			texts[from:min(from+batch, len(texts))])
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			out = append(out, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}
	if len(out) != len(texts) {
		t.Fatalf("%d texts in %s of %d", len(out), form, len(texts))
	}
	return out
}

// nfcDiffers is the code points of probes whose NFC by Go is not
// PostgreSQL's, each with the first text that shows it.
func (f fixture) nfcDiffers(t *testing.T, probes []nfcProbe) map[rune]string {
	t.Helper()
	texts := make([]string, len(probes))
	for i, p := range probes {
		texts[i] = p.text
	}
	pg := f.pgNormalized(t, "NFC", texts)
	out := map[rune]string{}
	for i, p := range probes {
		if goNFC := norm.NFC.String(p.text); goNFC != pg[i] {
			if _, ok := out[p.r]; !ok {
				out[p.r] = fmt.Sprintf("%U: Go %U, PostgreSQL %U", []rune(p.text), []rune(goNFC), []rune(pg[i]))
			}
		}
	}
	return out
}

// The search keys a text and parses a term by Go's NFC, and stores names in
// it, and reads a content in PostgreSQL's (M8 Codex review R2; fix check
// 1, C1-1). Their NFCs differ for a code point whose class or
// decomposition their Unicode versions tell otherwise, beyond the CGJ Go
// inserts in a run of more than 30 non-starters, which no text here
// holds. Every code point whose NFC may differ is put in NFC by both,
// alone, after a starter, among marks of classes 230 and 1, and
// decomposed: each Go knows as a non-starter, as decomposing, or as
// combining with the character before it, the primary composites and the
// Hangul syllables among them, decomposed by Go; and each no Unicode
// version Go knows assigns, which a newer PostgreSQL may, decomposed by
// PostgreSQL too, alone and after a starter: a composite only the newer
// Unicode has, which PostgreSQL composes and Go does not (M8 Codex fix
// check 2, C2-2). Each non-starter is then put after a mark of each
// class. Left out: the code points of planes 4 to
// 13, which no Unicode version assigns yet, private use and surrogates,
// which never decompose or combine, and the characters Go knows as plain
// starters, which keep that in every version (Unicode's stability
// policy). The code points that differ must be those recorded for the two
// versions (nfcGaps): a library or image upgrade that opens a gap, or
// closes one, fails here, naming them and the record to change.
func TestGoAndPostgreSQLWriteNFCAlikeButForTheGapsRecorded(t *testing.T) {
	f := newFixture(t)
	started := time.Now()
	var pgVersion string
	if err := f.pool.QueryRow(context.Background(), `SELECT unicode_version()`).Scan(&pgVersion); err != nil {
		t.Fatal(err)
	}
	assigned := rangetable.Assigned(norm.Version)
	if assigned == nil {
		t.Fatalf("x/text has no table of the code points Unicode %s assigns", norm.Version)
	}
	acute, overlay := string(rune(0x0301)), string(rune(0x0334))
	var probes []nfcProbe
	var marks []rune   // the non-starters Go knows
	var unknown []rune // the code points no Unicode version Go knows assigns
	points := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		switch plane := r >> 16; {
		case 0xD800 <= r && r <= 0xDFFF, 0xE000 <= r && r <= 0xF8FF, plane >= 4 && plane <= 13, plane >= 15:
			continue
		}
		s := string(r)
		p, nfd, known := norm.NFC.PropertiesString(s), norm.NFD.String(s), unicode.Is(assigned, r)
		if known && p.CCC() == 0 && p.LeadCCC() == 0 && p.TrailCCC() == 0 && p.BoundaryBefore() && nfd == s {
			continue
		}
		points++
		probes = append(probes, nfcProbe{r, s}, nfcProbe{r, "a" + s}, nfcProbe{r, "a" + acute + s + overlay})
		if nfd != s {
			probes = append(probes, nfcProbe{r, nfd})
		}
		if known && p.CCC() > 0 {
			marks = append(marks, r)
		}
		if !known {
			unknown = append(unknown, r)
		}
	}
	texts := make([]string, len(unknown))
	for i, r := range unknown {
		texts[i] = string(r)
	}
	for i, nfd := range f.pgNormalized(t, "NFD", texts) {
		if nfd != texts[i] {
			probes = append(probes, nfcProbe{unknown[i], nfd}, nfcProbe{unknown[i], "a" + nfd})
		}
	}
	differ := f.nfcDiffers(t, probes)
	// Each class's first mark both tell alike, after each mark both tell
	// alike.
	byClass := map[uint8]rune{}
	for _, r := range marks {
		ccc := norm.NFC.PropertiesString(string(r)).CCC()
		if _, ok := differ[r]; !ok && byClass[ccc] == 0 {
			byClass[ccc] = r
		}
	}
	var after []nfcProbe
	for _, r := range marks {
		if _, ok := differ[r]; ok {
			continue
		}
		for _, m := range byClass {
			if m != r {
				after = append(after, nfcProbe{r, "a" + string(m) + string(r)})
			}
		}
	}
	maps.Copy(differ, f.nfcDiffers(t, after))
	t.Logf("PostgreSQL's Unicode %s, x/text's %s: %d code points (%d unknown to x/text), %d texts, %d marks after %d classes, "+
		"%d differ, in %v", pgVersion, norm.Version, points, len(unknown), len(probes)+len(after), len(marks), len(byClass), len(differ),
		time.Since(started))

	key := pgVersion + " " + norm.Version
	recorded := map[rune]bool{}
	for _, g := range nfcGaps[key] {
		for r := g[0]; r <= g[1]; r++ {
			recorded[r] = true
		}
	}
	var opened, closed []rune
	for r := range differ {
		if !recorded[r] {
			opened = append(opened, r)
		}
	}
	for r := range recorded {
		if _, ok := differ[r]; !ok {
			closed = append(closed, r)
		}
	}
	slices.Sort(opened)
	slices.Sort(closed)
	if len(opened) > 0 {
		var shown []string
		for _, r := range opened[:min(len(opened), 10)] {
			shown = append(shown, differ[r])
		}
		t.Errorf("PostgreSQL's Unicode %s and x/text's %s put %d code points in NFC otherwise, none recorded: %s\n%s\n"+
			"A term Go's NFC changes takes PostgreSQL's NFC in contents; one Go's NFC leaves as it is does not. Review the gap: "+
			"check that a text using them, typed in either order and decomposed, finds itself, queued and built "+
			"(TestTheSearchFindsATermGosNFCChangesAsWritten); then record them in nfcGaps[%q]",
			pgVersion, norm.Version, len(opened), ranges(opened), strings.Join(shown, "\n"), key)
	}
	if len(closed) > 0 {
		t.Errorf("PostgreSQL's Unicode %s and x/text's %s put %d code points recorded in nfcGaps[%q] in NFC alike now: %s; take them out",
			pgVersion, norm.Version, len(closed), key, ranges(closed))
	}
}

// ranges is rs, sorted, as ranges of code points.
func ranges(rs []rune) string {
	var out []string
	for i := 0; i < len(rs); {
		j := i
		for j+1 < len(rs) && rs[j+1] == rs[j]+1 {
			j++
		}
		if j == i {
			out = append(out, fmt.Sprintf("%U", rs[i]))
		} else {
			out = append(out, fmt.Sprintf("%U–%U", rs[i], rs[j]))
		}
		i = j + 1
	}
	return strings.Join(out, ", ")
}
