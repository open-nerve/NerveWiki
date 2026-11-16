package bootstrap

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"net/http"
	"slices"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// unflagged is a vault's file, its name's bytes as given and not flagged
// UTF-8, with its extra fields.
type unflagged struct {
	name, data string
	extra      []byte
}

// unflaggedZip zips files, deflated, in their order, none flagged UTF-8:
// archive/zip flags a name valid UTF-8 past ASCII unless told NonUTF8.
func unflaggedZip(t *testing.T, files ...unflagged) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		fw, err := w.CreateHeader(&zip.FileHeader{Name: f.name, Method: zip.Deflate, NonUTF8: true, Extra: f.extra})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(f.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A zip of names not flagged UTF-8, as a Chinese Windows writes them,
// imports through serve with its names read (M12 design 4.6.6, T3; M12/P3
// design 3.12): GBK names, one of them valid UTF-8 by chance (图片/a.md,
// "ͼƬ/a.md"), as GBK, in one folder 图片 (decision 376), and a name "??.md"
// from its Info-ZIP Unicode Path field; the report lists each, in the
// archive's order, name_gbk and name_unicode_path, no skip, no to.
func TestAnImportReadsNamesNotInUTF8(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	gbk := func(s string) string {
		out, err := simplifiedchinese.GBK.NewEncoder().String(s)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	field := binary.LittleEndian.AppendUint16(nil, 0x7075)
	field = binary.LittleEndian.AppendUint16(field, uint16(5+len("说明.md")))
	field = binary.LittleEndian.AppendUint32(append(field, 1), crc32.ChecksumIEEE([]byte("??.md")))
	field = append(field, "说明.md"...)
	archive := unflaggedZip(t, unflagged{gbk("图片/截图.md"), "shot", nil}, unflagged{gbk("图片/a.md"), "a", nil},
		unflagged{gbk("测试资料.md"), "data", nil}, unflagged{"??.md", "readme", field})

	j := tm.imported(t, "alice", nb, "", archive)

	if j.State != "succeeded" || j.Report == nil || j.Report.Counts["pages"] != 5 || j.Report.Counts["skipped"] != 0 {
		t.Fatalf("the import = %+v, report %s; want succeeded, 5 pages, none skipped", j, describe(j.Report))
	}
	tree := queryStrings(t, tm.pool, `SELECT coalesce(p.name, '-') || '/' || n.name || ' ' || n.kind || ' ' || coalesce(c.content, '')
		FROM nodes n LEFT JOIN nodes p ON p.id = n.parent_id LEFT JOIN page_contents c ON c.node_id = n.id
		WHERE n.notebook_id = $1 AND n.deleted_at IS NULL`, nb)
	want := []string{"-/测试资料 page data", "-/说明 page readme", "-/图片 page ", "图片/a page a", "图片/截图 page shot"}
	slices.Sort(tree)
	slices.Sort(want)
	if !slices.Equal(tree, want) {
		t.Errorf("the tree = %q, want %q", tree, want)
	}
	status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/transfer-jobs/"+j.ID, tm.tokens["alice"], "")
	var detail struct {
		Problems []struct {
			Path string  `json:"path"`
			Code string  `json:"code"`
			To   *string `json:"to"`
		} `json:"problems"`
	}
	decodeAnswer(t, answer, &detail)
	var problems []string
	for _, p := range detail.Problems {
		line := p.Code + " " + p.Path
		if p.To != nil {
			line += " to " + *p.To
		}
		problems = append(problems, line)
	}
	if want := []string{"name_gbk 图片/截图.md", "name_gbk 图片/a.md", "name_gbk 测试资料.md", "name_unicode_path 说明.md"}; status != http.StatusOK ||
		!slices.Equal(problems, want) {
		t.Errorf("the report's problems = %d %q, want %q", status, problems, want)
	}
	checkPages(t, tm.pool)
}
