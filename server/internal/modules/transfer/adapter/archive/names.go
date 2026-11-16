package archiveadapter

import (
	"archive/zip"
	"encoding/binary"
	"hash/crc32"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// The names of an import's entries as it reads them (M12 design 4.6.6,
// T3; M12/P3 design 3.12.1). archive/zip gives a name's bytes as its
// header holds them, and does not read the Info-ZIP Unicode Path field;
// the import's domain judges a name read here, and lists in its report
// each one read from elsewhere than its header's bytes. The rule wants the
// flags, the extra fields, a CRC and a GBK decoder, an adapter's: the pure
// layers may not import golang.org/x/text/encoding.

// utf8Flag is the general purpose flag's bit 11: the name and comment are
// UTF-8 (APPNOTE 4.4.4). The flag is read, not archive/zip's NonUTF8,
// which a name not UTF-8 sets whatever the flag says.
const utf8Flag = 0x800

// unicodePathID is the Info-ZIP Unicode Path extra field's (APPNOTE 4.6.9).
const unicodePathID = 0x7075

// unicodePath is the name the Unicode Path field in extra gives an entry
// whose header's name is name: version 1, the CRC-32 of name, then the
// name in UTF-8. False when there is none, or the first is of another
// version, or its CRC is not name's (a tool renamed the entry and left the
// field), or its name is empty or not UTF-8, or the fields are cut short.
// The first such field decides: a later one is no better.
func unicodePath(extra []byte, name string) (string, bool) {
	for len(extra) >= 4 {
		id, size := binary.LittleEndian.Uint16(extra), int(binary.LittleEndian.Uint16(extra[2:]))
		if size > len(extra)-4 {
			return "", false
		}
		field := extra[4 : 4+size]
		extra = extra[4+size:]
		if id != unicodePathID {
			continue
		}
		if len(field) < 5 || field[0] != 1 || binary.LittleEndian.Uint32(field[1:]) != crc32.ChecksumIEEE([]byte(name)) {
			return "", false
		}
		read := string(field[5:])
		return read, read != "" && utf8.ValidString(read)
	}
	return "", false
}

// readName is an entry's name as the import reads it, and its source.
type readName struct {
	name string
	from domain.NameSource
}

// readNames are the names of files as the import reads them, and their
// sources: one flagged UTF-8 as written (Classify skips one that is not);
// else its Unicode Path field's, when valid; else, in a zip whose names
// neither flagged, read so nor in UTF-8 (the candidates) are one or more,
// and all read as GBK with no U+FFFD, its GBK reading, when it has bytes
// past ASCII and a reading with no U+FFFD: a GBK name may be valid UTF-8
// by chance, 目录 is Ŀ¼ (M12/P3 design 3.12.1, decision 376); else as
// written. A zip with no candidate has no name read otherwise but by its
// Unicode Path fields, and one of whose candidates GBK cannot read reads
// none as GBK. golang.org/x/text's GBK decoder never fails, writing
// U+FFFD for the bytes it cannot read: "read as GBK" can only mean that.
func readNames(files []*zip.File) []readName {
	out := make([]readName, len(files))
	gbk := simplifiedchinese.GBK.NewDecoder()
	readings := make([]string, len(files))
	candidates, unread := 0, false
	for i, f := range files {
		out[i] = readName{name: f.Name}
		if f.Flags&utf8Flag != 0 {
			continue
		}
		if name, ok := unicodePath(f.Extra, f.Name); ok {
			if name != f.Name {
				out[i].name, out[i].from = name, domain.NameFromUnicodePath
			}
			continue
		}
		if ascii(f.Name) {
			continue
		}
		reading, err := gbk.String(f.Name)
		read := err == nil && !strings.ContainsRune(reading, utf8.RuneError)
		if !utf8.ValidString(f.Name) {
			candidates++
			unread = unread || !read
		}
		if read {
			readings[i] = reading
		}
	}
	if candidates == 0 || unread {
		return out
	}
	for i, reading := range readings {
		if reading != "" {
			out[i].name, out[i].from = reading, domain.NameFromGBK
		}
	}
	return out
}

// ascii reports whether s is ASCII alone.
func ascii(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
