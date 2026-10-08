package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
)

// now is the fixed clock's time.
func now() time.Time { return time.Date(2026, 10, 8, 10, 30, 0, 0, time.UTC) }

// signKey is the bytes 0 to 31.
func signKey() []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// clock tells now(), moved on by step at each read after the first; it
// counts its reads, which an operation makes one of.
type clock struct {
	step  time.Duration
	reads int
}

func (c *clock) Now() time.Time {
	c.reads++
	return now().Add(time.Duration(c.reads-1) * c.step)
}

// memFiles is the store in memory: committed files by key, and the keys
// created. full refuses a Create; fullAfter fails a write once so many
// bytes are written; a Commit fails with commitErr, an Abort with
// abortErr, Free with freeErr; the keys deleted are kept, but
// undeletable's, whose Delete fails with errDenied. reading counts the
// files opened and not closed.
type memFiles struct {
	mu          sync.Mutex
	files       map[string][]byte
	created     []string
	writing     int
	reading     int
	deleted     []string
	full        bool
	fullAfter   int
	commitErr   error
	abortErr    error
	free        int64
	freeErr     error
	undeletable string
}

func newFiles() *memFiles { return &memFiles{files: map[string][]byte{}, free: 1 << 40} }

func (f *memFiles) Create(_ context.Context, key string) (app.FileWriter, error) {
	if f.full {
		return nil, domain.ErrStorageFull
	}
	f.mu.Lock()
	f.created = append(f.created, key)
	f.writing++
	f.mu.Unlock()
	return &memWriter{f: f, key: key}, nil
}

func (f *memFiles) Open(_ context.Context, key string) (app.File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.files[key]
	if !ok {
		return nil, app.ErrNoFile
	}
	f.reading++
	return &memFile{Reader: bytes.NewReader(b), f: f}, nil
}

func (f *memFiles) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if key == f.undeletable {
		return fmt.Errorf("remove %s: %w", key, errDenied)
	}
	delete(f.files, key)
	f.deleted = append(f.deleted, key)
	return nil
}

func (f *memFiles) Free(context.Context) (int64, error) { return f.free, f.freeErr }

// keys are the committed files' keys.
func (f *memFiles) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for k := range f.files {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

type memWriter struct {
	f    *memFiles
	key  string
	buf  bytes.Buffer
	done bool
}

func (w *memWriter) Write(p []byte) (int, error) {
	if w.f.fullAfter > 0 && w.buf.Len()+len(p) > w.f.fullAfter {
		return 0, domain.ErrStorageFull
	}
	return w.buf.Write(p)
}

func (w *memWriter) Commit() error {
	w.f.mu.Lock()
	defer w.f.mu.Unlock()
	w.done = true
	w.f.writing--
	if w.f.commitErr != nil {
		return w.f.commitErr
	}
	w.f.files[w.key] = w.buf.Bytes()
	return nil
}

func (w *memWriter) Abort() error {
	w.f.mu.Lock()
	defer w.f.mu.Unlock()
	w.done = true
	w.f.writing--
	return w.f.abortErr
}

type memFile struct {
	*bytes.Reader
	f      *memFiles
	closed bool
}

func (m *memFile) Close() error {
	m.f.mu.Lock()
	defer m.f.mu.Unlock()
	if !m.closed {
		m.closed = true
		m.f.reading--
	}
	return nil
}

func (*memFile) ModTime() time.Time { return now() }

// sniffer sniffs as told and reads an image's size as told; unsized, it
// answers the size read not, numbers and all.
type sniffer struct {
	sniffed       string
	width, height int
	unsized       bool
	reads         int
}

func (s *sniffer) Sniff([]byte) string { return s.sniffed }

func (s *sniffer) Size(r io.Reader) (int, int, bool) {
	s.reads++
	_, err := io.ReadAll(r)
	return s.width, s.height, err == nil && !s.unsized
}

// unitKey marks the context of the tree's unit, which the row is written
// in.
type unitKey struct{}

// memRows keeps the rows by node; err fails CreateBlob, readErr the reads.
// inUnit tells whether the last row was written in a unit's context.
type memRows struct {
	rows    map[uuid.UUID]domain.Blob
	err     error
	readErr error
	inUnit  bool
}

func newRows() *memRows { return &memRows{rows: map[uuid.UUID]domain.Blob{}} }

func (r *memRows) CreateBlob(ctx context.Context, b domain.Blob) error {
	r.inUnit = ctx.Value(unitKey{}) != nil
	if r.err != nil {
		return r.err
	}
	r.rows[b.NodeID] = b
	return nil
}

func (r *memRows) BlobOfNode(_ context.Context, nodeID uuid.UUID) (domain.Blob, error) {
	if r.readErr != nil {
		return domain.Blob{}, r.readErr
	}
	b, ok := r.rows[nodeID]
	if !ok {
		return domain.Blob{}, app.ErrNoRow
	}
	return b, nil
}

func (r *memRows) BlobsOfNodes(_ context.Context, nodeIDs []uuid.UUID) (map[uuid.UUID]domain.Blob, error) {
	if r.readErr != nil {
		return nil, r.readErr
	}
	out := map[uuid.UUID]domain.Blob{}
	for _, id := range nodeIDs {
		if b, ok := r.rows[id]; ok {
			out[id] = b
		}
	}
	return out, nil
}

// failingReader brings data, then fails with err.
type failingReader struct {
	data []byte
	err  error
}

func (r *failingReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

var errBroken = errors.New("the connection broke")

// errDenied is a deletion's failure; errPort a port's that is no domain
// error.
var (
	errDenied = errors.New("permission denied")
	errPort   = errors.New("connection reset")
)
