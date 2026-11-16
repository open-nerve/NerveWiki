package archiveadapter_test

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"slices"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// encoded is s in enc's bytes.
func encoded(t *testing.T, enc encoding.Encoding, s string) string {
	t.Helper()
	out, err := enc.NewEncoder().String(s)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// pathField is an Info-ZIP Unicode Path extra field (APPNOTE 4.6.9) of
// version, the CRC-32 of of's bytes, and name.
func pathField(version byte, of, name string) []byte {
	b := binary.LittleEndian.AppendUint16(nil, 0x7075)
	b = binary.LittleEndian.AppendUint16(b, uint16(5+len(name))) //nolint:gosec // a test's short name
	b = append(b, version)
	b = binary.LittleEndian.AppendUint32(b, crc32.ChecksumIEEE([]byte(of)))
	return append(b, name...)
}

// header is an entry's header named by name's bytes, flagged UTF-8 or not
// (archive/zip flags a name valid UTF-8 past ASCII unless told NonUTF8),
// with the extra fields given; a name ending in "/" is a folder's.
func header(name string, flagged bool, extra ...[]byte) *zip.FileHeader {
	h := &zip.FileHeader{Name: name, Method: zip.Deflate, NonUTF8: !flagged, Extra: bytes.Join(extra, nil)}
	if flagged {
		h.Flags = 0x800
	}
	return h
}

// read is an entry as the import reads it: its name, its source, whether
// it is a folder.
func read(name string, from domain.NameSource, folder bool) string {
	return fmt.Sprintf("%q %q %v", name, from, folder)
}

// The import reads a name not flagged UTF-8 from its Unicode Path field,
// when valid, else, in a zip whose names neither flagged, so read nor
// UTF-8 all read as GBK, as GBK, valid UTF-8 or not (M12 design 4.6.6, T3;
// M12/P3 design 3.12.1, decision 376); a folder by the name so read. Each
// zip is archive/zip's, its flags checked first.
func TestTheImportReadsNamesNotInUTF8(t *testing.T) {
	gbk := func(s string) string { return encoded(t, simplifiedchinese.GBK, s) }
	const up, as, cn = domain.NameFromUnicodePath, domain.NameAsWritten, domain.NameFromGBK
	ziliao, ceshi := gbk("测试资料.md"), gbk("测试.md")
	pictures := []*zip.FileHeader{header(gbk("图片/"), false), header(gbk("图片/a.png"), false), header(gbk("图片/截图.png"), false),
		header(gbk("目录/"), false)}
	for _, tt := range []struct {
		name  string
		files []*zip.FileHeader
		want  []string
	}{
		{"1 a GBK name and its valid Unicode Path", []*zip.FileHeader{header(ziliao, false, pathField(1, ziliao, "測試資料.md"))},
			[]string{read("測試資料.md", up, false)}},
		{"2 a Unicode Path of another name's CRC", []*zip.FileHeader{header(ziliao, false, pathField(1, ziliao+"x", "測試資料.md"))},
			[]string{read("测试资料.md", cn, false)}},
		{"3 a Unicode Path of version 2", []*zip.FileHeader{header(ceshi, false, pathField(2, ceshi, "測試.md"))},
			[]string{read("测试.md", cn, false)}},
		{"4 ASCII and UTF-8 names and their Unicode Paths", []*zip.FileHeader{header("??.md", false, pathField(1, "??.md", "测试.md")),
			header("ü.md", false, pathField(1, "ü.md", "ü.md"))},
			[]string{read("测试.md", up, false), read("ü.md", as, false)}},
		{"5 GBK names", []*zip.FileHeader{header(ziliao, false), header(gbk("目录/文件.md"), false)},
			[]string{read("测试资料.md", cn, false), read("目录/文件.md", cn, false)}},
		{"6 GBK names, all but one valid UTF-8 by chance", pictures,
			[]string{read("图片/", cn, true), read("图片/a.png", cn, false), read("图片/截图.png", cn, false), read("目录/", cn, true)}},
		{"7 GBK names all valid UTF-8 by chance", pictures[:2], []string{read("ͼƬ/", as, true), read("ͼƬ/a.png", as, false)}},
		{"8 names flagged", []*zip.FileHeader{header(gbk("资料.md"), true), header("ü.md", true), header(ziliao, false)},
			[]string{read(gbk("资料.md"), as, false), read("ü.md", as, false), read("测试资料.md", cn, false)}},
		{"9 a name GBK cannot read", append(slices.Clone(pictures), header("f\xff.md", false)),
			[]string{read("ͼƬ/", as, true), read("ͼƬ/a.png", as, false), read(gbk("图片/截图.png"), as, false), read("Ŀ¼/", as, true),
				read("f\xff.md", as, false)}},
		{"10 a UTF-8 name not flagged, beside a GBK name", []*zip.FileHeader{header("ü.md", false), header(ziliao, false)},
			[]string{read("眉.md", cn, false), read("测试资料.md", cn, false)}},
		{"10 a UTF-8 name not flagged, alone", []*zip.FileHeader{header("ü.md", false)}, []string{read("ü.md", as, false)}},
		{"11 GBK names whose last byte is 0x5C", []*zip.FileHeader{header(gbk("乗.md"), false), header(gbk("资料/乗"), false),
			header(gbk("资料/"), false)},
			[]string{read("乗.md", cn, false), read("资料/乗", cn, false), read("资料/", cn, true)}},
		{"12 a Shift-JIS name", []*zip.FileHeader{header(encoded(t, japanese.ShiftJIS, "テスト資料.md"), false)},
			[]string{read("僥僗僩帒椏.md", cn, false)}},
		{"13 GB18030's four bytes", []*zip.FileHeader{header("\x94\x39\xfc\x36.md", false), header(ceshi, false)},
			[]string{read("\x94\x39\xfc\x36.md", as, false), read(ceshi, as, false)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files := make([]any, len(tt.files))
			for i, h := range tt.files {
				c := *h
				files[i] = &c
			}
			data := zipped(t, "", files...)
			z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			for i, f := range z.File {
				if flagged := f.Flags&0x800 != 0; flagged != (tt.files[i].Flags&0x800 != 0) {
					t.Fatalf("entry %d %q reads flagged %v, not as written", i, f.Name, flagged)
				}
			}
			a, store := newArchives(t)
			ar, err := opened(t, a, store, data, 20)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for i, e := range ar.Entries() {
				if e.Index != i || e.Method != tt.files[i].Method && !e.Folder {
					t.Errorf("entry %d = %+v", i, e)
				}
				got = append(got, read(e.Name, e.From, e.Folder))
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Entries() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}
