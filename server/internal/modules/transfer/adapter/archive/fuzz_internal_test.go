package archiveadapter

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
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
// (checkData), or archive/zip stops at a record it counted, one whose
// zip64 field is too short for what it declares (shortZip64).
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
		case errors.Is(err, errMiscounted) && !shortZip64(data, len(zr.File)):
			t.Fatalf("readImport refused (%v) what archive/zip reads, %d entries, record %d's zip64 fields whole",
				err, len(zr.File), len(zr.File))
		case !errors.Is(err, errMiscounted):
			t.Fatalf("readImport refused (%v) what archive/zip reads, %d entries", err, len(zr.File))
		}
	})
}

// shortZip64 reports whether the directory's record k, from 0, holds a
// zip64 field too short for its sizes and offset that it declares as
// 2³²-1, 8 bytes each: archive/zip reads each zip64 field so
// (readDirectoryHeader, Go 1.27), refuses the record, and stops there.
func shortZip64(data []byte, k int) bool {
	r := bytes.NewReader(data)
	d, err := readEnd(r, int64(len(data)))
	if err != nil {
		return false
	}
	in := bufio.NewReader(io.NewSectionReader(r, d.start, int64(len(data))-d.start))
	for range k {
		if _, ok := nextRecord(in); !ok {
			return false
		}
	}
	var head [directoryHeaderLen]byte
	if _, err := io.ReadFull(in, head[:]); err != nil || binary.LittleEndian.Uint32(head[:]) != directoryHeaderSignature {
		return false
	}
	if _, err := in.Discard(int(binary.LittleEndian.Uint16(head[28:]))); err != nil {
		return false
	}
	extra := make([]byte, binary.LittleEndian.Uint16(head[30:]))
	if _, err := io.ReadFull(in, extra); err != nil {
		return false
	}
	const most = 1<<32 - 1
	declared := 0 // the uncompressed size, the compressed size, the offset
	for _, at := range []int{24, 20, 42} {
		if binary.LittleEndian.Uint32(head[at:]) == most {
			declared++
		}
	}
	for len(extra) >= 4 {
		tag, size := binary.LittleEndian.Uint16(extra), int(binary.LittleEndian.Uint16(extra[2:]))
		if len(extra)-4 < size {
			break
		}
		if tag == 1 && size < 8*declared {
			return true
		}
		extra = extra[4+size:]
	}
	return false
}
