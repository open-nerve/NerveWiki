package archiveadapter

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"
)

// The fuzz test (M7 closeout A-N6): a plain go test runs its seeds; the
// mutations run by hand:
//
//	cd server && go test -run '^$' -fuzz '^FuzzReadImportAgreesWithArchiveZip$' -fuzztime 10m ./internal/modules/transfer/adapter/archive

// seedZip adds a zip of the entries names, each its name as its data; with
// zip64, each declared past 4 GiB, without data.
func seedZip(f *testing.F, names []string, zip64 bool) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, n := range names {
		h := &zip.FileHeader{Name: n, Method: zip.Deflate}
		if zip64 {
			h.UncompressedSize64 = 1 << 32
		}
		fw, err := w.CreateHeader(h)
		if err != nil {
			f.Fatal(err)
		}
		if !zip64 {
			_, _ = fw.Write([]byte(n))
		}
	}
	_ = w.SetComment("c")
	if err := w.Close(); err != nil && !zip64 {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())
}

// readImport, its end's pre-read (zipdir.go) before archive/zip, never
// panics and reads a directory as archive/zip does: what it reads,
// archive/zip reads alike; what archive/zip reads within the most, it
// reads, unless the entries' data overlap or reach into the directory
// (checkData), or archive/zip stops at a record it counted.
func FuzzReadImportAgreesWithArchiveZip(f *testing.F) {
	seedZip(f, []string{"a.md", "b/", "b/c.png"}, false)
	seedZip(f, []string{"x"}, false)
	seedZip(f, []string{"big.bin"}, true)
	seedZip(f, nil, false)
	f.Add([]byte("PK\x05\x06" + string(make([]byte, 18))))
	const most = 50
	f.Fuzz(func(t *testing.T, data []byte) {
		zr, zerr := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		got, err := readImport(&counted{Reader: bytes.NewReader(data)}, most, MaxDirectory)
		zipOK := zerr == nil || errors.Is(zerr, zip.ErrInsecurePath)
		if err == nil {
			if !zipOK {
				t.Fatalf("readImport read what archive/zip refuses: %v", zerr)
			}
			if len(got.File) != len(zr.File) {
				t.Fatalf("readImport read %d entries, archive/zip %d", len(got.File), len(zr.File))
			}
			return
		}
		if zipOK && len(zr.File) <= most && !errors.Is(err, errOverlap) && !errors.Is(err, errMiscounted) {
			t.Fatalf("readImport refused (%v) what archive/zip reads, %d entries", err, len(zr.File))
		}
	})
}
