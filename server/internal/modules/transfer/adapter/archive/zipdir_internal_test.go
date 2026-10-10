package archiveadapter

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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
