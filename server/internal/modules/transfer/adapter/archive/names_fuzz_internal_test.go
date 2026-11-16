package archiveadapter

import (
	"encoding/binary"
	"hash/crc32"
	"testing"
	"unicode/utf8"
)

// The Unicode Path field's fuzz test (M12/P3 design 3.12.3; v0.1 design
// 13.4, item 11): a plain go test runs its seeds; the mutations run by
// hand:
//
//	cd server && go test -run '^$' -fuzz '^FuzzUnicodePathReadsOnlyAValidField$' -fuzztime 10m ./internal/modules/transfer/adapter/archive

// unicodePathField is a Unicode Path extra field of version, the CRC-32 of
// of's bytes, and name.
func unicodePathField(version byte, of, name string) []byte {
	b := binary.LittleEndian.AppendUint16(nil, unicodePathID)
	b = binary.LittleEndian.AppendUint16(b, uint16(5+len(name))) //nolint:gosec // a seed's short name
	b = binary.LittleEndian.AppendUint32(append(b, version), crc32.ChecksumIEEE([]byte(of)))
	return append(b, name...)
}

// firstPathField is the payload of the first Unicode Path field among
// extra's fields, read field by field as APPNOTE 4.5 lays them, until one
// is cut short.
func firstPathField(extra []byte) ([]byte, bool) {
	for len(extra) >= 4 {
		id, size := binary.LittleEndian.Uint16(extra), int(binary.LittleEndian.Uint16(extra[2:]))
		if size > len(extra)-4 {
			return nil, false
		}
		if id == unicodePathID {
			return extra[4 : 4+size], true
		}
		extra = extra[4+size:]
	}
	return nil, false
}

// Whatever its extra fields and its name, an entry's Unicode Path reads
// without panicking, and reads a name when, and only when, its first
// field is of version 1, under the CRC of the name's bytes, and names a
// name valid UTF-8 and not empty: that name.
func FuzzUnicodePathReadsOnlyAValidField(f *testing.F) {
	name := "\xb2\xe2\xca\xd4.md" // 测试.md in GBK
	timestamp := []byte{0x55, 0x54, 5, 0, 1, 0, 0, 0, 0}
	for _, extra := range [][]byte{
		unicodePathField(1, name, "测试.md"),
		unicodePathField(1, name+"x", "测试.md"),
		unicodePathField(2, name, "测试.md"),
		unicodePathField(1, name, "测试.md")[:8],
		append(unicodePathField(1, name+"x", "其他.md"), unicodePathField(1, name, "测试.md")...),
		append(timestamp, unicodePathField(1, name, "测试.md")...),
		unicodePathField(1, name, ""),
		unicodePathField(1, name, "\xff.md"),
		nil,
	} {
		f.Add(extra, name)
	}
	f.Fuzz(func(t *testing.T, extra []byte, name string) {
		got, ok := unicodePath(extra, name)
		field, found := firstPathField(extra)
		valid := found && len(field) > 5 && field[0] == 1 && binary.LittleEndian.Uint32(field[1:]) == crc32.ChecksumIEEE([]byte(name)) &&
			utf8.Valid(field[5:])
		if ok != valid || ok && got != string(field[5:]) {
			t.Errorf("unicodePath(%x, %q) = %q, %v; want its first field's name, when valid (%v)", extra, name, got, ok, valid)
		}
	})
}
