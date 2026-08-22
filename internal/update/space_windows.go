package update

import "golang.org/x/sys/windows"

// statfsFree returns the bytes available to the calling user on the volume
// containing dir. Windows has no statfs: GetDiskFreeSpaceEx already reports
// quota-adjusted free space, so there is no block arithmetic and no
// platform-specific block-size helper.
func statfsFree(dir string) (uint64, error) {
	name, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}

	var free uint64
	if err := windows.GetDiskFreeSpaceEx(name, &free, nil, nil); err != nil {
		return 0, err
	}

	return free, nil
}
