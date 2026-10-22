package domain

import (
	"cmp"
	"strings"
	"unicode/utf8"
	"uuid"

	"golang.org/x/text/cases"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The bounds of MCP's index (M9 design 4.9, 4.14; M9/P4 design 3.9.3).
const (
	// MaxIndexString is the most bytes of a type, a summary or a tag an
	// index's answer shows, cut at a character's boundary and marked.
	MaxIndexString = 256
	// MaxIndexTags is the most tags of a page an index's answer shows.
	MaxIndexTags = 20
	// MaxInbound is the most pages linking to a page an index counts.
	MaxInbound = 1000
	// MaxIndexType is the most bytes of index's type filter.
	MaxIndexType = 1024
	// MaxIndexKey is the most bytes of a type's key, the order's and the
	// cursor's; IndexTypeChars the characters of a type the key is taken
	// from: a key's byte takes at most 1.5 characters (u U+0308 U+0304 is
	// U+01D6, two bytes), and Go's NFC looks 30 further.
	MaxIndexKey    = 64
	IndexTypeChars = 128
	// IndexStartChars is the characters of a type's or a summary's string
	// the index reads, page_properties.value_start (00047, S2 review B-I1):
	// IndexTypeChars for the key, and MaxIndexString bytes and one more,
	// which tell a string cut from one whole.
	IndexStartChars = MaxIndexString + 1
)

// IndexCursor is the cursor after an index's item: its type's key and
// whether it had a type, its page's id. The pages with a type come first,
// by the key, then by id; those without, by id.
type IndexCursor struct {
	K     string    `json:"k"`
	Typed bool      `json:"t"`
	ID    uuid.UUID `json:"id"`
}

// CompareIndex is the index's order of the items a and b are the cursors
// of: negative when a comes first.
func CompareIndex(a, b IndexCursor) int {
	switch {
	case a.Typed != b.Typed && a.Typed:
		return -1
	case a.Typed != b.Typed:
		return 1
	}
	if c := cmp.Compare(a.K, b.K); c != 0 {
		return c
	}
	return a.ID.Compare(b.ID)
}

// IndexKey is a type's key, as an IndexKeyer takes it.
func IndexKey(typ string) string {
	return NewIndexKeyer().Key(typ)
}

// IndexKeyer takes types' keys one after another, its fold and its buffers
// kept from one to the next: a key's allocations are then about its own
// bytes, where TitleKey's three passes allocate anew each (a call of index
// takes 100,000 keys, M9/P4 design 3.9.3). It is not for concurrent use.
type IndexKeyer struct {
	fold             cases.Caser
	nfc, folded, key []byte
}

// NewIndexKeyer returns a keyer.
func NewIndexKeyer() *IndexKeyer {
	return &IndexKeyer{fold: cases.Fold()}
}

// Key is typ's key: the title key (shared.TitleKey: NFC, the full case
// fold, NFC again) of its first IndexTypeChars characters, cut to
// MaxIndexKey bytes at a character's boundary, so that a cursor is bounded
// (M9 design 4.9).
func (k *IndexKeyer) Key(typ string) string {
	n := 0
	for i := range typ {
		if n == IndexTypeChars {
			typ = typ[:i]
			break
		}
		n++
	}
	k.nfc = norm.NFC.AppendString(k.nfc[:0], typ)
	k.fold.Reset()
	var err error
	if k.folded, _, err = transform.Append(k.fold, k.folded[:0], k.nfc); err != nil {
		key, _ := cutAt(shared.TitleKey(typ), MaxIndexKey) // no fold of a whole text fails; were it to, TitleKey's
		return key
	}
	k.key = norm.NFC.Append(k.key[:0], k.folded...)
	end := min(len(k.key), MaxIndexKey)
	for end < len(k.key) && end > 0 && !utf8.RuneStart(k.key[end]) {
		end--
	}
	return string(k.key[:end])
}

// CutIndexString is s cut to MaxIndexString bytes at a character's
// boundary, and whether it was cut.
func CutIndexString(s string) (string, bool) {
	return cutAt(s, MaxIndexString)
}

// cutAt is s cut to most bytes at a character's boundary, and whether it
// was cut.
func cutAt(s string, most int) (string, bool) {
	if len(s) <= most {
		return s, false
	}
	end := most
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return strings.Clone(s[:end]), true
}
