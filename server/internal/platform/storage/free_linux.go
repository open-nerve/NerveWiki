package storage

import "syscall"

// freeBytes is the space an unprivileged process may still write on the
// file system of path: its available blocks times their size, which Linux
// counts in fragments (f_frsize, as df does), the block size only when a
// file system reports none.
func freeBytes(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	size := int64(st.Frsize)
	if size <= 0 {
		size = int64(st.Bsize)
	}
	return int64(st.Bavail) * size, nil
}
