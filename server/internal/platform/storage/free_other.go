//go:build !darwin && !linux

package storage

import (
	"errors"
	"runtime"
)

// freeBytes has no implementation on this system: the store runs on Linux
// and macOS.
func freeBytes(string) (int64, error) {
	return 0, errors.New("free space is not supported on " + runtime.GOOS)
}
