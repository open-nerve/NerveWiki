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
	"syscall"
	"time"
)

// tmpDir is the directory of an area's files being written. It sits in the
// area, so committing renames within one directory tree: an area mounted on
// its own file system still commits by rename.
const tmpDir = ".tmp"

// Local is the store on the local disk (M7/P1 design 3.3): the file at
// "<area>/<name>" is <dir>/<area>/<s1>/<s2>/<name>, s1 and s2 the first two
// bytes of the name's SHA-256 in hex, so no directory holds more than a
// fraction of the files. Only one process may use a directory: OpenLocal
// deletes the files being written that it finds.
type Local struct {
	dir     string
	minFree int64
}

var _ Store = (*Local)(nil)

// OpenLocal opens the store in dir, making dir when it is missing. It
// fails, naming the directory and the process's uid and gid, when it cannot
// write there: serve refuses to start (M0/P6 handoff, item 2). It then
// deletes what earlier processes left half written. Writes that would leave
// less than minFree bytes free answer ErrFull.
func OpenLocal(dir string, minFree int64) (*Local, error) {
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
	if err := probe(abs); err != nil {
		return nil, fmt.Errorf("storage: cannot write in %s as uid %d, gid %d: %w", abs, os.Getuid(), os.Getgid(), err)
	}
	l := &Local{dir: abs, minFree: minFree}
	if err := l.dropTemporaries(); err != nil {
		return nil, err
	}
	return l, nil
}

// probe makes dir when it is missing and writes, syncs and deletes a file
// in it.
func probe(dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".probe-*")
	if err != nil {
		return err
	}
	_, werr := f.Write([]byte{0})
	serr := f.Sync()
	cerr := f.Close()
	rerr := os.Remove(f.Name())
	return errors.Join(werr, serr, cerr, rerr)
}

// dropTemporaries deletes every area's files being written: with one
// process to a directory, they are what a process stopped midway left.
func (l *Local) dropTemporaries() error {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() || CheckArea(e.Name()) != nil {
			continue
		}
		if err := os.RemoveAll(filepath.Join(l.dir, e.Name(), tmpDir)); err != nil {
			return fmt.Errorf("storage: %w", err)
		}
	}
	return nil
}

// Create starts the file at key in the area's temporary directory. It
// answers ErrFull when fewer than the store's minimum of bytes are free.
func (l *Local) Create(ctx context.Context, key string) (Writer, error) {
	if err := CheckKey(key); err != nil {
		return nil, err
	}
	free, err := l.Free(ctx)
	if err != nil {
		return nil, err
	}
	if free < l.minFree {
		return nil, fmt.Errorf("%w: %d bytes free, %d kept", ErrFull, free, l.minFree)
	}
	area, name := split(key)
	tmp := filepath.Join(l.dir, area, tmpDir)
	if err := os.MkdirAll(tmp, 0o750); err != nil {
		return nil, fmt.Errorf("storage: %w", noSpace(err))
	}
	f, err := os.CreateTemp(tmp, name+".*")
	if err != nil {
		return nil, fmt.Errorf("storage: %w", noSpace(err))
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
	n, err := freeBytes(l.dir)
	if err != nil {
		return 0, fmt.Errorf("storage: free space of %s: %w", l.dir, err)
	}
	return n, nil
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
// that renames into it survives a crash.
func (l *Local) ensureDir(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(dir)
	if parent != l.dir {
		if err := l.ensureDir(parent); err != nil {
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

// noSpace makes a write that ran out of space ErrFull, keeping err.
func noSpace(err error) error {
	if errors.Is(err, syscall.ENOSPC) {
		return fmt.Errorf("%w: %w", ErrFull, err)
	}
	return err
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
}

var errDone = errors.New("storage: the file was already committed or aborted")

func (w *localWriter) Write(p []byte) (int, error) {
	if w.done {
		return 0, errDone
	}
	n, err := w.file.Write(p)
	if err != nil {
		return n, fmt.Errorf("storage: %w", noSpace(err))
	}
	return n, nil
}

// Commit syncs the file, renames it into its shard and syncs the shard, so
// a crash leaves either no file or the whole file. On failure the file is
// gone.
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
		return noSpaceErr(syncDir(dir))
	}
	_ = os.Remove(tmp)
	return noSpaceErr(err)
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

func noSpaceErr(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("storage: %w", noSpace(err))
}

// localFile is an open committed file with its size and modification time.
type localFile struct {
	*os.File
	size    int64
	modTime time.Time
}

func (f localFile) Size() int64        { return f.size }
func (f localFile) ModTime() time.Time { return f.modTime }
