package archiveadapter

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
)

// bounded lets left bytes be read, a read past them failing whole, until
// lifted.
func TestBoundedReadsNoMoreThanCounted(t *testing.T) {
	b := &bounded{r: bytes.NewReader(make([]byte, 100)), left: 10}
	if n, err := b.ReadAt(make([]byte, 6), 0); n != 6 || err != nil {
		t.Fatalf("ReadAt(6) = %d, %v", n, err)
	}
	if _, err := b.ReadAt(make([]byte, 5), 6); !errors.Is(err, errBounded) {
		t.Errorf("ReadAt(5) past the bound = %v, want errBounded", err)
	}
	if n, err := b.ReadAt(make([]byte, 4), 6); n != 4 || err != nil {
		t.Errorf("ReadAt(4) up to the bound = %d, %v", n, err)
	}
	b.lift()
	if n, err := b.ReadAt(make([]byte, 50), 10); n != 50 || err != nil {
		t.Errorf("ReadAt(50) lifted = %d, %v", n, err)
	}
}

// counted is an archive in memory that counts the bytes read of it.
type counted struct {
	*bytes.Reader
	read int64
}

func (c *counted) ReadAt(p []byte, off int64) (int, error) {
	n, err := c.Reader.ReadAt(p, off)
	c.read += int64(n)
	return n, err
}

func (c *counted) Close() error       { return nil }
func (c *counted) ModTime() time.Time { return time.Time{} }

// An archive of 70,000 records whose end counts 4,464, 70,000's low 16
// bits, as archive/zip compares them: the pre-count stops past the most
// read, and archive/zip reads none of its records, the bytes read fewer
// than the directory holds.
func TestADirectoryPastTheMostIsNotRead(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := range 70000 {
		if _, err := w.CreateHeader(&zip.FileHeader{Name: fmt.Sprintf("f%05d.md", i), Method: zip.Store}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	end := bytes.LastIndex(data, []byte{0x50, 0x4b, 0x05, 0x06})
	// archive/zip wrote a zip64 end for 70,000 records; this end says 4,464 and needs none.
	binary.LittleEndian.PutUint16(data[end+8:], 70000&0xffff)
	binary.LittleEndian.PutUint16(data[end+10:], 70000&0xffff)
	directory := int64(binary.LittleEndian.Uint32(data[end+12:]))
	if directory == 0xffffffff {
		t.Fatal("the directory's size is in the zip64 end")
	}
	f := &counted{Reader: bytes.NewReader(data)}
	if _, err := readImport(f, 50000, MaxDirectory); !errors.Is(err, app.ErrTooManyEntries) {
		t.Fatalf("readImport() = %v, want ErrTooManyEntries", err)
	}
	if f.read >= directory {
		t.Errorf("%d bytes read, want fewer than the directory's %d: archive/zip read it", f.read, directory)
	}
}

// dirRecord is a directory's record of name: its sizes and its local
// header's offset as the record writes them, and its extra field.
func dirRecord(name string, unpacked, packed, offset uint32, extra []byte) []byte {
	h := make([]byte, directoryHeaderLen)
	binary.LittleEndian.PutUint32(h, directoryHeaderSignature)
	binary.LittleEndian.PutUint16(h[4:], 20)
	binary.LittleEndian.PutUint16(h[6:], 20)
	binary.LittleEndian.PutUint32(h[20:], packed)
	binary.LittleEndian.PutUint32(h[24:], unpacked)
	binary.LittleEndian.PutUint16(h[28:], uint16(len(name)))  //nolint:gosec // short names
	binary.LittleEndian.PutUint16(h[30:], uint16(len(extra))) //nolint:gosec // short fields
	binary.LittleEndian.PutUint32(h[42:], offset)
	return slices.Concat(h, []byte(name), extra)
}

// zip64Field is an extra field's zip64 field of values, 8 bytes each.
func zip64Field(values ...uint64) []byte {
	b := binary.LittleEndian.AppendUint16(nil, zip64ExtraID)
	b = binary.LittleEndian.AppendUint16(b, uint16(8*len(values))) //nolint:gosec // a few
	for _, v := range values {
		b = binary.LittleEndian.AppendUint64(b, v)
	}
	return b
}

// dirEnd is a directory's end: its records, its size and its offset.
func dirEnd(records uint16, size, offset uint32) []byte {
	e := make([]byte, directoryEndLen)
	binary.LittleEndian.PutUint32(e, directoryEndSignature)
	binary.LittleEndian.PutUint16(e[8:], records)
	binary.LittleEndian.PutUint16(e[10:], records)
	binary.LittleEndian.PutUint32(e[12:], size)
	binary.LittleEndian.PutUint32(e[16:], offset)
	return e
}

// A record's zip64 fields are too short as archive/zip reads them (M7
// closeout FA2-N1): each case is checked against archive/zip, reading an
// archive of that record alone, as well as against what it says.
func TestZip64ShortReadsTheFieldsAsArchiveZip(t *testing.T) {
	const most = 1<<32 - 1
	other := []byte{0x55, 0x54, 5, 0, 1, 0, 0, 0, 0} // an extended time, 5 bytes
	for _, tt := range []struct {
		name                     string
		unpacked, packed, offset uint32
		extra                    []byte
		short                    bool
	}{
		{"nothing maxed, no field", 1, 1, 0, nil, false},
		{"nothing maxed, an empty field", 1, 1, 0, zip64Field(), false},
		{"a size maxed, an empty field", most, 1, 0, zip64Field(), true},
		{"a size maxed, its value", most, 1, 0, zip64Field(1 << 32), false},
		{"both sizes maxed, one value", most, most, 0, zip64Field(1 << 32), true},
		{"both sizes maxed, two values", most, most, 0, zip64Field(1<<32, 1<<32), false},
		{"sizes and offset maxed, three values", most, most, most, zip64Field(1<<32, 1<<32, 0), false},
		{"sizes and offset maxed, two values", most, most, most, zip64Field(1<<32, 1<<32), true},
		{"the offset given, a later field without it", most, 1, most, slices.Concat(zip64Field(1<<32, 0), zip64Field(1<<32)), false},
		{"the offset given as maxed, a later field without it", 1, 1, most, slices.Concat(zip64Field(most), zip64Field()), true},
		{"after another field, empty", most, 1, 0, slices.Concat(other, zip64Field()), true},
		{"after another field, its value", most, 1, 0, slices.Concat(other, zip64Field(1<<32)), false},
		{"a field running past the extra first", most, 1, 0, slices.Concat([]byte{1, 0, 9, 0}, zip64Field()), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := dirRecord("a", tt.unpacked, tt.packed, tt.offset, tt.extra)
			if got := zip64Short(rec[:directoryHeaderLen], tt.extra); got != tt.short {
				t.Errorf("zip64Short() = %v, want %v", got, tt.short)
			}
			data := slices.Concat(rec, dirEnd(1, uint32(len(rec)), 0)) //nolint:gosec // short
			if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); (err != nil) != tt.short {
				t.Errorf("archive/zip: %v, want it refused %v", err, tt.short)
			}
			if _, err := readImport(&counted{Reader: bytes.NewReader(data)}, 50, MaxDirectory); (err != nil) != tt.short {
				t.Errorf("readImport: %v, want it refused %v", err, tt.short)
			}
		})
	}
}

// Where the end names a base past 0, archive/zip takes 0 when it reads a
// record at the offset from 0; one it refuses there leaves the base (M7
// closeout FA2-M1): the pre-read reads as many records as it.
func TestThePreReadTakesTheBaseAsArchiveZip(t *testing.T) {
	refused := func(name string) []byte { return dirRecord(name, 1<<32-1, 1, 0, zip64Field()) }
	b1, b2 := dirRecord("b1", 1, 1, 0, nil), dirRecord("b2", 1, 1, 0, nil)
	for _, tt := range []struct {
		name string
		data []byte
		want int
	}{
		{"a record refused at the offset from 0", slices.Concat(refused("a"), bytes.Repeat([]byte("x"), 9), b1, b2,
			dirEnd(2, uint32(len(b1)+len(b2)), 0)), 2}, //nolint:gosec // short
		{"two records refused before the directory", slices.Concat(refused("a"), refused("x"), b1,
			dirEnd(1, uint32(len(b1)), 0)), 1}, //nolint:gosec // short
		{"a record read at the offset from 0", slices.Concat(b1, bytes.Repeat([]byte("x"), 9),
			dirEnd(1, uint32(len(b1)), 0)), 1}, //nolint:gosec // short
	} {
		t.Run(tt.name, func(t *testing.T) {
			zr, err := zip.NewReader(bytes.NewReader(tt.data), int64(len(tt.data)))
			if err != nil || len(zr.File) != tt.want {
				t.Fatalf("archive/zip: %v, want %d entries", err, tt.want)
			}
			got, err := readImport(&counted{Reader: bytes.NewReader(tt.data)}, 50, MaxDirectory)
			if err != nil {
				t.Fatalf("readImport() = %v, want %d entries", err, tt.want)
			}
			if len(got.File) != tt.want {
				t.Errorf("readImport read %d entries, want %d", len(got.File), tt.want)
			}
		})
	}
}

// withComment is the record rec with a comment of n bytes.
func withComment(rec []byte, n int) []byte {
	out := slices.Concat(rec, bytes.Repeat([]byte("c"), n))
	binary.LittleEndian.PutUint16(out[32:], uint16(n)) //nolint:gosec // short
	return out
}

// archive/zip reads a record whole before it refuses it, its name, extra
// and comment up to 192 KiB, or up to the file's end: the read's bound
// lets it (M7 closeout F3-M1), and the directory before it is read.
func TestARefusedRecordIsReadWithinTheBound(t *testing.T) {
	b1 := dirRecord("b1", 1, 1, 0, nil)
	// A size maxed, the zip64 field empty after n bytes of another field.
	refused := func(name, other, comment int) []byte {
		extra := binary.LittleEndian.AppendUint16([]byte{0x99, 0x99}, uint16(other)) //nolint:gosec // short
		extra = slices.Concat(extra, make([]byte, other), zip64Field())
		return withComment(dirRecord(string(bytes.Repeat([]byte("n"), name)), 1<<32-1, 1, 0, extra), comment)
	}
	// The end with a comment of n bytes: past 1 KiB, it is found in the last 65 KiB.
	endWith := func(size, n int) []byte {
		end := dirEnd(1, uint32(size), 0)                  //nolint:gosec // short
		binary.LittleEndian.PutUint16(end[20:], uint16(n)) //nolint:gosec // short
		return slices.Concat(end, bytes.Repeat([]byte("e"), n))
	}
	// A record's header saying a name, extra and comment of these lengths, the file ending within them.
	cut := func(name, extra, comment uint16) []byte {
		h := dirRecord("", 1, 1, 0, nil)
		binary.LittleEndian.PutUint16(h[28:], name)
		binary.LittleEndian.PutUint16(h[30:], extra)
		binary.LittleEndian.PutUint16(h[32:], comment)
		return h
	}
	long, shorter := refused(65535, 65527, 65535), refused(21000, 21000, 22000)
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"a record of 192 KiB refused", slices.Concat(b1, long, endWith(len(b1)+len(long), 0))},
		{"a record of 64 KiB refused, the end in the last 65 KiB", slices.Concat(b1, shorter, endWith(len(b1)+len(shorter), 2000))},
		{"a record's comment cut by the file's end", slices.Concat(b1, cut(65535, 65535, 65535), make([]byte, 150000), endWith(len(b1), 0))},
		{"a record's extra cut, the end in the last 65 KiB", slices.Concat(b1, cut(0, 65535, 0), make([]byte, 63000), endWith(len(b1), 2000))},
		{"a record's name cut, the end in the last 65 KiB", slices.Concat(b1, cut(65535, 0, 0), make([]byte, 63000), endWith(len(b1), 2000))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			zr, err := zip.NewReader(bytes.NewReader(tt.data), int64(len(tt.data)))
			if err != nil || len(zr.File) != 1 {
				t.Fatalf("archive/zip: %v, want 1 entry", err)
			}
			got, err := readImport(&counted{Reader: bytes.NewReader(tt.data)}, 50, MaxDirectory)
			if err != nil {
				t.Fatalf("readImport() = %v, want 1 entry", err)
			}
			if len(got.File) != 1 {
				t.Errorf("readImport read %d entries, want 1", len(got.File))
			}
		})
	}
}
