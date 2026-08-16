package update

import "golang.org/x/sys/unix"

// statfsBlockSize returns the filesystem block size as an unsigned count.
// On Linux Statfs_t.Bsize is int64; a successful statfs never reports a
// negative block size, but the guard keeps the conversion provably safe.
func statfsBlockSize(st *unix.Statfs_t) uint64 {
	if st.Bsize < 0 {
		return 0
	}

	return uint64(st.Bsize)
}
