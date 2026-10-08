// Package storage keeps files by key (M7 design 4.1; M7/P1 design 3.2): the
// attachments' bytes and the import and export archives. It knows keys and
// bytes only, not what a file is for. Local is its one implementation, a
// directory on the local disk; storagetest is the contract every
// implementation passes. It uses only the standard library.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// ErrNotFound is the error of Open for a key that has no committed file.
var ErrNotFound = errors.New("storage: no file at the key")

// ErrFull is the error of a write that would leave less free space than the
// store keeps, or that ran out of space.
var ErrFull = errors.New("storage: not enough free space")

// Store keeps files by key. A file is written once: Create, write, Commit;
// readers see it whole or not at all.
type Store interface {
	// Create starts a file at key. Nothing is visible at key until the
	// Writer commits; a file already there is replaced then.
	Create(ctx context.Context, key string) (Writer, error)
	// Open opens the committed file at key, or answers ErrNotFound.
	Open(ctx context.Context, key string) (File, error)
	// Delete removes the file at key; there being none is success.
	Delete(ctx context.Context, key string) error
	// List calls each with the key of every committed file of area last
	// modified before before, and stops at the first error each returns.
	List(ctx context.Context, area string, before time.Time, each func(key string) error) error
	// Free tells how many bytes are left to write.
	Free(ctx context.Context) (int64, error)
}

// Writer writes a file that becomes visible when it commits. Exactly one
// of Commit and Abort takes effect; any call after it is an error.
type Writer interface {
	io.Writer
	// Commit makes the file durable and visible at its key.
	Commit() error
	// Abort drops what was written.
	Abort() error
}

// File is a committed file open for reading.
type File interface {
	io.ReadSeekCloser
	io.ReaderAt
	Size() int64
	ModTime() time.Time
}

// maxName bounds a key's name, the part after its area.
const maxName = 128

// CheckKey tells whether key is a key: "<area>/<name>", the area lower-case
// ASCII letters, the name lower-case ASCII letters, digits, '.' and '-', not
// starting with '.', at most maxName bytes. A key never names a path outside
// the store, whoever wrote it.
func CheckKey(key string) error {
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			if err := CheckArea(key[:i]); err != nil {
				return fmt.Errorf("storage: key %q: %w", key, err)
			}
			if !isName(key[i+1:]) {
				return fmt.Errorf("storage: key %q: the name is not lower-case letters, digits, '.' and '-'", key)
			}
			return nil
		}
	}
	return fmt.Errorf("storage: key %q has no area", key)
}

// CheckArea tells whether area is an area: lower-case ASCII letters.
func CheckArea(area string) error {
	if area == "" {
		return errors.New("the area is empty")
	}
	for i := 0; i < len(area); i++ {
		if area[i] < 'a' || area[i] > 'z' {
			return fmt.Errorf("the area %q is not lower-case letters", area)
		}
	}
	return nil
}

func isName(s string) bool {
	if s == "" || len(s) > maxName || s[0] == '.' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '.' && c != '-' {
			return false
		}
	}
	return true
}
