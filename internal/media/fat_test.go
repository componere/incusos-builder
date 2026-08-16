package media

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRawSizesFailureBands checks the two content sizes where the old
// max(content+1 MiB, 256 MiB) partition left less usable FAT32 data area
// than the payload: ~255 MiB (512-byte clusters on the 256 MiB floor) and
// ~550 MiB (4 KiB clusters, FAT ~0.2% of the partition). The check is
// against go-diskfs v1.9.4 fat32.go:112-150 math, not a multi-hundred-MiB
// write: go-diskfs FAT writes are not sparse, so each payload byte would
// be copied and the test would spend seconds allocating hundreds of
// mebibytes for a formula already pinned to the library.
func TestRawSizesFailureBands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content int64
	}{
		{name: "255 MiB 512-byte-cluster band", content: 255 << 20},
		{name: "550 MiB 4KiB-cluster band", content: 550 << 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			part, disk := rawSizesFor(tt.content)
			require.GreaterOrEqual(t, part, fat32Floor)
			require.Equal(t, gptHead+part+gptTail, disk)
			require.Equal(t, int64(0), part%rawSector)

			usable := fat32UsableBytes(part)
			assert.GreaterOrEqual(t, usable, tt.content,
				"part=%d usable=%d content=%d", part, usable, tt.content)
		})
	}
}

// fat32UsableBytes is the data-area size go-diskfs v1.9.4 FAT32 Create
// leaves after reserved sectors and both FAT tables (fat32.go:112-150).
func fat32UsableBytes(partSize int64) int64 {
	const (
		reservedSectors    = 32
		blocksize          = 512
		kb                 = 1024
		mb                 = 1024 * kb
		gb                 = 1024 * mb
		cluster512Max      = 260 * mb
		cluster4kMax       = 8 * gb
		cluster8kMax       = 16 * gb
		cluster16kMax      = 32 * gb
		fatEntryBytes      = 4
		fatEntryExtraSects = 8
		fatCount           = 2
	)
	var clusterBytes int64
	switch {
	case partSize <= cluster512Max:
		clusterBytes = blocksize
	case partSize <= cluster4kMax:
		clusterBytes = 4 * kb
	case partSize <= cluster8kMax:
		clusterBytes = 8 * kb
	case partSize <= cluster16kMax:
		clusterBytes = 16 * kb
	default:
		clusterBytes = 32 * kb
	}
	sectorsPerCluster := clusterBytes / blocksize
	if sectorsPerCluster == 0 {
		sectorsPerCluster = 1
	}
	totalSectors := partSize / blocksize
	fatEntryDenom := blocksize*sectorsPerCluster + fatEntryExtraSects
	sectorsPerFat := (fatEntryBytes*(totalSectors-reservedSectors) + fatEntryDenom - 1) / fatEntryDenom
	dataSectors := totalSectors - reservedSectors - fatCount*sectorsPerFat
	if dataSectors <= 0 {
		return 0
	}
	clusterCount := dataSectors / sectorsPerCluster
	return clusterCount * clusterBytes
}
