//go:build !race

package domain_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand/v2"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// replaceTimeBound is how long Replace may take on a shape: about ten
// times the slowest here, an NFC retry's (M9/P2 design 3.9; the times are
// in its section 9). A search of strings on a crafted shape takes seconds
// to minutes.
const replaceTimeBound = 2 * time.Second

// primeRK is the base of the Rabin–Karp hash of strings.Index past
// bytealg.MaxLen, which the crafted shapes are solved against: the
// toolchain's, which the guard reads first.
const primeRK = 16777619

// rkHash is s's Rabin–Karp hash, as internal/bytealg's HashStr has it.
func rkHash(s string) uint32 {
	var h uint32
	for i := range len(s) {
		h = h*primeRK + uint32(s[i])
	}
	return h
}

// rkPow is primeRK to the k.
func rkPow(k int) uint32 {
	p := uint32(1)
	for range k {
		p *= primeRK
	}
	return p
}

// collision is three bytes of a run of a's changed, 'a'+d1 at k1 bytes
// from its end, 'a'+d2 at k2 and 'a'+d3 at its end, which move the run's
// hash by a target: review B's crafted shape (M9/P2 design review B-I2,
// review-B/rk).
type collision struct{ k1, k2, d1, d2, d3 int }

// collide solves d1·b^k1 + d2·b^k2 + d3 = target mod 2³² for k1 > k2 > 0,
// neither in avoid, and d1, d2, d3 of -60 to 25 but 0, each 'a'+d a
// printable byte: b^k1·d1 for k1 up to 20,000 in a table, looked up for
// each k2 up to 200 and d2, d3.
func collide(t *testing.T, target uint32, avoid ...int) collision {
	t.Helper()
	var ds []int
	for d := -60; d <= 25; d++ {
		if d != 0 {
			ds = append(ds, d)
		}
	}
	type at struct{ k, d int }
	table := map[uint32]at{}
	b := uint32(1)
	for k := 1; k <= 20_000; k++ {
		b *= primeRK
		if slices.Contains(avoid, k) {
			continue
		}
		for _, d := range ds {
			table[b*uint32(int32(d))] = at{k, d}
		}
	}
	b = 1
	for k := 1; k <= 200; k++ {
		b *= primeRK
		if slices.Contains(avoid, k) {
			continue
		}
		for _, x := range ds {
			for _, y := range ds {
				if a, ok := table[target-(b*uint32(int32(x))+uint32(int32(y)))]; ok && a.k > k {
					return collision{a.k, k, a.d, x, y}
				}
			}
		}
	}
	t.Fatalf("no collision for %d", target)
	return collision{}
}

// on is the run of n a's with c's three bytes.
func (c collision) on(n int) []byte {
	run := []byte(strings.Repeat("a", n))
	run[n-1-c.k1], run[n-1-c.k2], run[n-1] = byte('a'+c.d1), byte('a'+c.d2), byte('a'+c.d3)
	return run
}

// craftedRK is review B's old_string of m bytes, a's but for three: its
// hash is m a's, so that strings.Index on a page of a's compares it at
// every window.
func craftedRK(t *testing.T, c collision, m int) string {
	t.Helper()
	old := string(c.on(m))
	if rkHash(old) != rkHash(strings.Repeat("a", m)) {
		t.Fatalf("the crafted old_string of %d bytes does not collide", m)
	}
	return old
}

// The CR's and the LF's places from the end of the CRLF form of
// craftedCRLF: their bytes less an a's are -84 and -87.
const crPlace, lfPlace = 5, 4

// craftedCRLF is an old_string of m bytes, a's but for one bare LF and
// three bytes, whose CRLF form, the one the CRLF rule looks for, has the
// hash of m+1 a's (review B's check, craftedCRLF of review-B/check/kmp).
func craftedCRLF(t *testing.T, c collision, m int) string {
	t.Helper()
	n := m + 1
	crlf := c.on(n)
	crlf[n-1-crPlace], crlf[n-1-lfPlace] = '\r', '\n'
	if rkHash(string(crlf)) != rkHash(strings.Repeat("a", n)) {
		t.Fatalf("the crafted old_string of %d bytes does not collide as CRLF", m)
	}
	return string(crlf[:n-1-crPlace]) + string(crlf[n-crPlace:])
}

// bytealgPrimeRK is primeRK as the toolchain's internal/bytealg declares
// it, read with go/parser from the GOROOT that go env names.
func bytealgPrimeRK(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatalf("go env GOROOT: %v", err)
	}
	path := filepath.Join(strings.TrimSpace(string(out)), "src", "internal", "bytealg", "bytealg.go")
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, s := range g.Specs {
			v := s.(*ast.ValueSpec)
			for i, name := range v.Names {
				if lit, ok := v.Values[min(i, len(v.Values)-1)].(*ast.BasicLit); ok && name.Name == "primeRK" {
					return lit.Value
				}
			}
		}
	}
	t.Fatalf("%s declares no primeRK", path)
	return ""
}

// cutAt is s cut to at most n bytes at a character's start.
func cutAt(s string, n int) string {
	for !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// outcome is Replace's answer without its content: the count and CRLF,
// or the refusal.
func outcome(r domain.Replacement, err error) string {
	if err != nil {
		return said(err)
	}
	return fmt.Sprintf("replaced %d, CRLF %v", r.Count, r.CRLF)
}

// replaceShape is a call of Replace on a costly shape, and its answer.
type replaceShape struct {
	name              string
	content, old, new string
	all               bool
	want              string
}

// replaceShapes are the shapes of M9/P2 design 3.9's table, at 5 MiB of
// content and old_strings of up to 1 MiB: runs that make KMP fall back,
// review B's crafted shapes, alone, at both ends of the page, as CRLF and
// in the NFC retry, every occurrence replaced, a size reckoned before
// it is built, and the NFC forms that grow, compose and reorder.
func replaceShapes(t *testing.T) []replaceShape {
	const MiB = 1 << 20
	a := strings.Repeat("a", 5*MiB)
	r := rand.New(rand.NewPCG(2, 3))
	b := make([]byte, 5*MiB)
	for i := range b {
		b[i] = "acgt"[r.IntN(4)]
	}
	acgt := string(b)
	rk := collide(t, 0)
	crlf := collide(t, 84*rkPow(crPlace)+87*rkPow(lfPlace), crPlace, lfPlace)
	acute := "\u0301"
	shapes := []replaceShape{
		{"(ab)*; old (ab)* and ac, 1 MiB", strings.Repeat("ab", 5*MiB/2), strings.Repeat("ab", MiB/2-1) + "ac", "x", false, notInPage},
		{"acgt at random; old of 64 bytes not in it", acgt, strings.Repeat("acgt", 15) + "aaaz", "x", false, notInPage},
		{"acgt at random; old its own 1 MiB", acgt, acgt[2*MiB : 3*MiB], "x", false, "replaced 1, CRLF false"},
		{"a*; old (a*1023 and b)*1024, the deep fallbacks", a, strings.Repeat(strings.Repeat("a", 1023)+"b", 1024), "x", false, notInPage},
		{"a*; old a, all 5,242,880", a, "a", "b", true, "replaced 5242880, CRLF false"},
		{"a*; old a, new 1 MiB, all: the size first", a, "a", strings.Repeat("b", MiB), true, pageTooLong},
	}
	for _, k := range []int{1, 100, 10_000, MiB - 1} {
		shapes = append(shapes, replaceShape{fmt.Sprintf("a*; old a*%d and b", k), a, strings.Repeat("a", k) + "b", "x", false, notInPage})
	}
	for _, m := range []int{120_000, MiB} {
		shapes = append(shapes, replaceShape{fmt.Sprintf("a*; review B's crafted old of %d bytes", m), a, craftedRK(t, rk, m), "x", false, notInPage})
	}
	for _, m := range []int{30_000, 120_000, MiB} {
		old := craftedRK(t, rk, m)
		twice := old + a[:5*MiB-2*m] + old
		shapes = append(shapes,
			replaceShape{fmt.Sprintf("the crafted old of %d bytes at both ends", m), twice, old, "x", false, foundTimes(2)},
			replaceShape{fmt.Sprintf("the crafted old of %d bytes at both ends, all", m), twice, old, "x", true, "replaced 2, CRLF false"})
	}
	x := strings.Repeat("x\r\n", 5*MiB/3)
	nfd := strings.Repeat("e"+acute, 5*MiB/3)
	note := strings.Repeat("\U0001d160", 5*MiB/4)
	jamo := strings.Repeat("\u1100\u1161\u11a8", 5*MiB/9)
	marks := strings.Repeat("a"+strings.Repeat(acute+"\u0316", 14), 5*MiB/57)
	shapes = append(shapes,
		replaceShape{"the NFC retry: a* and e U+0301; the crafted old of 120,000 bytes", a[:5*MiB-3] + "e" + acute,
			craftedRK(t, rk, 120_000), "x", false, notInPage},
		replaceShape{"the CRLF rule: CRLF and a*; old crafted as CRLF, 120,000 bytes", "\r\n" + a[:5*MiB-2],
			craftedCRLF(t, crlf, 120_000), "x\n", false, notInPage},
		replaceShape{"(x CRLF)*; old of bare LFs, found as CRLF", x, "x\nx\nx\n", "y\n", false,
			foundTimes(strings.Count(x, "x\r\nx\r\nx\r\n"))},
		replaceShape{"(x CRLF)*; old of 1 MiB of bare LFs not in it", x, strings.Repeat("x\n", MiB/2-1) + "z\n", "y", false, notInPage},
		replaceShape{"(e U+0301)*, not NFC; old not in it", nfd, strings.Repeat("e"+acute, 1000) + "z", "x", false, notInPage},
		replaceShape{"(U+1D160)*, NFC three times the bytes; old of 1 MiB not in it", note, cutAt(note, MiB-8) + "z", "x", false, notInPage},
		replaceShape{"conjoining jamo, which NFC composes; old of 1 MiB not in it", jamo, cutAt(jamo, MiB-8) + "z", "x", false, notInPage},
		replaceShape{"marks out of order, which NFC reorders; old of 1 MiB not in it", marks, cutAt(marks, MiB-8) + "z", "x", false, notInPage},
		replaceShape{"the CRLF rule and the NFC retry: (e U+0301 CRLF)*; old of 1 MiB of bare LFs not in it", strings.Repeat("e"+acute+"\r\n", 5*MiB/5),
			strings.Repeat("e"+acute+"\n", MiB/4-1) + "e" + acute + "z", "y", false, notInPage},
	)
	return shapes
}

// Replace keeps within its time on every costly shape (M9/P2 design 3.9,
// 13.4 item 8): its one matcher is linear in the content and old_string,
// whatever their bytes, in finding, the second occurrence, counting and
// replacing, as CRLF, and in the NFC retry. Review B's crafted shapes
// make each hash of strings.Index's Rabin–Karp collide: a search there by
// strings takes seconds to minutes. They are solved here against
// primeRK, which the toolchain's internal/bytealg is read for first: one
// with another base would make them plain shapes. Each shape's time is
// logged, for CI's log under -v. -short skips it, as it does the globs'
// guard: make test-go-costs runs it alone.
func TestReplaceStaysWithinItsTime(t *testing.T) {
	if testing.Short() {
		t.Skip("a bound in time, measured alone by make test-go-costs")
	}
	if got := bytealgPrimeRK(t); got != strconv.Itoa(primeRK) {
		t.Fatalf("internal/bytealg's primeRK is %s, not %d: solve the crafted shapes again for it", got, primeRK)
	}
	var slowest time.Duration
	var slowestOf string
	for _, s := range replaceShapes(t) {
		start := time.Now()
		r, err := domain.Replace(domain.Args{}, s.content, s.old, s.new, s.all)
		took := time.Since(start)
		t.Logf("%s: %v, %s", s.name, took, outcome(r, err))
		if took > slowest {
			slowest, slowestOf = took, s.name
		}
		if got := outcome(r, err); got != s.want {
			t.Errorf("%s: Replace = %s, want %s", s.name, got, s.want)
		}
		if took > replaceTimeBound {
			t.Errorf("%s: Replace took %v, past %v", s.name, took, replaceTimeBound)
		}
	}
	t.Logf("the slowest %v of %v, %s", slowest, replaceTimeBound, slowestOf)
}
