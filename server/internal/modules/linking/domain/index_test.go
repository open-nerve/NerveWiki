package domain_test

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// whole is the first MaxIndexKey bytes of the title key of the whole of
// typ, cut at a character's boundary: what IndexKey must be, from its
// first IndexTypeChars characters alone.
func whole(typ string) string {
	key := shared.TitleKey(typ)
	if len(key) <= domain.MaxIndexKey {
		return key
	}
	end := domain.MaxIndexKey
	for !utf8.RuneStart(key[end]) {
		end--
	}
	return key[:end]
}

// A type's key is its title key's first 64 bytes, taken from its first 128
// characters alone (M9/P4 design 3.9.3; review B-N2): the longest types of
// each shape there, each's key the whole type's: three characters a
// composition makes two bytes (u U+0308 U+0304 is U+01D6), a letter and
// its marks, which NFC reorders and Go's stream-safe NFC breaks every 30,
// Hangul's jamo, which compose to syllables, and ß, which folds to ss.
func TestIndexKey(t *testing.T) {
	for _, tt := range []struct {
		name string
		typ  string
	}{
		{"compositions of three", strings.Repeat("u\u0308\u0304", 200)},
		{"capital compositions of three", strings.Repeat("U\u0308\u0304", 200)},
		{"a letter and its marks", "a" + strings.Repeat("\u0323\u0301", 300)},
		{"letters and marks", strings.Repeat("e\u0301\u0323", 100)},
		{"Hangul's jamo", strings.Repeat("\u1100\u1161\u11a8", 100)},
		{"sharp s", strings.Repeat("ß", 200)},
		{"capitals", strings.Repeat("Project/Active ", 30)},
		{"short", "project"},
		{"Chinese", strings.Repeat("项目", 100)},
	} {
		key := domain.IndexKey(tt.typ)
		if want := whole(tt.typ); key != want {
			t.Errorf("%s: IndexKey = %q (%d bytes), want the whole type's %q (%d bytes)", tt.name, key, len(key), want, len(want))
		}
		if len(key) > domain.MaxIndexKey || !utf8.ValidString(key) {
			t.Errorf("%s: IndexKey = %q, %d bytes, past %d or cut within a character", tt.name, key, len(key), domain.MaxIndexKey)
		}
	}
	// A key needs IndexTypeChars: one of the first 64 characters would
	// make the compositions' key short of the whole type's.
	typ := strings.Repeat("u\u0308\u0304", 200)
	short := shared.TitleKey(string([]rune(typ)[:64]))
	if short == whole(typ) {
		t.Errorf("the key of 64 characters %q is the whole's: the row tells nothing", short)
	}
}

// A keyer reused takes each type's own key, whatever came before: random
// types of letters that fold, marks NFC reorders and composes, jamo,
// sharp s and CJK, one keyer for all, each key the title key of its first
// 128 characters cut to 64 bytes. Its buffers are kept: a key of the
// costliest shape, a letter and 127 marks of four bytes, allocates its own
// bytes and about two more.
func TestAnIndexKeyerTakesEachKeyWithItsBuffers(t *testing.T) {
	alphabet := []rune("aAßΣσς\u0301\u0316\u0308\u0304uU\u1100\u1161\u11a8項 /\U0001E000\U0001D167İ")
	r := rand.New(rand.NewPCG(1, 2))
	k := domain.NewIndexKeyer()
	for range 2_000 {
		runes := make([]rune, r.IntN(300))
		for i := range runes {
			runes[i] = alphabet[r.IntN(len(alphabet))]
		}
		typ := string(runes)
		want := whole(string(runes[:min(len(runes), domain.IndexTypeChars)]))
		if got := k.Key(typ); got != want {
			t.Fatalf("Key(%q) = %q, want %q", typ, got, want)
		}
	}
	marks := strings.Repeat("\U0001E000\U0001D167", 64)
	worst := "A" + marks[:len(marks)-4]
	if n := testing.AllocsPerRun(100, func() { _ = k.Key(worst) }); n > 4 {
		t.Errorf("a key allocates %.1f times, want its own bytes and about two more", n)
	}
}

// A string is cut to MaxIndexString bytes at a character's boundary, and
// says so; one within them is as it is.
func TestCutIndexString(t *testing.T) {
	for _, tt := range []struct {
		s    string
		want string
		cut  bool
	}{
		{"plan", "plan", false},
		{strings.Repeat("a", 256), strings.Repeat("a", 256), false},
		{strings.Repeat("a", 257), strings.Repeat("a", 256), true},
		{"a" + strings.Repeat("項", 100), "a" + strings.Repeat("項", 85), true}, // 1 + 3 × 85 = 256
		{"ab" + strings.Repeat("項", 100), "ab" + strings.Repeat("項", 84), true},
	} {
		if got, cut := domain.CutIndexString(tt.s); got != tt.want || cut != tt.cut {
			t.Errorf("CutIndexString(%d bytes) = %d bytes, %v; want %d bytes, %v", len(tt.s), len(got), cut, len(tt.want), tt.cut)
		}
	}
}

// The index's order: the pages with a type first, by key, then by id;
// those without, by id.
func TestCompareIndex(t *testing.T) {
	a, b := uuid.MustParse("01920000-0000-7000-8000-00000000000a"), uuid.MustParse("01920000-0000-7000-8000-00000000000b")
	want := []domain.IndexCursor{
		{K: "", Typed: true, ID: b},
		{K: "area", Typed: true, ID: b},
		{K: "project", Typed: true, ID: a},
		{K: "project", Typed: true, ID: b},
		{Typed: false, ID: a},
		{Typed: false, ID: b},
	}
	got := slices.Clone(want)
	slices.Reverse(got)
	slices.SortFunc(got, domain.CompareIndex)
	if !slices.Equal(got, want) {
		t.Errorf("the order %+v, want %+v", got, want)
	}
}
