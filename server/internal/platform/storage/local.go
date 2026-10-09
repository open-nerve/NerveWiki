package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// tmpDir is the directory of an area's files being written. It sits in the
// area, so committing renames within one directory tree: an area mounted on
// its own file system still commits by rename.
const tmpDir = ".tmp"

// probePrefix starts the name of the file OpenLocal writes to learn that it
// can.
const probePrefix = ".probe-"

// freeCheck is how many bytes a file's writes go between two reads of the
// free space: a file far larger than an attachment, an export's archive,
// stops once fewer than the store's minimum of bytes are free, and an
// attachment of the default largest (asset.max_bytes) is never read again.
const freeCheck = 64 << 20

// Local is the store on the local disk (M7/P1 design 3.3): the file at
// "<area>/<name>" is <dir>/<area>/<s1>/<s2>/<name>, s1 and s2 the first two
// bytes of the name's SHA-256 in hex, so no directory holds more than a
// fraction of the files. Only one process may use a directory: OpenLocal
// deletes the files being written that it finds.
type Local struct {
	dir     string
	minFree int64
	create  createTemp
	// free reads the free bytes of a path's file system, and freeCheck is
	// how many bytes a file's writes go between two reads: freeBytes and
	// the constant, and in tests a disk that fills.
	free      func(path string) (int64, error)
	freeCheck int64
	mkdir     sync.Mutex // one goroutine at a time makes and syncs directories
	// fullAtOpen is the first out-of-space error opening's probes met.
	fullAtOpen error
}

// createTemp makes a new file in dir, its name pattern's "*" made unique:
// os.CreateTemp, and in tests a file on a full disk.
type createTemp func(dir, pattern string) (tempFile, error)

func osCreateTemp(dir, pattern string) (tempFile, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return f, nil
}

var _ Store = (*Local)(nil)

// OpenLocal opens the store in dir, making dir when it is missing. It
// deletes what earlier processes left half written, and fails, naming the
// directory and the process's uid and gid, when it cannot write there or
// in an area already in it: serve refuses to start (M0/P6 handoff, item 2).
// A disk out of space is no failure: Create, and a file's writes as they
// go, answer ErrFull once fewer than minFree bytes are free.
func OpenLocal(dir string, minFree int64) (*Local, error) {
	return openLocal(dir, minFree, osCreateTemp)
}

func openLocal(dir string, minFree int64, create createTemp) (*Local, error) {
	if dir == "" {
		return nil, errors.New("storage: no directory")
	}
	if minFree < 0 {
		return nil, fmt.Errorf("storage: min free bytes %d is negative", minFree)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("storage: %s: %w", dir, err)
	}
	l := &Local{dir: abs, minFree: minFree, create: create, free: freeBytes, freeCheck: freeCheck}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, cannotWrite(abs, err)
	}
	areas, err := l.dropTemporaries()
	if err != nil {
		return nil, err
	}
	// Probed once every deletion has made what room it could.
	if err := l.probe(abs); err != nil {
		return nil, cannotWrite(abs, err)
	}
	for _, area := range areas {
		tmp := filepath.Join(area, tmpDir)
		if err := l.probe(tmp); err != nil {
			return nil, cannotWrite(area, err)
		}
		if err := os.RemoveAll(tmp); err != nil {
			return nil, cannotWrite(area, err)
		}
	}
	return l, nil
}

func cannotWrite(dir string, err error) error {
	return fmt.Errorf("storage: cannot write in %s as uid %d, gid %d: %w", dir, os.Getuid(), os.Getgid(), err)
}

// probe makes dir when it is missing and writes, syncs and deletes a file
// in it. Running out of space passes, and is kept for FullAtOpen: the
// process may write there, and Create answers ErrFull until there is space
// again.
func (l *Local) probe(dir string) error {
	err := l.write(dir)
	if isFull(err) {
		if l.fullAtOpen == nil {
			l.fullAtOpen = err
		}
		return nil
	}
	return err
}

// FullAtOpen is the error of running out of space, on the disk or under a
// quota, that OpenLocal met, nil when it met none. The store opened all
// the same; its writes answer ErrFull until there is space again.
func (l *Local) FullAtOpen() error { return l.fullAtOpen }

func (l *Local) write(dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	f, err := l.create(dir, probePrefix+"*")
	if err != nil {
		return err
	}
	_, werr := f.Write([]byte{0})
	serr := f.Sync()
	cerr := f.Close()
	rerr := os.Remove(f.Name())
	return errors.Join(werr, serr, cerr, rerr)
}

// dropTemporaries deletes every area's directory of files being written
// and the probes left in the store's directory, and answers the areas'
// directories, which opening then probes in their directory of files being
// written. With one process to a directory, what it deletes is what a
// process stopped midway left. Deleting before probing frees the space a
// half-written file took, likely what filled the disk, and a file in the
// directory's place.
func (l *Local) dropTemporaries() ([]string, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, cannotWrite(l.dir, err)
	}
	var areas []string
	for _, e := range entries {
		path := filepath.Join(l.dir, e.Name())
		if strings.HasPrefix(e.Name(), probePrefix) {
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return nil, cannotWrite(l.dir, err)
			}
			continue
		}
		if CheckArea(e.Name()) != nil {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, cannotWrite(path, err) // a link to nothing, or out of reach
		}
		if !info.IsDir() {
			continue // not an area: a mounted area is a directory or a link to one
		}
		if err := os.RemoveAll(filepath.Join(path, tmpDir)); err != nil {
			return nil, cannotWrite(path, err)
		}
		areas = append(areas, path)
	}
	return areas, nil
}

// Dir is the store's directory, absolute.
func (l *Local) Dir() string { return l.dir }

// Create starts the file at key in the area's temporary directory. It
// answers ErrFull when fewer than the store's minimum of bytes are free;
// so do the file's writes, every freeCheck bytes.
func (l *Local) Create(ctx context.Context, key string) (Writer, error) {
	if err := CheckKey(key); err != nil {
		return nil, err
	}
	if err := l.room(ctx); err != nil {
		return nil, err
	}
	area, name := split(key)
	tmp := filepath.Join(l.dir, area, tmpDir)
	if err := l.ensureDir(tmp); err != nil {
		return nil, failed(err)
	}
	f, err := l.create(tmp, name+".*")
	if err != nil {
		return nil, failed(err)
	}
	return &localWriter{l: l, area: area, name: name, file: f}, nil
}

// Open opens the committed file at key.
func (l *Local) Open(_ context.Context, key string) (File, error) {
	if err := CheckKey(key); err != nil {
		return nil, err
	}
	f, err := os.Open(l.path(split(key)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("storage: %w", err)
	}
	return localFile{File: f, size: info.Size(), modTime: info.ModTime()}, nil
}

// Delete removes the file at key; there being none is success.
func (l *Local) Delete(_ context.Context, key string) error {
	if err := CheckKey(key); err != nil {
		return err
	}
	if err := os.Remove(l.path(split(key))); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("storage: %w", err)
	}
	return nil
}

// List walks the area's two levels of shards, leaving out the files being
// written.
func (l *Local) List(ctx context.Context, area string, before time.Time, each func(key string) error) error {
	if err := CheckArea(area); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	root := filepath.Join(l.dir, area)
	shards, err := shardsOf(root)
	if err != nil {
		return err
	}
	for _, s1 := range shards {
		inner, err := shardsOf(filepath.Join(root, s1))
		if err != nil {
			return err
		}
		for _, s2 := range inner {
			if err := ctx.Err(); err != nil {
				return err
			}
			dir := filepath.Join(root, s1, s2)
			files, err := os.ReadDir(dir)
			if errors.Is(err, fs.ErrNotExist) {
				continue // deleted since its shard was read
			}
			if err != nil {
				return fmt.Errorf("storage: %w", err)
			}
			for _, f := range files {
				if !f.Type().IsRegular() || !isName(f.Name()) {
					continue
				}
				info, err := f.Info()
				if errors.Is(err, fs.ErrNotExist) {
					continue // deleted since the directory was read
				}
				if err != nil {
					return fmt.Errorf("storage: %w", err)
				}
				if info.ModTime().Before(before) {
					if err := each(area + "/" + f.Name()); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// shardsOf lists the shard directories in dir: two hex digits each. A
// missing dir has none.
func shardsOf(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	var shards []string
	for _, e := range entries {
		if e.IsDir() && isShard(e.Name()) {
			shards = append(shards, e.Name())
		}
	}
	return shards, nil
}

func isShard(s string) bool {
	if len(s) != 2 {
		return false
	}
	for i := 0; i < 2; i++ {
		if (s[i] < '0' || s[i] > '9') && (s[i] < 'a' || s[i] > 'f') {
			return false
		}
	}
	return true
}

// Free tells how many bytes are left to write in the store's directory.
func (l *Local) Free(context.Context) (int64, error) {
	n, err := l.free(l.dir)
	if err != nil {
		return 0, fmt.Errorf("storage: free space of %s: %w", l.dir, err)
	}
	return n, nil
}

// room answers ErrFull when fewer than the store's minimum of bytes are
// free.
func (l *Local) room(ctx context.Context) error {
	free, err := l.Free(ctx)
	if err != nil {
		return err
	}
	if free < l.minFree {
		return fmt.Errorf("%w: %d bytes free, %d kept", ErrFull, free, l.minFree)
	}
	return nil
}

// split cuts a checked key into its area and name.
func split(key string) (area, name string) {
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

// shardDir is the directory of the file name in area.
func (l *Local) shardDir(area, name string) string {
	sum := sha256.Sum256([]byte(name))
	h := hex.EncodeToString(sum[:2])
	return filepath.Join(l.dir, area, h[:2], h[2:])
}

func (l *Local) path(area, name string) string {
	return filepath.Join(l.shardDir(area, name), name)
}

// ensureDir makes dir and the missing directories above it, up to the
// store's directory, and syncs the directory each was made in, so a commit
// that renames into it survives a crash. It does so one goroutine at a
// time: another finding dir made has it synced.
func (l *Local) ensureDir(dir string) error {
	l.mkdir.Lock()
	defer l.mkdir.Unlock()
	return l.makeDir(dir)
}

func (l *Local) makeDir(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(dir)
	if parent != l.dir {
		if err := l.makeDir(parent); err != nil {
			return err
		}
	}
	if err := os.Mkdir(dir, 0o750); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return syncDir(parent)
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	serr := d.Sync()
	return errors.Join(serr, d.Close())
}

// failed is a file operation's error, err kept: ErrFull when it ran out of
// space or of quota; nil when err is.
func failed(err error) error {
	switch {
	case err == nil:
		return nil
	case isFull(err):
		return fmt.Errorf("%w: %w", ErrFull, err)
	default:
		return fmt.Errorf("storage: %w", err)
	}
}

// tempFile is what a localWriter writes to: an *os.File.
type tempFile interface {
	Write(p []byte) (int, error)
	Sync() error
	Close() error
	Name() string
}

// localWriter writes a file in its area's temporary directory and renames
// it into place on Commit.
type localWriter struct {
	l          *Local
	area, name string
	file       tempFile
	done       bool
	// unchecked is how many bytes were written since the free space was
	// last read.
	unchecked int64
}

var errDone = errors.New("storage: the file was already committed or aborted")

// Write writes p; once freeCheck bytes went since the free space was last
// read, it reads it first, and answers ErrFull when fewer than the store's
// minimum of bytes are free.
func (w *localWriter) Write(p []byte) (int, error) {
	if w.done {
		return 0, errDone
	}
	if w.unchecked >= w.l.freeCheck {
		// A read of the free space that fails skips this check: the
		// writes need it no more than they did before it.
		if err := w.l.room(context.Background()); errors.Is(err, ErrFull) {
			return 0, err
		}
		w.unchecked = 0
	}
	n, err := w.file.Write(p)
	w.unchecked += int64(n)
	if err != nil {
		return n, failed(err)
	}
	return n, nil
}

// Commit syncs the file, renames it into its shard and syncs the shard, so
// a crash leaves either no file or the whole file. On failure the file is
// gone, unless only the last sync failed: then it is at key already and may
// not outlive a crash; the caller treats it as an orphan.
func (w *localWriter) Commit() error {
	if w.done {
		return errDone
	}
	w.done = true
	tmp := w.file.Name()
	err := w.file.Sync()
	err = errors.Join(err, w.file.Close())
	dir := w.l.shardDir(w.area, w.name)
	if err == nil {
		err = w.l.ensureDir(dir)
	}
	if err == nil {
		err = os.Rename(tmp, filepath.Join(dir, w.name))
	}
	if err == nil {
		return failed(syncDir(dir))
	}
	_ = os.Remove(tmp)
	return failed(err)
}

// Abort closes and deletes the file.
func (w *localWriter) Abort() error {
	if w.done {
		return errDone
	}
	w.done = true
	cerr := w.file.Close()
	rerr := os.Remove(w.file.Name())
	if errors.Is(rerr, fs.ErrNotExist) {
		rerr = nil
	}
	if err := errors.Join(cerr, rerr); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	return nil
}

// isFull tells whether err is a write's that ran out of space or of quota.
func isFull(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT)
}

// localFile is an open committed file with its size and modification time.
type localFile struct {
	*os.File
	size    int64
	modTime time.Time
}

func (f localFile) Size() int64        { return f.size }
func (f localFile) ModTime() time.Time { return f.modTime }
