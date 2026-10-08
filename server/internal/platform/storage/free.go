//go:build darwin || linux

package storage

import "syscall"

// freeBytes is the space an unprivileged process may still write on the
// file system of path: its available blocks times their size.
func freeBytes(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}
