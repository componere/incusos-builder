//go:build !windows

package update

import "golang.org/x/sys/unix"

// statfsFree returns available bytes on the filesystem containing dir.
// Bavail is uint64 on every supported Unix platform; the block size goes
// through the platform-specific statfsBlockSize helper (Bsize is signed on
// Linux, unsigned elsewhere).
func statfsFree(dir string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}

	return st.Bavail * statfsBlockSize(&st), nil
}
