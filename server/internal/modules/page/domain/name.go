package domain

import (
	"encoding/hex"
	"strconv"
	"strings"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// An attachment's name (M7/P2 design 3.3): a title, which an export writes
// as the file's name, that never reads as a page's file, nor loses the
// extension that tells its type.

// CheckAssetName checks s as field, an attachment's name: CheckTitle's
// rules, and no ".md" at its end, in any case, which an export would read
// back as a page. A name that breaks a rule is 422 on field.
func CheckAssetName(field, s string) (Title, error) {
	title, err := CheckTitle(field, s)
	if err != nil {
		return Title{}, err
	}
	if n := title.Name; len(n) >= len(".md") && strings.EqualFold(n[len(n)-len(".md"):], ".md") {
		return Title{}, NotAllowed(field, `An attachment's name must not end with ".md", which names a page's file.`)
	}
	return title, nil
}

// CheckAssetRename checks s as field, the new name of the attachment
// named old: CheckAssetName's rules, and an extension when old has one,
// which may change.
func CheckAssetRename(field, old, s string) (Title, error) {
	title, err := CheckAssetName(field, s)
	if err != nil {
		return Title{}, err
	}
	if hasExtension(old) && !hasExtension(title.Name) {
		return Title{}, NotAllowed(field, "An attachment's new name must keep an extension: its type is told by it.")
	}
	return title, nil
}

// hasExtension reports whether name ends with an extension: a dot that
// does not start it, and a character after it.
func hasExtension(name string) bool {
	i := strings.LastIndexByte(name, '.')
	return i > 0 && i < len(name)-1
}

// Numbered is name with the number n, as an import names a node whose
// name a sibling holds (M7/P6 design 3.4): "name n" for a page, and for an
// attachment with an extension the number before it, "stem n.ext"; the
// name, or its stem, cut at a character so that the whole fits a title's
// bytes. An extension too long to leave room for a character of the stem,
// its first, is numbered as a page's name is: a name of no stem, " n.ext",
// would start with a space, no title (M9/P3 S2 fix round 1, A-N1).
func Numbered(name string, n int, asset bool) string {
	return numbered(name, strconv.Itoa(n), asset)
}

// Family is what the names Numbered numbers name with, the numbers of n's
// digits, have in common: the name numbered with '#' for each digit, a
// character no title holds. Their keys are those of one name and of one
// extension (or none) cut alike, numbered.
func Family(name string, n int, asset bool) string {
	return numbered(name, strings.Repeat("#", len(strconv.Itoa(n))), asset)
}

// numbered is Numbered with the number written number.
func numbered(name, number string, asset bool) string {
	if asset && hasExtension(name) {
		i := strings.LastIndexByte(name, '.')
		if stem := cut(name[:i], shared.MaxTitleBytes-len(" "+number+name[i:])); stem != "" {
			return stem + " " + number + name[i:]
		}
	}
	return cut(name, shared.MaxTitleBytes-len(" "+number)) + " " + number
}

// cut is s cut at a character so that it is at most room bytes long:
// none of it when room is not one.
func cut(s string, room int) string {
	if len(s) <= room {
		return s
	}
	room = max(room, 0)
	for room > 0 && !utf8.RuneStart(s[room]) {
		room--
	}
	return s[:room]
}

// AsideName is the name a revert moves a node aside with for a while (M9
// design 4.12; M9/P3 design 3.17): name, the node's as the revert's plan
// began, with " ~" and the last 8 hex digits of id, which a UUIDv7 draws
// at random (its first ones are its time, those of a batch's nodes
// alike); else all 32; else all 32 and "-2", "-3" and on: the first whose
// key taken says is free. Built as Numbered builds a name, "~" and the
// digits for the number: name cut at a UTF-8 boundary so that the whole
// is at most shared.MaxTitleBytes, an attachment's extension kept after
// the digits ("photo ~1a2b3c4d.png"), one too long for that numbered as a
// page's name is.
func AsideName(name string, id uuid.UUID, asset bool, taken func(key string) bool) string {
	digits := hex.EncodeToString(id[:])
	for n := 0; ; n++ {
		mark := digits
		switch {
		case n == 0:
			mark = digits[len(digits)-8:]
		case n > 1:
			mark += "-" + strconv.Itoa(n)
		}
		if aside := numbered(name, "~"+mark, asset); !taken(shared.TitleKey(aside)) {
			return aside
		}
	}
}
