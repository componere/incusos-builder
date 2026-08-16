//go:build !linux

package update

import "golang.org/x/sys/unix"

// statfsBlockSize returns the filesystem block size as an unsigned count.
// On non-Linux Unix platforms Statfs_t.Bsize is already unsigned.
func statfsBlockSize(st *unix.Statfs_t) uint64 {
	return uint64(st.Bsize)
}
