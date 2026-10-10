package archiveadapter

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
)

// The fuzz test (M7 closeout A-N6): a plain go test runs its seeds; the
// mutations run by hand:
//
//	cd server && go test -run '^$' -fuzz '^FuzzReadImportAgreesWithArchiveZip$' -fuzztime 10m ./internal/modules/transfer/adapter/archive

// seedZip adds a zip of the entries names, each its name as its data and
// a comment; with zip64, each declared past 4 GiB, without data.
func seedZip(f *testing.F, names []string, zip64 bool) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, n := range names {
		h := &zip.FileHeader{Name: n, Method: zip.Deflate, Comment: "on " + n}
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
// (checkData), or the end says more records than the most, or a larger
// directory (endPastBounds).
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
		switch {
		case !zipOK || len(zr.File) > most || errors.Is(err, errOverlap):
		case errors.Is(err, app.ErrTooManyEntries) && endPastBounds(data, most):
		default:
			t.Fatalf("readImport refused (%v) what archive/zip reads, %d entries", err, len(zr.File))
		}
	})
}

// endPastBounds reports whether data's end says more records than most,
// or a directory larger than MaxDirectory: an import refuses it before it
// reads further (M7/P6 design 3.10), though archive/zip, comparing 16
// bits of the count, may read the records there are.
func endPastBounds(data []byte, most int) bool {
	d, err := readEnd(bytes.NewReader(data), int64(len(data)))
	return err == nil && (d.records > uint64(most) || d.size > MaxDirectory)
}
