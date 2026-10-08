package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
)

// headSniffer is sniffer that keeps the head it was given.
type headSniffer struct {
	*sniffer
	head []byte
}

func (s *headSniffer) Sniff(head []byte) string {
	s.head = slices.Clone(head)
	return s.sniffer.Sniff(head)
}

// Put commits the file at its key as the bytes pass, a read at a time, its
// SHA-256 and size counted; the type comes from the name and the first
// 512 bytes, however short the reads, and an image's size from the
// committed file, closed once read.
func TestPutWritesTheFileAndTellsWhatItIs(t *testing.T) {
	files, rows := newFiles(), newRows()
	s := &headSniffer{sniffer: &sniffer{sniffed: "image/png", width: 640, height: 480}}
	data := bytes.Repeat([]byte("0123456789abcdef"), 6<<10) // 96 KiB, three reads and more
	blobs := app.NewBlobs(files, rows, s, slog.New(slog.DiscardHandler))
	b, err := blobs.Put(context.Background(), "photo.png", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if b.ID == (uuid.UUID{}) || b.MIME != "image/png" || b.Bytes != int64(len(data)) || !bytes.Equal(b.SHA256, sum[:]) ||
		b.Width != 640 || b.Height != 480 {
		t.Errorf("Put() = %+v, want a PNG of %d bytes, its SHA-256, 640×480", b, len(data))
	}
	if got := files.files[domain.Key(b.ID)]; !bytes.Equal(got, data) || files.writing != 0 || s.reads != 1 || files.reading != 0 {
		t.Errorf("the file holds %d bytes, %d writers open, read %d times for its size, %d left open; want the data, none, once, none",
			len(got), files.writing, s.reads, files.reading)
	}
	if !bytes.Equal(s.head, data[:512]) {
		t.Errorf("sniffed %d bytes, want the first 512", len(s.head))
	}
	if _, err := blobs.Put(context.Background(), "photo.png", iotest.OneByteReader(bytes.NewReader(data[:1<<10])), 1<<10); err != nil ||
		!bytes.Equal(s.head, data[:512]) {
		t.Errorf("Put(a byte a read) = %v, sniffing %d bytes; want the first 512", err, len(s.head))
	}
}

// The type is the domain's of the name and the sniffed bytes; a file
// shorter than 512 bytes is sniffed whole. Only PNG, JPEG and GIF are
// read for their size, and a size past 65535, of a side of 0, or none is
// not kept.
func TestPutTellsTheTypeAndTheSize(t *testing.T) {
	for _, tt := range []struct {
		name, file, sniffed string
		width, height       int
		unsized             bool
		mime                string
		wantW, wantH, reads int
	}{
		{"an image", "a.gif", "image/gif", 3, 4, false, "image/gif", 3, 4, 1},
		{"HTML named as an image", "a.png", "text/html; charset=utf-8", 3, 4, false, "application/octet-stream", 0, 0, 0},
		{"a PDF", "a.pdf", "application/pdf", 3, 4, false, "application/pdf", 0, 0, 0},
		{"an image too wide", "a.png", "image/png", 65536, 4, false, "image/png", 0, 0, 1},
		{"an image too high", "a.jpg", "image/jpeg", 4, 65536, false, "image/jpeg", 0, 0, 1},
		{"an image of the largest size", "a.jpg", "image/jpeg", 65535, 65535, false, "image/jpeg", 65535, 65535, 1},
		{"an image of no width", "a.gif", "image/gif", 0, 4, false, "image/gif", 0, 0, 1},
		{"an image of no height", "a.gif", "image/gif", 4, 0, false, "image/gif", 0, 0, 1},
		{"an image whose size is not read", "a.png", "image/png", 3, 4, true, "image/png", 0, 0, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &headSniffer{sniffer: &sniffer{sniffed: tt.sniffed, width: tt.width, height: tt.height, unsized: tt.unsized}}
			blobs := app.NewBlobs(newFiles(), newRows(), s, slog.New(slog.DiscardHandler))
			b, err := blobs.Put(context.Background(), tt.file, bytes.NewReader([]byte("abc")), 3)
			if err != nil {
				t.Fatal(err)
			}
			if b.MIME != tt.mime || b.Width != tt.wantW || b.Height != tt.wantH || s.reads != tt.reads || string(s.head) != "abc" {
				t.Errorf("Put() = %+v after %d reads of the size, sniffing %q; want %s, %d×%d, %d reads, abc", b, s.reads, s.head,
					tt.mime, tt.wantW, tt.wantH, tt.reads)
			}
		})
	}
}

// A file past the largest, one the reader fails under, and one the store
// has no room for leave no file and no writer open; one of the largest
// size is kept.
func TestPutLeavesNoFileWhenItFails(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 40<<10)
	for _, tt := range []struct {
		name  string
		setup func(f *memFiles) (io.Reader, int64)
		check func(err error) bool
	}{
		{"a byte past the largest", func(*memFiles) (io.Reader, int64) { return bytes.NewReader(data), int64(len(data)) - 1 },
			func(err error) bool { return errors.Is(err, domain.ErrTooLarge) }},
		{"a reader that fails", func(*memFiles) (io.Reader, int64) {
			return &failingReader{data: data, err: errBroken}, 1 << 20
		}, func(err error) bool {
			var r *app.ReadError
			return errors.As(err, &r) && errors.Is(err, errBroken)
		}},
		{"a store without room", func(f *memFiles) (io.Reader, int64) {
			f.full = true
			return bytes.NewReader(data), 1 << 20
		}, func(err error) bool { return errors.Is(err, domain.ErrStorageFull) }},
		{"a store that runs out of room", func(f *memFiles) (io.Reader, int64) {
			f.fullAfter = 33 << 10
			return bytes.NewReader(data), 1 << 20
		}, func(err error) bool { return errors.Is(err, domain.ErrStorageFull) }},
		{"a commit that runs out of room", func(f *memFiles) (io.Reader, int64) {
			f.commitErr = domain.ErrStorageFull
			return bytes.NewReader(data), 1 << 20
		}, func(err error) bool { return errors.Is(err, domain.ErrStorageFull) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files := newFiles()
			r, maxBytes := tt.setup(files)
			_, err := app.NewBlobs(files, newRows(), &sniffer{}, slog.New(slog.DiscardHandler)).Put(context.Background(), "a.bin", r, maxBytes)
			if !tt.check(err) {
				t.Errorf("Put() = %v, want the failure", err)
			}
			if len(files.keys()) != 0 || files.writing != 0 {
				t.Errorf("files %q, %d writers open; want none", files.keys(), files.writing)
			}
		})
	}
	files := newFiles()
	blobs := app.NewBlobs(files, newRows(), &sniffer{}, slog.New(slog.DiscardHandler))
	if _, err := blobs.Put(context.Background(), "a.bin", bytes.NewReader(data), int64(len(data))); err != nil ||
		len(files.keys()) != 1 {
		t.Errorf("Put(the largest) = %v, files %q; want it kept", err, files.keys())
	}
}

// An abort that fails is logged as a warning, with the blob's id; Put
// answers the failure that stopped the write.
func TestPutLogsAnAbortThatFails(t *testing.T) {
	files := newFiles()
	files.abortErr = errors.New("remove .tmp/x: permission denied")
	var logs bytes.Buffer
	data := bytes.Repeat([]byte("x"), 40<<10)
	_, err := app.NewBlobs(files, newRows(), &sniffer{}, slog.New(slog.NewTextHandler(&logs, nil))).Put(context.Background(), "a.bin",
		bytes.NewReader(data), 1<<10)
	if !errors.Is(err, domain.ErrTooLarge) {
		t.Errorf("Put() = %v, want too large", err)
	}
	id, _ := domain.IDOf(files.created[0])
	if l := logs.String(); !strings.Contains(l, "level=WARN") || !strings.Contains(l, "blob_id="+id.String()) ||
		!strings.Contains(l, "permission denied") {
		t.Errorf("logs %q, want the abort's failure as a warning, with the blob's id", l)
	}
}

// Open answers the row of a node with its file; no row, or a row of
// another file, is asset.not_found; a row whose file is gone, ErrNoFile
// with the row. Drop deletes a blob's file.
func TestOpenAndDrop(t *testing.T) {
	ctx := context.Background()
	files, rows := newFiles(), newRows()
	blobs := app.NewBlobs(files, rows, &sniffer{}, slog.New(slog.DiscardHandler))
	b, err := blobs.Put(ctx, "a.txt", bytes.NewReader([]byte("abc")), 3)
	if err != nil {
		t.Fatal(err)
	}
	b.NodeID = uuid.NewV7()
	if err := blobs.Attach(ctx, b); err != nil {
		t.Fatal(err)
	}
	f, got, err := blobs.Open(ctx, b.NodeID, b.ID)
	if err != nil || got.ID != b.ID {
		t.Fatalf("Open() = %+v, %v; want the row", got, err)
	}
	if content, _ := io.ReadAll(f); string(content) != "abc" {
		t.Errorf("the file holds %q, want abc", content)
	}
	if _, _, err := blobs.Open(ctx, uuid.NewV7(), b.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Open(no row) = %v, want asset.not_found", err)
	}
	if _, _, err := blobs.Open(ctx, b.NodeID, uuid.NewV7()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Open(another file) = %v, want asset.not_found", err)
	}
	if err := blobs.Drop(ctx, b); err != nil || len(files.keys()) != 0 {
		t.Errorf("Drop() = %v, files %q; want the file deleted", err, files.keys())
	}
	if _, got, err := blobs.Open(ctx, b.NodeID, b.ID); !errors.Is(err, app.ErrNoFile) || got.ID != b.ID {
		t.Errorf("Open(a row without its file) = %+v, %v; want the row and ErrNoFile", got, err)
	}
}
